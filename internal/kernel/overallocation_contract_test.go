// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Hand oracle for the overallocation detector (design PS04-RES-002, oracle
// OR-01). Every expected value below was worked out by hand from the
// contract in docs/testing/resource-overallocation-contract.md, not
// captured from a detector run. Fixture IDs F-xx are the design's; OR-01-Sx
// are supplementary cases this suite adds. Each fixture runs against both
// the production detector and the frozen legacy copy, so a broken copy
// cannot pass the differential test unnoticed.

// The public detector surface must keep these exact signatures (RES-C01).
var (
	_ func(map[string]*Task) map[string][]float64                    = ResourceUsage
	_ func(ResourceCapacityPlan, []string, int) map[string][]float64 = ResourceCapacityProfiles
	_ func(map[string]*Task, map[string]float64) []Overallocation    = DetectOverallocations
	_ func(map[string]*Task, ResourceCapacityPlan) []Overallocation  = DetectOverallocationsWithPlan
)

type overallocationDetector struct {
	name   string
	detect func(map[string]*Task, ResourceCapacityPlan) []Overallocation
	usage  func(map[string]*Task) map[string][]float64
}

func overallocationDetectors() []overallocationDetector {
	return []overallocationDetector{
		{"production", DetectOverallocationsWithPlan, ResourceUsage},
		{"legacy", legacyDetectOverallocationsWithPlan, legacyResourceUsage},
	}
}

// resTask builds an already-scheduled task: ES and EF are set directly, so
// the fixture states its occupied days without running CPM.
func resTask(id string, duration, es, ef float64, assignments ...Assignment) *Task {
	return &Task{ID: id, Duration: duration, ES: es, EF: ef, Assignments: assignments}
}

func resAssign(resource string, units float64) Assignment {
	return Assignment{Resource: resource, Units: units}
}

func withWorkDays(t *Task, days ...int) *Task {
	t.WorkDays = days
	return t
}

// sameFloat compares binary64 values exactly, bit for bit, except that any
// NaN matches any NaN.
func sameFloat(got, want float64) bool {
	if math.IsNaN(want) {
		return math.IsNaN(got)
	}
	return math.Float64bits(got) == math.Float64bits(want)
}

func formatOverallocation(o Overallocation) string {
	return fmt.Sprintf("{%q day %d demand %v (%#x) capacity %v (%#x) task_ids %q}",
		o.Resource, o.Day, o.Demand, math.Float64bits(o.Demand), o.Capacity, math.Float64bits(o.Capacity), o.TaskIDs)
}

