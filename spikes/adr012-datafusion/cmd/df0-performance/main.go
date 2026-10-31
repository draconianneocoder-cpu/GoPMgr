// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb && (darwin || linux)

// Command df0-performance characterizes fresh portfolio backends. It does not
// qualify native UI latency, cold caches, warm workers, or performance budgets.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"gopmgr/internal/analytics"
	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/gorollup"
	"gopmgr/spikes/adr012-datafusion/internal/ipchost"
	"gopmgr/spikes/adr012-datafusion/internal/oracle"
	"gopmgr/spikes/adr012-datafusion/internal/rollupwire"
)

var providers = []string{"duckdb", "rust", "go"}

type sample struct {
	Provider       string `json:"provider"`
	Case           string `json:"case"`
	Phase          string `json:"phase"`
	Round          int    `json:"round"`
	WallNS         int64  `json:"wall_ns"`
	HostCPUNS      int64  `json:"host_cpu_ns"`
	WorkerCPUNS    *int64 `json:"worker_cpu_ns"`
	WorkerPeakRSS  *int64 `json:"worker_peak_rss_bytes"`
	SpawnToHelloNS *int64 `json:"spawn_to_hello_ns"`
	SourceBytes    int    `json:"worker_source_bytes"`
	OutputBytes    int    `json:"backend_json_bytes"`
}

type stats struct {
	N           int     `json:"n"`
	MinNS       int64   `json:"min_ns"`
	P50NS       int64   `json:"p50_ns"`
	P95NS       int64   `json:"p95_ns"`
	P99NS       *int64  `json:"p99_ns"`
	MaxNS       int64   `json:"max_ns"`
	MeanNS      float64 `json:"mean_ns"`
	VarianceNS2 float64 `json:"population_variance_ns2"`
}

func main() {
	worker := flag.String("worker", "", "reviewed native df0-worker executable (required)")
	n := flag.Int("samples", 30, "measured rounds after first observation and three warm-ups (minimum 30)")
	flag.Parse()
	if err := run(os.Stdout, *worker, *n); err != nil {
		fmt.Fprintln(os.Stderr, "df0-performance:", err)
		os.Exit(1)
	}
}

func selectedCases() []fixtures.PortfolioCase {
	var out []fixtures.PortfolioCase
	for _, c := range fixtures.PortfolioCases() {
		if c.ID == "portfolio-empty" || c.ID == "portfolio-scale-100" || c.ID == "portfolio-scale-1000" {
			out = append(out, c)
		}
	}
	return out
}

func run(out io.Writer, worker string, n int) error {
	if n < 30 {
		return errors.New("at least 30 measured samples per provider/case are required")
	}
	if worker == "" {
		return errors.New("-worker is required")
	}
	cases := selectedCases()
	if len(cases) != 3 {
		return fmt.Errorf("expected three portfolio fixtures, got %d", len(cases))
	}
	enc := json.NewEncoder(out)
	inputs := make(map[string]map[string]string)
	for _, c := range cases {
		b, err := fixtures.EncodePortfolio(c)
		if err != nil {
			return err
		}
		want, _ := oracle.Rollup(c.Projects)
		gold, err := json.Marshal(want.Summary)
		if err != nil {
			return err
		}
		inputs[c.ID] = map[string]string{"fixture_sha256": fmt.Sprintf("%x", sha256.Sum256(b)), "oracle_sha256": fmt.Sprintf("%x", sha256.Sum256(gold))}
	}
	if err := enc.Encode(map[string]any{"type": "metadata", "scope": "partial fresh-backend characterization; no acceptance verdict",
		"os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "gomaxprocs": runtime.GOMAXPROCS(0),
		"measured_rounds": n, "warmup_rounds": 3, "shuffle_seed": []int{12, 31}, "inputs": inputs}); err != nil {
		return err
	}
	rng := rand.New(rand.NewPCG(12, 31)) // deterministic scheduling, not security material
	type task struct {
		c        fixtures.PortfolioCase
		provider string
	}
	var tasks []task
	for _, c := range cases {
		for _, p := range providers {
			tasks = append(tasks, task{c, p})
		}
	}
	times := make(map[string][]int64)
	for round := -4; round < n; round++ {
		rng.Shuffle(len(tasks), func(i, j int) { tasks[i], tasks[j] = tasks[j], tasks[i] })
		for _, task := range tasks {
			s, err := measure(task.provider, worker, task.c)
			if err != nil {
				return fmt.Errorf("%s/%s round %d: %w", task.c.ID, task.provider, round, err)
			}
			s.Phase, s.Round = "measured", round
			if round == -4 {
				s.Phase = "first_observation"
			} else if round < 0 {
				s.Phase = "warmup"
			}
			if err := enc.Encode(map[string]any{"type": "sample", "sample": s}); err != nil {
				return err
			}
			if round >= 0 {
				key := s.Case + "/" + s.Provider
				times[key] = append(times[key], s.WallNS)
			}
		}
	}
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return err
	}
	// This shared process also links DuckDB when measuring the other providers.
	// Its lifetime peak is not attributable to a query/provider or additive to child peaks.
	peak := usage.Maxrss
	if runtime.GOOS == "linux" {
		peak *= 1024
	}
	summaries := make(map[string]stats)
	for key, values := range times {
		summaries[key] = distribution(values)
	}
	return enc.Encode(map[string]any{"type": "summary", "wall": summaries, "shared_host_lifetime_peak_rss_bytes": peak,
		"p99_status": "not estimated; characterization only, not the specified 1000-sample/multi-process qualification"})
}

