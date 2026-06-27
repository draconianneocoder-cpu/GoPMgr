// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
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
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
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

func TestOpenProjectComplianceModeRejectsTamperedAuditChain(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	file, err := app.CreateProject("Compliance Project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	project, err := app.OpenProject(file.Path)
	if err != nil {
		t.Fatalf("OpenProject initial: %v", err)
	}
	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	settings.ComplianceMode = true
	if err := app.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings compliance mode: %v", err)
	}
	if _, err := app.db.Conn.Exec(
		`UPDATE audit_events SET after_canonical_json = ? WHERE project_id = ? AND sequence_number = 1`,
		`{"name":"tampered"}`,
		project.ID,
	); err != nil {
		t.Fatalf("tamper audit chain: %v", err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}

	if _, err := app.OpenProject(file.Path); err == nil || !strings.Contains(err.Error(), "audit verification failed") {
		t.Fatalf("OpenProject tampered err = %v, want audit verification failure", err)
	}
}

func TestAppendProjectDeleteAuditWritesDeleteEvent(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	file, err := app.CreateProject("Delete Audit", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := app.appendProjectDeleteAudit(file.Path, "alice"); err != nil {
		t.Fatalf("appendProjectDeleteAudit: %v", err)
	}
	project, err := app.OpenProject(file.Path)
	if err != nil {
		t.Fatalf("OpenProject after delete audit: %v", err)
	}
	var count int
	if err := app.db.Conn.QueryRow(
		`SELECT COUNT(*) FROM audit_events WHERE project_id = ? AND event_type = 'project.delete'`,
		project.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count delete audit events: %v", err)
	}
	if count != 1 {
		t.Fatalf("delete audit events = %d, want 1", count)
	}
}
