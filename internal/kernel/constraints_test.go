// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
