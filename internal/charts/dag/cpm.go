// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"strconv"
	"strings"
	"time"

	"pmforge/internal/kernel"
)

// CPM (Critical Path Method) annotates each node with ES/EF/LS/LF,
// computes Float, and marks IsCritical=true for any node whose float
// is zero. The math lives in internal/kernel, so this file is a thin
// adapter between the LayeredDocument shape (used by the chart layer)
// and the kernel.Task shape (used by the scheduler).
//
// LayoutCPM mutates the input document in place. Date-bearing
// constraints (SNET/FNLT/MFO) are ignored on this un-anchored path;
// use LayoutCPMScheduled when a project start date is available.
func LayoutCPM(doc LayeredDocument) (Layout, error) {
	tasks := cpmTasksFromDoc(doc)
	if ok := kernel.CalculateCPM(tasks); !ok {
		return Layout{}, ErrCycle
	}
	kernel.DetectOverallocations(tasks, nil)
	copyCPMResults(doc, tasks)
	return LayoutLayered(doc, DefaultLayeredOptions())
}

// LayoutCPMScheduled is LayoutCPM with full schedule context: date
// constraints are armed against the project start + work calendar,
// the CPM passes honour them, and every node additionally gets
// calendar-anchored StartDate/FinishDate. isWorkday may be nil
// (every day working); capacities follows DetectOverallocations'
// convention (nil / missing entries = 1.0 per resource).
func LayoutCPMScheduled(doc LayeredDocument, projectStart time.Time, isWorkday kernel.WorkdayFunc, capacities map[string]float64) (Layout, error) {
	return LayoutCPMScheduledWithPlan(doc, projectStart, isWorkday, kernel.ResourceCapacityPlan{
		DefaultCapacity: 1,
		Capacities:      capacities,
	})
}

// LayoutCPMScheduledWithPlan is LayoutCPMScheduled with named
// resource calendars and per-day capacity overrides.
func LayoutCPMScheduledWithPlan(doc LayeredDocument, projectStart time.Time, isWorkday kernel.WorkdayFunc, plan kernel.ResourceCapacityPlan) (Layout, error) {
	tasks := cpmTasksFromDoc(doc)
	kernel.ApplyConstraintDates(tasks, projectStart, isWorkday)
	if ok := kernel.CalculateCPM(tasks); !ok {
		return Layout{}, ErrCycle
	}
	kernel.DetectOverallocationsWithPlan(tasks, plan)
	kernel.AnchorSchedule(tasks, projectStart, isWorkday)
	copyCPMResults(doc, tasks)
	return LayoutLayered(doc, DefaultLayeredOptions())
}

// cpmTasksFromDoc converts a LayeredDocument into the kernel task
// map: nodes become tasks, edges become typed links (ParseLinkLabel),
// and recognised constraint strings pass through.
func cpmTasksFromDoc(doc LayeredDocument) map[string]*kernel.Task {
	tasks := make(map[string]*kernel.Task, len(doc.Nodes))
	for _, n := range doc.Nodes {
		t := &kernel.Task{
			ID:                     n.ID,
			Title:                  n.Label,
			Duration:               n.Duration,
			DurationEstimate:       n.DurationEstimate,
			PercentComplete:        n.PercentComplete,
			Milestone:              n.Milestone,
			ActualStart:            n.ActualStart,
			ActualFinish:           n.ActualFinish,
			BudgetedCost:           n.BudgetedCost,
			BudgetedCostMinorUnits: n.BudgetedCostMinorUnits,
			ActualCost:             n.ActualCost,
			ActualCostMinorUnits:   n.ActualCostMinorUnits,
			Assignments:            n.Assignments,
		}
		switch kernel.ConstraintType(strings.ToUpper(strings.TrimSpace(n.Constraint))) {
		case kernel.AsLateAsPossible:
			t.Constraint = kernel.AsLateAsPossible
		case kernel.StartNoEarlierThan:
			t.Constraint, t.ConstraintDate = kernel.StartNoEarlierThan, n.ConstraintDate
		case kernel.FinishNoLaterThan:
			t.Constraint, t.ConstraintDate = kernel.FinishNoLaterThan, n.ConstraintDate
		case kernel.MustFinishOn:
			t.Constraint, t.ConstraintDate = kernel.MustFinishOn, n.ConstraintDate
		}
		tasks[n.ID] = t
	}
	for _, e := range doc.Edges {
		if t, ok := tasks[e.To]; ok {
			typ, lag := ParseLinkLabel(e.Label)
			t.Links = append(t.Links, kernel.Link{
				Pred: e.From,
				Type: typ,
				Lag:  lag,
			})
		}
	}
	return tasks
}

