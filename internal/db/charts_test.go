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
// TestListDocuments_WholeSecondTimestampDoesNotPanicOrDropRows, closing
// the gap that test's own comment named: charts.go has the identical
// RFC3339Nano-string "ORDER BY updated_at DESC" pattern, and its Chart
// struct's doc comment previously (incorrectly) asserted the ordering
// "stays chronological" -- corrected alongside this test. Same mechanism:
// RFC3339Nano omits the fractional-seconds component entirely when it's
// exactly zero (formats as "...T10:00:00Z", not
// "...T10:00:00.000000000Z"), and '.' (0x2E) sorts before 'Z' (0x5A) in
// ASCII, so a whole-second save can sort AFTER a later same-second
// fractional save in both SQLite's TEXT ordering and Go's string "<".
//
// In practice this requires a save landing at time.Now() with exactly
// zero nanoseconds -- vanishingly unlikely under real wall-clock jitter
// -- so it is not expected to occur in normal use, and fixing the
// underlying format/query is out of this increment's scope (a
// production change affecting every RFC3339Nano-string-timestamped
// table in this package, not just charts and documents -- see
// .agent_memory/db-charts-timestamp-ordering-increment-2026-08-11.md for
// the full list). This assertion deliberately checks only that both
// charts still come back (no row silently dropped, no panic) rather than
// pinning which specific order results, mirroring the documents.go
// test's reasoning: the specific order is the bug, and asserting it
// would make this test fail the day someone fixes the ordering.
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
