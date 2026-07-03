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

	res, err := app.LevelChartResources(c.ID, "")
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if res.Pinned != 1 {
		t.Fatalf("pinned = %d, want 1 (only the delayed task)", res.Pinned)
	}
	if len(res.UnplacedTaskIDs) != 0 {
		t.Fatalf("UnplacedTaskIDs = %v, want none (both tasks fit)", res.UnplacedTaskIDs)
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

	res, err := app.LevelChartResources(c.ID, "")
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if res.Pinned != 0 {
		t.Errorf("pinned = %d, want 0 (capacity 2 absorbs both tasks)", res.Pinned)
	}
}

// TestLevelChartResourcesReportsUnplaceableTasks proves that when a task's
// demand can never fit capacity (units 2 against a one-person resource), the
// action still succeeds and persists the placeable pins, but reports the
// unplaceable task by ID and label so the UI can warn the user. This is the
// production-path counterpart to the kernel's ErrLevelingHorizonExceeded.
func TestLevelChartResourcesReportsUnplaceableTasks(t *testing.T) {
	app, d, _ := newResourceTestApp(t)

	// C demands 2 units of alice (a one-person default resource): it can
	// never be levelled into capacity and must surface as unplaceable.
	c, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "cpm",
		Title:     "Overloaded",
		Data: `{
			"nodes": [
				{"id":"C","label":"Impossible task","duration":1,"assignments":[{"resource":"alice","units":2}]}
			],
			"edges": []
		}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	res, err := app.LevelChartResources(c.ID, "")
	if err != nil {
		t.Fatalf("LevelChartResources should not hard-fail on an over-constrained schedule: %v", err)
	}
	if len(res.UnplacedTaskIDs) != 1 || res.UnplacedTaskIDs[0] != "C" {
		t.Fatalf("UnplacedTaskIDs = %v, want [C]", res.UnplacedTaskIDs)
	}
	if len(res.UnplacedLabels) != 1 || res.UnplacedLabels[0] != "Impossible task" {
		t.Fatalf("UnplacedLabels = %v, want [Impossible task]", res.UnplacedLabels)
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
	if _, err := app.LevelChartResources(c.ID, ""); err == nil {
		t.Error("levelling without a project start date must error")
	}
}

// TestLevelChartResourcesStrategyDivergence proves the strategy argument
// reaches the kernel: the same over-subscribed schedule pins a different
// task under EDF than under the default LTF. A depends on P (early
// deadline); B is a long low-slack task; both need alice.
func TestLevelChartResourcesStrategyDivergence(t *testing.T) {
	data := `{
		"nodes": [
			{"id":"A","label":"A","duration":1,"assignments":[{"resource":"alice"}]},
			{"id":"P","label":"P","duration":1,"precedents":["A"]},
			{"id":"B","label":"B","duration":5,"assignments":[{"resource":"alice"}]},
			{"id":"LP","label":"LP","duration":6}
		],
		"edges": []
	}`

	constraintByID := func(app *App, d *db.Database, chartID string) map[string]string {
		t.Helper()
		got, err := d.GetChart(chartID)
		if err != nil {
			t.Fatalf("GetChart: %v", err)
		}
		var doc struct {
			Nodes []struct {
				ID         string `json:"id"`
				Constraint string `json:"constraint"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out := map[string]string{}
		for _, n := range doc.Nodes {
			out[n.ID] = n.Constraint
		}
		return out
	}

	// Default LTF: B (least float) keeps day 0, A is delayed and pinned.
	appLTF, dLTF, _ := newResourceTestApp(t)
	cLTF, err := dLTF.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "LTF", Data: data})
	if err != nil {
		t.Fatalf("SaveChart LTF: %v", err)
	}
	if _, err := appLTF.LevelChartResources(cLTF.ID, "ltf"); err != nil {
		t.Fatalf("LevelChartResources LTF: %v", err)
	}
	ltf := constraintByID(appLTF, dLTF, cLTF.ID)
	if ltf["A"] != "SNET" || ltf["B"] != "" {
		t.Errorf("LTF: A=%q B=%q, want A pinned (SNET), B free", ltf["A"], ltf["B"])
	}

	// EDF: A (earliest deadline) keeps day 0, B is delayed and pinned.
	appEDF, dEDF, _ := newResourceTestApp(t)
	cEDF, err := dEDF.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "EDF", Data: data})
	if err != nil {
		t.Fatalf("SaveChart EDF: %v", err)
	}
	if _, err := appEDF.LevelChartResources(cEDF.ID, "edf"); err != nil {
		t.Fatalf("LevelChartResources EDF: %v", err)
	}
	edf := constraintByID(appEDF, dEDF, cEDF.ID)
	if edf["B"] != "SNET" || edf["A"] != "" {
		t.Errorf("EDF: A=%q B=%q, want B pinned (SNET), A free", edf["A"], edf["B"])
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
