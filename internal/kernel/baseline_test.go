// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "testing"

func TestCompareSchedules(t *testing.T) {
	baseline := map[string]*Task{
		"A": {ID: "A", Duration: 2},
		"B": {ID: "B", Duration: 3, Precedents: []string{"A"}},
	}
	mustCPM(t, baseline)

	// Current plan: A grew by 2 days, pushing B.
	current := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {ID: "B", Duration: 3, Precedents: []string{"A"}},
		"C": {ID: "C", Duration: 1}, // new task, not in baseline
	}
	mustCPM(t, current)

	vars := CompareSchedules(current, baseline)

	a, ok := vars["A"]
	if !ok {
		t.Fatal("variance for A missing")
	}
	approx(t, "A.StartVarDays", a.StartVarDays, 0)
	approx(t, "A.FinishVarDays", a.FinishVarDays, 2)

	b := vars["B"]
	approx(t, "B.StartVarDays", b.StartVarDays, 2)
	approx(t, "B.FinishVarDays", b.FinishVarDays, 2)

	if _, present := vars["C"]; present {
		t.Error("task absent from the baseline must be skipped")
	}
}

func TestCompareSchedulesCarriesBaselineDates(t *testing.T) {
	baseline := map[string]*Task{
		"A": {ID: "A", Duration: 1, StartDate: "2026-06-01", FinishDate: "2026-06-01"},
	}
	current := map[string]*Task{
		"A": {ID: "A", Duration: 1, ES: 2, EF: 3},
	}
	vars := CompareSchedules(current, baseline)
	if vars["A"].BaselineStart != "2026-06-01" {
		t.Errorf("BaselineStart = %q, want 2026-06-01", vars["A"].BaselineStart)
	}
	approx(t, "A.StartVarDays", vars["A"].StartVarDays, 2)
}

func TestProgressClampedByCPM(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 1, PercentComplete: 150},
		"B": {ID: "B", Duration: 1, PercentComplete: -10},
	}
	mustCPM(t, tasks)
	approx(t, "A.PercentComplete", tasks["A"].PercentComplete, 100)
	approx(t, "B.PercentComplete", tasks["B"].PercentComplete, 0)
}
