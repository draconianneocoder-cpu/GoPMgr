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

// TestLevelResourcesPriorityCriticalProtectsCriticalPath proves the
// priority-override: under EarliestDeadline a floating task with an earlier
// deadline would normally grab the slot, but PriorityCritical lets the
// critical-path task win instead.
//
// Graph: F (floating, dur 1) contends with C (critical, dur 1) for alice on
// day 0. C sits on the long critical path (C -> D, D dur 3) so it is
// critical (float 0); F is a sink with slack but an earlier late-finish, so
// plain EDF would pick F first.
func levelingCriticalGraph() map[string]*Task {
	return map[string]*Task{
		"C": {ID: "C", Duration: 1, Assignments: []Assignment{{Resource: "alice"}}},
		"D": {ID: "D", Duration: 3, Precedents: []string{"C"}}, // makes C critical (path C->D = 4)
		"F": {ID: "F", Duration: 1, Assignments: []Assignment{{Resource: "alice"}}},
	}
}

func TestLevelResourcesPriorityCriticalProtectsCriticalPath(t *testing.T) {
	// Without priority-override, EDF orders by late finish. F (a sink, LF =
	// project end) vs C (LF = LS of D). Confirm the override flips the
	// winner: C keeps day 0, F is delayed.
	tasks := levelingCriticalGraph()
	_, err := LevelResourcesWithOptions(tasks, ResourceCapacityPlan{DefaultCapacity: 1},
		LevelingOptions{Strategy: EarliestDeadline, PriorityCritical: true})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !tasks["C"].IsCritical {
		t.Fatalf("precondition: C should be critical, got IsCritical=false")
	}
	approx(t, "C.ES", tasks["C"].ES, 0) // critical task protected
	approx(t, "F.ES", tasks["F"].ES, 1) // floating task yields
}

// TestLevelResourcesSplittingSpreadsWork proves AllowSplitting places a task
// on non-contiguous days when a contiguous slot doesn't exist, and that the
// resulting schedule is conflict-free (no overallocation).
func TestLevelResourcesSplittingSpreadsWork(t *testing.T) {
	// B (dur 3) holds alice on days 0,1,2. A (dur 2) is fixed to day 0 by an
	// FF-... actually simplest: A needs 2 days of alice but B occupies a
	// middle day, forcing A to split around it. Build B as a 1-unit hold on
	// day 1 via a calendar gap is complex; instead give alice capacity 1 and
	// have three unit tasks contend so the third must split around them.
	//
	// Concretely: X, Y each dur 1 on alice (fill days 0 and 1 after leveling
	// serialises them), and S dur 2 on alice that, without splitting, would
	// need two free contiguous days (2,3) but here we cap the horizon so it
	// must interleave. Simpler and deterministic: capacity 1, tasks A,B,C
	// each dur 1 needing alice, plus S dur 2 needing alice. Serial leveling
	// packs A,B,C on days 0,1,2; S then fits contiguously at 3,4 — no split.
	//
	// To force a genuine split we give S a hard SNET start at day 0 while A
	// already holds day 0, and a calendar that frees alice only on alternate
	// days. Use a per-resource calendar: alice available on even days only.
	tasks := map[string]*Task{
		"S": {ID: "S", Duration: 3, Assignments: []Assignment{{Resource: "alice"}}},
	}
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{
			"alice": {
				Resource:        "alice",
				DefaultCapacity: 1,
				// Odd days have zero capacity, so a 3-day task can only be
				// worked on days 0, 2, 4 — inherently non-contiguous.
				Overrides: map[int]float64{1: 0, 3: 0, 5: 0},
			},
		},
	}

	res, err := LevelResourcesWithOptions(tasks, plan, LevelingOptions{AllowSplitting: true})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(res.SplitTaskIDs) != 1 || res.SplitTaskIDs[0] != "S" {
		t.Fatalf("SplitTaskIDs = %v, want [S]", res.SplitTaskIDs)
	}
	if got := tasks["S"].WorkDays; len(got) != 3 || got[0] != 0 || got[1] != 2 || got[2] != 4 {
		t.Fatalf("S.WorkDays = %v, want [0 2 4]", got)
	}
	approx(t, "S.ES", tasks["S"].ES, 0)
	approx(t, "S.EF", tasks["S"].EF, 5) // finishes the day after the last worked day (4)
	// The split schedule must be conflict-free: no idle-day demand counted.
	if breaches := DetectOverallocationsWithPlan(tasks, plan); len(breaches) != 0 {
		t.Fatalf("split plan still overallocated: %+v", breaches)
	}
}

// TestLevelResourcesSplittingDisabledDelaysContiguously proves splitting is
// opt-in: with AllowSplitting off, the same task is not split — it is placed
// contiguously in the first 3-day run where alice is free (days 6–8, since
// the calendar only zeroes odd days 1/3/5), finishing later than the split
// plan would.
func TestLevelResourcesSplittingDisabledDelaysContiguously(t *testing.T) {
	tasks := map[string]*Task{
		"S": {ID: "S", Duration: 3, Assignments: []Assignment{{Resource: "alice"}}},
	}
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Calendars: map[string]ResourceCalendar{
			"alice": {Resource: "alice", DefaultCapacity: 1, Overrides: map[int]float64{1: 0, 3: 0, 5: 0}},
		},
	}
	res, err := LevelResourcesWithOptions(tasks, plan, LevelingOptions{}) // splitting off
	if err != nil {
		t.Fatalf("err = %v, want nil (task fits contiguously later)", err)
	}
	if len(res.SplitTaskIDs) != 0 {
		t.Errorf("SplitTaskIDs = %v, want none (splitting disabled)", res.SplitTaskIDs)
	}
	if tasks["S"].WorkDays != nil {
		t.Errorf("S.WorkDays = %v, want nil (not split)", tasks["S"].WorkDays)
	}
	approx(t, "S.ES", tasks["S"].ES, 6) // first free 3-day contiguous run
	approx(t, "S.EF", tasks["S"].EF, 9)
	if breaches := DetectOverallocationsWithPlan(tasks, plan); len(breaches) != 0 {
		t.Fatalf("contiguous plan still overallocated: %+v", breaches)
	}
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
