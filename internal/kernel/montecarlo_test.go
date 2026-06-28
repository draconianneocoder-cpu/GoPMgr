// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "testing"

func TestRunMonteCarloUsesDeterministicDurations(t *testing.T) {
	tasks := map[string]*Task{}
	mkTask(tasks, "A", 2)
	mkTask(tasks, "B", 3, "A")

	result := RunMonteCarlo(tasks, 8, 2)

	if !result.Valid {
		t.Fatalf("RunMonteCarlo returned invalid result: %s", result.Error)
	}
	near(t, "P50", result.P50, 5)
	near(t, "P80", result.P80, 5)
	near(t, "P90", result.P90, 5)
	near(t, "A critical frequency", result.CriticalPathFrequency["A"], 1)
	near(t, "B critical frequency", result.CriticalPathFrequency["B"], 1)
	if got := result.DurationPercentiles["A"]; got != [3]float64{2, 2, 2} {
		t.Fatalf("A duration percentiles = %v, want [2 2 2]", got)
	}
	if tasks["A"].EF != 0 || tasks["B"].EF != 0 {
		t.Fatal("RunMonteCarlo mutated the source task schedule")
	}
}

func TestRunMonteCarloIsStableAcrossWorkerCounts(t *testing.T) {
	tasks := map[string]*Task{
		"A": {
			ID: "A",
			DurationEstimate: DurationEstimate{
				Optimistic:   1,
				MostLikely:   2,
				Pessimistic:  4,
				Distribution: "triangular",
			},
		},
		"B": {
			ID:         "B",
			Precedents: []string{"A"},
			DurationEstimate: DurationEstimate{
				Optimistic:   2,
				MostLikely:   4,
				Pessimistic:  8,
				Distribution: "beta-pert",
			},
		},
	}

	serial := RunMonteCarlo(tasks, 250, 1)
	parallel := RunMonteCarlo(tasks, 250, 4)

	if !serial.Valid {
		t.Fatalf("serial RunMonteCarlo invalid: %s", serial.Error)
	}
	if !parallel.Valid {
		t.Fatalf("parallel RunMonteCarlo invalid: %s", parallel.Error)
	}
	near(t, "P50", parallel.P50, serial.P50)
	near(t, "P80", parallel.P80, serial.P80)
	near(t, "P90", parallel.P90, serial.P90)
	near(t, "A P90 duration", parallel.DurationPercentiles["A"][2], serial.DurationPercentiles["A"][2])
	near(t, "B P90 duration", parallel.DurationPercentiles["B"][2], serial.DurationPercentiles["B"][2])
}

func TestRunMonteCarloReportsBranchCriticalPathFrequency(t *testing.T) {
	tasks := map[string]*Task{
		"A": {ID: "A", Duration: 4},
		"B": {
			ID: "B",
			DurationEstimate: DurationEstimate{
				Optimistic:   1,
				MostLikely:   5,
				Pessimistic:  9,
				Distribution: "triangular",
			},
		},
		"Finish": {ID: "Finish", Precedents: []string{"A", "B"}},
	}

	result := RunMonteCarlo(tasks, 500, 3)

	if !result.Valid {
		t.Fatalf("RunMonteCarlo returned invalid result: %s", result.Error)
	}
	if got := result.CriticalPathFrequency["A"]; got <= 0 || got >= 1 {
		t.Fatalf("A critical frequency = %v, want a partial branch frequency", got)
	}
	if got := result.CriticalPathFrequency["B"]; got <= 0 || got >= 1 {
		t.Fatalf("B critical frequency = %v, want a partial branch frequency", got)
	}
	near(t, "Finish critical frequency", result.CriticalPathFrequency["Finish"], 1)
}

func TestRunMonteCarloRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		tasks      map[string]*Task
		iterations int
	}{
		{
			name:       "zero iterations",
			tasks:      map[string]*Task{"A": {ID: "A", Duration: 1}},
			iterations: 0,
		},
		{
			name: "invalid estimate ordering",
			tasks: map[string]*Task{
				"A": {
					ID: "A",
					DurationEstimate: DurationEstimate{
						Optimistic:  5,
						MostLikely:  3,
						Pessimistic: 8,
					},
				},
			},
			iterations: 10,
		},
		{
			name: "cycle",
			tasks: map[string]*Task{
				"A": {ID: "A", Duration: 1, Precedents: []string{"B"}},
				"B": {ID: "B", Duration: 1, Precedents: []string{"A"}},
			},
			iterations: 10,
		},
		{
			name: "map key mismatch",
			tasks: map[string]*Task{
				"wrong": {ID: "A", Duration: 1},
			},
			iterations: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunMonteCarlo(tt.tasks, tt.iterations, 1)
			if result.Valid {
				t.Fatal("RunMonteCarlo returned valid result for invalid input")
			}
			if result.Error == "" {
				t.Fatal("RunMonteCarlo did not include an error message")
			}
		})
	}
}
