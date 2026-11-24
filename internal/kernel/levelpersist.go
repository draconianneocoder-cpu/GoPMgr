// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "math"

// LevelStartsToKeep returns the levelled starts (working-day offsets, by
// task ID) a saved chart must hold so that scheduling it reproduces the
// levelled schedule in tasks, which must hold LevelResourcesWithOptions'
// result (Gate A R4.1, 2026-11-23). In topological order, each task's
// start in the saved form is what its links to its predecessors as saved
// (a split predecessor at its contiguous finish, as the forward pass sees
// it) and its forward constraints give, by the forward pass's own rules;
// a task levelled later than that keeps its levelled start. A successor
// that its links already move as far as levelling did keeps none, so the
// stored starts are the fewest that hold the schedule. An ALAP task
// levelling placed (work on a named resource) always keeps its levelled
// start, since the ALAP pass would otherwise move it to the saved
// schedule's late dates (owner decision 2026-11-23, "keep levelled
// position"); the forward pass then gives its successors its levelled
// dates. Returns false on a cycle.
func LevelStartsToKeep(tasks map[string]*Task) (map[string]float64, bool) {
	plan, ok := newCPMPlan(tasks)
	if !ok {
		return nil, false
	}
	savedES := make([]float64, len(plan.order))
	keep := map[string]float64{}
	for i, t := range plan.order {
		es := 0.0
		for _, l := range plan.preds[i] {
			es = math.Max(es, linkCandidate(l.typ, l.lag, savedES[l.idx], savedES[l.idx]+l.task.Duration, t.Duration))
		}
		es = ForwardConstrainedStart(t, es)
		alap := t.Constraint == AsLateAsPossible
		if (alap && levelledByResource(t)) || (!alap && t.ES > es+1e-9) {
			keep[t.ID] = t.ES
			es = t.ES
		}
		savedES[i] = es
	}
	return keep, true
}

// levelledByResource reports whether levelling places t: it has work and
// an assignment to a named resource.
func levelledByResource(t *Task) bool {
	if t.Duration <= 0 {
		return false
	}
	for _, a := range t.Assignments {
		if a.Resource != "" {
			return true
		}
	}
	return false
}

// RebuildSplitDays restores, after a schedule pass, the working days of
// each task levelling split, from the work segments saved relative to its
// start (PlannedWorkSegments; Gate A R4.2 and conditions C1, C2, M6). Call
// it after ScheduleCPM and before DetectOverallocationsWithPlan and
// AnchorSchedule, in every path that schedules a saved chart, so detection,
// dates and reports see the split as levelling placed it.
//
// The segments are used only while they still describe the task: it
// starts on a whole day and the segments are whole days adding up to
// ceil(Duration), as levelling writes them. Then WorkDays = ES + each day
// they cover and EF = the last of those days + 1, and a finish limit (FNLT,
// MFO) is checked again against that EF. Otherwise (a duration edited since
// levelling, a start moved part-way into a day) they are ignored and the
// task is scheduled contiguously; the stale-levelling warning covers it.
// Successor links keep the contiguous finish the forward pass used (the
// saved levelled starts hold successors where levelling put them), and
// LS, LF, Float and IsCritical describe the contiguous plan.
func RebuildSplitDays(tasks map[string]*Task) {
	for _, t := range tasks {
		days, ok := splitDays(t)
		if !ok {
			continue
		}
		t.WorkDays = days
		t.EF = float64(days[len(days)-1] + 1)
		if t.ConstraintArmed && (t.Constraint == FinishNoLaterThan || t.Constraint == MustFinishOn) &&
			t.EF > lateConstraintDay(t)+1e-9 {
			t.ConstraintViolated = true
		}
	}
}

// splitDays is t's working days from its saved segments, or false when
// the segments do not apply (see RebuildSplitDays).
func splitDays(t *Task) ([]int, bool) {
	var days []int
	if !walkSplitDays(t, func(d int) { days = append(days, d) }) {
		return nil, false
	}
	return days, true
}

// splitFits reports whether t's saved segments apply, as splitDays does,
// without collecting the days.
func splitFits(t *Task) bool {
	return walkSplitDays(t, func(int) {})
}

// walkSplitDays calls day with each working day t's saved segments give,
// in order, and reports whether the segments apply: t has work and starts
// on a whole day, and the segments are ascending, disjoint whole days
// adding up to ceil(Duration). day may have been called before a false
// result.
func walkSplitDays(t *Task, day func(int)) bool {
	if len(t.PlannedWorkSegments) == 0 || t.Duration <= 0 || !withinAllocationLimit(t.ES) {
		return false
	}
	start := math.Round(t.ES)
	if math.Abs(t.ES-start) > 1e-9 {
		return false
	}
	need := int(math.Ceil(t.Duration))
	count, last := 0, 0
	for _, s := range t.PlannedWorkSegments {
		lo, hi := math.Round(s.Start), math.Round(s.End)
		if math.Abs(s.Start-lo) > 1e-9 || math.Abs(s.End-hi) > 1e-9 || lo < 0 || hi <= lo ||
			count+int(hi-lo) > need || (count > 0 && int(start+lo) <= last) {
			return false
		}
		for d := lo; d < hi; d++ {
			last = int(start + d)
			count++
			day(last)
		}
	}
	return count == need
}
