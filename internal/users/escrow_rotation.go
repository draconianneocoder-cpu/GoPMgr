// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"

	"gopmgr/internal/crypto"
)

// Escrow rotation (ADR-004 decision 3, ADR-005 part 3). Rotation replaces the
// administrator key: it makes a new key, re-seals every account's DEK to it,
// and recomputes each account's pin from that DEK, so no user does anything
// and nothing trusts system.db's plaintext rows. Only someone holding the
// current key can rotate, and the new key goes only to them and to accounts
// they name in the same action, never to accounts read from a table.
//
// Rotation runs when someone stops holding the key. When the person who
// should rotate is not present, the rotation is marked pending and runs at
// the next administrator sign-in of a key holder; it never runs in the
// session of the person leaving.

// ErrStandbyChanged is returned when a rotation names, as the standby to
// keep the key, an account that is no longer the standby.
var ErrStandbyChanged = errors.New("users: the standby has changed")

// RotationResult counts what a rotation did.
type RotationResult struct {
	// Resealed is the number of accounts whose DEK was sealed to the new key.
	Resealed int
	// Reenroll is the number of accounts that could not be re-sealed and are
	// sealed again at their next sign-in.
	Reenroll int
}

// markRotationPendingTx records that the key must be replaced at the next
// chance, with the reason. Nothing is marked when no key exists.
func markRotationPendingTx(ctx context.Context, q accountWriter, reason string) error {
	if _, ok, err := activeEscrowKey(ctx, q); err != nil || !ok {
		return err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET reason = excluded.reason, marked_at = excluded.marked_at`,
		reason, nowStamp()); err != nil {
		return fmt.Errorf("users: mark rotation pending: %w", err)
	}
	return nil
}

// rotationPendingTx returns the pending rotation's reason, or "" when none
// is pending.
func rotationPendingTx(ctx context.Context, q accountWriter) (string, error) {
	var reason string
	err := q.QueryRowContext(ctx, `SELECT reason FROM escrow_rotation_pending WHERE id = 1`).Scan(&reason)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("users: read pending rotation: %w", err)
	}
	return reason, nil
}

// holdsAnyGrantTx reports whether username holds a grant of any key.
func holdsAnyGrantTx(ctx context.Context, q accountWriter, username string) (bool, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM escrow_grants WHERE admin_username = ?`, username).Scan(&n); err != nil {
		return false, fmt.Errorf("users: read escrow grant: %w", err)
	}
	return n > 0, nil
}

// rotateInSavepointTx rotates inside a savepoint of the caller's
// transaction. If the rotation fails, only it is rolled back: the action
// that triggered it still commits, the rotation stays pending, and the
// failure is logged. It returns the new key's session (nil on failure),
// which the caller must Close.
func rotateInSavepointTx(ctx context.Context, q accountWriter, old *EscrowSession, actor string, keep []string, reason string) (*EscrowSession, RotationResult, error) {
	if _, err := q.ExecContext(ctx, `SAVEPOINT escrow_rotate`); err != nil {
		return nil, RotationResult{}, err
	}
	next, res, rotErr := rotateTx(ctx, q, old, actor, keep, reason)
	if rotErr != nil {
		if _, err := q.ExecContext(ctx, `ROLLBACK TO escrow_rotate`); err != nil {
			return nil, RotationResult{}, errors.Join(rotErr, err)
		}
	}
	if _, err := q.ExecContext(ctx, `RELEASE escrow_rotate`); err != nil {
		next.Close()
		return nil, RotationResult{}, errors.Join(rotErr, err)
	}
	if rotErr != nil {
		log.Printf("users: replacing the administrator key failed; it stays pending: %v", rotErr)
		return nil, RotationResult{}, markRotationPendingTx(ctx, q, reason)
	}
	return next, res, nil
}

