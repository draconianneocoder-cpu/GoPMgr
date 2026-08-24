// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/agile"
	"gopmgr/internal/charts"
	"gopmgr/internal/db"
	"gopmgr/internal/timeline"
	"gopmgr/internal/users"
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

// TestBuildTimeline_CharterMilestonesReachTimelineThroughRealDocumentCreation
// is a regression test for a document-kind mismatch: projectMilestones once
// queried the literal string "charter", a kind no document can actually have
// -- every real creation path (App.NewDocument, the UI, and project-template
// seeding) only ever produces a kind from the documents registry, and
// Project Charter is registered as charter_word/charter_excel, never bare
// "charter". The bug went undetected because the other Charter-milestone
// tests in this file wrote "charter" directly via the lower-level
// db.SaveDocument, a shape no real user action can produce. This test
// instead creates the document through App.NewDocument so the kind string
// comes from the registry, the same as a real user action would.
func TestBuildTimeline_CharterMilestonesReachTimelineThroughRealDocumentCreation(t *testing.T) {
	for _, kind := range []string{"charter_word", "charter_excel"} {
		t.Run(kind, func(t *testing.T) {
			app, _, _ := newTimelineMoveTestApp(t)

			doc, err := app.NewDocument(kind, "")
			if err != nil {
				t.Fatalf("NewDocument(%q): %v", kind, err)
			}
			doc.Content = `{"milestones": [{"name": "Kickoff", "date": "2026-01-10"}]}`
			if _, err := app.SaveDocument(doc); err != nil {
				t.Fatalf("SaveDocument: %v", err)
			}

			entries, err := app.BuildTimeline()
			if err != nil {
				t.Fatalf("BuildTimeline: %v", err)
			}
			var found *timeline.Entry
			for i := range entries {
				if entries[i].Kind == timeline.KindMilestone && entries[i].Title == "Kickoff" {
					found = &entries[i]
				}
			}
			if found == nil {
				t.Fatalf("milestone from a real %s document did not reach the timeline: %#v", kind, entries)
			}
			if found.MilestoneSource != "charter" {
				t.Errorf("MilestoneSource = %q, want %q", found.MilestoneSource, "charter")
			}
		})
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
		Kind:      "charter_word",
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

// TestBuildTimeline_TwoIdenticalMilestonesGetDistinctSourceIDs guards the
// synthesized SourceID scheme (docID + array index) against two Charter
// milestones that share the same name and date -- the case the frontend's
// {#each entries} keying (source_id + kind + date) would collide on if the
// index weren't part of the ID.
func TestBuildTimeline_TwoIdenticalMilestonesGetDistinctSourceIDs(t *testing.T) {
	app, d, _ := newTimelineMoveTestApp(t)

	_, err := d.SaveDocument(db.Document{
		ID:        "charter-1",
		ProjectID: "project-1",
		Kind:      "charter_word",
		Title:     "Project Charter",
		Content: `{"milestones": [
			{"name": "Review", "date": "2026-03-01"},
			{"name": "Review", "date": "2026-03-01"}
		]}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	entries, err := app.BuildTimeline()
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	var ids []string
	for _, e := range entries {
		if e.Kind == timeline.KindMilestone {
			ids = append(ids, e.SourceID)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 milestone entries, got %d: %#v", len(ids), ids)
	}
	if ids[0] == ids[1] {
		t.Errorf("two same-name/same-date milestones got identical SourceID %q; frontend {#each} key would collide", ids[0])
	}
}

func TestBuildTimeline_IncludesScheduledChartMilestones(t *testing.T) {
	app, d, _ := newTimelineMoveTestApp(t)
	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	project.StartDate = "2026-01-05" // Monday; all dates below are weekdays.
	project.CountryCode = "US"
	if _, err := d.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	for _, chart := range []db.Chart{
		{
			ID:        "gantt-1",
			ProjectID: project.ID,
			Kind:      string(charts.KindGantt),
			Title:     "Delivery plan",
			Data: `{"nodes":[
				{"id":"design","label":"Design","duration":2},
				{"id":"review","label":"Review","duration":2,"milestone":true},
				{"id":"ship","label":"Ship","duration":0}
			],"edges":[{"from":"design","to":"review"},{"from":"review","to":"ship"}]}`,
		},
		{
			ID:        "cpm-1",
			ProjectID: project.ID,
			Kind:      string(charts.KindCPM),
			Title:     "Release plan",
			Data:      `{"nodes":[{"id":"release","label":"Release","duration":0}],"edges":[]}`,
		},
		{
			ID:        "wbs-1",
			ProjectID: project.ID,
			Kind:      string(charts.KindWBS),
			Title:     "Not a schedule",
			Data:      `{"nodes":[]}`,
		},
	} {
		if _, err := d.SaveChart(chart); err != nil {
			t.Fatalf("SaveChart(%s): %v", chart.ID, err)
		}
	}

	entries, err := app.BuildTimeline()
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	want := map[string]struct {
		title string
		date  string
	}{
		"chart:gantt-1:task:1:review": {"Delivery plan: Review", "2026-01-08"},
		"chart:gantt-1:task:2:ship":   {"Delivery plan: Ship", "2026-01-09"},
		"chart:cpm-1:task:0:release":  {"Release plan: Release", "2026-01-05"},
	}
	got := make(map[string]timeline.Entry)
	for _, entry := range entries {
		if entry.Kind == timeline.KindMilestone {
			got[entry.SourceID] = entry
		}
	}
	if len(got) != len(want) {
		t.Fatalf("chart milestones = %#v, want %d", got, len(want))
	}
	for id, expected := range want {
		entry, ok := got[id]
		if !ok {
			t.Errorf("missing chart milestone %q in %#v", id, got)
			continue
		}
		if entry.Title != expected.title || entry.Date.Format("2006-01-02") != expected.date {
			t.Errorf("chart milestone %q = %q on %s, want %q on %s", id, entry.Title, entry.Date.Format("2006-01-02"), expected.title, expected.date)
		}
		if entry.Editable {
			t.Errorf("chart milestone %q must be read-only", id)
		}
		if entry.MilestoneSource != "chart" {
			t.Errorf("chart milestone %q MilestoneSource = %q, want %q", id, entry.MilestoneSource, "chart")
		}
	}
}

func TestBuildTimeline_OmitsChartMilestonesWithoutProjectStart(t *testing.T) {
	app, d, _ := newTimelineMoveTestApp(t)
	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	project.StartDate = ""
	if _, err := d.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.SaveChart(db.Chart{
		ID:        "gantt-1",
		ProjectID: project.ID,
		Kind:      string(charts.KindGantt),
		Title:     "Delivery plan",
		Data:      `{"nodes":[{"id":"ship","label":"Ship","duration":0}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	entries, err := app.BuildTimeline()
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	for _, entry := range entries {
		if entry.SourceID == "chart:gantt-1:task:0:ship" {
			t.Fatalf("unanchored chart milestone unexpectedly present: %#v", entry)
		}
	}
}

// TestExportProjectICS_IncludesMilestone exercises the full iCal export
// path (not just BuildTimeline) with a Charter milestone present, since
// ExportProjectICS derives its VEVENT UID from SourceID+Kind independently
// of the Timeline view.
func TestExportProjectICS_IncludesMilestone(t *testing.T) {
	app, d, _ := newTimelineMoveTestApp(t)
	app.user = &users.Account{Username: "alice", DataDir: t.TempDir()}

	_, err := d.SaveDocument(db.Document{
		ID:        "charter-1",
		ProjectID: "project-1",
		Kind:      "charter_word",
		Title:     "Project Charter",
		Content:   `{"milestones": [{"name": "Kickoff", "date": "2026-01-10"}]}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	if _, err := d.SaveChart(db.Chart{
		ID:        "gantt-1",
		ProjectID: "project-1",
		Kind:      string(charts.KindGantt),
		Title:     "Delivery plan",
		Data:      `{"nodes":[{"id":"ship","label":"Ship","duration":0}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	path, err := app.ExportProjectICS(false)
	if err != nil {
		t.Fatalf("ExportProjectICS: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	ics := string(content)
	if !strings.Contains(ics, "SUMMARY:Kickoff") {
		t.Errorf("exported .ics missing milestone SUMMARY; got:\n%s", ics)
	}
	if !strings.Contains(ics, "UID:charter-1-0-milestone") {
		t.Errorf("exported .ics missing expected milestone UID; got:\n%s", ics)
	}
	if !strings.Contains(ics, "SUMMARY:Delivery plan: Ship") {
		t.Errorf("exported .ics missing chart milestone SUMMARY; got:\n%s", ics)
	}
	if !strings.Contains(ics, "UID:chart:gantt-1:task:0:ship-milestone") {
		t.Errorf("exported .ics missing chart milestone UID; got:\n%s", ics)
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
