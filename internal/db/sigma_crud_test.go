// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"testing"

	"gopmgr/internal/sigma/domain"
)

// newSigmaProjectFKTarget inserts a `project` row (required by
// sigma_projects' own FOREIGN KEY (id) REFERENCES project(id)) and
// returns its ID, without creating the sigma_projects row itself --
// tests exercising SigmaCreateProject/EnsureProjectSigmaLink need that
// second row to not already exist.
func newSigmaProjectFKTarget(t *testing.T, d *Database) string {
	t.Helper()
	p, err := d.UpsertProject(Project{Name: "Sigma Link Target"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	return p.ID
}

func TestSigmaCreateAndGetProject_RoundTrips(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := newSigmaProjectFKTarget(t, d)

	want := domain.Project{
		ID:           projectID,
		Title:        "Reduce Cycle Time",
		Description:  "Cut order-to-ship time by 30%",
		BeltLevel:    domain.BeltBlack,
		Phase:        domain.PhaseMeasure,
		Status:       domain.StatusActive,
		Sponsor:      "VP Ops",
		ProcessOwner: "Line Manager",
		BeltLead:     "J. Doe",
	}
	if err := d.SigmaCreateProject(want); err != nil {
		t.Fatalf("SigmaCreateProject: %v", err)
	}

	got, err := d.SigmaGetProject(projectID)
	if err != nil {
		t.Fatalf("SigmaGetProject: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title || got.Description != want.Description {
		t.Errorf("identity/description fields = %+v, want %+v", got, want)
	}
	if got.BeltLevel != want.BeltLevel || got.Phase != want.Phase || got.Status != want.Status {
		t.Errorf("enum fields = %+v, want %+v", got, want)
	}
	if got.Sponsor != want.Sponsor || got.ProcessOwner != want.ProcessOwner || got.BeltLead != want.BeltLead {
		t.Errorf("people fields = %+v, want %+v", got, want)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not parsed: %+v", got)
	}
}

// TestSigmaGetProject_MissingIDReturnsAnErrorAndNilProject documents a
// real asymmetry: every sibling Sigma getter (Charter, Fishbone,
// Solutions, ControlPlan, SIPOC, VoC) special-cases sql.ErrNoRows and
// returns a usable zero-value/default struct with no error.
// SigmaGetProject does not -- it returns the raw scan error unmodified.
// This asserts only the property that holds regardless of whether that
// asymmetry is ever resolved (an error, and no usable project), not the
// specific error value, so a future fix that adds ErrNoRows handling or
// a sentinel here wouldn't need this test deleted or rewritten.
func TestSigmaGetProject_MissingIDReturnsAnErrorAndNilProject(t *testing.T) {
	d := newBackupTestDB(t)
	got, err := d.SigmaGetProject("does-not-exist")
	if err == nil {
		t.Fatal("SigmaGetProject(missing) = nil error, want an error")
	}
	if got != nil {
		t.Errorf("SigmaGetProject(missing) returned a non-nil project on error: %+v", got)
	}
}

func setSigmaProjectUpdatedAt(t *testing.T, d *Database, id, updatedAt string) {
	t.Helper()
	if _, err := d.Conn.Exec(`UPDATE sigma_projects SET updated_at = ? WHERE id = ?`, updatedAt, id); err != nil {
		t.Fatalf("set sigma_projects.updated_at for %s: %v", id, err)
	}
}

// TestSigmaListProjects_OrdersByMostRecentlyUpdatedDescending uses
// clearly-fractional, non-whole-second timestamps for the same reason
// documents_read_test.go's ordering test does -- but unlike
// documents/charts, sigma_projects.updated_at is ALWAYS written via
// SQLite's own strftime('%Y-%m-%dT%H:%M:%fZ','now') default (both the
// initial INSERT and SigmaAdvancePhase's UPDATE), never via Go's
// time.Now().Format(RFC3339Nano) -- so this table is not exposed to the
// whole-second-omits-the-fraction hazard pinned for documents.go/
// charts.go. Distinct fractional timestamps are still used here to keep
// the test deterministic rather than relying on wall-clock jitter
// between rapid inserts.
func TestSigmaListProjects_OrdersByMostRecentlyUpdatedDescending(t *testing.T) {
	d := newBackupTestDB(t)
	oldestID := newSigmaProjectFKTarget(t, d)
	middleID := newSigmaProjectFKTarget(t, d)
	newestID := newSigmaProjectFKTarget(t, d)
	for id, title := range map[string]string{oldestID: "Oldest", middleID: "Middle", newestID: "Newest"} {
		if err := d.SigmaCreateProject(domain.Project{ID: id, Title: title}); err != nil {
			t.Fatalf("SigmaCreateProject(%s): %v", title, err)
		}
	}
	setSigmaProjectUpdatedAt(t, d, oldestID, "2026-01-01T10:00:00.100Z")
	setSigmaProjectUpdatedAt(t, d, middleID, "2026-01-01T10:00:00.500Z")
	setSigmaProjectUpdatedAt(t, d, newestID, "2026-01-01T10:00:00.900Z")

	got, err := d.SigmaListProjects()
	if err != nil {
		t.Fatalf("SigmaListProjects: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("SigmaListProjects returned %d projects, want 3", len(got))
	}
	wantOrder := []string{"Newest", "Middle", "Oldest"}
	for i, want := range wantOrder {
		if got[i].Title != want {
			t.Errorf("order[%d] = %q, want %q (full order: %v)", i, got[i].Title, want,
				[]string{got[0].Title, got[1].Title, got[2].Title})
		}
	}
}

// TestSigmaAdvancePhase_PersistsPhase deliberately gives the fixture a
// real, non-empty starting phase (domain.PhaseDefine, matching the
// sigma_projects.phase column's own DEFAULT 'define'). SigmaCreateProject
// writes every column explicitly, so passing a zero-value Phase in the
// fixture would land an empty string, not the column default -- a test
// built on that zero-value fixture could only prove "phase went from
// empty to analyze," not that AdvancePhase performs a real transition
// between two meaningful phases.
func TestSigmaAdvancePhase_PersistsPhase(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := newSigmaProjectFKTarget(t, d)
	if err := d.SigmaCreateProject(domain.Project{ID: projectID, Title: "Phase Test", Phase: domain.PhaseDefine}); err != nil {
		t.Fatalf("SigmaCreateProject: %v", err)
	}
	before, err := d.SigmaGetProject(projectID)
	if err != nil {
		t.Fatalf("SigmaGetProject (before): %v", err)
	}
	if before.Phase != domain.PhaseDefine {
		t.Fatalf("fixture precondition failed: Phase = %q, want %q", before.Phase, domain.PhaseDefine)
	}

	if err := d.SigmaAdvancePhase(projectID, domain.PhaseAnalyze); err != nil {
		t.Fatalf("SigmaAdvancePhase: %v", err)
	}

	got, err := d.SigmaGetProject(projectID)
	if err != nil {
		t.Fatalf("SigmaGetProject: %v", err)
	}
	if got.Phase != domain.PhaseAnalyze {
		t.Errorf("Phase = %q, want %q", got.Phase, domain.PhaseAnalyze)
	}
}

// TestEnsureProjectSigmaLink_DoesNotOverwriteExistingRow is the real
// data-integrity risk this function exists to avoid: its own doc comment
// says it creates the link "if it doesn't exist" specifically because
// calling it against an already-linked project must be a safe no-op, not
// a silent overwrite of real Sigma project data (title, belt level,
// phase, status) with whatever placeholder values a caller passes on a
// second call.
func TestEnsureProjectSigmaLink_DoesNotOverwriteExistingRow(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := newSigmaProjectFKTarget(t, d)

	if err := d.EnsureProjectSigmaLink(domain.Project{
		ID: projectID, Title: "Original Title", BeltLevel: domain.BeltBlack, Phase: domain.PhaseAnalyze, Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("EnsureProjectSigmaLink (first call): %v", err)
	}

	if err := d.EnsureProjectSigmaLink(domain.Project{
		ID: projectID, Title: "Clobbering Title", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusOnHold,
	}); err != nil {
		t.Fatalf("EnsureProjectSigmaLink (second call): %v", err)
	}

	got, err := d.SigmaGetProject(projectID)
	if err != nil {
		t.Fatalf("SigmaGetProject: %v", err)
	}
	if got.Title != "Original Title" || got.BeltLevel != domain.BeltBlack || got.Phase != domain.PhaseAnalyze || got.Status != domain.StatusActive {
		t.Errorf("EnsureProjectSigmaLink's second call overwrote the existing row: %+v, want the original values preserved", got)
	}
}

// TestSigmaSaveCharter_CreateThenUpdateRoundTripsJSONFields exercises
// SaveCharter's create path, its ON CONFLICT(project_id) update path,
// and the ScopeIn/ScopeOut/CTQs JSON round trip -- including a
// non-round CTQ spec float (1.005, not 1.0) and a second CTQ element,
// so a precision-losing encode/decode change or a marshal bug that
// silently drops all but the first slice element would be caught.
func TestSigmaSaveCharter_CreateThenUpdateRoundTripsJSONFields(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := newSigmaProjectFKTarget(t, d)
	if err := d.SigmaCreateProject(domain.Project{ID: projectID, Title: "Charter Test"}); err != nil {
		t.Fatalf("SigmaCreateProject: %v", err)
	}

	original := domain.Charter{
		ID:               "charter-" + projectID,
		ProjectID:        projectID,
		ProblemStatement: "Cycle time exceeds SLA",
		BusinessCase:     "Losing $50k/month to expediting",
		GoalStatement:    "Reduce cycle time to 3 days",
		ScopeIn:          []string{"Order intake", "Fulfillment"},
		ScopeOut:         []string{"Returns"},
		CTQs: []domain.CTQ{
			{CustomerNeed: "Fast delivery", CTQ: "Cycle time", LowerSpec: 1.005, UpperSpec: 3.0},
			{CustomerNeed: "Accurate orders", CTQ: "Error rate", LowerSpec: 0.0, UpperSpec: 0.02},
		},
		Sponsor: "VP Ops",
	}
	if err := d.SigmaSaveCharter(original); err != nil {
		t.Fatalf("SigmaSaveCharter (create): %v", err)
	}

	got, err := d.SigmaGetCharter(projectID)
	if err != nil {
		t.Fatalf("SigmaGetCharter: %v", err)
	}
	if got.ProblemStatement != original.ProblemStatement || got.BusinessCase != original.BusinessCase || got.GoalStatement != original.GoalStatement {
		t.Fatalf("text fields = %+v, want %+v", got, original)
	}
	if len(got.ScopeIn) != 2 || got.ScopeIn[0] != "Order intake" || got.ScopeIn[1] != "Fulfillment" {
		t.Fatalf("ScopeIn = %v, want [Order intake Fulfillment]", got.ScopeIn)
	}
	if len(got.ScopeOut) != 1 || got.ScopeOut[0] != "Returns" {
		t.Fatalf("ScopeOut = %v, want [Returns]", got.ScopeOut)
	}
	if len(got.CTQs) != 2 {
		t.Fatalf("CTQs length = %d, want 2 (a marshal bug dropping all but the first element would produce 1)", len(got.CTQs))
	}
	if got.CTQs[0].LowerSpec != 1.005 {
		t.Errorf("CTQs[0].LowerSpec = %v, want 1.005 (non-round value, to catch float precision loss)", got.CTQs[0].LowerSpec)
	}
	if got.CTQs[1].CustomerNeed != "Accurate orders" || got.CTQs[1].UpperSpec != 0.02 {
		t.Errorf("CTQs[1] = %+v, want the second CTQ's own fields (not the first CTQ's, or zero values)", got.CTQs[1])
	}

	// ON CONFLICT(project_id) DO UPDATE path.
	updated := original
	updated.ProblemStatement = "Cycle time now exceeds SLA by 2x"
	updated.CTQs = []domain.CTQ{{CustomerNeed: "Fast delivery", CTQ: "Cycle time", LowerSpec: 2.5, UpperSpec: 4.0}}
	if err := d.SigmaSaveCharter(updated); err != nil {
		t.Fatalf("SigmaSaveCharter (update): %v", err)
	}
	gotUpdated, err := d.SigmaGetCharter(projectID)
	if err != nil {
		t.Fatalf("SigmaGetCharter after update: %v", err)
	}
	if gotUpdated.ProblemStatement != updated.ProblemStatement {
		t.Errorf("ProblemStatement after update = %q, want %q", gotUpdated.ProblemStatement, updated.ProblemStatement)
	}
	if len(gotUpdated.CTQs) != 1 || gotUpdated.CTQs[0].LowerSpec != 2.5 {
		t.Errorf("CTQs after update = %+v, want a single CTQ with LowerSpec 2.5 (ON CONFLICT must replace, not append)", gotUpdated.CTQs)
	}
}

func TestSigmaGetCharter_ReturnsDefaultForMissingProject(t *testing.T) {
	d := newBackupTestDB(t)
	got, err := d.SigmaGetCharter("no-such-project")
	if err != nil {
		t.Fatalf("SigmaGetCharter(missing): %v", err)
	}
	if got.ProjectID != "no-such-project" {
		t.Errorf("default charter ProjectID = %q, want %q", got.ProjectID, "no-such-project")
	}
	if got.ID != "" || got.ProblemStatement != "" {
		t.Errorf("default charter should be otherwise zero-valued: %+v", got)
	}
}

// TestSigmaSaveAndGet_RoundTripsAndDefaultsToEmpty is table-driven
// across Solutions, ControlPlan, SIPOC, and VoC: all four share the same
// INSERT ... ON CONFLICT(project_id) DO UPDATE shape and the same
// "no row yet" default-value contract, so one shared harness covers all
// four without four near-duplicate test functions (each getter's
// malformed-JSON path is already covered by
// TestSigmaGettersRejectMalformedJSON in sigma_test.go).
//
// Solutions/ControlPlan return an allocated empty slice ([]T{}) on no
// rows; SIPOC/VoC return a struct pointer whose inner slice is nil --
// both are real, distinct contracts (pinned the same way
// ListAuditEvents' nil-vs-empty-slice contract was pinned earlier this
// session), so each case's "empty" assertion checks the actual shape,
// not just a length.
func TestSigmaSaveAndGet_RoundTripsAndDefaultsToEmpty(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := newSigmaProjectFKTarget(t, d)
	if err := d.SigmaCreateProject(domain.Project{ID: projectID, Title: "Save/Get Test"}); err != nil {
		t.Fatalf("SigmaCreateProject: %v", err)
	}

	t.Run("solutions", func(t *testing.T) {
		empty, err := d.SigmaGetSolutions(projectID)
		if err != nil {
			t.Fatalf("SigmaGetSolutions (no rows): %v", err)
		}
		if empty == nil || len(empty) != 0 {
			t.Fatalf("SigmaGetSolutions (no rows) = %#v, want a non-nil empty slice", empty)
		}

		want := []domain.Solution{{ID: "s1", Title: "Automate handoff", Impact: 8, Effort: 3, Selected: true, Status: "pilot"}}
		if err := d.SigmaSaveSolutions(projectID, want); err != nil {
			t.Fatalf("SigmaSaveSolutions: %v", err)
		}
		got, err := d.SigmaGetSolutions(projectID)
		if err != nil {
			t.Fatalf("SigmaGetSolutions: %v", err)
		}
		if len(got) != 1 || got[0].Title != "Automate handoff" || !got[0].Selected {
			t.Errorf("SigmaGetSolutions round trip = %+v, want %+v", got, want)
		}
	})

	t.Run("control plan", func(t *testing.T) {
		empty, err := d.SigmaGetControlPlan(projectID)
		if err != nil {
			t.Fatalf("SigmaGetControlPlan (no rows): %v", err)
		}
		if empty == nil || len(empty) != 0 {
			t.Fatalf("SigmaGetControlPlan (no rows) = %#v, want a non-nil empty slice", empty)
		}

		want := []domain.ControlPlanItem{{ID: "cp1", ProcessStep: "Intake", Metric: "Cycle time", Owner: "Line lead"}}
		if err := d.SigmaSaveControlPlan(projectID, want); err != nil {
			t.Fatalf("SigmaSaveControlPlan: %v", err)
		}
		got, err := d.SigmaGetControlPlan(projectID)
		if err != nil {
			t.Fatalf("SigmaGetControlPlan: %v", err)
		}
		if len(got) != 1 || got[0].ProcessStep != "Intake" || got[0].Owner != "Line lead" {
			t.Errorf("SigmaGetControlPlan round trip = %+v, want %+v", got, want)
		}
	})

	t.Run("sipoc", func(t *testing.T) {
		empty, err := d.SigmaGetSIPOC(projectID)
		if err != nil {
			t.Fatalf("SigmaGetSIPOC (no rows): %v", err)
		}
		if empty.Elements != nil {
			t.Fatalf("SigmaGetSIPOC (no rows).Elements = %#v, want nil", empty.Elements)
		}
		if empty.ProjectID != projectID {
			t.Errorf("SigmaGetSIPOC (no rows).ProjectID = %q, want %q", empty.ProjectID, projectID)
		}

		want := domain.SIPOCData{
			ProjectID:   projectID,
			ProcessName: "Order fulfillment",
			Elements:    []domain.SIPOCElement{{ID: "e1", Category: "supplier", Description: "Vendor"}},
		}
		if err := d.SigmaSaveSIPOC(projectID, want); err != nil {
			t.Fatalf("SigmaSaveSIPOC: %v", err)
		}
		got, err := d.SigmaGetSIPOC(projectID)
		if err != nil {
			t.Fatalf("SigmaGetSIPOC: %v", err)
		}
		if got.ProcessName != "Order fulfillment" || len(got.Elements) != 1 || got.Elements[0].Category != "supplier" {
			t.Errorf("SigmaGetSIPOC round trip = %+v, want %+v", got, want)
		}
	})

	t.Run("voc", func(t *testing.T) {
		empty, err := d.SigmaGetVoC(projectID)
		if err != nil {
			t.Fatalf("SigmaGetVoC (no rows): %v", err)
		}
		if empty.Entries != nil {
			t.Fatalf("SigmaGetVoC (no rows).Entries = %#v, want nil", empty.Entries)
		}

		want := domain.VoCData{
			ProjectID: projectID,
			Entries:   []domain.VoCEntry{{ID: "v1", CustomerNeed: "Fast delivery", CTQ: "Cycle time", Priority: 1, Source: "survey"}},
		}
		if err := d.SigmaSaveVoC(projectID, want); err != nil {
			t.Fatalf("SigmaSaveVoC: %v", err)
		}
		got, err := d.SigmaGetVoC(projectID)
		if err != nil {
			t.Fatalf("SigmaGetVoC: %v", err)
		}
		if len(got.Entries) != 1 || got.Entries[0].CustomerNeed != "Fast delivery" || got.Entries[0].Priority != 1 {
			t.Errorf("SigmaGetVoC round trip = %+v, want %+v", got, want)
		}
	})
}
