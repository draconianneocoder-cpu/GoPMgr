// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"

	"pmforge/internal/users"
)

func newAdminTestApp(t *testing.T) *App {
	t.Helper()
	store, err := users.Open(filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	app := &App{store: store}
	t.Cleanup(func() { app.shutdown(nil) })
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

func TestBecomeAdmin_SucceedsWhenNoAdminExists(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	signIn(t, app, "alice")

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

func TestAdminDeleteUser_CannotDeleteSelf(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	signIn(t, app, "alice")

	err := app.AdminDeleteUser("alice")
	if err == nil {
		t.Fatal("AdminDeleteUser self: got nil, want error")
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

func TestAdminDeleteUser_RejectsNonAdmin(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	signIn(t, app, "bob")

	if err := app.AdminDeleteUser("alice"); err == nil {
		t.Fatal("AdminDeleteUser as non-admin: got nil, want error")
	}
}
