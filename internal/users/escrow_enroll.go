// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"gopmgr/internal/crypto"
)

// Administrator escrow enrollment (ADR-004 phase 1). An account enrolls by
// having a personal key pair, a pin of the escrow key it trusts, and its
// DEK sealed to that escrow key. Administrators additionally hold a grant:
// the escrow private key sealed to their personal key. Nothing here opens
// another account's DEK; the recorded access flow is phase 2.
//
// Grants are added only inside an administrator's own explicit action:
// creating the escrow key, creating an administrator account, promoting an
// account, or enabling a disabled administrator. A sign-in never grants:
// is_admin and disabled are plain columns, so granting to whoever they name
// would hand the escrow key to anyone who can write system.db.

// ErrNoEscrowGrant is returned when an administrator holds no grant for
// the active escrow key yet. Another administrator's session adds it.
var ErrNoEscrowGrant = errors.New("users: no escrow grant for this administrator")

// ErrEscrowMismatch is returned when an escrow key does not match the pin
// that trusts it, so it may have been swapped or forged.
var ErrEscrowMismatch = errors.New("users: escrow key does not match its pin")

// ErrPersonalKeyNotAttested is returned when an account's personal public
// key fails the escrow key's attestation, so it may have been swapped.
var ErrPersonalKeyNotAttested = errors.New("users: personal key fails its attestation")

// ErrNoPersonalKey is returned when promoting an account that has no
// personal key yet: it must sign in once first, so the grant has a key to
// be sealed to.
var ErrNoPersonalKey = errors.New("users: account has no personal key yet")

// EscrowSession is an administrator's opened escrow key: its ID, the public
// key derived from the private key (never read from disk), and the private
// key itself. Close clears the private key.
type EscrowSession struct {
	id      string
	public  []byte
	private []byte
}

// Close clears the escrow private key. Safe to call on nil.
func (e *EscrowSession) Close() {
	if e != nil {
		clear(e.private)
		e.private = nil
	}
}

