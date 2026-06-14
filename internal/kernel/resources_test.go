// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "testing"

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
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
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
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
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
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
	}
	approx(t, "B.ES", tasks["B"].ES, 3) // A finishes day 2 + lag 1
}

func TestLevelResourcesFractionalUnitsShare(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice", Units: 0.5}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice", Units: 0.5}}},
	}
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
	}
	// Half-time each: both fit in parallel.
	approx(t, "A.ES", tasks["A"].ES, 0)
	approx(t, "B.ES", tasks["B"].ES, 0)
}

func TestLevelResourcesImpossibleDemandStaysPut(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice", Units: 2}}},
	}
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
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
	if LevelResources(tasks, nil) {
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
	if !LevelResources(tasks, nil) {
		t.Fatal("LevelResources reported a cycle")
	}
	approx(t, "X.ES", tasks["X"].ES, 0)
}
