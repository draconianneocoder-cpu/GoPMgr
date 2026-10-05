// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"gopmgr/internal/crypto"
)

// escrowStore returns a store whose first administrator, alice, has
// created the escrow key at her first session, and alice's DEK.
func escrowStore(t *testing.T) (*Store, []byte) {
	t.Helper()
	store := openTestStore(t)
	return store, firstAdmin(t, store)
}

func firstAdmin(t *testing.T, store *Store) []byte {
	t.Helper()
	_, dek, err := store.CreateAccountWithKey("", "alice", "Alice", statusPassword, true, nil)
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	t.Cleanup(func() { clear(dek) })
	if err := store.EnrollAdminSession("alice", dek); err != nil {
		t.Fatalf("alice's first session: %v", err)
	}
	return dek
}

// addUser creates a standard account as alice would, with or without her
// escrow session, and returns its DEK.
func addUser(t *testing.T, store *Store, username string, session *EscrowSession) []byte {
	t.Helper()
	_, dek, err := store.CreateAccountWithKey("alice", username, username, statusPassword, false, session)
	if err != nil {
		t.Fatalf("create %s: %v", username, err)
	}
	t.Cleanup(func() { clear(dek) })
	return dek
}

func openEscrow(t *testing.T, store *Store, admin string, dek []byte) *EscrowSession {
	t.Helper()
	session, err := store.OpenEscrow(admin, dek)
	if err != nil {
		t.Fatalf("OpenEscrow(%s): %v", admin, err)
	}
	t.Cleanup(session.Close)
	return session
}

// openSealedDEK opens username's sealed DEK with the session's escrow key.
func openSealedDEK(store *Store, session *EscrowSession, username string) ([]byte, error) {
	var keyID string
	var sealed []byte
	err := store.conn.QueryRow(`SELECT escrow_key_id, sealed FROM sealed_deks WHERE username = ?`, username).Scan(&keyID, &sealed)
	if err != nil {
		return nil, err
	}
	if keyID != session.id {
		return nil, fmt.Errorf("sealed to escrow key %s, want %s", keyID, session.id)
	}
	return crypto.OpenDEK(session.private, session.id, username, sealed)
}

func mustExec(t *testing.T, store *Store, query string, args ...any) {
	t.Helper()
	if _, err := store.conn.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func attackerKeyPair(t *testing.T) (priv, pub []byte) {
	t.Helper()
	priv, pub, err := crypto.GenerateEscrowKeyPair()
	if err != nil {
		t.Fatalf("GenerateEscrowKeyPair: %v", err)
	}
	return priv, pub
}

func requireEvent(t *testing.T, store *Store, want string) {
	t.Helper()
	if got := eventActions(t, store); !slices.Contains(got, want) {
		t.Fatalf("events = %q, want one %q", got, want)
	}
}

func TestFirstAdministratorCreatesTheEscrowKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)

	var stored []byte
	if err := store.conn.QueryRow(`SELECT public_key FROM escrow_keys WHERE id = ?`, session.id).Scan(&stored); err != nil {
		t.Fatalf("read escrow key: %v", err)
	}
	if !bytes.Equal(stored, session.public) {
		t.Fatal("the stored escrow public key is not the one alice's grant opens to")
	}
	if got, err := openSealedDEK(store, session, "alice"); err != nil || !bytes.Equal(got, aliceDEK) {
		t.Fatalf("alice's sealed DEK opens to %x, %v; want her DEK", got, err)
	}

	if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
		t.Fatalf("alice's second session: %v", err)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
		t.Fatalf("%d escrow keys after a second session, want 1", n)
	}
}