// escrowKeyRef is the escrow key an enrollment seals to: a session's when
// an administrator holds one, otherwise the active key read from disk.
type escrowKeyRef struct {
	id     string
	public []byte
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// activeEscrowKey returns the active escrow key on disk, or ok=false.
func activeEscrowKey(ctx context.Context, q accountWriter) (ref escrowKeyRef, ok bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT id, public_key FROM escrow_keys WHERE retired_at = ''`).Scan(&ref.id, &ref.public)
	if errors.Is(err, sql.ErrNoRows) {
		return escrowKeyRef{}, false, nil
	}
	if err != nil {
		return escrowKeyRef{}, false, fmt.Errorf("users: read escrow key: %w", err)
	}
	return ref, true, nil
}

// enrollTx enrolls username inside the caller's transaction. With session
// nil it trusts the active escrow key on disk (checked against the
// account's pin after the first time); with a session it pins and seals to
// the session's key and attests the account's personal key. With no escrow
// key at all there is nothing to enroll in, and it does nothing.
//
// It returns the account's personal public key, derived from its private
// key, once the account is enrolled, and nil when it enrolled nothing.
func enrollTx(ctx context.Context, q accountWriter, username string, dek []byte, session *EscrowSession) ([]byte, error) {
	var key escrowKeyRef
	if session != nil {
		key = escrowKeyRef{id: session.id, public: session.public}
	} else {
		active, ok, err := activeEscrowKey(ctx, q)
		if err != nil || !ok {
			return nil, err
		}
		key = active
	}

	hadPersonalKey, personalPublic, err := ensurePersonalKey(ctx, q, username, dek)
	if err != nil {
		return nil, err
	}

	var pinKeyID string
	var pin []byte
	err = q.QueryRowContext(ctx, `SELECT escrow_key_id, pin FROM escrow_pins WHERE username = ?`, username).Scan(&pinKeyID, &pin)
	hadPin := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("users: read escrow pin: %w", err)
	}
	if hadPin {
		ok := pinKeyID == key.id
		if ok {
			if ok, err = crypto.VerifyEscrowPin(dek, username, key.id, key.public, pin); err != nil {
				return nil, err
			}
		}
		if !ok {
			// Seal nothing to a key this account did not pin.
			return nil, recordAccountEvent(ctx, q, username, username, AccountEscrowKeyMismatch, "the administrator key in system.db is not the one this account trusts, so nothing was sealed")
		}
	} else {
		pin, err := crypto.EscrowPin(dek, username, key.id, key.public)
		if err != nil {
			return nil, err
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO escrow_pins (username, escrow_key_id, pin) VALUES (?, ?, ?)`, username, key.id, pin); err != nil {
			return nil, fmt.Errorf("users: store escrow pin: %w", err)
		}
	}
	// Pins and personal keys are created together, so having only one
	// means the other was removed: this enrollment trusts the escrow key
	// on disk again (trust on first use), which is recorded.
	if hadPin != hadPersonalKey {
		if err := recordAccountEvent(ctx, q, username, username, AccountEscrowReenrolled, ""); err != nil {
			return nil, err
		}
	}

	var sealedKeyID string
	err = q.QueryRowContext(ctx, `SELECT escrow_key_id FROM sealed_deks WHERE username = ?`, username).Scan(&sealedKeyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("users: read sealed DEK: %w", err)
	}
	if sealedKeyID != key.id {
		sealed, err := crypto.SealDEK(key.public, key.id, username, dek)
		if err != nil {
			return nil, err
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO sealed_deks (username, escrow_key_id, sealed, sealed_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(username) DO UPDATE SET escrow_key_id = excluded.escrow_key_id, sealed = excluded.sealed, sealed_at = excluded.sealed_at`,
			username, key.id, sealed, nowStamp()); err != nil {
			return nil, fmt.Errorf("users: store sealed DEK: %w", err)
		}
	}

	if session != nil {
		if err := attestTx(ctx, q, session, username, personalPublic); err != nil {
			return nil, err
		}
	}
	return personalPublic, nil
}

// enrollInSavepoint runs enrollTx inside a savepoint of the caller's
// transaction. If enrolling fails, only the enrollment is rolled back, so
// the account's own change (a new DEK, a new account) still commits and a
// later sign-in retries enrollment. It returns the enrollment error for the
// caller to report.
//
// With grant set and a session, the account is also granted the session's
// escrow key: an administrator account created by an administrator, whose
// personal key was made in this same transaction.
func enrollInSavepoint(ctx context.Context, q accountWriter, username string, dek []byte, session *EscrowSession, grant bool) error {
	if _, err := q.ExecContext(ctx, `SAVEPOINT escrow_enroll`); err != nil {
		return err
	}
	public, enrollErr := enrollTx(ctx, q, username, dek, session)
	if enrollErr == nil && grant && session != nil && public != nil {
		enrollErr = grantTx(ctx, q, session, username, public)
	}
	if enrollErr != nil {
		if _, err := q.ExecContext(ctx, `ROLLBACK TO escrow_enroll`); err != nil {
			return errors.Join(enrollErr, err)
		}
	}
	if _, err := q.ExecContext(ctx, `RELEASE escrow_enroll`); err != nil {
		return errors.Join(enrollErr, err)
	}
	return enrollErr
}

// ensurePersonalKey makes username's personal key pair if it has none, and
// otherwise opens it with the DEK and repairs a swapped public key. It
// reports whether a key existed before and returns the public key.
func ensurePersonalKey(ctx context.Context, q accountWriter, username string, dek []byte) (existed bool, public []byte, err error) {
	var stored, wrapped []byte
	err = q.QueryRowContext(ctx, `SELECT public_key, wrapped_private_key FROM personal_keys WHERE username = ?`, username).Scan(&stored, &wrapped)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, nil, fmt.Errorf("users: read personal key: %w", err)
	}
	existed = err == nil
	if existed {
		private, openErr := crypto.UnwrapPersonalKey(dek, username, wrapped)
		if openErr == nil {
			defer clear(private)
			derived, err := crypto.EscrowPublicKey(private)
			if err != nil {
				return true, nil, err
			}
			if !bytes.Equal(derived, stored) {
				// Someone changed the stored public key; the private key
				// is the account's own, so put its public key back. Any
				// attestation was of the swapped key, so it goes too.
				if _, err := q.ExecContext(ctx,
					`UPDATE personal_keys SET public_key = ?, attestation = x'', attested_escrow_key_id = '' WHERE username = ?`,
					derived, username); err != nil {
					return true, nil, fmt.Errorf("users: repair personal key: %w", err)
				}
				if err := recordAccountEvent(ctx, q, username, username, AccountPersonalKeyRepaired, ""); err != nil {
					return true, nil, err
				}
			}
			return true, derived, nil
		}
		// The stored private key does not open with this account's DEK,
		// so it is not the account's own: replace the pair below.
		if _, err := q.ExecContext(ctx, `DELETE FROM personal_keys WHERE username = ?`, username); err != nil {
			return true, nil, fmt.Errorf("users: replace personal key: %w", err)
		}
		if err := recordAccountEvent(ctx, q, username, username, AccountPersonalKeyRepaired, "the stored key could not be opened, so a new one was made"); err != nil {
			return true, nil, err
		}
	}

	private, public, err := crypto.GenerateEscrowKeyPair()
	if err != nil {
		return existed, nil, err
	}
	defer clear(private)
	wrapped, err = crypto.WrapPersonalKey(dek, username, private)
	if err != nil {
		return existed, nil, err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO personal_keys (username, public_key, wrapped_private_key, created_at) VALUES (?, ?, ?, ?)`,
		username, public, wrapped, nowStamp()); err != nil {
		return existed, nil, fmt.Errorf("users: store personal key: %w", err)
	}
	return existed, public, nil
}

// attestTx records the session escrow key's attestation of username's
// personal public key.
func attestTx(ctx context.Context, q accountWriter, session *EscrowSession, username string, personalPublic []byte) error {
	attestation, err := crypto.Attestation(session.private, username, personalPublic)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`UPDATE personal_keys SET attestation = ?, attested_escrow_key_id = ? WHERE username = ?`,
		attestation, session.id, username)
	return err
}

