// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"testing"
	"time"
)

// Monday 2026-06-01 anchors all tests; weekdaysOnly comes from
// anchor_test.go.
var conStart = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func TestDayOffset(t *testing.T) {
	cases := []struct {
		date string
		want float64
	}{
		{"2026-06-01", 0}, // Monday = day 0
		{"2026-06-03", 2}, // Wednesday
		{"2026-06-06", 5}, // Saturday -> next workday Mon 06-08
		{"2026-06-08", 5}, // Monday week 2
		{"2026-05-20", 0}, // before project start clamps to 0
	}
	for _, c := range cases {
		date, _ := time.Parse(DateLayout, c.date)
		got, ok := DayOffset(conStart, date, weekdaysOnly)
		if !ok || got != c.want {
			t.Errorf("DayOffset(%s) = (%v, %v), want (%v, true)",
				c.date, got, ok, c.want)
		}
	}
}

// KRN-02: offsets count calendar days, so the time of day and the offset
// of the project start must not move them. Each start below is shown as
// Monday 2026-06-01 by AnchorSchedule, so each must give the same offsets
// as a plain midnight-UTC start (TestDayOffset). Before the fix, the
// -05:00 evening start gave Wednesday offset 1 and the +02:00 start gave 3.
func TestDayOffsetIgnoresTheStartsTimeOfDay(t *testing.T) {
	starts := []string{
		"2026-06-01T23:00:00-05:00", // 04:00 UTC on Tuesday
		"2026-06-01T00:30:00+02:00", // 22:30 UTC on Sunday
		"2026-06-01T12:00:00Z",
		"2026-06-01T23:59:59+14:00",
	}
	cases := []struct {
		date string
		want float64
	}{
		{"2026-06-01", 0},
		{"2026-06-03", 2},
		{"2026-06-06", 5},
		{"2026-06-08", 5},
	}
	for _, s := range starts {
		start, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("parse %s: %v", s, err)
		}
		for _, c := range cases {
			date, _ := time.Parse(DateLayout, c.date)
			if got, ok := DayOffset(start, date, weekdaysOnly); !ok || got != c.want {
				t.Errorf("start %s: DayOffset(%s) = (%v, %v), want (%v, true)", s, c.date, got, ok, c.want)
			}
		}
	}
}

// The other argument is read the same way: a status date taken as a local
// evening instant is still that local calendar day.
func TestDayOffsetReadsTheDateInItsOwnZone(t *testing.T) {
	date, err := time.Parse(time.RFC3339, "2026-06-03T22:00:00-07:00") // Thursday 05:00 UTC
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := DayOffset(conStart, date, weekdaysOnly); !ok || got != 2 {
		t.Fatalf("DayOffset(Wednesday evening, -07:00) = (%v, %v), want (2, true)", got, ok)
	}
}

func TestSNETDelaysStart(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"},
			Constraint: StartNoEarlierThan, ConstraintDate: "2026-06-04"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)

	// Links say ES=1 (Tue); SNET Thursday = day 3 wins.
	approx(t, "B.ES", tasks["B"].ES, 3)
	AnchorSchedule(tasks, conStart, weekdaysOnly)
	if got := tasks["B"].StartDate; got != "2026-06-04" {
		t.Errorf("B.StartDate = %s, want 2026-06-04", got)
	}
	if tasks["B"].ConstraintViolated {
		t.Error("SNET that merely delays must not be a violation")
	}
}

func TestMFOPullsTaskToitsDate(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Constraint: MustFinishOn, ConstraintDate: "2026-06-03"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)

	// Finish ON Wednesday (day 2): EF = 3, ES = 1, LF pinned = 3.
	approx(t, "A.ES", tasks["A"].ES, 1)
	approx(t, "A.EF", tasks["A"].EF, 3)
	approx(t, "A.LF", tasks["A"].LF, 3)
	approx(t, "A.Float", tasks["A"].Float, 0)
	if tasks["A"].ConstraintViolated {
		t.Error("satisfiable MFO must not be flagged")
	}
	AnchorSchedule(tasks, conStart, weekdaysOnly)
	if got := tasks["A"].FinishDate; got != "2026-06-03" {
		t.Errorf("A.FinishDate = %s, want 2026-06-03", got)
	}
}

