// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"gopmgr/internal/crypto"
)

// activeKeyID returns the active escrow key's ID, failing unless exactly one
// key is active.
func activeKeyID(t *testing.T, store *Store) string {
	t.Helper()
	if n := countRows(t, store, "escrow_keys", "retired_at = ''"); n != 1 {
		t.Fatalf("%d active escrow keys, want exactly 1", n)
	}
	var id string
	if err := store.conn.QueryRow(`SELECT id FROM escrow_keys WHERE retired_at = ''`).Scan(&id); err != nil {
		t.Fatalf("read active escrow key: %v", err)
	}
	return id
}

func rotationPending(t *testing.T, store *Store) bool {
	t.Helper()
	return countRows(t, store, "escrow_rotation_pending", "1 = 1") == 1
}

func TestReplacingTheKeyReSealsEveryAccount(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	old := openEscrow(t, store, "alice", aliceDEK)
	oldID, oldPrivate := old.id, append([]byte(nil), old.private...)
	deks := map[string][]byte{"alice": aliceDEK, "bob": addUser(t, store, "bob", old), "carol": addUser(t, store, "carol", nil)}

	res, err := store.ReplaceEscrowKey("alice", aliceDEK, "")
	if err != nil || res != (RotationResult{Resealed: 3}) {
		t.Fatalf("ReplaceEscrowKey = %+v, %v; want 3 accounts re-sealed", res, err)
	}
	newID := activeKeyID(t, store)
	if newID == oldID || countRows(t, store, "escrow_keys", "id = '"+oldID+"' AND retired_at <> ''") != 1 {
		t.Fatal("the old key is not retired in favor of a new one")
	}
	next := openEscrow(t, store, "alice", aliceDEK)
	for name, dek := range deks {
		got, err := openSealedDEK(store, next, name)
		if err != nil || !bytes.Equal(got, dek) {
			t.Fatalf("%s's DEK under the new key = %x, %v; want it", name, got, err)
		}
	}
	// The old private key opens none of the DEKs now on disk.
	rows, err := store.conn.Query(`SELECT username, escrow_key_id, sealed FROM sealed_deks`)
	if err != nil {
		t.Fatalf("list sealed DEKs: %v", err)
	}
	for rows.Next() {
		var name, keyID string
		var sealed []byte
		if err := rows.Scan(&name, &keyID, &sealed); err != nil {
			t.Fatal(err)
		}
		if keyID != newID {
			t.Errorf("%s's DEK is sealed to %s, want the new key", name, keyID)
		}
		if _, err := crypto.OpenDEK(oldPrivate, oldID, name, sealed); err == nil {
			t.Errorf("the old key opens %s's sealed DEK", name)
		}
	}
	_ = rows.Close()
	// The attestations alice and bob had carry over; carol never had one.
	if n := countRows(t, store, "personal_keys", "attested_escrow_key_id = '"+newID+"'"); n != 2 {
		t.Fatalf("%d personal keys attested by the new key, want alice's and bob's", n)
	}
	requireEvent(t, store, "alice escrow_rotated alice")
	if e := latestEvent(t, store); e.Detail != "replaced by the super administrator; 3 accounts sealed to the new key" {
		t.Fatalf("rotation detail = %q", e.Detail)
	}

	// Each account's next sign-in trusts the new key: nothing is flagged
	// and nothing is sealed again.
	before := len(eventActions(t, store))
	for name, dek := range deks {
		if err := store.EnrollSession(name, dek); err != nil {
			t.Fatalf("%s's sign-in: %v", name, err)
		}
	}
	if after := len(eventActions(t, store)); after != before {
		t.Fatalf("sign-ins after the replacement recorded %v", eventActions(t, store)[before:])
	}
	// An account enrolled afterwards is sealed to the new key only.
	addUser(t, store, "dave", nil)
	var sealed []byte
	if err := store.conn.QueryRow(`SELECT sealed FROM sealed_deks WHERE username = 'dave'`).Scan(&sealed); err != nil {
		t.Fatalf("read dave's sealed DEK: %v", err)
	}
	if _, err := crypto.OpenDEK(oldPrivate, oldID, "dave", sealed); err == nil {
		t.Fatal("the old key opens an account enrolled after the replacement")
	}
}

