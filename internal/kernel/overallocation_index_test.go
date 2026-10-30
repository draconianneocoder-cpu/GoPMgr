// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"reflect"
	"testing"
)

// Structural tests for the participant index (PS-04 RES-A-1). The hand
// oracle and the differential test prove the detector's output; these prove
// the index stays proportional to runs of days, not to days, and that split
// days become non-overlapping, non-adjacent runs.

func TestOccupiedRuns(t *testing.T) {
	cases := []struct {
		name  string
		task  *Task
		limit int
		want  [][2]int
	}{
		{"a million-day span is one run",
			&Task{Duration: 1_000_000, ES: 0, EF: 1_000_000}, 1_000_000, [][2]int{{0, 1_000_000}}},
		{"a span is clipped to the profile",
			&Task{Duration: 50, ES: 5, EF: 55}, 10, [][2]int{{5, 10}}},
		{"a span wholly past the profile has no run",
			&Task{Duration: 1, ES: 20, EF: 21}, 10, nil},
		{"a zero-duration task without split days has no run",
			&Task{Duration: 0, ES: 0, EF: 0}, 10, nil},
		{"split days are sorted, deduplicated, merged when adjacent and clipped",
			&Task{Duration: 3, ES: 0, EF: 3, WorkDays: []int{5, 3, 3, 4, 9, -1, 12}}, 10, [][2]int{{3, 6}, {9, 10}}},
		{"split days of a zero-duration task still count",
			&Task{Duration: 0, WorkDays: []int{2}}, 10, [][2]int{{2, 3}}},
		{"an empty, non-nil WorkDays list means the span",
			&Task{Duration: 2, ES: 1, EF: 3, WorkDays: []int{}}, 10, [][2]int{{1, 3}}},
		{"a zero-length profile has no run",
			&Task{Duration: 0, WorkDays: []int{0}}, 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := occupiedRuns(c.task, c.limit); !reflect.DeepEqual(got, c.want) {
				t.Errorf("occupiedRuns = %v, want %v", got, c.want)
			}
		})
	}
}

func TestOverallocationMembershipRunsArePerEntryAndResource(t *testing.T) {
	shared := &Task{ID: "S", Duration: 2, ES: 0, EF: 2, Assignments: []Assignment{{Resource: "r"}}}
	tasks := map[string]*Task{
		// Two assignments to r and one to s: one run each, not one per assignment.
		"a": {ID: "A", Duration: 3, ES: 0, EF: 3, Assignments: []Assignment{{Resource: "r"}, {Resource: "r"}, {Resource: "s"}, {Resource: ""}}},
		// Two keys aliasing one task: two entries, so two runs.
		"k1": shared,
		"k2": shared,
		// Assigned only to a resource with no profile: no runs anywhere.
		"z": {ID: "Z", Duration: 0, ES: 0, EF: 0, Assignments: []Assignment{{Resource: "q"}}},
	}
	usage := ResourceUsage(tasks)
	entries := overallocationEntries(tasks)
	runs := overallocationMembershipRuns(entries, usage)

	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.key
	}
	if want := []string{"a", "k1", "k2", "z"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("entry order = %v, want %v (printed ID, then key)", keys, want)
	}
	want := map[string][]overallocationRun{
		"r": {{start: 0, end: 3, entry: 0}, {start: 0, end: 2, entry: 1}, {start: 0, end: 2, entry: 2}},
		"s": {{start: 0, end: 3, entry: 0}},
	}
	if !reflect.DeepEqual(runs, want) {
		t.Errorf("runs = %+v, want %+v", runs, want)
	}
}

// BenchmarkDetectOverallocationsWithPlanLarge covers the T2 and T3 tiers,
// which the participant scan this index replaced made impractical to run
// routinely. Same fixture generator as overallocation_bench_test.go.
func BenchmarkDetectOverallocationsWithPlanLarge(b *testing.B) {
	for _, tier := range []struct {
		name  string
		tasks int
	}{{"T2_50000", 50_000}, {"T3_100000", 100_000}} {
		b.Run(tier.name, func(b *testing.B) {
			tasks, plan := benchmarkOverallocationSchedule(tier.tasks)
			records := DetectOverallocationsWithPlan(tasks, plan)
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
