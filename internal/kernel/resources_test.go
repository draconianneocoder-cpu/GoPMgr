// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"testing"
)

func TestResourceUsageProfile(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 3,
			Assignments: []Assignment{{Resource: "alice", Units: 0.5}}},
	}
	mustCPM(t, tasks)

	usage := ResourceUsage(tasks)
	alice := usage["alice"]
	if len(alice) != 3 {
		t.Fatalf("profile length = %d, want 3", len(alice))
	}
	// Days 0-1: A (1.0) + B (0.5); day 2: B only.
	approx(t, "alice[0]", alice[0], 1.5)
	approx(t, "alice[1]", alice[1], 1.5)
	approx(t, "alice[2]", alice[2], 0.5)
}

func TestResourceUsageCapsAssignmentAtMaxUnits(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice", Units: 1.5, MaxUnits: 0.75}}},
	}
	mustCPM(t, tasks)

	usage := ResourceUsage(tasks)
	approx(t, "alice[0]", usage["alice"][0], 0.75)
}

func TestResourceCapacityProfilesUsesCalendarResolution(t *testing.T) {
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Capacities: map[string]float64{
			"alice": 0.75,
		},
		Calendars: map[string]ResourceCalendar{
			"alice": {
				Resource: "alice",
				WeeklyCapacity: map[int]float64{
					1: 0.5,
				},
				Overrides: map[int]float64{
					2: 0,
				},
			},
		},
	}

	profiles := ResourceCapacityProfiles(plan, []string{"bob", "alice", "alice"}, 4)

	if len(profiles) != 2 {
		t.Fatalf("profiles = %d, want 2: %+v", len(profiles), profiles)
	}
	alice := profiles["alice"]
	if len(alice) != 4 {
		t.Fatalf("alice profile length = %d, want 4", len(alice))
	}
	approx(t, "alice[0]", alice[0], 0.75)
	approx(t, "alice[1]", alice[1], 0.5)
	approx(t, "alice[2]", alice[2], 0)
	approx(t, "alice[3]", alice[3], 0.75)
	approx(t, "bob[0]", profiles["bob"][0], 1)
}

func TestDetectOverallocations(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 1,
			Assignments: []Assignment{{Resource: "alice"}}},
		"C": {ID: "C", Duration: 1,
			Assignments: []Assignment{{Resource: "bob"}}},
	}
	mustCPM(t, tasks)

	breaches := DetectOverallocations(tasks, nil)

	// alice is double-booked on day 0 (A+B); day 1 only A. bob fine.
	if len(breaches) != 1 {
		t.Fatalf("breaches = %d, want 1: %+v", len(breaches), breaches)
	}
	b := breaches[0]
	if b.Resource != "alice" || b.Day != 0 {
		t.Errorf("breach = %+v, want alice day 0", b)
	}
	approx(t, "demand", b.Demand, 2)
	if len(b.TaskIDs) != 2 || b.TaskIDs[0] != "A" || b.TaskIDs[1] != "B" {
		t.Errorf("TaskIDs = %v, want [A B]", b.TaskIDs)
	}
	if !tasks["A"].Overallocated || !tasks["B"].Overallocated {
		t.Error("A and B must be flagged overallocated")
	}
	if tasks["C"].Overallocated {
		t.Error("C must not be flagged")
	}
}

func TestDetectOverallocationsHonoursCapacity(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "team"}}},
		"B": {ID: "B", Duration: 1,
			Assignments: []Assignment{{Resource: "team"}}},
	}
	mustCPM(t, tasks)

	if breaches := DetectOverallocations(tasks, map[string]float64{"team": 2}); len(breaches) != 0 {
		t.Errorf("capacity 2 must absorb both tasks: %+v", breaches)
	}
	if tasks["A"].Overallocated || tasks["B"].Overallocated {
		t.Error("no task should stay flagged after a clean detection run")
	}
}

func TestDetectOverallocationsWithResourceCalendarOverrides(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
	}
	mustCPM(t, tasks)

	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{
			"alice": {
				Resource: "alice",
				Overrides: map[int]float64{
					1: 0, // sick leave / unavailable
				},
			},
		},
	}
	breaches := DetectOverallocationsWithPlan(tasks, plan)

	if len(breaches) != 1 {
		t.Fatalf("breaches = %d, want 1: %+v", len(breaches), breaches)
	}
	if breaches[0].Resource != "alice" || breaches[0].Day != 1 {
		t.Fatalf("breach = %+v, want alice day 1", breaches[0])
	}
	approx(t, "capacity", breaches[0].Capacity, 0)
	if !tasks["A"].Overallocated {
		t.Fatal("task occupying zero-capacity day must be flagged")
	}
}

