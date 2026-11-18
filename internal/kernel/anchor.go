// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"time"
)

// WorkdayFunc reports whether a calendar date is a working day. The
// kernel stays pure by taking the calendar as an injected predicate
// (internal/calendar provides one per country); no calendar data
// lives in this package.
type WorkdayFunc func(t time.Time) bool

// DateLayout is the format AnchorSchedule writes into
// Task.StartDate / Task.FinishDate.
const DateLayout = "2006-01-02"

// maxNonWorkdayRun is a defensive bound: if an injected WorkdayFunc
// rejects this many consecutive days, AnchorSchedule treats the next
// day as working rather than walking the calendar forever.
const maxNonWorkdayRun = 366

// AnchorSchedule maps the abstract CPM day-offsets produced by
// CalculateCPM onto real calendar dates. Offset 0 is the first
// working day on or after projectStart; each subsequent offset is the
// next working day. Results are written to Task.StartDate and
// Task.FinishDate in DateLayout form.
//
// Semantics per task:
//
//   - StartDate  = working day at offset floor(ES), the day the task
//     starts in (see startOffset)
//   - FinishDate = working day at offset ceil(EF)-1 (the last day the
//     task occupies), clamped to be no earlier than StartDate
//   - zero-duration tasks (milestones) start and finish the same day
//
// isWorkday may be nil, in which case every day counts as working.
// AnchorSchedule must be called after CalculateCPM; it reads ES/EF
// and never mutates them.
//
// The date table it builds has one entry per working day of the schedule.
// If any task's ES or EF is not a finite offset within maxAllocationDays
// (which CalculateCPM never produces), no table is built and every task's
// dates are cleared instead.
func AnchorSchedule(tasks map[string]*Task, projectStart time.Time, isWorkday WorkdayFunc) {
	if len(tasks) == 0 {
		return
	}
	for _, t := range tasks {
		if withinAllocationLimit(t.ES) && withinAllocationLimit(t.EF) {
			continue
		}
		for _, clear := range tasks {
			clear.StartDate, clear.FinishDate = "", ""
		}
		return
	}
	if isWorkday == nil {
		isWorkday = func(time.Time) bool { return true }
	}

	// Highest day offset any task needs.
	maxOffset := 0
	for _, t := range tasks {
		if f := finishOffset(t); f > maxOffset {
			maxOffset = f
		}
	}

	// Walk the calendar once and memoise offset -> date.
	dates := make([]time.Time, maxOffset+1)
	d := projectStart
	run := 0
	for !isWorkday(d) && run < maxNonWorkdayRun {
		d = d.AddDate(0, 0, 1)
		run++
	}
	dates[0] = d
	for i := 1; i <= maxOffset; i++ {
		d = d.AddDate(0, 0, 1)
		run = 0
		for !isWorkday(d) && run < maxNonWorkdayRun {
			d = d.AddDate(0, 0, 1)
			run++
		}
		dates[i] = d
	}

	for _, t := range tasks {
		s := startOffset(t)
		f := finishOffset(t)
		t.StartDate = dates[s].Format(DateLayout)
		t.FinishDate = dates[f].Format(DateLayout)
	}
}

// DayOffset is the inverse of AnchorSchedule's date mapping: it
// returns the working-day index (offset 0 = first working day on or
// after projectStart) that `date` falls on. A date before the first
// working day maps to 0; a non-working date maps to the index of the
// next working day. The defensive maxNonWorkdayRun cap applies, and
// the walk aborts at 100000 working days (~270 years) returning false.
//
// Both arguments are calendar dates: each is read as the date it shows in
// its own zone, and its time of day is ignored, as AnchorSchedule does when
// it formats dates. Comparing the instants instead let a start stored with
// a time and offset move every offset by a day (KRN-02), and counted a
// status time after midnight as one more completed day.
func DayOffset(projectStart, date time.Time, isWorkday WorkdayFunc) (float64, bool) {
	if isWorkday == nil {
		isWorkday = func(time.Time) bool { return true }
	}
	date = calendarDate(date)

	d := calendarDate(projectStart)
	run := 0
	for !isWorkday(d) && run < maxNonWorkdayRun {
		d = d.AddDate(0, 0, 1)
		run++
	}

	offset := 0
	for i := 0; i < 100000; i++ {
		if !d.Before(date) {
			return float64(offset), true
		}
		d = d.AddDate(0, 0, 1)
		run = 0
		for !isWorkday(d) && run < maxNonWorkdayRun {
			d = d.AddDate(0, 0, 1)
			run++
		}
		offset++
	}
	return 0, false
}