// EnrollSession enrolls an account at sign-in with its DEK, trusting the
// escrow key on disk the first time and its pin after that. It does
// nothing until an administrator has created the escrow key.
func (s *Store) EnrollSession(username string, dek []byte) error {
	return s.inWriteTx("escrow enrollment", func(ctx context.Context, q accountWriter) error {
		_, err := enrollTx(ctx, q, username, dek, nil)
		return err
	})
}

// openGrantTx opens admin's grant for the active escrow key with their DEK
// and checks the escrow key it holds against admin's pin, so a forged grant
// is never used. The caller must Close the session.
//
// It returns ErrNoEscrowGrant when admin has no grant, pin, or personal key
// for the active key. A grant that cannot be used (corrupt, forged, or
// sealed to a personal key admin no longer has) is deleted and recorded,
// and ErrEscrowMismatch returned: callers commit that, so admin's own
// enrollment is kept and another administrator can grant again.
func openGrantTx(ctx context.Context, q accountWriter, admin string, dek []byte) (*EscrowSession, error) {
	active, ok, err := activeEscrowKey(ctx, q)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoEscrowGrant
	}
	var grant, wrapped, pin []byte
	var pinKeyID string
	err = q.QueryRowContext(ctx, `SELECT sealed FROM escrow_grants WHERE admin_username = ? AND escrow_key_id = ?`, admin, active.id).Scan(&grant)
	if err == nil {
		err = q.QueryRowContext(ctx, `SELECT wrapped_private_key FROM personal_keys WHERE username = ?`, admin).Scan(&wrapped)
	}
	if err == nil {
		err = q.QueryRowContext(ctx, `SELECT escrow_key_id, pin FROM escrow_pins WHERE username = ?`, admin).Scan(&pinKeyID, &pin)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoEscrowGrant
	}
	if err != nil {
		return nil, fmt.Errorf("users: read escrow grant: %w", err)
	}

	unusable := func(detail string) (*EscrowSession, error) {
		if _, err := q.ExecContext(ctx, `DELETE FROM escrow_grants WHERE admin_username = ?`, admin); err != nil {
			return nil, fmt.Errorf("users: remove unusable escrow grant: %w", err)
		}
		if err := recordAccountEvent(ctx, q, admin, admin, AccountEscrowKeyMismatch, detail); err != nil {
			return nil, err
		}
		return nil, ErrEscrowMismatch
	}
	personal, err := crypto.UnwrapPersonalKey(dek, admin, wrapped)
	if err != nil {
		return unusable("this administrator's account key does not open, so their copy of the administrator key was removed")
	}
	defer clear(personal)
	private, err := crypto.OpenGrant(personal, active.id, admin, grant)
	if err != nil {
		return unusable("this administrator's copy of the administrator key does not open and was removed")
	}
	public, err := crypto.EscrowPublicKey(private)
	if err == nil && pinKeyID == active.id {
		ok, err = crypto.VerifyEscrowPin(dek, admin, active.id, public, pin)
	} else {
		ok = false
	}
	if err != nil || !ok {
		clear(private)
		return unusable("this administrator's copy of the administrator key is not the real one and was removed")
	}
	return &EscrowSession{id: active.id, public: public, private: private}, nil
}