// rotateTx replaces old, the active key opened from actor's own grant, with
// a new key, in the caller's transaction. Every grant is removed; the new
// key is granted to actor and to the accounts in keep, under the
// attestation check, and to nobody else. The caller must Close the
// returned session.
func rotateTx(ctx context.Context, q accountWriter, old *EscrowSession, actor string, keep []string, reason string) (*EscrowSession, RotationResult, error) {
	var res RotationResult
	id, err := crypto.NewEscrowKeyID()
	if err != nil {
		return nil, res, err
	}
	private, public, err := crypto.GenerateEscrowKeyPair()
	if err != nil {
		return nil, res, err
	}
	next := &EscrowSession{id: id, public: public, private: private}
	fail := func(err error) (*EscrowSession, RotationResult, error) {
		next.Close()
		return nil, res, err
	}

	// Retire first: at most one key is active (escrow_keys_one_active).
	now := nowStamp()
	if _, err := q.ExecContext(ctx, `UPDATE escrow_keys SET retired_at = ? WHERE id = ?`, now, old.id); err != nil {
		return fail(fmt.Errorf("users: retire escrow key: %w", err))
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO escrow_keys (id, kem_id, public_key, created_at) VALUES (?, ?, ?, ?)`,
		id, crypto.EscrowKEMID, public, now); err != nil {
		return fail(fmt.Errorf("users: store escrow key: %w", err))
	}

	if err := resealTx(ctx, q, old, next, &res); err != nil {
		return fail(err)
	}
	if err := reattestTx(ctx, q, old, next); err != nil {
		return fail(err)
	}

	if _, err := q.ExecContext(ctx, `DELETE FROM escrow_grants`); err != nil {
		return fail(fmt.Errorf("users: remove escrow grants: %w", err))
	}
	recipients := []string{actor}
	for _, name := range keep {
		if name != "" && !slices.Contains(recipients, name) {
			recipients = append(recipients, name)
		}
	}
	for _, name := range recipients {
		var public, attestation []byte
		var attestedKeyID string
		err := q.QueryRowContext(ctx,
			`SELECT public_key, attestation, attested_escrow_key_id FROM personal_keys WHERE username = ?`,
			name).Scan(&public, &attestation, &attestedKeyID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fail(fmt.Errorf("users: read personal key: %w", err))
		}
		err = attestAndGrantTx(ctx, q, next, actor, name, public, attestation, attestedKeyID)
		if errors.Is(err, ErrPersonalKeyNotAttested) {
			if err := recordAccountEvent(ctx, q, actor, name, AccountEscrowKeyMismatch,
				"the account key is not the one an administrator checked, so it was not given the new administrator key"); err != nil {
				return fail(err)
			}
			continue
		}
		if err != nil {
			return fail(err)
		}
	}

	if _, err := q.ExecContext(ctx, `DELETE FROM escrow_rotation_pending`); err != nil {
		return fail(fmt.Errorf("users: clear pending rotation: %w", err))
	}
	// A recovery code opens only the key it was made for (ADR-005 part 3b).
	if _, err := q.ExecContext(ctx, `DELETE FROM escrow_recovery`); err != nil {
		return fail(fmt.Errorf("users: remove recovery code: %w", err))
	}
	detail := fmt.Sprintf("%s; %d accounts sealed to the new key", reason, res.Resealed)
	if res.Reenroll > 0 {
		detail += fmt.Sprintf("; %d sealed again at their next sign-in", res.Reenroll)
	}
	if err := recordAccountEvent(ctx, q, actor, actor, AccountEscrowRotated, detail); err != nil {
		return fail(err)
	}
	return next, res, nil
}

// resealTx opens every DEK sealed to old, seals it to next, and recomputes
// the account's pin from the DEK. A DEK that does not open, and any pin or
// sealed DEK for another key, is removed so the account enrolls again at
// its next sign-in (recorded then as re-enrollment).
func resealTx(ctx context.Context, q accountWriter, old, next *EscrowSession, res *RotationResult) error {
	type sealedRow struct {
		username string
		sealed   []byte
	}
	rows, err := q.QueryContext(ctx, `SELECT username, sealed FROM sealed_deks WHERE escrow_key_id = ?`, old.id)
	if err != nil {
		return fmt.Errorf("users: list sealed DEKs: %w", err)
	}
	var sealed []sealedRow
	for rows.Next() {
		var r sealedRow
		if err := rows.Scan(&r.username, &r.sealed); err != nil {
			_ = rows.Close()
			return err
		}
		sealed = append(sealed, r)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	now := nowStamp()
	for _, r := range sealed {
		dek, err := crypto.OpenDEK(old.private, old.id, r.username, r.sealed)
		if err != nil {
			if err := recordAccountEvent(ctx, q, r.username, r.username, AccountEscrowKeyMismatch,
				"the account's sealed key does not open with the administrator key; it is sealed again at the account's next sign-in"); err != nil {
				return err
			}
			continue // removed below with the other stale rows
		}
		resealed, sealErr := crypto.SealDEK(next.public, next.id, r.username, dek)
		pin, pinErr := crypto.EscrowPin(dek, r.username, next.id, next.public)
		clear(dek)
		if err := errors.Join(sealErr, pinErr); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx,
			`UPDATE sealed_deks SET escrow_key_id = ?, sealed = ?, sealed_at = ? WHERE username = ?`,
			next.id, resealed, now, r.username); err != nil {
			return fmt.Errorf("users: store sealed DEK: %w", err)
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO escrow_pins (username, escrow_key_id, pin) VALUES (?, ?, ?)
			 ON CONFLICT(username) DO UPDATE SET escrow_key_id = excluded.escrow_key_id, pin = excluded.pin`,
			r.username, next.id, pin); err != nil {
			return fmt.Errorf("users: store escrow pin: %w", err)
		}
		res.Resealed++
	}

	stale, err := q.ExecContext(ctx, `DELETE FROM escrow_pins WHERE escrow_key_id <> ?`, next.id)
	if err != nil {
		return fmt.Errorf("users: remove stale escrow pins: %w", err)
	}
	if n, err := stale.RowsAffected(); err == nil {
		res.Reenroll = int(n)
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM sealed_deks WHERE escrow_key_id <> ?`, next.id); err != nil {
		return fmt.Errorf("users: remove stale sealed DEKs: %w", err)
	}
	return nil
}

// reattestTx attests with next every personal key whose attestation by old
// verifies, so the chain of checks carries over to the new key. One that
// does not verify is recorded and marked as attested by next while keeping
// its old attestation, which does not verify under next: a grant to it stays
// refused (ErrPersonalKeyNotAttested), as it was before, instead of being
// trusted on first use as a key the new key never saw.
func reattestTx(ctx context.Context, q accountWriter, old, next *EscrowSession) error {
	type personalRow struct {
		username            string
		public, attestation []byte
	}
	rows, err := q.QueryContext(ctx,
		`SELECT username, public_key, attestation FROM personal_keys WHERE attested_escrow_key_id = ?`, old.id)
	if err != nil {
		return fmt.Errorf("users: list attested personal keys: %w", err)
	}
	var attested []personalRow
	for rows.Next() {
		var r personalRow
		if err := rows.Scan(&r.username, &r.public, &r.attestation); err != nil {
			_ = rows.Close()
			return err
		}
		attested = append(attested, r)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, r := range attested {
		ok, err := crypto.VerifyAttestation(old.private, r.username, r.public, r.attestation)
		if err != nil {
			return err
		}
		if !ok {
			if err := recordAccountEvent(ctx, q, r.username, r.username, AccountEscrowKeyMismatch, "the account key is not the one an administrator checked"); err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, `UPDATE personal_keys SET attested_escrow_key_id = ? WHERE username = ?`, next.id, r.username); err != nil {
				return fmt.Errorf("users: keep refusing personal key: %w", err)
			}
			continue
		}
		if err := attestTx(ctx, q, next, r.username, r.public); err != nil {
			return err
		}
	}
	return nil
}

// revokeGrantTx removes username's grant and, when they held one, marks the
// key for replacement: they may have kept a copy of system.db with it.
func revokeGrantTx(ctx context.Context, q accountWriter, username, reason string) error {
	held, err := holdsAnyGrantTx(ctx, q, username)
	if err != nil || !held {
		return err
	}
	if err := removeGrantTx(ctx, q, username); err != nil {
		return err
	}
	return markRotationPendingTx(ctx, q, reason)
}

// rotateWithOwnKeyTx replaces the key now with actor's own key, keeping it
// for actor and keep, or marks the rotation pending when actor's key does
// not open.
func rotateWithOwnKeyTx(ctx context.Context, q accountWriter, actor string, actorDEK []byte, keep []string, reason string) error {
	session, err := openGrantTx(ctx, q, actor, actorDEK)
	if errors.Is(err, ErrNoEscrowGrant) || errors.Is(err, ErrEscrowMismatch) {
		return markRotationPendingTx(ctx, q, reason)
	}
	if err != nil {
		return err
	}
	defer session.Close()
	next, _, err := rotateInSavepointTx(ctx, q, session, actor, keep, reason)
	next.Close()
	return err
}

// rotatorTx reports whether actor, signing in or acting with their own
// usable key, should run a pending rotation: the super administrator, or
// the standby when the super administrator holds no key.
func rotatorTx(ctx context.Context, q accountWriter, actor string) (bool, error) {
	st, err := readSuccessionTx(ctx, q)
	if err != nil {
		return false, err
	}
	if actor == st.super {
		return true, nil
	}
	if actor != st.standby || st.super == "" {
		return false, nil
	}
	superHolds, err := holdsAnyGrantTx(ctx, q, st.super)
	return !superHolds, err
}

// runPendingRotationTx runs a pending rotation with session, actor's opened
// key, when actor is the one to run it. It returns the session to use
// afterwards: the new key's after a rotation, otherwise session itself.
func runPendingRotationTx(ctx context.Context, q accountWriter, session *EscrowSession, actor string) (*EscrowSession, error) {
	reason, err := rotationPendingTx(ctx, q)
	if err != nil || reason == "" {
		return session, err
	}
	ok, err := rotatorTx(ctx, q, actor)
	if err != nil || !ok {
		return session, err
	}
	next, _, err := rotateInSavepointTx(ctx, q, session, actor, nil, reason)
	if err != nil {
		return session, err
	}
	if next == nil {
		return session, nil
	}
	session.Close()
	return next, nil
}

// RunPendingRotation runs a pending rotation now when actor, with their DEK,
// is the one to run it, as after demoting, disabling, or deleting a key
// holder. It does nothing when no rotation is pending or actor holds no
// usable key.
func (s *Store) RunPendingRotation(actor string, dek []byte) error {
	return s.inWriteTx("pending key replacement", func(ctx context.Context, q accountWriter) error {
		if reason, err := rotationPendingTx(ctx, q); err != nil || reason == "" {
			return err
		}
		session, err := openGrantTx(ctx, q, actor, dek)
		if errors.Is(err, ErrNoEscrowGrant) || errors.Is(err, ErrEscrowMismatch) {
			return nil
		}
		if err != nil {
			return err
		}
		next, err := runPendingRotationTx(ctx, q, session, actor)
		next.Close()
		return err
	})
}

// ReplaceEscrowKey replaces the administrator key on demand, on behalf of
// actor, the super administrator, who must hold a usable key
// (ErrNoEscrowGrant otherwise). keepStandby, when not empty, must be the
// current standby, named by actor so they keep the key
// (ErrStandbyChanged otherwise); a standby not named loses it.
func (s *Store) ReplaceEscrowKey(actor string, dek []byte, keepStandby string) (RotationResult, error) {
	var res RotationResult
	var noKey bool
	err := s.inWriteTx("administrator key replacement", func(ctx context.Context, q accountWriter) error {
		if err := requireSuperTx(ctx, q, actor); err != nil {
			return err
		}
		if keepStandby != "" {
			st, err := readSuccessionTx(ctx, q)
			if err != nil {
				return err
			}
			if keepStandby != st.standby {
				return ErrStandbyChanged
			}
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
		next, r, err := rotateTx(ctx, q, session, actor, []string{keepStandby}, "replaced by the super administrator")
		if err != nil {
			return err
		}
		next.Close()
		res = r
		return nil
	})
	if err == nil && noKey {
		err = ErrNoEscrowGrant
	}
	return res, err
}
