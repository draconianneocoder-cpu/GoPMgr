// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

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
