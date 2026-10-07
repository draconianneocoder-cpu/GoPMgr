// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopmgr/internal/crypto"
)

// Administrator key recovery code (ADR-005 part 3b). The super
// administrator creates a code for the active administrator key and prints
// it; the key's private half is stored wrapped under the code. When nobody
// can open the key any more, entering the code restores that same key to
// the super administrator: no new key, no re-pinning. The code proves
// nothing on its own: the restored key must match the super
// administrator's own pin, so a row forged into system.db with another key
// and another code restores nothing.
//
// The code opens every account's data sealed to its key, together with
// any copy of system.db that holds its row: it is treated like the key
// itself, never logged or recorded, and a rotation voids it.

// ErrNoRecoveryCode is returned when the active administrator key has no
// recovery code.
var ErrNoRecoveryCode = errors.New("users: the administrator key has no recovery code")

// ErrInvalidEscrowRecoveryCode is returned for a recovery code that is
// malformed or does not open the administrator key.
var ErrInvalidEscrowRecoveryCode = errors.New("users: the recovery code does not open the administrator key")

// escrowRecoveryEncoding writes a code as 32 base32 characters.
var escrowRecoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newEscrowRecoveryCode returns a new code's bytes and how it is shown:
// four groups of eight characters.
func newEscrowRecoveryCode() ([]byte, string, error) {
	raw := make([]byte, crypto.EscrowRecoveryCodeSize)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, "", fmt.Errorf("users: recovery code entropy: %w", err)
	}
	enc := escrowRecoveryEncoding.EncodeToString(raw)
	return raw, enc[0:8] + "-" + enc[8:16] + "-" + enc[16:24] + "-" + enc[24:32], nil
}

// parseEscrowRecoveryCode reads a typed code. Case, spaces, and dashes do
// not matter, and the digits 0, 1, and 8, which the code never contains,
// are read as the letters O, I, and B they are easily written as.
func parseEscrowRecoveryCode(typed string) ([]byte, error) {
	c := strings.NewReplacer("0", "O", "1", "I", "8", "B").Replace(canonicalise(typed))
	// Exactly 32 characters: unpadded base32 would otherwise ignore a
	// stray character at the end.
	if len(c) != escrowRecoveryEncoding.EncodedLen(crypto.EscrowRecoveryCodeSize) {
		return nil, ErrInvalidEscrowRecoveryCode
	}
	raw, err := escrowRecoveryEncoding.DecodeString(c)
	if err != nil || len(raw) != crypto.EscrowRecoveryCodeSize {
		return nil, ErrInvalidEscrowRecoveryCode
	}
	return raw, nil
}

// recoveryStateTx reports whether the active key has a recovery code, and
// whether its holder has confirmed it by typing it back.
func recoveryStateTx(ctx context.Context, q accountWriter) (exists, confirmed bool, err error) {
	active, ok, err := activeEscrowKey(ctx, q)
	if err != nil || !ok {
		return false, false, err
	}
	var c int
	err = q.QueryRowContext(ctx, `SELECT confirmed FROM escrow_recovery WHERE escrow_key_id = ?`, active.id).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("users: read recovery code: %w", err)
	}
	return true, c == 1, nil
}

