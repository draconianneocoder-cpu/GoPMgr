// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Every path that produces an encrypted project must leave the user a way
// back in: working recovery codes, or an explicit acceptance this session.

func newReadinessApp(t *testing.T) *App {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return app
}

// encryptingPaths lists every App method that produces an encrypted
// project, each returning an error only.
func encryptingPaths(t *testing.T, app *App) map[string]func() error {
	t.Helper()
	return map[string]func() error{
		"CreateProject": func() error {
			_, err := app.CreateProject("Plan", "")
			return err
		},
		"CreateProjectFromLaunchpad": func() error {
			_, err := app.CreateProjectFromLaunchpad("Launchpad Plan", "", "software", "saas", "agile", "US", "America/New_York", nil)
			return err
		},
		"EncryptProjectAtRest": func() error {
			path := createPlaintextProjectForMigration(t, app)
			_, err := app.EncryptProjectAtRest(path)
			return err
		},
	}
}

// projectFolderEntries counts the entries directly in the signed-in user's
// projects folder, so a refused create is seen to leave no project folder.
func projectFolderEntries(t *testing.T, app *App) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(app.requireUser().DataDir, "projects"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read projects folder: %v", err)
	}
	return len(entries)
}

// encryptedProjectCount counts encrypted project files in the signed-in user's projects folder.
func encryptedProjectCount(t *testing.T, app *App) int {
	t.Helper()
	u := app.requireUser()
	root := filepath.Join(u.DataDir, "projects")
	n := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if encrypted, err := app.IsProjectEncrypted(p); err == nil && encrypted {
			n++
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("walk projects: %v", err)
	}
	return n
}

func TestEncryptingPathsRefuseWithoutRecoveryCodesOrAcceptance(t *testing.T) {
	for name := range encryptingPaths(t, newReadinessApp(t)) {
		t.Run(name, func(t *testing.T) {
			app := newReadinessApp(t)
			run := encryptingPaths(t, app)[name]

			before := projectFolderEntries(t, app)
			if err := run(); !errors.Is(err, ErrRecoveryCodesMissing) {
				t.Fatalf("%s with no codes: err = %v, want ErrRecoveryCodesMissing", name, err)
			}
			if n := encryptedProjectCount(t, app); n != 0 {
				t.Fatalf("%s refused but left %d encrypted project(s)", name, n)
			}
			if name != "EncryptProjectAtRest" {
				if after := projectFolderEntries(t, app); after != before {
					t.Fatalf("%s refused but changed the projects folder: %d entries before, %d after", name, before, after)
				}
			}

			if err := app.AcceptEncryptionWithoutRecoveryCodes(); err != nil {
				t.Fatalf("AcceptEncryptionWithoutRecoveryCodes: %v", err)
			}
			if err := run(); err != nil {
				t.Fatalf("%s after accepting: %v", name, err)
			}
			if n := encryptedProjectCount(t, app); n == 0 {
				t.Fatalf("%s after accepting produced no encrypted project", name)
			}
		})
	}
}

func TestEncryptingPathsRefuseLegacyCodesEvenWhenAccepted(t *testing.T) {
	for name := range encryptingPaths(t, newReadinessApp(t)) {
		t.Run(name, func(t *testing.T) {
			app := newReadinessApp(t)
			if _, err := app.store.IssueRecoveryCodes("alice", nil); err != nil {
				t.Fatalf("issue legacy codes: %v", err)
			}
			if err := app.AcceptEncryptionWithoutRecoveryCodes(); err != nil {
				t.Fatalf("AcceptEncryptionWithoutRecoveryCodes: %v", err)
			}
			if err := encryptingPaths(t, app)[name](); !errors.Is(err, ErrRecoveryCodesRequireReissue) {
				t.Fatalf("%s with legacy codes: err = %v, want ErrRecoveryCodesRequireReissue", name, err)
			}
			if n := encryptedProjectCount(t, app); n != 0 {
				t.Fatalf("%s refused but left %d encrypted project(s)", name, n)
			}
		})
	}
}

func TestEncryptingPathsNeedNoAcceptanceWithWorkingCodes(t *testing.T) {
	for name := range encryptingPaths(t, newReadinessApp(t)) {
		t.Run(name, func(t *testing.T) {
			app := newReadinessApp(t)
			if _, err := app.IssueRecoveryCodes(); err != nil {
				t.Fatalf("IssueRecoveryCodes: %v", err)
			}
			if err := encryptingPaths(t, app)[name](); err != nil {
				t.Fatalf("%s with working codes: %v", name, err)
			}
		})
	}
}

// TestAcceptanceLastsOnlyForTheSessionThatGaveIt signs out and back in, as
// the same user and as another, after accepting.
func TestAcceptanceLastsOnlyForTheSessionThatGaveIt(t *testing.T) {
	app := newReadinessApp(t)
	if _, err := app.CreateAccount("bob", "Bob", "bob-password", false); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	status := func() RecoveryCodeStatusWire {
		t.Helper()
		s, err := app.RecoveryCodeStatus()
		if err != nil {
			t.Fatalf("RecoveryCodeStatus: %v", err)
		}
		return s
	}

	if status().EncryptionReady {
		t.Fatal("encryption_ready with no codes and no acceptance")
	}
	if err := app.AcceptEncryptionWithoutRecoveryCodes(); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if !status().EncryptionReady {
		t.Fatal("encryption_ready false after accepting")
	}

	for _, who := range []struct{ name, password string }{{"bob", "bob-password"}, {"alice", "alice-password"}} {
		if err := app.Logout(); err != nil {
			t.Fatalf("Logout: %v", err)
		}
		if _, err := app.Login(who.name, who.password); err != nil {
			t.Fatalf("Login %s: %v", who.name, err)
		}
		if _, err := app.CreateProject("After sign-in", ""); !errors.Is(err, ErrRecoveryCodesMissing) {
			t.Fatalf("CreateProject as %s after an earlier session accepted: err = %v, want ErrRecoveryCodesMissing", who.name, err)
		}
		if status().EncryptionReady {
			t.Fatalf("encryption_ready for %s carried over from an earlier session", who.name)
		}
	}
}

func TestAcceptEncryptionWithoutRecoveryCodesNeedsASession(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if err := app.AcceptEncryptionWithoutRecoveryCodes(); err == nil {
		t.Fatal("AcceptEncryptionWithoutRecoveryCodes with no session: got nil, want error")
	}
}
