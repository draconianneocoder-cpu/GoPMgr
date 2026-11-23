// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package kernel implements GoPMgr's scheduling math: Critical Path
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
	// consuming its own float, unless it has an armed levelled start:
	// then it stays where levelling put it. Needs no constraint date.
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
	// MustStartOn pins the start to the constraint day exactly: early
	// and late start both. Links still win: a task whose links force a
	// later start keeps that start and is flagged, as Primavera P6's
	// "Start On" does. Microsoft Project's default "honor constraint
	// dates" instead holds MSO, MFO, SNLT and FNLT against the logic.
	MustStartOn ConstraintType = "MSO"
	// StartNoLaterThan caps the late start at the constraint day; an
	// early start after it flags a violation.
	StartNoLaterThan ConstraintType = "SNLT"
	// FinishNoEarlierThan keeps EF at or after the constraint day. It
	// only delays, so it is never violated.
	FinishNoEarlierThan ConstraintType = "FNET"
)

// IsDateConstraint reports whether a constraint type carries a date that
// ApplyConstraintDates must convert before it takes effect.
func IsDateConstraint(c ConstraintType) bool {
	switch c {
	case StartNoEarlierThan, StartNoLaterThan, MustStartOn,
		FinishNoEarlierThan, FinishNoLaterThan, MustFinishOn:
		return true
	}
	return false
}

// isFinishConstraint reports whether a date constraint limits the
// finish (compared with the exclusive EF) rather than the start (ES).
func isFinishConstraint(c ConstraintType) bool {
	return c == FinishNoEarlierThan || c == FinishNoLaterThan || c == MustFinishOn
}

// ConstraintIssue is why a date constraint was not applied.
type ConstraintIssue string

const (
	// ConstraintInvalidDate: the constraint date is missing or is not a
	// YYYY-MM-DD date.
	ConstraintInvalidDate ConstraintIssue = "invalid_date"
	// ConstraintOutOfRange: the date lies too far from the project start
	// for the calendar walk (about 270 years of working days or more).
	ConstraintOutOfRange ConstraintIssue = "out_of_range"
	// ConstraintNoStart: the project has no start date, so no date can be
	// placed. Set by MarkConstraintsUnapplied.
	ConstraintNoStart ConstraintIssue = "no_start"
)

// MarkConstraintsUnapplied records reason on every task with a date
// constraint. Callers that schedule without a project start, and so never
// call ApplyConstraintDates, use it with ConstraintNoStart.
func MarkConstraintsUnapplied(tasks map[string]*Task, reason ConstraintIssue) {
	for _, t := range tasks {
		t.ConstraintArmed = false
		t.ConstraintUnapplied = ""
		if IsDateConstraint(t.Constraint) {
			t.ConstraintUnapplied = reason
		}
	}
}

// ForwardConstrainedStart is the start an armed date constraint gives a
// task whose links allow linkStart: SNET and FNET delay it, MSO and MFO
// pull it to their date when the links allow. It does not apply the
// levelled start and sets no violation; the forward pass and the levelling
// persistence pass both use it, so they agree.
func ForwardConstrainedStart(t *Task, linkStart float64) float64 {
	es := linkStart
	if !t.ConstraintArmed {
		return es
	}
	switch t.Constraint {
	case StartNoEarlierThan:
		es = max(es, t.ConstraintDay)
	case MustFinishOn:
		if target := t.ConstraintDay - t.Duration; target >= es {
			es = target
		}
	case MustStartOn:
		if t.ConstraintDay >= es {
			es = t.ConstraintDay
		}
	case FinishNoEarlierThan:
		es = max(es, t.ConstraintDay-t.Duration)
	}
	return es
}

// linkCandidate is the earliest start one link allows a successor of the
// given duration whose predecessor runs [predES, predEF). The forward pass
// and LevelStartsToKeep share it.
func linkCandidate(typ LinkType, lag, predES, predEF, duration float64) float64 {
	switch typ {
	case StartToStart:
		return predES + lag
	case FinishToFinish:
		return predEF + lag - duration
	case StartToFinish:
		return predES + lag - duration
	default: // FinishToStart
		return predEF + lag
	}
}