// calendarDate is t's date in its own zone, as midnight UTC.
func calendarDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ApplyConstraintDates arms each task's date-bearing constraint
// (SNET, SNLT, MSO, FNET, FNLT, MFO; see IsDateConstraint) by
// converting ConstraintDate into working-day offsets against the project
// start and work calendar. Tasks with no constraint, ASAP, ALAP, or a
// date that cannot be converted are left unarmed. Every call first resets
// the fields it sets. Call it BEFORE CalculateCPM; without it, date
// constraints are ignored (un-anchored schedules).
//
// Two readings of the date: ConstraintDay is the first working day on or
// after it (DayOffset), which an earliest limit uses; the last working
// day on or before it is ConstraintLateShift working days earlier, and a
// latest limit uses that. They differ only for a non-working day or a
// date before the project's first working day, which then has a negative
// latest index: an SNLT on a Saturday means "by Friday", and an MSO on a
// Saturday cannot be met. A date whose latest reading cannot be
// converted either is left unarmed.
//
// Offset convention: a START constraint (SNET, SNLT, MSO) compares
// against ES, which IS a day index. A FINISH constraint (FNET, FNLT,
// MFO) compares against EF, which is exclusive — a task finishing ON
// day index d has EF = d+1 (AnchorSchedule's finishOffset is
// ceil(EF)-1) — so finish constraints store day+1; the shift is the same
// for both readings. This holds for zero-duration milestones too: a
// milestone sits on the boundary after the work before it (a task ending
// Wednesday puts its successor milestone at the start of Thursday, index
// 3), so a finish date D means "by the end of D" and the milestone meets
// it at index D+1.
func ApplyConstraintDates(tasks map[string]*Task, projectStart time.Time, isWorkday WorkdayFunc) {
	for _, t := range tasks {
		t.ConstraintArmed = false
		t.ConstraintDay = 0
		t.ConstraintLateShift = 0
		if !IsDateConstraint(t.Constraint) {
			continue
		}
		date, err := time.Parse(DateLayout, t.ConstraintDate)
		if err != nil {
			continue
		}
		day, ok := DayOffset(projectStart, date, isWorkday)
		if !ok {
			continue
		}
		late, ok := latestWorkdayOffset(projectStart, date, isWorkday)
		if !ok {
			continue
		}
		if isFinishConstraint(t.Constraint) {
			day++
			late++
		}
		t.ConstraintDay = day
		t.ConstraintLateShift = day - late
		t.ConstraintArmed = true
	}
}

// latestWorkdayOffset is the working-day index of the last working day on
// or before date (each read as a calendar date, as DayOffset does).
// Index 0 is the first working day on or after projectStart; working days
// before it count back from -1. It walks at most 100000 working days in
// either direction and then returns false.
func latestWorkdayOffset(projectStart, date time.Time, isWorkday WorkdayFunc) (float64, bool) {
	if isWorkday == nil {
		isWorkday = func(time.Time) bool { return true }
	}
	date = calendarDate(date)
	first := calendarDate(projectStart)
	run := 0
	for !isWorkday(first) && run < maxNonWorkdayRun {
		first = first.AddDate(0, 0, 1)
		run++
	}

	if !date.Before(first) {
		// The last working day on or before date is no earlier than
		// first, which is itself a working day (or the defensive cap's
		// stand-in), so DayOffset gives its index.
		d := date
		run = 0
		for d.After(first) && !isWorkday(d) && run < maxNonWorkdayRun {
			d = d.AddDate(0, 0, -1)
			run++
		}
		return DayOffset(projectStart, d, isWorkday)
	}

	d := first
	for index := -1; index >= -100000; index-- {
		d = d.AddDate(0, 0, -1)
		run = 0
		for !isWorkday(d) && run < maxNonWorkdayRun {
			d = d.AddDate(0, 0, -1)
			run++
		}
		if !d.After(date) {
			return float64(index), true
		}
	}
	return 0, false
}

// startOffset is the working-day index a task begins on: the day its
// start falls in, so a task starting part-way through day d (ES = d+0.5)
// is shown on d. The epsilon absorbs floating-point noise from summed
// fractional durations (2.9999999999 is day 3). Resource-day attribution
// (taskSpan) keeps its own rounding, which the OR-01 contract fixes.
func startOffset(t *Task) int {
	s := int(math.Floor(t.ES + 1e-9))
	if s < 0 {
		return 0
	}
	return s
}

// finishOffset is the working-day index of the last day a task
// occupies: the day its exclusive finish EF falls in, less one when EF
// lies on a day boundary. A zero-duration milestone finishes the day it
// starts.
func finishOffset(t *Task) int {
	s := startOffset(t)
	f := int(math.Ceil(t.EF-1e-9)) - 1
	if f < s {
		return s
	}
	return f
}
