// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"fmt"
	"testing"
)

// benchmarkMonteCarloSchedule builds a layered network of n tasks: each
// task after the first ten depends on two tasks from earlier layers, and
// every task has a three-point estimate, so every iteration samples and
// schedules the whole network.
func benchmarkMonteCarloSchedule(n int) map[string]*Task {
	tasks := make(map[string]*Task, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("T%06d", i)
		task := &Task{
			ID:       id,
			Duration: 5,
			DurationEstimate: DurationEstimate{
				Optimistic:   3,
				MostLikely:   5,
				Pessimistic:  float64(8 + i%7),
				Distribution: []DistributionName{MonteCarloTriangular, MonteCarloBetaPERT}[i%2],
			},
		}
		if i >= 10 {
			task.Precedents = []string{fmt.Sprintf("T%06d", i-10), fmt.Sprintf("T%06d", (i*7)%(i-9))}
		}
		tasks[id] = task
	}
	return tasks
}

func BenchmarkRunMonteCarlo(b *testing.B) {
	for _, size := range []struct{ tasks, iterations int }{{100, 1000}, {500, 1000}, {500, 10000}} {
		tasks := benchmarkMonteCarloSchedule(size.tasks)
		b.Run(fmt.Sprintf("tasks=%d/iterations=%d", size.tasks, size.iterations), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if r := RunMonteCarlo(tasks, size.iterations, 0); !r.Valid {
					b.Fatalf("simulation invalid: %s", r.Error)
				}
			}
		})
	}
}

// BenchmarkCalculateCPM measures one schedule pass, which every Gantt or
// CPM edit runs. Durations are reset each loop so every pass does the
// same work.
func BenchmarkCalculateCPM(b *testing.B) {
	for _, n := range []int{500, 5000} {
		tasks := benchmarkMonteCarloSchedule(n)
		b.Run(fmt.Sprintf("tasks=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !CalculateCPM(tasks) {
					b.Fatal("cycle")
				}
			}
		})
	}
}
