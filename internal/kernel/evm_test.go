// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"errors"
	"math"
	"testing"

	"gopmgr/internal/money"
)

func mustComputeEVM(t *testing.T, tasks map[string]*Task, asOfDay float64) EVMetrics {
	t.Helper()
	got, err := ComputeEVM(tasks, asOfDay)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Textbook scenario: A (4d, $400) feeds B (4d, $400). Status day 4:
// A planned done, B planned not started. A is only 75% complete and
// has cost $500 so far; B untouched.
func evmFixture(t *testing.T) map[string]*Task {
	t.Helper()
	tasks := map[string]*Task{
		"A": {ID: "A", Title: "Design", Duration: 4,
			BudgetedCost: 400, PercentComplete: 75, ActualCost: 500},
		"B": {ID: "B", Title: "Build", Duration: 4, Precedents: []string{"A"},
			BudgetedCost: 400},
	}
	mustCPM(t, tasks)
	return tasks
}

func TestComputeEVM_Totals(t *testing.T) {
	m := mustComputeEVM(t, evmFixture(t), 4)

	approx(t, "BAC", m.BAC, 800)
	approx(t, "PV", m.PV, 400) // A fully planned, B not started
	approx(t, "EV", m.EV, 300) // 75% of A's 400
	approx(t, "AC", m.AC, 500)
	approx(t, "SV", m.SV, -100) // behind schedule
	approx(t, "CV", m.CV, -200) // over cost
	approx(t, "SPI", m.SPI, 0.75)
	approx(t, "CPI", m.CPI, 0.6)
	approx(t, "EAC", m.EAC, 1333.33)
	approx(t, "ETC", m.ETC, 833.33)
	approx(t, "VAC", m.VAC, -533.33)
	if m.BACMinorUnits != 80000 || m.PVMinorUnits != 40000 || m.EVMinorUnits != 30000 || m.ACMinorUnits != 50000 {
		t.Fatalf("EVM minor units = BAC:%d PV:%d EV:%d AC:%d, want 80000/40000/30000/50000",
			m.BACMinorUnits, m.PVMinorUnits, m.EVMinorUnits, m.ACMinorUnits)
	}
	if m.EACMinorUnits != 133333 || m.ETCMinorUnits != 83333 || m.VACMinorUnits != -53333 {
		t.Fatalf("EAC/ETC/VAC minor units = %d/%d/%d, want 133333/83333/-53333",
			m.EACMinorUnits, m.ETCMinorUnits, m.VACMinorUnits)
	}
}

func TestComputeEVM_MidTaskPVIsLinear(t *testing.T) {
	m := mustComputeEVM(t, evmFixture(t), 6)
	// Day 6: A fully planned (400) + B halfway (200).
	approx(t, "PV", m.PV, 600)
}

func TestComputeEVM_SplitTaskPVPausesDuringGaps(t *testing.T) {
	tasks := map[string]*Task{
		"A": {
			ID:                     "A",
			Duration:               3,
			BudgetedCostMinorUnits: 30000,
			PlannedWorkSegments: []WorkSegment{
				{Start: 0, End: 1},
				{Start: 2, End: 3},
				{Start: 4, End: 5},
			},
		},
	}
	mustCPM(t, tasks)

	tests := []struct {
		name        string
		asOfDay     float64
		wantPVMinor int64
	}{
		{name: "mid first segment", asOfDay: 0.5, wantPVMinor: 5000},
		{name: "first gap plateau", asOfDay: 1.5, wantPVMinor: 10000},
		{name: "mid second segment", asOfDay: 2.5, wantPVMinor: 15000},
		{name: "second gap plateau", asOfDay: 3.5, wantPVMinor: 20000},
		{name: "complete", asOfDay: 5, wantPVMinor: 30000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustComputeEVM(t, tasks, tt.asOfDay)
			if got.PVMinorUnits != tt.wantPVMinor {
				t.Fatalf("PVMinorUnits = %d, want %d", got.PVMinorUnits, tt.wantPVMinor)
			}
		})
	}
}

