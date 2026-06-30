// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package kernel implements PMForge's scheduling math: Critical Path
// Method (CPM) with typed links, lag, and constraints; calendar
// anchoring of CPM offsets onto real dates (AnchorSchedule); baseline
// comparison (CompareSchedules); Earned Value Management (ComputeEVM);
// and the data shapes consumed by the MSPDI exporter.
//
// The kernel is intentionally pure: no I/O, no globals, no database
// access. All inputs come in and all results come out via plain maps
// and structs so the algorithm can be unit-tested in isolation.
package kernel

import "sort"

// LinkType enumerates the four precedence-diagramming (PDM)
// relationship types between a predecessor and its successor.
type LinkType string

const (
	// FinishToStart: successor starts after predecessor finishes
	// (the classic CPM arrow; the default everywhere).
	FinishToStart LinkType = "FS"
	// StartToStart: successor starts after predecessor starts.
	StartToStart LinkType = "SS"
	// FinishToFinish: successor finishes after predecessor finishes.
	FinishToFinish LinkType = "FF"
	// StartToFinish: successor finishes after predecessor starts.
	StartToFinish LinkType = "SF"
)

// Link is one typed precedence relationship. Lag is in working days;
// a negative lag is a lead. The zero value of Type is normalised to
// FinishToStart by CalculateCPM.
type Link struct {
	Pred string   `json:"pred"`
	Type LinkType `json:"type"`
	Lag  float64  `json:"lag,omitempty"`
}

// ConstraintType enumerates the scheduling constraints a task may
// carry. The zero value (empty string) means ASAP, the default CPM
// behaviour.
type ConstraintType string

const (
	// AsSoonAsPossible is the default: the forward pass alone
	// determines the early dates.
	AsSoonAsPossible ConstraintType = "ASAP"
	// AsLateAsPossible schedules the task at its late dates,
	// consuming its own float. Needs no constraint date.
	AsLateAsPossible ConstraintType = "ALAP"
	// StartNoEarlierThan keeps ES at or after the constraint day.
	StartNoEarlierThan ConstraintType = "SNET"
	// FinishNoLaterThan caps LF at the constraint day; a forward pass
	// finishing after it flags a violation.
	FinishNoLaterThan ConstraintType = "FNLT"
	// MustFinishOn pins the finish to the constraint day exactly,
	// pulling the task later if links allow and flagging a violation
	// if links force it past the date.
	MustFinishOn ConstraintType = "MFO"
)

// Task is the scheduling-side view of an activity. The persistence
// layer (db.tasks) is a near-mirror, but kernel.Task carries the
// computed CPM fields (ES/EF/LS/LF/Float and IsCritical) that the GUI
// renders directly.
//
// Precedence can be expressed two ways: the legacy Precedents list
// (plain finish-to-start, zero lag) and the richer Links list
// (FS/SS/FF/SF with lag). Both are honoured; if the same predecessor
// appears in both, the typed Link wins.
//
// Date-bearing constraints (SNET/FNLT/MFO) carry their date in
// ConstraintDate (kernel.DateLayout). Because CPM runs in abstract
// working-day offsets, the date only takes effect after
// ApplyConstraintDates converts it to ConstraintDay against a project
// start + work calendar; an un-anchored schedule ignores date
// constraints. ALAP needs no date and always applies.
type Task struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Duration   float64  `json:"duration"`
	Precedents []string `json:"precedents"`
	Links      []Link   `json:"links,omitempty"`

	// DurationEstimate carries optional three-point schedule-risk
	// inputs for Monte Carlo simulation. When empty, Duration is used
	// as a deterministic sample.
	DurationEstimate DurationEstimate `json:"duration_estimate,omitempty"`

	// Scheduling constraint (see ConstraintType). Empty = ASAP.
	Constraint     ConstraintType `json:"constraint,omitempty"`
	ConstraintDate string         `json:"constraint_date,omitempty"`

	// Set by ApplyConstraintDates: ConstraintDay is the working-day
	// offset of ConstraintDate; ConstraintArmed reports that the
	// conversion succeeded and the constraint is in force.
	ConstraintDay   float64 `json:"constraint_day,omitempty"`
	ConstraintArmed bool    `json:"constraint_armed,omitempty"`

	// ConstraintViolated is set by CalculateCPM when precedence links
	// make the constraint unsatisfiable (MFO pushed past its date, or
	// FNLT finishing late).
	ConstraintViolated bool `json:"constraint_violated,omitempty"`

	// Progress tracking (reporting-only; never reschedules the plan).
	// PercentComplete is clamped to 0..100 by CalculateCPM. Milestone
	// marks an explicit milestone (conventionally zero duration).
	// ActualStart / ActualFinish record observed dates (DateLayout).
	PercentComplete float64 `json:"percent_complete,omitempty"`
	Milestone       bool    `json:"milestone,omitempty"`
	ActualStart     string  `json:"actual_start,omitempty"`
	ActualFinish    string  `json:"actual_finish,omitempty"`

	// Cost tracking for Earned Value Management (see ComputeEVM).
	// BudgetedCost is the task's budget at completion; ActualCost is
	// the cost incurred to date. The MinorUnits fields are canonical
	// when present; float fields remain for UI compatibility.
	BudgetedCost           float64 `json:"budgeted_cost,omitempty"`
	BudgetedCostMinorUnits int64   `json:"budgeted_cost_minor_units,omitempty"`
	ActualCost             float64 `json:"actual_cost,omitempty"`
	ActualCostMinorUnits   int64   `json:"actual_cost_minor_units,omitempty"`

	// Resource assignments (see resources.go). Overallocated is
	// computed by DetectOverallocations: true when any of this task's
	// resources is over capacity on a day the task occupies.
	Assignments   []Assignment `json:"assignments,omitempty"`
	Overallocated bool         `json:"overallocated,omitempty"`

	// CPM outputs — populated by CalculateCPM.
	ES         float64 `json:"es"`
	EF         float64 `json:"ef"`
	LS         float64 `json:"ls"`
	LF         float64 `json:"lf"`
	Float      float64 `json:"float"`
	IsCritical bool    `json:"is_critical"`

	// Calendar-anchored dates — populated by AnchorSchedule once a
	// project start date and work calendar are applied. Empty for an
	// un-anchored schedule. Layout: kernel.DateLayout (YYYY-MM-DD).
	StartDate  string `json:"start_date,omitempty"`
	FinishDate string `json:"finish_date,omitempty"`
}

