// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
	approx(t, "EAC", m.EAC, 800/0.6)
	approx(t, "ETC", m.ETC, 800/0.6-500)
	approx(t, "VAC", m.VAC, 800-800/0.6)
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