// Every place a DEK is created leaves it sealed, and the sealed DEK is the
// one the account's password unlocks (the key its encrypted projects use).
func TestEveryDEKCreationSiteSealsTheAccountsDEK(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)

	for name, create := range map[string]func(username string) []byte{
		"created by an administrator holding the escrow key": func(username string) []byte {
			return addUser(t, store, username, session)
		},
		"created by an administrator without it": func(username string) []byte {
			return addUser(t, store, username, nil)
		},
		"older than ADR-001, DEK made at sign-in": func(username string) []byte {
			if _, err := store.CreateAccount(username, username, statusPassword, false); err != nil {
				t.Fatalf("create %s: %v", username, err)
			}
			dek, err := store.UnlockDEK(username, statusPassword)
			if err != nil {
				t.Fatalf("UnlockDEK(%s): %v", username, err)
			}
			return dek
		},
		"enrolled at sign-in after escrow began": func(username string) []byte {
			dek := addUser(t, store, username, nil)
			for _, table := range []string{"personal_keys", "escrow_pins", "sealed_deks"} {
				mustExec(t, store, `DELETE FROM `+table+` WHERE username = ?`, username)
			}
			if err := store.EnrollSession(username, dek); err != nil {
				t.Fatalf("EnrollSession(%s): %v", username, err)
			}
			return dek
		},
	} {
		username := fmt.Sprintf("user%d", countRows(t, store, "users", "1 = 1"))
		dek := create(username)
		unlocked, err := store.UnlockDEK(username, statusPassword)
		if err != nil || !bytes.Equal(unlocked, dek) {
			t.Fatalf("%s: UnlockDEK = %v; want the DEK made at creation, one DEK per account", name, err)
		}
		if got, err := openSealedDEK(store, session, username); err != nil || !bytes.Equal(got, dek) {
			t.Errorf("%s: sealed DEK opens to %x, %v; want the account's DEK", name, got, err)
		}
	}
}

func TestAccountCreatedWithTheEscrowKeyIsAttested(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	addUser(t, store, "bob", session)

	var public, attestation []byte
	var keyID string
	if err := store.conn.QueryRow(`SELECT public_key, attestation, attested_escrow_key_id FROM personal_keys WHERE username = 'bob'`).Scan(&public, &attestation, &keyID); err != nil {
		t.Fatalf("read bob's personal key: %v", err)
	}
	if ok, err := crypto.VerifyAttestation(session.private, "bob", public, attestation); keyID != session.id || err != nil || !ok {
		t.Fatalf("bob's attestation = %v, %v (key %q); want it to verify with %q", ok, err, keyID, session.id)
	}
}

// is_admin and the personal key are plain columns anyone who can write
// system.db can change. An administrator's sign-in must not hand the
// escrow key to whoever they name.
func TestSignInNeverGrantsTheEscrowKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	addUser(t, store, "erin", nil)

	// bob makes himself an administrator.
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'bob'`)
	// erin is made an administrator with an attacker's personal key and
	// no attestation.
	attackerPriv, attackerPub := attackerKeyPair(t)
	wrapped, err := crypto.WrapPersonalKey(make([]byte, crypto.DEKSize), "erin", attackerPriv)
	if err != nil {
		t.Fatalf("WrapPersonalKey: %v", err)
	}
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'erin'`)
	mustExec(t, store, `UPDATE personal_keys SET public_key = ?, wrapped_private_key = ?, attestation = x'', attested_escrow_key_id = '' WHERE username = 'erin'`, attackerPub, wrapped)

	if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
		t.Fatalf("alice's session: %v", err)
	}
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username IN ('bob', 'erin')"); n != 0 {
		t.Fatalf("%d grants for accounts made administrators in system.db, want 0", n)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
		t.Fatalf("%d escrow keys, want 1", n)
	}
}

// Under ADR-005 a promoted administrator is a subordinate and never holds
// the administrator key, whether promoted, created as an administrator, or
// signing in as one. (Intentional change from phase 1, where promotion
// granted the key.)
func TestPromotedAdministratorsHoldNoKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	bobDEK := addUser(t, store, "bob", nil)

	if err := store.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("PromoteAdmin(bob): %v", err)
	}
	requireEvent(t, store, "alice promoted bob")
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	if _, err := store.OpenEscrow("bob", bobDEK); !errors.Is(err, ErrNoEscrowGrant) {
		t.Fatalf("a subordinate's OpenEscrow = %v, want ErrNoEscrowGrant", err)
	}
	_, frankDEK, err := store.CreateAccountWithKey("alice", "frank", "Frank", statusPassword, true, session)
	if err != nil {
		t.Fatalf("create administrator frank: %v", err)
	}
	clear(frankDEK)
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'alice'"); n != 0 {
		t.Fatalf("%d subordinate administrators hold the key, want none", n)
	}

	// Promoting an administrator again changes nothing.
	before := len(eventActions(t, store))
	if err := store.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("PromoteAdmin(bob) again: %v", err)
	}
	if after := len(eventActions(t, store)); after != before {
		t.Fatalf("promoting an administrator again recorded %d events", after-before)
	}
}

