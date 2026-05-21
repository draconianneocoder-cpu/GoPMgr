// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package timeline

import (
	"testing"
	"time"

	"pmforge/internal/agile"
	"pmforge/internal/db"
)

// TestBuildEmpty: a project with no dates and no sprints/deploys
// yields an empty timeline rather than failing.
func TestBuildEmpty(t *testing.T) {
	got := Build(db.Project{}, nil, nil)
	if len(got) != 0 {
		t.Errorf("empty inputs: want 0 entries, got %d", len(got))
	}
}

// TestBuildProjectDates: start_date and end_date on the project
// itself produce two entries, ordered chronologically.
func TestBuildProjectDates(t *testing.T) {
	p := db.Project{
		ID:        "p1",
		Name:      "Test",
		StartDate: "2026-01-15",
		EndDate:   "2026-06-30",
	}
	got := Build(p, nil, nil)
	if len(got) != 2 {
		t.Fatalf("want 2 entries (start + end), got %d", len(got))
	}
	if got[0].Kind != KindProjectStart {
		t.Errorf("[0]: want project_start, got %v", got[0].Kind)
	}
	if got[1].Kind != KindProjectEnd {
		t.Errorf("[1]: want project_end, got %v", got[1].Kind)
	}
	if !got[0].Date.Before(got[1].Date) {
		t.Errorf("start (%v) should be before end (%v)", got[0].Date, got[1].Date)
	}
}

// TestBuildSkipsEmptyDates: empty-string dates do not produce
// entries (we don't emit a "today" placeholder for missing data).
func TestBuildSkipsEmptyDates(t *testing.T) {
	p := db.Project{StartDate: "", EndDate: "2026-12-31"}
	got := Build(p, nil, nil)
	if len(got) != 1 {
		t.Errorf("want 1 entry (end only), got %d", len(got))
	}
}

// TestBuildSprintRangeAndDeployment: a planned sprint contributes
// two entries (start + end); a deployment contributes one. All four
// land in chronological order.
func TestBuildSprintRangeAndDeployment(t *testing.T) {
	sprints := []agile.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: "2026-02-01", EndDate: "2026-02-14", Goal: "g"},
	}
	deploys := []agile.Deployment{
		{ID: "d1", Version: "v1.0", TS: time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC), Successful: true},
	}
	got := Build(db.Project{ID: "p", StartDate: "2026-01-01"}, sprints, deploys)
	// project_start + sprint_start + deployment + sprint_end = 4
	if len(got) != 4 {
		t.Fatalf("want 4 entries, got %d", len(got))
	}
	// Order: project_start (Jan 1) → sprint_start (Feb 1) →
	// deployment (Feb 10) → sprint_end (Feb 14)
	wantKinds := []EntryKind{
		KindProjectStart,
		KindSprintStart,
		KindDeployment,
		KindSprintEnd,
	}
	for i, k := range wantKinds {
		if got[i].Kind != k {
			t.Errorf("[%d]: want %v, got %v", i, k, got[i].Kind)
		}
	}
}

// TestBuildAcceptsRFC3339: parseDate accepts both date-only and
// RFC3339 timestamps. (Useful when sprint dates come from anywhere.)
func TestBuildAcceptsRFC3339(t *testing.T) {
	sprints := []agile.Sprint{
		{ID: "s1", Name: "S", StartDate: "2026-03-01T09:00:00Z", EndDate: ""},
	}
	got := Build(db.Project{}, sprints, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Date.Year() != 2026 || got[0].Date.Month() != 3 {
		t.Errorf("parseDate dropped RFC3339 timestamp: got %v", got[0].Date)
	}
}

// TestBuildSkipsZeroDeployTS: a deployment with a zero TS is skipped
// (defensive — Go's time.Time zero value would otherwise sort first
// and corrupt the timeline).
func TestBuildSkipsZeroDeployTS(t *testing.T) {
	deploys := []agile.Deployment{
		{ID: "d1", Version: "v1.0"}, // TS not set
	}
	got := Build(db.Project{}, nil, deploys)
	if len(got) != 0 {
		t.Errorf("zero-TS deployment should be skipped; got %d entries", len(got))
	}
}