// lateConstraintDay is the offset a latest limit compares with: the last
// working day on or before the constraint date (see ConstraintLateShift).
func lateConstraintDay(t *Task) float64 {
	return t.ConstraintDay - t.ConstraintLateShift
}

// WorkSegment is one half-open run of planned work, expressed as offsets
// relative to a task's scheduled start. Resource leveling persists these
// segments when a task is interrupted; EVM uses them to avoid accruing
// planned value during the intervening idle periods.
type WorkSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

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
// Date-bearing constraints (SNET, SNLT, MSO, FNET, FNLT, MFO) carry their date in
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

	// ConstraintLateShift is how many working days the constraint's latest
	// limit lies before ConstraintDay. ConstraintDay reads the date as the
	// first working day on or after it, which is right for an earliest
	// limit (SNET, FNET, and the early side of MSO and MFO). A latest
	// limit (SNLT, FNLT, and the late side of MSO and MFO) reads it as the
	// last working day on or before it, which is earlier when the date is a
	// non-working day or lies before the project's first working day. Zero
	// means both readings agree, so a task armed directly keeps that
	// meaning. Set by ApplyConstraintDates.
	ConstraintLateShift float64 `json:"constraint_late_shift,omitempty"`

	// ConstraintUnapplied says why a date constraint is not in force, so
	// the editor can show it instead of scheduling the task as if it had
	// none without notice (KRN-01 b). Empty when armed or not dated.
	ConstraintUnapplied ConstraintIssue `json:"constraint_unapplied,omitempty"`

	// LevelStart is the start date (DateLayout) resource levelling gave
	// the task. It is not a constraint: levelling owns it, writing and
	// clearing it on each run, and never touches Constraint. Once
	// ApplyConstraintDates has armed it (LevelStartArmed, LevelStartDay:
	// the first working day on or after it), the forward pass starts the
	// task no earlier, before any constraint is checked, so a levelled
	// delay that breaks a latest limit is flagged. It is never violated
	// itself. LevelResourcesWithOptions disarms it, because levelling
	// recomputes it from the logic and the user's constraints.
	LevelStart string `json:"level_start,omitempty"`
	// LevelStartFraction is the share of LevelStart's working day already
	// passed when the task starts (0 <= f < 1; owner decision D-STORE,
	// 2026-11-23): levelling in part days can start a task part-way
	// through a day. ApplyConstraintDates arms LevelStartDay as the day's
	// offset plus this fraction; a value outside [0, 1) is read as 0.
	LevelStartFraction float64 `json:"level_start_fraction,omitempty"`
	LevelStartDay      float64 `json:"level_start_day,omitempty"`
	LevelStartArmed    bool    `json:"level_start_armed,omitempty"`
	// LevelStartBinding is set by the forward pass when the levelled start
	// is later than the start the links give, so levelling, not the
	// logic, holds the task where it is.
	// Not serialised: it is recomputed on every run. Since 2026-11-23 it
	// compares with LevelEarliest (links plus forward constraints).
	LevelStartBinding bool `json:"-"`
	// LevelEarliest is set by the forward pass: the start the links and the
	// forward constraints give, without the levelled start. A levelled
	// delay is measured from it (persistence, MSPDI LevelingDelay).
	LevelEarliest float64 `json:"-"`

	// ConstraintViolated is set by CalculateCPM when precedence links
	// make the constraint unsatisfiable: an MSO or MFO task pushed past,
	// or needed before, its pinned date; an SNLT task starting late; an
	// FNLT task finishing late.
	ConstraintViolated bool `json:"constraint_violated,omitempty"`

	// Progress tracking (reporting-only; never reschedules the plan).
	// PercentComplete is clamped to 0..100 by CalculateCPM. Milestone
	// marks an explicit milestone (conventionally zero duration).
	// ActualStart / ActualFinish record observed dates (DateLayout).
	//
	// Timeline aggregation includes this task, and any zero-duration task,
	// from Gantt/CPM charts after calendar-aware layout. See
	// timeline.Milestone (internal/timeline/timeline.go) for the date and
	// unanchored-project policy.
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

	// WorkDays, when non-empty, lists the exact working-day offsets a task
	// occupies its resources on. It is set only by resource leveling with
	// AllowSplitting (activity splitting); an empty slice means the task
	// occupies its contiguous ES..EF span. ResourceUsage and
	// DetectOverallocations consult it so a split task's idle days are not
	// counted as resource demand.
	WorkDays []int `json:"work_days,omitempty"`

	// PlannedWorkSegments preserves a persisted split plan as offsets
	// relative to ES. Unlike WorkDays, these segments are reporting input:
	// they remain stable when CPM moves the task and let EVM pause planned
	// value accrual during a split task's idle gaps.
	PlannedWorkSegments []WorkSegment `json:"work_segments,omitempty"`

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
// Constraints: armed SNET, FNET, MSO and MFO act in the forward pass;
// MSO, MFO, SNLT and FNLT in the backward pass; and ALAP in a post-pass that moves the task to
// its late dates (an ALAP task with an armed levelled start stays at it). Links always win over constraints; an unsatisfiable
// constraint sets ConstraintViolated instead of breaking precedence.
// Earliest limits use ConstraintDay and latest limits ConstraintDay minus
// ConstraintLateShift, so an MSO or MFO dated on a non-working day, or
// before the project's first working day, can never be met and is flagged.
//
// CalculateCPM returns false, leaving no CPM results on any task, if the
// graph contains a cycle or the schedule is longer than MaxScheduleDays.
// ScheduleCPM does the same work and says which.
func CalculateCPM(tasks map[string]*Task) bool {
	return ScheduleCPM(tasks) == nil
}

