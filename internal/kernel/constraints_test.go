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
		if a := tasks["A"]; !a.ConstraintArmed || a.ConstraintDay != c.want {
			t.Errorf("%s (duration %v): armed %v day %v, want armed day %v", c.c, c.duration, a.ConstraintArmed, a.ConstraintDay, c.want)
		}
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
