// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"testing"
)

// newBaselineFixture creates the project + chart rows the baselines
// table's foreign keys require, returning their IDs.
func newBaselineFixture(t *testing.T, d *Database) (projectID, chartID string) {
	t.Helper()
	p, err := d.UpsertProject(Project{Name: "Baseline Test Project"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	c, err := d.SaveChart(Chart{ProjectID: p.ID, Kind: "cpm", Title: "Schedule"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	return p.ID, c.ID
}

func TestBaselineCRUD(t *testing.T) {
	d := newBackupTestDB(t)
	projectID, chartID := newBaselineFixture(t, d)

	saved, err := d.SaveBaseline(Baseline{
		ProjectID: projectID,
		ChartID:   chartID,
		Name:      "Plan of record",
		Data:      `{"A":{"id":"A","duration":2}}`,
	})
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("SaveBaseline did not assign an ID")
	}
	if saved.CreatedAt.IsZero() {
		t.Error("SaveBaseline did not set CreatedAt")
	}

	got, err := d.GetBaseline(saved.ID)
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if got.Name != "Plan of record" || got.Data != saved.Data {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	second, err := d.SaveBaseline(Baseline{
		ProjectID: projectID, ChartID: chartID, Name: "Replan",
	})
	if err != nil {
		t.Fatalf("SaveBaseline (second): %v", err)
	}
	if second.Data != "{}" {
		t.Errorf("empty Data should default to {}, got %q", second.Data)
	}

	list, err := d.ListBaselines(chartID)
	if err != nil {
		t.Fatalf("ListBaselines: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListBaselines returned %d rows, want 2", len(list))
	}
	if list[0].ID != second.ID {
		t.Error("ListBaselines must order newest first")
	}

	if err := d.DeleteBaseline(saved.ID); err != nil {
		t.Fatalf("DeleteBaseline: %v", err)
	}
	if _, err := d.GetBaseline(saved.ID); err == nil {
		t.Error("GetBaseline after delete must fail")
	}
	list, _ = d.ListBaselines(chartID)
	if len(list) != 1 {
		t.Errorf("after delete: %d rows, want 1", len(list))
	}
}

func TestListBaselinesEmptyChart(t *testing.T) {
	d := newBackupTestDB(t)
	list, err := d.ListBaselines("no-such-chart")
	if err != nil {
		t.Fatalf("ListBaselines: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected no baselines, got %d", len(list))
	}
}

// A scenario names its source baseline and BranchScenarioChart looks it up
// again on every branch, so that baseline cannot be deleted until the
// scenario's source changes. Nothing is deleted or audited on refusal.
func TestDeleteBaselineRefusesAScenarioSource(t *testing.T) {
	d := newBackupTestDB(t)
	projectID, chartID := newBaselineFixture(t, d)
	base, err := d.SaveBaseline(Baseline{ProjectID: projectID, ChartID: chartID, Name: "Plan of record"})
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	scenario, err := d.SaveScenario(Scenario{ProjectID: projectID, Name: "Late vendor", SourceBaselineID: base.ID})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	countAudit := func() int {
		t.Helper()
		var n int
		if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE event_type = 'baseline.delete'`).Scan(&n); err != nil {
			t.Fatalf("count audit events: %v", err)
		}
		return n
	}

	err = d.DeleteBaseline(base.ID)
	var inUse *BaselineInUseError
	if !errors.As(err, &inUse) || inUse.Scenario != "Late vendor" {
		t.Fatalf("DeleteBaseline of a scenario source: err = %v, want *BaselineInUseError naming Late vendor", err)
	}
	if _, err := d.GetBaseline(base.ID); err != nil {
		t.Fatalf("refused delete removed the baseline: %v", err)
	}
	if n := countAudit(); n != 0 {
		t.Fatalf("refused delete wrote %d audit events", n)
	}

	scenario.SourceBaselineID = ""
	if _, err := d.SaveScenario(scenario); err != nil {
		t.Fatalf("SaveScenario (clear source): %v", err)
	}
	if err := d.DeleteBaseline(base.ID); err != nil {
		t.Fatalf("DeleteBaseline after the scenario stopped using it: %v", err)
	}
	if n := countAudit(); n != 1 {
		t.Fatalf("delete wrote %d audit events, want 1", n)
	}
}