func formatOverallocations(list []Overallocation) string {
	if list == nil {
		return "nil"
	}
	parts := make([]string, len(list))
	for i, o := range list {
		parts[i] = formatOverallocation(o)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// overallocationDiff returns "" when got matches want exactly: nil-ness,
// order, every field, and Demand and Capacity to the bit.
func overallocationDiff(got, want []Overallocation) string {
	if (got == nil) != (want == nil) || len(got) != len(want) {
		return fmt.Sprintf("\n got:  %s\n want: %s", formatOverallocations(got), formatOverallocations(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Resource != w.Resource || g.Day != w.Day || !sameFloat(g.Demand, w.Demand) ||
			!sameFloat(g.Capacity, w.Capacity) || !reflect.DeepEqual(g.TaskIDs, w.TaskIDs) {
			return fmt.Sprintf("record %d\n got:  %s\n want: %s", i, formatOverallocation(g), formatOverallocation(w))
		}
	}
	return ""
}

// usageDiff returns "" when two ResourceUsage results have the same
// resources, profile lengths and per-day demand (bit for bit).
func usageDiff(got, want map[string][]float64) string {
	if !reflect.DeepEqual(sortedUsageKeys(got), sortedUsageKeys(want)) {
		return fmt.Sprintf("resources: got %q, want %q", sortedUsageKeys(got), sortedUsageKeys(want))
	}
	for r, w := range want {
		g := got[r]
		if len(g) != len(w) {
			return fmt.Sprintf("%q profile length: got %d, want %d", r, len(g), len(w))
		}
		for day := range w {
			if !sameFloat(g[day], w[day]) {
				return fmt.Sprintf("%q day %d: got %v (%#x), want %v (%#x)",
					r, day, g[day], math.Float64bits(g[day]), w[day], math.Float64bits(w[day]))
			}
		}
	}
	return ""
}

func sortedUsageKeys(usage map[string][]float64) []string {
	keys := make([]string, 0, len(usage))
	for k := range usage {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// detectorInputSnapshot renders everything detection reads, with the one
// field it may write (Task.Overallocated) blanked, so a test can prove the
// call changed nothing else.
func detectorInputSnapshot(tasks map[string]*Task, plan ResourceCapacityPlan) string {
	keys := make([]string, 0, len(tasks))
	for k := range tasks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		task := *tasks[k]
		task.Overallocated = false
		fmt.Fprintf(&b, "%s=%#v\n", k, task)
	}
	fmt.Fprintf(&b, "plan=%#v", plan)
	return b.String()
}

// sameInEveryOrder sums each order left to right in binary64, the way
// ResourceUsage accumulates a day, and returns the sum. It fails the test
// if the orders disagree, because then the fixture needs a set of accepted
// values rather than one.
func sameInEveryOrder(t *testing.T, fixture string, orders ...[]float64) float64 {
	t.Helper()
	var sums []float64
	for _, order := range orders {
		sum := 0.0
		for _, v := range order {
			sum += v
		}
		sums = append(sums, sum)
	}
	for _, s := range sums[1:] {
		if math.Float64bits(s) != math.Float64bits(sums[0]) {
			t.Fatalf("%s: summation orders give different binary64 sums %v; the fixture needs a set oracle", fixture, sums)
		}
	}
	return sums[0]
}

type overallocationFixture struct {
	id    string
	build func() map[string]*Task
	plan  ResourceCapacityPlan
	want  []Overallocation
	flags map[string]bool      // expected Task.Overallocated for every map key
	usage map[string][]float64 // expected ResourceUsage, exactly
}

func overallocationFixtures(t *testing.T) []overallocationFixture {
	unit := ResourceCapacityPlan{DefaultCapacity: 1}
	calendar := func(resource string, cal ResourceCalendar) ResourceCapacityPlan {
		return ResourceCapacityPlan{DefaultCapacity: 1, Calendars: map[string]ResourceCalendar{resource: cal}}
	}

	// F-02: map order decides only whether A's two assignments or B's one is
	// added first; A's own assignments are always added in slice order.
	f02 := sameInEveryOrder(t, "F-02", []float64{0.4, 0.5, 0.3}, []float64{0.3, 0.4, 0.5})
	s5 := sameInEveryOrder(t, "OR-01-S5", []float64{0.1, 0.2}, []float64{0.2, 0.1})

	return []overallocationFixture{
		{
			id: "F-01 ordinary overlap",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": resTask("A", 2, 0, 2, resAssign("alice", 1)),
					"B": resTask("B", 1, 0, 1, resAssign("alice", 1)),
					"C": resTask("C", 1, 0, 1, resAssign("bob", 1)),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true, "C": false},
			usage: map[string][]float64{"alice": {2, 1}, "bob": {1, 0}},
		},
		{
			id: "F-02 repeated assignments add demand, not participants",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": resTask("A", 1, 0, 1, resAssign("alice", 0.4), resAssign("alice", 0.5)),
					"B": resTask("B", 1, 0, 1, resAssign("alice", 0.3)),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: f02, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"alice": {f02}},
		},
		{
			id: "F-03 capped and defaulted units",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": resTask("A", 1, 0, 1, Assignment{Resource: "alice", Units: 1.5, MaxUnits: 0.75}),
					"B": resTask("B", 1, 0, 1, Assignment{Resource: "alice"}),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: 1.75, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"alice": {1.75}},
		},
		{
			id: "F-03 negative units count as 1 and a cap applies to defaulted units",
			build: func() map[string]*Task {
				return map[string]*Task{
					"C": resTask("C", 1, 0, 1, Assignment{Resource: "alice", Units: -2}),
					"D": resTask("D", 1, 0, 1, Assignment{Resource: "alice", MaxUnits: 0.5}),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: 1.5, Capacity: 1, TaskIDs: []string{"C", "D"}}},
			flags: map[string]bool{"C": true, "D": true},
			usage: map[string][]float64{"alice": {1.5}},
		},
		{
			id: "F-04 split work with gaps",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": withWorkDays(resTask("A", 2, 0, 3, resAssign("alice", 1)), 0, 2),
					"B": withWorkDays(resTask("B", 2, 0, 3, resAssign("alice", 1)), 1, 2),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 2, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"alice": {1, 1, 2}},
		},
		{
			// A's split days are deliberately unsorted: detection must not
			// reorder them.
			id: "OR-01-S1 a split gap is not participation",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": withWorkDays(resTask("A", 2, 0, 3, resAssign("alice", 1)), 2, 0),
					"B": resTask("B", 1, 1, 2, resAssign("alice", 1)),
					"C": resTask("C", 1, 1, 2, resAssign("alice", 1)),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 1, Demand: 2, Capacity: 1, TaskIDs: []string{"B", "C"}}},
			flags: map[string]bool{"A": false, "B": true, "C": true},
			usage: map[string][]float64{"alice": {1, 2, 1}},
		},
		{
			id: "F-05 zero-capacity override",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 2, 0, 2, resAssign("alice", 1))}
			},
			plan:  calendar("alice", ResourceCalendar{Overrides: map[int]float64{1: 0}}),
			want:  []Overallocation{{Resource: "alice", Day: 1, Demand: 1, Capacity: 0, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"alice": {1, 1}},
		},
		{
			id: "F-06 explicit override beats the weekly phase",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 3, 0, 3, resAssign("alice", 0.75))}
			},
			plan: ResourceCapacityPlan{
				DefaultCapacity: 1,
				Capacities:      map[string]float64{"alice": 1},
				Calendars: map[string]ResourceCalendar{"alice": {
					WeeklyCapacity: map[int]float64{1: 0.5},
					Overrides:      map[int]float64{1: 0},
				}},
			},
			want:  []Overallocation{{Resource: "alice", Day: 1, Demand: 0.75, Capacity: 0, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"alice": {0.75, 0.75, 0.75}},
		},
		{
			id: "F-06 weekly phase is the project-day offset mod 7",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 3, 0, 3, resAssign("alice", 0.75))}
			},
			plan: ResourceCapacityPlan{
				DefaultCapacity: 1,
				Capacities:      map[string]float64{"alice": 1},
				Calendars: map[string]ResourceCalendar{"alice": {
					WeeklyCapacity: map[int]float64{1: 0.5},
				}},
			},
			want:  []Overallocation{{Resource: "alice", Day: 1, Demand: 0.75, Capacity: 0.5, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"alice": {0.75, 0.75, 0.75}},
		},
		{
			id: "F-08 resource-major ordering",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": resTask("A", 1, 0, 1, resAssign("zebra", 1), resAssign("alpha", 1)),
					"B": resTask("B", 1, 0, 1, resAssign("zebra", 1), resAssign("alpha", 1)),
					"C": resTask("C", 1, 1, 2, resAssign("alpha", 2)),
				}
			},
			plan: unit,
			want: []Overallocation{
				{Resource: "alpha", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}},
				{Resource: "alpha", Day: 1, Demand: 2, Capacity: 1, TaskIDs: []string{"C"}},
				{Resource: "zebra", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}},
			},
			flags: map[string]bool{"A": true, "B": true, "C": true},
			usage: map[string][]float64{"alpha": {2, 2}, "zebra": {2, 0}},
		},
		{
			id: "F-09 detection ignores the assignment calendar",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 1, 0, 1,
					Assignment{Resource: "crew", Units: 1, CalendarID: "winter"})}
			},
			plan:  calendar("winter", ResourceCalendar{Overrides: map[int]float64{0: 0}}),
			flags: map[string]bool{"A": false},
			usage: map[string][]float64{"crew": {1}},
		},
		{
			id: "F-10 duplicate split day doubles demand, not participants",
			build: func() map[string]*Task {
				return map[string]*Task{"A": withWorkDays(resTask("A", 2, 0, 3, resAssign("alice", 1)), 0, 0, 2)}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"alice": {2, 0, 1}},
		},
		{
			id: "F-11 split day past the span horizon is dropped",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": withWorkDays(resTask("A", 1, 0, 1, resAssign("alice", 1)), 0, 10),
					"B": withWorkDays(resTask("B", 1, 0, 1, resAssign("alice", 1)), 0, 10),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"alice": {2}},
		},
		{
			id: "F-15 duplicate printed IDs and a blank resource",
			build: func() map[string]*Task {
				return map[string]*Task{
					"k1": resTask("A", 1, 0, 1, resAssign("r", 1)),
					"k2": resTask("A", 1, 0, 1, resAssign("r", 1)),
					"k3": resTask("B", 1, 0, 1, resAssign("", 2)),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "r", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "A"}}},
			flags: map[string]bool{"k1": true, "k2": true, "k3": false},
			usage: map[string][]float64{"r": {2}},
		},
		{
			id:    "F-16 empty task map",
			build: func() map[string]*Task { return map[string]*Task{} },
			plan:  unit,
			flags: map[string]bool{},
			usage: map[string][]float64{},
		},
		{
			id: "F-16 zero duration and no assignments add no demand",
			build: func() map[string]*Task {
				return map[string]*Task{
					"Z": resTask("Z", 0, 0, 0, resAssign("r", 1)),
					"N": resTask("N", 1, 0, 1),
				}
			},
			plan:  unit,
			flags: map[string]bool{"Z": false, "N": false},
			usage: map[string][]float64{},
		},
		{
			id: "F-16 zero-duration split task alone has an empty profile",
			build: func() map[string]*Task {
				return map[string]*Task{"Z": withWorkDays(resTask("Z", 0, 0, 0, resAssign("r", 1)), 0)}
			},
			plan:  unit,
			flags: map[string]bool{"Z": false},
			usage: map[string][]float64{"r": {}},
		},
		{
			id: "F-16 zero-duration split task inside a peer's horizon",
			build: func() map[string]*Task {
				return map[string]*Task{
					"Z": withWorkDays(resTask("Z", 0, 0, 0, resAssign("r", 1)), 0),
					"B": resTask("B", 1, 0, 1, resAssign("r", 1)),
				}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "r", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"B", "Z"}}},
			flags: map[string]bool{"Z": true, "B": true},
			usage: map[string][]float64{"r": {2}},
		},
		{
			// An empty but non-nil WorkDays, as JSON "work_days":[] decodes,
			// means the contiguous span, the same as nil.
			id: "F-16 an empty WorkDays list means the contiguous span",
			build: func() map[string]*Task {
				a := resTask("A", 2, 0, 2, resAssign("alice", 1))
				a.WorkDays = []int{}
				return map[string]*Task{"A": a, "B": resTask("B", 1, 1, 2, resAssign("alice", 1))}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "alice", Day: 1, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}}},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"alice": {1, 2}},
		},
		{
			id:    "F-16 nil task map",
			build: func() map[string]*Task { return nil },
			plan:  unit,
			flags: map[string]bool{},
			usage: map[string][]float64{},
		},
		{
			// Each day's demand is summed from zero. Day 2 is exactly 0.2;
			// a running sum that adds 0.1 and 0.2 and later subtracts 0.1
			// gives 0.20000000000000004 instead.
			id: "OR-01-S5 demand is summed per day, not carried across days",
			build: func() map[string]*Task {
				return map[string]*Task{
					"A": resTask("A", 2, 0, 2, resAssign("r", 0.1)),
					"B": resTask("B", 3, 0, 3, resAssign("r", 0.2)),
				}
			},
			plan: ResourceCapacityPlan{DefaultCapacity: 1, Capacities: map[string]float64{"r": 0.15}},
			want: []Overallocation{
				{Resource: "r", Day: 0, Demand: s5, Capacity: 0.15, TaskIDs: []string{"A", "B"}},
				{Resource: "r", Day: 1, Demand: s5, Capacity: 0.15, TaskIDs: []string{"A", "B"}},
				{Resource: "r", Day: 2, Demand: 0.2, Capacity: 0.15, TaskIDs: []string{"B"}},
			},
			flags: map[string]bool{"A": true, "B": true},
			usage: map[string][]float64{"r": {s5, s5, 0.2}},
		},
		{
			id: "F-18 NaN demand passes the literal comparison",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 1, 0, 1, resAssign("r", math.NaN()))}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "r", Day: 0, Demand: math.NaN(), Capacity: 1, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"r": {math.NaN()}},
		},
		{
			id: "F-19 two map keys alias one task",
			build: func() map[string]*Task {
				p := resTask("A", 1, 0, 1, resAssign("r", 1))
				return map[string]*Task{"first": p, "second": p}
			},
			plan:  unit,
			want:  []Overallocation{{Resource: "r", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "A"}}},
			flags: map[string]bool{"first": true, "second": true},
			usage: map[string][]float64{"r": {2}},
		},
		{
			id: "F-20 fractional ES and EF occupy rounded days",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 0.76, 0.49, 1.25, resAssign("r", 0.75))}
			},
			plan: ResourceCapacityPlan{DefaultCapacity: 1, Capacities: map[string]float64{"r": 0.5}},
			want: []Overallocation{
				{Resource: "r", Day: 0, Demand: 0.75, Capacity: 0.5, TaskIDs: []string{"A"}},
				{Resource: "r", Day: 1, Demand: 0.75, Capacity: 0.5, TaskIDs: []string{"A"}},
			},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"r": {0.75, 0.75}},
		},
		{
			// round(0.5) is 1 (half away from zero), and ceil(0.9)-1 = 0 is
			// raised to the first day, so the task occupies day 1 only.
			id: "OR-01-S4 a half-day start rounds up",
			build: func() map[string]*Task {
				return map[string]*Task{"A": resTask("A", 0.4, 0.5, 0.9, resAssign("r", 1))}
			},
			plan:  ResourceCapacityPlan{DefaultCapacity: 1, Capacities: map[string]float64{"r": 0.5}},
			want:  []Overallocation{{Resource: "r", Day: 1, Demand: 1, Capacity: 0.5, TaskIDs: []string{"A"}}},
			flags: map[string]bool{"A": true},
			usage: map[string][]float64{"r": {0, 1}},
		},
		{
			id: "F-21 out-of-range split day leaves zero demand on a zero-capacity day",
			build: func() map[string]*Task {
				return map[string]*Task{"A": withWorkDays(resTask("A", 1, 0, 1, resAssign("r", 1)), -1)}
			},
			plan:  calendar("r", ResourceCalendar{Overrides: map[int]float64{0: 0}}),
			flags: map[string]bool{"A": false},
			usage: map[string][]float64{"r": {0}},
		},
		{
			// Lexical, not numeric: T10 sorts before T2. Nine participants
			// make an accidentally sorted map order vanishingly unlikely.
			id: "OR-01-S2 nine participants sorted by printed ID",
			build: func() map[string]*Task {
				tasks := map[string]*Task{}
				for _, id := range []string{"T8", "T2", "T10", "T5", "T1", "T7", "T3", "T6", "T4"} {
					tasks[id] = resTask(id, 1, 0, 1, resAssign("r", 1))
				}
				return tasks
			},
			plan: unit,
			want: []Overallocation{{Resource: "r", Day: 0, Demand: 9, Capacity: 1,
				TaskIDs: []string{"T1", "T10", "T2", "T3", "T4", "T5", "T6", "T7", "T8"}}},
			flags: map[string]bool{"T1": true, "T10": true, "T2": true, "T3": true, "T4": true,
				"T5": true, "T6": true, "T7": true, "T8": true},
			usage: map[string][]float64{"r": {9}},
		},
	}
}

