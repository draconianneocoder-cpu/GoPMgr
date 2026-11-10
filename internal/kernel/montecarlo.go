// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"strings"
	"sync"

	"gonum.org/v1/gonum/stat/distuv"
)

const (
	MonteCarloTriangular DistributionName = "triangular"
	MonteCarloBetaPERT   DistributionName = "beta-pert"
	MonteCarloNormal     DistributionName = "normal"

	monteCarloSeedA uint64 = 0x504d466f72676531
	monteCarloSeedB uint64 = 0x6b65726e656c4d43

	// maxMonteCarloIterations bounds a single simulation. Memory is
	// O(iterations x tasks) — finishes plus per-task duration samples are
	// preallocated with `iterations` capacity — so an unbounded value from a
	// caller would OOM or panic in makeslice. The GUI already clamps to
	// 10,000; this backend ceiling is the real guard for any caller (CLI, a
	// compromised webview) that does not.
	maxMonteCarloIterations = 100_000

	// maxMonteCarloWorkers caps the goroutine fan-out. Results are
	// deterministic per iteration index regardless of worker count, so
	// clamping is invisible; it only prevents a huge `workers` argument from
	// spawning that many goroutines.
	maxMonteCarloWorkers = 128
)

// DistributionName names a supported Monte Carlo duration
// distribution.
type DistributionName string

// DurationEstimate is a three-point duration estimate for
// probabilistic scheduling. Optimistic, MostLikely, and Pessimistic
// are working-day durations. Distribution defaults to triangular.
type DurationEstimate struct {
	Optimistic   float64          `json:"optimistic,omitempty"`
	MostLikely   float64          `json:"most_likely,omitempty"`
	Pessimistic  float64          `json:"pessimistic,omitempty"`
	Distribution DistributionName `json:"distribution,omitempty"`
}

// Empty reports whether the estimate should fall back to Task.Duration.
func (e DurationEstimate) Empty() bool {
	return e.Optimistic == 0 && e.MostLikely == 0 && e.Pessimistic == 0 && e.Distribution == ""
}

// ProbabilityPoint is one cumulative probability point for a sampled
// finish-day curve.
type ProbabilityPoint struct {
	Day         float64 `json:"day"`
	Probability float64 `json:"probability"`
}

// TornadoDriver ranks a task's schedule-risk contribution for compact
// tornado chart rendering.
type TornadoDriver struct {
	TaskID            string  `json:"task_id"`
	CriticalFrequency float64 `json:"critical_frequency"`
	P50Duration       float64 `json:"p50_duration"`
	P80Duration       float64 `json:"p80_duration"`
	P90Duration       float64 `json:"p90_duration"`
	DurationSpread    float64 `json:"duration_spread"`
	Score             float64 `json:"score"`
}

// SimResult is the output of a Monte Carlo schedule simulation. P50,
// P80, and P90 are finish-day percentiles. FinishCDF stores a compact
// cumulative finish-day curve for S-curve rendering. DurationPercentiles stores
// each task's sampled P50/P80/P90 durations in that order. TornadoDrivers ranks
// the top schedule-risk drivers by critical-path frequency multiplied by
// P90-P50 duration spread.
type SimResult struct {
	Valid                 bool                  `json:"valid"`
	Error                 string                `json:"error,omitempty"`
	Iterations            int                   `json:"iterations"`
	Workers               int                   `json:"workers"`
	P50                   float64               `json:"p50"`
	P80                   float64               `json:"p80"`
	P90                   float64               `json:"p90"`
	FinishCDF             []ProbabilityPoint    `json:"finish_cdf"`
	CriticalPathFrequency map[string]float64    `json:"critical_path_frequency"`
	DurationPercentiles   map[string][3]float64 `json:"duration_percentiles"`
	TornadoDrivers        []TornadoDriver       `json:"tornado_drivers"`
}

