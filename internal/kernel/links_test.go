// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"testing"
)

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func runCPM(t *testing.T, tasks map[string]*Task) {
	t.Helper()
	if !CalculateCPM(tasks) {
		t.Fatal("CalculateCPM reported a cycle in an acyclic graph")
	}
}

func TestCPM_FSWithLag(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 2,
			Links: []Link{{Pred: "A", Type: FinishToStart, Lag: 2}}},
	}
	runCPM(t, tasks)

	approx(t, "B.ES", tasks["B"].ES, 5) // 3 (A.EF) + 2 lag
	approx(t, "B.EF", tasks["B"].EF, 7)
	// Both critical: the lag consumes no float.
	if !tasks["A"].IsCritical || !tasks["B"].IsCritical {
		t.Error("A and B should both be critical through an FS+2 chain")
	}
}

func TestCPM_StartToStart(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 5},
		"B": {ID: "B", Duration: 2,
			Links: []Link{{Pred: "A", Type: StartToStart, Lag: 1}}},
	}
	runCPM(t, tasks)

	approx(t, "B.ES", tasks["B"].ES, 1) // A.ES(0) + 1
	approx(t, "B.EF", tasks["B"].EF, 3)
	// Project finish is A's EF=5; B has float 5-3=2.
	approx(t, "B.Float", tasks["B"].Float, 2)
	if !tasks["A"].IsCritical {
		t.Error("A drives the finish and should be critical")
	}
}

func TestCPM_FinishToFinish(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {ID: "B", Duration: 2,
			Links: []Link{{Pred: "A", Type: FinishToFinish, Lag: 1}}},
	}
	runCPM(t, tasks)

	// B must finish >= A.EF(4) + 1 = 5 → ES = 5 - 2 = 3.
	approx(t, "B.ES", tasks["B"].ES, 3)
	approx(t, "B.EF", tasks["B"].EF, 5)
	if !tasks["A"].IsCritical || !tasks["B"].IsCritical {
		t.Error("FF chain driving the finish should be fully critical")
	}
}

func TestCPM_StartToFinish(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2},
		"B": {ID: "B", Duration: 1,
			Links: []Link{{Pred: "A", Type: StartToFinish, Lag: 3}}},
	}
	runCPM(t, tasks)

	// B must finish >= A.ES(0) + 3 = 3 → ES = 3 - 1 = 2.
	approx(t, "B.ES", tasks["B"].ES, 2)
	approx(t, "B.EF", tasks["B"].EF, 3)
}

func TestCPM_NegativeLagLeadClampsAtProjectStart(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1},
		"B": {ID: "B", Duration: 1,
			Links: []Link{{Pred: "A", Type: FinishToStart, Lag: -5}}},
	}
	runCPM(t, tasks)

	// A.EF(1) - 5 = -4 → clamped to 0.
	approx(t, "B.ES", tasks["B"].ES, 0)
}

func TestCPM_LegacyPrecedentsStillWork(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 2, Precedents: []string{"A"}},
	}
	runCPM(t, tasks)

	approx(t, "B.ES", tasks["B"].ES, 3)
	if !tasks["A"].IsCritical || !tasks["B"].IsCritical {
		t.Error("plain-precedent chain should be fully critical")
	}
}

func TestCPM_TypedLinkWinsOverDuplicatePrecedent(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 2,
			Precedents: []string{"A"},
			Links:      []Link{{Pred: "A", Type: StartToStart, Lag: 0}}},
	}
	runCPM(t, tasks)

	// SS wins: B starts with A, not after it.
	approx(t, "B.ES", tasks["B"].ES, 0)
}

func TestCPM_UnknownLinkTypeNormalisesToFS(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 3},
		"B": {ID: "B", Duration: 2,
			Links: []Link{{Pred: "A", Type: "XX", Lag: 0}}},
	}
	runCPM(t, tasks)

	approx(t, "B.ES", tasks["B"].ES, 3)
}

func TestCPM_LinkCycleDetected(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1,
			Links: []Link{{Pred: "B", Type: StartToStart}}},
		"B": {ID: "B", Duration: 1,
			Links: []Link{{Pred: "A", Type: FinishToStart}}},
	}
	if CalculateCPM(tasks) {
		t.Error("cycle through typed links must be detected")
	}
}

func TestCPM_MixedLinksBackwardPass(t *testing.T) {
	// A(2) -FS-> C(2); B(1) -SS+1-> C. C terminal.
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2},
		"B": {ID: "B", Duration: 1},
		"C": {ID: "C", Duration: 2, Links: []Link{
			{Pred: "A", Type: FinishToStart},
			{Pred: "B", Type: StartToStart, Lag: 1},
		}},
	}
	runCPM(t, tasks)

	// Forward: C.ES = max(A.EF=2, B.ES+1=1) = 2; C.EF = 4.
	approx(t, "C.ES", tasks["C"].ES, 2)
	// Backward: C.LS = 2. A.LF = 2 (FS) → float 0.
	// B: SS candidate = C.LS - 1 + dur(1) = 2 → B.LF = 2, LS = 1, float 1.
	approx(t, "A.Float", tasks["A"].Float, 0)
	approx(t, "B.Float", tasks["B"].Float, 1)
	if !tasks["A"].IsCritical || tasks["B"].IsCritical {
		t.Error("A critical, B not — got the reverse")
	}
}