// cpmPlan is the part of CalculateCPM that depends only on the network's
// shape: the topological order and, for each task, its links resolved to
// the tasks at their other ends. Building it once lets the Monte Carlo
// simulation rerun the passes with new durations without re-sorting the
// network or re-deriving its links every iteration. A plan holds pointers
// to the tasks it was built from and is only valid while their links and
// constraints are unchanged.
type cpmPlan struct {
	order []*Task
	preds [][]cpmLink // links into order[i], in effectiveLinks order
	succs [][]cpmLink // links out of order[i]
}

// cpmLink is one link, resolved to the task at its other end.
type cpmLink struct {
	task *Task
	typ  LinkType
	lag  float64
}

// newCPMPlan sorts tasks and resolves their links. It changes no task and
// reports false for a cyclic network. Links to tasks missing from the map
// are dropped. Successor lists are keyed by the predecessor's map key and
// read by Task.ID, as the schedule pass always has; callers key the map by
// Task.ID.
func newCPMPlan(tasks map[string]*Task) (*cpmPlan, bool) {
	ids, ok := topoSort(tasks)
	if !ok {
		return nil, false
	}
	plan := &cpmPlan{
		order: make([]*Task, len(ids)),
		preds: make([][]cpmLink, len(ids)),
		succs: make([][]cpmLink, len(ids)),
	}
	successors := make(map[string][]cpmLink, len(tasks))
	for i, id := range ids {
		t := tasks[id]
		plan.order[i] = t
		for _, l := range effectiveLinks(t) {
			p, exists := tasks[l.Pred]
			if !exists {
				continue
			}
			plan.preds[i] = append(plan.preds[i], cpmLink{task: p, typ: l.Type, lag: l.Lag})
			successors[l.Pred] = append(successors[l.Pred], cpmLink{task: t, typ: l.Type, lag: l.Lag})
		}
	}
	for i, t := range plan.order {
		plan.succs[i] = successors[t.ID]
	}
	return plan, true
}

