// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"path/filepath"
	"testing"

	"pmforge/internal/db"
	"pmforge/internal/sigma/domain"
)

// newStatusTestSvc returns a ProjectService backed by a real temp database
// with one seeded project ("p1"), so GetToolStatus's DB-touching branches
// (define/analyze/improve/control) can be exercised end-to-end rather than
// relying on the nil-DB shortcut the input-validation tests in
// service_test.go use.
func newStatusTestSvc(t *testing.T) *ProjectService {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "status.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Conn.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	if _, err := d.Conn.Exec(`INSERT INTO project (id, name) VALUES (?, ?)`, "p1", "Status Test"); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := d.Conn.Exec(`INSERT INTO sigma_projects (id, title) VALUES (?, ?)`, "p1", "Status Test"); err != nil {
		t.Fatalf("insert sigma project: %v", err)
	}
	return &ProjectService{DB: d}
}

func toolStatus(t *testing.T, tools []ToolStatus, name string) string {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool.Status
		}
	}
	t.Fatalf("tool %q not found in %#v", name, tools)
	return ""
}

// --- define phase: Project Charter + SIPOC Diagram thresholds ---

func TestGetToolStatus_DefinePhase_NothingSaved_BothNotStarted(t *testing.T) {
	svc := newStatusTestSvc(t)
	result := svc.GetToolStatus("p1", "define")
	if got := toolStatus(t, result.Tools, "Project Charter"); got != "not_started" {
		t.Errorf("Project Charter = %q, want not_started", got)
	}
	if got := toolStatus(t, result.Tools, "SIPOC Diagram"); got != "not_started" {
		t.Errorf("SIPOC Diagram = %q, want not_started", got)
	}
	// Voice of Customer has no persistence wiring in GetToolStatus yet;
	// pin the current always-not_started behavior so a future change is
	// a deliberate, visible diff.
	if got := toolStatus(t, result.Tools, "Voice of Customer"); got != "not_started" {
		t.Errorf("Voice of Customer = %q, want not_started", got)
	}
}

func TestGetToolStatus_DefinePhase_PartialContent_Active(t *testing.T) {
	svc := newStatusTestSvc(t)
	// ProblemStatement under the 20-char completion threshold.
	if err := svc.SaveCharter(domain.Charter{ProjectID: "p1", ProblemStatement: "short"}); err != nil {
		t.Fatalf("SaveCharter: %v", err)
	}
	// Under the 5-element completion threshold.
	if err := svc.SaveSIPOC("p1", domain.SIPOCData{Elements: []domain.SIPOCElement{
		{Category: "supplier"}, {Category: "input"}, {Category: "process"},
	}}); err != nil {
		t.Fatalf("SaveSIPOC: %v", err)
	}

	result := svc.GetToolStatus("p1", "define")
	if got := toolStatus(t, result.Tools, "Project Charter"); got != "active" {
		t.Errorf("Project Charter = %q, want active", got)
	}
	if got := toolStatus(t, result.Tools, "SIPOC Diagram"); got != "active" {
		t.Errorf("SIPOC Diagram = %q, want active", got)
	}
}

func TestGetToolStatus_DefinePhase_FullContent_Completed(t *testing.T) {
	svc := newStatusTestSvc(t)
	// At-or-over the 20-char completion threshold.
	if err := svc.SaveCharter(domain.Charter{
		ProjectID:        "p1",
		ProblemStatement: "this problem statement is well over twenty characters long",
	}); err != nil {
		t.Fatalf("SaveCharter: %v", err)
	}
	// At-or-over the 5-element completion threshold.
	if err := svc.SaveSIPOC("p1", domain.SIPOCData{Elements: []domain.SIPOCElement{
		{Category: "supplier"}, {Category: "input"}, {Category: "process"},
		{Category: "output"}, {Category: "customer"},
	}}); err != nil {
		t.Fatalf("SaveSIPOC: %v", err)
	}

	result := svc.GetToolStatus("p1", "define")
	if got := toolStatus(t, result.Tools, "Project Charter"); got != "completed" {
		t.Errorf("Project Charter = %q, want completed", got)
	}
	if got := toolStatus(t, result.Tools, "SIPOC Diagram"); got != "completed" {
		t.Errorf("SIPOC Diagram = %q, want completed", got)
	}
}

