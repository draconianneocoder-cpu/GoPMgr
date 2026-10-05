// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// historyRows returns every account_events row, oldest first, as text.
func historyRows(t *testing.T, store *Store) []string {
	t.Helper()
	rows, err := store.conn.Query(`SELECT id, occurred_at, actor, username, action, detail FROM account_events ORDER BY id`)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id int64
		var at, actor, user, action, detail string
		if err := rows.Scan(&id, &at, &actor, &user, &action, &detail); err != nil {
			t.Fatalf("scan history: %v", err)
		}
		out = append(out, fmt.Sprintf("%d|%s|%s|%s|%s|%s", id, at, actor, user, action, detail))
	}
	return out
}

func historyTableSQL(t *testing.T, store *Store) string {
	t.Helper()
	var ddl string
	if err := store.conn.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'account_events'`).Scan(&ddl); err != nil {
		t.Fatalf("read history table SQL: %v", err)
	}
	return ddl
}

// TestOpenRebuildsTheOriginalHistoryTableWithoutItsCheck opens a system.db
// whose account_events was created with the original CHECK on action.
func TestOpenRebuildsTheOriginalHistoryTableWithoutItsCheck(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.CreateAccount("alice", "Alice", statusPassword, true); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.conn.Exec(`
		DROP TABLE account_events;
		CREATE TABLE account_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at TEXT NOT NULL,
			actor       TEXT NOT NULL,
			username    TEXT NOT NULL,
			action      TEXT NOT NULL CHECK (action IN ('disabled', 'enabled', 'purged', 'folder_not_removed')),
			detail      TEXT NOT NULL DEFAULT ''
		);
		CREATE TRIGGER account_events_no_update BEFORE UPDATE ON account_events
		BEGIN SELECT RAISE(ABORT, 'account_events is append-only'); END;
		CREATE TRIGGER account_events_no_delete BEFORE DELETE ON account_events
		BEGIN SELECT RAISE(ABORT, 'account_events is append-only'); END;
		INSERT INTO account_events (id, occurred_at, actor, username, action, detail) VALUES
			(3, '2026-09-24T10:00:00Z', 'alice', 'bob', 'disabled', ''),
			(5, '2026-09-24T10:01:00Z', 'alice', 'bob', 'enabled', ''),
			(9, '2026-09-24T10:02:00Z', 'alice', 'carol', 'folder_not_removed', '/x: denied');`); err != nil {
		t.Fatalf("recreate the original history table: %v", err)
	}
	before := historyRows(t, store)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store, err = Open(root)
	if err != nil {
		t.Fatalf("Open with the original history table: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if ddl := historyTableSQL(t, store); strings.Contains(ddl, "CHECK") {
		t.Fatalf("history table still has its CHECK after Open:\n%s", ddl)
	}
	if after := historyRows(t, store); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("history changed by the rebuild:\nbefore %q\nafter  %q", before, after)
	}
	if _, err := store.conn.Exec(`UPDATE account_events SET action = 'enabled'`); err == nil {
		t.Fatal("UPDATE on the rebuilt history succeeded; the append-only trigger was not recreated")
	}
	if _, err := store.conn.Exec(`DELETE FROM account_events`); err == nil {
		t.Fatal("DELETE on the rebuilt history succeeded; the append-only trigger was not recreated")
	}

	// A new action the old CHECK would have refused, numbered after the rest.
	if _, err := store.CreateAccountAs("alice", "dave", "Dave", statusPassword, false); err != nil {
		t.Fatalf("CreateAccountAs after the rebuild: %v", err)
	}
	events, err := store.AccountEvents()
	if err != nil || len(events) != 4 || events[0].Action != AccountCreated || events[0].ID <= 9 {
		t.Fatalf("newest event after the rebuild = %+v, %v; want a created event with id above 9", events, err)
	}

	// A second Open changes nothing.
	settled := historyRows(t, store)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	store, err = Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if again := historyRows(t, store); fmt.Sprint(again) != fmt.Sprint(settled) {
		t.Fatalf("a second Open changed the history:\nbefore %q\nafter  %q", settled, again)
	}
}

func TestRecordAccountEventRefusesAnUnknownAction(t *testing.T) {
	store := openTestStore(t)
	if err := recordAccountEvent(context.Background(), store.conn, "alice", "bob", "renamed", ""); err == nil {
		t.Fatal("recordAccountEvent with an unknown action: got nil, want error")
	}
	if rows := historyRows(t, store); len(rows) != 0 {
		t.Fatalf("history after a refused action = %q, want nothing", rows)
	}
}

func TestAccountCreationIsRecordedWithItsRole(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccountAs("", "alice", "Alice", statusPassword, false); err != nil {
		t.Fatalf("first account: %v", err)
	}
	if _, err := store.CreateAccountAs("alice", "bob", "Bob", statusPassword, false); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if _, err := store.CreateAccountAs("alice", "carol", "Carol", statusPassword, true); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	// A refused creation records nothing.
	if _, err := store.CreateAccountAs("bob", "eve", "Eve", statusPassword, false); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("creation by a standard user: err = %v, want ErrNotAdmin", err)
	}
	events, err := store.AccountEvents()
	if err != nil {
		t.Fatalf("AccountEvents: %v", err)
	}
	var got []string
	for i := len(events) - 1; i >= 0; i-- {
		got = append(got, events[i].Actor+" "+events[i].Action+" "+events[i].Username+" "+events[i].Detail)
	}
	// Creating an administrator account is the super administrator's, so
	// alice's role is assigned (ADR-005) before carol is created.
	want := []string{"alice created alice administrator", "alice created bob standard", "alice super_admin_assigned alice the earliest administrator", "alice created carol administrator"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("history = %q, want %q", got, want)
	}
}

func TestRoleChangesAreRecordedOnlyForAnEnabledAdministrator(t *testing.T) {
	store := newStatusStore(t)
	if _, err := store.CreateAccount("carol", "Carol", statusPassword, true); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	if err := store.SetDisabled("alice", "carol", true); err != nil {
		t.Fatalf("disable carol: %v", err)
	}
	for _, actor := range []string{"bob", "carol", "nobody", ""} {
		if err := store.SetAdmin(actor, "bob", true); !errors.Is(err, ErrNotAdmin) {
			t.Fatalf("SetAdmin by %q: err = %v, want ErrNotAdmin", actor, err)
		}
	}
	if roles := accountRoles(t, store); roles["bob"] {
		t.Fatal("a refused promotion made bob an administrator")
	}

	if err := store.SetAdmin("alice", "bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	if err := store.SetAdmin("alice", "bob", true); err != nil {
		t.Fatalf("promote bob again: %v", err)
	}
	if err := store.SetAdmin("alice", "bob", false); err != nil {
		t.Fatalf("demote bob: %v", err)
	}
	assertEvents(t, store, "alice super_admin_assigned alice", "alice disabled carol", "alice promoted bob", "alice demoted bob")
}

func TestClaimAdminOnlyWithNoAdministrator(t *testing.T) {
	store := newStatusStore(t)
	if err := store.ClaimAdmin("bob"); !errors.Is(err, ErrAdminExists) {
		t.Fatalf("ClaimAdmin with an administrator: err = %v, want ErrAdminExists", err)
	}
	assertEvents(t, store)

	// An install whose accounts predate the first-account rule.
	legacy := openTestStore(t)
	for _, name := range []string{"bob", "dave"} {
		if _, err := legacy.CreateAccount(name, name, statusPassword, false); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, name := range []string{"bob", "dave"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = legacy.ClaimAdmin(name)
		}()
	}
	close(start)
	wg.Wait()

	claimed := 0
	for _, err := range errs {
		switch {
		case err == nil:
			claimed++
		case errors.Is(err, ErrAdminExists):
		default:
			t.Fatalf("ClaimAdmin: unexpected error %v", err)
		}
	}
	// The claimant also becomes the super administrator (ADR-005).
	events, err := legacy.AccountEvents()
	if err != nil || claimed != 1 || len(events) != 2 ||
		events[1].Action != AccountPromoted || events[1].Actor != events[1].Username ||
		events[0].Action != AccountSuperAssigned || events[0].Username != events[1].Username {
		t.Fatalf("claims = %d, events = %+v, %v; want one self-recorded promotion, then the claimant made super administrator", claimed, events, err)
	}
}

func TestClaimAdminRefusesADisabledOrUnknownAccount(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("bob", "Bob", statusPassword, false); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if _, err := store.conn.Exec(`UPDATE users SET disabled = 1 WHERE username = 'bob'`); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if err := store.ClaimAdmin("bob"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("ClaimAdmin by a disabled account: err = %v, want ErrAccountDisabled", err)
	}
	if err := store.ClaimAdmin("nobody"); !errors.Is(err, ErrNoSuchUser) {
		t.Fatalf("ClaimAdmin by an unknown account: err = %v, want ErrNoSuchUser", err)
	}
	if roles := accountRoles(t, store); roles["bob"] {
		t.Fatal("a refused claim made bob an administrator")
	}
	assertEvents(t, store)
}
