// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"path/filepath"
	"testing"

	"gopmgr/internal/users"
)

func newAdminTestApp(t *testing.T) *App {
	t.Helper()
	store, err := users.Open(filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	app := &App{store: store}
	t.Cleanup(func() { app.shutdown(context.Background()) })
	return app
}

// signIn sets app.user to the account with the given username, simulating a
// successful login without needing the full Authenticate path.
func signIn(t *testing.T, app *App, username string) {
	t.Helper()
	accs, err := app.store.List()
	if err != nil {
		t.Fatalf("store.List: %v", err)
	}
	for i := range accs {
		if accs[i].Username == username {
			app.mu.Lock()
			app.user = &accs[i]
			app.mu.Unlock()
			return
		}
	}
	t.Fatalf("signIn: user %q not found", username)
}

func TestCreateAccount_BlockedForNonAdminOnceSomeoneIsAdmin(t *testing.T) {
	app := newAdminTestApp(t)
	// First account: admin.
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	// No session (simulates a new visitor trying to self-register).
	app.mu.Lock()
	app.user = nil
	app.mu.Unlock()

	_, err := app.CreateAccount("eve", "Eve", "passphrase-long", false)
	if err == nil {
		t.Fatal("CreateAccount with no session and admin already present: got nil, want error")
	}
}

func TestCreateAccount_AllowedForAdminSession(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount first: %v", err)
	}
	signIn(t, app, "alice")

	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount as admin: %v", err)
	}
}

// seedAccountsWithoutAdmin reproduces an install whose accounts predate
// the rule that the first account is an administrator. The app can no
// longer create this state, so the store is written directly.
func seedAccountsWithoutAdmin(t *testing.T, app *App, usernames ...string) {
	t.Helper()
	for _, name := range usernames {
		if _, err := app.store.CreateAccount(name, name, "passphrase-long", false); err != nil {
			t.Fatalf("store.CreateAccount(%s): %v", name, err)
		}
	}
}

