// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"pmforge/internal/agile"
	"pmforge/internal/db"
)

// mustOpenProject is a test helper that creates a project and opens it so
// that a.db is non-nil.  It returns the path of the project file.
func mustOpenProject(t *testing.T, app *App, name string) string {
	t.Helper()
	pf, err := app.CreateProject(name, "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := app.OpenProject(pf.Path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	return pf.Path
}

// auditCount queries the audit_log table in the currently-open project DB and
// returns the number of rows matching the given action and targetID.
func auditCount(t *testing.T, app *App, action, targetID string) int {
	t.Helper()
	app.mu.RLock()
	conn := app.db.Conn
	app.mu.RUnlock()
	var n int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM audit_log WHERE action = ? AND target_id = ?`,
		action, targetID,
	).Scan(&n); err != nil {
		t.Fatalf("audit_log query: %v", err)
	}
	return n
}

// TestCloneOpenProject_DataSurvivesSnapshot verifies that data committed to
// the open project is present in the clone.  This is the correctness invariant
// of the WAL fix: VACUUM INTO produces a fully-checkpointed snapshot, so any
// chart saved before the clone must be readable in it.  A raw copyFile could
// miss data that was committed to the WAL but not yet checkpointed to the main
// file.
func TestCloneOpenProject_DataSurvivesSnapshot(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	srcPath := mustOpenProject(t, app, "Source Plan")

	// Commit a chart to the open project (this write goes into the WAL).
	chart, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Snapshot WBS"})
	if err != nil {
		t.Fatalf("SaveChart before clone: %v", err)
	}

	// Clone while the project is open.  a.db != nil && samePath(a.dbPath, src)
	// triggers the VACUUM INTO path.
	clone, err := app.CloneProject(srcPath)
	if err != nil {
		t.Fatalf("CloneProject (open project): %v", err)
	}
	if clone.Path == srcPath {
		t.Fatal("clone path equals source path")
	}
	if clone.Name != "Source Plan copy" {
		t.Fatalf("clone name = %q, want %q", clone.Name, "Source Plan copy")
	}

	// Open the clone as the active project and verify the chart is present.
	// If VACUUM INTO missed WAL data, ListCharts would return empty here.
	if _, err := app.OpenProject(clone.Path); err != nil {
		t.Fatalf("OpenProject on clone: %v", err)
	}
	charts, err := app.ListCharts("")
	if err != nil {
		t.Fatalf("ListCharts in clone: %v", err)
	}
	found := false
	for _, c := range charts {
		if c.ID == chart.ID && c.Title == chart.Title {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chart %q (id %s) not found in clone; charts = %v", chart.Title, chart.ID, charts)
	}
}

// TestDeleteChart_WritesAuditLog confirms that DeleteChart records a
// delete_chart entry in the audit_log before removing the chart.
func TestDeleteChart_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	chart, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Audit WBS"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if err := app.DeleteChart(chart.ID); err != nil {
		t.Fatalf("DeleteChart: %v", err)
	}

	if n := auditCount(t, app, "delete_chart", chart.ID); n != 1 {
		t.Fatalf("audit_log delete_chart count = %d, want 1", n)
	}
}

// TestDeleteDocument_WritesAuditLog confirms that DeleteDocument records a
// delete_document entry in the audit_log before removing the document.
func TestDeleteDocument_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	doc, err := app.NewDocument("charter_word", "Audit Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	if err := app.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	if n := auditCount(t, app, "delete_document", doc.ID); n != 1 {
		t.Fatalf("audit_log delete_document count = %d, want 1", n)
	}
}

// TestDeleteWorkItem_WritesAuditLog confirms that DeleteWorkItem records a
// delete_work_item entry in the audit_log before removing the work item.
func TestDeleteWorkItem_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	if err := app.SetAgileEnabled(true); err != nil {
		t.Fatalf("SetAgileEnabled: %v", err)
	}
	bwc, err := app.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	defaultState := ""
	if len(bwc.Columns) > 0 {
		defaultState = bwc.Columns[0].ID
	}

	wi, err := app.SaveWorkItem(agile.WorkItem{
		Type:  agile.WorkItemStory,
		Title: "Audit Story",
		State: defaultState,
	})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}

	if err := app.DeleteWorkItem(wi.ID); err != nil {
		t.Fatalf("DeleteWorkItem: %v", err)
	}

	if n := auditCount(t, app, "delete_work_item", wi.ID); n != 1 {
		t.Fatalf("audit_log delete_work_item count = %d, want 1", n)
	}
}
