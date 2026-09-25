// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gopmgr/internal/auth"
	"gopmgr/internal/crypto"
)

// Rotating recovery codes from App Settings happens in two steps, so a
// user never ends up with no working codes: PrepareRecoveryCodes makes the
// new set without storing it, and ConfirmRecoveryCodes stores it only
// after the user says they saved the codes. Until then the old codes keep
// working, and a closed window or crash leaves them in place.

// ErrRecoveryCodesChanged is returned by ConfirmRecoveryCodes when the
// account's codes changed after the new set was prepared, for example
// because a code was used or another GoPMgr process issued new ones.
var ErrRecoveryCodesChanged = errors.New("users: recovery codes changed since the new ones were prepared")

// PendingRecoveryCodes is a prepared, not yet stored, set of codes.
type PendingRecoveryCodes struct {
	username string
	set      recoveryCodeSet
	basis    string
}

// Codes returns the plaintext codes to show the user.
func (p *PendingRecoveryCodes) Codes() []string {
	return append([]string(nil), p.set.plain...)
}

// Username returns the account the codes were prepared for.
func (p *PendingRecoveryCodes) Username() string {
	return p.username
}

// recoveryCodesBasis summarises username's stored codes: any issue, reset,
// or use changes it, because issuing inserts rows with new IDs and using a
// code marks it used.
func recoveryCodesBasis(ctx context.Context, q accountWriter, username string) (string, error) {
	var n, maxID, used int64
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(id), 0), COALESCE(SUM(used), 0) FROM recovery_codes WHERE username = ?`,
		username,
	).Scan(&n, &maxID, &used)
	return fmt.Sprintf("%d/%d/%d", n, maxID, used), err
}

// PrepareRecoveryCodes makes a new set of codes for username without
// storing anything. currentPassword must verify (auth.ErrMismatch
// otherwise): new codes are a lasting way into the account, so issuing
// them needs the same proof as a password change. The DEK is unwrapped
// from the stored password wrap and wrapped into every new code, so the
// codes always recover the stored key.
func (s *Store) PrepareRecoveryCodes(username, currentPassword string) (*PendingRecoveryCodes, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, ErrNoSuchUser
	}
	var hash, wrapped string
	err := s.conn.QueryRow(
		`SELECT password_hash, wrapped_dek_pw FROM users WHERE username = ?`, username,
	).Scan(&hash, &wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSuchUser
	}
	if err != nil {
		return nil, err
	}
	if err := auth.VerifyPassword(currentPassword, hash); err != nil {
		return nil, err
	}
	if wrapped == "" {
		return nil, ErrPasswordWrapCorrupt
	}
	dek, err := crypto.UnwrapKey(wrapped, currentPassword)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPasswordWrapCorrupt, err)
	}
	defer zero(dek)

	basis, err := recoveryCodesBasis(context.Background(), s.conn, username)
	if err != nil {
		return nil, err
	}
	set, err := newRecoveryCodeSet(dek)
	if err != nil {
		return nil, err
	}
	return &PendingRecoveryCodes{username: username, set: set, basis: basis}, nil
}

// ConfirmRecoveryCodes stores a prepared set, replacing every existing
// code for its account in one transaction. It returns
// ErrRecoveryCodesChanged, and stores nothing, if the account's codes
// changed after the set was prepared.
func (s *Store) ConfirmRecoveryCodes(p *PendingRecoveryCodes) error {
	if p == nil {
		return errors.New("users: no prepared recovery codes")
	}
	return s.inWriteTx("recovery code rotation", func(ctx context.Context, q accountWriter) error {
		basis, err := recoveryCodesBasis(ctx, q, p.username)
		if err != nil {
			return err
		}
		if basis != p.basis {
			return ErrRecoveryCodesChanged
		}
		return replaceRecoveryCodes(ctx, q, p.username, p.set)
	})
}

// RecoveryCodeStatus reports how many of username's codes are unused and
// whether any unused code is a legacy one without a DEK wrap. Using a
// legacy code resets the password with a new DEK, which cannot open
// encrypted projects, so such codes must not be counted as a safe way back.
func (s *Store) RecoveryCodeStatus(username string) (unused int, legacy bool, err error) {
	unused, err = s.RemainingRecoveryCodes(username)
	if err != nil {
		return 0, false, err
	}
	legacy, err = s.HasLegacyRecoveryCodeWraps(username)
	return unused, legacy, err
}
