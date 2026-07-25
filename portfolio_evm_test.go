// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"
	"time"

	"pmforge/internal/analytics"
	"pmforge/internal/db"
)

func newPortfolioEVMTestProject(t *testing.T, startDate string) (*db.Database, db.Project) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "portfolio.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	project, err := d.UpsertProject(db.Project{
		ID:               "project-1",
		Name:             "Delivery",
		StartDate:        startDate,
		BudgetMinorUnits: 100_000,
		CountryCode:      "US",
		TimeZone:         "America/Chicago",
	})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.SaveStakeholder(db.Stakeholder{
		ProjectID:               project.ID,
		Name:                    "Supplier",
		Category:                db.StakeholderVendor,
		ContractValueMinorUnits: 20_000,
	}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	return d, project
}

func savePortfolioEVMChart(t *testing.T, d *db.Database, projectID, data string) {
	t.Helper()
	if _, err := d.SaveChart(db.Chart{
		ProjectID: projectID,
		Kind:      "cpm",
		Title:     "Current schedule",
		Data:      data,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
}

func mustPortfolioProjectMetrics(
	t *testing.T,
	d *db.Database,
	project db.Project,
	asOf time.Time,
) analytics.ProjectMetrics {
	t.Helper()
	got, err := portfolioProjectMetrics(d, project, "", asOf)
	if err != nil {
		t.Fatalf("portfolioProjectMetrics: %v", err)
	}
	return got
}

func TestPortfolioProjectMetricsKeepsCommittedAndEVMActualCostSeparate(t *testing.T) {
	d, project := newPortfolioEVMTestProject(t, "2026-01-05")
	savePortfolioEVMChart(t, d, project.ID, `{
		"nodes":[{
			"id":"task-1",
			"label":"Build",
			"duration":10,
			"percent_complete":50,
			"budgeted_cost_minor_units":10000,
			"actual_cost_minor_units":4000
		}],
		"edges":[]
	}`)

	got := mustPortfolioProjectMetrics(
		t,
		d,
		project,
		time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC),
	)

	if got.CommittedCostMinorUnits != 20_000 {
		t.Fatalf("CommittedCostMinorUnits = %d, want 20000", got.CommittedCostMinorUnits)
	}
	if got.ActualCostMinorUnits != 4_000 {
		t.Fatalf("ActualCostMinorUnits = %d, want task AC 4000", got.ActualCostMinorUnits)
	}
	if got.EarnedValueMinorUnits != 5_000 || got.PlannedValueMinorUnits != 5_000 {
		t.Fatalf(
			"EV/PV minor units = %d/%d, want 5000/5000",
			got.EarnedValueMinorUnits,
			got.PlannedValueMinorUnits,
		)
	}
	if !got.EVMAvailable {
		t.Fatal("EVMAvailable = false, want true for an anchored costed schedule")
	}
}

func TestPortfolioProjectMetricsMarksUnanchoredScheduleUnavailable(t *testing.T) {
	d, project := newPortfolioEVMTestProject(t, "")
	savePortfolioEVMChart(t, d, project.ID, `{
		"nodes":[{
			"id":"task-1",
			"label":"Build",
			"duration":10,
			"percent_complete":50,
			"budgeted_cost_minor_units":10000,
			"actual_cost_minor_units":4000
		}],
		"edges":[]
	}`)

	got := mustPortfolioProjectMetrics(
		t,
		d,
		project,
		time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC),
	)

	if got.EVMAvailable {
		t.Fatal("EVMAvailable = true, want false without a project start date")
	}
	if got.ActualCostMinorUnits != 0 || got.EarnedValueMinorUnits != 0 || got.PlannedValueMinorUnits != 0 {
		t.Fatalf(
			"unavailable EVM values = AC:%d EV:%d PV:%d, want all zero",
			got.ActualCostMinorUnits,
			got.EarnedValueMinorUnits,
			got.PlannedValueMinorUnits,
		)
	}
	if got.CommittedCostMinorUnits != 20_000 {
		t.Fatalf("CommittedCostMinorUnits = %d, want committed estimate preserved", got.CommittedCostMinorUnits)
	}
}

func TestPortfolioProjectMetricsRejectsCyclicSchedule(t *testing.T) {
	d, project := newPortfolioEVMTestProject(t, "2026-01-05")
	savePortfolioEVMChart(t, d, project.ID, `{
		"nodes":[
			{"id":"a","label":"A","duration":1,"budgeted_cost_minor_units":1000},
			{"id":"b","label":"B","duration":1,"budgeted_cost_minor_units":1000}
		],
		"edges":[
			{"from":"a","to":"b"},
			{"from":"b","to":"a"}
		]
	}`)

	got := mustPortfolioProjectMetrics(
		t,
		d,
		project,
		time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC),
	)

	if got.EVMAvailable {
		t.Fatal("EVMAvailable = true, want false for a cyclic schedule")
	}
}