func TestMFOViolatedWhenLinksPushPast(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"},
			Constraint: MustFinishOn, ConstraintDate: "2026-06-03"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)

	// Links force B.EF = 6 > pinned 3: links win, violation flagged.
	approx(t, "B.ES", tasks["B"].ES, 4)
	if !tasks["B"].ConstraintViolated {
		t.Error("unsatisfiable MFO must set ConstraintViolated")
	}
}

func TestFNLTViolationAndNegativeFloat(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 3, Precedents: []string{"A"},
			Constraint: FinishNoLaterThan, ConstraintDate: "2026-06-04"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)

	// Links: B.EF = 6; FNLT Thursday => EF cap 4. Violated, and the
	// squeezed late dates surface as negative float upstream.
	if !tasks["B"].ConstraintViolated {
		t.Error("late finish must set ConstraintViolated")
	}
	if tasks["B"].Float >= 0 {
		t.Errorf("B.Float = %v, want negative (super-critical)", tasks["B"].Float)
	}
	if !tasks["A"].IsCritical || !tasks["B"].IsCritical {
		t.Error("squeezed chain must be fully critical")
	}
}

func TestFNLTSatisfiedIsQuiet(t *testing.T) {
	// B (6d) drives the project finish to 6; A (2d) has FNLT Friday
	// (day 4 -> EF cap 5), which tightens A's float from 4 to 3
	// without any violation.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Constraint: FinishNoLaterThan, ConstraintDate: "2026-06-05"},
		"B": {ID: "B", Duration: 6},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)

	if tasks["A"].ConstraintViolated {
		t.Error("satisfied FNLT must not be flagged")
	}
	approx(t, "A.LF", tasks["A"].LF, 5)
	approx(t, "A.Float", tasks["A"].Float, 3)
}

func TestALAPConsumesFloat(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 5},
		"B": {ID: "B", Duration: 1, Constraint: AsLateAsPossible},
	}
	mustCPM(t, tasks) // no dates needed for ALAP

	// Project finish 5; B floats 4 -> ALAP moves it to ES=4, EF=5.
	approx(t, "B.ES", tasks["B"].ES, 4)
	approx(t, "B.EF", tasks["B"].EF, 5)
	approx(t, "B.Float", tasks["B"].Float, 0)
}

func TestUnarmedDateConstraintIsIgnored(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Constraint: StartNoEarlierThan, ConstraintDate: "2026-06-04"},
	}
	// No ApplyConstraintDates call (un-anchored schedule).
	mustCPM(t, tasks)
	approx(t, "A.ES", tasks["A"].ES, 0)
}

func TestApplyConstraintDatesBadDateStaysUnarmed(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Constraint: MustFinishOn, ConstraintDate: "not-a-date"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	if tasks["A"].ConstraintArmed {
		t.Error("unparseable date must leave the constraint unarmed")
	}
	mustCPM(t, tasks)
	approx(t, "A.ES", tasks["A"].ES, 0)
}

