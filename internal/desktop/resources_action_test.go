// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/db"
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

// levelledNode is what levelling persists on a node, as stored.
type levelledNode struct {
	Constraint, ConstraintDate, LevelStart string
	LevelStartFraction                     float64
}

// storedLevelling reads every node's constraint and levelled start from
// the saved chart.
func storedLevelling(t *testing.T, d *db.Database, chartID string) map[string]levelledNode {
	t.Helper()
	got, err := d.GetChart(chartID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	var doc struct {
		Nodes []struct {
			ID             string  `json:"id"`
			Constraint     string  `json:"constraint"`
			ConstraintDate string  `json:"constraint_date"`
			LevelStart     string  `json:"level_start"`
			Fraction       float64 `json:"level_start_fraction"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
		t.Fatalf("unmarshal persisted doc: %v", err)
	}
	out := map[string]levelledNode{}
	for _, n := range doc.Nodes {
		out[n.ID] = levelledNode{n.Constraint, n.ConstraintDate, n.LevelStart, n.Fraction}
	}
	return out
}

// storedLevelledInputs reads the levelling digest stored in the chart.
func storedLevelledInputs(t *testing.T, d *db.Database, chartID string) string {
	t.Helper()
	got, err := d.GetChart(chartID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	var doc struct {
		LevelledInputs string `json:"levelled_inputs"`
	}
	if err := json.Unmarshal([]byte(got.Data), &doc); err != nil {
		t.Fatalf("unmarshal persisted doc: %v", err)
	}
	return doc.LevelledInputs
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

	// A keeps day 0 (no levelled start); B is pushed behind A's 2 days:
	// Mon+Tue for A, so B starts Wednesday 2026-06-03. Levelling stores
	// that as B's levelled start and writes no constraint (owner decision
	// 2026-11-18).
	byID := storedLevelling(t, d, c.ID)
	if byID["A"] != (levelledNode{}) {
		t.Errorf("A = %+v, want nothing stored", byID["A"])
	}
	if byID["B"] != (levelledNode{LevelStart: "2026-06-03"}) {
		t.Errorf("B = %+v, want levelled start 2026-06-03 and no constraint", byID["B"])
	}
	// Levelling records what it read, so the editors can warn after a
	// schedule edit (owner decision 2026-11-21).
	if got := storedLevelledInputs(t, d, c.ID); !strings.HasPrefix(got, "v2:") {
		t.Errorf("levelled_inputs = %q, want a v2 digest", got)
	}

	// Levelling again keeps B's levelled start: it recomputes from the
	// logic, not from the previous run's delay. (Before 2026-11-18 a
	// second run cleared the SNET pin it still needed.)
	res, err = app.LevelChartResources(c.ID, "", false, false)
	if err != nil {
		t.Fatalf("second LevelChartResources: %v", err)
	}
	if res.Pinned != 1 {
		t.Fatalf("second run pinned = %d, want 1", res.Pinned)
	}
	if again := storedLevelling(t, d, c.ID); again["B"] != (levelledNode{LevelStart: "2026-06-03"}) || again["A"] != (levelledNode{}) {
		t.Errorf("after a second run: A = %+v, B = %+v; want A nothing, B levelled start 2026-06-03", again["A"], again["B"])
	}

	// With alice able to do both at once, the delay is no longer needed:
	// levelling clears B's levelled start rather than keeping its own.
	if _, err := d.SaveStakeholder(db.Stakeholder{ProjectID: "project-1", Name: "alice", Availability: 2}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	if res, err = app.LevelChartResources(c.ID, "", false, false); err != nil || res.Pinned != 0 {
		t.Fatalf("third run: pinned %d, err %v; want 0, nil", res.Pinned, err)
	}
	if freed := storedLevelling(t, d, c.ID); freed["B"] != (levelledNode{}) {
		t.Errorf("with capacity 2: B = %+v, want nothing stored", freed["B"])
	}
	// Nothing levelled, nothing that can go stale.
	if got := storedLevelledInputs(t, d, c.ID); got != "" {
		t.Errorf("with nothing levelled: levelled_inputs = %q, want none", got)
	}
}

// Levelling never writes, rewrites or removes a constraint (owner decision
// 2026-11-18): a task with its own constraint gets a levelled start like
// any other, a hand-set SNET that is not needed stays, an explicit ASAP is
// levelled like no constraint, and an ALAP task gets no levelled start.
// It also names the tasks whose date levelling broke, and only those.
//
// Every task but LP needs alice full-time; there are no links; the project
// starts Monday 2026-06-01 (index 0). Late starts from the plain pass:
// LP (6 days, no resource) sets the project finish 6; V (MSO Fri 05-29,
// before the start) is already violated, LF pinned -1+1 = 0, LS -1;
// F (FNLT Mon, latest finish index 1) LF 1, LS 0; S (SNLT Mon) LS 0;
// E (explicit ASAP), H (hand-set SNET Mon), M (MSO Mon 06-08, index 5)
// and Q (ALAP) LS 5. LTF places by (LS, ID): V Mon (0); F Tue (1),
// finishing at index 2, past its FNLT; LP needs no resource; S Wed (2),
// breaking its SNLT; E Thu (3); H Fri (4); M stays on its MSO day (5); Q
// wants day 5, finds alice busy and moves to 6, but ALAP tasks get no
// levelled start. Violated after and not before: F and S.
func TestLevelChartResourcesNeverChangesConstraints(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Constrained", Data: `{
		"nodes": [
			{"id":"LP","label":"LP","duration":6},
			{"id":"S","label":"Survey","duration":1,"constraint":"SNLT","constraint_date":"2026-06-01","assignments":[{"resource":"alice"}]},
			{"id":"F","label":"Fit-out","duration":1,"constraint":"FNLT","constraint_date":"2026-06-01","assignments":[{"resource":"alice"}]},
			{"id":"V","label":"V","duration":1,"constraint":"MSO","constraint_date":"2026-05-29","assignments":[{"resource":"alice"}]},
			{"id":"E","label":"E","duration":1,"constraint":"ASAP","assignments":[{"resource":"alice"}]},
			{"id":"M","label":"M","duration":1,"constraint":"MSO","constraint_date":"2026-06-08","assignments":[{"resource":"alice"}]},
			{"id":"H","label":"H","duration":1,"constraint":"SNET","constraint_date":"2026-06-01","assignments":[{"resource":"alice"}]},
			{"id":"Q","label":"Q","duration":1,"constraint":"ALAP","assignments":[{"resource":"alice"}]}
		],
		"edges": []
	}`})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	res, err := app.LevelChartResources(c.ID, "ltf", false, false)
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	got := storedLevelling(t, d, c.ID)
	want := map[string]levelledNode{
		"LP": {},
		"F":  {Constraint: "FNLT", ConstraintDate: "2026-06-01", LevelStart: "2026-06-02"},
		"S":  {Constraint: "SNLT", ConstraintDate: "2026-06-01", LevelStart: "2026-06-03"},
		"V":  {Constraint: "MSO", ConstraintDate: "2026-05-29"},
		"E":  {Constraint: "ASAP", LevelStart: "2026-06-04"},
		"M":  {Constraint: "MSO", ConstraintDate: "2026-06-08"},
		"H":  {Constraint: "SNET", ConstraintDate: "2026-06-01", LevelStart: "2026-06-05"},
		"Q":  {Constraint: "ALAP"},
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s = %+v, want %+v", id, got[id], w)
		}
	}
	if res.Pinned != 4 {
		t.Errorf("pinned = %d, want 4 (F, S, E, H)", res.Pinned)
	}
	if !reflect.DeepEqual(res.ConstraintViolationTaskIDs, []string{"F", "S"}) ||
		!reflect.DeepEqual(res.ConstraintViolationLabels, []string{"Fit-out", "Survey"}) {
		t.Errorf("violations = %v %v, want [F S] [Fit-out Survey] (V was violated before levelling)", res.ConstraintViolationTaskIDs, res.ConstraintViolationLabels)
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

	// Default LTF: B (least float) keeps day 0, A is delayed and pinned.
	appLTF, dLTF, _ := newResourceTestApp(t)
	cLTF, err := dLTF.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "LTF", Data: data})
	if err != nil {
		t.Fatalf("SaveChart LTF: %v", err)
	}
	if _, err := appLTF.LevelChartResources(cLTF.ID, "ltf", false, false); err != nil {
		t.Fatalf("LevelChartResources LTF: %v", err)
	}
	ltf := storedLevelling(t, dLTF, cLTF.ID)
	if ltf["A"].LevelStart == "" || ltf["B"] != (levelledNode{}) {
		t.Errorf("LTF: A=%+v B=%+v, want A levelled, B free", ltf["A"], ltf["B"])
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
	edf := storedLevelling(t, dEDF, cEDF.ID)
	if edf["B"].LevelStart == "" || edf["A"] != (levelledNode{}) {
		t.Errorf("EDF: A=%+v B=%+v, want B levelled, A free", edf["A"], edf["B"])
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
	// EDF without override: the critical task C is delayed/pinned.
	appOff, dOff, _ := newResourceTestApp(t)
	cOff, err := dOff.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "off", Data: data})
	if err != nil {
		t.Fatalf("SaveChart off: %v", err)
	}
	if _, err := appOff.LevelChartResources(cOff.ID, "edf", false, false); err != nil {
		t.Fatalf("LevelChartResources off: %v", err)
	}
	if off := storedLevelling(t, dOff, cOff.ID); off["C"].LevelStart == "" || off["C"].Constraint != "" {
		t.Errorf("EDF (no override): C=%+v, want C levelled (critical delayed), no constraint", off["C"])
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
	on := storedLevelling(t, dOn, cOn.ID)
	if on["C"] != (levelledNode{}) {
		t.Errorf("EDF+priorityCritical: C=%+v, want C free (critical protected)", on["C"])
	}
	if on["F"].LevelStart == "" || on["F"].Constraint != "" {
		t.Errorf("EDF+priorityCritical: F=%+v, want F levelled (yielded to critical C), no constraint", on["F"])
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

// Part-day levelling (owner decision 2026-11-21): a chain of part-day
// tasks on one person that fits within the day's capacity is left where
// the logic puts it, so levelling stores no levelled start. A (0.5 day),
// B (0.4) and C (0.5) run 0 to 1.4: day 0 holds exactly 1 day of work.
func TestLevelChartResourcesLeavesAFittingPartDayChain(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Part days", Data: `{
		"nodes": [
			{"id":"A","label":"A","duration":0.5,"assignments":[{"resource":"alice"}]},
			{"id":"B","label":"B","duration":0.4,"assignments":[{"resource":"alice"}]},
			{"id":"C","label":"C","duration":0.5,"assignments":[{"resource":"alice"}]}
		],
		"edges": [{"from":"A","to":"B"},{"from":"B","to":"C"}]
	}`})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	res, err := app.LevelChartResources(c.ID, "", false, false)
	if err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	if res.Pinned != 0 || len(res.UnplacedTaskIDs) != 0 {
		t.Fatalf("result = %+v, want nothing delayed or unplaced", res)
	}
	for id, n := range storedLevelling(t, d, c.ID) {
		if n != (levelledNode{}) {
			t.Errorf("%s = %+v, want nothing stored", id, n)
		}
	}
}

// Levelling stores a digest of what it read, and the chart's layout
// compares it with the stored chart. Round-tripped through the store as
// the app does it, a just-levelled chart is not stale (assignments here
// carry an empty skill-tag list, which saving drops); changing a duration
// makes it stale (owner decision 2026-11-21).
func TestLevelledChartIsStaleOnlyAfterAScheduleEdit(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Tags", Data: `{
		"nodes": [
			{"id":"A","label":"A","duration":2,"assignments":[{"resource":"alice","skill_tags":[]}]},
			{"id":"B","label":"B","duration":2,"assignments":[{"resource":"alice","skill_tags":[]}]}
		],
		"edges": []
	}`})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := app.LevelChartResources(c.ID, "", false, false); err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	stale := func() bool {
		t.Helper()
		res, err := app.LayoutChart(c.ID)
		if err != nil {
			t.Fatalf("LayoutChart: %v", err)
		}
		var body struct {
			Doc struct {
				LevellingStale bool `json:"levelling_stale"`
			} `json:"doc"`
		}
		if err := json.Unmarshal(res.Body, &body); err != nil {
			t.Fatalf("decode layout: %v", err)
		}
		return body.Doc.LevellingStale
	}
	if stale() {
		t.Fatal("a chart just levelled reports stale levelling")
	}

	stored, err := d.GetChart(c.ID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	stored.Data = strings.Replace(stored.Data, `"duration":2`, `"duration":3`, 1)
	if _, err := d.SaveChart(stored); err != nil {
		t.Fatalf("SaveChart (edit): %v", err)
	}
	if !stale() {
		t.Fatal("a duration edit after levelling does not report stale levelling")
	}
}

// What levelling read outside the chart is part of what can go stale: the
// project start date, and the capacity of the resources the chart uses
// (stakeholder availability, their calendars). Calendar notes and
// resources the chart does not use are not. Each case levels the A/B alice
// chart (a calendar for alice exists from the start), changes one thing,
// and lays the chart out again.
func TestLevellingGoesStaleWhenItsContextChanges(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(t *testing.T, d *db.Database)
		stale bool
	}{
		{"project start date", func(t *testing.T, d *db.Database) {
			if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Resource Test", StartDate: "2026-06-08"}); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"capacity of a resource the chart uses", func(t *testing.T, d *db.Database) {
			if _, err := d.SaveStakeholder(db.Stakeholder{ProjectID: "project-1", Name: "alice", Availability: 0.5}); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"its calendar's capacity", func(t *testing.T, d *db.Database) {
			saveAliceCalendar(t, d, map[int]float64{4: 0}, nil)
		}, true},
		{"its calendar's notes", func(t *testing.T, d *db.Database) {
			saveAliceCalendar(t, d, nil, map[int]string{4: "training"})
		}, false},
		{"a resource the chart does not use", func(t *testing.T, d *db.Database) {
			if _, err := d.SaveStakeholder(db.Stakeholder{ProjectID: "project-1", Name: "bob", Availability: 3}); err != nil {
				t.Fatal(err)
			}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, d, c := newResourceTestApp(t)
			saveAliceCalendar(t, d, nil, nil)
			if _, err := app.LevelChartResources(c.ID, "", false, false); err != nil {
				t.Fatalf("LevelChartResources: %v", err)
			}
			if layoutReportsStale(t, app, c.ID) {
				t.Fatal("a chart just levelled reports stale levelling")
			}
			tc.edit(t, d)
			if got := layoutReportsStale(t, app, c.ID); got != tc.stale {
				t.Errorf("stale after the change = %v, want %v", got, tc.stale)
			}
		})
	}
}

// saveAliceCalendar stores (or replaces) alice's resource calendar.
func saveAliceCalendar(t *testing.T, d *db.Database, overrides map[int]float64, notes map[int]string) {
	t.Helper()
	cals, err := d.ListResourceCalendars("project-1")
	if err != nil {
		t.Fatal(err)
	}
	cal := db.ResourceCalendar{ProjectID: "project-1", Resource: "alice", DefaultCapacity: 1, Overrides: overrides, Notes: notes}
	for _, existing := range cals {
		if existing.Resource == "alice" {
			cal.ID = existing.ID
		}
	}
	if _, err := d.SaveResourceCalendar(cal); err != nil {
		t.Fatal(err)
	}
}

// layoutReportsStale lays a chart out through the app and reads the flag.
func layoutReportsStale(t *testing.T, app *App, chartID string) bool {
	t.Helper()
	res, err := app.LayoutChart(chartID)
	if err != nil {
		t.Fatalf("LayoutChart: %v", err)
	}
	var body struct {
		Doc struct {
			LevellingStale bool `json:"levelling_stale"`
		} `json:"doc"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("decode layout: %v", err)
	}
	return body.Doc.LevellingStale
}

// The saved chart lays out as levelling placed it (owner decisions
// 2026-11-23): levelling, saving and laying out again gives each
// task its levelled start and flags nothing. Project from Monday
// 2026-06-01, weekdays.
func TestLevelledChartLaysOutAsLevelled(t *testing.T) {
	cases := []struct {
		name     string
		data     string
		split    bool
		calendar map[int]float64
		stored   map[string]levelledNode
		es       map[string]float64
	}{
		{
			// A and B (half a day each on alice) would overlap at 0; A goes
			// first (ID order) and B starts when A ends, half-way through
			// Monday. Only B needs a stored start: Monday plus 0.5.
			name: "a part-day start is stored with its share of the day",
			data: `{"nodes":[
				{"id":"A","label":"A","duration":0.5,"assignments":[{"resource":"alice"}]},
				{"id":"B","label":"B","duration":0.5,"assignments":[{"resource":"alice"}]}],"edges":[]}`,
			stored: map[string]levelledNode{"A": {}, "B": {LevelStart: "2026-06-01", LevelStartFraction: 0.5}},
			es:     map[string]float64{"A": 0, "B": 0.5},
		},
		{
			// alice is away on days 1 and 3, so S (3 days) splits onto days
			// 0, 2 and 4 and finishes at 5; T follows S. Saved, the forward
			// pass sees S end at 3, so T keeps its own start: day 5, Monday
			// 06-08. S itself starts where the logic puts it.
			name:     "the successor of a split task keeps its start",
			split:    true,
			calendar: map[int]float64{1: 0, 3: 0},
			data: `{"nodes":[
				{"id":"S","label":"S","duration":3,"assignments":[{"resource":"alice"}]},
				{"id":"T","label":"T","duration":1,"assignments":[{"resource":"alice"}]}],
				"edges":[{"from":"S","to":"T"}]}`,
			stored: map[string]levelledNode{"S": {}, "T": {LevelStart: "2026-06-08"}},
			es:     map[string]float64{"S": 0, "T": 5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, d, _ := newResourceTestApp(t)
			if tc.calendar != nil {
				saveAliceCalendar(t, d, tc.calendar, nil)
			}
			c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Levelled", Data: tc.data})
			if err != nil {
				t.Fatalf("SaveChart: %v", err)
			}
			if _, err := app.LevelChartResources(c.ID, "", false, tc.split); err != nil {
				t.Fatalf("LevelChartResources: %v", err)
			}
			if got := storedLevelling(t, d, c.ID); !reflect.DeepEqual(got, tc.stored) {
				t.Errorf("stored %+v, want %+v", got, tc.stored)
			}

			res, err := app.LayoutChart(c.ID)
			if err != nil {
				t.Fatalf("LayoutChart: %v", err)
			}
			var body struct {
				Doc struct {
					Nodes []struct {
						ID            string  `json:"id"`
						ES            float64 `json:"es"`
						Overallocated bool    `json:"overallocated"`
					} `json:"nodes"`
				} `json:"doc"`
			}
			if err := json.Unmarshal(res.Body, &body); err != nil {
				t.Fatalf("decode layout: %v", err)
			}
			for _, n := range body.Doc.Nodes {
				if n.ES != tc.es[n.ID] || n.Overallocated {
					t.Errorf("%s laid out at ES %v (overallocated %v), want ES %v and not overallocated", n.ID, n.ES, n.Overallocated, tc.es[n.ID])
				}
			}
		})
	}
}
