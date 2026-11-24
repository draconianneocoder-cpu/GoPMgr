// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"fmt"
	"math"
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
			// A and B (2 days each) are serialised, B to day 2. Q (ALAP)
			// waits for alice from its late start, 1, to day 4. Saved
			// without its start, the ALAP pass would put Q at the saved
			// schedule's late start, 3, on top of B, so Q keeps 4 (owner
			// decision 2026-11-23, "keep levelled position").
			name: "a levelled ALAP task keeps its levelled start",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 2, Assignments: alice},
				"B": {ID: "B", Duration: 2, Assignments: alice},
				"Q": {ID: "Q", Duration: 1, Constraint: AsLateAsPossible, Assignments: alice},
			},
			want: map[string]float64{"B": 2, "Q": 4},
		},
		{
			// R is ALAP with no resource: levelling does not place it, so
			// the ALAP pass keeps doing so.
			name: "an ALAP task without a resource keeps nothing",
			tasks: map[string]*Task{
				"R": {ID: "R", Duration: 1, Constraint: AsLateAsPossible},
				"S": {ID: "S", Duration: 3},
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
// start, finish and working days, ALAP tasks levelling places included
// (since 2026-11-23 they keep their levelled start). An ALAP task with no
// resource is not placed by levelling: the ALAP pass puts it at the saved
// schedule's late dates, which levelling may have moved.
func TestSavedLevelledScheduleMatchesLevelling(t *testing.T) {
	rng := rand.New(rand.NewPCG(20261123, 11))
	kept, split, alap := 0, 0, 0
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
				task.Constraint = pickFrom(rng, StartNoEarlierThan, FinishNoEarlierThan, MustStartOn, MustFinishOn, AsLateAsPossible)
				task.ConstraintArmed, task.ConstraintDay = task.Constraint != AsLateAsPossible, float64(rng.IntN(6))
				if task.Constraint == AsLateAsPossible {
					task.ConstraintDay = 0
				}
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
		for id := range keep {
			if levelled[id].Constraint == AsLateAsPossible {
				alap++
			}
		}

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
			if want.Constraint == AsLateAsPossible && !levelledByResource(want) {
				continue // the ALAP pass places it, on the saved schedule
			}
			got := saved[id]
			if !sameOffset(got.ES, want.ES) || !sameOffset(got.EF, want.EF) || !reflect.DeepEqual(got.WorkDays, want.WorkDays) {
				t.Fatalf("case %d task %s (%+v): saved ES %v EF %v days %v, levelled ES %v EF %v days %v (kept %v)",
					i, id, opts, got.ES, got.EF, got.WorkDays, want.ES, want.EF, want.WorkDays, keep)
			}
		}
	}
	t.Logf("%d levelled starts kept (%d ALAP), %d split tasks", kept, alap, split)
	if kept < 300 || split < 50 || alap < 100 {
		t.Errorf("generator kept %d starts (%d ALAP) and split %d tasks, want at least 300 (100) and 50", kept, alap, split)
	}
}