func TestOverallocationHandOracle(t *testing.T) {
	for _, d := range overallocationDetectors() {
		for _, f := range overallocationFixtures(t) {
			t.Run(d.name+"/"+f.id, func(t *testing.T) {
				tasks := f.build()
				if len(f.flags) != len(tasks) {
					t.Fatalf("fixture lists flags for %d keys, task map has %d", len(f.flags), len(tasks))
				}
				// Stale flags from an earlier run must be cleared (RES-C03).
				for _, task := range tasks {
					task.Overallocated = true
				}
				before := detectorInputSnapshot(tasks, f.plan)

				got := d.detect(tasks, f.plan)

				if diff := overallocationDiff(got, f.want); diff != "" {
					t.Errorf("records differ:%s", diff)
				}
				if after := detectorInputSnapshot(tasks, f.plan); after != before {
					t.Errorf("detection changed more than Task.Overallocated:\nbefore:\n%s\nafter:\n%s", before, after)
				}
				for key, task := range tasks {
					want, ok := f.flags[key]
					if !ok {
						t.Fatalf("fixture has no expected flag for key %q", key)
					}
					if task.Overallocated != want {
						t.Errorf("tasks[%q].Overallocated = %v, want %v", key, task.Overallocated, want)
					}
				}
				if diff := usageDiff(d.usage(f.build()), f.usage); diff != "" {
					t.Errorf("ResourceUsage: %s", diff)
				}
			})
		}
	}
}