// The new key goes to the actor and the accounts they name, never to an
// account read from system.db.
func TestReplacingTheKeyGrantsOnlyTheActorAndTheNamedStandby(t *testing.T) {
	store, aliceDEK, bobDEK, daveDEK := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	// erin: a subordinate whose personal key carries a valid attestation,
	// with a grant row copied in by someone editing system.db.
	addUser(t, store, "erin", openEscrow(t, store, "alice", aliceDEK))
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'erin'`)
	mustExec(t, store, `INSERT INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at)
		SELECT 'erin', escrow_key_id, sealed, granted_at FROM escrow_grants WHERE admin_username = 'alice'`)

	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, "dave"); !errors.Is(err, ErrStandbyChanged) {
		t.Fatalf("naming dave, who is not the standby = %v, want ErrStandbyChanged", err)
	}
	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("ReplaceEscrowKey keeping bob: %v", err)
	}
	openEscrow(t, store, "alice", aliceDEK)
	openEscrow(t, store, "bob", bobDEK)
	if n := countRows(t, store, "escrow_grants", "admin_username NOT IN ('alice', 'bob')"); n != 0 {
		t.Fatalf("%d grants went to accounts not named", n)
	}

	// A standby row forged to name erin, with a grant row: still nothing.
	mustExec(t, store, `UPDATE super_succession SET standby = 'erin'`)
	mustExec(t, store, `INSERT INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at)
		SELECT 'erin', escrow_key_id, sealed, granted_at FROM escrow_grants WHERE admin_username = 'alice'`)
	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, ""); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'alice'"); n != 0 {
		t.Fatalf("%d grants besides alice's after a replacement naming nobody", n)
	}
	clear(daveDEK)
}

// A standby not named in a replacement loses the key, and the Succession
// section can say so.
func TestAStandbyNotNamedLosesTheKey(t *testing.T) {
	store, aliceDEK, _, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, ""); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	if got := mustSuccession(t, store, "alice"); got.Standby != "bob" || got.StandbyHoldsKey {
		t.Fatalf("Succession = %+v; want bob still standby, without the key", got)
	}
}

func TestOnlyTheSuperWithAUsableKeyReplacesIt(t *testing.T) {
	store, aliceDEK, bobDEK := superStore(t)
	if _, err := store.ReplaceEscrowKey("bob", bobDEK, ""); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("ReplaceEscrowKey by a subordinate = %v, want ErrNotSuper", err)
	}
	oldID := activeKeyID(t, store)
	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)
	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, ""); !errors.Is(err, ErrNoEscrowGrant) {
		t.Fatalf("ReplaceEscrowKey with a key that does not open = %v, want ErrNoEscrowGrant", err)
	}
	if activeKeyID(t, store) != oldID {
		t.Fatal("the key was replaced without a usable key")
	}
	requireEvent(t, store, "alice escrow_key_mismatch alice") // committed, not rolled back
}

// A DEK that does not open with the old key is not carried over; the
// account is sealed again, on trust, at its next sign-in.
func TestUnopenableDEKsAreSealedAgainAtSignIn(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	carolDEK := addUser(t, store, "carol", nil)
	mustExec(t, store, `UPDATE sealed_deks SET sealed = x'00' WHERE username = 'carol'`)

	res, err := store.ReplaceEscrowKey("alice", aliceDEK, "")
	if err != nil || res != (RotationResult{Resealed: 1, Reenroll: 1}) {
		t.Fatalf("ReplaceEscrowKey = %+v, %v; want alice re-sealed and carol to enroll again", res, err)
	}
	if countRows(t, store, "sealed_deks", "username = 'carol'")+countRows(t, store, "escrow_pins", "username = 'carol'") != 0 {
		t.Fatal("carol's unopenable DEK or stale pin was kept")
	}
	requireEvent(t, store, "carol escrow_key_mismatch carol")
	if err := store.EnrollSession("carol", carolDEK); err != nil {
		t.Fatalf("carol's sign-in: %v", err)
	}
	requireEvent(t, store, "carol escrow_reenrolled carol")
	if got, err := openSealedDEK(store, openEscrow(t, store, "alice", aliceDEK), "carol"); err != nil || !bytes.Equal(got, carolDEK) {
		t.Fatalf("carol's DEK under the new key = %x, %v", got, err)
	}
}

// A hand-over never replaces the key in the session of the person leaving:
// the new super administrator does it at their next sign-in.
func TestHandOverLeavesTheReplacementToTheNewSuper(t *testing.T) {
	store, aliceDEK, bobDEK := superStore(t)
	oldID := activeKeyID(t, store)
	if keyPassed, err := store.HandOverSuper("alice", aliceDEK, "bob"); err != nil || !keyPassed {
		t.Fatalf("HandOverSuper = %v, %v", keyPassed, err)
	}
	if activeKeyID(t, store) != oldID || !rotationPending(t, store) {
		t.Fatal("want the replacement pending, not done in the leaving session")
	}

	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's sign-in: %v", err)
	}
	if activeKeyID(t, store) == oldID || rotationPending(t, store) {
		t.Fatal("bob's sign-in did not replace the key")
	}
	openEscrow(t, store, "bob", bobDEK)
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'bob'"); n != 0 {
		t.Fatalf("%d other grants after bob's replacement", n)
	}
	if e := latestEvent(t, store); e.Actor != "bob" || e.Action != AccountEscrowRotated ||
		e.Detail != "the super administrator role was handed to bob; 3 accounts sealed to the new key" {
		t.Fatalf("latest event = %+v; want bob's replacement", e)
	}
}

// After a takeover by someone without the key, the passed-over standby
// replaces it at their next sign-in, while the super administrator holds
// none; while the super administrator holds a key, only they do.
func TestWhoRunsAPendingReplacement(t *testing.T) {
	store, aliceDEK, bobDEK, daveDEK := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	oldID := activeKeyID(t, store)

	mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'test', '2026-10-07T00:00:00Z')`)
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's sign-in: %v", err)
	}
	if activeKeyID(t, store) != oldID {
		t.Fatal("the standby replaced the key while the super administrator holds one")
	}
	mustExec(t, store, `DELETE FROM escrow_rotation_pending`)

	superInactiveFor(t, store, 31*day)
	away := time.Now().UTC().Add(-31 * day).Format(time.RFC3339Nano)
	mustExec(t, store, `UPDATE users SET last_login = ? WHERE username = 'bob'`, away)
	mustExec(t, store, `UPDATE super_succession SET standby_named_at = ?`, away)
	if _, err := store.TakeOverSuper("dave", daveDEK, "vacation", ""); err != nil {
		t.Fatalf("TakeOverSuper(dave): %v", err)
	}
	if activeKeyID(t, store) != oldID || !rotationPending(t, store) {
		t.Fatal("a takeover without the key must leave the replacement pending")
	}
	if err := store.EnrollAdminSession("dave", daveDEK); err != nil {
		t.Fatalf("dave's sign-in: %v", err)
	}
	if activeKeyID(t, store) != oldID {
		t.Fatal("dave, holding no key, replaced it")
	}
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's sign-in: %v", err)
	}
	if activeKeyID(t, store) == oldID || rotationPending(t, store) {
		t.Fatal("the standby did not replace the key while the super administrator holds none")
	}
	openEscrow(t, store, "bob", bobDEK)
}