func TestComputeEVM_ZeroDenominators(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2, BudgetedCost: 100},
	}
	mustCPM(t, tasks)
	m := mustComputeEVM(t, tasks, 0)

	// Nothing planned, earned, or spent yet.
	approx(t, "PV", m.PV, 0)
	approx(t, "SPI", m.SPI, 0) // n/a convention
	approx(t, "CPI", m.CPI, 0)
	approx(t, "EAC", m.EAC, 100) // falls back to BAC
}

func TestComputeEVM_MilestonePV(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2},
		"M": {ID: "M", Duration: 0, Precedents: []string{"A"},
			Milestone: true, BudgetedCost: 50},
	}
	mustCPM(t, tasks)

	before := mustComputeEVM(t, tasks, 1)
	approx(t, "PV before milestone", before.PV, 0)
	after := mustComputeEVM(t, tasks, 2)
	approx(t, "PV at milestone", after.PV, 50)
}

func TestComputeEVM_DeterministicTaskOrder(t *testing.T) {
	m := mustComputeEVM(t, evmFixture(t), 4)
	if len(m.Tasks) != 2 || m.Tasks[0].TaskID != "A" || m.Tasks[1].TaskID != "B" {
		t.Errorf("per-task breakdown not ID-ordered: %+v", m.Tasks)
	}
	approx(t, "Tasks[0].EV", m.Tasks[0].EV, 300)
}

func TestComputeEVM_UsesMinorUnitsForMoney(t *testing.T) {
	tasks := map[string]*Task{
		"A": {
			ID:                     "A",
			Title:                  "Fractional",
			Duration:               3,
			BudgetedCost:           999,
			BudgetedCostMinorUnits: 3333,
			ActualCost:             999,
			ActualCostMinorUnits:   2000,
			PercentComplete:        33.333333333333336,
		},
	}
	mustCPM(t, tasks)

	m := mustComputeEVM(t, tasks, 1)

	if m.BACMinorUnits != 3333 {
		t.Fatalf("BACMinorUnits = %d, want 3333", m.BACMinorUnits)
	}
	if m.PVMinorUnits != 1111 {
		t.Fatalf("PVMinorUnits = %d, want 1111", m.PVMinorUnits)
	}
	if m.EVMinorUnits != 1111 {
		t.Fatalf("EVMinorUnits = %d, want 1111", m.EVMinorUnits)
	}
	if m.ACMinorUnits != 2000 {
		t.Fatalf("ACMinorUnits = %d, want 2000", m.ACMinorUnits)
	}
	if m.EACMinorUnits != 6000 {
		t.Fatalf("EACMinorUnits = %d, want 6000", m.EACMinorUnits)
	}
	if len(m.Tasks) != 1 || m.Tasks[0].BACMinorUnits != 3333 || m.Tasks[0].PVMinorUnits != 1111 {
		t.Fatalf("task minor unit breakdown = %+v", m.Tasks)
	}
	approx(t, "BAC display", m.BAC, 33.33)
	approx(t, "PV display", m.PV, 11.11)
	approx(t, "EV display", m.EV, 11.11)
	approx(t, "AC display", m.AC, 20)
	approx(t, "EAC display", m.EAC, 60)
}

func TestComputeEVMAllowsExactCancellation(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", BudgetedCostMinorUnits: math.MaxInt64},
		"B": {ID: "B", BudgetedCostMinorUnits: 1},
		"C": {ID: "C", BudgetedCostMinorUnits: -1},
	}
	got := mustComputeEVM(t, tasks, -1)
	if got.BACMinorUnits != math.MaxInt64 {
		t.Fatalf("BAC = %d, want %d", got.BACMinorUnits, int64(math.MaxInt64))
	}
}

