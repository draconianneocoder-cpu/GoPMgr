// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"
)

// Baseline benchmarks for the overallocation detector (design PS04-RES-002,
// RES-A08). They time the unchanged detector so a faster one can be
// compared on the same machine and fixture. The fixture is this file's own
// seeded synthetic schedule, not the ADR-010 S0 generator, so these
// numbers are not comparable with S0's.
//
//	go test -run '^$' -bench 'ResourceUsage|DetectOverallocations' -benchmem ./internal/kernel/
//
// The part-day variant (2026-11-23, D-TIME) starts and sizes tasks in
// eighths of a day, so most resource-days take the time-interval sweep
// rather than the whole-day path.

var overallocationBenchTiers = []struct {
	name  string
	tasks int
}{
	{"T0_500", 500},
	{"T1_10000", 10_000},
}

// benchmarkOverallocationSchedule spreads n tasks of 1 to 15 days over a
// 400-day horizon, one resource per 40 tasks, so resource-days carry about
// one task each on average and a share of them are overallocated at every
// tier. 15% of tasks have a second assignment.
func benchmarkOverallocationSchedule(n int) (map[string]*Task, ResourceCapacityPlan) {
	return benchmarkSchedule(n, 1)
}

// benchmarkPartDaySchedule is the same schedule with starts and durations
// in eighths of a day (an eighth to 15 days).
func benchmarkPartDaySchedule(n int) (map[string]*Task, ResourceCapacityPlan) {
	return benchmarkSchedule(n, 8)
}

func benchmarkSchedule(n, perDay int) (map[string]*Task, ResourceCapacityPlan) {
	const horizon = 400
	rng := rand.New(rand.NewPCG(20261027, uint64(n)))
	resources := max(4, n/40)
	tasks := make(map[string]*Task, n)
	step := float64(perDay)
	for i := range n {
		duration := float64(1+rng.IntN(15*perDay)) / step
		es := float64(rng.IntN((horizon-15)*perDay)) / step
		task := &Task{ID: fmt.Sprintf("T%06d", i), Duration: duration, ES: es, EF: es + duration}
		task.Assignments = []Assignment{{Resource: fmt.Sprintf("R%04d", rng.IntN(resources)), Units: pickFrom(rng, 0.5, 1)}}
		if rng.IntN(100) < 15 {
			task.Assignments = append(task.Assignments,
				Assignment{Resource: fmt.Sprintf("R%04d", rng.IntN(resources)), Units: pickFrom(rng, 0.5, 1)})
		}
		tasks[task.ID] = task
	}
	return tasks, ResourceCapacityPlan{DefaultCapacity: 1}
}

func BenchmarkResourceUsage(b *testing.B) {
	for _, tier := range overallocationBenchTiers {
		b.Run(tier.name, func(b *testing.B) {
			tasks, _ := benchmarkOverallocationSchedule(tier.tasks)
			b.ReportAllocs()
			for b.Loop() {
				ResourceUsage(tasks)
			}
		})
	}
}

func BenchmarkDetectOverallocationsWithPlan(b *testing.B) {
	benchmarkDetect(b, benchmarkOverallocationSchedule)
}

func BenchmarkDetectOverallocationsPartDay(b *testing.B) {
	benchmarkDetect(b, benchmarkPartDaySchedule)
}

func benchmarkDetect(b *testing.B, schedule func(int) (map[string]*Task, ResourceCapacityPlan)) {
	for _, tier := range overallocationBenchTiers {
		b.Run(tier.name, func(b *testing.B) {
			tasks, plan := schedule(tier.tasks)
			records := DetectOverallocationsWithPlan(tasks, plan)
			if len(records) == 0 {
				b.Fatal("the benchmark schedule has no overallocations, so it would not time the participant scan")
			}
			refs := 0
			for _, rec := range records {
				refs += len(rec.TaskIDs)
			}
			b.ReportAllocs()
			for b.Loop() {
				DetectOverallocationsWithPlan(tasks, plan)
			}
			b.ReportMetric(float64(len(records)), "breaches/op")
			b.ReportMetric(float64(refs), "task-refs/op")
		})
	}
}

// BenchmarkLevelResources levels the same fixtures (2026-11-23): the
// whole-day one times the whole-day totals path, the part-day one the
// part-day booking lists and booking-end candidates. Every task starts at
// 0 (no links), so each resource's tasks are serialised. Before 2026-11-23
// the leveller checked a task's assignments to one resource separately and
// "placed" the few that need more than the resource has; they are now
// unplaced and search the whole horizon, which this benchmark includes.
func BenchmarkLevelResources(b *testing.B) {
	for _, fixture := range []struct {
		name     string
		schedule func(int) (map[string]*Task, ResourceCapacityPlan)
	}{
		{"whole_day", benchmarkOverallocationSchedule},
		{"part_day", benchmarkPartDaySchedule},
	} {
		for _, tier := range overallocationBenchTiers {
			b.Run(fixture.name+"/"+tier.name, func(b *testing.B) {
				tasks, plan := fixture.schedule(tier.tasks)
				b.ReportAllocs()
				for b.Loop() {
					work := cloneTaskMap(tasks)
					// A few fixture tasks assign one resource twice for more
					// than it has, so they are reported unplaced.
					if _, err := LevelResourcesWithOptions(work, plan, LevelingOptions{}); err != nil && !errors.Is(err, ErrLevelingHorizonExceeded) {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
