// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func accessEvents(t *testing.T, store *Store) []AccountEvent {
	t.Helper()
	events, err := store.AccountEvents()
	if err != nil {
		t.Fatalf("AccountEvents: %v", err)
	}
	var out []AccountEvent
	for _, e := range events {
		if e.Action == AccountAdminAccess {
			out = append(out, e)
		}
	}
	return out
}

func TestAdministratorAccessIsRecordedAndReleasesTheDEK(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)

	dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "  support ticket 42 \n")
	if err != nil {
		t.Fatalf("OpenUserForAdmin: %v", err)
	}
	defer clear(dek)
	if !bytes.Equal(dek, bobDEK) {
		t.Fatal("the released key is not bob's DEK")
	}
	events := accessEvents(t, store)
	if len(events) != 1 || events[0].Actor != "alice" || events[0].Username != "bob" || events[0].Detail != "support ticket 42" {
		t.Fatalf("access events = %+v; want one by alice of bob with the trimmed reason", events)
	}

	// The longest allowed reason, counted in characters, not bytes.
	long := strings.Repeat("é", MaxAccessReasonLength)
	again, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", long)
	if err != nil {
		t.Fatalf("OpenUserForAdmin with a %d-character reason: %v", MaxAccessReasonLength, err)
	}
	clear(again)
}

// A refused request records nothing and releases no key.
func TestAdministratorAccessRefusalsRecordNothing(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	bobDEK := addUser(t, store, "bob", nil)
	addUser(t, store, "dave", nil)
	if _, err := store.CreateAccount("carol", "Carol", statusPassword, false); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	// frank is an administrator with no grant.
	if _, err := store.CreateAccount("frank", "Frank", statusPassword, true); err != nil {
		t.Fatalf("create frank: %v", err)
	}
	frankDEK, err := store.UnlockDEK("frank", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK(frank): %v", err)
	}
	defer clear(frankDEK)
	// erin was an administrator and was demoted.
	erinDEK := addUser(t, store, "erin", nil)
	if err := store.PromoteAdmin("alice", aliceDEK, "erin"); err != nil {
		t.Fatalf("PromoteAdmin(erin): %v", err)
	}
	if err := store.SetAdmin("alice", "erin", false); err != nil {
		t.Fatalf("demote erin: %v", err)
	}

	for name, tc := range map[string]struct {
		actor  string
		dek    []byte
		target string
		reason string
		want   error
	}{
		"no reason":                 {"alice", aliceDEK, "bob", " \t ", ErrAccessReasonRequired},
		"reason too long":           {"alice", aliceDEK, "bob", strings.Repeat("é", MaxAccessReasonLength+1), ErrAccessReasonTooLong},
		"own account":               {"alice", aliceDEK, "ALICE", "checking", ErrSelfAccess},
		"standard account caller":   {"bob", bobDEK, "dave", "checking", ErrNotAdmin},
		"demoted administrator":     {"erin", erinDEK, "bob", "checking", ErrNotAdmin},
		"subordinate administrator": {"frank", frankDEK, "bob", "checking", ErrNotSuper},
		"unknown account":           {"alice", aliceDEK, "nobody", "checking", ErrNoSuchUser},
		"account not enrolled":      {"alice", aliceDEK, "carol", "checking", ErrNotEnrolled},
	} {
		dek, err := store.OpenUserForAdmin(tc.actor, tc.dek, tc.target, tc.reason)
		if !errors.Is(err, tc.want) || dek != nil {
			t.Errorf("%s: OpenUserForAdmin = %d bytes, %v; want no key and %v", name, len(dek), err, tc.want)
		}
	}
	if events := accessEvents(t, store); len(events) != 0 {
		t.Fatalf("refused requests recorded access: %+v", events)
	}
}

// If the access record cannot be written, no key is released.
func TestFailedAccessRecordReleasesNoKey(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	mustExec(t, store, `CREATE TRIGGER refuse_access BEFORE INSERT ON account_events
		WHEN NEW.action = 'admin_access' BEGIN SELECT RAISE(ABORT, 'injected'); END`)

	dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "checking")
	if err == nil || dek != nil {
		t.Fatalf("OpenUserForAdmin with the record failing = %d bytes, %v; want no key and an error", len(dek), err)
	}
	if events := accessEvents(t, store); len(events) != 0 {
		t.Fatalf("access recorded although the record failed: %+v", events)
	}
}

// If the transaction holding the record fails to commit, no key is
// released. A deferred foreign key, added for the test, fails the COMMIT
// after the record and the key are in hand.
func TestAccessReleasesNoKeyWhenTheCommitFails(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	mustExec(t, store, `CREATE TABLE fail_commit (ref INTEGER REFERENCES missing_parent(id) DEFERRABLE INITIALLY DEFERRED)`)
	mustExec(t, store, `CREATE TABLE missing_parent (id INTEGER PRIMARY KEY)`)
	mustExec(t, store, `CREATE TRIGGER fail_commit_on_access AFTER INSERT ON account_events
		WHEN NEW.action = 'admin_access' BEGIN INSERT INTO fail_commit (ref) VALUES (1); END`)

	dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "checking")
	if err == nil || dek != nil {
		t.Fatalf("OpenUserForAdmin with a failing commit = %d bytes, %v; want no key and an error", len(dek), err)
	}
	if !strings.Contains(err.Error(), "commit") {
		t.Fatalf("err = %v; want the commit to be what failed", err)
	}
	if events := accessEvents(t, store); len(events) != 0 {
		t.Fatalf("access recorded although the commit failed: %+v", events)
	}
}

func TestAccessWithAnUnusableGrantRecordsOnlyTheMismatch(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)

	dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "checking")
	if !errors.Is(err, ErrEscrowMismatch) || dek != nil {
		t.Fatalf("OpenUserForAdmin with an unusable grant = %d bytes, %v; want ErrEscrowMismatch", len(dek), err)
	}
	requireEvent(t, store, "alice escrow_key_mismatch alice")
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 0 {
		t.Fatal("the unusable grant was kept")
	}
	if events := accessEvents(t, store); len(events) != 0 {
		t.Fatalf("access recorded with an unusable grant: %+v", events)
	}
}

func TestAccessToASealedDEKThatDoesNotOpenRecordsOnlyTheMismatch(t *testing.T) {
	store, aliceDEK := escrowStore(t)
	addUser(t, store, "bob", nil)
	// alice's sealed DEK copied onto bob's row.
	mustExec(t, store, `UPDATE sealed_deks SET sealed = (SELECT sealed FROM sealed_deks WHERE username = 'alice') WHERE username = 'bob'`)

	dek, err := store.OpenUserForAdmin("alice", aliceDEK, "bob", "checking")
	if !errors.Is(err, ErrEscrowMismatch) || dek != nil {
		t.Fatalf("OpenUserForAdmin of a DEK that does not open = %d bytes, %v; want ErrEscrowMismatch", len(dek), err)
	}
	requireEvent(t, store, "bob escrow_key_mismatch bob")
	for _, table := range []string{"sealed_deks", "escrow_pins"} {
		if n := countRows(t, store, table, "username = 'bob'"); n != 0 {
			t.Errorf("%s keeps bob's row; his next sign-in should seal again", table)
		}
	}
	if events := accessEvents(t, store); len(events) != 0 {
		t.Fatalf("access recorded for a key that did not open: %+v", events)
	}
}
