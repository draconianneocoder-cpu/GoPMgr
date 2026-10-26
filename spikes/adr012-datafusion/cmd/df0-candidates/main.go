// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command df0-candidates scores ADR-012 candidates B and C (DF0-B, DF0-B2)
// with the same oracle and comparator that scored the shipped DuckDB engine:
//
//   - B: Rust DataFusion for both operations (rust/target/release/df0-cli);
//   - C: the native Go rollup (internal/gorollup) plus DataFusion import.
//
// The host normalizes portfolio money with the shipped rule before handing
// minor units to the Rust engine (DF-06: Go owns money semantics).
//
//	go run ./cmd/df0-candidates -corpus DIR -cli rust/target/release/df0-cli -out evidence/df0-b
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopmgr/internal/analytics"
	"gopmgr/internal/money"
	"gopmgr/spikes/adr012-datafusion/internal/canon"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/gorollup"
	"gopmgr/spikes/adr012-datafusion/internal/oracle"
)

func main() {
	corpus := flag.String("corpus", "", "corpus directory (required)")
	cli := flag.String("cli", "rust/target/release/df0-cli", "DataFusion candidate binary")
	baseline := flag.String("baseline", "evidence/df0-a/duckdb-tabular.jsonl", "engine A results, for side-by-side verdicts")
	out := flag.String("out", "evidence/df0-b", "evidence directory")
	flag.Parse()
	if err := run(*corpus, *cli, *baseline, *out); err != nil {
		fmt.Fprintln(os.Stderr, "df0-candidates:", err)
		os.Exit(1)
	}
}

