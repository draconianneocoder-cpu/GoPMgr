// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopmgr/internal/crypto"
)

// Recorded administrator access (ADR-004 phase 2).

// MaxAccessReasonLength is the longest reason, in characters, an
// administrator may give for opening another account's data.
const MaxAccessReasonLength = 500

// ErrAccessReasonRequired is returned when an access reason is empty.
var ErrAccessReasonRequired = errors.New("users: a reason is required to open another account's data")

// ErrAccessReasonTooLong is returned when an access reason is longer than
// MaxAccessReasonLength characters.
var ErrAccessReasonTooLong = errors.New("users: the reason is too long")

// ErrSelfAccess is returned when an administrator asks to open their own
// account's data through administrator access.
var ErrSelfAccess = errors.New("users: administrators open their own data by signing in")

// ErrNotEnrolled is returned when the target account has no DEK sealed to
// the active escrow key, usually because it has not signed in since escrow
// began.
var ErrNotEnrolled = errors.New("users: account is not enrolled in administrator access")

// OpenUserForAdmin records actor's access to target's data and returns
// target's DEK. actor must be an enabled administrator holding a usable
// grant, with actorDEK their own DEK, and reason must say why.
//
// Everything is checked, the grant and the target's sealed DEK are opened,
// and admin_access is recorded with the reason in one BEGIN IMMEDIATE
// transaction. The DEK is returned only after that transaction commits, so
// no key is released without its record, and the record never lists access
// that could not happen. Refusals before the transaction record nothing.
// An unusable grant or a sealed DEK that does not open is recorded as a
// mismatch (and removed, as an administrator session would) and returns
// ErrEscrowMismatch with no access recorded.
//
// The caller must clear the returned DEK.
func (s *Store) OpenUserForAdmin(actor string, actorDEK []byte, target, reason string) ([]byte, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrAccessReasonRequired
	}
	if utf8.RuneCountInString(reason) > MaxAccessReasonLength {
		return nil, ErrAccessReasonTooLong
	}
	if strings.EqualFold(actor, target) {
		return nil, ErrSelfAccess
	}

	var dek []byte
	var refused error
	err := s.inWriteTx("administrator access", func(ctx context.Context, q accountWriter) error {
		if err := requireEnabledAdmin(ctx, q, actor); err != nil {
			return err
		}
		if _, _, err := accountRole(ctx, q, target); err != nil {
			return err
		}
		active, ok, err := activeEscrowKey(ctx, q)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEnrolled
		}
		var sealed []byte
		err = q.QueryRowContext(ctx,
			`SELECT sealed FROM sealed_deks WHERE username = ? AND escrow_key_id = ?`,
			target, active.id).Scan(&sealed)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotEnrolled
		}
		if err != nil {
			return fmt.Errorf("users: read sealed DEK: %w", err)
		}

		session, err := openGrantTx(ctx, q, actor, actorDEK)
		if errors.Is(err, ErrEscrowMismatch) {
			refused = err
			return nil // commit the recorded mismatch
		}
		if err != nil {
			return err
		}
		defer session.Close()

		opened, err := crypto.OpenDEK(session.private, session.id, target, sealed)
		if err != nil {
			refused = ErrEscrowMismatch
			if _, err := q.ExecContext(ctx, `DELETE FROM sealed_deks WHERE username = ?`, target); err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, `DELETE FROM escrow_pins WHERE username = ?`, target); err != nil {
				return err
			}
			return recordAccountEvent(ctx, q, target, target, AccountEscrowKeyMismatch,
				"the account's sealed key does not open with the administrator key; it is sealed again at the account's next sign-in")
		}
		if err := recordAccountEvent(ctx, q, actor, target, AccountAdminAccess, reason); err != nil {
			clear(opened)
			return err
		}
		dek = opened
		return nil
	})
	if err != nil {
		clear(dek)
		return nil, err
	}
	if refused != nil {
		clear(dek)
		return nil, refused
	}
	return dek, nil
}

// RequireEnabledAdmin returns ErrNotAdmin unless actor is an administrator
// who can sign in, read from system.db rather than a session.
func (s *Store) RequireEnabledAdmin(actor string) error {
	return requireEnabledAdmin(context.Background(), s.conn, actor)
}
