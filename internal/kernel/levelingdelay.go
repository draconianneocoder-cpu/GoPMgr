// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"sort"
	"time"
)

// Levelling delays in Microsoft Project terms (owner decisions 2026-11-23:
// D-MSPDI, and tenths of elapsed minutes in the file, as MPXJ reads
// Microsoft Project's; the MSPDI encoding is in internal/export). A task's
// levelling delay is how long levelling held its start back from where its links and
// constraints put it, measured on the clock: the working-day offsets are
// placed on Microsoft Project's standard day (StartClock) and the
// difference is elapsed time, as LevelingDelayFormat's elapsed units read
// it. GoPMgr stores the result itself (LevelStart and LevelStartFraction);
// these helpers convert for MSPDI export and import.

// maxElapsedDelayMinutes bounds an elapsed delay read from a file: twice
// the calendar span of the longest schedule the kernel allocates for, far
// below time.Duration's limit.
const maxElapsedDelayMinutes = float64(maxAllocationDays) * 2 * 24 * 60

// LevelingDelay is one task's delay read from a file, in elapsed minutes.
type LevelingDelay struct {
	Minutes float64
}

// workdayTable maps working-day offsets to dates (offset 0 = the first
// working day on or after the project start), grown on demand. The
// project start is read as the calendar date it shows (calendarDate), so
// the dates and the days isWorkday is asked about do not depend on its
// time of day or zone. AnchorSchedule, WorkdayDates and the levelling-delay
// helpers share it.
type workdayTable struct {
	isWorkday WorkdayFunc
	dates     []time.Time
}

func newWorkdayTable(projectStart time.Time, isWorkday WorkdayFunc) *workdayTable {
	if isWorkday == nil {
		isWorkday = func(time.Time) bool { return true }
	}
	w := &workdayTable{isWorkday: isWorkday}
	w.dates = []time.Time{w.nextWorkday(calendarDate(projectStart))}
	return w
}

// nextWorkday is d, or the first working day after it (bounded as
// AnchorSchedule bounds it).
func (w *workdayTable) nextWorkday(d time.Time) time.Time {
	for run := 0; !w.isWorkday(d) && run < maxNonWorkdayRun; run++ {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// date is the date of working day k (k >= 0, within maxAllocationDays).
func (w *workdayTable) date(k int) time.Time {
	for len(w.dates) <= k {
		w.dates = append(w.dates, w.nextWorkday(w.dates[len(w.dates)-1].AddDate(0, 0, 1)))
	}
	return w.dates[k]
}

// at is the date and time a working-day offset starts at.
func (w *workdayTable) at(offset float64) time.Time {
	day, fraction := SplitDayOffset(offset)
	h, m := StartClock(fraction)
	d := w.date(max(day, 0))
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, time.UTC)
}

// offset is the working-day offset a start at t falls on: a time outside
// working hours moves to the next working start (12:00 to 13:00, after
// 17:00 to 08:00 the next working day), as does a non-working date. ok is
// false beyond maxAllocationDays.
func (w *workdayTable) offset(t time.Time) (float64, bool) {
	day := calendarDate(t)
	minute := t.Hour()*60 + t.Minute()
	if !w.isWorkday(day) {
		day, minute = w.nextWorkday(day), 0
	} else if minute >= afternoonMinute+workdayMinutes-morningMinutes {
		day, minute = w.nextWorkday(day.AddDate(0, 0, 1)), 0
	}
	if day.Before(w.dates[0]) {
		return 0, true
	}
	for !w.dates[len(w.dates)-1].After(day) {
		if len(w.dates) > maxAllocationDays {
			return 0, false
		}
		w.date(len(w.dates))
	}
	k := sort.Search(len(w.dates), func(i int) bool { return !w.dates[i].Before(day) })
	var worked int
	switch {
	case minute <= dayStartMinute:
		worked = 0
	case minute < dayStartMinute+morningMinutes:
		worked = minute - dayStartMinute
	case minute < afternoonMinute:
		worked = morningMinutes
	default:
		worked = morningMinutes + minute - afternoonMinute
	}
	return float64(k) + float64(worked)/workdayMinutes, true
}

// LevelingDelayMinutes is, by task ID, the levelling delay of each task
// that carries a stored levelled start (LevelStart) and starts later than
// its links and constraints allow (ES beyond LevelEarliest), in elapsed
// minutes on the clock (rounded). ALAP tasks have none. tasks must have
// been scheduled (ScheduleCPM) against projectStart and isWorkday.
func LevelingDelayMinutes(tasks map[string]*Task, projectStart time.Time, isWorkday WorkdayFunc) map[string]int {
	w := newWorkdayTable(projectStart, isWorkday)
	out := map[string]int{}
	for id, t := range tasks {
		if t.LevelStart == "" || t.Constraint == AsLateAsPossible || !(t.ES > t.LevelEarliest+1e-9) ||
			!withinAllocationLimit(t.ES) || t.LevelEarliest < 0 {
			continue
		}
		if minutes := int(math.Round(w.at(t.ES).Sub(w.at(t.LevelEarliest)).Minutes())); minutes > 0 {
			out[id] = minutes
		}
	}
	return out
}

// ApplyLevelingDelays schedules tasks forward with each task's delay
// applied after its links and forward constraints (the forward pass's own
// rules), and returns the levelled start (working-day offset) of each task
// a delay moves, by task ID. A delay is added on the clock from the start
// the links and constraints give (StartClock), and a result outside
// working time moves to the next working start. Delays on ALAP tasks, and starts that would put
// the task beyond MaxScheduleDays, are not applied and are returned in
// skipped (sorted). tasks must have their constraints armed
// (ApplyConstraintDates against the same start and calendar); their ES and
// EF are not changed. ok is false on a cycle.
func ApplyLevelingDelays(tasks map[string]*Task, delays map[string]LevelingDelay, projectStart time.Time, isWorkday WorkdayFunc) (starts map[string]float64, skipped []string, ok bool) {
	plan, ok := newCPMPlan(tasks)
	if !ok {
		return nil, nil, false
	}
	w := newWorkdayTable(projectStart, isWorkday)
	es := make([]float64, len(plan.order))
	starts = map[string]float64{}
	for i, t := range plan.order {
		start := 0.0
		for _, l := range plan.preds[i] {
			start = math.Max(start, linkCandidate(l.typ, l.lag, es[l.idx], es[l.idx]+l.task.Duration, t.Duration))
		}
		start = ForwardConstrainedStart(t, start)
		if d, has := delays[t.ID]; has && d.Minutes > 0 {
			// Beyond any schedule (and time.Duration) a delay is not applied.
			levelled, fits := 0.0, false
			if d.Minutes <= maxElapsedDelayMinutes {
				levelled, fits = w.offset(w.at(start).Add(time.Duration(math.Round(d.Minutes)) * time.Minute))
			}
			switch {
			case t.Constraint == AsLateAsPossible || !fits || math.IsNaN(levelled) || levelled+t.Duration > MaxScheduleDays:
				skipped = append(skipped, t.ID)
			case levelled > start+1e-9:
				starts[t.ID] = levelled
				start = levelled
			}
		}
		es[i] = start
	}
	sort.Strings(skipped)
	return starts, skipped, true
}
