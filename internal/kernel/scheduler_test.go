// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"testing"
)

// mkTask inserts a Task into the map and returns it.
func mkTask(m map[string]*Task, id string, dur float64, preds ...string) *Task {
	t := &Task{ID: id, Duration: dur, Precedents: preds}
	m[id] = t
	return t
}

// near asserts that got and want are within floating-point epsilon.
func near(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func TestCalculateCPMEmpty(t *testing.T) {
	if !CalculateCPM(map[string]*Task{}) {
		t.Fatal("empty map should return true (acyclic by definition)")
	}
}

func TestCalculateCPMSingleTask(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 5)

	if !CalculateCPM(tasks) {
		t.Fatal("expected true")
	}
	a := tasks["A"]
	near(t, "A.ES", a.ES, 0)
	near(t, "A.EF", a.EF, 5)
	near(t, "A.LS", a.LS, 0)
	near(t, "A.LF", a.LF, 5)
	near(t, "A.Float", a.Float, 0)
	if !a.IsCritical {
		t.Error("sole task must be critical")
	}
}

// TestCalculateCPMLinearChain verifies a sequential A(3)→B(2)→C(4) network.
// All tasks lie on the critical path (no slack exists).
func TestCalculateCPMLinearChain(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 3)
	mkTask(tasks, "B", 2, "A")
	mkTask(tasks, "C", 4, "B")

	if !CalculateCPM(tasks) {
		t.Fatal("expected true")
	}

	a, b, c := tasks["A"], tasks["B"], tasks["C"]

	// Forward pass
	near(t, "A.ES", a.ES, 0)
	near(t, "A.EF", a.EF, 3)
	near(t, "B.ES", b.ES, 3)
	near(t, "B.EF", b.EF, 5)
	near(t, "C.ES", c.ES, 5)
	near(t, "C.EF", c.EF, 9)

	// Backward pass
	near(t, "A.LS", a.LS, 0)
	near(t, "A.LF", a.LF, 3)
	near(t, "B.LS", b.LS, 3)
	near(t, "B.LF", b.LF, 5)
	near(t, "C.LS", c.LS, 5)
	near(t, "C.LF", c.LF, 9)

	for _, id := range []string{"A", "B", "C"} {
		near(t, id+".Float", tasks[id].Float, 0)
		if !tasks[id].IsCritical {
			t.Errorf("%s should be critical in a linear chain", id)
		}
	}
}

// TestCalculateCPMDiamond verifies a diamond network:
//
//	A(3) → B(5) → D(1)
//	     → C(2) → D(1)
//
// Critical path A→B→D (duration 9). C has float = 3.
func TestCalculateCPMDiamond(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 3)
	mkTask(tasks, "B", 5, "A")
	mkTask(tasks, "C", 2, "A")
	mkTask(tasks, "D", 1, "B", "C")

	if !CalculateCPM(tasks) {
		t.Fatal("expected true")
	}

	a, b, c, d := tasks["A"], tasks["B"], tasks["C"], tasks["D"]

	// Forward pass
	near(t, "A.EF", a.EF, 3)
	near(t, "B.ES", b.ES, 3)
	near(t, "B.EF", b.EF, 8)
	near(t, "C.ES", c.ES, 3)
	near(t, "C.EF", c.EF, 5)
	near(t, "D.ES", d.ES, 8) // max(EF_B=8, EF_C=5)
	near(t, "D.EF", d.EF, 9)

	// Backward pass
	near(t, "D.LF", d.LF, 9)
	near(t, "D.LS", d.LS, 8)
	near(t, "B.LF", b.LF, 8)
	near(t, "B.LS", b.LS, 3)
	near(t, "C.LF", c.LF, 8) // pulls back from D.LS = 8
	near(t, "C.LS", c.LS, 6)
	near(t, "A.LF", a.LF, 3) // min(LS_B=3, LS_C=6) = 3
	near(t, "A.LS", a.LS, 0)

	// Float and criticality
	near(t, "A.Float", a.Float, 0)
	near(t, "B.Float", b.Float, 0)
	near(t, "C.Float", c.Float, 3)
	near(t, "D.Float", d.Float, 0)

	if !a.IsCritical {
		t.Error("A should be critical")
	}
	if !b.IsCritical {
		t.Error("B should be critical")
	}
	if c.IsCritical {
		t.Error("C should NOT be critical (float=3)")
	}
	if !d.IsCritical {
		t.Error("D should be critical")
	}
}

// TestCalculateCPMParallelEqualPaths verifies that two parallel branches
// of identical length are both critical.
func TestCalculateCPMParallelEqualPaths(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 3)
	mkTask(tasks, "B", 4, "A")
	mkTask(tasks, "C", 4, "A")
	mkTask(tasks, "D", 2, "B", "C")

	if !CalculateCPM(tasks) {
		t.Fatal("expected true")
	}

	for _, id := range []string{"A", "B", "C", "D"} {
		near(t, id+".Float", tasks[id].Float, 0)
		if !tasks[id].IsCritical {
			t.Errorf("%s should be critical (equal-length parallel paths)", id)
		}
	}
}

// TestCalculateCPMZeroDurationMilestones verifies that zero-duration tasks
// (milestones) participate correctly in the forward/backward pass.
func TestCalculateCPMZeroDurationMilestones(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "Start", 0)
	mkTask(tasks, "Work", 5, "Start")
	mkTask(tasks, "End", 0, "Work")

	if !CalculateCPM(tasks) {
		t.Fatal("expected true")
	}

	near(t, "End.EF", tasks["End"].EF, 5)
	near(t, "End.Float", tasks["End"].Float, 0)
	if !tasks["End"].IsCritical {
		t.Error("zero-duration terminal milestone should be critical")
	}
}

func TestCalculateCPMCycleDetected(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 1, "B")
	mkTask(tasks, "B", 1, "A")

	if CalculateCPM(tasks) {
		t.Fatal("expected false for cyclic graph")
	}
}

func TestCalculateCPMSelfLoop(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 2, "A")

	if CalculateCPM(tasks) {
		t.Fatal("expected false for self-loop")
	}
}

// TestTopoSortDependencyOrder verifies that every predecessor appears before
// its successors in the output.
func TestTopoSortDependencyOrder(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "X", 1)
	mkTask(tasks, "Y", 1, "X")
	mkTask(tasks, "Z", 1, "Y")

	order, ok := topoSort(tasks)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	if pos["X"] >= pos["Y"] {
		t.Error("X must come before Y")
	}
	if pos["Y"] >= pos["Z"] {
		t.Error("Y must come before Z")
	}
}

// TestTopoSortDeterministic verifies that independent tasks sort
// alphabetically so snapshot tests remain reproducible across runs.
func TestTopoSortDeterministic(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "Beta", 1)
	mkTask(tasks, "Alpha", 1)

	order, ok := topoSort(tasks)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(order) != 2 || order[0] != "Alpha" || order[1] != "Beta" {
		t.Errorf("expected [Alpha Beta], got %v", order)
	}
}
