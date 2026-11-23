// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

// A start time read from a file moves to the next working start when it
// falls outside working time (08:00-12:00, 13:00-17:00 on weekdays).
// Project from Monday 2026-06-01.
func TestWorkingClockReadsTimesBack(t *testing.T) {
	w := newWorkdayTable(conStart, weekdaysOnly)
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 6, day, hour, minute, 0, 0, time.UTC) }
	for _, c := range []struct {
		at   time.Time
		want float64
	}{
		{at(1, 8, 0), 0},
		{at(1, 7, 0), 0},
		{at(1, 10, 0), 0.25},
		{at(1, 12, 30), 0.5}, // lunch: the afternoon's start
		{at(1, 13, 0), 0.5},
		{at(1, 15, 0), 0.75},
		{at(1, 17, 0), 1},    // end of day: Tuesday 08:00
		{at(5, 18, 0), 5},    // Friday evening: Monday 06-08
		{at(6, 10, 0), 5},    // Saturday: Monday 06-08
		{at(8, 10, 0), 5.25}, // Monday 06-08 10:00
		{time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), 0}, // before the start
	} {
		if got, ok := w.offset(c.at); !ok || got != c.want {
			t.Errorf("%s read back as %v (ok %v), want %v", c.at.Format(time.RFC3339), got, ok, c.want)
		}
	}
}

// delayNetwork is P (4.75 days, so C after it can start Friday 06-05
// 15:00), B (no links) and C after P, with levelled starts as given.
func delayNetwork() map[string]*Task {
	return map[string]*Task{
		"P": {ID: "P", Duration: 4.75},
		"B": {ID: "B", Duration: 1},
		"C": {ID: "C", Duration: 1, Precedents: []string{"P"}},
		"Q": {ID: "Q", Duration: 1, Constraint: AsLateAsPossible},
		"N": {ID: "N", Duration: 1},
	}
}

// Delays are measured on the clock from where the links and constraints
// put the task: B from Monday 08:00 to Friday 15:00 is 4 days and 7 hours
// (6180 minutes); C from Friday 15:00 to Monday 10:00 spans the weekend,
// 67 hours (4020 minutes). An ALAP task and a levelled start that does
// not bind (N's is its own earliest start) have none.
func TestLevelingDelayMinutes(t *testing.T) {
	tasks := delayNetwork()
	tasks["B"].LevelStart, tasks["B"].LevelStartFraction = "2026-06-05", 0.75
	tasks["C"].LevelStart, tasks["C"].LevelStartFraction = "2026-06-08", 0.25
	tasks["Q"].LevelStart = "2026-06-03"
	tasks["N"].LevelStart = "2026-06-01"
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	mustCPM(t, tasks)
	got := LevelingDelayMinutes(tasks, conStart, weekdaysOnly)
	if want := map[string]int{"B": 6180, "C": 4020}; !reflect.DeepEqual(got, want) {
		t.Errorf("delays %v, want %v", got, want)
	}
}

// Applying delays read from a file gives the levelled starts back: the
// elapsed delays above, a working delay of 1.5 days, an elapsed delay
// ending on a Saturday (to Monday 08:00, offset 5), and skips: an ALAP
// task and a delay beyond the schedule limit.
func TestApplyLevelingDelays(t *testing.T) {
	tasks := delayNetwork()
	tasks["S"] = &Task{ID: "S", Duration: 1}
	tasks["X"] = &Task{ID: "X", Duration: 1}
	ApplyConstraintDates(tasks, conStart, weekdaysOnly)
	starts, skipped, ok := ApplyLevelingDelays(tasks, map[string]LevelingDelay{
		"B": {6180, true},
		"C": {4020, true},
		"N": {1.5, false},
		"S": {5 * 1440, true}, // Monday 08:00 + 5 days: Saturday 08:00
		"Q": {60, true},
		"X": {MaxScheduleDays, false},
	}, conStart, weekdaysOnly)
	if !ok {
		t.Fatal("cycle")
	}
	if want := map[string]float64{"B": 4.75, "C": 5.25, "N": 1.5, "S": 5}; !reflect.DeepEqual(starts, want) {
		t.Errorf("starts %v, want %v", starts, want)
	}
	if want := []string{"Q", "X"}; !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped %v, want %v", skipped, want)
	}
	if tasks["B"].ES != 0 || tasks["B"].EF != 0 {
		t.Error("ApplyLevelingDelays changed the tasks' dates")
	}
}

// Measuring the delays of a levelled schedule and applying them to the
// same network without its levelled starts gives the same starts, on
// random networks with links, lags and part-day starts on the eighth-day
// grid (whole minutes, so the round trip is exact).
func TestLevelingDelaysRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(20261123, 13))
	moved := 0
	for i := range 500 {
		build := func() map[string]*Task {
			r := rand.New(rand.NewPCG(uint64(i), 1))
			tasks := map[string]*Task{}
			for k := range 2 + r.IntN(10) {
				task := &Task{ID: fmt.Sprintf("T%02d", k), Duration: float64(r.IntN(17)) / 8}
				if k > 0 && r.IntN(2) == 0 {
					task.Links = []Link{{Pred: fmt.Sprintf("T%02d", r.IntN(k)),
						Type: pickFrom(r, FinishToStart, StartToStart, FinishToFinish), Lag: float64(r.IntN(9)-2) / 8}}
				}
				tasks[task.ID] = task
			}
			return tasks
		}
		levelled := build()
		ApplyConstraintDates(levelled, conStart, weekdaysOnly)
		mustCPM(t, levelled)
		// Hold some tasks back by a whole number of eighths, in topological
		// order so each delay is measured from the delayed predecessors.
		want := map[string]float64{}
		order, _ := topoSort(levelled)
		for _, id := range order {
			if rng.IntN(3) != 0 {
				continue
			}
			mustCPM(t, levelled)
			at := levelled[id].ES + float64(1+rng.IntN(24))/8
			day, f := SplitDayOffset(at)
			levelled[id].LevelStart, levelled[id].LevelStartFraction = weekdayDate(day), f
			ApplyConstraintDates(levelled, conStart, weekdaysOnly)
			want[id] = at
		}
		mustCPM(t, levelled)
		delays := map[string]LevelingDelay{}
		for id, minutes := range LevelingDelayMinutes(levelled, conStart, weekdaysOnly) {
			delays[id] = LevelingDelay{float64(minutes), true}
		}
		fresh := build()
		ApplyConstraintDates(fresh, conStart, weekdaysOnly)
		got, skipped, _ := ApplyLevelingDelays(fresh, delays, conStart, weekdaysOnly)
		if len(skipped) > 0 || len(got) != len(want) {
			t.Fatalf("case %d: applied %v (skipped %v), want %v", i, got, skipped, want)
		}
		for id, at := range want {
			if !sameOffset(got[id], at) {
				t.Fatalf("case %d: %s applied at %v, levelled at %v (delays %v)", i, id, got[id], at, delays)
			}
		}
		moved += len(want)
	}
	if moved < 500 {
		t.Errorf("generator delayed %d tasks, want at least 500", moved)
	}
}
