// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

// Which levelled starts the saved chart keeps (Gate A R4.1). One resource,
// capacity 1; hand-worked schedules.
func TestLevelStartsToKeep(t *testing.T) {
	alice := []Assignment{{Resource: "alice"}}
	cases := []struct {
		name  string
		tasks map[string]*Task
		split bool
		want  map[string]float64
	}{
		{
			// X (critical through Y) takes day 0; A waits to day 1 and B
			// follows A by its link: only A needs its levelled start.
			name: "a delayed task keeps its start; its successor follows by the link",
			tasks: map[string]*Task{
				"X": {ID: "X", Duration: 1, Assignments: alice},
				"Y": {ID: "Y", Duration: 2, Precedents: []string{"X"}},
				"A": {ID: "A", Duration: 1, Assignments: alice},
				"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
			},
			want: map[string]float64{"A": 1},
		},
		{
			// X takes day 0 and W day 2 (both critical); A (2 days) splits
			// onto days 1 and 3, finishing at 4. Saved, A runs [1, 3) as
			// far as the forward pass sees, so B (after A) needs its own
			// levelled start, 4.
			name:  "the successor of a split task keeps its start",
			split: true,
			tasks: map[string]*Task{
				"P":  {ID: "P", Duration: 2},
				"X":  {ID: "X", Duration: 1, Assignments: alice},
				"X2": {ID: "X2", Duration: 5, Precedents: []string{"X"}},
				"W":  {ID: "W", Duration: 1, Precedents: []string{"P"}, Assignments: alice},
				"W2": {ID: "W2", Duration: 3, Precedents: []string{"W"}},
				"A":  {ID: "A", Duration: 2, Assignments: alice},
				"B":  {ID: "B", Duration: 1, Precedents: []string{"A"}},
			},
			want: map[string]float64{"A": 1, "B": 4},
		},
		{
			// Q is ALAP: levelling places it at its late start, 2, but the
			// ALAP pass does that itself, so it keeps nothing.
			name: "an ALAP task keeps nothing",
			tasks: map[string]*Task{
				"Q": {ID: "Q", Duration: 1, Constraint: AsLateAsPossible, Assignments: alice},
				"R": {ID: "R", Duration: 3},
			},
			want: map[string]float64{},
		},
		{
			// S has SNET day 2 and nothing contends with it there: its own
			// constraint holds it, so it keeps nothing. T is held to day 3
			// by contention with S, past its SNET day 1: it keeps 3.
			name: "a task its own constraint holds keeps nothing",
			tasks: map[string]*Task{
				"S":  {ID: "S", Duration: 1, Assignments: alice, Constraint: StartNoEarlierThan, ConstraintArmed: true, ConstraintDay: 2},
				"S2": {ID: "S2", Duration: 4, Precedents: []string{"S"}},
				"T":  {ID: "T", Duration: 2, Assignments: alice, Constraint: StartNoEarlierThan, ConstraintArmed: true, ConstraintDay: 1},
			},
			want: map[string]float64{"T": 3},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LevelResourcesWithOptions(tc.tasks, capacityPlanFromMap(nil), LevelingOptions{AllowSplitting: tc.split}); err != nil {
				t.Fatalf("LevelResourcesWithOptions: %v", err)
			}
			got, ok := LevelStartsToKeep(tc.tasks)
			if !ok {
				t.Fatal("LevelStartsToKeep reported a cycle")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("kept %v, want %v", got, tc.want)
			}
		})
	}
}

// RebuildSplitDays uses saved segments only while they describe the task
// (Gate A R4.2, M6) and checks a finish limit against the rebuilt finish
// (C1).
func TestRebuildSplitDays(t *testing.T) {
	segs := []WorkSegment{{Start: 0, End: 1}, {Start: 2, End: 3}}
	cases := []struct {
		name     string
		task     Task
		days     []int
		ef       float64
		violated bool
	}{
		{"the tail day of a split counts", Task{Duration: 2, PlannedWorkSegments: segs}, []int{0, 2}, 3, false},
		{"a duration edited since levelling ignores the segments", Task{Duration: 3, PlannedWorkSegments: segs}, nil, 3, false},
		{"a part-day start ignores the segments", Task{Duration: 2, LevelStartArmed: true, LevelStartDay: 0.5, PlannedWorkSegments: segs}, nil, 2.5, false},
		{"segments that are not whole days are ignored", Task{Duration: 2, PlannedWorkSegments: []WorkSegment{{0, 0.5}, {2, 3.5}}}, nil, 2, false},
		{"overlapping segments are ignored", Task{Duration: 2, PlannedWorkSegments: []WorkSegment{{0, 1}, {0, 1}}}, nil, 2, false},
		{
			// FNLT by the end of day 1 (stored as 2): the contiguous finish,
			// 2, meets it; the rebuilt finish, 3, does not.
			"a split tail past a finish limit is flagged",
			Task{Duration: 2, PlannedWorkSegments: segs, Constraint: FinishNoLaterThan, ConstraintArmed: true, ConstraintDay: 2},
			[]int{0, 2}, 3, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.task
			task.ID = "S"
			tasks := map[string]*Task{"S": &task}
			if err := ScheduleCPM(tasks); err != nil {
				t.Fatal(err)
			}
			if task.ConstraintViolated {
				t.Fatal("violated before the rebuild")
			}
			RebuildSplitDays(tasks)
			if !reflect.DeepEqual(task.WorkDays, tc.days) {
				t.Errorf("WorkDays %v, want %v", task.WorkDays, tc.days)
			}
			approx(t, "EF", task.EF, tc.ef)
			if task.ConstraintViolated != tc.violated {
				t.Errorf("violated %v, want %v", task.ConstraintViolated, tc.violated)
			}
		})
	}
}

