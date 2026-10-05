// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func mustSuper(t *testing.T, store *Store) string {
	t.Helper()
	var super string
	err := store.inWriteTx("super administrator", func(ctx context.Context, q accountWriter) error {
		var err error
		super, err = ensureSuperTx(ctx, q)
		return err
	})
	if err != nil {
		t.Fatalf("ensureSuperTx: %v", err)
	}
	return super
}

// superStore returns a store where alice is the super administrator holding
// the key, bob a subordinate administrator who has signed in, and carol a
// standard account; with alice's and bob's DEKs.
func superStore(t *testing.T) (*Store, []byte, []byte) {
	t.Helper()
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	addUser(t, store, "carol", nil)
	if err := store.PromoteAdmin("alice", aliceDEK, "bob"); err != nil {
		t.Fatalf("PromoteAdmin(bob): %v", err)
	}
	return store, aliceDEK, bobDEK
}

func TestFirstAccountIsTheSuperAdministrator(t *testing.T) {
	store, _ := escrowStore(t)
	if super := mustSuper(t, store); super != "alice" {
		t.Fatalf("super administrator = %q, want alice", super)
	}
	requireEvent(t, store, "alice super_admin_assigned alice")
}

// On an install that already has several administrators, the
// earliest-created enabled administrator becomes the super administrator
// (owner decision, 2026-10-05), compared as times, not as text.
func TestExistingInstallMakesTheEarliestAdministratorSuper(t *testing.T) {
	store := openTestStore(t)
	for _, name := range []string{"bob", "carol", "dave"} {
		if _, err := store.CreateAccount(name, name, statusPassword, true); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	// Times that sort wrongly as RFC 3339 text: carol is earliest, but
	// her trimmed fraction sorts after bob's as a string.
	mustExec(t, store, `UPDATE users SET created_at = '2026-01-01T00:00:00.5Z' WHERE username = 'bob'`)
	mustExec(t, store, `UPDATE users SET created_at = '2026-01-01T00:00:00.25Z' WHERE username = 'carol'`)
	mustExec(t, store, `UPDATE users SET created_at = ? WHERE username = 'dave'`, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	mustExec(t, store, `UPDATE users SET disabled = 1 WHERE username = 'dave'`)

	if super := mustSuper(t, store); super != "carol" {
		t.Fatalf("super administrator = %q, want carol, the earliest enabled administrator", super)
	}
}

func TestSuperIsReassignedOnlyWhenItNamesNoEnabledAdministrator(t *testing.T) {
	store, _, _ := superStore(t)
	mustExec(t, store, `UPDATE super_admin SET username = 'bob'`)
	before := len(eventActions(t, store))
	if super := mustSuper(t, store); super != "bob" {
		t.Fatalf("super administrator = %q; a row naming an enabled administrator must be kept", super)
	}
	if after := len(eventActions(t, store)); after != before {
		t.Fatal("reading the role recorded an assignment")
	}

	mustExec(t, store, `UPDATE users SET disabled = 1 WHERE username = 'bob'`)
	if super := mustSuper(t, store); super != "alice" {
		t.Fatalf("super administrator = %q; want alice once bob cannot sign in", super)
	}
	events, _ := store.AccountEvents()
	if events[0].Action != AccountSuperAssigned || events[0].Detail != "the earliest administrator, since bob is no longer an administrator who can sign in" {
		t.Fatalf("latest event = %+v; want the reassignment recorded with its reason", events[0])
	}
}

// Every administrator-affecting store action refuses a subordinate.
func TestOnlyTheSuperChangesAdministrators(t *testing.T) {
	store, _, bobDEK := superStore(t)
	daveDEK := addUser(t, store, "dave", nil)
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'dave'`)
	clear(daveDEK)

	for name, action := range map[string]func() error{
		"promote":            func() error { return store.PromoteAdmin("bob", bobDEK, "carol") },
		"promote (SetAdmin)": func() error { return store.SetAdmin("bob", "carol", true) },
		"demote":             func() error { return store.SetAdmin("bob", "dave", false) },
		"disable an admin":   func() error { return store.SetDisabled("bob", "dave", true) },
		"delete an admin":    func() error { return store.PurgeAccount("bob", "dave") },
		"create an admin": func() error {
			_, _, err := store.CreateAccountWithKey("bob", "zed", "Zed", statusPassword, true, nil)
			return err
		},
		"open data":            func() error { _, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking"); return err },
		"hand over":            func() error { _, err := store.HandOverSuper("bob", bobDEK, "dave"); return err },
		"demote the super":     func() error { return store.SetAdmin("bob", "alice", false) },
		"disable the super":    func() error { return store.SetDisabled("bob", "alice", true) },
		"delete the super":     func() error { return store.PurgeAccount("bob", "alice") },
		"check the super role": func() error { return store.RequireSuper("bob") },
	} {
		if err := action(); !errors.Is(err, ErrNotSuper) {
			t.Errorf("%s by a subordinate = %v, want ErrNotSuper", name, err)
		}
	}
	if roles := accountRoles(t, store); !roles["dave"] || roles["carol"] {
		t.Fatalf("roles = %v; refused actions must change nothing", roles)
	}

	// What subordinates may still do: standard accounts.
	if _, _, err := store.CreateAccountWithKey("bob", "erin", "Erin", statusPassword, false, nil); err != nil {
		t.Fatalf("subordinate creating a standard account: %v", err)
	}
	if err := store.SetDisabled("bob", "carol", true); err != nil {
		t.Fatalf("subordinate disabling a standard account: %v", err)
	}
	if err := store.SetDisabled("bob", "carol", false); err != nil {
		t.Fatalf("subordinate enabling a standard account: %v", err)
	}
	if err := store.PurgeAccount("bob", "erin"); err != nil {
		t.Fatalf("subordinate deleting a standard account: %v", err)
	}
}

func TestNobodyCanRemoveTheSuperAdministrator(t *testing.T) {
	store, _, _ := superStore(t)
	for name, action := range map[string]func() error{
		"demote":  func() error { return store.SetAdmin("alice", "alice", false) },
		"disable": func() error { return store.SetDisabled("alice", "alice", true) },
		"delete":  func() error { return store.PurgeAccount("alice", "alice") },
	} {
		if err := action(); !errors.Is(err, ErrTargetIsSuper) {
			t.Errorf("%s the super administrator = %v, want ErrTargetIsSuper", name, err)
		}
	}
	if super := mustSuper(t, store); super != "alice" {
		t.Fatalf("super administrator = %q, want alice", super)
	}
}

func TestHandOverMovesTheRoleAndTheKey(t *testing.T) {
	store, aliceDEK, bobDEK := superStore(t)
	aliceSession := openEscrow(t, store, "alice", aliceDEK)
	aliceKey := append([]byte(nil), aliceSession.public...)

	keyPassed, err := store.HandOverSuper("alice", aliceDEK, "bob")
	if err != nil || !keyPassed {
		t.Fatalf("HandOverSuper(bob) = %v, %v; want the role and the key passed", keyPassed, err)
	}
	if super := mustSuper(t, store); super != "bob" {
		t.Fatalf("super administrator = %q, want bob", super)
	}
	if got := openEscrow(t, store, "bob", bobDEK); !bytes.Equal(got.public, aliceKey) {
		t.Fatal("bob's grant does not open to the administrator key")
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("the former super administrator kept the key")
	}
	requireEvent(t, store, "alice super_admin_handed_over bob")
	if _, err := store.OpenUserForAdmin("alice", aliceDEK, "carol", "checking"); !errors.Is(err, ErrNotSuper) {
		t.Fatalf("former super administrator opening data = %v, want ErrNotSuper", err)
	}
	dek, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking")
	if err != nil {
		t.Fatalf("new super administrator opening data: %v", err)
	}
	clear(dek)
	// The new super administrator can hand it back.
	if _, err := store.HandOverSuper("bob", bobDEK, "alice"); err != nil {
		t.Fatalf("handing back: %v", err)
	}
	if super := mustSuper(t, store); super != "alice" {
		t.Fatalf("super administrator after handing back = %q, want alice", super)
	}
}

func TestHandOverRefusals(t *testing.T) {
	store, aliceDEK, _ := superStore(t)
	session := openEscrow(t, store, "alice", aliceDEK)
	_, attackerPub := attackerKeyPair(t)
	// dave: an administrator whose attested key was swapped.
	addUser(t, store, "dave", session)
	mustExec(t, store, `UPDATE users SET is_admin = 1 WHERE username = 'dave'`)
	mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'dave'`, attackerPub)
	// erin: an administrator who has not signed in since enrollment.
	if _, err := store.CreateAccount("erin", "Erin", statusPassword, true); err != nil {
		t.Fatalf("create erin: %v", err)
	}
	// frank: a disabled administrator.
	addUser(t, store, "frank", nil)
	mustExec(t, store, `UPDATE users SET is_admin = 1, disabled = 1 WHERE username = 'frank'`)

	for target, want := range map[string]error{
		"carol": ErrTargetNotAdmin,
		"frank": ErrTargetNotAdmin,
		"erin":  ErrNoPersonalKey,
		"dave":  ErrPersonalKeyNotAttested,
		"alice": ErrTargetIsSuper,
	} {
		if _, err := store.HandOverSuper("alice", aliceDEK, target); !errors.Is(err, want) {
			t.Errorf("HandOverSuper(%s) = %v, want %v", target, err, want)
		}
	}
	if super := mustSuper(t, store); super != "alice" {
		t.Fatalf("super administrator = %q after refused hand-overs, want alice", super)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username <> 'alice'"); n != 0 {
		t.Fatalf("%d grants were given by refused hand-overs", n)
	}
	requireEvent(t, store, "alice escrow_key_mismatch dave")
}

// With a key that won't open, the role still moves and the history says the
// key could not be passed.
func TestHandOverWithoutAUsableKeyMovesTheRoleOnly(t *testing.T) {
	store, aliceDEK, _ := superStore(t)
	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)

	keyPassed, err := store.HandOverSuper("alice", aliceDEK, "bob")
	if err != nil || keyPassed {
		t.Fatalf("HandOverSuper = %v, %v; want the role moved without the key", keyPassed, err)
	}
	if super := mustSuper(t, store); super != "bob" {
		t.Fatalf("super administrator = %q, want bob", super)
	}
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 0 {
		t.Fatalf("%d grants remain; want none", n)
	}
	events, _ := store.AccountEvents()
	if events[0].Action != AccountSuperHandedOver || events[0].Detail != "the administrator key could not be passed" {
		t.Fatalf("latest event = %+v; want the hand-over recorded without the key", events[0])
	}
}

