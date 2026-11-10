// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
)

// Reference copies of the schedule pass and the Monte Carlo simulation as
// they were before the 2026-11-10 performance change, kept verbatim apart
// from their names. scheduler_equivalence_test.go checks that the current
// code produces bit-identical results. Do not "fix" these: they are the
// specification the faster code is held to.

// referenceIteration is the per-iteration result type before the change.
type referenceIteration struct {
	finish    float64
	critical  map[string]bool
	durations map[string]float64
}

func referenceCalculateCPM(tasks map[string]*Task) bool {
	order, ok := topoSort(tasks)
	if !ok {
		return false
	}

	// --- Forward pass: ES / EF ---
	for _, id := range order {
		t := tasks[id]
		t.ConstraintViolated = false
		es := 0.0
		for _, l := range effectiveLinks(t) {
			p, exists := tasks[l.Pred]
			if !exists {
				continue
			}
			var candidate float64
			switch l.Type {
			case StartToStart:
				candidate = p.ES + l.Lag
			case FinishToFinish:
				candidate = p.EF + l.Lag - t.Duration
			case StartToFinish:
				candidate = p.ES + l.Lag - t.Duration
			default: // FinishToStart
				candidate = p.EF + l.Lag
			}
			if candidate > es {
				es = candidate
			}
		}

		// Forward-acting constraints.
		if t.ConstraintArmed {
			switch t.Constraint {
			case StartNoEarlierThan:
				if t.ConstraintDay > es {
					es = t.ConstraintDay
				}
			case MustFinishOn:
				target := t.ConstraintDay - t.Duration
				if target >= es {
					es = target
				} else {
					// Links force a finish past the pinned date.
					t.ConstraintViolated = true
				}
			}
		}

		t.ES = es
		t.EF = t.ES + t.Duration
	}

	// Project finish = max EF across all tasks.
	projectEF := 0.0
	for _, t := range tasks {
		if t.EF > projectEF {
			projectEF = t.EF
		}
	}

	// Reverse adjacency: successors[id] = links leaving id, paired
	// with the successor task they enter. Used in the backward pass.
	type succLink struct {
		succ *Task
		typ  LinkType
		lag  float64
	}
	successors := make(map[string][]succLink, len(tasks))
	for _, t := range tasks {
		for _, l := range effectiveLinks(t) {
			if _, exists := tasks[l.Pred]; exists {
				successors[l.Pred] = append(successors[l.Pred],
					succLink{succ: t, typ: l.Type, lag: l.Lag})
			}
		}
	}

	// --- Backward pass: LF / LS ---
	// Walk the topo order in reverse so every task is visited AFTER its
	// successors.
	for i := len(order) - 1; i >= 0; i-- {
		t := tasks[order[i]]
		// Every task is bounded by the project finish; successor links
		// can only tighten that. (With SS links a predecessor's finish
		// is not constrained by its successor at all, and with leads an
		// FS candidate can exceed the project finish — in both cases
		// the projectEF bound is what keeps float meaningful.)
		t.LF = projectEF
		for _, sl := range successors[t.ID] {
			var candidate float64
			switch sl.typ {
			case StartToStart:
				candidate = sl.succ.LS - sl.lag + t.Duration
			case FinishToFinish:
				candidate = sl.succ.LF - sl.lag
			case StartToFinish:
				candidate = sl.succ.LF - sl.lag + t.Duration
			default: // FinishToStart
				candidate = sl.succ.LS - sl.lag
			}
			if candidate < t.LF {
				t.LF = candidate
			}
		}

		// Backward-acting constraints.
		if t.ConstraintArmed {
			switch t.Constraint {
			case MustFinishOn:
				if t.LF < t.ConstraintDay-1e-9 {
					// A successor needs this task finished before its
					// pinned date.
					t.ConstraintViolated = true
				}
				t.LF = t.ConstraintDay
			case FinishNoLaterThan:
				if t.ConstraintDay < t.LF {
					t.LF = t.ConstraintDay
				}
				if t.EF > t.ConstraintDay+1e-9 {
					t.ConstraintViolated = true
				}
			}
		}

		t.LS = t.LF - t.Duration
		t.Float = t.LS - t.ES
		// Use a tiny epsilon to absorb floating-point noise. Float ~ 0
		// defines critical; a NEGATIVE float (possible when FNLT/MFO
		// constraints squeeze the schedule) is super-critical and is
		// flagged too.
		t.IsCritical = t.Float < 1e-9
	}

	// Clamp progress to its valid range (defensive against raw UI
	// input; reporting-only, so this never moves dates).
	for _, t := range tasks {
		if t.PercentComplete < 0 {
			t.PercentComplete = 0
		} else if t.PercentComplete > 100 {
			t.PercentComplete = 100
		}
	}

	// --- ALAP post-pass ---
	// As-late-as-possible tasks sit at their late dates, consuming
	// their own float. Applied after both passes so no other task's
	// dates shift (successors were computed from the early dates).
	for _, t := range tasks {
		if t.Constraint == AsLateAsPossible {
			t.ES, t.EF = t.LS, t.LF
			t.Float = 0
			t.IsCritical = true
		}
	}

	return true
}

