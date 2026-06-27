// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

func TestScenariosTableExists(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("scenarios")
	if err != nil {
		t.Fatalf("columnSet scenarios: %v", err)
	}
	for _, name := range []string{"id", "project_id", "name", "source_baseline_id", "description", "is_active", "created_at", "updated_at"} {
		if _, ok := cols[name]; !ok {
			t.Fatalf("scenarios.%s column missing", name)
		}
	}
}

func TestScenarioChartsTableExists(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("scenario_charts")
	if err != nil {
		t.Fatalf("columnSet scenario_charts: %v", err)
	}
	for _, name := range []string{"id", "scenario_id", "project_id", "source_chart_id", "source_baseline_id", "kind", "title", "data", "config", "baseline_data", "created_at", "updated_at"} {
		if _, ok := cols[name]; !ok {
			t.Fatalf("scenario_charts.%s column missing", name)
		}
	}
}

func TestScenarioCRUDAndActiveSelection(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Scenario Plan"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	base, err := d.SaveScenario(Scenario{
		ProjectID:        p.ID,
		Name:             "Accelerated procurement",
		SourceBaselineID: "baseline_123",
		Description:      "Pull long-lead equipment earlier.",
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("SaveScenario base: %v", err)
	}
	if base.ID == "" {
		t.Fatal("SaveScenario did not assign an ID")
	}
	if base.CreatedAt == "" || base.UpdatedAt == "" {
		t.Fatalf("timestamps not populated: %+v", base)
	}
	if !base.IsActive {
		t.Fatalf("base scenario is_active = false, want true")
	}

	second, err := d.SaveScenario(Scenario{
		ProjectID:   p.ID,
		Name:        "Staffing delay",
		Description: "Delay field mobilization.",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("SaveScenario second: %v", err)
	}
	if !second.IsActive {
		t.Fatalf("second scenario is_active = false, want true")
	}

	got, err := d.GetScenario(base.ID)
	if err != nil {
		t.Fatalf("GetScenario base: %v", err)
	}
	if got.IsActive {
		t.Fatalf("base scenario remained active after selecting second: %+v", got)
	}

	list, err := d.ListScenarios(p.ID)
	if err != nil {
		t.Fatalf("ListScenarios: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListScenarios length = %d, want 2", len(list))
	}
	activeCount := 0
	for _, s := range list {
		if s.IsActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("active scenario count = %d, want 1: %+v", activeCount, list)
	}

	second.Name = "Staffing delay - revised"
	second.IsActive = false
	updated, err := d.SaveScenario(second)
	if err != nil {
		t.Fatalf("SaveScenario update: %v", err)
	}
	if updated.Name != "Staffing delay - revised" || updated.IsActive {
		t.Fatalf("updated scenario mismatch: %+v", updated)
	}

	if err := d.DeleteScenario(base.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}
	list, err = d.ListScenarios(p.ID)
	if err != nil {
		t.Fatalf("ListScenarios after delete: %v", err)
	}
	if len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("after delete list = %+v, want second only", list)
	}
}

func TestBranchScenarioChartCopiesChartAndBaselineData(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Scenario Branch Plan"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{
		ProjectID: p.ID,
		Kind:      "cpm",
		Title:     "Master CPM",
		Data:      `{"nodes":[{"id":"a","label":"A"}],"edges":[]}`,
		Config:    `{"scale":"workday"}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	baseline, err := d.SaveBaseline(Baseline{
		ProjectID: p.ID,
		ChartID:   chart.ID,
		Name:      "Approved baseline",
		Data:      `{"a":{"early_start":0,"early_finish":5}}`,
	})
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{
		ProjectID:        p.ID,
		Name:             "Accelerated procurement",
		SourceBaselineID: baseline.ID,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	branched, err := d.BranchScenarioChart(scenario.ID, chart.ID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}
	if branched.ID == "" {
		t.Fatal("BranchScenarioChart did not assign an ID")
	}
	if branched.ScenarioID != scenario.ID || branched.ProjectID != p.ID {
		t.Fatalf("scenario chart scope mismatch: %+v", branched)
	}
	if branched.SourceChartID != chart.ID || branched.SourceBaselineID != baseline.ID {
		t.Fatalf("source references mismatch: %+v", branched)
	}
	if branched.Data != chart.Data || branched.Config != chart.Config {
		t.Fatalf("chart copy mismatch: %+v", branched)
	}
	if branched.BaselineData != baseline.Data {
		t.Fatalf("baseline_data = %q, want %q", branched.BaselineData, baseline.Data)
	}

	chart.Data = `{"nodes":[{"id":"a","label":"Changed"}],"edges":[]}`
	if _, err := d.SaveChart(chart); err != nil {
		t.Fatalf("SaveChart live update: %v", err)
	}
	got, err := d.GetScenarioChart(branched.ID)
	if err != nil {
		t.Fatalf("GetScenarioChart: %v", err)
	}
	if got.Data == chart.Data {
		t.Fatalf("scenario chart was not isolated from live chart update: %+v", got)
	}

	list, err := d.ListScenarioCharts(scenario.ID)
	if err != nil {
		t.Fatalf("ListScenarioCharts: %v", err)
	}
	if len(list) != 1 || list[0].ID != branched.ID {
		t.Fatalf("ListScenarioCharts = %+v, want branched chart", list)
	}
}
