// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os"
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
		app.shutdown(context.Background())
	})
	return app
}

func TestCreateProjectEncryptsAndReopensWithSessionDEK(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
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
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	res, err := app.CreateProjectFromLaunchpad(
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
	if res.Project.Name != "Launchpad Secret" || len(res.Seeds) != 0 {
		t.Fatalf("project=%#v receipts=%#v", res.Project, res.Seeds)
	}
	encrypted, err := db.IsEncryptedFile(res.Path)
	if err != nil {
		t.Fatalf("IsEncryptedFile: %v", err)
	}
	if !encrypted {
		t.Fatalf("launchpad project %s is plaintext", res.Path)
	}
}

// TestCreateProjectFromLaunchpadActivatesProject verifies that the project is
// immediately usable after CreateProjectFromLaunchpad without a separate
// OpenProject call. Previously the backend closed the DB and returned without
// setting a.db, so every chart/document operation failed until the user
// manually closed and reopened the project.
func TestCreateProjectFromLaunchpadActivatesProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if _, err := app.CreateProjectFromLaunchpad(
		"Immediate Access",
		"db must be active",
		"software",
		"web",
		"agile",
		"US",
		nil,
	); err != nil {
		t.Fatalf("CreateProjectFromLaunchpad: %v", err)
	}

	// Call a method that requires requireDB() without calling OpenProject first.
	if _, err := app.ListCharts(""); err != nil {
		t.Fatalf("ListCharts immediately after launchpad creation (without OpenProject): %v", err)
	}
}

func TestOpenProjectRejectsDifferentUsersDEK(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	file, err := app.CreateProject("Alice Only", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := app.Logout(); err != nil {
		t.Fatalf("Logout alice: %v", err)
	}
	if _, err := app.CreateAccount("bob", "Bob", "bob-password", false); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}

	if _, err := app.OpenProject(file.Path); err == nil {
		t.Fatal("OpenProject accepted a project encrypted with another user's DEK")
	}
}

func TestOpenProjectPlaintextRequiresMigration(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	acc, err := app.CreateAccount("alice", "Alice", "alice-password", false)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	// A legacy plaintext project lives in the user's own projects dir (flat
	// layout). OpenProject confines to that dir, so the file must be there for
	// the migration-required path to be exercised rather than rejected.
	projectsDir := filepath.Join(acc.DataDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	path := filepath.Join(projectsDir, "legacy.pmforge")
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
