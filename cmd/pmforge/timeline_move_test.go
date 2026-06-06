// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"testing"

	"pmforge/internal/agile"
	"pmforge/internal/db"
	"pmforge/internal/timeline"
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

func timelineContainsEditableDate(entries []timeline.Entry, kind, sourceID, date string) bool {
	for _, e := range entries {
		if string(e.Kind) == kind && e.SourceID == sourceID && e.Editable && e.Date.Format("2006-01-02") == date {
			return true
		}
	}
	return false
}
