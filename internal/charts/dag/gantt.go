// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"sort"
	"time"

	"pmforge/internal/kernel"
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
// rows sorted by (ES, ID). Un-anchored: day offsets only.
func LayoutGantt(doc LayeredDocument) (GanttLayout, error) {
	tasks := cpmTasksFromDoc(doc)
	if ok := kernel.CalculateCPM(tasks); !ok {
		return GanttLayout{}, ErrCycle
	}
	kernel.DetectOverallocations(tasks, nil)
	copyCPMResults(doc, tasks)
	return ganttFromDoc(doc, false), nil
}

// LayoutGanttScheduled is LayoutGantt with schedule context: date
// constraints armed, real dates on every row, and overallocation
// checked against the given capacities (nil = 1.0 per resource).
func LayoutGanttScheduled(doc LayeredDocument, projectStart time.Time, isWorkday kernel.WorkdayFunc, capacities map[string]float64) (GanttLayout, error) {
	tasks := cpmTasksFromDoc(doc)
	kernel.ApplyConstraintDates(tasks, projectStart, isWorkday)
	if ok := kernel.CalculateCPM(tasks); !ok {
		return GanttLayout{}, ErrCycle
	}
	kernel.DetectOverallocations(tasks, capacities)
	kernel.AnchorSchedule(tasks, projectStart, isWorkday)
	copyCPMResults(doc, tasks)
	return ganttFromDoc(doc, true), nil
}

func ganttFromDoc(doc LayeredDocument, anchored bool) GanttLayout {
	layout := GanttLayout{Anchored: anchored}
	for _, n := range doc.Nodes {
		layout.Rows = append(layout.Rows, GanttRow{
			ID:                 n.ID,
			Label:              n.Label,
			ES:                 n.ES,
			EF:                 n.EF,
			Float:              n.Float,
			IsCritical:         n.IsCritical,
			Milestone:          n.Milestone || n.Duration == 0,
			PercentComplete:    n.PercentComplete,
			StartDate:          n.StartDate,
			FinishDate:         n.FinishDate,
			Overallocated:      n.Overallocated,
			ConstraintViolated: n.ConstraintViolated,
		})
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
		layout.Deps = append(layout.Deps, GanttDep{From: e.From, To: e.To, Label: e.Label})
	}
	return layout
}