// TestOverallocationThresholdIsLiteral pins the comparison
// `demand <= capacity + 1e-9` (skip) at binary64 resolution (F-07, RES-A07).
// The threshold is computed the way the detector computes it: a runtime
// float64 addition.
func TestOverallocationThresholdIsLiteral(t *testing.T) {
	cases := []struct {
		name     string
		plan     ResourceCapacityPlan
		capacity float64
	}{
		{"default capacity 1", ResourceCapacityPlan{DefaultCapacity: 1}, 1},
		{"zero-capacity override", ResourceCapacityPlan{DefaultCapacity: 1, Calendars: map[string]ResourceCalendar{
			"r": {Overrides: map[int]float64{0: 0}}}}, 0},
		{"base capacity 0.5", ResourceCapacityPlan{DefaultCapacity: 1, Capacities: map[string]float64{"r": 0.5}}, 0.5},
	}
	for _, d := range overallocationDetectors() {
		for _, c := range cases {
			threshold := c.capacity + 1e-9
			points := []struct {
				name   string
				demand float64
				breach bool
			}{
				{"equal to the threshold", threshold, false},
				{"one ULP above", math.Nextafter(threshold, math.Inf(1)), true},
				{"one ULP below", math.Nextafter(threshold, math.Inf(-1)), false},
			}
			for _, p := range points {
				t.Run(fmt.Sprintf("%s/%s/%s", d.name, c.name, p.name), func(t *testing.T) {
					tasks := map[string]*Task{"A": resTask("A", 1, 0, 1, resAssign("r", p.demand))}
					got := d.detect(tasks, c.plan)
					var want []Overallocation
					if p.breach {
						want = []Overallocation{{Resource: "r", Day: 0, Demand: p.demand, Capacity: c.capacity, TaskIDs: []string{"A"}}}
					}
					if diff := overallocationDiff(got, want); diff != "" {
						t.Errorf("demand %#x against threshold %#x:%s",
							math.Float64bits(p.demand), math.Float64bits(threshold), diff)
					}
				})
			}
		}
	}
}