// KRN-01 b: a date constraint that is not applied says why, so the editor
// can show it; one that is applied, or carries no date, says nothing.
func TestApplyConstraintDatesSaysWhyAConstraintIsNotApplied(t *testing.T) {
	tasks := map[string]*Task{
		"bad":    {ID: "bad", Duration: 1, Constraint: MustFinishOn, ConstraintDate: "not-a-date"},
		"empty":  {ID: "empty", Duration: 1, Constraint: StartNoEarlierThan},
		"far":    {ID: "far", Duration: 1, Constraint: StartNoEarlierThan, ConstraintDate: "9999-12-31"},
		"farOld": {ID: "farOld", Duration: 1, Constraint: StartNoLaterThan, ConstraintDate: "1500-01-01"},
		"ok":     {ID: "ok", Duration: 1, Constraint: StartNoEarlierThan, ConstraintDate: "2026-06-04"},
		"alap":   {ID: "alap", Duration: 1, Constraint: AsLateAsPossible},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	want := map[string]ConstraintIssue{
		"bad": ConstraintInvalidDate, "empty": ConstraintInvalidDate,
		"far": ConstraintOutOfRange, "farOld": ConstraintOutOfRange,
		"ok": "", "alap": "",
	}
	for id, w := range want {
		if got := tasks[id].ConstraintUnapplied; got != w {
			t.Errorf("%s: unapplied %q, want %q", id, got, w)
		}
	}

	// Corrected and applied again, the reason is cleared.
	tasks["bad"].ConstraintDate = "2026-06-04"
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	if b := tasks["bad"]; !b.ConstraintArmed || b.ConstraintUnapplied != "" {
		t.Errorf("corrected date: armed %v unapplied %q, want armed and no reason", b.ConstraintArmed, b.ConstraintUnapplied)
	}

	// Without a project start, every dated constraint is marked, and an
	// earlier arming is withdrawn.
	MarkConstraintsUnapplied(tasks, ConstraintNoStart)
	for id, task := range tasks {
		w := ConstraintNoStart
		if id == "alap" {
			w = ""
		}
		if task.ConstraintUnapplied != w || task.ConstraintArmed {
			t.Errorf("no start, %s: armed %v unapplied %q, want unarmed %q", id, task.ConstraintArmed, task.ConstraintUnapplied, w)
		}
	}
}

// ----- KRN-01: MSO, SNLT and FNET (2026-11-18) -----
// Day indices on the weekday calendar from Monday 2026-06-01:
// Mon 0, Tue 1, Wed 2, Thu 3, Fri 4. Every expected value below is
// derived by hand in its comment.

func TestApplyConstraintDatesArmsEveryDateConstraint(t *testing.T) {
	cases := []struct {
		c        ConstraintType
		duration float64
		want     float64
	}{
		{StartNoEarlierThan, 2, 3},
		{StartNoLaterThan, 2, 3},
		{MustStartOn, 2, 3},
		{FinishNoEarlierThan, 2, 4}, // finishing ON Thursday: EF = 3+1
		{FinishNoLaterThan, 2, 4},
		{MustFinishOn, 2, 4},
		{FinishNoEarlierThan, 0, 4}, // a milestone too: "by the end of Thursday"
		{FinishNoLaterThan, 0, 4},
		{MustFinishOn, 0, 4},
	}
	for _, c := range cases {
		tasks := map[string]*Task{"A": {ID: "A", Duration: c.duration, Constraint: c.c, ConstraintDate: "2026-06-04"}}
		ApplyConstraintDates(tasks, conStart, weekdaysOnly)
		// Thursday is a working day, so both readings agree: no shift.
		if a := tasks["A"]; !a.ConstraintArmed || a.ConstraintDay != c.want || a.ConstraintLateShift != 0 {
			t.Errorf("%s (duration %v): armed %v day %v shift %v, want armed day %v shift 0", c.c, c.duration, a.ConstraintArmed, a.ConstraintDay, a.ConstraintLateShift, c.want)
		}
	}
}

// ----- Dates on non-working days and before the project start -----
// (owner decision 2026-11-18, "flag as violated"). On the weekday
// calendar from Monday 2026-06-01: Fri 06-05 is 4, Sat 06-06 and Sun
// 06-07 are not working days, Mon 06-08 is 5; before the start, Fri
// 05-29 is -1 and Thu 05-28 is -2. An earliest limit reads a date as the
// next working day, a latest limit as the previous one.

func TestLatestWorkdayOffset(t *testing.T) {
	saturdayStart := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		start time.Time
		date  time.Time
		want  float64
		ok    bool
	}{
		{"working day", conStart, time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), 3, true},
		{"the start itself", conStart, conStart, 0, true},
		{"Saturday reads as Friday", conStart, time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC), 4, true},
		{"Sunday reads as Friday", conStart, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), 4, true},
		{"Sunday before the start", conStart, time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), -1, true},
		{"Friday before the start", conStart, time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), -1, true},
		{"Thursday before the start", conStart, time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC), -2, true},
		{"Saturday start, first Monday", saturdayStart, conStart, 0, true},
		{"Saturday start, the start itself", saturdayStart, saturdayStart, -1, true},
		{"beyond the walk limit", conStart, time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC), 0, false},
	}
	for _, c := range cases {
		got, ok := latestWorkdayOffset(c.start, c.date, weekdaysOnly)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s: got %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestApplyConstraintDatesReadsAWeekendDateBothWays(t *testing.T) {
	// Saturday 06-06: the next working day is Mon 5, the previous Fri 4,
	// so every type is armed with a shift of 1; finish types add 1 to both.
	cases := []struct {
		c    ConstraintType
		want float64
	}{
		{StartNoEarlierThan, 5}, {StartNoLaterThan, 5}, {MustStartOn, 5},
		{FinishNoEarlierThan, 6}, {FinishNoLaterThan, 6}, {MustFinishOn, 6},
	}
	for _, c := range cases {
		tasks := map[string]*Task{"A": {ID: "A", Duration: 1, Constraint: c.c, ConstraintDate: "2026-06-06"}}
		ApplyConstraintDates(tasks, conStart, weekdaysOnly)
		if a := tasks["A"]; !a.ConstraintArmed || a.ConstraintDay != c.want || a.ConstraintLateShift != 1 {
			t.Errorf("%s: armed %v day %v shift %v, want armed day %v shift 1", c.c, a.ConstraintArmed, a.ConstraintDay, a.ConstraintLateShift, c.want)
		}
	}
	// A date so far back that the latest reading cannot be walked is left
	// unarmed, like any other date that cannot be converted.
	tasks := map[string]*Task{"A": {ID: "A", Duration: 1, Constraint: StartNoLaterThan, ConstraintDate: "1500-01-01"}}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	if tasks["A"].ConstraintArmed {
		t.Error("a date beyond the walk limit must stay unarmed")
	}
}

func TestSNLTOnASaturdayMeansFriday(t *testing.T) {
	// A (5 days) runs Mon-Fri, so B can start Mon 06-08 (5) at the
	// earliest. SNLT Sat 06-06 means "by Friday" (4): violated. Project
	// finish 6; B's LF is capped at 4+1 = 5, LS 4, float -1; A's LF is
	// B's LS 4, LS -1, float -1.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 5},
		"B": {ID: "B", Duration: 1, Precedents: []string{"A"}, Constraint: StartNoLaterThan, ConstraintDate: "2026-06-06"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 5)
	approx(t, "B.LF", tasks["B"].LF, 5)
	approx(t, "B.Float", tasks["B"].Float, -1)
	approx(t, "A.Float", tasks["A"].Float, -1)
	if !tasks["B"].ConstraintViolated {
		t.Error("a Monday start does not meet SNLT Saturday")
	}

	// A of 4 days lets B start Friday (4): met. Project finish 5, B's LF
	// min(5, 4+1) = 5, LS 4, float 0.
	tasks["A"].Duration = 4
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES (A 4 days)", tasks["B"].ES, 4)
	approx(t, "B.Float (A 4 days)", tasks["B"].Float, 0)
	if tasks["B"].ConstraintViolated {
		t.Error("a Friday start meets SNLT Saturday")
	}
}

func TestFNLTBeforeTheProjectStartIsViolated(t *testing.T) {
	// FNLT Fri 05-29, the working day before the start: latest finish
	// index -1+1 = 0. The task runs ES 0, EF 1: violated; project finish
	// 1, LF min(1, 0) = 0, LS -1, float -1.
	tasks := map[string]*Task{
		"C": {ID: "C", Duration: 1, Constraint: FinishNoLaterThan, ConstraintDate: "2026-05-29"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "C.ES", tasks["C"].ES, 0)
	approx(t, "C.LF", tasks["C"].LF, 0)
	approx(t, "C.Float", tasks["C"].Float, -1)
	if !tasks["C"].ConstraintViolated {
		t.Error("a finish on the first project day does not meet a date before it")
	}
}

func TestMustDatesThatAreNotWorkingDaysCannotBeMet(t *testing.T) {
	// Each task is alone (1 day, no links).
	//  - MSO Sat 06-06: early 5, late 4. ES pulled to 5 (Monday), 5 > 4
	//    violated; EF 6 = project finish; LF pinned 4+1 = 5, LS 4, float -1.
	//  - MFO Sun 06-07: early finish 5+1 = 6, late finish 4+1 = 5. ES
	//    6-1 = 5, EF 6 > 5 violated; LF 5, LS 4, float -1.
	//  - MSO Thu 05-28, before the start: early 0, late -2. ES 0 > -2
	//    violated; LF -2+1 = -1, LS -2, float -2.
	cases := []struct {
		name      string
		c         ConstraintType
		date      string
		es, lf, f float64
	}{
		{"MSO Saturday", MustStartOn, "2026-06-06", 5, 5, -1},
		{"MFO Sunday", MustFinishOn, "2026-06-07", 5, 5, -1},
		{"MSO before the start", MustStartOn, "2026-05-28", 0, -1, -2},
	}
	for _, c := range cases {
		tasks := map[string]*Task{"A": {ID: "A", Duration: 1, Constraint: c.c, ConstraintDate: c.date}}
		ApplyConstraintDates(tasks, conStart, weekdaysOnly)
		mustCPM(t, tasks)
		a := tasks["A"]
		approx(t, c.name+" ES", a.ES, c.es)
		approx(t, c.name+" LF", a.LF, c.lf)
		approx(t, c.name+" Float", a.Float, c.f)
		if !a.ConstraintViolated {
			t.Errorf("%s: must be violated", c.name)
		}
	}

	// The earliest side is unchanged: SNET Saturday starts the task on
	// Monday (5) and is never violated.
	tasks := map[string]*Task{"A": {ID: "A", Duration: 1, Constraint: StartNoEarlierThan, ConstraintDate: "2026-06-06"}}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "SNET Saturday ES", tasks["A"].ES, 5)
	if tasks["A"].ConstraintViolated {
		t.Error("SNET is never violated")
	}
}

func TestMSOPinsStartAndLateStart(t *testing.T) {
	// Links allow B to start Tuesday (1); MSO Thursday (3) pulls it to
	// 3, EF 5. B pinned: LF = 3+2 = 5, LS 3, float 0. A's late finish is
	// B's late start 3: LS 2, float 2.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"}, Constraint: MustStartOn, ConstraintDate: "2026-06-04"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 3)
	approx(t, "B.EF", tasks["B"].EF, 5)
	approx(t, "B.LS", tasks["B"].LS, 3)
	approx(t, "B.Float", tasks["B"].Float, 0)
	approx(t, "A.Float", tasks["A"].Float, 2)
	if tasks["B"].ConstraintViolated {
		t.Error("satisfiable MSO must not be flagged")
	}
	AnchorSchedule(tasks, conStart, weekdaysOnly)
	if b := tasks["B"]; b.StartDate != "2026-06-04" || b.FinishDate != "2026-06-05" {
		t.Errorf("B dates = %s..%s, want 2026-06-04..2026-06-05", b.StartDate, b.FinishDate)
	}
}

func TestMSOViolatedWhenLinksPushPast(t *testing.T) {
	// A (4d) forces B to start day 4; MSO Tuesday (1) cannot hold. Links
	// win: B.ES 4, flagged. B's late dates stay pinned: LF 1+2 = 3, LS 1,
	// float 1-4 = -3; A's LF = B.LS 1, LS -3, float -3.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"}, Constraint: MustStartOn, ConstraintDate: "2026-06-02"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 4)
	approx(t, "B.Float", tasks["B"].Float, -3)
	approx(t, "A.Float", tasks["A"].Float, -3)
	if !tasks["B"].ConstraintViolated {
		t.Error("MSO pushed past its date must set ConstraintViolated")
	}
}

func TestMSOViolatedWhenASuccessorNeedsItEarlier(t *testing.T) {
	// B: MSO Thursday, ES 3, EF 5. C (1d, FS after B) must finish on
	// Thursday (MFO, armed 3+1 = 4), so C's late start is 3 and B would
	// have to finish by 3, before its pinned finish 5: B is flagged in the
	// backward pass although its own start holds. B keeps LF pinned at 5.
	tasks := map[string]*Task{
		"B": {ID: "B", Duration: 2, Constraint: MustStartOn, ConstraintDate: "2026-06-04"},
		"C": {ID: "C", Duration: 1, Precedents: []string{"B"}, Constraint: MustFinishOn, ConstraintDate: "2026-06-04"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 3)
	approx(t, "B.LF", tasks["B"].LF, 5)
	if !tasks["B"].ConstraintViolated {
		t.Error("an MSO task its successor needs earlier must be flagged")
	}
}

func TestMSOWithStartToStartLinkFromPredecessor(t *testing.T) {
	// B starts at least 2 days after A starts (SS+2): links allow 2; MSO
	// Thursday pulls B to 3. Backward: A's SS candidate is
	// B.LS - 2 + A.Duration = 3-2+3 = 4; project finish max(3, 5) = 5;
	// A.LF 4, LS 1, float 1.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 2, Links: []Link{{Pred: "A", Type: StartToStart, Lag: 2}}, Constraint: MustStartOn, ConstraintDate: "2026-06-04"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 3)
	approx(t, "A.LF", tasks["A"].LF, 4)
	approx(t, "A.Float", tasks["A"].Float, 1)
	if tasks["B"].ConstraintViolated {
		t.Error("satisfiable MSO behind an SS link must not be flagged")
	}
}

func TestSNLTCapsLateStart(t *testing.T) {
	// C (8d) sets the project finish 8. B (2d, after A 1d) starts Tuesday
	// (1); SNLT Thursday caps its late start at 3: LF 3+2 = 5, LS 3,
	// float 2 (instead of 5 without the cap). A: LF = B.LS 3, float 2.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"}, Constraint: StartNoLaterThan, ConstraintDate: "2026-06-04"},
		"C": {ID: "C", Duration: 8},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 1)
	approx(t, "B.LS", tasks["B"].LS, 3)
	approx(t, "B.Float", tasks["B"].Float, 2)
	approx(t, "A.Float", tasks["A"].Float, 2)
	if tasks["B"].ConstraintViolated {
		t.Error("satisfied SNLT must not be flagged")
	}
}

func TestSNLTViolatedWhenLinksPushPast(t *testing.T) {
	// A (4d) forces B to start day 4, after SNLT Tuesday (1): flagged.
	// LF capped 1+2 = 3, LS 1, float -3; A float 1-4 = -3.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"}, Constraint: StartNoLaterThan, ConstraintDate: "2026-06-02"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "B.ES", tasks["B"].ES, 4)
	approx(t, "B.Float", tasks["B"].Float, -3)
	approx(t, "A.Float", tasks["A"].Float, -3)
	if !tasks["B"].ConstraintViolated {
		t.Error("SNLT started late must set ConstraintViolated")
	}
}

func TestFNETDelaysFinish(t *testing.T) {
	// A (2d) must finish no earlier than Friday (armed 4+1 = 5): ES 3,
	// EF 5, shown Thursday..Friday. B (2d after a 4d task) already
	// finishes after Wednesday, so its FNET Wednesday changes nothing.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2, Constraint: FinishNoEarlierThan, ConstraintDate: "2026-06-05"},
		"P": {ID: "P", Duration: 4},
		"B": {ID: "B", Duration: 2, Precedents: []string{"P"}, Constraint: FinishNoEarlierThan, ConstraintDate: "2026-06-03"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "A.ES", tasks["A"].ES, 3)
	approx(t, "A.EF", tasks["A"].EF, 5)
	approx(t, "B.ES", tasks["B"].ES, 4)
	if tasks["A"].ConstraintViolated || tasks["B"].ConstraintViolated {
		t.Error("FNET only delays and is never violated")
	}
	AnchorSchedule(tasks, conStart, weekdaysOnly)
	if a := tasks["A"]; a.StartDate != "2026-06-04" || a.FinishDate != "2026-06-05" {
		t.Errorf("A dates = %s..%s, want 2026-06-04..2026-06-05", a.StartDate, a.FinishDate)
	}
}

func TestMilestoneFinishConstraintsMeanTheEndOfTheDay(t *testing.T) {
	// A milestone sits on the boundary after the work before it: after A
	// (3 days, Monday to Wednesday) it is at index 3, the start of
	// Thursday. A finish constraint on Wednesday means "by the end of
	// Wednesday", armed 2+1 = 3, so MFO and FNLT Wednesday are met (MFO
	// pins LF 3; float 0 for M and for A). After B (4 days, to Thursday)
	// the milestone is at 4, past FNLT Wednesday: flagged. FNET Wednesday
	// on a free milestone holds it at 3.
	tasks := map[string]*Task{
		"A":    {ID: "A", Duration: 3},
		"MFO":  {ID: "MFO", Duration: 0, Milestone: true, Precedents: []string{"A"}, Constraint: MustFinishOn, ConstraintDate: "2026-06-03"},
		"FNLT": {ID: "FNLT", Duration: 0, Milestone: true, Precedents: []string{"A"}, Constraint: FinishNoLaterThan, ConstraintDate: "2026-06-03"},
		"B":    {ID: "B", Duration: 4},
		"LATE": {ID: "LATE", Duration: 0, Milestone: true, Precedents: []string{"B"}, Constraint: FinishNoLaterThan, ConstraintDate: "2026-06-03"},
		"FNET": {ID: "FNET", Duration: 0, Milestone: true, Constraint: FinishNoEarlierThan, ConstraintDate: "2026-06-03"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "MFO.ES", tasks["MFO"].ES, 3)
	approx(t, "MFO.Float", tasks["MFO"].Float, 0)
	approx(t, "FNET.ES", tasks["FNET"].ES, 3)
	for _, id := range []string{"MFO", "FNLT", "FNET"} {
		if tasks[id].ConstraintViolated {
			t.Errorf("%s milestone at the end of Wednesday must not be flagged", id)
		}
	}
	if !tasks["LATE"].ConstraintViolated {
		t.Error("a milestone after work ending Thursday misses FNLT Wednesday and must be flagged")
	}
}

// A levelled start (written by resource levelling, not a constraint)
// delays a task like a start floor once armed, and a latest limit it
// breaks is flagged on the next schedule. A (1 day) with levelled start
// Wed 06-03 starts at 2; B with MSO Tue 06-02 (1) and levelled start Thu
// 06-04: the levelled start sets ES 3 before MSO is checked, and 3 > 1,
// so MSO is violated.
// An unreadable levelled start is ignored.
func TestLevelStartDelaysATaskAndCanBreakItsConstraint(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, LevelStart: "2026-06-03"},
		"B": {ID: "B", Duration: 1, LevelStart: "2026-06-04", Constraint: MustStartOn, ConstraintDate: "2026-06-02"},
		"C": {ID: "C", Duration: 1, LevelStart: "junk"},
	}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	approx(t, "A.ES", tasks["A"].ES, 2)
	approx(t, "B.ES", tasks["B"].ES, 3)
	approx(t, "C.ES", tasks["C"].ES, 0)
	if !tasks["B"].ConstraintViolated || tasks["A"].ConstraintViolated {
		t.Errorf("violated: A %v B %v, want only B", tasks["A"].ConstraintViolated, tasks["B"].ConstraintViolated)
	}
}
