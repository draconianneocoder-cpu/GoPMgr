// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopmgr/internal/auth"
	"gopmgr/internal/sqlitedriver"
)

const statusPassword = "passphrase-long"

// newStatusStore returns a store with alice (administrator) and bob
// (standard), both created through the store directly.
func newStatusStore(t *testing.T) *Store {
	t.Helper()
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", statusPassword, true); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.CreateAccount("bob", "Bob", statusPassword, false); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	return store
}

func eventActions(t *testing.T, store *Store) []string {
	t.Helper()
	events, err := store.AccountEvents()
	if err != nil {
		t.Fatalf("AccountEvents: %v", err)
	}
	// Oldest first reads more naturally in assertions.
	out := make([]string, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		out = append(out, events[i].Actor+" "+events[i].Action+" "+events[i].Username)
	}
	return out
}

func assertEvents(t *testing.T, store *Store, want ...string) {
	t.Helper()
	got := eventActions(t, store)
	if len(got) != len(want) {
		t.Fatalf("events = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %q, want %q", got, want)
		}
	}
}

func lastLogin(t *testing.T, store *Store, username string) string {
	t.Helper()
	var v string
	if err := store.conn.QueryRow(`SELECT last_login FROM users WHERE username = ?`, username).Scan(&v); err != nil {
		t.Fatalf("read last_login: %v", err)
	}
	return v
}

func TestDisabledAccountCannotSignInUntilEnabled(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	before := lastLogin(t, store, "bob")

	if _, err := store.Authenticate("bob", statusPassword); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("Authenticate disabled bob: err = %v, want ErrAccountDisabled", err)
	}
	// A wrong password gets the ordinary mismatch, so the disabled state
	// is only visible to someone who knows the password.
	if _, err := store.Authenticate("bob", "wrong-password"); !errors.Is(err, auth.ErrMismatch) {
		t.Fatalf("Authenticate disabled bob with wrong password: err = %v, want auth.ErrMismatch", err)
	}
	if after := lastLogin(t, store, "bob"); after != before {
		t.Fatalf("last_login changed by a refused sign-in: %q -> %q", before, after)
	}
	accs := accountRoles(t, store)
	if _, ok := accs["bob"]; !ok {
		t.Fatal("disabled account missing from List")
	}

	if err := store.SetDisabled("alice", "bob", false); err != nil {
		t.Fatalf("enable bob: %v", err)
	}
	if _, err := store.Authenticate("bob", statusPassword); err != nil {
		t.Fatalf("Authenticate after enabling: %v", err)
	}
	assertEvents(t, store, "alice disabled bob", "alice enabled bob")
}

func TestListReportsDisabledAccounts(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	accs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, a := range accs {
		if a.Disabled != (a.Username == "bob") {
			t.Fatalf("%s Disabled = %v", a.Username, a.Disabled)
		}
	}
}

func TestSetDisabledWithNoChangeRecordsNothing(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetDisabled("alice", "bob", false); err != nil {
		t.Fatalf("enable an enabled account: %v", err)
	}
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob again: %v", err)
	}
	assertEvents(t, store, "alice disabled bob")
}

// TestLastEnabledAdminGuards covers every way an enabled administrator
// can be taken out of use while a disabled administrator exists.
func TestLastEnabledAdminGuards(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetAdmin("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}

	if err := store.SetAdmin("alice", false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote the only enabled admin: err = %v, want ErrLastAdmin", err)
	}
	if err := store.SetDisabled("bob", "alice", true); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable the only enabled admin: err = %v, want ErrLastAdmin", err)
	}
	if err := store.PurgeAccount("bob", "alice"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("purge the only enabled admin: err = %v, want ErrLastAdmin", err)
	}
	if roles := accountRoles(t, store); !roles["alice"] {
		t.Fatalf("roles after refusals = %v, want alice still an administrator", roles)
	}
	// Only the disable of bob; the refused actions recorded nothing.
	assertEvents(t, store, "alice disabled bob")

	if err := store.SetAdmin("bob", false); err != nil {
		t.Fatalf("demote a disabled admin: %v", err)
	}
}

func TestHasAnyAdminIgnoresDisabledAdministrators(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", statusPassword, true); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	// The guards forbid this state; it can only come from editing
	// system.db, and then Become administrator must be offered.
	if _, err := store.conn.Exec(`UPDATE users SET disabled = 1 WHERE username = 'alice'`); err != nil {
		t.Fatalf("disable alice directly: %v", err)
	}
	if got, err := store.HasAnyAdmin(); err != nil || got {
		t.Fatalf("HasAnyAdmin with only a disabled admin = %v, %v; want false", got, err)
	}
}

func TestDisabledAdministratorCannotCreateAccounts(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetAdmin("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if _, err := store.CreateAccountAs("bob", "carol", "", statusPassword, false); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("CreateAccountAs by a disabled admin: err = %v, want ErrNotAdmin", err)
	}
	assertNoAccountFolder(t, store, "carol")
}