// An account is promoted only once it has signed in since enrollment
// began, disabled or not, so a later hand-over has a key to grant to.
func TestPromotionNeedsAnAccountThatHasSignedIn(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	if _, err := store.CreateAccount("carol", "Carol", statusPassword, false); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	if err := store.PromoteAdmin("alice", aliceDEK, "carol"); !errors.Is(err, ErrNoPersonalKey) {
		t.Fatalf("PromoteAdmin(carol) = %v, want ErrNoPersonalKey", err)
	}
	if err := store.SetDisabled("alice", "carol", true); err != nil {
		t.Fatalf("SetDisabled(carol): %v", err)
	}
	if err := store.PromoteAdmin("alice", aliceDEK, "carol"); !errors.Is(err, ErrNoPersonalKey) {
		t.Fatalf("PromoteAdmin(disabled carol) = %v, want ErrNoPersonalKey", err)
	}
	if n := countRows(t, store, "users", "username = 'carol' AND is_admin = 1"); n != 0 {
		t.Fatal("carol was promoted")
	}
}

// A grant left on a subordinate from before ADR-005 is removed when the
// super administrator demotes or disables them, or at their own sign-in.
func TestLeftoverSubordinateGrantsAreRemoved(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	if err := store.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("PromoteAdmin(bob): %v", err)
	}
	plant := func() {
		t.Helper()
		mustExec(t, store, `INSERT OR REPLACE INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at)
			SELECT 'bob', id, x'01', '2026-10-05T00:00:00Z' FROM escrow_keys WHERE retired_at = ''`)
	}
	grants := func() int { return countRows(t, store, "escrow_grants", "admin_username = 'bob'") }

	plant()
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil || grants() != 0 {
		t.Fatalf("bob's session = %v with %d grants, want the leftover removed", err, grants())
	}
	requireEvent(t, store, "bob escrow_key_mismatch bob")
	plant()
	if err := store.SetAdmin("alice", "bob", false); err != nil || grants() != 0 {
		t.Fatalf("demoting bob = %v with %d grants, want 0", err, grants())
	}
	if err := store.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("PromoteAdmin(bob) again: %v", err)
	}
	plant()
	if err := store.SetDisabled("alice", "bob", true); err != nil || grants() != 0 {
		t.Fatalf("disabling bob = %v with %d grants, want 0", err, grants())
	}
	if err := store.SetDisabled("alice", "bob", false); err != nil || grants() != 0 {
		t.Fatalf("enabling bob = %v with %d grants, want none given", err, grants())
	}
}

func TestSwappedEscrowPublicKeySealsNothing(t *testing.T) {
	store, _ := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	_, attackerPub := attackerKeyPair(t)
	mustExec(t, store, `DELETE FROM sealed_deks WHERE username = 'bob'`)
	mustExec(t, store, `UPDATE escrow_keys SET public_key = ?`, attackerPub)

	if err := store.EnrollSession("bob", bobDEK); err != nil {
		t.Fatalf("EnrollSession(bob): %v", err)
	}
	if n := countRows(t, store, "sealed_deks", "username = 'bob'"); n != 0 {
		t.Fatal("bob's DEK was sealed to an escrow key his pin does not trust")
	}
	requireEvent(t, store, "bob escrow_key_mismatch bob")
}