// TestOverallocationCapacityResolution makes every day breach (demand 100)
// so each record reports the capacity resolved for that day (OR-01-S3,
// RES-A06): plan default, then positive per-resource base, then positive
// calendar default, then a nonnegative weekly value for day mod 7, then a
// nonnegative override for that exact day. The detector looks the calendar
// up by resource name only.
func TestOverallocationCapacityResolution(t *testing.T) {
	cases := []struct {
		name string
		plan ResourceCapacityPlan
		want [16]float64
	}{
		{"zero plan default means 1", ResourceCapacityPlan{},
			[16]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
		{"positive plan default", ResourceCapacityPlan{DefaultCapacity: 2},
			[16]float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}},
		{"negative plan default and zero base mean 1",
			ResourceCapacityPlan{DefaultCapacity: -3, Capacities: map[string]float64{"r": 0}},
			[16]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
		{"negative base is ignored",
			ResourceCapacityPlan{DefaultCapacity: 1.5, Capacities: map[string]float64{"r": -1}},
			[16]float64{1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5, 1.5}},
		{"positive base",
			ResourceCapacityPlan{DefaultCapacity: 1, Capacities: map[string]float64{"r": 3}},
			[16]float64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}},
		{"calendar default replaces the base",
			ResourceCapacityPlan{Capacities: map[string]float64{"r": 3},
				Calendars: map[string]ResourceCalendar{"r": {DefaultCapacity: 4}}},
			[16]float64{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}},
		{"nonpositive calendar default is ignored",
			ResourceCapacityPlan{Capacities: map[string]float64{"r": 3},
				Calendars: map[string]ResourceCalendar{"r": {DefaultCapacity: -1}}},
			[16]float64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}},
		{"weekly phase is day mod 7 and a negative weekly value is ignored",
			ResourceCapacityPlan{DefaultCapacity: 2,
				Calendars: map[string]ResourceCalendar{"r": {WeeklyCapacity: map[int]float64{0: 0, 3: 0.5, 6: -2}}}},
			[16]float64{0, 2, 2, 0.5, 2, 2, 2, 0, 2, 2, 0.5, 2, 2, 2, 0, 2}},
		{"calendar default with a weekly zero",
			ResourceCapacityPlan{Calendars: map[string]ResourceCalendar{"r": {
				DefaultCapacity: 2, WeeklyCapacity: map[int]float64{5: 0}}}},
			[16]float64{2, 2, 2, 2, 2, 0, 2, 2, 2, 2, 2, 2, 0, 2, 2, 2}},
		{"override beats weekly and a negative override is ignored",
			ResourceCapacityPlan{DefaultCapacity: 1,
				Calendars: map[string]ResourceCalendar{"r": {
					WeeklyCapacity: map[int]float64{0: 0, 2: 0.75},
					Overrides:      map[int]float64{1: 0, 2: -1, 7: 0.25, 9: 5},
				}}},
			[16]float64{0, 0, 0.75, 1, 1, 1, 1, 0.25, 1, 5, 1, 1, 1, 1, 0, 1}},
		{"calendar under another resource's name is not used",
			ResourceCapacityPlan{DefaultCapacity: 1,
				Calendars: map[string]ResourceCalendar{"other": {DefaultCapacity: 7, Overrides: map[int]float64{0: 0}}}},
			[16]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}},
	}
	for _, d := range overallocationDetectors() {
		for _, c := range cases {
			t.Run(d.name+"/"+c.name, func(t *testing.T) {
				tasks := map[string]*Task{"P": resTask("P", 16, 0, 16, resAssign("r", 100))}
				want := make([]Overallocation, 0, len(c.want))
				for day, capacity := range c.want {
					want = append(want, Overallocation{Resource: "r", Day: day, Demand: 100, Capacity: capacity, TaskIDs: []string{"P"}})
				}
				if diff := overallocationDiff(d.detect(tasks, c.plan), want); diff != "" {
					t.Errorf("records differ:%s", diff)
				}
			})
		}
	}
}