func TestPurgeAccountDeletesAccountFolderAndCodes(t *testing.T) {
	store := newStatusStore(t)
	dek, err := store.UnlockDEK("bob", statusPassword)
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}
	if _, err := store.IssueRecoveryCodes("bob", dek); err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	project := filepath.Join(store.RootDir(), "bob", "projects", "plan.gopmgr")
	if err := os.WriteFile(project, []byte("bob's project"), 0o600); err != nil {
		t.Fatalf("write project: %v", err)
	}

	if err := store.PurgeAccount("alice", "bob"); err != nil {
		t.Fatalf("PurgeAccount: %v", err)
	}
	if roles := accountRoles(t, store); len(roles) != 1 {
		t.Fatalf("accounts after purge = %v, want only alice", roles)
	}
	var codes int
	if err := store.conn.QueryRow(`SELECT COUNT(*) FROM recovery_codes WHERE username = 'bob'`).Scan(&codes); err != nil || codes != 0 {
		t.Fatalf("bob's recovery codes after purge = %d, %v; want 0", codes, err)
	}
	assertNoAccountFolder(t, store, "bob")
	assertEvents(t, store, "alice purged bob")

	// The name is free again: the folder went with the account.
	if _, err := store.CreateAccountAs("alice", "bob", "Another Bob", statusPassword, false); err != nil {
		t.Fatalf("recreate bob after purge: %v", err)
	}
}

// TestPurgeAccountDoesNotFollowSymlinks proves a link inside the account's
// folder is removed without touching what it points to.
func TestPurgeAccountDoesNotFollowSymlinks(t *testing.T) {
	store := newStatusStore(t)
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("not bob's"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(store.RootDir(), "bob", "projects", "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := store.PurgeAccount("alice", "bob"); err != nil {
		t.Fatalf("PurgeAccount: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "not bob's" {
		t.Fatalf("file outside bob's folder after purge: %q, %v", data, err)
	}
	assertNoAccountFolder(t, store, "bob")
}

func TestPurgeAccountReportsFolderItCouldNotRemove(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not block removal on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	store := newStatusStore(t)
	locked := filepath.Join(store.RootDir(), "bob", "projects", "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatalf("mkdir locked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(locked, "plan.gopmgr"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write locked file: %v", err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatalf("chmod locked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	err := store.PurgeAccount("alice", "bob")
	if !errors.Is(err, ErrPurgeIncomplete) {
		t.Fatalf("PurgeAccount with an unremovable file: err = %v, want ErrPurgeIncomplete", err)
	}
	if roles := accountRoles(t, store); len(roles) != 1 {
		t.Fatalf("accounts after incomplete purge = %v, want only alice", roles)
	}
	assertEvents(t, store, "alice purged bob", "alice folder_not_removed bob")
}

// TestPurgeAccountRefusesGoPMgrsOwnFolder covers an account named "logs"
// from before that name was reserved: its folder is the app's log folder.
func TestPurgeAccountRefusesGoPMgrsOwnFolder(t *testing.T) {
	store := newStatusStore(t)
	logs := filepath.Join(store.RootDir(), "logs")
	if err := os.MkdirAll(filepath.Join(logs, "projects"), 0o700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logs, "gopmgr.log"), []byte("log"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := store.conn.Exec(
		`INSERT INTO users (username, display_name, password_hash, data_dir, created_at, is_admin) VALUES ('logs', 'Logs', 'x', ?, '2026-01-01T00:00:00Z', 0)`,
		logs,
	); err != nil {
		t.Fatalf("seed logs account: %v", err)
	}

	if err := store.PurgeAccount("alice", "logs"); !errors.Is(err, ErrReservedUsername) {
		t.Fatalf("PurgeAccount(logs): err = %v, want ErrReservedUsername", err)
	}
	if _, err := os.Stat(filepath.Join(logs, "gopmgr.log")); err != nil {
		t.Fatalf("log file after refused purge: %v", err)
	}
	if roles := accountRoles(t, store); len(roles) != 3 {
		t.Fatalf("accounts after refused purge = %v, want logs kept", roles)
	}
	assertEvents(t, store)
}

func TestAccountEventsAreAppendOnly(t *testing.T) {
	store := newStatusStore(t)
	if err := store.SetDisabled("alice", "bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if _, err := store.conn.Exec(`UPDATE account_events SET action = 'enabled'`); err == nil {
		t.Fatal("UPDATE on account_events succeeded, want it refused")
	}
	if _, err := store.conn.Exec(`DELETE FROM account_events`); err == nil {
		t.Fatal("DELETE on account_events succeeded, want it refused")
	}
	assertEvents(t, store, "alice disabled bob")
}

// TestOpenAddsAccountStatusToAnOlderDatabase opens a system.db created
// before disabled accounts existed.
func TestOpenAddsAccountStatusToAnOlderDatabase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	old, err := sql.Open(sqlitedriver.Name, filepath.Join(root, "system.db"))
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	if _, err := old.Exec(`
		CREATE TABLE users (
			username      TEXT PRIMARY KEY,
			display_name  TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			data_dir      TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			last_login    TEXT NOT NULL DEFAULT '',
			is_admin      INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO users (username, display_name, password_hash, data_dir, created_at, is_admin)
		VALUES ('alice', 'Alice', 'x', '', '2026-01-01T00:00:00Z', 1);`); err != nil {
		t.Fatalf("seed old db: %v", err)
	}
	if err := old.Close(); err != nil {
		t.Fatalf("close old db: %v", err)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open older database: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	accs, err := store.List()
	if err != nil || len(accs) != 1 || accs[0].Disabled || !accs[0].IsAdmin {
		t.Fatalf("List after upgrade = %+v, %v; want alice, enabled administrator", accs, err)
	}
	if events, err := store.AccountEvents(); err != nil || len(events) != 0 {
		t.Fatalf("AccountEvents after upgrade = %v, %v; want none", events, err)
	}
}