func TestLevelResourcesSerialisesContention(t *testing.T) {
	// A and B both need alice full-time and could run in parallel.
	// B has less float in a richer graph; here LS ties are broken by
	// ID, so A goes first.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}

	approx(t, "A.ES", tasks["A"].ES, 0)
	approx(t, "B.ES", tasks["B"].ES, 2) // pushed behind A
	if breaches := DetectOverallocations(tasks, nil); len(breaches) != 0 {
		t.Errorf("levelled plan still overallocated: %+v", breaches)
	}
}

func TestLevelResourcesPrioritisesLeastFloat(t *testing.T) {
	// C depends on B, so B is critical-ish (smaller LS); A floats.
	// Both A and B want alice on day 0: B must win the slot.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"C": {ID: "C", Duration: 3, Precedents: []string{"B"}},
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}

	approx(t, "B.ES", tasks["B"].ES, 0) // least float goes first
	approx(t, "A.ES", tasks["A"].ES, 2) // floats behind B
	approx(t, "C.ES", tasks["C"].ES, 2) // still right after B
}

func TestLevelResourcesRespectsLinksAndLag(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 1,
			Links:       []Link{{Pred: "A", Type: FinishToStart, Lag: 1}},
			Assignments: []Assignment{{Resource: "alice"}}},
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}
	approx(t, "B.ES", tasks["B"].ES, 3) // A finishes day 2 + lag 1
}

func TestLevelResourcesWithCalendarsSkipsUnavailableDays(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice"}}},
	}
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{
			"alice": {
				Resource: "alice",
				Overrides: map[int]float64{
					0: 0, // unavailable on the earliest day
				},
			},
		},
	}

	if !levelResourcesWithPlan(tasks, plan) {
		t.Fatal("leveling reported a cycle")
	}
	approx(t, "A.ES", tasks["A"].ES, 1)
	approx(t, "A.EF", tasks["A"].EF, 2)
	if breaches := DetectOverallocationsWithPlan(tasks, plan); len(breaches) != 0 {
		t.Fatalf("levelled plan still overallocated: %+v", breaches)
	}
}

func TestLevelResourcesWithAssignmentCalendarID(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "crew", CalendarID: "winter"}}},
	}
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{
			"winter": {
				ID:              "winter",
				DefaultCapacity: 1,
				Overrides: map[int]float64{
					0: 0,
				},
			},
		},
	}

	if !levelResourcesWithPlan(tasks, plan) {
		t.Fatal("leveling reported a cycle")
	}
	approx(t, "A.ES", tasks["A"].ES, 1)
	approx(t, "A.EF", tasks["A"].EF, 2)
}

func TestLevelResourcesFractionalUnitsShare(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice", Units: 0.5}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice", Units: 0.5}}},
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}
	// Half-time each: both fit in parallel.
	approx(t, "A.ES", tasks["A"].ES, 0)
	approx(t, "B.ES", tasks["B"].ES, 0)
}

