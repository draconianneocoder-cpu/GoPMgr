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

// A new administrator key when nobody holds the old one (ADR-005 part 3c).
// Rotation and the recovery code both need the old key. When neither is
// available, the super administrator makes a new key: it cannot re-seal
// anyone's DEK, so every other account's pin and sealed DEK are removed,
// and each account pins and seals to the new key at its next sign-in, by
// the same trust on first use as an account whose pin is missing. Until
// then nobody can open that account's data.
//
// This is the one key operation that rests on the plaintext role row, so
// every other account is told: an escrow_key_new event names it, shown in
// its sign-in notice until acknowledged (see access_notice.go).

// MaxNewKeyReasonLength is the longest reason, in characters, for making a
// new administrator key.
const MaxNewKeyReasonLength = 500

// ErrNewKeyReasonRequired is returned when the reason for a new
// administrator key is empty.
var ErrNewKeyReasonRequired = errors.New("users: a reason is required to make a new administrator key")

// ErrNewKeyReasonTooLong is returned when the reason is longer than
// MaxNewKeyReasonLength characters.
var ErrNewKeyReasonTooLong = fmt.Errorf("users: the reason can be at most %d characters", MaxNewKeyReasonLength)

// ErrHoldsEscrowKey is returned when the super administrator holds a usable
// administrator key: they replace it instead, which keeps everyone's data
// open.
var ErrHoldsEscrowKey = errors.New("users: the super administrator holds the administrator key")

// ErrKeyStillHeld matches a KeyHeldError.
var ErrKeyStillHeld = errors.New("users: another account holds the administrator key")

// KeyHeldError is returned when an enabled administrator other than the
// super administrator holds the active administrator key, so it is not
// lost. Removing their key allows a new one: removing the standby as
// standby, or demoting or disabling another administrator. The check reads
// plaintext rows, so a forged grant can only refuse a new key, never cause
// one.
type KeyHeldError struct {
	Holder  string
	Standby bool // Holder is the standby successor
}

func (e *KeyHeldError) Error() string {
	return fmt.Sprintf("users: %s holds the administrator key", e.Holder)
}

// Is reports whether target is ErrKeyStillHeld.
func (e *KeyHeldError) Is(target error) bool { return target == ErrKeyStillHeld }