func measure(provider, worker string, c fixtures.PortfolioCase) (sample, error) {
	s := sample{Provider: provider, Case: c.ID}
	before, err := hostCPU()
	if err != nil {
		return s, err
	}
	start := time.Now()
	var got analytics.PortfolioSummary
	switch provider {
	case "duckdb":
		eng := analytics.New()
		got, err = eng.PortfolioRollup(context.Background(), toMetrics(c.Projects))
		err = errors.Join(err, eng.Close())
	case "go":
		got, err = gorollup.Rollup(toMetrics(c.Projects))
	case "rust":
		norm := make([]map[string]any, len(c.Projects))
		for i, p := range c.Projects {
			norm[i] = rollupwire.Normalise(p)
		}
		var src []byte
		src, err = json.Marshal(norm)
		if err != nil {
			break
		}
		s.SourceBytes = len(src)
		r := ipchost.Run(ipchost.Job{Name: "perf-" + c.ID, Worker: worker, Source: src,
			Execute: &df0pb.Execute{Operation: "rollup"}, Timeout: 10 * time.Second})
		if err = checkWorker(r); err != nil {
			break
		}
		cpu := r.WorkerUserNS + r.WorkerSystemNS
		s.WorkerCPUNS, s.WorkerPeakRSS, s.SpawnToHelloNS = &cpu, &r.WorkerPeakRSS, &r.SpawnToHelloNS
		var result rollupwire.Result
		if err = json.Unmarshal([]byte(r.ResultJSON), &result); err != nil {
			break
		}
		if result.Error != "" {
			err = errors.New(result.Error)
			break
		}
		got, err = rollupwire.Summary(result)
	default:
		err = fmt.Errorf("unknown provider %q", provider)
	}
	if err != nil {
		return s, err
	}
	// Common backend serialization cost only, not the production Wails wire DTO.
	b, err := json.Marshal(got)
	if err != nil {
		return s, err
	}
	s.WallNS, s.OutputBytes = time.Since(start).Nanoseconds(), len(b)
	after, err := hostCPU()
	if err != nil {
		return s, err
	}
	s.HostCPUNS = after - before
	return s, checkSummary(got, c) // independent oracle is outside the timed region
}

func checkWorker(r ipchost.Result) error {
	if r.Outcome != "completed" || r.WorkerExit != "exit 0" || r.Killed || !r.EndpointRemoved ||
		r.DigestMatch == nil || !*r.DigestMatch || r.SourceMatch == nil || !*r.SourceMatch ||
		r.WorkerPeakRSS <= 0 || r.WorkerUserNS < 0 || r.WorkerSystemNS < 0 {
		return fmt.Errorf("worker completion/resources/cleanup failed: %s, %s, %s", r.Outcome, r.WorkerExit, r.Detail)
	}
	return nil
}

func checkSummary(got analytics.PortfolioSummary, c fixtures.PortfolioCase) error {
	want, _ := oracle.Rollup(c.Projects)
	if want.Err != "" {
		return fmt.Errorf("unexpected oracle error: %s", want.Err)
	}
	if why := oracle.CompareSummary(got, want.Summary); len(why) != 0 {
		return errors.New(strings.Join(why, "; "))
	}
	return nil
}

func hostCPU() (int64, error) {
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		return 0, err
	}
	return r.Utime.Nano() + r.Stime.Nano(), nil
}

func toMetrics(ps []fixtures.Project) []analytics.ProjectMetrics {
	out := make([]analytics.ProjectMetrics, len(ps))
	for i, p := range ps {
		out[i] = analytics.ProjectMetrics{ProjectID: p.ProjectID, Name: p.Name,
			BudgetedCost: p.BudgetedCost, CommittedCost: p.CommittedCost, ActualCost: p.ActualCost,
			EarnedValue: p.EarnedValue, PlannedValue: p.PlannedValue,
			BudgetedCostMinorUnits: p.BudgetedCostMinorUnits, CommittedCostMinorUnits: p.CommittedCostMinorUnits,
			ActualCostMinorUnits: p.ActualCostMinorUnits, EarnedValueMinorUnits: p.EarnedValueMinorUnits,
			PlannedValueMinorUnits: p.PlannedValueMinorUnits, PercentComplete: p.PercentComplete, EVMAvailable: p.EVMAvailable}
	}
	return out
}

func distribution(values []int64) stats {
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	d := stats{N: len(ordered), MinNS: ordered[0], MaxNS: ordered[len(ordered)-1],
		P50NS: ordered[int(math.Ceil(float64(len(ordered))*.50))-1], P95NS: ordered[int(math.Ceil(float64(len(ordered))*.95))-1]}
	for _, v := range ordered {
		d.MeanNS += float64(v)
	}
	d.MeanNS /= float64(d.N)
	for _, v := range ordered {
		delta := float64(v) - d.MeanNS
		d.VarianceNS2 += delta * delta
	}
	d.VarianceNS2 /= float64(d.N)
	return d
}
