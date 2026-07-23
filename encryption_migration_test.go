// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"pmforge/internal/db"
)

// createPlaintextProjectForMigration writes a legacy plaintext project into
// the signed-in user's own projects dir (flat layout). It must live there
// because EncryptProjectAtRest/IsProjectEncrypted now confine to that dir.
func createPlaintextProjectForMigration(t *testing.T, app *App) string {
	t.Helper()
	user := app.requireUser()
	if user == nil {
		t.Fatal("createPlaintextProjectForMigration: no signed-in user")
		return ""
	}
	projectsDir := filepath.Join(user.DataDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	path := filepath.Join(projectsDir, "legacy.pmforge")
	legacy, err := db.InitDB(path)
	if err != nil {
		t.Fatalf("InitDB legacy: %v", err)
	}
	proj, err := legacy.UpsertProject(db.Project{
		Name:  "Legacy Plaintext",
		Owner: "alice",
	})
	if err != nil {
		t.Fatalf("UpsertProject legacy: %v", err)
	}
	if _, err := legacy.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      "cpm",
		Title:     "Legacy Chart",
		Data:      `{"nodes":[],"edges":[]}`,
	}); err != nil {
		t.Fatalf("SaveChart legacy: %v", err)
	}
	if _, err := legacy.SaveDocument(db.Document{
		ProjectID: proj.ID,
		Kind:      "charter_word",
		Title:     "Legacy Charter",
		Content:   `{"summary":"keep me"}`,
	}); err != nil {
		t.Fatalf("SaveDocument legacy: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}
	return path
}

func TestEncryptProjectAtRestRequiresWrappedRecoveryCodes(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path := createPlaintextProjectForMigration(t, app)
	if _, err := app.store.IssueRecoveryCodes("alice", nil); err != nil {
		t.Fatalf("IssueRecoveryCodes legacy: %v", err)
	}

	backup, err := app.EncryptProjectAtRest(path)
	if !errors.Is(err, ErrRecoveryCodesRequireReissue) {
		t.Fatalf("EncryptProjectAtRest err = %v, want ErrRecoveryCodesRequireReissue", err)
	}
	if backup != "" {
		t.Fatalf("backup = %q, want empty on blocked migration", backup)
	}
	encrypted, err := app.IsProjectEncrypted(path)
	if err != nil {
		t.Fatalf("IsProjectEncrypted: %v", err)
	}
	if encrypted {
		t.Fatal("blocked migration encrypted the project")
	}
}

func TestEncryptProjectAtRestMigratesAfterRecoveryCodeReissue(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	path := createPlaintextProjectForMigration(t, app)
	if encrypted, err := app.IsProjectEncrypted(path); err != nil || encrypted {
		t.Fatalf("IsProjectEncrypted before = %v, %v; want false, nil", encrypted, err)
	}
	if _, err := app.store.IssueRecoveryCodes("alice", nil); err != nil {
		t.Fatalf("IssueRecoveryCodes legacy: %v", err)
	}
	if _, err := app.IssueRecoveryCodes(); err != nil {
		t.Fatalf("IssueRecoveryCodes reissue: %v", err)
	}

	backup, err := app.EncryptProjectAtRest(path)
	if err != nil {
		t.Fatalf("EncryptProjectAtRest: %v", err)
	}
	if backup != path+".pre-encryption.bak" {
		t.Fatalf("backup = %q, want %q", backup, path+".pre-encryption.bak")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if encrypted, err := app.IsProjectEncrypted(path); err != nil || !encrypted {
		t.Fatalf("IsProjectEncrypted after = %v, %v; want true, nil", encrypted, err)
	}

	proj, err := app.OpenProject(path)
	if err != nil {
		t.Fatalf("OpenProject migrated: %v", err)
	}
	if proj.Name != "Legacy Plaintext" || proj.Owner != "alice" {
		t.Fatalf("project = %#v, want migrated legacy project", proj)
	}
	openDB := app.requireDB()
	charts, err := openDB.ListCharts(proj.ID, "")
	if err != nil {
		t.Fatalf("ListCharts migrated: %v", err)
	}
	if len(charts) != 1 || charts[0].Title != "Legacy Chart" {
		t.Fatalf("charts = %#v, want Legacy Chart preserved", charts)
	}
	docs, err := openDB.ListDocuments(proj.ID, "")
	if err != nil {
		t.Fatalf("ListDocuments migrated: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Legacy Charter" {
		t.Fatalf("documents = %#v, want Legacy Charter preserved", docs)
	}
}
