// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

// equivalenceNetwork builds a seeded random network that exercises every
// rule the schedule pass applies: all four link types with positive and
// negative lags, an unknown link type (treated as FS), plain precedents,
// a predecessor listed both as a link and a precedent, links to missing
// tasks, duplicate links, every constraint armed and unarmed, ALAP tasks,
// zero-length milestones, out-of-range progress and three-point estimates.
// Links only point at earlier tasks, so the network is acyclic.
func equivalenceNetwork(seed uint64, n int) map[string]*Task {
	r := rand.New(rand.NewPCG(seed, seed^0x5eed))
	linkTypes := []LinkType{FinishToStart, StartToStart, FinishToFinish, StartToFinish, "XX"}
	constraints := []ConstraintType{"", AsSoonAsPossible, AsLateAsPossible, StartNoEarlierThan, FinishNoLaterThan, MustFinishOn}
	tasks := make(map[string]*Task, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("t%03d", i)
		task := &Task{ID: id, Duration: float64(r.IntN(9))}
		if r.IntN(8) == 0 {
			task.Duration = 0
			task.Milestone = true
		}
		for k := r.IntN(4); k > 0 && i > 0; k-- {
			pred := fmt.Sprintf("t%03d", r.IntN(i))
			task.Links = append(task.Links, Link{Pred: pred, Type: linkTypes[r.IntN(len(linkTypes))], Lag: float64(r.IntN(7) - 3)})
		}
		if i > 0 && r.IntN(3) == 0 {
			task.Precedents = append(task.Precedents, fmt.Sprintf("t%03d", r.IntN(i)))
		}
		if len(task.Links) > 0 && r.IntN(4) == 0 {
			task.Precedents = append(task.Precedents, task.Links[0].Pred) // listed both ways
			task.Links = append(task.Links, task.Links[0])                // and duplicated
		}
		if r.IntN(10) == 0 {
			task.Links = append(task.Links, Link{Pred: "missing", Type: FinishToStart})
			task.Precedents = append(task.Precedents, "also-missing")
		}
		task.Constraint = constraints[r.IntN(len(constraints))]
		task.ConstraintArmed = r.IntN(2) == 0
		task.ConstraintDay = float64(r.IntN(40))
		task.PercentComplete = float64(r.IntN(140) - 20)
		if r.IntN(2) == 0 {
			lo := float64(r.IntN(3))
			task.DurationEstimate = DurationEstimate{
				Optimistic:   lo,
				MostLikely:   lo + float64(r.IntN(4)),
				Pessimistic:  lo + 4 + float64(r.IntN(6)),
				Distribution: []DistributionName{"", MonteCarloTriangular, MonteCarloBetaPERT, MonteCarloNormal}[r.IntN(4)],
			}
		}
		tasks[id] = task
	}
	return tasks
}

func cloneNetwork(tasks map[string]*Task) map[string]*Task {
	out := make(map[string]*Task, len(tasks))
	for id, t := range tasks {
		out[id] = cloneTaskForSimulation(t)
	}
	return out
}

func TestCalculateCPMMatchesReference(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 300; seed++ {
		n := 1 + int(seed%60)
		want := equivalenceNetwork(seed, n)
		got := cloneNetwork(want)
		wantOK := referenceCalculateCPM(want)
		gotOK := CalculateCPM(got)
		if gotOK != wantOK {
			t.Fatalf("seed %d: CalculateCPM = %v, reference %v", seed, gotOK, wantOK)
		}
		if !reflect.DeepEqual(got, want) {
			for id := range want {
				if !reflect.DeepEqual(got[id], want[id]) {
					t.Fatalf("seed %d task %s:\n got  %+v\n want %+v", seed, id, *got[id], *want[id])
				}
			}
		}
	}
}

func TestCalculateCPMLeavesACycleUntouched(t *testing.T) {
	t.Parallel()
	tasks := map[string]*Task{
		"a": {ID: "a", Duration: 2, Precedents: []string{"b"}, ES: 7, EF: 9},
		"b": {ID: "b", Duration: 3, Precedents: []string{"a"}, ES: 1, EF: 4},
	}
	before := cloneNetwork(tasks)
	if CalculateCPM(tasks) {
		t.Fatal("CalculateCPM accepted a cycle")
	}
	if !reflect.DeepEqual(tasks, before) {
		t.Fatal("CalculateCPM changed tasks in a cyclic network")
	}
}

func TestRunMonteCarloMatchesReference(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		seed       uint64
		tasks      int
		iterations int
	}{{11, 1, 50}, {12, 25, 400}, {13, 60, 300}, {14, 40, 1}} {
		network := equivalenceNetwork(tc.seed, tc.tasks)
		want := referenceRunMonteCarlo(cloneNetwork(network), tc.iterations, 1)
		if !want.Valid || want.P90 == 0 {
			t.Fatalf("seed %d: the reference simulation did not run (valid %v, error %q, P90 %v)", tc.seed, want.Valid, want.Error, want.P90)
		}
		for _, workers := range []int{1, 3, 0} {
			got := RunMonteCarlo(cloneNetwork(network), tc.iterations, workers)
			got.Workers = want.Workers // the worker count is reported, not a result
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("seed %d, %d workers: RunMonteCarlo differs from the reference\n got  %+v\n want %+v", tc.seed, workers, got, want)
			}
		}
	}
}

func TestRunMonteCarloLeavesInputTasksUnchanged(t *testing.T) {
	t.Parallel()
	network := equivalenceNetwork(21, 30)
	before := cloneNetwork(network)
	RunMonteCarlo(network, 200, 4)
	if !reflect.DeepEqual(network, before) {
		t.Fatal("RunMonteCarlo changed the caller's tasks")
	}
}