// The saved form reproduces the levelled schedule (Gate A M5): on random
// networks with every link type, lags, start and finish constraints, part
// days and splitting, scheduling a fresh copy that holds only the kept
// levelled starts and the split segments gives every task the levelled
// start, finish and working days. ALAP tasks are left out: the ALAP pass
// can place a task later than levelling did (a documented limit, covered
// by the hand case above and the desktop tests).
func TestSavedLevelledScheduleMatchesLevelling(t *testing.T) {
	rng := rand.New(rand.NewPCG(20261123, 11))
	kept, split := 0, 0
	for i := range 1000 {
		input := map[string]*Task{}
		for k := range 4 + rng.IntN(12) {
			// Half whole days (splitting books whole days), half eighths.
			task := &Task{ID: fmt.Sprintf("T%02d", k), Duration: float64(1 + rng.IntN(3))}
			if rng.IntN(2) == 0 {
				task.Duration = float64(rng.IntN(25)) / 8
			}
			if k > 0 && rng.IntN(2) == 0 {
				task.Links = append(task.Links, Link{Pred: fmt.Sprintf("T%02d", rng.IntN(k)),
					Type: pickFrom(rng, FinishToStart, StartToStart, FinishToFinish, StartToFinish),
					Lag:  pickFrom(rng, 0, 0, 0.5, 1, -0.5)})
			}
			if rng.IntN(5) == 0 {
				task.Constraint = pickFrom(rng, StartNoEarlierThan, FinishNoEarlierThan, MustStartOn, MustFinishOn)
				task.ConstraintArmed, task.ConstraintDay = true, float64(rng.IntN(6))
			}
			for range rng.IntN(3) {
				task.Assignments = append(task.Assignments,
					Assignment{Resource: pickFrom(rng, "r0", "r1"), Units: pickFrom(rng, 0.5, 1)})
			}
			input[task.ID] = task
		}
		levelled := cloneTaskMap(input)
		opts := LevelingOptions{Horizon: 40, AllowSplitting: rng.IntN(2) == 0}
		if _, err := LevelResourcesWithOptions(levelled, capacityPlanFromMap(nil), opts); err != nil && !errors.Is(err, ErrLevelingHorizonExceeded) {
			t.Fatalf("case %d: %v", i, err)
		}
		keep, ok := LevelStartsToKeep(levelled)
		if !ok {
			t.Fatalf("case %d: cycle", i)
		}
		kept += len(keep)

		saved := cloneTaskMap(input)
		for id, task := range saved {
			if day, ok := keep[id]; ok {
				task.LevelStartArmed, task.LevelStartDay = true, day
			}
			if wd := levelled[id].WorkDays; len(wd) > 0 {
				split++
				for _, d := range wd {
					task.PlannedWorkSegments = append(task.PlannedWorkSegments,
						WorkSegment{Start: float64(d - wd[0]), End: float64(d - wd[0] + 1)})
				}
			}
		}
		if err := ScheduleCPM(saved); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		RebuildSplitDays(saved)
		for id, want := range levelled {
			got := saved[id]
			if !sameOffset(got.ES, want.ES) || !sameOffset(got.EF, want.EF) || !reflect.DeepEqual(got.WorkDays, want.WorkDays) {
				t.Fatalf("case %d task %s (%+v): saved ES %v EF %v days %v, levelled ES %v EF %v days %v (kept %v)",
					i, id, opts, got.ES, got.EF, got.WorkDays, want.ES, want.EF, want.WorkDays, keep)
			}
		}
	}
	t.Logf("%d levelled starts kept, %d split tasks", kept, split)
	if kept < 300 || split < 50 {
		t.Errorf("generator kept %d starts and split %d tasks, want at least 300 and 50", kept, split)
	}
}

func sameOffset(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
