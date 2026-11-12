// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

// archiveOfPlaintextProject bundles a plaintext project, after running the
// given SQL on it, into a .pmba archive and returns the archive's path.
func archiveOfPlaintextProject(t *testing.T, statements ...string) string {
	t.Helper()
	dir := t.TempDir()
	d, err := db.InitDB(filepath.Join(dir, "source.gopmgr"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.UpsertProject(db.Project{Name: "Backed up"}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range statements {
		if _, err := d.Conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	archive := filepath.Join(dir, "backup.pmba")
	if err := d.CreateArchivalBundle(archive, nil); err != nil {
		t.Fatal(err)
	}
	return archive
}

func projectsFolderEntries(t *testing.T, app *App) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(app.currentSession().user.DataDir, "projects"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestRestoreRefusesAnArchiveWithAPlantedTrigger(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("ada", "Ada", "ada-strong-password", false); err != nil {
		t.Fatal(err)
	}
	acceptNoRecoveryCodes(t, app)
	before := projectsFolderEntries(t, app)

	archive := archiveOfPlaintextProject(t,
		`CREATE TRIGGER planted AFTER UPDATE ON project BEGIN DELETE FROM project; END`)
	_, err := app.restoreProjectArchiveFrom(archive, app.currentSession())
	if !errors.Is(err, db.ErrForeignSchemaObject) {
		t.Fatalf("restore = %v, want ErrForeignSchemaObject", err)
	}
	// The backup chose the trigger's name; the user must not see it.
	if strings.Contains(err.Error(), "planted") {
		t.Fatalf("the message shows a name from the backup: %q", err)
	}
	if after := projectsFolderEntries(t, app); len(after) != len(before) {
		t.Fatalf("a refused restore left files behind: before %v, after %v", before, after)
	}

	// The same archive without the trigger restores, so the refusal above
	// is the check's doing and not a broken fixture.
	restored, err := app.restoreProjectArchiveFrom(archiveOfPlaintextProject(t), app.currentSession())
	if err != nil {
		t.Fatalf("clean restore: %v", err)
	}
	if restored.Name != "Backed up (restored)" {
		t.Fatalf("restored project = %+v", restored)
	}
	// The read-only check must not leave a database or journal file that
	// other local accounts can read.
	files, err := os.ReadDir(filepath.Dir(restored.Path))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().IsRegular() && info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s has mode %v; want no group or other access", f.Name(), info.Mode().Perm())
		}
	}
}

// A backup can also change what later writes do without any trigger: by
// redefining one of GoPMgr's own tables. Here deleting a document would
// silently delete every stakeholder. The backup must be refused before the
// project is opened for writing.
func TestRestoreRefusesAnArchiveWithARedefinedTable(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("ada", "Ada", "ada-strong-password", false); err != nil {
		t.Fatal(err)
	}
	acceptNoRecoveryCodes(t, app)
	before := projectsFolderEntries(t, app)

	archive := archiveOfPlaintextProject(t,
		`PRAGMA foreign_keys = OFF`,
		`DROP TABLE stakeholders`,
		`CREATE TABLE stakeholders (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL,
			planted_ref TEXT REFERENCES documents(id) ON DELETE CASCADE)`)
	_, err := app.restoreProjectArchiveFrom(archive, app.currentSession())
	if !errors.Is(err, db.ErrForeignSchemaObject) {
		t.Fatalf("restore = %v, want ErrForeignSchemaObject", err)
	}
	if strings.Contains(err.Error(), "planted") || strings.Contains(err.Error(), "stakeholders") {
		t.Fatalf("the message shows a name from the backup: %q", err)
	}
	if after := projectsFolderEntries(t, app); len(after) != len(before) {
		t.Fatalf("a refused restore left files behind: before %v, after %v", before, after)
	}
}
