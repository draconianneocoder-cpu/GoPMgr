// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package timeline assembles every dated entity in a project —
// sprints, milestones from document fields, agile deployments,
// project start/end — into one chronological stream consumed by:
//
//   - the Timeline view (a horizontal strip rendering in Svelte)
//   - the iCal exporter (one VEVENT per entry)
//
// The package is read-only: it observes the project's data and
// produces a flat slice. Callers that want to embed the timeline
// in PDF combined reports can iterate the same slice.
package timeline

import (
	"sort"
	"time"

	"pmforge/internal/agile"
	"pmforge/internal/db"
)

// EntryKind enumerates the kinds of timeline events PMForge knows
// about. The GUI uses this to colour-code the strip.
type EntryKind string

const (
	KindSprintStart EntryKind = "sprint_start"
	KindSprintEnd   EntryKind = "sprint_end"
	KindDeployment  EntryKind = "deployment"
	KindMilestone   EntryKind = "milestone"
	KindProjectStart EntryKind = "project_start"
	KindProjectEnd   EntryKind = "project_end"
)

// Entry is one event on the timeline.
type Entry struct {
	Kind        EntryKind `json:"kind"`
	Title       string    `json:"title"`
	Date        time.Time `json:"date"`
	EndDate     time.Time `json:"end_date,omitempty"`  // for ranges (sprints)
	Description string    `json:"description,omitempty"`
	SourceID    string    `json:"source_id,omitempty"` // sprint ID, deployment ID, etc.
}

// Build returns every timeline Entry for the project in ascending
// date order. The caller passes the project + a list of sprints and
// deployments (pre-fetched) so this package stays database-free.
func Build(project db.Project, sprints []agile.Sprint, deploys []agile.Deployment) []Entry {
	var out []Entry

	if t, ok := parseDate(project.StartDate); ok {
		out = append(out, Entry{
			Kind:  KindProjectStart,
			Title: project.Name + " — start",
			Date:  t,
			SourceID: project.ID,
		})
	}
	if t, ok := parseDate(project.EndDate); ok {
		out = append(out, Entry{
			Kind:     KindProjectEnd,
			Title:    project.Name + " — end",
			Date:     t,
			SourceID: project.ID,
		})
	}

	for _, s := range sprints {
		if t, ok := parseDate(s.StartDate); ok {
			end, _ := parseDate(s.EndDate)
			out = append(out, Entry{
				Kind:        KindSprintStart,
				Title:       s.Name + " starts",
				Date:        t,
				EndDate:     end,
				Description: s.Goal,
				SourceID:    s.ID,
			})
		}
		if t, ok := parseDate(s.EndDate); ok {
			out = append(out, Entry{
				Kind:     KindSprintEnd,
				Title:    s.Name + " ends",
				Date:     t,
				SourceID: s.ID,
			})
		}
	}

	for _, d := range deploys {
		if d.TS.IsZero() {
			continue
		}
		title := "Deploy " + d.Version
		if !d.Successful {
			title += " (failed)"
		}
		out = append(out, Entry{
			Kind:        KindDeployment,
			Title:       title,
			Date:        d.TS,
			Description: d.Notes,
			SourceID:    d.ID,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date.Before(out[j].Date)
	})
	return out
}

// parseDate accepts both ISO-8601 dates (YYYY-MM-DD) and RFC3339
// timestamps, so timeline.Build is robust to either being supplied.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}
