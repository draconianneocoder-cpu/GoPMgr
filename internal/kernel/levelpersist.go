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
// stored starts are the fewest that hold the schedule. ALAP tasks keep
// none: the ALAP pass places them, and the forward pass gives their
// successors their early dates. Returns false on a cycle.
func LevelStartsToKeep(tasks map[string]*Task) (map[string]float64, bool) {
	plan, ok := newCPMPlan(tasks)
	if !ok {
		return nil, false
	}
	index := make(map[*Task]int, len(plan.order))
	for i, t := range plan.order {
		index[t] = i
	}
	savedES := make([]float64, len(plan.order))
	keep := map[string]float64{}
	for i, t := range plan.order {
		es := 0.0
		for _, l := range plan.preds[i] {
			p := index[l.task]
			es = math.Max(es, linkCandidate(l.typ, l.lag, savedES[p], savedES[p]+l.task.Duration, t.Duration))
		}
		es = ForwardConstrainedStart(t, es)
		if t.Constraint != AsLateAsPossible && t.ES > es+1e-9 {
			keep[t.ID] = t.ES
			es = t.ES
		}
		savedES[i] = es
	}
	return keep, true
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
	if len(t.PlannedWorkSegments) == 0 || t.Duration <= 0 || !withinAllocationLimit(t.ES) {
		return nil, false
	}
	start := math.Round(t.ES)
	if math.Abs(t.ES-start) > 1e-9 {
		return nil, false
	}
	need := int(math.Ceil(t.Duration))
	var days []int
	for _, s := range t.PlannedWorkSegments {
		lo, hi := math.Round(s.Start), math.Round(s.End)
		if math.Abs(s.Start-lo) > 1e-9 || math.Abs(s.End-hi) > 1e-9 || lo < 0 || hi <= lo ||
			len(days)+int(hi-lo) > need || (len(days) > 0 && int(start+lo) <= days[len(days)-1]) {
			return nil, false
		}
		for d := lo; d < hi; d++ {
			days = append(days, int(start+d))
		}
	}
	if len(days) != need {
		return nil, false
	}
	return days, true
}
