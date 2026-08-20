// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"gopmgr/internal/agile"
	"gopmgr/internal/db"
	"gopmgr/internal/timeline"
)

func newTimelineMoveTestApp(t *testing.T) (*App, *db.Database, agile.Sprint) {
	t.Helper()

	d, err := db.InitDB(filepath.Join(t.TempDir(), "timeline-move.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	project, err := d.UpsertProject(db.Project{
		ID:        "project-1",
		Name:      "Timeline Move",
		StartDate: "2026-01-01",
		EndDate:   "2026-01-31",
	})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	store := agile.NewStore(d.Conn, project.ID)
	sprint, err := store.SaveSprint(agile.Sprint{
		ID:        "sprint-1",
		ProjectID: project.ID,
		Name:      "Sprint 1",
		Status:    agile.SprintPlanning,
		StartDate: "2026-01-05",
		EndDate:   "2026-01-12",
		Capacity:  12,
	})
	if err != nil {
		t.Fatalf("SaveSprint: %v", err)
	}

	return &App{db: d}, d, sprint
}

func TestMoveTimelineEntry_UpdatesProjectAndSprintDates(t *testing.T) {
	app, d, sprint := newTimelineMoveTestApp(t)

	entries, err := app.MoveTimelineEntry("project_start", "project-1", "2026-01-03")
	if err != nil {
		t.Fatalf("MoveTimelineEntry project_start: %v", err)
	}
	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if project.StartDate != "2026-01-03" {
		t.Fatalf("project start date = %q, want 2026-01-03", project.StartDate)
	}
	if !timelineContainsEditableDate(entries, "project_start", "project-1", "2026-01-03") {
		t.Fatalf("returned timeline did not include editable moved project start: %#v", entries)
	}

	entries, err = app.MoveTimelineEntry("sprint_end", sprint.ID, "2026-01-15")
	if err != nil {
		t.Fatalf("MoveTimelineEntry sprint_end: %v", err)
	}
	gotSprint, err := agile.NewStore(d.Conn, project.ID).GetSprint(sprint.ID)
	if err != nil {
		t.Fatalf("GetSprint: %v", err)
	}
	if gotSprint.EndDate != "2026-01-15" {
		t.Fatalf("sprint end date = %q, want 2026-01-15", gotSprint.EndDate)
	}
	if !timelineContainsEditableDate(entries, "sprint_end", sprint.ID, "2026-01-15") {
		t.Fatalf("returned timeline did not include editable moved sprint end: %#v", entries)
	}
}

func TestMoveTimelineEntry_RejectsReadOnlyAndInvalidMoves(t *testing.T) {
	app, _, _ := newTimelineMoveTestApp(t)

	if _, err := app.MoveTimelineEntry("deployment", "deploy-1", "2026-01-03"); err == nil {
		t.Fatal("deployment timeline moves should be rejected")
	}
	if _, err := app.MoveTimelineEntry("project_end", "project-1", "2025-12-31"); err == nil {
		t.Fatal("project end before project start should be rejected")
	}
	if _, err := app.MoveTimelineEntry("project_start", "wrong-project", "2026-01-03"); !errors.Is(err, errTimelineSourceMismatch) {
		t.Fatalf("source mismatch error = %v, want errTimelineSourceMismatch", err)
	}
}

// TestBuildTimeline_IncludesCharterMilestones is an end-to-end check (real
// DB, real document content JSON) that projectMilestones actually reaches
// BuildTimeline -- the unit tests in internal/timeline exercise Build()
// directly given a []timeline.Milestone, but not the JSON-extraction path.
func TestBuildTimeline_IncludesCharterMilestones(t *testing.T) {
	app, d, _ := newTimelineMoveTestApp(t)

	_, err := d.SaveDocument(db.Document{
		ID:        "charter-1",
		ProjectID: "project-1",
		Kind:      "charter",
		Title:     "Project Charter",
		Content: `{"milestones": [
			{"name": "Kickoff", "date": "2026-01-10"},
			{"name": "not a milestone", "date": ""},
			{"name": "", "date": "2026-02-01"}
		]}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	entries, err := app.BuildTimeline()
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}

	var found *timeline.Entry
	for i := range entries {
		if entries[i].Kind == timeline.KindMilestone {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no milestone entry in timeline: %#v", entries)
	}
	if found.Title != "Kickoff" {
		t.Errorf("milestone title = %q, want %q", found.Title, "Kickoff")
	}
	if found.SourceID != "charter-1-0" {
		t.Errorf("milestone source id = %q, want %q", found.SourceID, "charter-1-0")
	}
	if found.Editable {
		t.Error("milestone entries must not be Editable")
	}

	// Only one well-formed milestone: the empty-date and empty-name entries
	// in the fixture above must both be skipped, not turned into a zero-value
	// entry.
	count := 0
	for _, e := range entries {
		if e.Kind == timeline.KindMilestone {
			count++
		}
	}
	if count != 1 {
		t.Errorf("want exactly 1 milestone entry (malformed ones skipped), got %d", count)
	}
}

func timelineContainsEditableDate(entries []timeline.Entry, kind, sourceID, date string) bool {
	for _, e := range entries {
		if string(e.Kind) == kind && e.SourceID == sourceID && e.Editable && e.Date.Format("2006-01-02") == date {
			return true
		}
	}
	return false
}