// OpenEscrow opens admin's escrow key for one operation, such as creating
// an account that is enrolled in the same transaction. The caller must
// Close the session. It returns ErrNoEscrowGrant or ErrEscrowMismatch when
// there is no usable grant; a mismatch is recorded.
func (s *Store) OpenEscrow(admin string, dek []byte) (*EscrowSession, error) {
	var session *EscrowSession
	var openErr error
	err := s.inWriteTx("escrow open", func(ctx context.Context, q accountWriter) error {
		session, openErr = openGrantTx(ctx, q, admin, dek)
		if errors.Is(openErr, ErrEscrowMismatch) {
			return nil // commit the recorded mismatch
		}
		return openErr
	})
	if err != nil {
		session.Close()
		return nil, err
	}
	return session, openErr
}

// EnrollAdminSession runs at an administrator's sign-in. It enrolls the
// administrator and, holding their escrow key for this call only:
//
//   - creates the escrow key if none has ever existed, and grants it to
//     them;
//   - checks every personal key attested by the escrow key against its
//     attestation, recording any that fails;
//   - checks that every sealed DEK opens with the escrow key, and removes
//     one that does not, so its owner's next sign-in seals it again.
//
// It grants nothing to other administrators: that happens only when an
// administrator promotes or enables one (PromoteAdmin, EnableAccount). An
// administrator without a grant stays without one until then. A caller
// that is not an enabled administrator is only enrolled.
func (s *Store) EnrollAdminSession(admin string, dek []byte) error {
	return s.inWriteTx("administrator escrow", func(ctx context.Context, q accountWriter) error {
		if err := requireEnabledAdmin(ctx, q, admin); errors.Is(err, ErrNotAdmin) {
			_, err := enrollTx(ctx, q, admin, dek, nil)
			return err
		} else if err != nil {
			return err
		}

		var keys int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM escrow_keys`).Scan(&keys); err != nil {
			return fmt.Errorf("users: count escrow keys: %w", err)
		}
		if keys == 0 {
			return bootstrapTx(ctx, q, admin, dek)
		}
		if _, active, err := activeEscrowKey(ctx, q); err != nil {
			return err
		} else if !active {
			// Escrow keys are retired only by rotation, which makes a new
			// active key in the same step; none active means the file was
			// changed. Never create a second escrow key.
			return recordAccountEvent(ctx, q, admin, admin, AccountEscrowKeyMismatch, "system.db has no current administrator key")
		}

		if _, err := enrollTx(ctx, q, admin, dek, nil); err != nil {
			return err
		}
		session, err := openGrantTx(ctx, q, admin, dek)
		if errors.Is(err, ErrNoEscrowGrant) || errors.Is(err, ErrEscrowMismatch) {
			return nil // waits for a grant, or the mismatch is recorded
		}
		if err != nil {
			return err
		}
		defer session.Close()
		return adminDutiesTx(ctx, q, session)
	})
}

// bootstrapTx creates the machine's escrow key, enrolls admin against it,
// and grants it to admin.
func bootstrapTx(ctx context.Context, q accountWriter, admin string, dek []byte) error {
	id, err := crypto.NewEscrowKeyID()
	if err != nil {
		return err
	}
	private, public, err := crypto.GenerateEscrowKeyPair()
	if err != nil {
		return err
	}
	session := &EscrowSession{id: id, public: public, private: private}
	defer session.Close()
	if _, err := q.ExecContext(ctx,
		`INSERT INTO escrow_keys (id, kem_id, public_key, created_at) VALUES (?, ?, ?, ?)`,
		id, crypto.EscrowKEMID, public, nowStamp()); err != nil {
		return fmt.Errorf("users: store escrow key: %w", err)
	}
	personal, err := enrollTx(ctx, q, admin, dek, session)
	if err != nil {
		return err
	}
	return grantTx(ctx, q, session, admin, personal)
}

// grantTx seals the session's escrow key to admin's personal public key.
// The caller passes a public key it has authenticated (derived from the
// private key, or checked against its attestation), never one read from
// disk unchecked.
func grantTx(ctx context.Context, q accountWriter, session *EscrowSession, admin string, personalPublic []byte) error {
	if len(personalPublic) == 0 {
		return ErrNoPersonalKey
	}
	sealed, err := crypto.SealGrant(personalPublic, session.id, admin, session.private)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(admin_username) DO UPDATE SET escrow_key_id = excluded.escrow_key_id, sealed = excluded.sealed, granted_at = excluded.granted_at`,
		admin, session.id, sealed, nowStamp())
	if err != nil {
		return fmt.Errorf("users: store escrow grant: %w", err)
	}
	return nil
}

// adminDutiesTx is the administrator-session work listed on
// EnrollAdminSession.
func adminDutiesTx(ctx context.Context, q accountWriter, session *EscrowSession) error {
	type personalRow struct {
		username            string
		public, attestation []byte
	}
	rows, err := q.QueryContext(ctx,
		`SELECT username, public_key, attestation FROM personal_keys WHERE attested_escrow_key_id = ?`, session.id)
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
		ok, err := crypto.VerifyAttestation(session.private, r.username, r.public, r.attestation)
		if err != nil {
			return err
		}
		if !ok {
			if err := recordAccountEvent(ctx, q, r.username, r.username, AccountEscrowKeyMismatch, "the account key is not the one an administrator checked"); err != nil {
				return err
			}
		}
	}
	return checkSealedDEKsTx(ctx, q, session)
}

