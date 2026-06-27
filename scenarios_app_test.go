// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"pmforge/internal/db"
)

func TestScenarioAppMethodsScopeToOpenProject(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Scenario Bridge Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}

	saved, err := app.SaveScenario(db.Scenario{
		ProjectID:        "wrong-project",
		Name:             "Accelerate vendor award",
		SourceBaselineID: "baseline_123",
		Description:      "Pull procurement into the first month.",
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	if saved.ProjectID != project.ID {
		t.Fatalf("SaveScenario project_id = %q, want open project %q", saved.ProjectID, project.ID)
	}
	if !saved.IsActive {
		t.Fatal("saved scenario is not active")
	}

	second, err := app.SaveScenario(db.Scenario{
		Name:        "Delay mobilization",
		Description: "Field start slips by two weeks.",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("SaveScenario second: %v", err)
	}
	if second.ProjectID != project.ID {
		t.Fatalf("second scenario project_id = %q, want open project %q", second.ProjectID, project.ID)
	}

	got, err := app.GetScenario(saved.ID)
	if err != nil {
		t.Fatalf("GetScenario: %v", err)
	}
	if got.IsActive {
		t.Fatalf("first scenario remained active after activating second: %+v", got)
	}

	list, err := app.ListScenarios()
	if err != nil {
		t.Fatalf("ListScenarios: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListScenarios length = %d, want 2", len(list))
	}
	if !list[0].IsActive || list[0].ID != second.ID {
		t.Fatalf("active scenario not first: %+v", list)
	}

	if err := app.DeleteScenario(saved.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}
	list, err = app.ListScenarios()
	if err != nil {
		t.Fatalf("ListScenarios after delete: %v", err)
	}
	if len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("after delete list = %+v, want second scenario only", list)
	}
}

func TestScenarioChartAppMethodsCopyScheduleData(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Scenario Chart Bridge Plan")

	chart, err := app.SaveChart(db.Chart{
		Kind:   "cpm",
		Title:  "Scenario Source CPM",
		Data:   `{"nodes":[{"id":"a","label":"A"}],"edges":[]}`,
		Config: `{"view":"network"}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	baseline, err := app.SetScheduleBaseline(chart.ID, "Approved")
	if err != nil {
		t.Fatalf("SetScheduleBaseline: %v", err)
	}
	scenario, err := app.SaveScenario(db.Scenario{
		Name:             "Scenario copy",
		SourceBaselineID: baseline.ID,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	copied, err := app.BranchScenarioChart(scenario.ID, chart.ID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}
	if copied.ScenarioID != scenario.ID || copied.SourceChartID != chart.ID {
		t.Fatalf("copied scenario chart references mismatch: %+v", copied)
	}
	if copied.BaselineData == "" || copied.BaselineData == "{}" {
		t.Fatalf("baseline_data was not copied: %+v", copied)
	}

	list, err := app.ListScenarioCharts(scenario.ID)
	if err != nil {
		t.Fatalf("ListScenarioCharts: %v", err)
	}
	if len(list) != 1 || list[0].ID != copied.ID {
		t.Fatalf("ListScenarioCharts = %+v, want copied chart", list)
	}
}
