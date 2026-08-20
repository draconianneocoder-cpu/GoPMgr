// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

// setChartUpdatedAt directly overwrites a chart's updated_at column,
// bypassing SaveChart's time.Now()-derived value. Same rationale as
// documents_read_test.go's setDocumentUpdatedAt: deterministic control
// over timestamps is necessary to exercise specific orderings without
// relying on wall-clock jitter between calls.
func setChartUpdatedAt(t *testing.T, d *Database, id, updatedAt string) {
	t.Helper()
	if _, err := d.Conn.Exec(`UPDATE charts SET updated_at = ? WHERE id = ?`, updatedAt, id); err != nil {
		t.Fatalf("set updated_at for %s: %v", id, err)
	}
}

func TestListCharts_OrdersByMostRecentlyUpdatedDescending(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Charts"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	oldest, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Oldest"})
	if err != nil {
		t.Fatalf("SaveChart oldest: %v", err)
	}
	middle, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Middle"})
	if err != nil {
		t.Fatalf("SaveChart middle: %v", err)
	}
	newest, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "Newest"})
	if err != nil {
		t.Fatalf("SaveChart newest: %v", err)
	}
	// Clearly-ordered fractional timestamps, all within the same second,
	// deliberately avoiding the whole-second-vs-fractional edge case
	// pinned separately below.
	setChartUpdatedAt(t, d, oldest.ID, "2026-01-01T10:00:00.1Z")
	setChartUpdatedAt(t, d, middle.ID, "2026-01-01T10:00:00.5Z")
	setChartUpdatedAt(t, d, newest.ID, "2026-01-01T10:00:00.9Z")

	got, err := d.ListCharts(project.ID, "")
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListCharts returned %d charts, want 3", len(got))
	}
	wantOrder := []string{"Newest", "Middle", "Oldest"}
	for i, want := range wantOrder {
		if got[i].Title != want {
			t.Errorf("ListCharts order[%d] = %q, want %q (full order: %v)", i, got[i].Title, want,
				[]string{got[0].Title, got[1].Title, got[2].Title})
		}
	}
}

// TestListCharts_WholeSecondTimestampDoesNotPanicOrDropRows is charts.go's
// counterpart to documents_read_test.go's
// TestListDocuments_WholeSecondTimestampDoesNotPanicOrDropRows.
//
// Historical note: ListCharts originally ordered by the RFC3339Nano TEXT
// `updated_at` column, which has a real string-ordering hazard (a
// whole-second save formats without a fractional component and can sort
// after a later same-second fractional save). That hazard was closed by
// internal/db/timestamps.go's `updated_at_unixnano` retrofit -- ListCharts
// now orders by that INTEGER column instead, which has no such ambiguity.
//
// This test's setChartUpdatedAt helper only overwrites the TEXT
// `updated_at` column, not `updated_at_unixnano`, so it can no longer
// reproduce the original hazard against the live query. It still pins the
// one thing it asserts: both rows must come back present, in either order,
// with no panic and no row silently dropped.
func TestListCharts_WholeSecondTimestampDoesNotPanicOrDropRows(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Charts"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	wholeSecond, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "ExactSecond"})
	if err != nil {
		t.Fatalf("SaveChart wholeSecond: %v", err)
	}
	halfSecondLater, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "HalfSecondLater"})
	if err != nil {
		t.Fatalf("SaveChart halfSecondLater: %v", err)
	}
	setChartUpdatedAt(t, d, wholeSecond.ID, "2026-01-01T10:00:00Z")
	setChartUpdatedAt(t, d, halfSecondLater.ID, "2026-01-01T10:00:00.5Z")

	got, err := d.ListCharts(project.ID, "")
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListCharts returned %d charts, want 2 (both rows must survive regardless of ordering)", len(got))
	}
	gotTitles := map[string]bool{got[0].Title: true, got[1].Title: true}
	if !gotTitles["ExactSecond"] || !gotTitles["HalfSecondLater"] {
		t.Errorf("ListCharts returned %v, want both ExactSecond and HalfSecondLater present (in either order)", []string{got[0].Title, got[1].Title})
	}
}