// Deleting a pin returns the account to trust on first use. The
// re-enrollment is recorded, and an administrator session finds a DEK
// sealed to a planted key (or moved from another account) and removes it,
// so the next sign-in seals to the real key.
func TestAdministratorSessionFindsDEKsSealedToAPlantedKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	addUser(t, store, "carol", nil)
	var realPub []byte
	if err := store.conn.QueryRow(`SELECT public_key FROM escrow_keys`).Scan(&realPub); err != nil {
		t.Fatalf("read escrow key: %v", err)
	}
	_, attackerPub := attackerKeyPair(t)

	mustExec(t, store, `DELETE FROM escrow_pins WHERE username = 'bob'`)
	mustExec(t, store, `DELETE FROM sealed_deks WHERE username = 'bob'`)
	mustExec(t, store, `UPDATE escrow_keys SET public_key = ?`, attackerPub)
	if err := store.EnrollSession("bob", bobDEK); err != nil {
		t.Fatalf("EnrollSession(bob): %v", err)
	}
	requireEvent(t, store, "bob escrow_reenrolled bob")
	mustExec(t, store, `UPDATE escrow_keys SET public_key = ?`, realPub)
	// alice's sealed DEK copied onto carol's row.
	mustExec(t, store, `UPDATE sealed_deks SET sealed = (SELECT sealed FROM sealed_deks WHERE username = 'alice') WHERE username = 'carol'`)

	if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
		t.Fatalf("alice's session: %v", err)
	}
	for _, username := range []string{"bob", "carol"} {
		if n := countRows(t, store, "sealed_deks", "username = ?", username); n != 0 {
			t.Errorf("%s's sealed DEK does not open and was kept", username)
		}
		requireEvent(t, store, username+" escrow_key_mismatch "+username)
	}

	if err := store.EnrollSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's next sign-in: %v", err)
	}
	session := openEscrow(t, store, "alice", aliceDEK)
	if got, err := openSealedDEK(store, session, "bob"); err != nil || !bytes.Equal(got, bobDEK) {
		t.Fatalf("after the next sign-in bob's sealed DEK opens to %x, %v; want his DEK", got, err)
	}
}

func TestForgedGrantFailsThePinCheck(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	var alicePub []byte
	var keyID string
	if err := store.conn.QueryRow(`SELECT public_key FROM personal_keys WHERE username = 'alice'`).Scan(&alicePub); err != nil {
		t.Fatalf("read alice's personal key: %v", err)
	}
	if err := store.conn.QueryRow(`SELECT id FROM escrow_keys`).Scan(&keyID); err != nil {
		t.Fatalf("read escrow key: %v", err)
	}
	attackerPriv, _ := attackerKeyPair(t)
	forged, err := crypto.SealGrant(alicePub, keyID, "alice", attackerPriv)
	if err != nil {
		t.Fatalf("SealGrant: %v", err)
	}
	mustExec(t, store, `UPDATE escrow_grants SET sealed = ? WHERE admin_username = 'alice'`, forged)

	if session, err := store.OpenEscrow("alice", aliceDEK); !errors.Is(err, ErrEscrowMismatch) {
		session.Close()
		t.Fatalf("OpenEscrow with a forged grant = %v, want ErrEscrowMismatch", err)
	}
	requireEvent(t, store, "alice escrow_key_mismatch alice")
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("the forged grant was kept")
	}
}

