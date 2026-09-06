// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"gopmgr/internal/db"
)

func TestStakeholderAppMethodsRejectForeignProjectRowsInSameFile(t *testing.T) {
	app, d, openProject := newSameFileProjectScopeFixture(t)
	foreign, err := d.SaveStakeholder(db.Stakeholder{ProjectID: "prj-foreign", Name: "Foreign"})
	if err != nil {
		t.Fatalf("seed foreign stakeholder: %v", err)
	}

	if _, err := app.SaveStakeholder(foreign); !errors.Is(err, db.ErrNoStakeholder) {
		t.Fatalf("SaveStakeholder(foreign) error = %v, want db.ErrNoStakeholder", err)
	}
	if err := app.DeleteStakeholder(foreign.ID); err != nil {
		t.Fatalf("DeleteStakeholder(foreign) error = %v, want nil no-op", err)
	}
	if _, err := d.GetStakeholder(foreign.ID); err != nil {
		t.Fatalf("foreign stakeholder was mutated or deleted: %v", err)
	}

	saved, err := app.SaveStakeholder(db.Stakeholder{ProjectID: "prj-foreign", Name: "Owned"})
	if err != nil {
		t.Fatalf("SaveStakeholder(new): %v", err)
	}
	if saved.ProjectID != openProject.ID {
		t.Fatalf("SaveStakeholder ProjectID = %q, want %q", saved.ProjectID, openProject.ID)
	}
	if err := app.DeleteStakeholder("stakeholder-missing"); err != nil {
		t.Fatalf("DeleteStakeholder(missing) error = %v, want nil no-op", err)
	}
}

func TestResourceCalendarAppMethodsRejectForeignProjectRowsInSameFile(t *testing.T) {
	app, d, openProject := newSameFileProjectScopeFixture(t)
	foreign, err := d.SaveResourceCalendar(db.ResourceCalendar{ProjectID: "prj-foreign", Resource: "Foreign"})
	if err != nil {
		t.Fatalf("seed foreign resource calendar: %v", err)
	}

	if _, err := app.SaveResourceCalendar(foreign); !errors.Is(err, db.ErrNoResourceCalendar) {
		t.Fatalf("SaveResourceCalendar(foreign) error = %v, want db.ErrNoResourceCalendar", err)
	}
	if err := app.DeleteResourceCalendar(foreign.ID); err != nil {
		t.Fatalf("DeleteResourceCalendar(foreign) error = %v, want nil no-op", err)
	}
	got, err := d.GetResourceCalendar(foreign.ID)
	if err != nil {
		t.Fatalf("foreign resource calendar was moved or deleted: %v", err)
	}
	if got.ProjectID != "prj-foreign" {
		t.Fatalf("foreign resource calendar moved to project %q", got.ProjectID)
	}

	saved, err := app.SaveResourceCalendar(db.ResourceCalendar{ProjectID: "prj-foreign", Resource: "Owned"})
	if err != nil {
		t.Fatalf("SaveResourceCalendar(new): %v", err)
	}
	if saved.ProjectID != openProject.ID {
		t.Fatalf("SaveResourceCalendar ProjectID = %q, want %q", saved.ProjectID, openProject.ID)
	}
	if err := app.DeleteResourceCalendar("calendar-missing"); err != nil {
		t.Fatalf("DeleteResourceCalendar(missing) error = %v, want nil no-op", err)
	}
}

func newSameFileProjectScopeFixture(t *testing.T) (*App, *db.Database, db.Project) {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")
	app.mu.RLock()
	d := app.db
	app.mu.RUnlock()
	openProject, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-foreign", "Foreign Project"); err != nil {
		t.Fatalf("seed foreign project row: %v", err)
	}
	return app, d, openProject
}