// Part-day levelling (owner decision 2026-11-21): a task is tried at its
// earliest start, part days included, and demand counts the share of each
// day used, so part-day work can share a day. Everything runs on alice at
// capacity 1 unless stated; every oracle is worked by hand in its case.
func TestLevelResourcesInPartDays(t *testing.T) {
	alice := []Assignment{{Resource: "alice"}}
	cases := []struct {
		name     string
		tasks    map[string]*Task
		capacity float64
		want     map[string]float64 // levelled ES
	}{
		{
			// A [0, 0.5), B [0.5, 0.9), C [0.9, 1.4): day 0 holds 0.5 + 0.4
			// + 0.1 = 1, day 1 0.4; nothing moves. Rounding up used to put
			// B on day 1 and C on day 2.
			name: "a chain of part-day tasks stays on its days",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0.5, Assignments: alice},
				"B": {ID: "B", Duration: 0.4, Precedents: []string{"A"}, Assignments: alice},
				"C": {ID: "C", Duration: 0.5, Precedents: []string{"B"}, Assignments: alice},
			},
			capacity: 1,
			want:     map[string]float64{"A": 0, "B": 0.5, "C": 0.9},
		},
		{
			// Two unlinked half-day tasks fill day 0 exactly.
			name: "two half-day tasks share a day",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0.5, Assignments: alice},
				"B": {ID: "B", Duration: 0.5, Assignments: alice},
			},
			capacity: 1,
			want:     map[string]float64{"A": 0, "B": 0},
		},
		{
			// A (0.75 day) goes first (smaller late start: the project
			// finish is 0.75, so A's LS is 0 and B's 0.25) and takes 0.75
			// of day 0; B's 0.5 more would make 1.25 > 1, so B moves to the
			// next whole day, 1.
			name: "a half-day task that does not fit moves to the next day",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0.75, Assignments: alice},
				"B": {ID: "B", Duration: 0.5, Assignments: alice},
			},
			capacity: 1,
			want:     map[string]float64{"A": 0, "B": 1},
		},
		{
			// P (no resource) ends at 0.5, so Y could start at 0.5; X fills
			// day 0, so Y's half day does not fit there and Y moves to 1.
			name: "a part-day earliest start that does not fit moves to the next day",
			tasks: map[string]*Task{
				"P": {ID: "P", Duration: 0.5},
				"X": {ID: "X", Duration: 1, Assignments: alice},
				"Y": {ID: "Y", Duration: 0.5, Precedents: []string{"P"}, Assignments: alice},
			},
			capacity: 1,
			want:     map[string]float64{"P": 0, "X": 0, "Y": 1},
		},
		{
			// A milestone between two half-day tasks keeps its part-day
			// start (0.5), so B starts at 0.5 too: day 0 holds 0.5 + 0.5.
			name: "a milestone in a part-day chain is not rounded up",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0.5, Assignments: alice},
				"M": {ID: "M", Duration: 0, Milestone: true, Precedents: []string{"A"}},
				"B": {ID: "B", Duration: 0.5, Precedents: []string{"M"}, Assignments: alice},
			},
			capacity: 1,
			want:     map[string]float64{"A": 0, "M": 0.5, "B": 0.5},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !levelResources(tc.tasks, map[string]float64{"alice": tc.capacity}) {
				t.Fatal("leveling reported a cycle")
			}
			for id, es := range tc.want {
				approx(t, id+".ES", tc.tasks[id].ES, es)
			}
			if over := DetectOverallocations(tc.tasks, map[string]float64{"alice": tc.capacity}); len(over) != 0 {
				t.Errorf("levelled schedule still overallocated: %+v", over)
			}
		})
	}
}

// A task that can never fit (two units of a one-person resource) stays at
// its earliest start, part days included, and is reported unplaced: P (0.5
// day, no resource) ends at 0.5, so Y starts at 0.5, not at day 1.
func TestLevelResourcesUnplaceablePartDayStartStays(t *testing.T) {
	tasks := map[string]*Task{
		"P": {ID: "P", Duration: 0.5},
		"Y": {ID: "Y", Duration: 1, Precedents: []string{"P"}, Assignments: []Assignment{{Resource: "alice", Units: 2}}},
	}
	res, err := LevelResourcesWithOptions(tasks, capacityPlanFromMap(map[string]float64{"alice": 1}), LevelingOptions{})
	if !errors.Is(err, ErrLevelingHorizonExceeded) {
		t.Fatalf("error = %v, want ErrLevelingHorizonExceeded", err)
	}
	if len(res.UnplacedTaskIDs) != 1 || res.UnplacedTaskIDs[0] != "Y" {
		t.Fatalf("unplaced = %v, want [Y]", res.UnplacedTaskIDs)
	}
	approx(t, "Y.ES", tasks["Y"].ES, 0.5)
}

func TestLevelResourcesImpossibleDemandStaysPut(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice", Units: 2}}},
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}
	// Units 2 can never fit capacity 1: stays at earliest, flagged by
	// a subsequent detection run.
	approx(t, "A.ES", tasks["A"].ES, 0)
	if breaches := DetectOverallocations(tasks, nil); len(breaches) != 1 {
		t.Errorf("impossible demand must remain visible: %+v", breaches)
	}
}

func TestLevelResourcesCycleDetected(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, Precedents: []string{"B"}},
		"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
	}
	if levelResources(tasks, nil) {
		t.Error("cycle must fail leveling")
	}
}

func TestLevelResourcesUnassignedTasksUntouchedByContention(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"X": {ID: "X", Duration: 1}, // no resources
	}
	if !levelResources(tasks, nil) {
		t.Fatal("leveling reported a cycle")
	}
	approx(t, "X.ES", tasks["X"].ES, 0)
}