func TestBecomeAdmin_SucceedsWhenNoAdminExists(t *testing.T) {
	app := newAdminTestApp(t)
	seedAccountsWithoutAdmin(t, app, "alice")
	if _, err := app.Login("alice", "passphrase-long"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := app.BecomeAdmin(); err != nil {
		t.Fatalf("BecomeAdmin: %v", err)
	}
	ok, err := app.store.HasAnyAdmin()
	if err != nil {
		t.Fatalf("HasAnyAdmin: %v", err)
	}
	if !ok {
		t.Fatal("HasAnyAdmin = false after BecomeAdmin, want true")
	}
	if u := app.requireUser(); u == nil || !u.IsAdmin {
		t.Fatalf("session after BecomeAdmin = %+v, want an administrator session", u)
	}
	// The new administrator can use admin-only methods without signing in
	// again: each reads the role from the session BecomeAdmin updated.
	if _, err := app.AdminListUsers(); err != nil {
		t.Fatalf("AdminListUsers after BecomeAdmin: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount after BecomeAdmin: %v", err)
	}
}

func TestBecomeAdmin_ErrorsWhenAdminAlreadyExists(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	signIn(t, app, "bob")

	if err := app.BecomeAdmin(); err == nil {
		t.Fatal("BecomeAdmin with existing admin: got nil, want error")
	}
}

func TestAdminPurgeUser_CannotDeleteSelf(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	signIn(t, app, "alice")

	err := app.AdminPurgeUser("alice", "alice")
	if err == nil {
		t.Fatal("AdminPurgeUser self: got nil, want error")
	}
}

func TestAdminSetUserRole_CannotChangeSelf(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	signIn(t, app, "alice")

	err := app.AdminSetUserRole("alice", false)
	if err == nil {
		t.Fatal("AdminSetUserRole self: got nil, want error")
	}
}

func TestAdminPurgeUser_RejectsNonAdmin(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	signIn(t, app, "bob")

	if err := app.AdminPurgeUser("alice", "alice"); err == nil {
		t.Fatal("AdminPurgeUser as non-admin: got nil, want error")
	}
}

func TestCreateAccount_FirstAccountIsAlwaysAdmin(t *testing.T) {
	app := newAdminTestApp(t)
	acc, err := app.CreateAccount("alice", "Alice", "passphrase-long", false)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if !acc.IsAdmin {
		t.Fatal("first account returned as a standard account, want administrator")
	}
	if u := app.requireUser(); u == nil || u.Username != "alice" || !u.IsAdmin {
		t.Fatalf("session after first account = %+v, want alice signed in as administrator", u)
	}
}

// TestCreateAccount_RefusedWhenAccountsExistWithoutAdmin covers installs
// whose accounts predate the first-account rule: nobody may add an account
// until someone claims the administrator role.
func TestCreateAccount_RefusedWhenAccountsExistWithoutAdmin(t *testing.T) {
	app := newAdminTestApp(t)
	seedAccountsWithoutAdmin(t, app, "alice")

	if _, err := app.CreateAccount("eve", "Eve", "passphrase-long", true); err == nil {
		t.Fatal("CreateAccount with no session: got nil, want error")
	}
	if _, err := app.Login("alice", "passphrase-long"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := app.CreateAccount("eve", "Eve", "passphrase-long", true); err == nil {
		t.Fatal("CreateAccount by a standard user: got nil, want error")
	}
	accs, err := app.store.List()
	if err != nil {
		t.Fatalf("store.List: %v", err)
	}
	if len(accs) != 1 {
		t.Fatalf("accounts = %d, want only alice", len(accs))
	}
}

func TestAccountSetupReportsFirstRunState(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, app *App)
		want AccountSetupWire
	}{
		{"no accounts", func(*testing.T, *App) {}, AccountSetupWire{}},
		{"accounts without an administrator", func(t *testing.T, app *App) {
			seedAccountsWithoutAdmin(t, app, "alice")
		}, AccountSetupWire{HasAccounts: true}},
		{"an administrator", func(t *testing.T, app *App) {
			if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
				t.Fatalf("CreateAccount: %v", err)
			}
		}, AccountSetupWire{HasAccounts: true, HasAdmin: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newAdminTestApp(t)
			tc.seed(t, app)
			got, err := app.AccountSetup()
			if err != nil {
				t.Fatalf("AccountSetup: %v", err)
			}
			if got != tc.want {
				t.Fatalf("AccountSetup = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAdminPurgeUser_RequiresTheExactUsername(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}
	for _, typed := range []string{"", "Bob", "bob ", "alice"} {
		if err := app.AdminPurgeUser("bob", typed); err == nil {
			t.Fatalf("AdminPurgeUser(bob) confirmed with %q: got nil, want error", typed)
		}
	}
	events, err := app.AdminListAccountEvents()
	if err != nil {
		t.Fatalf("AdminListAccountEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events after refused purges = %+v, want none", events)
	}
	if err := app.AdminPurgeUser("bob", "bob"); err != nil {
		t.Fatalf("AdminPurgeUser with the exact name: %v", err)
	}
	events, err = app.AdminListAccountEvents()
	if err != nil || len(events) != 1 || events[0].Action != users.AccountPurged || events[0].Actor != "alice" {
		t.Fatalf("events after purge = %+v, %v; want one purge by alice", events, err)
	}
}

func TestAdminSetUserDisabled_BlocksAndRestoresSignIn(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}
	// With bob an administrator too, only the self-check can refuse this;
	// the last-administrator guard would not.
	if err := app.AdminSetUserRole("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	if err := app.AdminSetUserDisabled("alice", true); err == nil {
		t.Fatal("AdminSetUserDisabled self: got nil, want error")
	}
	if err := app.AdminSetUserRole("bob", false); err != nil {
		t.Fatalf("demote bob: %v", err)
	}
	if err := app.AdminSetUserDisabled("bob", true); err != nil {
		t.Fatalf("disable bob: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	_, err := app.Login("bob", "passphrase-long")
	if err == nil || err.Error() != "this account is disabled; ask your administrator to enable it" {
		t.Fatalf("Login disabled bob: err = %v, want the disabled message", err)
	}
	if _, err := app.Login("bob", "wrong-password"); err == nil || err.Error() != "invalid credentials" {
		t.Fatalf("Login disabled bob with a wrong password: err = %v, want invalid credentials", err)
	}
	if u := app.requireUser(); u != nil {
		t.Fatalf("session after refused sign-in = %+v, want none", u)
	}
	if err := app.AdminSetUserDisabled("alice", false); err == nil {
		t.Fatal("AdminSetUserDisabled with no session: got nil, want error")
	}

	if _, err := app.Login("alice", "passphrase-long"); err != nil {
		t.Fatalf("Login alice: %v", err)
	}
	if err := app.AdminSetUserDisabled("bob", false); err != nil {
		t.Fatalf("enable bob: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.Login("bob", "passphrase-long"); err != nil {
		t.Fatalf("Login bob after enabling: %v", err)
	}
	if _, err := app.AdminListAccountEvents(); err == nil {
		t.Fatal("AdminListAccountEvents as a standard user: got nil, want error")
	}
}

func TestChangePassword_KeepsSessionAndEncryptedProjects(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "original-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	project, err := app.CreateProject("Encrypted Plan", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	encrypted, err := app.IsProjectEncrypted(project.Path)
	if err != nil || !encrypted {
		t.Fatalf("IsProjectEncrypted = %v, %v; want an encrypted project", encrypted, err)
	}

	if err := app.ChangePassword("wrong-password", "replacement-password"); err == nil || err.Error() != "current password is incorrect" {
		t.Fatalf("ChangePassword with a wrong current password: err = %v", err)
	}
	if err := app.ChangePassword("original-password", "short"); err == nil || err.Error() != "new password must be at least 8 characters" {
		t.Fatalf("ChangePassword with a short new password: err = %v", err)
	}
	if err := app.ChangePassword("original-password", "replacement-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if u := app.requireUser(); u == nil || u.Username != "alice" {
		t.Fatalf("session after ChangePassword = %+v, want alice still signed in", u)
	}

	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.Login("alice", "original-password"); err == nil {
		t.Fatal("Login with the old password succeeded")
	}
	if _, err := app.Login("alice", "replacement-password"); err != nil {
		t.Fatalf("Login with the new password: %v", err)
	}
	if _, err := app.OpenProject(project.Path); err != nil {
		t.Fatalf("OpenProject after the password change: %v", err)
	}
}

func TestChangePassword_RequiresASession(t *testing.T) {
	app := newAdminTestApp(t)
	if err := app.ChangePassword("anything-long", "replacement-password"); err == nil {
		t.Fatal("ChangePassword with no session: got nil, want error")
	}
}

func TestCreateAccount_RefusesAShortPasswordInPlainWords(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "short", false); err == nil || err.Error() != "password must be at least 8 characters" {
		t.Fatalf("CreateAccount with a short password: err = %v", err)
	}
}
