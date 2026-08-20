// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package timeline assembles every dated entity in a project —
// sprints, agile deployments, project start/end, Charter milestones —
// into one chronological stream consumed by:
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

	"gopmgr/internal/agile"
	"gopmgr/internal/db"
)

// EntryKind enumerates the kinds of timeline events GoPMgr knows
// about. The GUI uses this to colour-code the strip.
type EntryKind string

const (
	KindSprintStart  EntryKind = "sprint_start"
	KindSprintEnd    EntryKind = "sprint_end"
	KindDeployment   EntryKind = "deployment"
	KindMilestone    EntryKind = "milestone"
	KindProjectStart EntryKind = "project_start"
	KindProjectEnd   EntryKind = "project_end"
)

// Entry is one event on the timeline.
type Entry struct {
	Kind        EntryKind `json:"kind"`
	Title       string    `json:"title"`
	Date        time.Time `json:"date"`
	EndDate     time.Time `json:"end_date,omitempty"` // for ranges (sprints)
	Description string    `json:"description,omitempty"`
	SourceID    string    `json:"source_id,omitempty"` // sprint ID, deployment ID, etc.
	Editable    bool      `json:"editable,omitempty"`
	EditField   string    `json:"edit_field,omitempty"`
}

// Milestone is a single dated entry from a document's structured
// "milestones" field (currently only the Charter template has one).
// The caller is responsible for extracting these from document
// content — this package stays database- and documents-package-free.
type Milestone struct {
	// ID identifies the source document + position so the frontend can
	// key the entry; it is not a persisted row ID, since milestone
	// objects have no ID field of their own in document content.
	ID   string
	Name string
	Date string // ISO-8601 or RFC3339, same formats parseDate accepts
}

// Build returns every timeline Entry for the project in ascending
// date order. The caller passes the project, a list of sprints and
// deployments, and any document-sourced milestones (all pre-fetched)
// so this package stays database-free.
func Build(project db.Project, sprints []agile.Sprint, deploys []agile.Deployment, milestones []Milestone) []Entry {
	var out []Entry

	if t, ok := parseDate(project.StartDate); ok {
		out = append(out, Entry{
			Kind:      KindProjectStart,
			Title:     project.Name + " — start",
			Date:      t,
			SourceID:  project.ID,
			Editable:  true,
			EditField: "start_date",
		})
	}
	if t, ok := parseDate(project.EndDate); ok {
		out = append(out, Entry{
			Kind:      KindProjectEnd,
			Title:     project.Name + " — end",
			Date:      t,
			SourceID:  project.ID,
			Editable:  true,
			EditField: "end_date",
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
				Editable:    true,
				EditField:   "start_date",
			})
		}
		if t, ok := parseDate(s.EndDate); ok {
			out = append(out, Entry{
				Kind:      KindSprintEnd,
				Title:     s.Name + " ends",
				Date:      t,
				SourceID:  s.ID,
				Editable:  true,
				EditField: "end_date",
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

	for _, m := range milestones {
		if t, ok := parseDate(m.Date); ok {
			out = append(out, Entry{
				Kind:     KindMilestone,
				Title:    m.Name,
				Date:     t,
				SourceID: m.ID,
			})
		}
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