// Removing a key holder in any way marks the key for replacement; the
// super administrator's key then replaces it.
func TestRemovingAKeyHolderMarksTheReplacement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		remove func(s *Store) error
	}{
		{"demoted", func(s *Store) error { return s.SetAdmin("alice", "bob", false) }},
		{"disabled", func(s *Store) error { return s.SetDisabled("alice", "bob", true) }},
		{"deleted", func(s *Store) error { return s.PurgeAccount("alice", "bob") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, aliceDEK, _, _ := standbyStore(t)
			mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
			oldID := activeKeyID(t, store)
			if err := tc.remove(store); err != nil {
				t.Fatalf("remove bob: %v", err)
			}
			if !rotationPending(t, store) {
				t.Fatal("removing a key holder did not mark the key for replacement")
			}
			if err := store.RunPendingRotation("alice", aliceDEK); err != nil {
				t.Fatalf("RunPendingRotation: %v", err)
			}
			if activeKeyID(t, store) == oldID || rotationPending(t, store) {
				t.Fatal("the pending replacement did not run")
			}
			openEscrow(t, store, "alice", aliceDEK)
		})
	}

	// Removing an administrator who holds no key marks nothing.
	store, aliceDEK, _, _ := standbyStore(t)
	if err := store.SetDisabled("alice", "dave", true); err != nil {
		t.Fatalf("disable dave: %v", err)
	}
	if rotationPending(t, store) {
		t.Fatal("disabling an administrator without the key marked a replacement")
	}
	clear(aliceDEK)
}

