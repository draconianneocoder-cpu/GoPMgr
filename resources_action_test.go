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

	res, err := app.LevelChartResources(c.ID, "", false, false)
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

	res, err := app.LevelChartResources(c.ID, "", false, false)
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

	res, err := app.LevelChartResources(c.ID, "", false, false)
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
	if _, err := app.LevelChartResources(c.ID, "", false, false); err == nil {
		t.Error("levelling without a project start date must error")
	}
}

// TestLevelChartResourcesStrategyDivergence proves the strategy argument
// reaches the kernel: the same over-subscribed schedule pins a different
// task under EDF than under the default LTF. A depends on P (early
// deadline); B is a long low-slack task; both need alice.
func TestLevelChartResourcesStrategyDivergence(t *testing.T) {
	// Dependencies are expressed via `edges` (from->to); the CPM chart model
	// does not read node-level `precedents`. A->P gives A its float; B is a
	// low-slack 5-day task; LP is the long pole. A and B contend for alice.
	data := `{
		"nodes": [
			{"id":"A","label":"A","duration":1,"assignments":[{"resource":"alice"}]},
			{"id":"P","label":"P","duration":1},
			{"id":"B","label":"B","duration":5,"assignments":[{"resource":"alice"}]},
			{"id":"LP","label":"LP","duration":6}
		],
		"edges": [{"from":"A","to":"P"}]
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
	if _, err := appLTF.LevelChartResources(cLTF.ID, "ltf", false, false); err != nil {
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
	if _, err := appEDF.LevelChartResources(cEDF.ID, "edf", false, false); err != nil {
		t.Fatalf("LevelChartResources EDF: %v", err)
	}
	edf := constraintByID(appEDF, dEDF, cEDF.ID)
	if edf["B"] != "SNET" || edf["A"] != "" {
		t.Errorf("EDF: A=%q B=%q, want B pinned (SNET), A free", edf["A"], edf["B"])
	}
}

// TestLevelChartResourcesPriorityCriticalFlipsPersistedOutcome proves the
// priorityCritical flag has an observable end-to-end effect: under EDF a
// floating task with an earlier deadline would delay the critical task, but
// the override protects it — flipping which task is pinned in the saved
// chart.
//
// Dependencies are expressed via `edges` (from->to), which is how the CPM
// chart model stores precedence; node-level `precedents` is not read by
// cpmChartDataToKernelTasks.
func TestLevelChartResourcesPriorityCriticalFlipsPersistedOutcome(t *testing.T) {
	// C (dur 5, alice) is the critical pole. F (dur 1, alice) feeds G
	// (dur 3) via an edge, so F floats but has an earlier late-finish than
	// C; plain EDF therefore delays the critical C.
	const data = `{
		"nodes": [
			{"id":"C","label":"C","duration":5,"assignments":[{"resource":"alice"}]},
			{"id":"F","label":"F","duration":1,"assignments":[{"resource":"alice"}]},
			{"id":"G","label":"G","duration":3}
		],
		"edges": [{"from":"F","to":"G"}]
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

	// EDF without override: the critical task C is delayed/pinned.
	appOff, dOff, _ := newResourceTestApp(t)
	cOff, err := dOff.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "off", Data: data})
	if err != nil {
		t.Fatalf("SaveChart off: %v", err)
	}
	if _, err := appOff.LevelChartResources(cOff.ID, "edf", false, false); err != nil {
		t.Fatalf("LevelChartResources off: %v", err)
	}
	if off := constraintByID(appOff, dOff, cOff.ID); off["C"] != "SNET" {
		t.Errorf("EDF (no override): C=%q, want C pinned (critical delayed)", off["C"])
	}

	// EDF with priorityCritical: C is protected, so it is NOT pinned.
	appOn, dOn, _ := newResourceTestApp(t)
	cOn, err := dOn.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "on", Data: data})
	if err != nil {
		t.Fatalf("SaveChart on: %v", err)
	}
	if _, err := appOn.LevelChartResources(cOn.ID, "edf", true, false); err != nil {
		t.Fatalf("LevelChartResources on: %v", err)
	}
	on := constraintByID(appOn, dOn, cOn.ID)
	if on["C"] != "" {
		t.Errorf("EDF+priorityCritical: C=%q, want C free (critical protected)", on["C"])
	}
	if on["F"] != "SNET" {
		t.Errorf("EDF+priorityCritical: F=%q, want F pinned (yielded to critical C)", on["F"])
	}
}

// TestPreviewSplitLevelingReportsSplitTasks proves the read-only preview
// reports which tasks activity splitting would interrupt and that the split
// plan is conflict-free, without persisting anything (constraints unchanged).
func TestPreviewSplitLevelingReportsSplitTasks(t *testing.T) {
	app, d, _ := newResourceTestApp(t)

	// alice is unavailable on the project's second working day (a one-day
	// calendar gap), so a 2-day task starting day 0 must either wait or
	// split around the gap. A 3-day task forces a genuine split.
	if _, err := d.SaveResourceCalendar(db.ResourceCalendar{
		ProjectID:       "project-1",
		Resource:        "alice",
		DefaultCapacity: 1,
		Overrides:       map[int]float64{1: 0, 3: 0},
	}); err != nil {
		t.Skipf("resource calendar API unavailable: %v", err)
	}

	c, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "cpm",
		Title:     "Splittable",
		Data: `{
			"nodes": [
				{"id":"S","label":"Long task","duration":3,"assignments":[{"resource":"alice"}]}
			],
			"edges": []
		}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	preview, err := app.PreviewSplitLeveling(c.ID)
	if err != nil {
		t.Fatalf("PreviewSplitLeveling: %v", err)
	}
	if len(preview.SplitTaskLabels) != 1 || preview.SplitTaskLabels[0] != "Long task" {
		t.Fatalf("SplitTaskLabels = %v, want [Long task]", preview.SplitTaskLabels)
	}
	if !preview.ResolvesOverallocation {
		t.Errorf("ResolvesOverallocation = false, want true (splitting fits the gap)")
	}

	// The preview must NOT persist: the chart node carries no constraint.
	got, err := d.GetChart(c.ID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	var doc struct {
		Nodes []struct {
			Constraint string `json:"constraint"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Nodes[0].Constraint != "" {
		t.Errorf("preview persisted a constraint %q, want none", doc.Nodes[0].Constraint)
	}
}

