// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"testing"
)

// TestLevelResourcesWithOptions_HorizonExceededReturnsSentinel proves that a
// task whose demand can never fit capacity is surfaced via
// ErrLevelingHorizonExceeded and named in UnplacedTaskIDs, rather than being
// silently capped.
func TestLevelResourcesWithOptions_HorizonExceededReturnsSentinel(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice", Units: 2}}}, // 2 > capacity 1
	}
	res, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{})
	if !errors.Is(err, ErrLevelingHorizonExceeded) {
		t.Fatalf("err = %v, want ErrLevelingHorizonExceeded", err)
	}
	if len(res.UnplacedTaskIDs) != 1 || res.UnplacedTaskIDs[0] != "A" {
		t.Fatalf("UnplacedTaskIDs = %v, want [A]", res.UnplacedTaskIDs)
	}
	// The unplaceable task stays at its earliest start and remains visible
	// to overallocation detection.
	approx(t, "A.ES", tasks["A"].ES, 0)
	if breaches := DetectOverallocations(tasks, nil); len(breaches) != 1 {
		t.Errorf("impossible demand must remain visible: %+v", breaches)
	}
}

// TestLevelResourcesWithOptions_CycleReturnsSentinel proves a dependency
// cycle is reported as ErrSchedulingCycle (distinct from a horizon overflow).
func TestLevelResourcesWithOptions_CycleReturnsSentinel(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, Precedents: []string{"B"}},
		"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
	}
	res, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{})
	if !errors.Is(err, ErrSchedulingCycle) {
		t.Fatalf("err = %v, want ErrSchedulingCycle", err)
	}
	if len(res.UnplacedTaskIDs) != 0 {
		t.Errorf("UnplacedTaskIDs = %v, want empty on cycle", res.UnplacedTaskIDs)
	}
}

// TestLevelResourcesWithOptions_CustomHorizonBounds proves the per-schedule
// Horizon is honoured: the same contention that a too-small horizon cannot
// resolve is placed cleanly once the horizon is large enough.
func TestLevelResourcesWithOptions_CustomHorizonBounds(t *testing.T) {
	newTasks := func() map[string]*Task {
		return map[string]*Task{
			"A": {ID: "A", Duration: 2,
				Assignments: []Assignment{{Resource: "alice"}}},
			"B": {ID: "B", Duration: 2,
				Assignments: []Assignment{{Resource: "alice"}}}, // must shift 2 days behind A
		}
	}

	// Horizon 1 cannot reach the offset-2 free slot: B is left unplaced.
	tight := newTasks()
	res, err := LevelResourcesWithOptions(tight, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{Horizon: 1})
	if !errors.Is(err, ErrLevelingHorizonExceeded) {
		t.Fatalf("tight horizon: err = %v, want ErrLevelingHorizonExceeded", err)
	}
	if len(res.UnplacedTaskIDs) != 1 || res.UnplacedTaskIDs[0] != "B" {
		t.Fatalf("tight horizon: UnplacedTaskIDs = %v, want [B]", res.UnplacedTaskIDs)
	}

	// Horizon 5 reaches the offset-2 slot: fully levelled, no overflow.
	roomy := newTasks()
	res, err = LevelResourcesWithOptions(roomy, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{Horizon: 5})
	if err != nil {
		t.Fatalf("roomy horizon: err = %v, want nil", err)
	}
	if len(res.UnplacedTaskIDs) != 0 {
		t.Fatalf("roomy horizon: UnplacedTaskIDs = %v, want empty", res.UnplacedTaskIDs)
	}
	approx(t, "B.ES", roomy["B"].ES, 2)
	if breaches := DetectOverallocations(roomy, nil); len(breaches) != 0 {
		t.Errorf("roomy horizon: plan still overallocated: %+v", breaches)
	}
}

// TestLevelResourcesWithOptions_FullyLevelledReturnsNil covers the clean
// success path: no cycle, nothing unplaceable.
func TestLevelResourcesWithOptions_FullyLevelledReturnsNil(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 2,
			Assignments: []Assignment{{Resource: "alice"}}},
	}
	res, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(res.UnplacedTaskIDs) != 0 {
		t.Fatalf("UnplacedTaskIDs = %v, want empty", res.UnplacedTaskIDs)
	}
}

