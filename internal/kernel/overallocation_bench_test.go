// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
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
	const horizon = 400
	rng := rand.New(rand.NewPCG(20261027, uint64(n)))
	resources := max(4, n/40)
	tasks := make(map[string]*Task, n)
	for i := range n {
		duration := float64(1 + rng.IntN(15))
		es := float64(rng.IntN(horizon - 15))
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
	for _, tier := range overallocationBenchTiers {
		b.Run(tier.name, func(b *testing.B) {
			tasks, plan := benchmarkOverallocationSchedule(tier.tasks)
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