// --- analyze phase: Fishbone Diagram is binary (no "active" state) ---

// GetFishbone never actually returns (nil, nil): on sql.ErrNoRows it
// returns the default 6-M skeleton (db.SigmaGetFishbone), so status.go's
// `fb != nil` check is defensive rather than reachable today. This test
// pins the practical case — an unsaved fishbone still has zero causes.
func TestGetToolStatus_AnalyzePhase_UnsavedFishbone_NotStarted(t *testing.T) {
	svc := newStatusTestSvc(t)
	result := svc.GetToolStatus("p1", "analyze")
	if got := toolStatus(t, result.Tools, "Fishbone Diagram"); got != "not_started" {
		t.Errorf("Fishbone Diagram = %q, want not_started", got)
	}
}

func TestGetToolStatus_AnalyzePhase_BranchWithNoCauses_NotStarted(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveFishbone(domain.FishboneData{
		Branches: []domain.FishboneBranch{{Category: "Method"}},
	}, "p1"); err != nil {
		t.Fatalf("SaveFishbone: %v", err)
	}
	result := svc.GetToolStatus("p1", "analyze")
	if got := toolStatus(t, result.Tools, "Fishbone Diagram"); got != "not_started" {
		t.Errorf("Fishbone Diagram = %q, want not_started (branch has no causes)", got)
	}
}

func TestGetToolStatus_AnalyzePhase_BranchWithCause_Completed(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveFishbone(domain.FishboneData{
		Branches: []domain.FishboneBranch{
			{Category: "Method"},
			{Category: "Machine", Causes: []domain.Cause{{ID: "c1", Description: "worn tooling"}}},
		},
	}, "p1"); err != nil {
		t.Fatalf("SaveFishbone: %v", err)
	}
	result := svc.GetToolStatus("p1", "analyze")
	if got := toolStatus(t, result.Tools, "Fishbone Diagram"); got != "completed" {
		t.Errorf("Fishbone Diagram = %q, want completed (one branch has a cause)", got)
	}
}

// --- improve phase: Solution Matrix tracks whether any solution is selected ---

func TestGetToolStatus_ImprovePhase_NoSolutions_NotStarted(t *testing.T) {
	svc := newStatusTestSvc(t)
	result := svc.GetToolStatus("p1", "improve")
	if got := toolStatus(t, result.Tools, "Solution Matrix"); got != "not_started" {
		t.Errorf("Solution Matrix = %q, want not_started", got)
	}
}

func TestGetToolStatus_ImprovePhase_SolutionsNoneSelected_Active(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveSolutions("p1", []domain.Solution{
		{Title: "Add inspection step", Selected: false},
	}); err != nil {
		t.Fatalf("SaveSolutions: %v", err)
	}
	result := svc.GetToolStatus("p1", "improve")
	if got := toolStatus(t, result.Tools, "Solution Matrix"); got != "active" {
		t.Errorf("Solution Matrix = %q, want active", got)
	}
}

func TestGetToolStatus_ImprovePhase_SolutionSelected_Completed(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveSolutions("p1", []domain.Solution{
		{Title: "Add inspection step", Selected: false},
		{Title: "Automate handoff", Selected: true},
	}); err != nil {
		t.Fatalf("SaveSolutions: %v", err)
	}
	result := svc.GetToolStatus("p1", "improve")
	if got := toolStatus(t, result.Tools, "Solution Matrix"); got != "completed" {
		t.Errorf("Solution Matrix = %q, want completed", got)
	}
}

// --- control phase: Control Plan tracks whether any row has an owner ---