// TestOverallocationUnassignedTaskSetsTheHorizon pins F-17 (RES-A09, RES-C10):
// the dense profile spans every task's days, including a far task that uses
// no resource.
func TestOverallocationUnassignedTaskSetsTheHorizon(t *testing.T) {
	for _, d := range overallocationDetectors() {
		t.Run(d.name, func(t *testing.T) {
			tasks := map[string]*Task{
				"A": resTask("A", 1, 0, 1, resAssign("r", 1)),
				"X": resTask("X", 1, 999_999, 1_000_000),
			}
			usage := d.usage(tasks)
			profile := usage["r"]
			if len(usage) != 1 || len(profile) != 1_000_000 {
				t.Fatalf("usage has %d resources and len(usage[r]) = %d, want 1 and 1000000", len(usage), len(profile))
			}
			if profile[0] != 1 || profile[999_999] != 0 {
				t.Errorf("usage[r][0] = %v, usage[r][999999] = %v, want 1 and 0", profile[0], profile[999_999])
			}
			if got := d.detect(tasks, ResourceCapacityPlan{DefaultCapacity: 1}); got != nil {
				t.Errorf("records = %s, want nil", formatOverallocations(got))
			}
			if tasks["A"].Overallocated || tasks["X"].Overallocated {
				t.Error("no task may be flagged")
			}
		})
	}
}