func TestSwappedPersonalKeyIsRepairedAtSignIn(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	bobDEK := addUser(t, store, "bob", session)
	readBob := func() (public, wrapped []byte, keyID string) {
		t.Helper()
		if err := store.conn.QueryRow(`SELECT public_key, wrapped_private_key, attested_escrow_key_id FROM personal_keys WHERE username = 'bob'`).Scan(&public, &wrapped, &keyID); err != nil {
			t.Fatalf("read bob's personal key: %v", err)
		}
		return public, wrapped, keyID
	}
	original, _, _ := readBob()
	_, attackerPub := attackerKeyPair(t)

	mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'bob'`, attackerPub)
	if err := store.EnrollSession("bob", bobDEK); err != nil {
		t.Fatalf("EnrollSession(bob): %v", err)
	}
	public, _, keyID := readBob()
	if !bytes.Equal(public, original) || keyID != "" {
		t.Fatalf("after a swapped public key: public restored = %v, attested by %q; want restored and unattested", bytes.Equal(public, original), keyID)
	}
	requireEvent(t, store, "bob personal_key_repaired bob")

	mustExec(t, store, `UPDATE personal_keys SET wrapped_private_key = x'00112233' WHERE username = 'bob'`)
	if err := store.EnrollSession("bob", bobDEK); err != nil {
		t.Fatalf("EnrollSession(bob) with a replaced private key: %v", err)
	}
	public, wrapped, _ := readBob()
	private, err := crypto.UnwrapPersonalKey(bobDEK, "bob", wrapped)
	if err != nil {
		t.Fatalf("bob's new personal key does not open with his DEK: %v", err)
	}
	if derived, err := crypto.EscrowPublicKey(private); err != nil || !bytes.Equal(derived, public) || bytes.Equal(public, original) {
		t.Fatal("bob's personal key was not replaced by a new, consistent pair")
	}
}

func TestAnEscrowKeyIsNeverCreatedWhileOneExists(t *testing.T) {
	t.Run("planted before the first session", func(t *testing.T) {
		store := openTestStore(t)
		_, plantedPub := attackerKeyPair(t)
		mustExec(t, store, `INSERT INTO escrow_keys (id, kem_id, public_key, created_at) VALUES ('planted', ?, ?, '2026-10-02T00:00:00Z')`, crypto.EscrowKEMID, plantedPub)
		firstAdmin(t, store)
		if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
			t.Fatalf("%d escrow keys, want only the planted one", n)
		}
		if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
			t.Fatal("a grant was made for an escrow key nobody holds")
		}
	})
	t.Run("the active key retired", func(t *testing.T) {
		store, aliceDEK := escrowStore(t)
		mustExec(t, store, `UPDATE escrow_keys SET retired_at = '2026-10-02T00:00:00Z'`)
		if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
			t.Fatalf("alice's session: %v", err)
		}
		if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
			t.Fatalf("%d escrow keys, want 1", n)
		}
		requireEvent(t, store, "alice escrow_key_mismatch alice")
	})
}

func TestAdministratorSessionFlagsAFailingAttestation(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	addUser(t, store, "bob", session)
	_, attackerPub := attackerKeyPair(t)
	mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'bob'`, attackerPub)

	if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
		t.Fatalf("alice's session: %v", err)
	}
	requireEvent(t, store, "bob escrow_key_mismatch bob")
}

// Two GoPMgr processes sharing one data root: their first administrator
// sessions create one escrow key, and enrolling one account twice at
// once leaves one pin, one personal key, and one sealed DEK.
func TestTwoProcessesEnrollingAtOnce(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	stores := make([]*Store, 2)
	for i := range stores {
		store, err := Open(root)
		if err != nil {
			t.Fatalf("Open %d: %v", i, err)
		}
		t.Cleanup(func() { _ = store.Close() })
		stores[i] = store
	}
	_, aliceDEK, err := stores[0].CreateAccountWithKey("", "alice", "Alice", statusPassword, true, nil)
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bobDEK := addUser(t, stores[0], "bob", nil) // no escrow key yet: not enrolled

	race := func(fn func(store *Store) error) {
		t.Helper()
		start := make(chan struct{})
		errs := make([]error, len(stores))
		var wg sync.WaitGroup
		for i, store := range stores {
			wg.Go(func() {
				<-start
				errs[i] = fn(store)
			})
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("process %d: %v", i, err)
			}
		}
	}
	race(func(store *Store) error { return store.EnrollAdminSession("alice", aliceDEK) })
	race(func(store *Store) error { return store.EnrollSession("bob", bobDEK) })

	store := stores[0]
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 1 {
		t.Fatalf("%d escrow keys, want 1", n)
	}
	for _, table := range []string{"personal_keys", "escrow_pins", "sealed_deks"} {
		if n := countRows(t, store, table, "username = 'bob'"); n != 1 {
			t.Errorf("%s has %d rows for bob, want 1", table, n)
		}
	}
	// Both processes' sessions ran; alice was made super administrator once.
	if got := eventActions(t, store); fmt.Sprint(got) != fmt.Sprint([]string{"alice created alice", "alice created bob", "alice super_admin_assigned alice"}) {
		t.Errorf("events = %q; want the two creations and one super administrator assignment", got)
	}
	session := openEscrow(t, store, "alice", aliceDEK)
	if got, err := openSealedDEK(store, session, "bob"); err != nil || !bytes.Equal(got, bobDEK) {
		t.Fatalf("bob's sealed DEK opens to %x, %v; want his DEK", got, err)
	}
}

