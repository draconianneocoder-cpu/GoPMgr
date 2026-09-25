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

// TestRecoveryCodeRotation_ConfirmStoresAndSignOutDiscards walks the App
// Settings flow: prepare, then confirm or sign out.
func TestRecoveryCodeRotation_ConfirmStoresAndSignOutDiscards(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	oldCodes, err := app.IssueRecoveryCodes()
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}

	if _, err := app.PrepareRecoveryCodes("wrong-password"); err == nil || err.Error() != "current password is incorrect" {
		t.Fatalf("PrepareRecoveryCodes with a wrong password: err = %v", err)
	}
	if err := app.ConfirmRecoveryCodes(); err == nil {
		t.Fatal("ConfirmRecoveryCodes with nothing prepared: got nil, want error")
	}

	// Prepared, then discarded by signing out: the old codes stay.
	if _, err := app.PrepareRecoveryCodes("alice-password"); err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := app.Login("alice", "alice-password"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := app.ConfirmRecoveryCodes(); err == nil {
		t.Fatal("ConfirmRecoveryCodes after signing out and back in: got nil, want error")
	}

	// Prepared, then discarded explicitly.
	if _, err := app.PrepareRecoveryCodes("alice-password"); err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	app.DiscardRecoveryCodes()
	if err := app.ConfirmRecoveryCodes(); err == nil {
		t.Fatal("ConfirmRecoveryCodes after a discard: got nil, want error")
	}
	status, err := app.RecoveryCodeStatus()
	if err != nil || status.Unused != 8 || status.Total != 8 || status.Legacy {
		t.Fatalf("status before confirming = %+v, %v; want the old 8 unused", status, err)
	}

	// Prepared and confirmed: the new codes replace the old ones.
	newCodes, err := app.PrepareRecoveryCodes("alice-password")
	if err != nil {
		t.Fatalf("PrepareRecoveryCodes: %v", err)
	}
	if err := app.ConfirmRecoveryCodes(); err != nil {
		t.Fatalf("ConfirmRecoveryCodes: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if err := app.ResetWithRecoveryCode("alice", oldCodes[0], "after-old-code"); err == nil {
		t.Fatal("an old code still reset the password after new codes were confirmed")
	}
	if err := app.ResetWithRecoveryCode("alice", newCodes[0], "after-new-code"); err != nil {
		t.Fatalf("a new code did not reset the password: %v", err)
	}
}

// TestIssueRecoveryCodesRefusesASessionWithoutAKey checks codes are never
// issued without the DEK: they would reset the password with a new key and
// orphan encrypted projects.
func TestIssueRecoveryCodesRefusesASessionWithoutAKey(t *testing.T) {
	app := newAdminTestApp(t)
	if _, err := app.store.CreateAccount("alice", "Alice", "alice-password", true); err != nil {
		t.Fatalf("store.CreateAccount: %v", err)
	}
	signIn(t, app, "alice") // a session with no DEK
	if _, err := app.IssueRecoveryCodes(); err == nil {
		t.Fatal("IssueRecoveryCodes without a session key: got nil, want error")
	}
	legacy, err := app.store.HasLegacyRecoveryCodeWraps("alice")
	if err != nil || legacy {
		t.Fatalf("legacy codes after a refused issue = %v, %v; want none", legacy, err)
	}
}
