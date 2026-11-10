// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"fmt"
	"math"
	"sort"
)

// MaxScheduleDays is the longest schedule the kernel will compute: 50
// years of working days. Date anchoring, resource profiles and leveling
// allocate memory in proportion to a schedule's length, so a single typed
// or imported duration of a billion days would otherwise exhaust memory.
// No task duration, lag, estimate or computed date may lie beyond it.
const MaxScheduleDays = 50 * 365

// maxAllocationDays bounds the day-indexed slices AnchorSchedule and the
// resource profiles build. It is looser than MaxScheduleDays because
// resource leveling may delay a task by up to its horizon (itself capped at
// MaxScheduleDays) past a schedule that was within the limit.
const maxAllocationDays = 4 * MaxScheduleDays

// withinAllocationLimit reports whether a CPM offset is safe to turn into
// a slice index: finite and no further than maxAllocationDays from the
// project start. Converting an out-of-range float to an int is undefined
// in Go, and sizing a slice from one is how an over-long schedule would
// exhaust memory.
func withinAllocationLimit(v float64) bool {
	return math.Abs(v) <= maxAllocationDays
}

// ErrScheduleTooLong is returned by ScheduleCPM when a task value or the
// computed schedule lies beyond MaxScheduleDays. A value that is not a
// finite number is reported the same way.
var ErrScheduleTooLong = fmt.Errorf("kernel: schedule is longer than %d working days", MaxScheduleDays)

// withinScheduleLimit reports whether v is a finite number of days no
// further than MaxScheduleDays from the project start in either direction.
// NaN and the infinities fail the comparison.
func withinScheduleLimit(v float64) bool {
	return math.Abs(v) <= MaxScheduleDays
}

// taskName is how a refusal names a task to the person who has to fix it:
// its title, or its ID when it has none.
func taskName(t *Task) string {
	if t.Title != "" {
		return t.Title
	}
	return t.ID
}

// checkTaskLimits returns ErrScheduleTooLong, naming the first offending
// task in ID order, if any duration, lag or duration estimate is beyond
// MaxScheduleDays. It runs before the CPM passes so that those never add
// up out-of-range numbers.
func checkTaskLimits(tasks map[string]*Task) error {
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := tasks[id]
		if t == nil {
			continue
		}
		if !withinScheduleLimit(t.Duration) {
			return fmt.Errorf("%w: task %q has a duration of %g days", ErrScheduleTooLong, taskName(t), t.Duration)
		}
		e := t.DurationEstimate
		if !withinScheduleLimit(e.Optimistic) || !withinScheduleLimit(e.MostLikely) || !withinScheduleLimit(e.Pessimistic) {
			return fmt.Errorf("%w: task %q has a duration estimate beyond it", ErrScheduleTooLong, taskName(t))
		}
		for _, l := range t.Links {
			if !withinScheduleLimit(l.Lag) {
				return fmt.Errorf("%w: task %q has a lag of %g days", ErrScheduleTooLong, taskName(t), l.Lag)
			}
		}
	}
	return nil
}

// checkSpan returns ErrScheduleTooLong if the passes placed any task's
// early dates beyond MaxScheduleDays. Each duration and lag can be within
// the limit while a chain of them is not.
func (plan *cpmPlan) checkSpan() error {
	for _, t := range plan.order {
		if !withinScheduleLimit(t.ES) || !withinScheduleLimit(t.EF) {
			return fmt.Errorf("%w: task %q would finish on working day %.0f", ErrScheduleTooLong, taskName(t), t.EF)
		}
	}
	return nil
}

// clearResults zeroes every CPM output, so a caller that ignores a refused
// schedule cannot go on to use dates that are out of range.
func (plan *cpmPlan) clearResults() {
	for _, t := range plan.order {
		t.ES, t.EF, t.LS, t.LF, t.Float = 0, 0, 0, 0, 0
		t.IsCritical = false
		t.ConstraintViolated = false
	}
}

// ScheduleCPM is CalculateCPM with the reason for a refusal: it returns
// ErrSchedulingCycle for a cyclic network and ErrScheduleTooLong (wrapped
// with the offending task) for a schedule beyond MaxScheduleDays. Either
// way no task carries CPM results afterwards. Test with errors.Is.
func ScheduleCPM(tasks map[string]*Task) error {
	if err := checkTaskLimits(tasks); err != nil {
		return err
	}
	plan, ok := newCPMPlan(tasks)
	if !ok {
		return ErrSchedulingCycle
	}
	plan.run()
	if err := plan.checkSpan(); err != nil {
		plan.clearResults()
		return err
	}
	return nil
}