// The role is a plaintext row: naming a subordinate there gives them no
// way to the key. (ADR-005: every key operation needs the caller's own
// usable grant.)
func TestForgedSuperRowGivesNoKey(t *testing.T) {
	store, _, bobDEK := superStore(t)
	mustExec(t, store, `UPDATE super_admin SET username = 'bob'`)

	if dek, err := store.OpenUserForAdmin("bob", bobDEK, "carol", "checking"); !errors.Is(err, ErrNoEscrowGrant) || dek != nil {
		t.Fatalf("OpenUserForAdmin by a forged super administrator = %d bytes, %v; want ErrNoEscrowGrant", len(dek), err)
	}
	keyPassed, err := store.HandOverSuper("bob", bobDEK, "alice")
	if err != nil || keyPassed {
		t.Fatalf("HandOverSuper by a forged super administrator = %v, %v; want no key passed", keyPassed, err)
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); n != 0 {
		t.Fatal("a forged super administrator obtained a grant")
	}
}

func TestClaimingTheAdministratorRoleMakesTheClaimantSuper(t *testing.T) {
	store := openTestStore(t)
	for _, name := range []string{"bob", "carol"} {
		if _, err := store.CreateAccount(name, name, statusPassword, false); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	if err := store.ClaimAdmin("carol"); err != nil {
		t.Fatalf("ClaimAdmin(carol): %v", err)
	}
	if super := mustSuper(t, store); super != "carol" {
		t.Fatalf("super administrator = %q, want the claimant", super)
	}
	requireEvent(t, store, "carol super_admin_assigned carol")
}

// On an upgraded install the earliest administrator is super; another
// administrator who signs in first must not create the key and so become
// its holder. The super administrator's own first session creates it.
func TestSubordinateSignInDoesNotCreateTheKey(t *testing.T) {
	store := openTestStore(t)
	for _, name := range []string{"carol", "bob"} {
		if _, err := store.CreateAccount(name, name, statusPassword, true); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mustExec(t, store, `UPDATE users SET created_at = '2026-01-01T00:00:00Z' WHERE username = 'carol'`)
	mustExec(t, store, `UPDATE users SET created_at = '2026-02-01T00:00:00Z' WHERE username = 'bob'`)
	bobDEK, err := store.UnlockDEK("bob", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(bob): %v", err)
	}
	defer clear(bobDEK)
	if err := store.EnrollAdminSession("bob", bobDEK); err != nil {
		t.Fatalf("bob's session: %v", err)
	}
	if n := countRows(t, store, "escrow_keys", "1 = 1"); n != 0 {
		t.Fatal("a subordinate's sign-in created the administrator key")
	}
	carolDEK, err := store.UnlockDEK("carol", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(carol): %v", err)
	}
	defer clear(carolDEK)
	if err := store.EnrollAdminSession("carol", carolDEK); err != nil {
		t.Fatalf("carol's session: %v", err)
	}
	openEscrow(t, store, "carol", carolDEK)
	if n := countRows(t, store, "escrow_grants", "1 = 1"); n != 1 {
		t.Fatalf("%d grants, want only the super administrator's", n)
	}
}