// run computes every task's dates, float and critical flag from its current
// Duration. It reads no output of an earlier run, so it can be repeated
// after changing durations.
func (plan *cpmPlan) run() {
	// --- Forward pass: ES / EF ---
	for i, t := range plan.order {
		t.ConstraintViolated = false
		t.LevelStartBinding = false
		es := 0.0
		for _, l := range plan.preds[i] {
			if candidate := linkCandidate(l.typ, l.lag, l.task.ES, l.task.EF, t.Duration); candidate > es {
				es = candidate
			}
		}

		// Forward-acting constraints, then the levelled start: LevelEarliest
		// is the start the links and constraints give, the start levelling
		// measures its delay from.
		es = ForwardConstrainedStart(t, es)
		t.LevelEarliest = es
		if t.LevelStartArmed && t.LevelStartDay > es {
			// The epsilon keeps noise from summed fractional durations
			// from naming levelling as the cause.
			t.LevelStartBinding = t.LevelStartDay > es+1e-9
			es = t.LevelStartDay
		}
		// Links or a non-working date force a start (MSO) or finish (MFO)
		// past the date, so no start or finish can fall on it.
		if t.ConstraintArmed {
			switch t.Constraint {
			case MustStartOn:
				if es > lateConstraintDay(t)+1e-9 {
					t.ConstraintViolated = true
				}
			case MustFinishOn:
				if es+t.Duration > lateConstraintDay(t)+1e-9 {
					t.ConstraintViolated = true
				}
			}
		}

		t.ES = es
		t.EF = t.ES + t.Duration
	}

	// Project finish = max EF across all tasks.
	projectEF := 0.0
	for _, t := range plan.order {
		if t.EF > projectEF {
			projectEF = t.EF
		}
	}

	// --- Backward pass: LF / LS ---
	// Walk the topo order in reverse so every task is visited AFTER its
	// successors.
	for i := len(plan.order) - 1; i >= 0; i-- {
		t := plan.order[i]
		// Every task is bounded by the project finish; successor links
		// can only tighten that. (With SS links a predecessor's finish
		// is not constrained by its successor at all, and with leads an
		// FS candidate can exceed the project finish — in both cases
		// the projectEF bound is what keeps float meaningful.)
		t.LF = projectEF
		for _, sl := range plan.succs[i] {
			var candidate float64
			switch sl.typ {
			case StartToStart:
				candidate = sl.task.LS - sl.lag + t.Duration
			case FinishToFinish:
				candidate = sl.task.LF - sl.lag
			case StartToFinish:
				candidate = sl.task.LF - sl.lag + t.Duration
			default: // FinishToStart
				candidate = sl.task.LS - sl.lag
			}
			if candidate < t.LF {
				t.LF = candidate
			}
		}

		// Backward-acting constraints.
		if t.ConstraintArmed {
			late := lateConstraintDay(t)
			switch t.Constraint {
			case MustFinishOn:
				if t.LF < late-1e-9 {
					// A successor needs this task finished before its
					// pinned date.
					t.ConstraintViolated = true
				}
				t.LF = late
			case FinishNoLaterThan:
				if late < t.LF {
					t.LF = late
				}
				if t.EF > late+1e-9 {
					t.ConstraintViolated = true
				}
			case MustStartOn:
				pinned := late + t.Duration
				if t.LF < pinned-1e-9 {
					// A successor needs this task finished before its
					// pinned start allows.
					t.ConstraintViolated = true
				}
				t.LF = pinned
			case StartNoLaterThan:
				if capped := late + t.Duration; capped < t.LF {
					t.LF = capped
				}
				if t.ES > late+1e-9 {
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
	for _, t := range plan.order {
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
	// An ALAP task with an armed levelled start stays where levelling put
	// it (owner decision 2026-11-23, "keep levelled position"): levelling
	// placed it as late as capacity allowed, and moving it to the late
	// dates of the saved schedule could put it on top of other work.
	for _, t := range plan.order {
		if t.Constraint == AsLateAsPossible && !t.LevelStartArmed {
			t.ES, t.EF = t.LS, t.LF
			t.Float = 0
			t.IsCritical = true
		}
	}
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
