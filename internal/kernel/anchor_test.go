// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"testing"
	"time"
)

// weekdaysOnly is a stub work calendar: Monday-Friday, no holidays.
func weekdaysOnly(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func mustCPM(t *testing.T, tasks map[string]*Task) {
	t.Helper()
	if !CalculateCPM(tasks) {
		t.Fatal("CalculateCPM reported a cycle in an acyclic graph")
	}
}

func TestAnchorScheduleSkipsWeekends(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "A", Duration: 2},
		"B": {ID: "B", Title: "B", Duration: 3, Precedents: []string{"A"}},
	}
	mustCPM(t, tasks)

	// Friday 2026-06-05. Offset 0 = Fri, 1 = Mon 06-08, 2 = Tue, ...
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	AnchorSchedule(tasks, start, weekdaysOnly)

	if got, want := tasks["A"].StartDate, "2026-06-05"; got != want {
		t.Errorf("A.StartDate = %s, want %s", got, want)
	}
	// A occupies Fri + Mon.
	if got, want := tasks["A"].FinishDate, "2026-06-08"; got != want {
		t.Errorf("A.FinishDate = %s, want %s", got, want)
	}
	// B starts the next working day (Tue) and runs 3 days: Tue-Thu.
	if got, want := tasks["B"].StartDate, "2026-06-09"; got != want {
		t.Errorf("B.StartDate = %s, want %s", got, want)
	}
	if got, want := tasks["B"].FinishDate, "2026-06-11"; got != want {
		t.Errorf("B.FinishDate = %s, want %s", got, want)
	}
}

func TestAnchorScheduleWeekendStartRollsForward(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "A", Duration: 1},
	}
	mustCPM(t, tasks)

	// Saturday 2026-06-06 rolls to Monday 2026-06-08.
	start := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	AnchorSchedule(tasks, start, weekdaysOnly)

	if got, want := tasks["A"].StartDate, "2026-06-08"; got != want {
		t.Errorf("A.StartDate = %s, want %s", got, want)
	}
	if got, want := tasks["A"].FinishDate, "2026-06-08"; got != want {
		t.Errorf("A.FinishDate = %s, want %s", got, want)
	}
}

func TestAnchorScheduleMilestone(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "A", Duration: 2},
		"M": {ID: "M", Title: "Milestone", Duration: 0, Precedents: []string{"A"}},
	}
	mustCPM(t, tasks)

	// Monday start, every day working.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	AnchorSchedule(tasks, start, nil)

	// Milestone ES = EF = 2 → starts and finishes on offset-2 day.
	if tasks["M"].StartDate != tasks["M"].FinishDate {
		t.Errorf("milestone start %s != finish %s",
			tasks["M"].StartDate, tasks["M"].FinishDate)
	}
	if got, want := tasks["M"].StartDate, "2026-06-03"; got != want {
		t.Errorf("M.StartDate = %s, want %s", got, want)
	}
}

func TestAnchorScheduleNilCalendarCountsEveryDay(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "A", Duration: 7},
	}
	mustCPM(t, tasks)

	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC) // Friday
	AnchorSchedule(tasks, start, nil)

	// 7 consecutive days including weekend: Jun 5 .. Jun 11.
	if got, want := tasks["A"].FinishDate, "2026-06-11"; got != want {
		t.Errorf("A.FinishDate = %s, want %s", got, want)
	}
}

func TestAnchorSchedulePathologicalCalendarTerminates(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "A", Duration: 1},
	}
	mustCPM(t, tasks)

	never := func(time.Time) bool { return false }
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	AnchorSchedule(tasks, start, never) // must not hang

	if tasks["A"].StartDate == "" {
		t.Error("StartDate empty: defensive cap did not assign a date")
	}
}

func TestAnchorScheduleEmptyMapIsNoop(t *testing.T) {
	AnchorSchedule(nil, time.Now(), weekdaysOnly) // must not panic
}

// A task that starts part-way through a day is shown on that day. A
// (0.5 day) then B (0.4 day): B runs 0.5 to 0.9, inside Monday 06-01, so
// it is shown Monday to Monday and meets FNLT Monday (finish index 1,
// EF 0.9). Rounding the start used to show it on Tuesday.
func TestAnchorScheduleShowsAPartDayStartOnItsDay(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 0.5},
		"B": {ID: "B", Duration: 0.4, Precedents: []string{"A"}, Constraint: FinishNoLaterThan, ConstraintDate: "2026-06-01"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	AnchorSchedule(tasks, conStart, weekdaysOnly)
	if b := tasks["B"]; b.StartDate != "2026-06-01" || b.FinishDate != "2026-06-01" || b.ConstraintViolated {
		t.Fatalf("B = %s..%s violated %v, want 2026-06-01..2026-06-01 not violated", b.StartDate, b.FinishDate, b.ConstraintViolated)
	}

	// Floating-point noise from summed fractions does not move a day:
	// ES 2.9999999999 is day 3 (Thu 06-04); EF 3.0000000001 ends on day 2
	// (Wed 06-03).
	noisy := map[string]*Task{
		"X": {ID: "X", ES: 2.9999999999, EF: 3.9999999999, Duration: 1},
		"Y": {ID: "Y", ES: 2, EF: 3.0000000001, Duration: 1},
	}
	AnchorSchedule(noisy, conStart, weekdaysOnly)
	if x := noisy["X"]; x.StartDate != "2026-06-04" || x.FinishDate != "2026-06-04" {
		t.Errorf("X = %s..%s, want 2026-06-04..2026-06-04", x.StartDate, x.FinishDate)
	}
	if y := noisy["Y"]; y.StartDate != "2026-06-03" || y.FinishDate != "2026-06-03" {
		t.Errorf("Y = %s..%s, want 2026-06-03..2026-06-03", y.StartDate, y.FinishDate)
	}
}