func referenceRunMonteCarlo(tasks map[string]*Task, iterations int, workers int) SimResult {
	result := SimResult{
		Iterations:            iterations,
		CriticalPathFrequency: map[string]float64{},
		DurationPercentiles:   map[string][3]float64{},
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	if workers > iterations && iterations > 0 {
		workers = iterations
	}
	if workers > maxMonteCarloWorkers {
		workers = maxMonteCarloWorkers
	}
	result.Workers = workers

	taskIDs, err := validateMonteCarloInputs(tasks, iterations)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if len(taskIDs) == 0 {
		result.Valid = true
		return result
	}

	jobs := make(chan int)
	results := make(chan referenceIteration, workers)

	for w := 0; w < workers; w++ {
		go func() {
			for iteration := range jobs {
				results <- referenceMonteCarloIteration(tasks, taskIDs, iteration)
			}
		}()
	}

	go func() {
		for iteration := 0; iteration < iterations; iteration++ {
			jobs <- iteration
		}
		close(jobs)
	}()

	finishes := make([]float64, 0, iterations)
	criticalCounts := make(map[string]int, len(taskIDs))
	durationSamples := make(map[string][]float64, len(taskIDs))
	for _, id := range taskIDs {
		durationSamples[id] = make([]float64, 0, iterations)
	}

	for i := 0; i < iterations; i++ {
		iter := <-results
		finishes = append(finishes, iter.finish)
		for id, critical := range iter.critical {
			if critical {
				criticalCounts[id]++
			}
		}
		for id, duration := range iter.durations {
			durationSamples[id] = append(durationSamples[id], duration)
		}
	}

	result.P50 = referencePercentile(finishes, 0.50)
	result.P80 = referencePercentile(finishes, 0.80)
	result.P90 = referencePercentile(finishes, 0.90)
	result.FinishCDF = referenceFinishCDF(finishes)
	for _, id := range taskIDs {
		result.CriticalPathFrequency[id] = float64(criticalCounts[id]) / float64(iterations)
		samples := durationSamples[id]
		result.DurationPercentiles[id] = [3]float64{
			referencePercentile(samples, 0.50),
			referencePercentile(samples, 0.80),
			referencePercentile(samples, 0.90),
		}
	}
	result.TornadoDrivers = tornadoDrivers(taskIDs, result.CriticalPathFrequency, result.DurationPercentiles)
	result.Valid = true
	return result
}

func referenceMonteCarloIteration(tasks map[string]*Task, taskIDs []string, iteration int) referenceIteration {
	// #nosec G115 -- iteration is bounded [0, maxMonteCarloIterations] by
	// validateMonteCarloInputs, so the int->uint64 conversion cannot
	// overflow; and this seeds a statistical PRNG, not a security boundary.
	src := rand.NewPCG(monteCarloSeedA+uint64(iteration)*0x9e3779b97f4a7c15, monteCarloSeedB^uint64(iteration))
	sampled := make(map[string]*Task, len(tasks))
	durations := make(map[string]float64, len(taskIDs))

	for _, id := range taskIDs {
		original := tasks[id]
		task := cloneTaskForSimulation(original)
		task.Duration = sampleDuration(original, src)
		durations[id] = task.Duration
		sampled[id] = task
	}

	referenceCalculateCPM(sampled)
	critical := make(map[string]bool, len(taskIDs))
	finish := 0.0
	for _, id := range taskIDs {
		task := sampled[id]
		if task.EF > finish {
			finish = task.EF
		}
		critical[id] = task.IsCritical
	}
	return referenceIteration{
		finish:    finish,
		critical:  critical,
		durations: durations,
	}
}

func referencePercentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := q * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func referenceFinishCDF(finishes []float64) []ProbabilityPoint {
	const points = 21
	if len(finishes) == 0 {
		return nil
	}
	curve := make([]ProbabilityPoint, 0, points)
	for i := 0; i < points; i++ {
		probability := float64(i) / float64(points-1)
		curve = append(curve, ProbabilityPoint{
			Day:         referencePercentile(finishes, probability),
			Probability: probability,
		})
	}
	return curve
}
