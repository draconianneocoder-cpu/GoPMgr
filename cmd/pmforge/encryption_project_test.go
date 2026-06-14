// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"pmforge/internal/db"
	"pmforge/internal/users"
)

func newEncryptionProjectTestApp(t *testing.T) *App {
	t.Helper()
	store, err := users.Open(filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	app := &App{store: store}
	t.Cleanup(func() {
		app.shutdown(nil)
	})
	return app
}

func TestCreateProjectEncryptsAndReopensWithSessionDEK(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	file, err := app.CreateProject("Secret Plan", "keep private")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	encrypted, err := db.IsEncryptedFile(file.Path)
	if err != nil {
		t.Fatalf("IsEncryptedFile: %v", err)
	}
	if !encrypted {
		t.Fatalf("created project %s is plaintext", file.Path)
	}

	proj, err := app.OpenProject(file.Path)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if proj.Name != "Secret Plan" || proj.Owner != "alice" {
		t.Fatalf("project = %#v, want Secret Plan owned by alice", proj)
	}
}

func TestCreateProjectFromLaunchpadEncryptsProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	proj, receipts, path, err := app.CreateProjectFromLaunchpad(
		"Launchpad Secret",
		"seeded and encrypted",
		"software",
		"saas",
		"agile",
		"US",
		nil,
	)
	if err != nil {
		t.Fatalf("CreateProjectFromLaunchpad: %v", err)
	}
	if proj.Name != "Launchpad Secret" || len(receipts) != 0 {
		t.Fatalf("project=%#v receipts=%#v", proj, receipts)
	}
	encrypted, err := db.IsEncryptedFile(path)
	if err != nil {
		t.Fatalf("IsEncryptedFile: %v", err)
	}
	if !encrypted {
		t.Fatalf("launchpad project %s is plaintext", path)
	}
}

func TestOpenProjectRejectsDifferentUsersDEK(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password"); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	file, err := app.CreateProject("Alice Only", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout alice: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "bob-password"); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}

	if _, err := app.OpenProject(file.Path); err == nil {
		t.Fatal("OpenProject accepted a project encrypted with another user's DEK")
	}
}

func TestOpenProjectPlaintextRequiresMigration(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path := filepath.Join(t.TempDir(), "legacy.pmforge")
	legacy, err := db.InitDB(path)
	if err != nil {
		t.Fatalf("InitDB legacy: %v", err)
	}
	if _, err := legacy.UpsertProject(db.Project{Name: "Legacy Plaintext"}); err != nil {
		t.Fatalf("UpsertProject legacy: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	if _, err := app.OpenProject(path); !errors.Is(err, ErrProjectRequiresEncryptionMigration) {
		t.Fatalf("OpenProject plaintext err = %v, want ErrProjectRequiresEncryptionMigration", err)
	}
}
