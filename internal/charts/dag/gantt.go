// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"sort"
	"time"

	"gopmgr/internal/kernel"
)

// GanttRow is one schedule bar. ES/EF are working-day offsets (the
// frontend and pdfrender scale them onto their own axes); the date
// strings are present when the layout was calendar-anchored.
type GanttRow struct {
	ID                 string  `json:"id"`
	Label              string  `json:"label"`
	ES                 float64 `json:"es"`
	EF                 float64 `json:"ef"`
	Float              float64 `json:"float"`
	IsCritical         bool    `json:"is_critical"`
	Milestone          bool    `json:"milestone"`
	PercentComplete    float64 `json:"percent_complete"`
	StartDate          string  `json:"start_date,omitempty"`
	FinishDate         string  `json:"finish_date,omitempty"`
	Overallocated      bool    `json:"overallocated,omitempty"`
	ConstraintViolated bool    `json:"constraint_violated,omitempty"`
	// ConstraintUnapplied says why a date constraint is not in force
	// (kernel.ConstraintIssue); empty when it is, or there is none.
	ConstraintUnapplied string `json:"constraint_unapplied,omitempty"`
	// WorkSegments, when present, are the ABSOLUTE working-day runs of a
	// split task (offsets on the same axis as ES/EF). The renderer draws a
	// bar piece per segment instead of a single ES..EF bar.
	WorkSegments []WorkSegment `json:"work_segments,omitempty"`
}

// GanttDep is one dependency arrow between two rows.
type GanttDep struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// GanttLayout is the frontend-/pdfrender-ready Gantt shape.
type GanttLayout struct {
	Rows     []GanttRow `json:"rows"`
	Deps     []GanttDep `json:"deps"`
	Horizon  float64    `json:"horizon"` // max EF in working days
	Anchored bool       `json:"anchored"`
}

// LayoutGantt computes a Gantt layout from the shared layered/CPM
// document shape: full CPM (typed links, lag), overallocation flags,
// rows sorted by (ES, ID). Un-anchored: day offsets only, and date
// constraints are marked unapplied (no_start).
func LayoutGantt(doc LayeredDocument) (GanttLayout, error) {
	tasks := cpmTasksFromDoc(doc)
	kernel.MarkConstraintsUnapplied(tasks, kernel.ConstraintNoStart)
	if err := scheduleCPM(tasks); err != nil {
		return GanttLayout{}, err
	}
	kernel.DetectOverallocations(tasks, nil)
	copyCPMResults(doc, tasks)
	return ganttFromDoc(doc, tasks, false), nil
}

// LayoutGanttScheduledWithPlan is LayoutGantt with schedule context: date
// constraints armed, real dates on every row, and overallocation checked
// against plan (resource capacities, named resource calendars, and per-day
// capacity overrides).
func LayoutGanttScheduledWithPlan(doc LayeredDocument, projectStart time.Time, isWorkday kernel.WorkdayFunc, plan kernel.ResourceCapacityPlan) (GanttLayout, error) {
	tasks := cpmTasksFromDoc(doc)
	kernel.ApplyConstraintDates(tasks, projectStart, isWorkday)
	if err := scheduleCPM(tasks); err != nil {
		return GanttLayout{}, err
	}
	kernel.DetectOverallocationsWithPlan(tasks, plan)
	kernel.AnchorSchedule(tasks, projectStart, isWorkday)
	copyCPMResults(doc, tasks)
	return ganttFromDoc(doc, tasks, true), nil
}

// workDayRuns turns the working days a split task was scheduled on
// (kernel.RebuildSplitDays) into absolute bar pieces, one per run of
// consecutive days. A task the schedule runs in one piece, including one
// whose stored segments no longer fit it (a duration edited since
// levelling), gets none, so the bar shows what the schedule uses.
func workDayRuns(t *kernel.Task) []WorkSegment {
	if t == nil || len(t.WorkDays) == 0 {
		return nil
	}
	var runs []WorkSegment
	for _, d := range t.WorkDays {
		if n := len(runs); n > 0 && runs[n-1].End == float64(d) {
			runs[n-1].End++
			continue
		}
		runs = append(runs, WorkSegment{Start: float64(d), End: float64(d + 1)})
	}
	return runs
}

func ganttFromDoc(doc LayeredDocument, tasks map[string]*kernel.Task, anchored bool) GanttLayout {
	layout := GanttLayout{Anchored: anchored}
	for _, n := range doc.Nodes {
		layout.Rows = append(layout.Rows, GanttRow{
			ID:                  n.ID,
			Label:               n.Label,
			ES:                  n.ES,
			EF:                  n.EF,
			Float:               n.Float,
			IsCritical:          n.IsCritical,
			Milestone:           n.Milestone || n.Duration == 0,
			PercentComplete:     n.PercentComplete,
			StartDate:           n.StartDate,
			FinishDate:          n.FinishDate,
			Overallocated:       n.Overallocated,
			ConstraintViolated:  n.ConstraintViolated,
			ConstraintUnapplied: n.ConstraintUnapplied,
			WorkSegments:        workDayRuns(tasks[n.ID]),
		})
		// EF already ends at a split task's last working day.
		if n.EF > layout.Horizon {
			layout.Horizon = n.EF
		}
	}
	sort.Slice(layout.Rows, func(i, j int) bool {
		if layout.Rows[i].ES != layout.Rows[j].ES {
			return layout.Rows[i].ES < layout.Rows[j].ES
		}
		return layout.Rows[i].ID < layout.Rows[j].ID
	})
	for _, e := range doc.Edges {
		layout.Deps = append(layout.Deps, GanttDep(e))
	}
	return layout
}
