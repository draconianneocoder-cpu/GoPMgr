// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"pmforge/internal/db"
)

func newResourceTestApp(t *testing.T) (*App, *db.Database, db.Chart) {
	t.Helper()

	d, err := db.InitDB(filepath.Join(t.TempDir(), "resources.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	proj, err := d.UpsertProject(db.Project{
		ID:        "project-1",
		Name:      "Resource Test",
		StartDate: "2026-06-01", // Monday
	})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	// A and B both need alice full-time, no precedence between them:
	// classic contention the leveller must serialise.
	data := `{
		"nodes": [
			{"id":"A","label":"A","duration":2,"assignments":[{"resource":"alice"}]},
			{"id":"B","label":"B","duration":2,"assignments":[{"resource":"alice"}]}
		],
		"edges": []
	}`
	c, err := d.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      "cpm",
		Title:     "Schedule",
		Data:      data,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	return &App{db: d}, d, c
}

func TestLevelChartResourcesPinsDelayedTask(t *testing.T) {
	app, d, c := newResourceTestApp(t)

	pinned, err := app.LevelChartResources(c.ID)
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if pinned != 1 {
		t.Fatalf("pinned = %d, want 1 (only the delayed task)", pinned)
	}

	got, err := d.GetChart(c.ID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	var doc struct {
		Nodes []struct {
			ID             string `json:"id"`
			Constraint     string `json:"constraint"`
			ConstraintDate string `json:"constraint_date"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
		t.Fatalf("unmarshal persisted doc: %v", err)
	}

	byID := map[string]struct {
		Constraint, Date string
	}{}
	for _, n := range doc.Nodes {
		byID[n.ID] = struct{ Constraint, Date string }{n.Constraint, n.ConstraintDate}
	}

	// A keeps day 0 (no pin); B is pushed behind A's 2 days:
	// Mon+Tue for A, so B starts Wednesday 2026-06-03.
	if byID["A"].Constraint != "" {
		t.Errorf("A must not be pinned, got %q", byID["A"].Constraint)
	}
	if byID["B"].Constraint != "SNET" || byID["B"].Date != "2026-06-03" {
		t.Errorf("B = %+v, want SNET @ 2026-06-03", byID["B"])
	}
}

func TestLevelChartResourcesHonoursStakeholderAvailability(t *testing.T) {
	app, d, c := newResourceTestApp(t)

	// alice is a two-person pool: both tasks fit in parallel, so
	// levelling should pin nothing.
	if _, err := d.SaveStakeholder(db.Stakeholder{
		ProjectID:    "project-1",
		Name:         "alice",
		Availability: 2,
	}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}

	pinned, err := app.LevelChartResources(c.ID)
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if pinned != 0 {
		t.Errorf("pinned = %d, want 0 (capacity 2 absorbs both tasks)", pinned)
	}
}

func TestStakeholderAvailabilityRoundTrip(t *testing.T) {
	_, d, _ := newResourceTestApp(t)

	s, err := d.SaveStakeholder(db.Stakeholder{
		ProjectID: "project-1", Name: "bob", Availability: 0.5,
	})
	if err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	if s.Availability != 0.5 {
		t.Errorf("Availability = %v, want 0.5", s.Availability)
	}

	// Zero/unset availability defaults to 1 (full-time).
	s2, err := d.SaveStakeholder(db.Stakeholder{ProjectID: "project-1", Name: "carol"})
	if err != nil {
		t.Fatalf("SaveStakeholder (default): %v", err)
	}
	if s2.Availability != 1 {
		t.Errorf("default Availability = %v, want 1", s2.Availability)
	}
}

func TestLevelChartResourcesNeedsStartDate(t *testing.T) {
	app, d, c := newResourceTestApp(t)
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Resource Test", StartDate: ""}); err != nil {
		t.Fatalf("clear start date: %v", err)
	}
	if _, err := app.LevelChartResources(c.ID); err == nil {
		t.Error("levelling without a project start date must error")
	}
}

func TestGenerateResourceHistogram(t *testing.T) {
	app, d, c := newResourceTestApp(t)

	hist, err := app.GenerateResourceHistogram(c.ID)
	if err != nil {
		t.Fatalf("GenerateResourceHistogram: %v", err)
	}
	if hist.Kind != "bar" {
		t.Errorf("Kind = %s, want bar", hist.Kind)
	}
	if !strings.Contains(hist.Data, `"alice"`) {
		t.Errorf("histogram missing alice series:\n%s", hist.Data)
	}
	// Anchored project: day labels are real dates.
	if !strings.Contains(hist.Data, "2026-06-01") {
		t.Errorf("histogram categories should be dates:\n%s", hist.Data)
	}

	// Regenerating must update the SAME chart, not create another.
	again, err := app.GenerateResourceHistogram(c.ID)
	if err != nil {
		t.Fatalf("GenerateResourceHistogram (again): %v", err)
	}
	if again.ID != hist.ID {
		t.Errorf("regeneration created a new chart: %s != %s", again.ID, hist.ID)
	}
	bars, err := d.ListCharts("project-1", "bar")
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(bars) != 1 {
		t.Errorf("bar charts = %d, want 1", len(bars))
	}
}

func TestGenerateResourceHistogramNoAssignments(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	c, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "cpm",
		Title:     "Bare",
		Data:      `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := app.GenerateResourceHistogram(c.ID); err == nil {
		t.Error("histogram with no assignments must error")
	}
}