// A replacement that fails is rolled back alone: the change that triggered
// it stands, the key is unchanged, and the replacement stays pending.
func TestAFailedReplacementLeavesTheChangeAndStaysPending(t *testing.T) {
	store, aliceDEK, _, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	oldID := activeKeyID(t, store)
	mustExec(t, store, `CREATE TRIGGER block_new_keys BEFORE INSERT ON escrow_keys BEGIN SELECT RAISE(ABORT, 'blocked'); END`)

	mustSetStandby(t, store, "alice", aliceDEK, "dave", 30)
	if got := mustSuccession(t, store, "alice"); got.Standby != "dave" {
		t.Fatalf("standby = %q, want the change kept", got.Standby)
	}
	if activeKeyID(t, store) != oldID || !rotationPending(t, store) {
		t.Fatal("want the key unchanged and the replacement pending")
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); n != 0 {
		t.Fatal("the replaced standby kept the key when the replacement failed")
	}
	openEscrow(t, store, "alice", aliceDEK)

	mustExec(t, store, `DROP TRIGGER block_new_keys`)
	if err := store.RunPendingRotation("alice", aliceDEK); err != nil {
		t.Fatalf("RunPendingRotation: %v", err)
	}
	if activeKeyID(t, store) == oldID || rotationPending(t, store) {
		t.Fatal("the pending replacement did not run once it could")
	}
}

// An administrator's session opened before a replacement must not enroll a
// new account against the retired key: the App opens it in one transaction
// and creates the account in another.
func TestAStaleSessionEnrollsAgainstTheActiveKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	stale := openEscrow(t, store, "alice", aliceDEK)
	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, ""); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	newID := activeKeyID(t, store)

	_, dek, err := store.CreateAccountWithKey("alice", "erin", "Erin", statusPassword, false, stale)
	if err != nil {
		t.Fatalf("create erin with a stale session: %v", err)
	}
	t.Cleanup(func() { clear(dek) })
	if countRows(t, store, "sealed_deks", "username = 'erin' AND escrow_key_id = '"+newID+"'") != 1 ||
		countRows(t, store, "escrow_pins", "username = 'erin' AND escrow_key_id = '"+newID+"'") != 1 {
		t.Fatal("erin was not sealed and pinned to the active key")
	}
	if got, err := openSealedDEK(store, openEscrow(t, store, "alice", aliceDEK), "erin"); err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("erin's DEK under the active key = %x, %v", got, err)
	}
	before := len(eventActions(t, store))
	if err := store.EnrollSession("erin", dek); err != nil {
		t.Fatalf("erin's sign-in: %v", err)
	}
	if after := len(eventActions(t, store)); after != before {
		t.Fatalf("erin's sign-in recorded %v", eventActions(t, store)[before:])
	}
}

// A personal key swapped before a replacement was refused before it and
// must still be refused after it: the replacement must not turn it into a
// key that is trusted on first use.
func TestReplacingTheKeyKeepsRefusingASwappedPersonalKey(t *testing.T) {
	store, aliceDEK, bobDEK, _ := standbyStore(t)
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	_, attackerPub := attackerKeyPair(t)
	mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'bob'`, attackerPub)

	if _, err := store.ReplaceEscrowKey("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("ReplaceEscrowKey: %v", err)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); n != 0 {
		t.Fatal("the new key was granted to a swapped personal key")
	}
	if _, err := store.SetStandby("alice", aliceDEK, "bob", 30); !errors.Is(err, ErrPersonalKeyNotAttested) {
		t.Fatalf("passing the key to bob after the replacement = %v, want ErrPersonalKeyNotAttested", err)
	}
	if n := countRows(t, store, "account_events", "action = 'personal_key_trusted' AND username = 'bob'"); n != 1 {
		t.Fatalf("bob's key was trusted on first use %d times, want only the original naming", n)
	}

	// bob's own sign-in repairs the key from his private key and clears the
	// attestation, as it does without a replacement; then he can be passed
	// the key again, trusted on first use and recorded.
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's sign-in: %v", err)
	}
	requireEvent(t, store, "bob personal_key_repaired bob")
	mustSetStandby(t, store, "alice", aliceDEK, "bob", 30)
	if n := countRows(t, store, "account_events", "action = 'personal_key_trusted' AND username = 'bob'"); n != 2 {
		t.Fatalf("bob's repaired key was trusted on first use %d times, want 2", n)
	}
	openEscrow(t, store, "bob", bobDEK)
}