func TestComputeEVMReportsOverflow(t *testing.T) {
	tests := []struct {
		name  string
		tasks map[string]*Task
	}{
		{
			name: "aggregate",
			tasks: map[string]*Task{
				"A": {ID: "A", BudgetedCostMinorUnits: math.MaxInt64},
				"B": {ID: "B", BudgetedCostMinorUnits: 1},
			},
		},
		{
			name: "cost variance",
			tasks: map[string]*Task{
				"A": {ID: "A", BudgetedCostMinorUnits: math.MaxInt64, ActualCostMinorUnits: -1, PercentComplete: 100},
			},
		},
		{
			name: "estimate at completion",
			tasks: map[string]*Task{
				"A": {ID: "A", BudgetedCostMinorUnits: 2, ActualCostMinorUnits: math.MaxInt64, PercentComplete: 50},
			},
		},
		{
			// Planned value: two fully-planned (ES <= asOfDay) tasks at
			// BudgetedCostMinorUnits=MaxInt64 push the PV total past
			// int64 range, while a third task's negative budget keeps
			// the BAC total itself within range (MaxInt64-MaxInt64+
			// MaxInt64=MaxInt64, exactly at the boundary) so BAC's own
			// check passes and PV's is what actually fires.
			name: "planned value",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0, ES: -1, BudgetedCostMinorUnits: math.MaxInt64},
				"B": {ID: "B", Duration: 0, ES: 1000, BudgetedCostMinorUnits: -math.MaxInt64},
				"C": {ID: "C", Duration: 0, ES: -1, BudgetedCostMinorUnits: math.MaxInt64},
			},
		},
		{
			// Earned value: same shape as "planned value" but driven by
			// PercentComplete instead of ES, with every task's own PV
			// forced to 0 (ES in the future) so only the EV check fires.
			name: "earned value",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0, ES: 1000, BudgetedCostMinorUnits: math.MaxInt64, PercentComplete: 100},
				"B": {ID: "B", Duration: 0, ES: 1000, BudgetedCostMinorUnits: -math.MaxInt64, PercentComplete: 0},
				"C": {ID: "C", Duration: 0, ES: 1000, BudgetedCostMinorUnits: math.MaxInt64, PercentComplete: 100},
			},
		},
		{
			// Actual cost: ActualCostMinorUnits is an independent field
			// from BudgetedCost, so a zero budget on both tasks keeps
			// BAC/PV/EV at 0 (no overflow there) while AC's own
			// aggregation overflows on its own.
			name: "actual cost",
			tasks: map[string]*Task{
				"A": {ID: "A", ActualCostMinorUnits: math.MaxInt64},
				"B": {ID: "B", ActualCostMinorUnits: 1},
			},
		},
		{
			// Schedule variance (EV-PV): task A contributes PV=-1, EV=0;
			// task B contributes PV=0, EV=MaxInt64 (fully earned, not yet
			// planned) — individually-valid EV and PV whose difference
			// overflows.
			name: "schedule variance",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0, ES: -1, BudgetedCostMinorUnits: -1, PercentComplete: 0},
				"B": {ID: "B", Duration: 0, ES: 1000, BudgetedCostMinorUnits: math.MaxInt64, PercentComplete: 100},
			},
		},
		{
			// Estimate to complete (EAC-AC): task A has EV=0
			// (PercentComplete=0), forcing the EAC-falls-back-to-BAC
			// branch rather than the EV>0&&AC>0 ratio branch, so
			// EAC=BAC=MaxInt64 exactly (individually valid). Task B's
			// independent AC=-1 makes EAC-AC overflow.
			name: "estimate to complete",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 0, ES: -1, BudgetedCostMinorUnits: math.MaxInt64, PercentComplete: 0},
				"B": {ID: "B", Duration: 0, ES: -1, BudgetedCostMinorUnits: 0, ActualCostMinorUnits: -1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ComputeEVM(tt.tasks, -1)
			if !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("error = %v, want ErrOverflow", err)
			}
		})
	}
}
