// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gopmgr/internal/sigma/domain"
)

// TestSigmaAppMethods_RejectSigmaProjectFromAnotherProjectRowInSameFile is
// the App-layer regression test for the residual risk left by the Sigma
// DMAIC FK-bug fix: every Sigma App method keyed by a bare Sigma project id
// resolved it against whatever GoPMgr project happened to be open, with no
// check that it actually belongs to that project.
//
// An earlier version of this test used two SEPARATE .gopmgr files (open A,
// create a Sigma project, close A, open B, create another) and asserted
// SigmaGetProject(A's sigma id) failed while B was open. That version
// passed, but proved nothing about the fix: CreateProject/OpenProject
// always create/open an independent SQLite database file per project
// (app_projects.go's CreateProject calls db.InitEncryptedDB on a freshly
// generated path every time), so A's Sigma project id never exists as a
// row in B's database at all -- the lookup fails on ordinary not-found,
// exactly the same way it would have failed before this fix existed. This
// was caught while building this test's replacement, not by inspection of
// the original.
//
// The real risk requires TWO project rows inside the SAME open database --
// the exact scenario the Risk #2 ledger entry's reverted trigger attempt
// confirmed is possible (UpsertProject has no schema-level single-row
// enforcement) and is already exercised by this package's own
// project_id-scoping tests elsewhere (agile/documents/audit_events/
// scenarios). This fixture reproduces that directly: open one project
// normally so GetProject()'s `SELECT ... FROM project LIMIT 1` resolves to
// it deterministically (the only row at that point), then insert a second
// project row via raw SQL into the SAME open database, and attach a Sigma
// project to THAT second row via the DB layer directly -- bypassing the
// App layer's SigmaCreateProject, which would itself resolve the now
// two-row-ambiguous "open project" via the same GetProject() call and
// misattribute the new Sigma project to the first row.
func TestSigmaAppMethods_RejectSigmaProjectFromAnotherProjectRowInSameFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Open Project")

	app.mu.RLock()
	conn := app.db.Conn
	dbConn := app.db
	app.mu.RUnlock()

	openProject, err := dbConn.GetProject()
	if err != nil {
		t.Fatalf("GetProject (precondition): %v", err)
	}

	var rowCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM project`).Scan(&rowCount); err != nil {
		t.Fatalf("count project rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("fixture precondition failed: %d project rows before seeding the second, want 1 -- the rest of this test's premise (which row GetProject() resolves to) depends on this", rowCount)
	}

	if _, err := conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "prj-other-row", "Other Row"); err != nil {
		t.Fatalf("seed second project row: %v", err)
	}

	foreign, err := dbConn.SigmaCreateProject(domain.Project{GopmgrProjectID: "prj-other-row", Title: "Foreign Sigma Project"})
	if err != nil {
		t.Fatalf("SigmaCreateProject (foreign, DB layer): %v", err)
	}

	own, err := app.SigmaCreateProject("Own Sigma Project", "", "green_belt")
	if err != nil {
		t.Fatalf("SigmaCreateProject (own, App layer): %v", err)
	}
	if own.GopmgrProjectID != openProject.ID {
		t.Fatalf("fixture precondition failed: own Sigma project's GopmgrProjectID = %q, want %q -- GetProject() must still resolve to the originally-opened row after the second row was inserted for this test's premise to hold", own.GopmgrProjectID, openProject.ID)
	}

	cases := []struct {
		name string
		call func(id string) error
	}{
		{"SigmaGetProject", func(id string) error { _, err := app.SigmaGetProject(id); return err }},
		{"SigmaSaveCharter", func(id string) error { return app.SigmaSaveCharter(domain.Charter{ProjectID: id}) }},
		{"SigmaGetCharter", func(id string) error { _, err := app.SigmaGetCharter(id); return err }},
		{"SigmaAdvancePhase", func(id string) error { return app.SigmaAdvancePhase(id, "analyze") }},
		{"SigmaCheckReadiness", func(id string) error { _, err := app.SigmaCheckReadiness(id, "define"); return err }},
		{"SigmaSaveFishbone", func(id string) error { return app.SigmaSaveFishbone(id, domain.FishboneData{}) }},
		{"SigmaGetFishbone", func(id string) error { _, err := app.SigmaGetFishbone(id); return err }},
		{"SigmaSaveSolutions", func(id string) error { return app.SigmaSaveSolutions(id, nil) }},
		{"SigmaGetSolutions", func(id string) error { _, err := app.SigmaGetSolutions(id); return err }},
		{"SigmaSaveControlPlan", func(id string) error { return app.SigmaSaveControlPlan(id, nil) }},
		{"SigmaGetControlPlan", func(id string) error { _, err := app.SigmaGetControlPlan(id); return err }},
		{"SigmaSaveSIPOC", func(id string) error { return app.SigmaSaveSIPOC(id, domain.SIPOCData{}) }},
		{"SigmaGetSIPOC", func(id string) error { _, err := app.SigmaGetSIPOC(id); return err }},
		{"SigmaSaveVoC", func(id string) error { return app.SigmaSaveVoC(id, domain.VoCData{}) }},
		{"SigmaGetVoC", func(id string) error { _, err := app.SigmaGetVoC(id); return err }},
		{"SigmaGetToolStatus", func(id string) error { _, err := app.SigmaGetToolStatus(id, "define"); return err }},
		{"SigmaExportProjectReport", func(id string) error { _, err := app.SigmaExportProjectReport(id); return err }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(foreign.ID); err == nil {
				t.Fatalf("%s(foreign sigma id): want an error, got nil", tc.name)
			}
		})
	}

	// Positive case: the guard must not block legitimate same-file
	// access. Only SigmaGetProject is asserted end-to-end here (its
	// return value is simple to check); the other 15 methods' happy
	// paths are already covered by this file's/sigma_crud_test.go's
	// existing tests, which call them against a freshly created Sigma
	// project in an ordinary single-project-row file and already pass --
	// proving the guard doesn't block them under normal conditions.
	if got, err := app.SigmaGetProject(own.ID); err != nil {
		t.Fatalf("SigmaGetProject(own sigma id): want success, got %v", err)
	} else if got.Title != "Own Sigma Project" {
		t.Fatalf("SigmaGetProject(own): got %+v, want Title=\"Own Sigma Project\"", got)
	}
}