// Enrollment is best effort: when it fails, the account and its DEK are
// still stored, and none of the enrollment's rows are left behind.
func TestFailedEnrollmentKeepsTheAccountAndLeavesNoEscrowRows(t *testing.T) {
	store, _ := escrowStore(t)
	// An escrow public key that cannot be sealed to.
	mustExec(t, store, `UPDATE escrow_keys SET public_key = x'01'`)

	created := addUser(t, store, "bob", nil)
	if _, err := store.CreateAccount("carol", "Carol", statusPassword, false); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	legacy, err := store.UnlockDEK("carol", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(carol) with enrollment failing: %v", err)
	}
	for username, dek := range map[string][]byte{"bob": created, "carol": legacy} {
		if unlocked, err := store.UnlockDEK(username, statusPassword); err != nil || !bytes.Equal(unlocked, dek) {
			t.Errorf("%s: UnlockDEK = %v; want the DEK stored despite the failed enrollment", username, err)
		}
		for _, table := range []string{"personal_keys", "escrow_pins", "sealed_deks"} {
			if n := countRows(t, store, table, "username = ?", username); n != 0 {
				t.Errorf("%s: a failed enrollment left %d row(s) in %s", username, n, table)
			}
		}
	}
}

// The super administrator's grant that cannot be used (corrupt, or sealed
// to a personal key they no longer have) is removed and recorded, and their
// own enrollment and key repair still commit.
func TestUnusableSuperGrantIsRemovedAndRecorded(t *testing.T) {
	for name, damage := range map[string]string{
		"corrupt grant":                 `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`,
		"personal private key replaced": `UPDATE personal_keys SET wrapped_private_key = x'00112233' WHERE username = 'alice'`,
	} {
		t.Run(name, func(t *testing.T) {
			store, aliceDEK := escrowStore(t)
			mustExec(t, store, damage)
			if err := store.EnrollAdminSession("alice", aliceDEK); err != nil {
				t.Fatalf("alice's session with an unusable grant: %v", err)
			}
			requireEvent(t, store, "alice escrow_key_mismatch alice")
			if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
				t.Fatal("the unusable grant was kept")
			}
			var wrapped []byte
			if err := store.conn.QueryRow(`SELECT wrapped_private_key FROM personal_keys WHERE username = 'alice'`).Scan(&wrapped); err != nil {
				t.Fatalf("read alice's personal key: %v", err)
			}
			if _, err := crypto.UnwrapPersonalKey(aliceDEK, "alice", wrapped); err != nil {
				t.Fatalf("alice's personal key does not open with her DEK after her session: %v", err)
			}
		})
	}
}

// A super administrator whose stored personal key no longer opens cannot
// open data: the grant sealed to it is removed and recorded.
func TestSuperWhosePersonalKeyDoesNotOpenLosesTheGrant(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	mustExec(t, store, `UPDATE personal_keys SET wrapped_private_key = x'00112233' WHERE username = 'alice'`)

	if dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "checking"); !errors.Is(err, ErrEscrowMismatch) || dek != nil {
		t.Fatalf("OpenUserForAdmin = %d bytes, %v; want ErrEscrowMismatch", len(dek), err)
	}
	requireEvent(t, store, "alice escrow_key_mismatch alice")
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("a grant sealed to a personal key that does not open was kept")
	}
}