// CreateEscrowRecoveryCode makes a recovery code for the active
// administrator key, replacing any earlier one, on behalf of actor, the
// super administrator, who must hold a usable key (ErrNoEscrowGrant
// otherwise). It returns the code, shown once; it counts as made only once
// confirmed with ConfirmEscrowRecoveryCode.
func (s *Store) CreateEscrowRecoveryCode(actor string, dek []byte) (string, error) {
	var code string
	var noKey bool
	err := s.inWriteTx("recovery code", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		session, err := openGrantTx(ctx, q, actor, dek)
		if errors.Is(err, ErrNoEscrowGrant) || errors.Is(err, ErrEscrowMismatch) {
			noKey = true
			return nil // commit a recorded mismatch
		}
		if err != nil {
			return err
		}
		defer session.Close()

		raw, display, err := newEscrowRecoveryCode()
		if err != nil {
			return err
		}
		wrapped, err := crypto.WrapEscrowRecovery(raw, session.id, session.private)
		clear(raw)
		if err != nil {
			return err
		}
		res, err := q.ExecContext(ctx, `DELETE FROM escrow_recovery`)
		if err != nil {
			return fmt.Errorf("users: remove recovery code: %w", err)
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO escrow_recovery (escrow_key_id, wrapped, confirmed, created_at, created_by) VALUES (?, ?, 0, ?, ?)`,
			session.id, wrapped, nowStamp(), actor); err != nil {
			return fmt.Errorf("users: store recovery code: %w", err)
		}
		detail := ""
		if n, _ := res.RowsAffected(); n > 0 {
			detail = "replaces the earlier one"
		}
		if err := recordAccountEvent(ctx, q, actor, actor, AccountEscrowRecoveryCreated, detail); err != nil {
			return err
		}
		code = display
		return nil
	})
	if err == nil && noKey {
		err = ErrNoEscrowGrant
	}
	return code, err
}

// openRecoveryTx opens the active key's recovery row with typed, returning
// the active key and the private key the code opens (the caller must clear
// it). It returns ErrNoRecoveryCode or ErrInvalidEscrowRecoveryCode.
func openRecoveryTx(ctx context.Context, q accountWriter, typed string) (escrowKeyRef, []byte, error) {
	active, ok, err := activeEscrowKey(ctx, q)
	if err != nil {
		return escrowKeyRef{}, nil, err
	}
	if !ok {
		return escrowKeyRef{}, nil, ErrNoRecoveryCode
	}
	var wrapped []byte
	err = q.QueryRowContext(ctx, `SELECT wrapped FROM escrow_recovery WHERE escrow_key_id = ?`, active.id).Scan(&wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return escrowKeyRef{}, nil, ErrNoRecoveryCode
	}
	if err != nil {
		return escrowKeyRef{}, nil, fmt.Errorf("users: read recovery code: %w", err)
	}
	raw, err := parseEscrowRecoveryCode(typed)
	if err != nil {
		return escrowKeyRef{}, nil, err
	}
	defer clear(raw)
	private, err := crypto.OpenEscrowRecovery(raw, active.id, wrapped)
	if err != nil {
		return escrowKeyRef{}, nil, ErrInvalidEscrowRecoveryCode
	}
	return active, private, nil
}

// CheckEscrowRecoveryCode reports whether typed is the active key's
// recovery code, for actor, the super administrator, without confirming or
// recording anything: printing a code checks it first, and a typo is not an
// attack.
func (s *Store) CheckEscrowRecoveryCode(actor, typed string) error {
	return s.inWriteTx("recovery code check", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		_, private, err := openRecoveryTx(ctx, q, typed)
		if err != nil {
			return err
		}
		clear(private)
		return nil
	})
}

// ConfirmEscrowRecoveryCode marks the active key's recovery code as kept,
// once actor, the super administrator, types it back correctly
// (ErrInvalidEscrowRecoveryCode otherwise; nothing is recorded for a typo).
func (s *Store) ConfirmEscrowRecoveryCode(actor, typed string) error {
	return s.inWriteTx("recovery code confirmation", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		active, private, err := openRecoveryTx(ctx, q, typed)
		if err != nil {
			return err
		}
		clear(private)
		if _, err := q.ExecContext(ctx, `UPDATE escrow_recovery SET confirmed = 1 WHERE escrow_key_id = ?`, active.id); err != nil {
			return fmt.Errorf("users: confirm recovery code: %w", err)
		}
		return nil
	})
}

// RecoverEscrowKey restores the active administrator key to actor, the
// super administrator, from its recovery code. The key the code opens must
// be the one actor's own pin trusts and the one system.db names as active;
// otherwise nothing is granted and the mismatch is recorded
// (ErrEscrowMismatch). Actor must have signed in since the key was made, so
// that a pin exists: there is no trust on first use here. A code that does
// not open is recorded, without the code (ErrInvalidEscrowRecoveryCode).
// Afterwards a pending replacement of the key runs, which voids the code.
func (s *Store) RecoverEscrowKey(actor string, dek []byte, typed string) error {
	var refused error
	err := s.inWriteTx("administrator key recovery", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		active, private, err := openRecoveryTx(ctx, q, typed)
		if errors.Is(err, ErrInvalidEscrowRecoveryCode) {
			refused = err
			return recordAccountEvent(ctx, q, actor, actor, AccountEscrowKeyMismatch,
				"a recovery code that does not open the administrator key was entered")
		}
		if err != nil {
			return err
		}
		defer clear(private)

		mismatch := func(detail string) error {
			refused = ErrEscrowMismatch
			return recordAccountEvent(ctx, q, actor, actor, AccountEscrowKeyMismatch, detail)
		}
		derived, err := crypto.EscrowPublicKey(private)
		if err != nil {
			return mismatch("the recovery code opened something that is not an administrator key")
		}
		var pinKeyID string
		var pin []byte
		err = q.QueryRowContext(ctx, `SELECT escrow_key_id, pin FROM escrow_pins WHERE username = ?`, actor).Scan(&pinKeyID, &pin)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && pinKeyID != active.id) {
			return mismatch("the super administrator's account does not yet trust this administrator key; sign in again, then use the recovery code")
		}
		if err != nil {
			return fmt.Errorf("users: read escrow pin: %w", err)
		}
		if ok, err := crypto.VerifyEscrowPin(dek, actor, active.id, derived, pin); err != nil {
			return err
		} else if !ok {
			return mismatch("the key the recovery code opened is not the administrator key this account trusts")
		}
		if !bytes.Equal(derived, active.public) {
			return mismatch("system.db's record of the administrator key was changed outside GoPMgr")
		}

		_, personal, err := ensurePersonalKey(ctx, q, actor, dek)
		if err != nil {
			return err
		}
		session := &EscrowSession{id: active.id, public: derived, private: bytes.Clone(private)}
		if err := attestTx(ctx, q, session, actor, personal); err != nil {
			session.Close()
			return err
		}
		if err := grantTx(ctx, q, session, actor, personal); err != nil {
			session.Close()
			return err
		}
		if err := recordAccountEvent(ctx, q, actor, actor, AccountEscrowRecovered, ""); err != nil {
			session.Close()
			return err
		}
		next, err := runPendingRotationTx(ctx, q, session, actor)
		next.Close()
		return err
	})
	if err == nil {
		err = refused
	}
	return err
}
