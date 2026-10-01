// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"gopmgr/internal/users"
)

// A password reset gives the user a new password for the same key: an
// encrypted project created before the reset opens after it.

// newResetTestApp signs alice in and creates one encrypted project,
// returning its path and a set of working recovery codes.
func newResetTestApp(t *testing.T) (app *App, projectPath string, codes []string) {
	t.Helper()
	app = newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	codes, err := app.IssueRecoveryCodes()
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	file, err := app.CreateProject("Secret Plan", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if encrypted, err := app.IsProjectEncrypted(file.Path); err != nil || !encrypted {
		t.Fatalf("project %s encrypted = %v (%v), want an encrypted project", file.Path, encrypted, err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	return app, file.Path, codes
}

func signInAndOpen(t *testing.T, app *App, password, projectPath string) {
	t.Helper()
	if _, err := app.Login("alice", password); err != nil {
		t.Fatalf("Login: %v", err)
	}
	proj, err := app.OpenProject(projectPath)
	if err != nil {
		t.Fatalf("OpenProject after the reset: %v", err)
	}
	if proj.Name != "Secret Plan" {
		t.Fatalf("opened project %q, want Secret Plan", proj.Name)
	}
}

func TestRecoveryResetKeepsEncryptedProjectsReadable(t *testing.T) {
	app, projectPath, codes := newResetTestApp(t)

	if err := app.ResetWithRecoveryCode("alice", codes[0], "reset-password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}
	if _, err := app.Login("alice", "alice-password"); err == nil {
		t.Fatal("the old password still signs in after a reset")
	}
	signInAndOpen(t, app, "reset-password", projectPath)
}

// The printable sheet tells the user each code works once and to tick it
// off: after one reset the used code is refused and every other code still
// resets the password and keeps the encrypted project readable.
func TestRecoveryResetLeavesTheOtherCodesWorking(t *testing.T) {
	app, projectPath, codes := newResetTestApp(t)

	if err := app.ResetWithRecoveryCode("alice", codes[0], "first-reset"); err != nil {
		t.Fatalf("first ResetWithRecoveryCode: %v", err)
	}
	signInAndOpen(t, app, "first-reset", projectPath)
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if err := app.ResetWithRecoveryCode("alice", codes[0], "reuse-attempt"); !errors.Is(err, users.ErrInvalidRecoveryCode) {
		t.Fatalf("reusing a code = %v, want ErrInvalidRecoveryCode", err)
	}
	if err := app.ResetWithRecoveryCode("alice", codes[len(codes)-1], "second-reset"); err != nil {
		t.Fatalf("a second code after the first was used: %v", err)
	}
	signInAndOpen(t, app, "second-reset", projectPath)
}

// A code from before ADR-001 cannot unlock the key, so the reset is
// refused and the account is left as it was: the old password still signs
// in and the project still opens.
func TestRecoveryResetWithLegacyCodeLeavesAccountUnchanged(t *testing.T) {
	app, projectPath, _ := newResetTestApp(t)
	legacy, err := app.store.IssueRecoveryCodes("alice", nil)
	if err != nil {
		t.Fatalf("issue legacy codes: %v", err)
	}

	err = app.ResetWithRecoveryCode("alice", legacy[0], "reset-password")
	if !errors.Is(err, users.ErrLegacyRecoveryCode) {
		t.Fatalf("legacy reset err = %v, want users.ErrLegacyRecoveryCode", err)
	}
	// RecoveryReset.svelte matches this text, and RecoveryReset.test.ts
	// feeds it the same string; reword all three together.
	const want = "This recovery code is from an older version of GoPMgr and can't unlock your encryption key, so nothing was changed."
	if err.Error() != want {
		t.Fatalf("legacy reset error text = %q, want %q", err.Error(), want)
	}
	if _, err := app.Login("alice", "reset-password"); err == nil {
		t.Fatal("the refused reset's new password signs in")
	}
	signInAndOpen(t, app, "alice-password", projectPath)
}
