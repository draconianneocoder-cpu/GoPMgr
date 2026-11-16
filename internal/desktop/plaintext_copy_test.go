// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"errors"
	"os"
	"testing"

	"gopmgr/internal/db"
)

// The unencrypted copy can be deleted only once the encrypted project has
// opened in this session, and deleting it does not disturb the open project.
func TestDeletePlaintextCopyNeedsTheEncryptedProjectOpen(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-password", false); err != nil {
		t.Fatal(err)
	}
	path := createPlaintextProjectForMigration(t, app)
	acceptNoRecoveryCodes(t, app)
	backup, err := app.EncryptProjectAtRest(path)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypting leaves the project closed: no offer, and no deletion.
	if got, err := app.PlaintextCopyPath(path); err != nil || got != "" {
		t.Fatalf("PlaintextCopyPath before opening = %q, %v; want none", got, err)
	}
	if _, err := app.DeletePlaintextCopy(path); !errors.Is(err, errOpenToDeleteCopy) {
		t.Fatalf("DeletePlaintextCopy before opening = %v, want errOpenToDeleteCopy", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("the copy went before the project opened: %v", err)
	}

	if _, err := app.OpenProject(path); err != nil {
		t.Fatal(err)
	}
	if got, err := app.PlaintextCopyPath(path); err != nil || got != backup {
		t.Fatalf("PlaintextCopyPath once open = %q, %v; want %q", got, err, backup)
	}
	proj, err := app.db.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	// A write on the open connection leaves WAL the deletion's read-only
	// check has to read past.
	if _, err := app.db.SaveChart(db.Chart{ProjectID: proj.ID, Kind: "cpm", Title: "Before", Data: `{"nodes":[],"edges":[]}`}); err != nil {
		t.Fatal(err)
	}

	removed, err := app.DeletePlaintextCopy(path)
	if err != nil || removed != backup {
		t.Fatalf("DeletePlaintextCopy = %q, %v; want %q", removed, err, backup)
	}
	if _, err := os.Stat(backup); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the copy is still there: %v", err)
	}
	if got, err := app.PlaintextCopyPath(path); err != nil || got != "" {
		t.Fatalf("PlaintextCopyPath after deleting = %q, %v; want none", got, err)
	}
	// Asked again, a copy already gone counts as deleted, not as an error.
	if again, err := app.DeletePlaintextCopy(path); err != nil || again != backup {
		t.Fatalf("DeletePlaintextCopy again = %q, %v; want %q", again, err, backup)
	}
	if _, err := app.db.SaveChart(db.Chart{ProjectID: proj.ID, Kind: "cpm", Title: "After", Data: `{"nodes":[],"edges":[]}`}); err != nil {
		t.Fatalf("the open project stopped working after the deletion: %v", err)
	}
}
