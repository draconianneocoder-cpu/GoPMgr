// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"gopmgr/internal/sigma/domain"
	"gopmgr/internal/sqlitedriver"
)

// newLegacySigmaProjectsFile creates a raw SQLite file with the ORIGINAL
// sigma_projects shape: id forced equal to the sole project row's id (both
// the primary key and the only foreign key target), no project_id column.
// This is the schema every real GoPMgr file had before this migration
// shipped -- a 1:1 shape that made every Sigma project create through the
// real UI fail outright, since that UI creates and lists many. Returns
// the file path; InitDB(path) then exercises Migrate()'s rebuild.
func newLegacySigmaProjectsFile(t *testing.T, projectID string, seedSigmaRow bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy-sigma.pmforge")
	conn, err := sql.Open(sqlitedriver.Name, path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("pragma foreign_keys: %v", err)
	}
	// Includes every column migrateLegacyColumns expects (industry,
	// budget_minor_units, etc.) so opening this file only exercises the
	// sigma_projects rebuild under test, not unrelated legacy-column
	// backfills this fixture isn't trying to simulate.
	if _, err := conn.Exec(`
		CREATE TABLE project (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			description   TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL DEFAULT 'planning',
			phase         TEXT NOT NULL DEFAULT 'initiation',
			start_date    TEXT NOT NULL DEFAULT '',
			end_date      TEXT NOT NULL DEFAULT '',
			budget        NUMERIC NOT NULL DEFAULT 0,
			budget_minor_units INTEGER NOT NULL DEFAULT 0,
			owner         TEXT NOT NULL DEFAULT '',
			industry      TEXT NOT NULL DEFAULT '',
			sub_category  TEXT NOT NULL DEFAULT '',
			methodology   TEXT NOT NULL DEFAULT '',
			country_code  TEXT NOT NULL DEFAULT 'US',
			time_zone     TEXT NOT NULL DEFAULT 'America/New_York',
			created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)
	`); err != nil {
		t.Fatalf("create legacy project table: %v", err)
	}
	if _, err := conn.Exec(`
		CREATE TABLE sigma_projects (
			id             TEXT PRIMARY KEY,
			title          TEXT NOT NULL,
			description    TEXT NOT NULL DEFAULT '',
			belt_level     TEXT NOT NULL DEFAULT 'green',
			phase          TEXT NOT NULL DEFAULT 'define',
			status         TEXT NOT NULL DEFAULT 'active',
			sponsor        TEXT NOT NULL DEFAULT '',
			process_owner  TEXT NOT NULL DEFAULT '',
			belt_lead      TEXT NOT NULL DEFAULT '',
			created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			FOREIGN KEY (id) REFERENCES project(id) ON DELETE CASCADE
		)
	`); err != nil {
		t.Fatalf("create legacy sigma_projects table: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, projectID, "Legacy Project"); err != nil {
		t.Fatalf("insert legacy project row: %v", err)
	}
	if seedSigmaRow {
		if _, err := conn.Exec(`INSERT INTO sigma_projects (id, title) VALUES (?, ?)`, projectID, "Pre-existing Sigma Row"); err != nil {
			t.Fatalf("insert legacy sigma_projects row: %v", err)
		}
	}
	return path
}

// TestMigrateSigmaProjectsSchema_RebuildsLegacyTableAndPreservesRows is
// the migration-path regression test: a file with the old FK-as-PK
// schema and an existing (successfully-inserted-before-any-UI-touched-
// it) sigma_projects row must, after opening once, gain project_id
// (backfilled from the single project row), keep the existing row's
// data intact, AND support a second, differently-titled Sigma project --
// the exact scenario the old schema could never satisfy.
func TestMigrateSigmaProjectsSchema_RebuildsLegacyTableAndPreservesRows(t *testing.T) {
	projectID := "legacy-p1"
	path := newLegacySigmaProjectsFile(t, projectID, true)

	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB (triggers Migrate): %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	cols, err := d.columnSet("sigma_projects")
	if err != nil {
		t.Fatalf("columnSet: %v", err)
	}
	if _, ok := cols["project_id"]; !ok {
		t.Fatal("sigma_projects missing project_id column after migration")
	}

	got, err := d.SigmaGetProject(projectID)
	if err != nil {
		t.Fatalf("SigmaGetProject after migration: %v", err)
	}
	if got.Title != "Pre-existing Sigma Row" {
		t.Errorf("Title = %q, want the pre-existing row preserved", got.Title)
	}
	if got.GopmgrProjectID != projectID {
		t.Errorf("GopmgrProjectID = %q, want %q (backfilled from the single project row)", got.GopmgrProjectID, projectID)
	}

	second, err := d.SigmaCreateProject(domain.Project{GopmgrProjectID: projectID, Title: "Second Project"})
	if err != nil {
		t.Fatalf("SigmaCreateProject after migration (second project): %v", err)
	}
	if second.ID == got.ID {
		t.Fatalf("second project reused the first project's ID (%q) -- IDs must be independent", second.ID)
	}

	all, err := d.SigmaListProjects()
	if err != nil {
		t.Fatalf("SigmaListProjects: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("SigmaListProjects returned %d rows, want 2: %+v", len(all), all)
	}

	rows, err := d.Conn.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		t.Error("foreign_key_check reported a violation after migration")
	}
}

// TestMigrateSigmaProjectsSchema_IdempotentAcrossRepeatedOpens proves the
// rebuild does not run again (and does not error) on a file that already
// has project_id -- every InitDB call runs Migrate() unconditionally, so
// a non-idempotent rebuild would corrupt or fail on a file's second open.
func TestMigrateSigmaProjectsSchema_IdempotentAcrossRepeatedOpens(t *testing.T) {
	projectID := "legacy-p2"
	path := newLegacySigmaProjectsFile(t, projectID, false)

	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB (first open): %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close after first open: %v", err)
	}

	d2, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB (second open, must be a no-op rebuild): %v", err)
	}
	t.Cleanup(func() { _ = d2.Close() })

	cols, err := d2.columnSet("sigma_projects")
	if err != nil {
		t.Fatalf("columnSet: %v", err)
	}
	if _, ok := cols["project_id"]; !ok {
		t.Fatal("sigma_projects missing project_id column after second open")
	}

	created, err := d2.SigmaCreateProject(domain.Project{GopmgrProjectID: projectID, Title: "Fresh Row"})
	if err != nil {
		t.Fatalf("SigmaCreateProject after repeated opens: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a generated ID")
	}
}
