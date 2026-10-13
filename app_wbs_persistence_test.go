// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gopmgr/internal/charts/dag"
	"gopmgr/internal/db"
)

// Complements deferred-commit component tests with the real Wails-facing
// methods and a file-backed database. This is not a native transport test.
func TestWBSChartAppCommitAndLayoutRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wbs.gopmgr")
	d, err := db.InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	project, err := d.UpsertProject(db.Project{Name: "WBS ordering"})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{db: d}
	chart := db.Chart{Kind: "wbs", Title: "WBS", Config: "{}"}
	// SaveChart is last-write-wins even with metadata from an earlier response.
	// An independent older preview arriving last really can replace newer data;
	// the frontend must prevent that schedule rather than assume revision checks.
	for _, title := range []string{"older", "newest", "older", "newest"} {
		chart.Data = `{"root":{"id":"r","title":"` + title + `","children":[]}}`
		saved, err := app.SaveChart(chart)
		if err != nil {
			t.Fatalf("SaveChart(%q): %v", title, err)
		}
		chart.ID = saved.ID
		stored, err := app.GetChart(saved.ID)
		if err != nil || stored.Data != chart.Data || stored.ProjectID != project.ID {
			t.Fatalf("committed chart = %+v, error %v, want %q in %s", stored, err, chart.Data, project.ID)
		}
		result, err := app.LayoutChart(saved.ID)
		if err != nil {
			t.Fatalf("LayoutChart(%q): %v", title, err)
		}
		body, err := json.Marshal(result.Body)
		if err != nil {
			t.Fatal(err)
		}
		var layout dag.Layout
		if err := json.Unmarshal(body, &layout); err != nil {
			t.Fatal(err)
		}
		if len(layout.Nodes) != 1 || layout.Nodes[0].Title != title || layout.Nodes[0].Number != "1" {
			t.Fatalf("layout after commit = %+v, want one numbered %q root", layout, title)
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	d = reopened
	app.db = d
	stored, err := app.GetChart(chart.ID)
	if err != nil || stored.Data != chart.Data {
		t.Fatalf("reopened chart = %+v, error %v, want %q", stored, err, chart.Data)
	}
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil || !report.Valid || report.CheckedEvents != 5 {
		t.Fatalf("reopened audit = %+v, error %v, want five valid events", report, err)
	}
}