// RunMonteCarlo samples task durations, runs CPM for each sampled
// network, and aggregates finish percentiles plus critical-path
// frequency. Sampling is deterministic by iteration index, so results
// are stable regardless of worker count.
func RunMonteCarlo(tasks map[string]*Task, iterations int, workers int) SimResult {
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

	// Each iteration writes only its own slot in finishes and samples, so
	// workers share them without locking. Critical-path counts are kept per
	// worker and summed after every worker has finished.
	finishes := make([]float64, iterations)
	samples := make([][]float64, len(taskIDs))
	for k := range samples {
		samples[k] = make([]float64, iterations)
	}
	criticalCounts := make([][]int, workers)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		sim := newMonteCarloWorker(tasks, taskIDs)
		criticalCounts[w] = sim.criticalCounts
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range jobs {
				finishes[iteration] = sim.run(iteration, samples)
			}
		}()
	}
	for iteration := 0; iteration < iterations; iteration++ {
		jobs <- iteration
	}
	close(jobs)
	wg.Wait()

	sort.Float64s(finishes)
	result.P50 = sortedPercentile(finishes, 0.50)
	result.P80 = sortedPercentile(finishes, 0.80)
	result.P90 = sortedPercentile(finishes, 0.90)
	result.FinishCDF = finishCDF(finishes)
	for k, id := range taskIDs {
		critical := 0
		for _, counts := range criticalCounts {
			critical += counts[k]
		}
		result.CriticalPathFrequency[id] = float64(critical) / float64(iterations)
		sort.Float64s(samples[k])
		result.DurationPercentiles[id] = [3]float64{
			sortedPercentile(samples[k], 0.50),
			sortedPercentile(samples[k], 0.80),
			sortedPercentile(samples[k], 0.90),
		}
	}
	result.TornadoDrivers = tornadoDrivers(taskIDs, result.CriticalPathFrequency, result.DurationPercentiles)
	result.Valid = true
	return result
}