// copyCPMResults writes the kernel's outputs back into the shared
// node slice (LayoutCPM's documented in-place mutation).
func copyCPMResults(doc LayeredDocument, tasks map[string]*kernel.Task) {
	for i := range doc.Nodes {
		t, ok := tasks[doc.Nodes[i].ID]
		if !ok {
			continue
		}
		doc.Nodes[i].ES = t.ES
		doc.Nodes[i].EF = t.EF
		doc.Nodes[i].LS = t.LS
		doc.Nodes[i].LF = t.LF
		doc.Nodes[i].Float = t.Float
		doc.Nodes[i].IsCritical = t.IsCritical
		doc.Nodes[i].StartDate = t.StartDate
		doc.Nodes[i].FinishDate = t.FinishDate
		doc.Nodes[i].ConstraintViolated = t.ConstraintViolated
		doc.Nodes[i].Overallocated = t.Overallocated
	}
}

// ParseLinkLabel decodes a layered-edge label like "FS", "SS+2",
// "FF-1.5" into a PDM link type and lag (working days; negative =
// lead). The grammar is:
//
//	label  := [type] [sign number]
//	type   := FS | SS | FF | SF   (case-insensitive)
//	sign   := + | -
//
// An empty or unrecognisable label falls back to plain FS with zero
// lag, which matches the historic behaviour of unlabeled edges. A
// bare "+2" / "-1" is FS with that lag.
func ParseLinkLabel(label string) (kernel.LinkType, float64) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(label), " ", ""))
	if s == "" {
		return kernel.FinishToStart, 0
	}

	typ := kernel.FinishToStart
	switch {
	case strings.HasPrefix(s, "FS"):
		s = s[2:]
	case strings.HasPrefix(s, "SS"):
		typ, s = kernel.StartToStart, s[2:]
	case strings.HasPrefix(s, "FF"):
		typ, s = kernel.FinishToFinish, s[2:]
	case strings.HasPrefix(s, "SF"):
		typ, s = kernel.StartToFinish, s[2:]
	}

	if s == "" {
		return typ, 0
	}
	if s[0] != '+' && s[0] != '-' {
		// Unrecognised suffix (or a label that never was a link spec,
		// e.g. a free-text annotation): fail soft to FS+0.
		return kernel.FinishToStart, 0
	}
	lag, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return kernel.FinishToStart, 0
	}
	return typ, lag
}

// FormatLinkLabel is ParseLinkLabel's inverse: it renders a typed
// link as an edge label. Plain FS with zero lag yields "" (the
// historic unlabeled-edge form); lag is formatted without trailing
// zeros ("SS+2", "FF-1.5").
func FormatLinkLabel(typ kernel.LinkType, lag float64) string {
	name := string(typ)
	switch typ {
	case kernel.StartToStart, kernel.FinishToFinish, kernel.StartToFinish:
	default:
		name = string(kernel.FinishToStart)
	}
	if lag == 0 {
		if name == string(kernel.FinishToStart) {
			return ""
		}
		return name
	}
	sign := "+"
	if lag < 0 {
		sign = "" // strconv keeps the minus
	}
	return name + sign + strconv.FormatFloat(lag, 'f', -1, 64)
}

// AnchorCPMDates maps the ES/EF annotations LayoutCPM wrote into doc
// onto real calendar dates via kernel.AnchorSchedule, writing
// StartDate/FinishDate back to each node. Call it after LayoutCPM and
// only when a project start date is known; it is a no-op for an empty
// document. isWorkday may be nil (every day counts as working).
func AnchorCPMDates(doc *LayeredDocument, projectStart time.Time, isWorkday kernel.WorkdayFunc) {
	if doc == nil || len(doc.Nodes) == 0 {
		return
	}

	tasks := make(map[string]*kernel.Task, len(doc.Nodes))
	for _, n := range doc.Nodes {
		tasks[n.ID] = &kernel.Task{
			ID:       n.ID,
			Duration: n.Duration,
			ES:       n.ES,
			EF:       n.EF,
		}
	}

	kernel.AnchorSchedule(tasks, projectStart, isWorkday)

	for i := range doc.Nodes {
		if t, ok := tasks[doc.Nodes[i].ID]; ok {
			doc.Nodes[i].StartDate = t.StartDate
			doc.Nodes[i].FinishDate = t.FinishDate
		}
	}
}
