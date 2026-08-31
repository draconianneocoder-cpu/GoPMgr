// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package timeline

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/agile"
	"gopmgr/internal/db"
)

// TestBuildEmpty: a project with no dates and no sprints/deploys
// yields an empty timeline rather than failing.
func TestBuildEmpty(t *testing.T) {
	got := Build(db.Project{}, nil, nil, nil)
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
	got := Build(p, nil, nil, nil)
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
	got := Build(p, nil, nil, nil)
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
	got := Build(db.Project{ID: "p", StartDate: "2026-01-01"}, sprints, deploys, nil)
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
	got := Build(db.Project{}, sprints, nil, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Date.Year() != 2026 || got[0].Date.Month() != 3 {
		t.Errorf("parseDate dropped RFC3339 timestamp: got %v", got[0].Date)
	}
	// The sprint's own EndDate is empty, so the sprint_start entry must
	// get a nil EndDate pointer (not a &time.Time{} zero value) -- this
	// is the other branch of the `if e, ok := parseDate(s.EndDate); ok`
	// conditional in Build(); TestMarshalIncludesRealEndDate only
	// exercises the ok=true branch.
	if got[0].EndDate != nil {
		t.Errorf("sprint with no end date must produce a nil Entry.EndDate, got %v", *got[0].EndDate)
	}
}

// TestBuildSkipsZeroDeployTS: a deployment with a zero TS is skipped
// (defensive: Go's time.Time zero value would otherwise sort first
// and corrupt the timeline).
func TestBuildSkipsZeroDeployTS(t *testing.T) {
	deploys := []agile.Deployment{
		{ID: "d1", Version: "v1.0"}, // TS not set
	}
	got := Build(db.Project{}, nil, deploys, nil)
	if len(got) != 0 {
		t.Errorf("zero-TS deployment should be skipped; got %d entries", len(got))
	}
}

// TestBuildFailedDeploymentTitle: an unsuccessful deployment is labelled
// "(failed)" so the timeline distinguishes good releases from rollbacks.
func TestBuildFailedDeploymentTitle(t *testing.T) {
	deploys := []agile.Deployment{
		{ID: "d1", Version: "v2.0", TS: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), Successful: false},
	}
	got := Build(db.Project{}, nil, deploys, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Title != "Deploy v2.0 (failed)" {
		t.Errorf("failed deploy title: got %q, want %q", got[0].Title, "Deploy v2.0 (failed)")
	}
}

// TestBuildMilestones: Charter milestones become KindMilestone entries,
// sorted alongside everything else, and are never Editable — unlike
// sprint/project boundaries, a milestone has no dedicated MoveTimelineEntry
// case; it can only be changed by editing the source Charter document.
func TestBuildMilestones(t *testing.T) {
	milestones := []Milestone{
		{ID: "doc1-0", Name: "Kickoff", Date: "2026-01-10", Source: "charter"},
		{ID: "doc1-1", Name: "Beta", Date: "2026-05-01", Source: "chart"},
	}
	got := Build(db.Project{ID: "p", StartDate: "2026-01-01"}, nil, nil, milestones)
	if len(got) != 3 {
		t.Fatalf("want 3 entries (project_start + 2 milestones), got %d", len(got))
	}
	wantKinds := []EntryKind{KindProjectStart, KindMilestone, KindMilestone}
	for i, k := range wantKinds {
		if got[i].Kind != k {
			t.Errorf("[%d]: want %v, got %v", i, k, got[i].Kind)
		}
	}
	if got[1].Title != "Kickoff" || got[1].SourceID != "doc1-0" {
		t.Errorf("[1]: want title %q sourceID %q, got %q %q", "Kickoff", "doc1-0", got[1].Title, got[1].SourceID)
	}
	if got[1].Editable {
		t.Error("milestone entries must not be Editable: there is no MoveTimelineEntry case for them")
	}
	if got[1].MilestoneSource != "charter" || got[2].MilestoneSource != "chart" {
		t.Errorf("MilestoneSource not carried through: got[1]=%q, got[2]=%q, want %q, %q",
			got[1].MilestoneSource, got[2].MilestoneSource, "charter", "chart")
	}
}

// TestBuildSkipsMilestonesWithEmptyDate: a milestone with an unparseable
// or empty date is skipped, mirroring how project/sprint dates behave.
func TestBuildSkipsMilestonesWithEmptyDate(t *testing.T) {
	milestones := []Milestone{
		{ID: "doc1-0", Name: "No date", Date: ""},
		{ID: "doc1-1", Name: "Bad date", Date: "not-a-date"},
	}
	got := Build(db.Project{}, nil, nil, milestones)
	if len(got) != 0 {
		t.Errorf("want 0 entries, got %d", len(got))
	}
}

// TestMarshalOmitsUnsetEndDate: a point-in-time entry (no natural end
// date, e.g. project_end) must not serialize an end_date at all. Before
// EndDate became *time.Time, json's `omitempty` was a documented no-op
// on the non-pointer time.Time field, so the zero value always
// serialized as "0001-01-01T00:00:00Z" -- silently corrupting any
// consumer that treats a present end_date as meaningful (the frontend's
// holiday-range lookup did exactly that, producing a query spanning
// ~2000 years instead of the intended date window).
func TestMarshalOmitsUnsetEndDate(t *testing.T) {
	p := db.Project{ID: "p1", Name: "Testing", EndDate: "2026-08-31"}
	got := Build(p, nil, nil, nil)
	if len(got) != 1 || got[0].Kind != KindProjectEnd {
		t.Fatalf("want 1 project_end entry, got %+v", got)
	}
	b, err := json.Marshal(got[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"end_date":`) {
		t.Errorf("point-event entry must omit the end_date key entirely, got %s", b)
	}
}

// TestMarshalIncludesRealEndDate: a sprint_start entry with a genuine
// end date must still serialize it -- the pointer conversion must not
// regress the case omitempty was never broken for.
func TestMarshalIncludesRealEndDate(t *testing.T) {
	sprints := []agile.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: "2026-02-01", EndDate: "2026-02-14"},
	}
	got := Build(db.Project{}, sprints, nil, nil)
	if len(got) != 2 || got[0].Kind != KindSprintStart {
		t.Fatalf("want sprint_start first, got %+v", got)
	}
	if got[0].EndDate == nil {
		t.Fatal("sprint_start with a real end date must carry a non-nil EndDate")
	}
	b, err := json.Marshal(got[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"end_date":"2026-02-14`) {
		t.Errorf("want end_date present with the real value, got %s", b)
	}
}

// TestParseDate exercises parseDate directly across its accepted formats
// and rejection paths. The RFC3339Nano layout is a superset of RFC3339,
// so a non-empty unparseable string is the only way to reach the final
// false return.
func TestParseDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"iso date", "2026-05-15", true},
		{"rfc3339", "2026-05-15T08:30:00Z", true},
		{"rfc3339 nano", "2026-05-15T08:30:00.123456789Z", true},
		{"garbage", "not-a-date", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDate(tt.in)
			if ok != tt.want {
				t.Errorf("parseDate(%q) ok = %v, want %v", tt.in, ok, tt.want)
			}
			if ok && got.IsZero() {
				t.Errorf("parseDate(%q) returned ok but zero time", tt.in)
			}
		})
	}
}