// CreateNewEscrowKey makes a new administrator key on behalf of actor, the
// super administrator, with dek their own DEK, when nobody holds the old
// one. It returns the number of other accounts sealed to the new key at
// their next sign-in.
//
// It refuses, changing nothing, when actor holds a usable key
// (ErrHoldsEscrowKey), when another enabled administrator holds a grant of
// the active key (a KeyHeldError), and when no administrator key has ever been made: the
// first administrator sign-in makes it. A grant of actor's that does not
// open is removed and recorded, as at sign-in, and does not count as held.
// A database with keys but none active (changed outside GoPMgr) is
// repaired by the new key.
func (s *Store) CreateNewEscrowKey(actor string, dek []byte, reason string) (int, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return 0, ErrNewKeyReasonRequired
	}
	if utf8.RuneCountInString(reason) > MaxNewKeyReasonLength {
		return 0, ErrNewKeyReasonTooLong
	}
	var others int
	err := s.inWriteTx("new administrator key", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		var keys int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM escrow_keys`).Scan(&keys); err != nil {
			return fmt.Errorf("users: count escrow keys: %w", err)
		}
		if keys == 0 {
			return ErrNoEscrowGrant
		}
		session, err := openGrantTx(ctx, q, actor, dek)
		if err == nil {
			session.Close()
			return ErrHoldsEscrowKey
		}
		if !errors.Is(err, ErrNoEscrowGrant) && !errors.Is(err, ErrEscrowMismatch) {
			return err
		}
		// A grant left on an account that cannot sign in as an
		// administrator is not a holder; the new key removes it.
		var holder string
		err = q.QueryRowContext(ctx,
			`SELECT g.admin_username FROM escrow_grants g
			 JOIN escrow_keys k ON k.id = g.escrow_key_id AND k.retired_at = ''
			 JOIN users u ON u.username = g.admin_username AND u.is_admin = 1 AND u.disabled = 0
			 WHERE g.admin_username <> ? ORDER BY g.admin_username LIMIT 1`, actor).Scan(&holder)
		if err == nil {
			st, err := readSuccessionTx(ctx, q)
			if err != nil {
				return err
			}
			return &KeyHeldError{Holder: holder, Standby: holder == st.standby}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("users: read escrow grants: %w", err)
		}
		others, err = newEscrowKeyTx(ctx, q, actor, dek, reason)
		return err
	})
	if err != nil {
		return 0, err
	}
	return others, nil
}

// newEscrowKeyTx retires the active key (if any), makes a new one, removes
// every grant, sealed DEK, recovery code, pending replacement, and every
// other account's pin, enrolls actor in the new key and grants it to them,
// and records the new key for every other account.
func newEscrowKeyTx(ctx context.Context, q accountWriter, actor string, dek []byte, reason string) (int, error) {
	id, err := crypto.NewEscrowKeyID()
	if err != nil {
		return 0, err
	}
	private, public, err := crypto.GenerateEscrowKeyPair()
	if err != nil {
		return 0, err
	}
	session := &EscrowSession{id: id, public: public, private: private}
	defer session.Close()

	now := nowStamp()
	if _, err := q.ExecContext(ctx, `UPDATE escrow_keys SET retired_at = ? WHERE retired_at = ''`, now); err != nil {
		return 0, fmt.Errorf("users: retire escrow key: %w", err)
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO escrow_keys (id, kem_id, public_key, created_at) VALUES (?, ?, ?, ?)`,
		id, crypto.EscrowKEMID, public, now); err != nil {
		return 0, fmt.Errorf("users: store escrow key: %w", err)
	}
	for _, stmt := range []string{
		`DELETE FROM escrow_grants`,
		`DELETE FROM sealed_deks`,
		`DELETE FROM escrow_recovery`,
		`DELETE FROM escrow_rotation_pending`,
	} {
		if _, err := q.ExecContext(ctx, stmt); err != nil {
			return 0, fmt.Errorf("users: clear the old administrator key: %w", err)
		}
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM escrow_pins WHERE username <> ?`, actor); err != nil {
		return 0, fmt.Errorf("users: remove escrow pins: %w", err)
	}

	// actor made the key, so their pin moves to it now, and enrollTx then
	// seals their DEK and attests their personal key as usual.
	pin, err := crypto.EscrowPin(dek, actor, id, public)
	if err != nil {
		return 0, err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO escrow_pins (username, escrow_key_id, pin) VALUES (?, ?, ?)
		 ON CONFLICT(username) DO UPDATE SET escrow_key_id = excluded.escrow_key_id, pin = excluded.pin`,
		actor, id, pin); err != nil {
		return 0, fmt.Errorf("users: store escrow pin: %w", err)
	}
	personal, err := enrollTx(ctx, q, actor, dek, session)
	if err != nil {
		return 0, err
	}
	if err := grantTx(ctx, q, session, actor, personal); err != nil {
		return 0, err
	}

	rows, err := q.QueryContext(ctx, `SELECT username FROM users WHERE username <> ? ORDER BY username`, actor)
	if err != nil {
		return 0, fmt.Errorf("users: list accounts: %w", err)
	}
	var others []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return 0, err
		}
		others = append(others, name)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return 0, err
	}
	for _, name := range others {
		if err := recordAccountEvent(ctx, q, actor, name, AccountEscrowKeyNew, reason); err != nil {
			return 0, err
		}
	}
	detail := fmt.Sprintf("%s; %d accounts sealed to it at their next sign-in", reason, len(others))
	if len(others) == 1 {
		detail = reason + "; 1 account sealed to it at its next sign-in"
	}
	if err := recordAccountEvent(ctx, q, actor, actor, AccountEscrowKeyNew, detail); err != nil {
		return 0, err
	}
	return len(others), nil
}
