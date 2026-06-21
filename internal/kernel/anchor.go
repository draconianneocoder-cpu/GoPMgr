// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
//   - StartDate  = working day at offset round(ES)
//   - FinishDate = working day at offset ceil(EF)-1 (the last day the
//     task occupies), clamped to be no earlier than StartDate
//   - zero-duration tasks (milestones) start and finish the same day
//
// isWorkday may be nil, in which case every day counts as working.
// AnchorSchedule must be called after CalculateCPM; it reads ES/EF
// and never mutates them.
func AnchorSchedule(tasks map[string]*Task, projectStart time.Time, isWorkday WorkdayFunc) {
	if len(tasks) == 0 {
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
func DayOffset(projectStart, date time.Time, isWorkday WorkdayFunc) (float64, bool) {
	if isWorkday == nil {
		isWorkday = func(time.Time) bool { return true }
	}

	d := projectStart
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

// ApplyConstraintDates arms each task's date-bearing constraint
// (SNET/FNLT/MFO) by converting ConstraintDate into a working-day
// offset (ConstraintDay) against the project start and work calendar.
// Tasks with no constraint, an ALAP constraint, or an unparseable
// date are left unarmed. Call it BEFORE CalculateCPM; without it,
// date constraints are ignored (un-anchored schedules).
//
// Offset convention: a START constraint (SNET) compares against ES,
// which IS a day index. A FINISH constraint (FNLT/MFO) compares
// against EF, which is exclusive — a task finishing ON day index d
// has EF = d+1 (AnchorSchedule's finishOffset is ceil(EF)-1) — so
// finish constraints store day+1.
func ApplyConstraintDates(tasks map[string]*Task, projectStart time.Time, isWorkday WorkdayFunc) {
	for _, t := range tasks {
		t.ConstraintArmed = false
		switch t.Constraint {
		case StartNoEarlierThan, FinishNoLaterThan, MustFinishOn:
		default:
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
		if t.Constraint == FinishNoLaterThan || t.Constraint == MustFinishOn {
			day++
		}
		t.ConstraintDay = day
		t.ConstraintArmed = true
	}
}

// startOffset is the working-day index a task begins on.
func startOffset(t *Task) int {
	s := int(math.Round(t.ES))
	if s < 0 {
		return 0
	}
	return s
}

// finishOffset is the working-day index of the last day a task
// occupies. A zero-duration milestone finishes the day it starts.
func finishOffset(t *Task) int {
	s := startOffset(t)
	f := int(math.Ceil(t.EF)) - 1
	if f < s {
		return s
	}
	return f
}