func TestGetToolStatus_ControlPhase_NoControlPlan_NotStarted(t *testing.T) {
	svc := newStatusTestSvc(t)
	result := svc.GetToolStatus("p1", "control")
	if got := toolStatus(t, result.Tools, "Control Plan"); got != "not_started" {
		t.Errorf("Control Plan = %q, want not_started", got)
	}
}

func TestGetToolStatus_ControlPhase_ItemsNoOwner_Active(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveControlPlan("p1", []domain.ControlPlanItem{
		{ProcessStep: "Final inspection", Metric: "Defect rate"},
	}); err != nil {
		t.Fatalf("SaveControlPlan: %v", err)
	}
	result := svc.GetToolStatus("p1", "control")
	if got := toolStatus(t, result.Tools, "Control Plan"); got != "active" {
		t.Errorf("Control Plan = %q, want active", got)
	}
}

func TestGetToolStatus_ControlPhase_ItemHasOwner_Completed(t *testing.T) {
	svc := newStatusTestSvc(t)
	if err := svc.SaveControlPlan("p1", []domain.ControlPlanItem{
		{ProcessStep: "Final inspection", Metric: "Defect rate"},
		{ProcessStep: "Calibration check", Owner: "QA Lead"},
	}); err != nil {
		t.Fatalf("SaveControlPlan: %v", err)
	}
	result := svc.GetToolStatus("p1", "control")
	if got := toolStatus(t, result.Tools, "Control Plan"); got != "completed" {
		t.Errorf("Control Plan = %q, want completed", got)
	}
}

// --- GetProjectReportData ---

func TestGetProjectReportData_AssemblesAllPhaseData(t *testing.T) {
	// newStatusTestSvc already seeds a "p1" project row directly, so this
	// test only needs to attach phase data and read it back.
	svc := newStatusTestSvc(t)
	if err := svc.SaveCharter(domain.Charter{ProjectID: "p1", ProblemStatement: "widgets ship late"}); err != nil {
		t.Fatalf("SaveCharter: %v", err)
	}
	if err := svc.SaveSolutions("p1", []domain.Solution{{Title: "Automate handoff", Selected: true}}); err != nil {
		t.Fatalf("SaveSolutions: %v", err)
	}

	project, charter, sipoc, fishbone, solutions, controlPlan, err := svc.GetProjectReportData("p1")
	if err != nil {
		t.Fatalf("GetProjectReportData: %v", err)
	}
	if project.ID != "p1" {
		t.Errorf("project.ID = %q, want p1", project.ID)
	}
	if charter == nil || charter.ProblemStatement != "widgets ship late" {
		t.Errorf("charter = %#v, want ProblemStatement %q", charter, "widgets ship late")
	}
	// SIPOC, Fishbone, and Control Plan are auto-provisioned with default
	// structure the first time a project is read (e.g. Fishbone seeds the
	// 6 M's), so they come back non-nil even though this test never saved
	// them explicitly — only Elements/Causes/rows stay empty.
	if sipoc == nil || len(sipoc.Elements) != 0 {
		t.Errorf("sipoc = %#v, want non-nil with no elements", sipoc)
	}
	if fishbone == nil || len(fishbone.Branches) == 0 {
		t.Errorf("fishbone = %#v, want non-nil with default branches", fishbone)
	}
	for _, b := range fishbone.Branches {
		if len(b.Causes) != 0 {
			t.Errorf("branch %q has causes %#v, want none (never saved)", b.Category, b.Causes)
		}
	}
	if len(solutions) != 1 || !solutions[0].Selected {
		t.Errorf("solutions = %#v, want one selected solution", solutions)
	}
	if len(controlPlan) != 0 {
		t.Errorf("controlPlan = %#v, want empty (never saved)", controlPlan)
	}
}

func TestGetProjectReportData_UnknownProject_ReturnsError(t *testing.T) {
	svc := newStatusTestSvc(t)
	if _, _, _, _, _, _, err := svc.GetProjectReportData("does-not-exist"); err == nil {
		t.Fatal("GetProjectReportData(unknown) returned nil error, want error")
	}
}