// TestLevelResourcesWithPlan_HorizonOverflowStillReturnsTrue pins the
// backward-compatible bool wrapper: a horizon overflow is NOT a cycle, so the
// wrapper still reports success (true), matching the pre-sentinel behaviour.
func TestLevelResourcesWithPlan_HorizonOverflowStillReturnsTrue(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Assignments: []Assignment{{Resource: "alice", Units: 2}}},
	}
	if !LevelResourcesWithPlan(tasks, ResourceCapacityPlan{DefaultCapacity: 1}) {
		t.Error("horizon overflow must still return true (only a cycle returns false)")
	}
}

// levelingStrategyGraph builds a fresh contention graph where the
// LeastTotalFloat and EarliestDeadline strategies disagree about which of
// two alice-contending tasks (A, B) claims day 0:
//
//   - A: dur 1, feeds a sink P that pins A's deadline early (LF=5, LS=4).
//   - B: dur 5, a sink with a late deadline but little start slack
//     (LF=6, LS=1).
//   - LP: dur 6, no resource — the long pole that sets project length 6.
//
// LTF orders by LS: B (1) < A (4) → B wins day 0.
// EDF orders by LF: A (5) < B (6) → A wins day 0.
func levelingStrategyGraph() map[string]*Task {
	return map[string]*Task{
		"A":  {ID: "A", Duration: 1, Assignments: []Assignment{{Resource: "alice"}}},
		"P":  {ID: "P", Duration: 1, Precedents: []string{"A"}}, // tightens A's deadline
		"B":  {ID: "B", Duration: 5, Assignments: []Assignment{{Resource: "alice"}}},
		"LP": {ID: "LP", Duration: 6}, // long pole, no resource, sets project length
	}
}

// TestLevelResourcesStrategyLeastTotalFloat pins the default: the
// least-float task (B) claims day 0 and the floating task (A) is delayed.
func TestLevelResourcesStrategyLeastTotalFloat(t *testing.T) {
	tasks := levelingStrategyGraph()
	_, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1},
		LevelingOptions{Strategy: LeastTotalFloat})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	approx(t, "B.ES", tasks["B"].ES, 0) // least float wins the slot
	approx(t, "A.ES", tasks["A"].ES, 5) // floats behind B's 5-day span
}

// TestLevelResourcesStrategyEarliestDeadline proves EDF diverges from the
// default: the earlier-deadline task (A) claims day 0 instead.
func TestLevelResourcesStrategyEarliestDeadline(t *testing.T) {
	tasks := levelingStrategyGraph()
	_, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1},
		LevelingOptions{Strategy: EarliestDeadline})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	approx(t, "A.ES", tasks["A"].ES, 0) // earliest deadline wins the slot
	approx(t, "B.ES", tasks["B"].ES, 1) // starts once alice frees after A's day 0
}

// TestLevelResourcesEmptyStrategyDefaultsToLeastTotalFloat proves the empty
// strategy is treated as LeastTotalFloat (backward-compatible default).
func TestLevelResourcesEmptyStrategyDefaultsToLeastTotalFloat(t *testing.T) {
	tasks := levelingStrategyGraph()
	if _, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1},
		LevelingOptions{}); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	approx(t, "B.ES", tasks["B"].ES, 0) // same as explicit LeastTotalFloat
	approx(t, "A.ES", tasks["A"].ES, 5)
}

// TestDefaultLevelingHorizonUsedWhenUnset proves a zero Horizon falls back to
// DefaultLevelingHorizon rather than refusing to search at all.
func TestDefaultLevelingHorizonUsedWhenUnset(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, Assignments: []Assignment{{Resource: "alice"}}},
		"B": {ID: "B", Duration: 1, Assignments: []Assignment{{Resource: "alice"}}},
	}
	// Horizon:0 -> DefaultLevelingHorizon; B shifts one day behind A and fits.
	res, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1}, LevelingOptions{Horizon: 0})
	if err != nil {
		t.Fatalf("err = %v, want nil (default horizon should place B)", err)
	}
	if len(res.UnplacedTaskIDs) != 0 {
		t.Fatalf("UnplacedTaskIDs = %v, want empty", res.UnplacedTaskIDs)
	}
	approx(t, "B.ES", tasks["B"].ES, 1)
}
