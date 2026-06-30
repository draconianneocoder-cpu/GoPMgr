// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "testing"

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
	m := ComputeEVM(evmFixture(t), 4)

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
	m := ComputeEVM(evmFixture(t), 6)
	// Day 6: A fully planned (400) + B halfway (200).
	approx(t, "PV", m.PV, 600)
}

func TestComputeEVM_ZeroDenominators(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 2, BudgetedCost: 100},
	}
	mustCPM(t, tasks)
	m := ComputeEVM(tasks, 0)

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

	before := ComputeEVM(tasks, 1)
	approx(t, "PV before milestone", before.PV, 0)
	after := ComputeEVM(tasks, 2)
	approx(t, "PV at milestone", after.PV, 50)
}

func TestComputeEVM_DeterministicTaskOrder(t *testing.T) {
	m := ComputeEVM(evmFixture(t), 4)
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

	m := ComputeEVM(tasks, 1)

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