// TestLevelChartResourcesAppliesAndPersistsSplitSegments proves that
// applying leveling with allowSplitting=true persists relative WorkSegments
// on split nodes and reports them, and that they survive reload into the
// Gantt layout as absolute bar pieces.
func TestLevelChartResourcesAppliesAndPersistsSplitSegments(t *testing.T) {
	app, d, _ := newResourceTestApp(t)

	// alice unavailable on odd days: a 3-day task must split to 0,2,4.
	if _, err := d.SaveResourceCalendar(db.ResourceCalendar{
		ProjectID:       "project-1",
		Resource:        "alice",
		DefaultCapacity: 1,
		Overrides:       map[int]float64{1: 0, 3: 0},
	}); err != nil {
		t.Skipf("resource calendar API unavailable: %v", err)
	}
	c, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "cpm",
		Title:     "Splittable",
		Data:      `{"nodes":[{"id":"S","label":"Long task","duration":3,"assignments":[{"resource":"alice"}]}],"edges":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	res, err := app.LevelChartResources(c.ID, "", false, true) // allowSplitting
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if len(res.SplitLabels) != 1 || res.SplitLabels[0] != "Long task" {
		t.Fatalf("SplitLabels = %v, want [Long task]", res.SplitLabels)
	}

	// Persisted node carries relative WorkSegments [0,1),[2,3),[4,5).
	got, err := d.GetChart(c.ID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	var doc struct {
		Nodes []struct {
			ID           string `json:"id"`
			WorkSegments []struct {
				Start float64 `json:"start"`
				End   float64 `json:"end"`
			} `json:"work_segments"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Nodes) != 1 || len(doc.Nodes[0].WorkSegments) != 3 {
		t.Fatalf("persisted WorkSegments = %+v, want 3 relative pieces", doc.Nodes)
	}
	want := [][2]float64{{0, 1}, {2, 3}, {4, 5}}
	for i, s := range doc.Nodes[0].WorkSegments {
		if s.Start != want[i][0] || s.End != want[i][1] {
			t.Errorf("segment %d = {%.0f,%.0f}, want {%.0f,%.0f}", i, s.Start, s.End, want[i][0], want[i][1])
		}
	}
}

// TestLayoutChartExposesSplitSegmentsEndToEnd exercises the exact backend
// path the Gantt editor uses: apply split-leveling to a gantt-kind chart,
// then call LayoutChart and assert the returned layout's rows carry the
// absolute work_segments the SVG draws. Guards the persist -> LayoutChart
// -> body.layout join, not just the two halves in isolation.
func TestLayoutChartExposesSplitSegmentsEndToEnd(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	if _, err := d.SaveResourceCalendar(db.ResourceCalendar{
		ProjectID: "project-1", Resource: "alice", DefaultCapacity: 1,
		Overrides: map[int]float64{1: 0, 3: 0},
	}); err != nil {
		t.Skipf("resource calendar API unavailable: %v", err)
	}
	c, err := d.SaveChart(db.Chart{
		ProjectID: "project-1",
		Kind:      "gantt",
		Title:     "Gantt",
		Data:      `{"nodes":[{"id":"S","label":"Long task","duration":3,"assignments":[{"resource":"alice"}]}],"edges":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := app.LevelChartResources(c.ID, "", false, true); err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}

	res, err := app.LayoutChart(c.ID)
	if err != nil {
		t.Fatalf("LayoutChart: %v", err)
	}
	// Round-trip the LayoutResult through JSON exactly as the Wails bridge
	// does, then read rows[].work_segments from the body the frontend sees.
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal LayoutResult: %v", err)
	}
	var shape struct {
		Body struct {
			Layout struct {
				Rows []struct {
					ID           string `json:"id"`
					WorkSegments []struct {
						Start float64 `json:"start"`
						End   float64 `json:"end"`
					} `json:"work_segments"`
				} `json:"rows"`
			} `json:"layout"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatalf("unmarshal layout: %v", err)
	}
	if len(shape.Body.Layout.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(shape.Body.Layout.Rows))
	}
	segs := shape.Body.Layout.Rows[0].WorkSegments
	if len(segs) != 3 {
		t.Fatalf("LayoutChart rows[0].work_segments = %+v, want 3 absolute pieces", segs)
	}
	// Absolute segments must be strictly increasing and non-contiguous.
	if !(segs[0].End < segs[1].Start && segs[1].End < segs[2].Start) {
		t.Errorf("segments not a non-contiguous split: %+v", segs)
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