func sameOffset(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// Monte Carlo follows a levelled split (owner decision 2026-11-24, "follow
// the split days"). S (3 days) is split onto days 0, 2 and 4 and is the
// last work, so the schedule finishes at 5. A run works the sampled
// duration through those days in order, a whole day each: 3 days (the
// plan) ends at 5, 2 days at the end of day 2 (3), 4 days runs one day
// past day 4 (6). A split the schedule ignores (S needing 4 whole days
// for its three split days) is ignored: 4 days from 0 end at 4.
func TestMonteCarloFollowsSplitDays(t *testing.T) {
	split := []WorkSegment{{Start: 0, End: 1}, {Start: 2, End: 3}, {Start: 4, End: 5}}
	fixed := func(d float64) DurationEstimate {
		return DurationEstimate{Optimistic: d, MostLikely: d, Pessimistic: d}
	}
	for _, c := range []struct {
		name     string
		duration float64
		estimate DurationEstimate
		want     float64
	}{
		{"planned duration", 3, DurationEstimate{}, 5},
		{"shorter sample", 3, fixed(2), 3},
		{"longer sample", 3, fixed(4), 6},
		{"part of a day shorter", 3, fixed(2.5), 5},
		{"a split the schedule ignores", 4, DurationEstimate{}, 4},
	} {
		tasks := map[string]*Task{"S": {ID: "S", Duration: c.duration, DurationEstimate: c.estimate, PlannedWorkSegments: split}}
		r := RunMonteCarlo(tasks, 20, 2)
		if !r.Valid || r.P50 != c.want || r.P90 != c.want {
			t.Errorf("%s: P50 %v P90 %v (valid %v, %s), want %v", c.name, r.P50, r.P90, r.Valid, r.Error, c.want)
		}
	}

	// A normal estimate from 0 clamps about half its draws to exactly 0: no
	// work, so those runs finish where S starts (0), not after its first
	// split day (1). The finish curve's 40% point falls among them.
	zero := DurationEstimate{Optimistic: 0, MostLikely: 0, Pessimistic: 1, Distribution: MonteCarloNormal}
	r := RunMonteCarlo(map[string]*Task{"S": {ID: "S", Duration: 3, DurationEstimate: zero, PlannedWorkSegments: split}}, 200, 2)
	if !r.Valid || len(r.FinishCDF) != 21 || r.FinishCDF[8].Probability != 0.4 || r.FinishCDF[8].Day != 0 {
		t.Errorf("zero-length samples: finish curve %+v (valid %v, %s), want day 0 at 40%%", r.FinishCDF, r.Valid, r.Error)
	}
}

// With no estimates, every Monte Carlo run is the schedule: on random
// levelled networks with splits, ALAP tasks, constraints and part days,
// the simulated finish equals the saved schedule's latest finish, split
// days rebuilt (the guide: "with every duration at its estimate the
// simulation finishes when the schedule does").
func TestMonteCarloFinishesWithTheSavedSchedule(t *testing.T) {
	rng := rand.New(rand.NewPCG(20261124, 3))
	splits := 0
	for i := range 2000 {
		input := map[string]*Task{}
		for k := range 4 + rng.IntN(10) {
			task := &Task{ID: fmt.Sprintf("T%02d", k), Duration: float64(1 + rng.IntN(3))}
			if rng.IntN(2) == 0 {
				task.Duration = float64(1+rng.IntN(24)) / 8
			}
			if k > 0 && rng.IntN(2) == 0 {
				task.Links = []Link{{Pred: fmt.Sprintf("T%02d", rng.IntN(k)), Type: pickFrom(rng, FinishToStart, StartToStart, FinishToFinish)}}
			}
			if rng.IntN(6) == 0 {
				task.Constraint = AsLateAsPossible
			}
			for range rng.IntN(3) {
				task.Assignments = append(task.Assignments, Assignment{Resource: pickFrom(rng, "r0", "r1"), Units: pickFrom(rng, 0.5, 1)})
			}
			input[task.ID] = task
		}
		levelled := cloneTaskMap(input)
		if _, err := LevelResourcesWithOptions(levelled, capacityPlanFromMap(nil), LevelingOptions{Horizon: 40, AllowSplitting: true}); err != nil && !errors.Is(err, ErrLevelingHorizonExceeded) {
			t.Fatalf("case %d: %v", i, err)
		}
		keep, _ := LevelStartsToKeep(levelled)
		saved := cloneTaskMap(input)
		for id, task := range saved {
			if day, ok := keep[id]; ok {
				task.LevelStartArmed, task.LevelStartDay = true, day
			}
			if wd := levelled[id].WorkDays; len(wd) > 0 {
				splits++
				for _, d := range wd {
					task.PlannedWorkSegments = append(task.PlannedWorkSegments, WorkSegment{Start: float64(d - wd[0]), End: float64(d - wd[0] + 1)})
				}
			}
		}
		simulated := RunMonteCarlo(cloneTaskMap(saved), 3, 1)
		if err := ScheduleCPM(saved); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		RebuildSplitDays(saved)
		finish := 0.0
		for _, task := range saved {
			finish = math.Max(finish, task.EF)
		}
		if !simulated.Valid || !sameOffset(simulated.P50, finish) {
			t.Fatalf("case %d: simulated finish %v (%s), schedule %v", i, simulated.P50, simulated.Error, finish)
		}
	}
	if splits < 50 {
		t.Errorf("generator split %d tasks, want at least 50", splits)
	}
}