func run(corpus, cli, baseline, out string) error {
	if corpus == "" {
		return errors.New("-corpus is required")
	}
	if _, err := os.Stat(cli); err != nil {
		return fmt.Errorf("candidate binary: %w (build with: cd rust && cargo build --release --locked)", err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // #nosec G301 -- evidence directory.
		return err
	}
	aVerdicts, err := loadVerdicts(baseline)
	if err != nil {
		return err
	}
	if err := portfolio(corpus, cli, filepath.Join(out, "portfolio.jsonl")); err != nil {
		return err
	}
	if err := tabular(corpus, cli, aVerdicts, filepath.Join(out, "datafusion-tabular.jsonl")); err != nil {
		return err
	}
	if err := filesystem(cli, filepath.Join(out, "datafusion-filesystem.jsonl")); err != nil {
		return err
	}
	return environment(cli, filepath.Join(out, "environment.json"))
}

func writeJSONL(f *os.File, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// runCLI runs the candidate with a timeout and returns its stdout.
func runCLI(cli string, args ...string) ([]byte, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(ctx, cli, args...) // #nosec G204 -- the spike's own binary and corpus paths.
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	return b, time.Since(start), err
}

// --- portfolio ---

type rustRollup struct {
	ProjectCount               int      `json:"project_count"`
	EVMProjectCount            int      `json:"evm_project_count"`
	EVMUnavailableProjectCount int      `json:"evm_unavailable_project_count"`
	Totals                     []string `json:"totals"`
	SPI                        float64  `json:"spi"`
	CPI                        float64  `json:"cpi"`
	Error                      string   `json:"error"`
}

type portfolioRecord struct {
	TestIDs  []string `json:"test_ids"`
	CaseID   string   `json:"case_id"`
	Provider string   `json:"provider"` // B (DataFusion) or C (Go)
	Oracle   string   `json:"oracle_error,omitempty"`
	Error    string   `json:"error,omitempty"`
	Parity   string   `json:"legacy_parity"`
	Issues   []string `json:"mismatches,omitempty"`
	WallNS   int64    `json:"wall_ns"`
}

func normalise(p fixtures.Project) map[string]any {
	leg := func(minor int64, major float64) string {
		if minor == 0 && major != 0 {
			return strconv.FormatInt(money.FromMajorFloat(major).MinorUnits, 10)
		}
		return strconv.FormatInt(minor, 10)
	}
	return map[string]any{
		"budgeted_cost_minor_units":  leg(p.BudgetedCostMinorUnits, p.BudgetedCost),
		"committed_cost_minor_units": leg(p.CommittedCostMinorUnits, p.CommittedCost),
		"actual_cost_minor_units":    leg(p.ActualCostMinorUnits, p.ActualCost),
		"earned_value_minor_units":   leg(p.EarnedValueMinorUnits, p.EarnedValue),
		"planned_value_minor_units":  leg(p.PlannedValueMinorUnits, p.PlannedValue),
		"evm_available":              p.EVMAvailable,
	}
}

func metrics(ps []fixtures.Project) []analytics.ProjectMetrics {
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

func score(rec *portfolioRecord, got analytics.PortfolioSummary, gerr error, want oracle.Outcome) {
	if gerr != nil {
		rec.Error = gerr.Error()
	}
	rec.Oracle = want.Err
	switch {
	case gerr != nil && want.Err != "":
		rec.Parity = "pass"
	case gerr != nil || want.Err != "":
		rec.Parity = "fail"
		rec.Issues = append(rec.Issues, fmt.Sprintf("error %q, oracle error %q", rec.Error, want.Err))
	default:
		rec.Issues = oracle.CompareSummary(got, want.Summary)
		rec.Parity = map[bool]string{true: "pass", false: "fail"}[len(rec.Issues) == 0]
	}
}

func portfolio(corpus, cli, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tmp, err := os.MkdirTemp("", "df0-rollup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	pass := map[string]int{}
	total := 0
	ids := []string{"DF-002", "DF-003", "DF-004", "DF-005", "DF-006", "DF-008", "DF-009", "DF-054", "DF-061"}
	for _, c := range fixtures.PortfolioCases() {
		b, err := os.ReadFile(filepath.Join(corpus, "portfolio", c.ID+".json")) // #nosec G304 -- corpus file.
		if err != nil {
			return err
		}
		pc, err := fixtures.DecodePortfolio(b)
		if err != nil {
			return err
		}
		want, _ := oracle.Rollup(pc.Projects)
		total++

		// B: DataFusion, with host-normalized minor units.
		norm := make([]map[string]any, len(pc.Projects))
		for i, p := range pc.Projects {
			norm[i] = normalise(p)
		}
		in := filepath.Join(tmp, c.ID+".json")
		nb, _ := json.Marshal(norm)
		if err := os.WriteFile(in, nb, 0o600); err != nil {
			return err
		}
		recB := portfolioRecord{TestIDs: ids, CaseID: c.ID, Provider: "B"}
		outB, wall, err := runCLI(cli, "rollup", in)
		recB.WallNS = wall.Nanoseconds()
		var rr rustRollup
		if err == nil {
			err = json.Unmarshal(outB, &rr)
		}
		var gotB analytics.PortfolioSummary
		var errB error
		switch {
		case err != nil:
			errB = err
		case rr.Error != "":
			errB = errors.New(rr.Error)
		default:
			gotB, errB = summaryFrom(rr)
		}
		score(&recB, gotB, errB, want)

		// C: native Go.
		recC := portfolioRecord{TestIDs: ids, CaseID: c.ID, Provider: "C"}
		start := time.Now()
		gotC, errC := gorollup.Rollup(metrics(pc.Projects))
		recC.WallNS = time.Since(start).Nanoseconds()
		score(&recC, gotC, errC, want)
		for _, r := range []portfolioRecord{recB, recC} {
			if r.Parity == "pass" {
				pass[r.Provider]++
			}
			if err := writeJSONL(f, r); err != nil {
				return err
			}
		}
	}
	fmt.Printf("df0-candidates: portfolio: B %d of %d, C %d of %d match the legacy oracle\n", pass["B"], total, pass["C"], total)
	return nil
}

// summaryFrom builds the analytics summary the host would return from the
// Rust totals: display floats as minor/100, indices as computed by Rust.
func summaryFrom(r rustRollup) (analytics.PortfolioSummary, error) {
	if len(r.Totals) != 5 {
		return analytics.PortfolioSummary{}, fmt.Errorf("want 5 totals, got %d", len(r.Totals))
	}
	var t [5]int64
	for i, s := range r.Totals {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return analytics.PortfolioSummary{}, fmt.Errorf("total %d: %w", i, err)
		}
		t[i] = v
	}
	f := func(m int64) float64 { return money.Amount{MinorUnits: m}.MajorFloat() }
	return analytics.PortfolioSummary{
		ProjectCount: r.ProjectCount, EVMProjectCount: r.EVMProjectCount, EVMUnavailableProjectCount: r.EVMUnavailableProjectCount,
		TotalBudgetedCostMinorUnits: t[0], TotalCommittedCostMinorUnits: t[1], TotalActualCostMinorUnits: t[2],
		TotalEarnedValueMinorUnits: t[3], TotalPlannedValueMinorUnits: t[4],
		TotalBudgetedCost: f(t[0]), TotalCommittedCost: f(t[1]), TotalActualCost: f(t[2]), TotalEarnedValue: f(t[3]), TotalPlannedValue: f(t[4]),
		SchedulePerformanceIndex: r.SPI, CostPerformanceIndex: r.CPI,
	}, nil
}

// --- tabular ---

type rustImport struct {
	Columns     []string       `json:"columns"`
	ColumnTypes []string       `json:"column_types"`
	Rows        [][]canon.Cell `json:"rows"`
	RowsSeen    int            `json:"rows_seen"`
	Truncated   bool           `json:"truncated"`
	Error       string         `json:"error"`
	Reader      string         `json:"reader"`
}

type tabularRecord struct {
	TestIDs        []string       `json:"test_ids"`
	Class          fixtures.Class `json:"class"`
	Format         string         `json:"format"`
	Reader         string         `json:"reader"`
	Result         canon.Result   `json:"result"`
	TruncationFlag string         `json:"truncation_flag"` // DF-015/016: correct, wrong, or n/a
	BaselineA      string         `json:"baseline_a_verdict"`
	WallNS         int64          `json:"wall_ns"`
}

func loadVerdicts(path string) (map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 -- committed evidence.
	if err != nil {
		return nil, fmt.Errorf("baseline A results: %w", err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var r struct {
			Result canon.Result `json:"result"`
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		out[r.Result.CaseID] = r.Result.Verdict
	}
	return out, sc.Err()
}

func tabularIDs(format string) []string {
	switch format {
	case "parquet":
		return []string{"DF-012", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
	case "json", "ndjson":
		return []string{"DF-013", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
	}
	return []string{"DF-011", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
}

func tabular(corpus, cli string, aVerdicts map[string]string, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	counts := map[string]int{}
	for _, c := range fixtures.TabularCases() {
		if c.Large {
			continue
		}
		file := filepath.Join(corpus, "tabular", c.File)
		outB, wall, err := runCLI(cli, "import", c.Format, file)
		var ri rustImport
		if err == nil {
			err = json.Unmarshal(outB, &ri)
		}
		if err == nil && ri.Error != "" {
			err = errors.New(fixtures.RedactDir(ri.Error, filepath.Dir(file)))
		}
		rec := tabularRecord{TestIDs: tabularIDs(c.Format), Class: c.Class, Format: c.Format, Reader: ri.Reader,
			Result: canon.Compare(c, ri.Columns, ri.ColumnTypes, ri.Rows, err), BaselineA: aVerdicts[c.ID], WallNS: wall.Nanoseconds()}
		rec.TruncationFlag = "n/a"
		if err == nil && c.Accept == fixtures.AcceptRows {
			want := c.TotalRows > fixtures.PreviewRows
			rec.TruncationFlag = map[bool]string{true: "correct", false: "wrong"}[ri.Truncated == want]
		}
		counts[rec.Result.Verdict]++
		if err := writeJSONL(f, rec); err != nil {
			return err
		}
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("df0-candidates: tabular (B and C) %s: %d\n", k, counts[k])
	}
	return nil
}

// --- filesystem ---

type fsRecord struct {
	CaseID  string `json:"case_id"`
	Note    string `json:"note"`
	Rows    int    `json:"rows"`
	Error   string `json:"error,omitempty"`
	Finding string `json:"finding,omitempty"`
}

func filesystem(cli, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dir, err := os.MkdirTemp("", "df0-fs-b-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	put := func(name, body string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(body), 0o600)
		return p
	}
	put("a.csv", "id,src\n1,a\n")
	put("b.csv", "id,src\n2,b\n")
	cases := []struct{ id, note, path string }{
		{"fs-glob-star", "selected file named *.csv beside a.csv and b.csv", put("*.csv", "id,src\n3,selected\n")},
		{"fs-glob-question", "selected file q?.csv beside q1.csv", func() string { put("q1.csv", "id,src\n5,other\n"); return put("q?.csv", "id,src\n4,selected\n") }()},
		{"fs-quote-in-name", "a single quote in the file name", put("it's.csv", "id\n7\n")},
		{"fs-missing", "a path that does not exist", filepath.Join(dir, "missing.csv")},
	}
	for _, c := range cases {
		outB, _, err := runCLI(cli, "import", "csv", c.path)
		var ri rustImport
		if err == nil {
			err = json.Unmarshal(outB, &ri)
		}
		r := fsRecord{CaseID: c.id, Note: c.note, Rows: len(ri.Rows)}
		if err != nil {
			r.Error = err.Error()
		} else if ri.Error != "" {
			r.Error = strings.ReplaceAll(ri.Error, dir, "<dir>")
		}
		if strings.HasPrefix(c.id, "fs-glob") && r.Error == "" && r.Rows != 1 {
			r.Finding = fmt.Sprintf("the path was expanded as a glob: %d rows from files the user did not select", r.Rows)
		}
		if err := writeJSONL(f, r); err != nil {
			return err
		}
	}
	return nil
}

// --- environment ---

func environment(cli, path string) error {
	env := map[string]any{"go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"cpus": runtime.NumCPU(), "run_at": time.Now().UTC().Format(time.RFC3339)}
	if b, err := exec.Command("rustc", "--version").Output(); err == nil {
		env["rustc"] = strings.TrimSpace(string(b))
	}
	if head, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		env["source_commit"] = strings.TrimSpace(string(head))
	}
	if st, err := exec.Command("git", "status", "--porcelain", "--", "../..", ":(exclude)evidence").Output(); err == nil {
		env["worktree_dirty"] = len(strings.TrimSpace(string(st))) > 0
	}
	if b, err := os.ReadFile("rust/Cargo.lock"); err == nil {
		var pins []string
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			for _, name := range []string{"datafusion", "arrow", "parquet", "tokio", "serde_json"} {
				if l == fmt.Sprintf("name = %q", name) && i+1 < len(lines) {
					pins = append(pins, name+" "+strings.Trim(strings.TrimPrefix(lines[i+1], "version = "), `"`))
				}
			}
		}
		env["crates"] = pins
	}
	if b, _, err := runCLI(cli, "probe-sum-overflow"); err == nil {
		env["datafusion_bigint_sum_overflow"] = strings.Trim(strings.TrimSpace(string(b)), `"`)
	}
	if fi, err := os.Stat(cli); err == nil {
		env["cli_bytes"] = fi.Size()
	}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644) // #nosec G306 -- evidence file.
}