func validateMonteCarloInputs(tasks map[string]*Task, iterations int) ([]string, error) {
	if iterations <= 0 {
		return nil, fmt.Errorf("monte carlo: iterations must be positive")
	}
	if iterations > maxMonteCarloIterations {
		return nil, fmt.Errorf("monte carlo: iterations %d exceeds maximum %d", iterations, maxMonteCarloIterations)
	}
	ids := make([]string, 0, len(tasks))
	for id, task := range tasks {
		if task == nil {
			return nil, fmt.Errorf("monte carlo: task %q is nil", id)
		}
		if task.ID == "" {
			return nil, fmt.Errorf("monte carlo: task key %q has empty task id", id)
		}
		if task.ID != id {
			return nil, fmt.Errorf("monte carlo: task key %q does not match task id %q", id, task.ID)
		}
		if task.Duration < 0 {
			return nil, fmt.Errorf("monte carlo: task %q has negative duration", task.ID)
		}
		if err := validateDurationEstimate(task); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if err := checkTaskLimits(tasks); err != nil {
		return nil, fmt.Errorf("monte carlo: %w", err)
	}
	if _, ok := topoSort(tasks); !ok {
		return nil, fmt.Errorf("monte carlo: task graph contains a cycle")
	}
	return ids, nil
}

func validateDurationEstimate(task *Task) error {
	e := task.DurationEstimate
	if e.Empty() {
		return nil
	}
	distribution := normalizedDistribution(e.Distribution)
	switch distribution {
	case MonteCarloTriangular, MonteCarloBetaPERT, MonteCarloNormal:
	default:
		return fmt.Errorf("monte carlo: task %q has unsupported distribution %q", task.ID, e.Distribution)
	}
	if e.Optimistic < 0 || e.MostLikely < 0 || e.Pessimistic < 0 {
		return fmt.Errorf("monte carlo: task %q has negative duration estimate", task.ID)
	}
	if e.Optimistic > e.MostLikely || e.MostLikely > e.Pessimistic {
		return fmt.Errorf("monte carlo: task %q estimate must satisfy optimistic <= most likely <= pessimistic", task.ID)
	}
	return nil
}

// monteCarloWorker reruns the schedule pass on its own copy of the
// network. The copy and its plan are built once; each iteration only sets
// new durations, because the pass recomputes every output from them.
type monteCarloWorker struct {
	originals      []*Task // by taskIDs index, for sampling
	clones         []*Task // by taskIDs index, scheduled each iteration
	plan           *cpmPlan
	criticalCounts []int // by taskIDs index
}

func newMonteCarloWorker(tasks map[string]*Task, taskIDs []string) *monteCarloWorker {
	sim := &monteCarloWorker{
		originals:      make([]*Task, len(taskIDs)),
		clones:         make([]*Task, len(taskIDs)),
		criticalCounts: make([]int, len(taskIDs)),
	}
	network := make(map[string]*Task, len(taskIDs))
	for k, id := range taskIDs {
		sim.originals[k] = tasks[id]
		sim.clones[k] = cloneTaskForSimulation(tasks[id])
		network[id] = sim.clones[k]
	}
	// validateMonteCarloInputs has already rejected a cyclic network.
	sim.plan, _ = newCPMPlan(network)
	return sim
}

// run samples and schedules one iteration, records each task's sampled
// duration in samples[k][iteration], counts critical tasks, and returns
// the project finish. Sampling depends only on the iteration index, so the
// result does not depend on which worker runs it.
func (sim *monteCarloWorker) run(iteration int, samples [][]float64) float64 {
	// #nosec G115 -- iteration is bounded [0, maxMonteCarloIterations] by
	// validateMonteCarloInputs, so the int->uint64 conversion cannot
	// overflow; and this seeds a statistical PRNG, not a security boundary.
	src := rand.NewPCG(monteCarloSeedA+uint64(iteration)*0x9e3779b97f4a7c15, monteCarloSeedB^uint64(iteration))
	for k, original := range sim.originals {
		duration := sampleDuration(original, src)
		sim.clones[k].Duration = duration
		samples[k][iteration] = duration
	}
	sim.plan.run()
	finish := 0.0
	for k, task := range sim.clones {
		if task.EF > finish {
			finish = task.EF
		}
		if task.IsCritical {
			sim.criticalCounts[k]++
		}
	}
	return finish
}

func cloneTaskForSimulation(original *Task) *Task {
	task := *original
	task.Precedents = append([]string(nil), original.Precedents...)
	task.Links = append([]Link(nil), original.Links...)
	task.Assignments = append([]Assignment(nil), original.Assignments...)
	return &task
}

func sampleDuration(task *Task, src rand.Source) float64 {
	e := task.DurationEstimate
	if e.Empty() {
		return task.Duration
	}
	if e.Optimistic == e.Pessimistic {
		return e.Optimistic
	}

	switch normalizedDistribution(e.Distribution) {
	case MonteCarloBetaPERT:
		scale := e.Pessimistic - e.Optimistic
		alpha := 1 + 4*((e.MostLikely-e.Optimistic)/scale)
		beta := 1 + 4*((e.Pessimistic-e.MostLikely)/scale)
		sample := distuv.Beta{Alpha: alpha, Beta: beta, Src: src}.Rand()
		return e.Optimistic + sample*scale
	case MonteCarloNormal:
		sigma := (e.Pessimistic - e.Optimistic) / 6
		sample := distuv.Normal{Mu: e.MostLikely, Sigma: sigma, Src: src}.Rand()
		return clamp(sample, e.Optimistic, e.Pessimistic)
	default:
		return distuv.NewTriangle(e.Optimistic, e.Pessimistic, e.MostLikely, src).Rand()
	}
}

func normalizedDistribution(name DistributionName) DistributionName {
	normalized := DistributionName(strings.ToLower(strings.TrimSpace(string(name))))
	if normalized == "" {
		return MonteCarloTriangular
	}
	return normalized
}

// sortedPercentile interpolates the q-quantile of values, which must be
// sorted in ascending order.
func sortedPercentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
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

// finishCDF expects finishes sorted in ascending order.
func finishCDF(finishes []float64) []ProbabilityPoint {
	const points = 21
	if len(finishes) == 0 {
		return nil
	}
	curve := make([]ProbabilityPoint, 0, points)
	for i := 0; i < points; i++ {
		probability := float64(i) / float64(points-1)
		curve = append(curve, ProbabilityPoint{
			Day:         sortedPercentile(finishes, probability),
			Probability: probability,
		})
	}
	return curve
}

func tornadoDrivers(taskIDs []string, frequencies map[string]float64, durations map[string][3]float64) []TornadoDriver {
	const maxDrivers = 10

	drivers := make([]TornadoDriver, 0, len(taskIDs))
	for _, id := range taskIDs {
		duration := durations[id]
		spread := duration[2] - duration[0]
		frequency := frequencies[id]
		drivers = append(drivers, TornadoDriver{
			TaskID:            id,
			CriticalFrequency: frequency,
			P50Duration:       duration[0],
			P80Duration:       duration[1],
			P90Duration:       duration[2],
			DurationSpread:    spread,
			Score:             frequency * spread,
		})
	}
	sort.SliceStable(drivers, func(i, j int) bool {
		if drivers[i].Score != drivers[j].Score {
			return drivers[i].Score > drivers[j].Score
		}
		if drivers[i].CriticalFrequency != drivers[j].CriticalFrequency {
			return drivers[i].CriticalFrequency > drivers[j].CriticalFrequency
		}
		if drivers[i].DurationSpread != drivers[j].DurationSpread {
			return drivers[i].DurationSpread > drivers[j].DurationSpread
		}
		return drivers[i].TaskID < drivers[j].TaskID
	})
	if len(drivers) > maxDrivers {
		return drivers[:maxDrivers]
	}
	return drivers
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