// TestOverallocationRepeatCallClearsFlags calls the capacity-map API three
// times on F-01's map: the second call, with enough capacity, must clear the
// flags the first call set (RES-C03), and the third, after an edit, must
// reflect the edited schedule rather than an earlier result.
func TestOverallocationRepeatCallClearsFlags(t *testing.T) {
	tasks := map[string]*Task{
		"A": resTask("A", 2, 0, 2, resAssign("alice", 1)),
		"B": resTask("B", 1, 0, 1, resAssign("alice", 1)),
		"C": resTask("C", 1, 0, 1, resAssign("bob", 1)),
	}
	first := DetectOverallocations(tasks, nil)
	want := []Overallocation{{Resource: "alice", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}}}
	if diff := overallocationDiff(first, want); diff != "" {
		t.Fatalf("first call:%s", diff)
	}
	if !tasks["A"].Overallocated || !tasks["B"].Overallocated || tasks["C"].Overallocated {
		t.Fatal("first call must flag A and B only")
	}

	if second := DetectOverallocations(tasks, map[string]float64{"alice": 2}); second != nil {
		t.Errorf("second call = %s, want nil", formatOverallocations(second))
	}
	for key, task := range tasks {
		if task.Overallocated {
			t.Errorf("tasks[%q] still flagged after a conflict-free run", key)
		}
	}

	// Edit the same map: B moves to day 1 and C moves onto alice, so day 0's
	// participants change from [A B] to [A C]. A detector that reused
	// participants from an earlier call would report the old list.
	tasks["B"].ES, tasks["B"].EF = 1, 2
	tasks["C"].Assignments = []Assignment{resAssign("alice", 1)}
	third := DetectOverallocations(tasks, nil)
	want = []Overallocation{
		{Resource: "alice", Day: 0, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "C"}},
		{Resource: "alice", Day: 1, Demand: 2, Capacity: 1, TaskIDs: []string{"A", "B"}},
	}
	if diff := overallocationDiff(third, want); diff != "" {
		t.Errorf("after editing the schedule:%s", diff)
	}
}