// EffectiveLinks exposes the precedence merge (typed Links plus
// legacy Precedents as FS+0; a typed Link wins for a duplicated
// predecessor) for adapters such as the MSPDI exporter.
func EffectiveLinks(t *Task) []Link { return effectiveLinks(t) }

// effectiveLinks returns the task's typed links merged with its
// legacy Precedents (which become FS+0). A predecessor named in both
// keeps only its typed Link. Unknown link types normalise to FS.
func effectiveLinks(t *Task) []Link {
	links := make([]Link, 0, len(t.Links)+len(t.Precedents))
	seen := make(map[string]bool, len(t.Links))
	for _, l := range t.Links {
		switch l.Type {
		case FinishToStart, StartToStart, FinishToFinish, StartToFinish:
		default:
			l.Type = FinishToStart
		}
		links = append(links, l)
		seen[l.Pred] = true
	}
	for _, pID := range t.Precedents {
		if !seen[pID] {
			links = append(links, Link{Pred: pID, Type: FinishToStart})
		}
	}
	return links
}

// CalculateCPM runs the full Critical Path Method (precedence
// diagramming method) on the given task graph in place. The input map
// is keyed by Task.ID; every Precedents / Link.Pred entry MUST refer
// to a key in the map (dangling references are skipped).
//
// Algorithm:
//
//  1. Topologically sort the graph (predecessors before successors).
//  2. Forward pass — for each link into a task, the earliest-start
//     candidate is:
//     FS: pred.EF + lag    SS: pred.ES + lag
//     FF: pred.EF + lag − Duration    SF: pred.ES + lag − Duration
//     ES = max(0, candidates...), EF = ES + Duration. The clamp to 0
//     keeps a lead (negative lag) from scheduling before the project
//     start.
//  3. Backward pass — for each link out of a task, the latest-finish
//     candidate is:
//     FS: succ.LS − lag    SS: succ.LS − lag + Duration
//     FF: succ.LF − lag    SF: succ.LF − lag + Duration
//     LF = min(candidates...), defaulting to the project finish for
//     terminal tasks. LS = LF − Duration.
//  4. Float = LS − ES. Tasks with Float <= 0 lie on the critical path
//     (negative float means a constraint squeezed the schedule).
//
// Constraints: armed SNET/MFO act in the forward pass, MFO/FNLT in
// the backward pass, and ALAP in a post-pass that moves the task to
// its late dates. Links always win over constraints; an unsatisfiable
// constraint sets ConstraintViolated instead of breaking precedence.
//
// If the graph contains a cycle, CalculateCPM returns false; the task
// map is left in an undefined intermediate state.
func CalculateCPM(tasks map[string]*Task) bool {
	order, ok := topoSort(tasks)
	if !ok {
		return false
	}

	// --- Forward pass: ES / EF ---
	for _, id := range order {
		t := tasks[id]
		t.ConstraintViolated = false
		es := 0.0
		for _, l := range effectiveLinks(t) {
			p, exists := tasks[l.Pred]
			if !exists {
				continue
			}
			var candidate float64
			switch l.Type {
			case StartToStart:
				candidate = p.ES + l.Lag
			case FinishToFinish:
				candidate = p.EF + l.Lag - t.Duration
			case StartToFinish:
				candidate = p.ES + l.Lag - t.Duration
			default: // FinishToStart
				candidate = p.EF + l.Lag
			}
			if candidate > es {
				es = candidate
			}
		}

		// Forward-acting constraints.
		if t.ConstraintArmed {
			switch t.Constraint {
			case StartNoEarlierThan:
				if t.ConstraintDay > es {
					es = t.ConstraintDay
				}
			case MustFinishOn:
				target := t.ConstraintDay - t.Duration
				if target >= es {
					es = target
				} else {
					// Links force a finish past the pinned date.
					t.ConstraintViolated = true
				}
			}
		}

		t.ES = es
		t.EF = t.ES + t.Duration
	}

	// Project finish = max EF across all tasks.
	projectEF := 0.0
	for _, t := range tasks {
		if t.EF > projectEF {
			projectEF = t.EF
		}
	}

	// Reverse adjacency: successors[id] = links leaving id, paired
	// with the successor task they enter. Used in the backward pass.
	type succLink struct {
		succ *Task
		typ  LinkType
		lag  float64
	}
	successors := make(map[string][]succLink, len(tasks))
	for _, t := range tasks {
		for _, l := range effectiveLinks(t) {
			if _, exists := tasks[l.Pred]; exists {
				successors[l.Pred] = append(successors[l.Pred],
					succLink{succ: t, typ: l.Type, lag: l.Lag})
			}
		}
	}

	// --- Backward pass: LF / LS ---
	// Walk the topo order in reverse so every task is visited AFTER its
	// successors.
	for i := len(order) - 1; i >= 0; i-- {
		t := tasks[order[i]]
		// Every task is bounded by the project finish; successor links
		// can only tighten that. (With SS links a predecessor's finish
		// is not constrained by its successor at all, and with leads an
		// FS candidate can exceed the project finish — in both cases
		// the projectEF bound is what keeps float meaningful.)
		t.LF = projectEF
		for _, sl := range successors[t.ID] {
			var candidate float64
			switch sl.typ {
			case StartToStart:
				candidate = sl.succ.LS - sl.lag + t.Duration
			case FinishToFinish:
				candidate = sl.succ.LF - sl.lag
			case StartToFinish:
				candidate = sl.succ.LF - sl.lag + t.Duration
			default: // FinishToStart
				candidate = sl.succ.LS - sl.lag
			}
			if candidate < t.LF {
				t.LF = candidate
			}
		}

		// Backward-acting constraints.
		if t.ConstraintArmed {
			switch t.Constraint {
			case MustFinishOn:
				if t.LF < t.ConstraintDay-1e-9 {
					// A successor needs this task finished before its
					// pinned date.
					t.ConstraintViolated = true
				}
				t.LF = t.ConstraintDay
			case FinishNoLaterThan:
				if t.ConstraintDay < t.LF {
					t.LF = t.ConstraintDay
				}
				if t.EF > t.ConstraintDay+1e-9 {
					t.ConstraintViolated = true
				}
			}
		}

		t.LS = t.LF - t.Duration
		t.Float = t.LS - t.ES
		// Use a tiny epsilon to absorb floating-point noise. Float ~ 0
		// defines critical; a NEGATIVE float (possible when FNLT/MFO
		// constraints squeeze the schedule) is super-critical and is
		// flagged too.
		t.IsCritical = t.Float < 1e-9
	}

	// Clamp progress to its valid range (defensive against raw UI
	// input; reporting-only, so this never moves dates).
	for _, t := range tasks {
		if t.PercentComplete < 0 {
			t.PercentComplete = 0
		} else if t.PercentComplete > 100 {
			t.PercentComplete = 100
		}
	}

	// --- ALAP post-pass ---
	// As-late-as-possible tasks sit at their late dates, consuming
	// their own float. Applied after both passes so no other task's
	// dates shift (successors were computed from the early dates).
	for _, t := range tasks {
		if t.Constraint == AsLateAsPossible {
			t.ES, t.EF = t.LS, t.LF
			t.Float = 0
			t.IsCritical = true
		}
	}

	return true
}

// topoSort returns task IDs in dependency order (predecessors first).
// Returns (nil, false) if the graph contains a cycle.
//
// Implementation: Kahn's algorithm with deterministic ordering. The
// stable sort on `ready` keeps output reproducible across runs, which
// matters for snapshot-style tests.
func topoSort(tasks map[string]*Task) ([]string, bool) {
	indegree := make(map[string]int, len(tasks))
	for id := range tasks {
		indegree[id] = 0
	}
	for _, t := range tasks {
		for _, l := range effectiveLinks(t) {
			if _, ok := tasks[l.Pred]; ok {
				indegree[t.ID]++
			}
		}
	}

	// Reverse adjacency for the queue step.
	out := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		for _, l := range effectiveLinks(t) {
			if _, ok := tasks[l.Pred]; ok {
				out[l.Pred] = append(out[l.Pred], t.ID)
			}
		}
	}

	var ready []string
	for id, n := range indegree {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(tasks))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)

		successors := out[id]
		sort.Strings(successors)
		for _, sID := range successors {
			indegree[sID]--
			if indegree[sID] == 0 {
				ready = append(ready, sID)
				sort.Strings(ready)
			}
		}
	}

	if len(order) != len(tasks) {
		return nil, false // cycle
	}
	return order, true
}
