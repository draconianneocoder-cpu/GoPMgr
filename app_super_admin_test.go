// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

// superApp returns an app signed in as alice, the super administrator,
// with bob a subordinate administrator who has signed in, and carol a
// standard account.
func superApp(t *testing.T) *App {
	t.Helper()
	app := newAdminTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", escrowTestPassword, true); err != nil {
		t.Fatalf("CreateAccount(alice): %v", err)
	}
	for _, name := range []string{"bob", "carol"} {
		if _, err := app.CreateAccount(name, name, escrowTestPassword, false); err != nil {
			t.Fatalf("CreateAccount(%s): %v", name, err)
		}
	}
	if err := app.AdminSetUserRole("bob", true); err != nil {
		t.Fatalf("promote bob: %v", err)
	}
	return app
}

func TestAppSubordinatesManageStandardAccountsOnly(t *testing.T) {
	app := superApp(t)
	if roles, err := app.AdminRoles(); err != nil || roles.Super != "alice" {
		t.Fatalf("AdminRoles = %+v, %v; want alice", roles, err)
	}
	switchUser(t, app, "bob")

	wantNotSuper := "only the super administrator can change administrators or open users' data"
	for name, call := range map[string]func() error{
		"promote":           func() error { return app.AdminSetUserRole("carol", true) },
		"demote the super":  func() error { return app.AdminSetUserRole("alice", false) },
		"disable the super": func() error { return app.AdminSetUserDisabled("alice", true) },
		"delete the super":  func() error { return app.AdminPurgeUser("alice", "alice") },
	} {
		if err := call(); err == nil || err.Error() != wantNotSuper {
			t.Errorf("%s by a subordinate: err = %v, want %q", name, err, wantNotSuper)
		}
	}
	if _, err := app.CreateAccount("zed", "Zed", escrowTestPassword, true); err == nil || err.Error() != "only the super administrator can create administrator accounts" {
		t.Errorf("subordinate creating an administrator: err = %v", err)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil || err.Error() != "only the super administrator can open users' data" {
		t.Errorf("subordinate opening data: err = %v", err)
	}
	if _, err := app.AdminHandOverSuper("carol"); err == nil || err.Error() != "only the super administrator can hand over the role" {
		t.Errorf("subordinate handing over: err = %v", err)
	}
	if _, err := app.AdminIssueRecoveryCodes("alice", escrowTestPassword); err == nil || err.Error() != wantNotSuper {
		t.Errorf("subordinate issuing an administrator's codes: err = %v", err)
	}

	// Standard accounts are theirs to manage.
	if _, err := app.CreateAccount("dave", "Dave", escrowTestPassword, false); err != nil {
		t.Fatalf("subordinate creating a standard account: %v", err)
	}
	if err := app.AdminSetUserDisabled("carol", true); err != nil {
		t.Fatalf("subordinate disabling a standard account: %v", err)
	}
	if err := app.AdminSetUserDisabled("carol", false); err != nil {
		t.Fatalf("subordinate enabling a standard account: %v", err)
	}
}

func TestAppHandOverMovesTheRoleAndEndsOpenAccess(t *testing.T) {
	app := superApp(t)
	if err := app.AdminOpenUserData("carol", "checking"); err != nil {
		t.Fatalf("AdminOpenUserData: %v", err)
	}
	app.mu.RLock()
	key := app.access.dek
	app.mu.RUnlock()

	result, err := app.AdminHandOverSuper("bob")
	if err != nil || !result.KeyPassed {
		t.Fatalf("AdminHandOverSuper(bob) = %+v, %v; want the key passed", result, err)
	}
	app.mu.RLock()
	access := app.access
	app.mu.RUnlock()
	if access != nil || !isZero(key) {
		t.Fatal("the former super administrator's open access was not ended")
	}
	if _, err := app.AdminListUserProjects(); err == nil {
		t.Fatal("the former super administrator can still read the account they had open")
	}
	if roles, _ := app.AdminRoles(); roles.Super != "bob" {
		t.Fatalf("super administrator = %q, want bob", roles.Super)
	}
	if err := app.AdminOpenUserData("carol", "checking"); err == nil {
		t.Fatal("the former super administrator opened data")
	}

	switchUser(t, app, "bob")
	if err := app.AdminOpenUserData("carol", "checking"); err != nil {
		t.Fatalf("new super administrator opening data: %v", err)
	}
	want := "carol must be an administrator who can sign in"
	if _, err := app.AdminHandOverSuper("carol"); err == nil || err.Error() != want {
		t.Fatalf("hand-over to a standard account: err = %v, want %q", err, want)
	}
}

func isZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