// checkSealedDEKsTx opens every DEK sealed to the session's escrow key and
// removes any that does not open, recording it. A DEK that does not open
// was sealed to a planted key the account trusted, so the account's pin of
// that key goes too: its next sign-in pins the escrow key again (recorded
// as re-enrollment) and seals its DEK. Each opened DEK is cleared at once;
// nothing is recorded as access because nothing is used.
func checkSealedDEKsTx(ctx context.Context, q accountWriter, session *EscrowSession) error {
	rows, err := q.QueryContext(ctx, `SELECT username, sealed FROM sealed_deks WHERE escrow_key_id = ?`, session.id)
	if err != nil {
		return fmt.Errorf("users: list sealed DEKs: %w", err)
	}
	var bad []string
	for rows.Next() {
		var username string
		var sealed []byte
		if err := rows.Scan(&username, &sealed); err != nil {
			_ = rows.Close()
			return err
		}
		dek, err := crypto.OpenDEK(session.private, session.id, username, sealed)
		if err != nil {
			bad = append(bad, username)
			continue
		}
		clear(dek)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, username := range bad {
		if _, err := q.ExecContext(ctx, `DELETE FROM sealed_deks WHERE username = ?`, username); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM escrow_pins WHERE username = ?`, username); err != nil {
			return err
		}
		if err := recordAccountEvent(ctx, q, username, username, AccountEscrowKeyMismatch, "the account's sealed key does not open with the administrator key; it is sealed again at the account's next sign-in"); err != nil {
			return err
		}
	}
	return nil
}

// removeGrantTx deletes admin's grant, at demotion and when disabled.
func removeGrantTx(ctx context.Context, q accountWriter, admin string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM escrow_grants WHERE admin_username = ?`, admin)
	return err
}

// grantOnActionTx grants the active escrow key to username, an
// administrator, inside actor's promotion or enabling of them. actor's own
// grant supplies the escrow key. username's personal key must pass its
// attestation by that key; one this key never attested is trusted now, on
// first use, and the event is recorded so the Admin panel shows it.
//
// It grants nothing, without error, when no escrow key exists or actor
// holds no usable grant: the change goes ahead and another administrator
// can grant later by promoting again. With requireKey set, an account with
// no personal key yet returns ErrNoPersonalKey.
func grantOnActionTx(ctx context.Context, q accountWriter, actor string, actorDEK []byte, username string, requireKey bool) error {
	active, ok, err := activeEscrowKey(ctx, q)
	if err != nil || !ok {
		return err
	}
	var public, attestation []byte
	var attestedKeyID string
	err = q.QueryRowContext(ctx,
		`SELECT public_key, attestation, attested_escrow_key_id FROM personal_keys WHERE username = ?`,
		username).Scan(&public, &attestation, &attestedKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		if requireKey {
			return ErrNoPersonalKey
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("users: read personal key: %w", err)
	}
	var granted int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM escrow_grants WHERE admin_username = ? AND escrow_key_id = ?`,
		username, active.id).Scan(&granted); err != nil {
		return fmt.Errorf("users: read escrow grant: %w", err)
	}
	if granted > 0 {
		return nil
	}

	session, err := openGrantTx(ctx, q, actor, actorDEK)
	if errors.Is(err, ErrNoEscrowGrant) || errors.Is(err, ErrEscrowMismatch) {
		return nil
	}
	if err != nil {
		return err
	}
	defer session.Close()
	return attestAndGrantTx(ctx, q, session, actor, username, public, attestation, attestedKeyID)
}

// attestAndGrantTx grants session's escrow key to username after checking
// their personal key against its attestation by that key; a key this
// escrow key never attested is trusted now, on first use, and recorded.
func attestAndGrantTx(ctx context.Context, q accountWriter, session *EscrowSession, actor, username string, public, attestation []byte, attestedKeyID string) error {
	if attestedKeyID == session.id {
		ok, err := crypto.VerifyAttestation(session.private, username, public, attestation)
		if err != nil {
			return err
		}
		if !ok {
			return ErrPersonalKeyNotAttested
		}
	} else {
		if err := attestTx(ctx, q, session, username, public); err != nil {
			return err
		}
		if err := recordAccountEvent(ctx, q, actor, username, AccountPersonalKeyTrusted, ""); err != nil {
			return err
		}
	}
	return grantTx(ctx, q, session, username, public)
}

// requirePersonalKeyTx returns ErrNoPersonalKey unless username has a
// personal key.
func requirePersonalKeyTx(ctx context.Context, q accountWriter, username string) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM personal_keys WHERE username = ?`, username).Scan(&n); err != nil {
		return fmt.Errorf("users: read personal key: %w", err)
	}
	if n == 0 {
		return ErrNoPersonalKey
	}
	return nil
}

// PromoteAdmin makes username an administrator on behalf of actor, an
// enabled administrator whose DEK is actorDEK, and grants them the escrow
// key in the same transaction (grantOnActionTx); a disabled account is
// granted when it is enabled. Promoting an account that is already an
// administrator only adds a missing grant. An account that
// has not signed in since enrollment began returns ErrNoPersonalKey, and
// one whose personal key fails its attestation returns
// ErrPersonalKeyNotAttested; neither is promoted, and the failed
// attestation is recorded.
func (s *Store) PromoteAdmin(actor string, actorDEK []byte, username string) error {
	err := s.inWriteTx("role change", func(ctx context.Context, q accountWriter) error {
		if err := requireEnabledAdmin(ctx, q, actor); err != nil {
			return err
		}
		isAdmin, disabled, err := accountRole(ctx, q, username)
		if err != nil {
			return err
		}
		if disabled {
			// A disabled administrator holds no grant; EnableAccount
			// grants. The account must still have signed in once.
			if err := requirePersonalKeyTx(ctx, q, username); err != nil {
				return err
			}
		} else if err := grantOnActionTx(ctx, q, actor, actorDEK, username, true); err != nil {
			return err
		}
		if isAdmin {
			return nil
		}
		if _, err := q.ExecContext(ctx, `UPDATE users SET is_admin = 1 WHERE username = ?`, username); err != nil {
			return err
		}
		return recordAccountEvent(ctx, q, actor, username, AccountPromoted, "")
	})
	if errors.Is(err, ErrPersonalKeyNotAttested) {
		s.recordAttestationFailure(actor, username)
	}
	return err
}

// EnableAccount enables username on behalf of actor, as SetDisabled does,
// and grants an administrator the escrow key in the same transaction. A
// disabled administrator cannot sign in to make a personal key, so one
// without a key is enabled without a grant rather than refused. Called for
// an administrator who is already enabled, it only adds a missing grant.
func (s *Store) EnableAccount(actor string, actorDEK []byte, username string) error {
	err := s.inWriteTx("account status change", func(ctx context.Context, q accountWriter) error {
		if err := setDisabledTx(ctx, q, actor, username, false); err != nil {
			return err
		}
		isAdmin, _, err := accountRole(ctx, q, username)
		if err != nil || !isAdmin {
			return err
		}
		return grantOnActionTx(ctx, q, actor, actorDEK, username, false)
	})
	if errors.Is(err, ErrPersonalKeyNotAttested) {
		s.recordAttestationFailure(actor, username)
	}
	return err
}

// recordAttestationFailure records a refused promotion or enabling in its
// own transaction, since the refused one rolled back.
func (s *Store) recordAttestationFailure(actor, username string) {
	err := s.inWriteTx("escrow event", func(ctx context.Context, q accountWriter) error {
		return recordAccountEvent(ctx, q, actor, username, AccountEscrowKeyMismatch, "the account key is not the one an administrator checked")
	})
	if err != nil {
		log.Printf("users: record attestation failure for %s: %v", username, err)
	}
}

// ErrTargetNotAdmin is returned by GrantKey when the account is not an
// enabled administrator.
var ErrTargetNotAdmin = errors.New("users: account is not an enabled administrator")

// GrantKey gives username, an enabled administrator without a grant, the
// active escrow key, on behalf of actor, whose DEK is actorDEK. Unlike
// promotion, it fails rather than doing nothing when it cannot grant:
// ErrNoEscrowGrant or ErrEscrowMismatch when actor holds no usable grant
// (a mismatch is recorded), ErrNoPersonalKey when username has not signed
// in since enrollment began, and ErrPersonalKeyNotAttested (recorded) when
// their personal key fails its attestation. An administrator who already
// holds a grant is left alone.
func (s *Store) GrantKey(actor string, actorDEK []byte, username string) error {
	var refused error
	err := s.inWriteTx("escrow grant", func(ctx context.Context, q accountWriter) error {
		if err := requireEnabledAdmin(ctx, q, actor); err != nil {
			return err
		}
		if err := requireEnabledAdmin(ctx, q, username); errors.Is(err, ErrNotAdmin) {
			return ErrTargetNotAdmin
		} else if err != nil {
			return err
		}
		// The caller's own key first: without it nothing else matters.
		session, err := openGrantTx(ctx, q, actor, actorDEK)
		if errors.Is(err, ErrEscrowMismatch) {
			refused = err
			return nil // commit the recorded mismatch
		}
		if err != nil {
			return err
		}
		defer session.Close()
		var public, attestation []byte
		var attestedKeyID string
		err = q.QueryRowContext(ctx,
			`SELECT public_key, attestation, attested_escrow_key_id FROM personal_keys WHERE username = ?`,
			username).Scan(&public, &attestation, &attestedKeyID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoPersonalKey
		}
		if err != nil {
			return fmt.Errorf("users: read personal key: %w", err)
		}
		var granted int
		if err := q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM escrow_grants WHERE admin_username = ? AND escrow_key_id = ?`,
			username, session.id).Scan(&granted); err != nil {
			return fmt.Errorf("users: read escrow grant: %w", err)
		}
		if granted > 0 {
			return nil
		}
		return attestAndGrantTx(ctx, q, session, actor, username, public, attestation, attestedKeyID)
	})
	if errors.Is(err, ErrPersonalKeyNotAttested) {
		s.recordAttestationFailure(actor, username)
	}
	if err != nil {
		return err
	}
	return refused
}

// AdminsWithoutGrant lists the enabled administrators who hold no grant
// for the active escrow key, so another administrator can give them one.
// With no escrow key yet it lists nobody.
func (s *Store) AdminsWithoutGrant() ([]string, error) {
	rows, err := s.conn.Query(
		`SELECT u.username FROM users u, escrow_keys k
		 WHERE k.retired_at = '' AND u.is_admin = 1 AND u.disabled = 0
		   AND NOT EXISTS (SELECT 1 FROM escrow_grants g WHERE g.admin_username = u.username AND g.escrow_key_id = k.id)
		 ORDER BY u.username`)
	if err != nil {
		return nil, fmt.Errorf("users: list administrators without a grant: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
