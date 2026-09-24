// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gopmgr/internal/users"
)

// TestAdminIssueRecoveryCodesForCreatedUser locks in the admin-created-account
// recovery gap fix: an admin can mint recovery codes for an account they
// provisioned, so it has the same recovery footing as a self-registered user.
func TestAdminIssueRecoveryCodesForCreatedUser(t *testing.T) {
	app := newEncryptionProjectTestApp(t)

	// First account is the admin and auto-signs-in as the session user.
	if _, err := app.CreateAccount("admin", "Admin", "admin-password", true); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	// Admin provisions a second account (admin session stays active).
	if _, err := app.CreateAccount("bob", "Bob", "bob-password-12", false); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	codes, err := app.AdminIssueRecoveryCodes("bob", "bob-password-12")
	if err != nil {
		t.Fatalf("AdminIssueRecoveryCodes: %v", err)
	}
	if len(codes) != users.RecoveryCodeCount {
		t.Fatalf("got %d codes, want %d", len(codes), users.RecoveryCodeCount)
	}

	// Wrong password must be rejected.
	if _, err := app.AdminIssueRecoveryCodes("bob", "not-the-password"); err == nil {
		t.Fatal("AdminIssueRecoveryCodes accepted a wrong password")
	}
}

// TestAdminIssueRecoveryCodesRequiresAdmin verifies a non-admin session cannot
// mint recovery codes for an account.
func TestAdminIssueRecoveryCodesRequiresAdmin(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	// The first account is always an administrator, so an admin creates
	// Carol as a standard user and she signs in on her own.
	if _, err := app.CreateAccount("admin", "Admin", "admin-password", true); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := app.CreateAccount("carol", "Carol", "carol-password", false); err != nil {
		t.Fatalf("create carol: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout admin: %v", err)
	}
	if _, err := app.Login("carol", "carol-password"); err != nil {
		t.Fatalf("Login carol: %v", err)
	}
	if _, err := app.AdminIssueRecoveryCodes("carol", "carol-password"); err == nil {
		t.Fatal("non-admin was allowed to issue recovery codes")
	}
}