// TestOverallocationJSONShape pins the wire form (RES-A01): no conflicts is a
// nil slice that marshals to null, field names are snake_case, and a NaN
// demand (F-18) is a record encoding/json refuses to marshal.
func TestOverallocationJSONShape(t *testing.T) {
	empty := DetectOverallocationsWithPlan(map[string]*Task{}, ResourceCapacityPlan{})
	clean := DetectOverallocations(map[string]*Task{"A": resTask("A", 1, 0, 1, resAssign("r", 1))}, nil)
	for name, result := range map[string][]Overallocation{"empty map": empty, "no conflicts": clean} {
		if result != nil {
			t.Errorf("%s: result = %#v, want nil", name, result)
		}
		if b, err := json.Marshal(result); err != nil || string(b) != "null" {
			t.Errorf("%s: json = %s, %v; want null", name, b, err)
		}
	}

	records := DetectOverallocations(map[string]*Task{
		"A": resTask("A", 2, 0, 2, resAssign("alice", 1)),
		"B": resTask("B", 1, 0, 1, resAssign("alice", 1)),
	}, nil)
	b, err := json.Marshal(records)
	const want = `[{"resource":"alice","day":0,"demand":2,"capacity":1,"task_ids":["A","B"]}]`
	if err != nil || string(b) != want {
		t.Errorf("json = %s, %v; want %s", b, err, want)
	}

	nan := DetectOverallocations(map[string]*Task{"A": resTask("A", 1, 0, 1, resAssign("r", math.NaN()))}, nil)
	if len(nan) != 1 || !math.IsNaN(nan[0].Demand) {
		t.Fatalf("F-18 records = %s, want one NaN-demand record", formatOverallocations(nan))
	}
	var unsupported *json.UnsupportedValueError
	if _, err := json.Marshal(nan); !errors.As(err, &unsupported) {
		t.Errorf("marshalling a NaN demand: err = %v, want *json.UnsupportedValueError", err)
	}
}

// TestDetectorAndLevelerReadDifferentCalendars is the two-sided F-09
// tripwire (RES-C08): detection resolves capacity by resource name, while
// leveling honours Assignment.CalendarID. Both halves are current behaviour,
// not a judgement that the difference is right.
func TestDetectorAndLevelerReadDifferentCalendars(t *testing.T) {
	plan := ResourceCapacityPlan{DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{"winter": {Overrides: map[int]float64{0: 0}}}}
	tasks := map[string]*Task{"A": {ID: "A", Duration: 1,
		Assignments: []Assignment{{Resource: "crew", Units: 1, CalendarID: "winter"}}}}

	if _, err := LevelResourcesWithOptions(tasks, plan, LevelingOptions{}); err != nil {
		t.Fatalf("LevelResourcesWithOptions: %v", err)
	}
	if tasks["A"].ES != 1 {
		t.Errorf("leveling put A at ES %v, want 1: the winter calendar has no capacity on day 0", tasks["A"].ES)
	}
	tasks["A"].ES, tasks["A"].EF = 0, 1
	if got := DetectOverallocationsWithPlan(tasks, plan); got != nil {
		t.Errorf("detection = %s, want nil: it reads the crew calendar, which does not exist", formatOverallocations(got))
	}
}
