// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

// Command df0-baseline runs GoPMgr's shipped DuckDB analytics engine
// (gopmgr/internal/analytics, unmodified) over the DF0 corpus and records,
// for every case, what it returns against the independent truth: the
// fixture generator for tabular files and the big-integer oracle for
// portfolio rollups (ADR-012 DF0-A and DF0-A2). It writes JSONL evidence.
//
//	go run -tags duckdb ./cmd/df0-baseline -corpus DIR -out evidence/df0-a
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/duckdb/duckdb-go/v2"

	"gopmgr/internal/analytics"
	"gopmgr/spikes/adr012-datafusion/internal/canon"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/oracle"
)

func main() {
	corpus := flag.String("corpus", "", "corpus directory written by df0-fixtures (required)")
	out := flag.String("out", "evidence/df0-a", "evidence directory")
	flag.Parse()
	if err := run(*corpus, *out); err != nil {
		fmt.Fprintln(os.Stderr, "df0-baseline:", err)
		os.Exit(1)
	}
}

func run(corpus, out string) error {
	if corpus == "" {
		return errors.New("-corpus is required")
	}
	if !analytics.New().Available() {
		return errors.New("the shipped engine is unavailable; build with -tags duckdb")
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // #nosec G301 -- evidence directory.
		return err
	}
	ctx := context.Background()
	pf, err := os.Create(filepath.Join(out, "duckdb-portfolio.jsonl"))
	if err != nil {
		return err
	}
	defer pf.Close()
	pass, total := 0, 0
	for _, c := range fixtures.PortfolioCases() {
		rec, ok, err := portfolioCase(ctx, corpus, c)
		if err != nil {
			return err
		}
		total++
		if ok {
			pass++
		}
		if err := writeJSONL(pf, rec); err != nil {
			return err
		}
	}
	fmt.Printf("df0-baseline: portfolio: %d of %d cases match the legacy oracle\n", pass, total)

	tf, err := os.Create(filepath.Join(out, "duckdb-tabular.jsonl"))
	if err != nil {
		return err
	}
	defer tf.Close()
	verdicts := map[string]int{}
	var wireFailures []string
	for _, c := range fixtures.TabularCases() {
		if c.Large {
			continue
		}
		rec := tabularCase(ctx, filepath.Join(corpus, "tabular", c.File), c)
		verdicts[rec.Result.Verdict]++
		if rec.Wire.Error != "" {
			wireFailures = append(wireFailures, c.ID)
		}
		if err := writeJSONL(tf, rec); err != nil {
			return err
		}
	}
	keys := []string{}
	for k := range verdicts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("df0-baseline: tabular %s: %d\n", k, verdicts[k])
	}
	// The verdict compares engine values with the fixture; the wire is a
	// separate check (DF-065), so a matching case can still fail here.
	fmt.Printf("df0-baseline: tabular wire failures (Dataset not JSON-encodable): %d %v\n", len(wireFailures), wireFailures)

	ff, err := os.Create(filepath.Join(out, "duckdb-filesystem.jsonl"))
	if err != nil {
		return err
	}
	defer ff.Close()
	for _, rec := range filesystemCases(ctx) {
		if err := writeJSONL(ff, rec); err != nil {
			return err
		}
	}
	return writeEnvironment(filepath.Join(out, "environment.json"))
}

func writeJSONL(f *os.File, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// --- portfolio ---

type portfolioRecord struct {
	TestIDs      []string          `json:"test_ids"`
	CaseID       string            `json:"case_id"`
	Class        fixtures.Class    `json:"class"`
	Projects     int               `json:"projects"`
	DuckDBError  string            `json:"duckdb_error,omitempty"`
	OracleError  string            `json:"oracle_error,omitempty"`
	Mismatches   []string          `json:"mismatches,omitempty"`
	LegacyParity string            `json:"legacy_parity"` // pass or fail: DuckDB equals the legacy oracle
	Correctness  string            `json:"correctness"`   // exact, ambiguous or invalid input
	Ambiguous    []string          `json:"ambiguous,omitempty"`
	Invalid      []string          `json:"invalid,omitempty"`
	Totals       map[string]string `json:"totals,omitempty"`
	SPI          string            `json:"spi,omitempty"`
	CPI          string            `json:"cpi,omitempty"`
	SecondCall   string            `json:"second_call,omitempty"` // DF-009: identical on a repeat call
	ElapsedNS    int64             `json:"elapsed_ns"`
}

func toMetrics(ps []fixtures.Project) []analytics.ProjectMetrics {
	out := make([]analytics.ProjectMetrics, len(ps))
	for i, p := range ps {
		out[i] = analytics.ProjectMetrics{
			ProjectID: p.ProjectID, Name: p.Name,
			BudgetedCost: p.BudgetedCost, CommittedCost: p.CommittedCost, ActualCost: p.ActualCost,
			EarnedValue: p.EarnedValue, PlannedValue: p.PlannedValue,
			BudgetedCostMinorUnits: p.BudgetedCostMinorUnits, CommittedCostMinorUnits: p.CommittedCostMinorUnits,
			ActualCostMinorUnits: p.ActualCostMinorUnits, EarnedValueMinorUnits: p.EarnedValueMinorUnits,
			PlannedValueMinorUnits: p.PlannedValueMinorUnits, PercentComplete: p.PercentComplete,
			EVMAvailable: p.EVMAvailable,
		}
	}
	return out
}

// rollupOnce runs the shipped path exactly as RunPortfolioAnalytics does:
// a new engine per call, closed afterwards.
func rollupOnce(ctx context.Context, m []analytics.ProjectMetrics) (analytics.PortfolioSummary, error) {
	eng := analytics.New()
	defer func() { _ = eng.Close() }()
	return eng.PortfolioRollup(ctx, m)
}

func portfolioCase(ctx context.Context, corpus string, c fixtures.PortfolioCase) (portfolioRecord, bool, error) {
	// Read the case back from the corpus file, so the engine sees exactly the
	// bytes the manifest pins.
	b, err := os.ReadFile(filepath.Join(corpus, "portfolio", c.ID+".json")) // #nosec G304 -- corpus file.
	if err != nil {
		return portfolioRecord{}, false, err
	}
	fromFile, err := fixtures.DecodePortfolio(b)
	if err != nil {
		return portfolioRecord{}, false, fmt.Errorf("%s: %w", c.ID, err)
	}
	rec := portfolioRecord{TestIDs: []string{"DF-001", "DF-002", "DF-003", "DF-004", "DF-005", "DF-006", "DF-008", "DF-009", "DF-054", "DF-061"},
		CaseID: c.ID, Class: c.Class, Projects: len(fromFile.Projects)}
	want, corr := oracle.Rollup(fromFile.Projects)
	switch {
	case len(corr.Invalid) > 0:
		rec.Correctness = "invalid_input"
	case len(corr.Ambiguous) > 0:
		rec.Correctness = "ambiguous_input"
	default:
		rec.Correctness = "exact"
	}
	rec.Ambiguous, rec.Invalid = corr.Ambiguous, corr.Invalid
	m := toMetrics(fromFile.Projects)
	start := time.Now()
	got, gerr := rollupOnce(ctx, m)
	rec.ElapsedNS = time.Since(start).Nanoseconds()
	if gerr != nil {
		rec.DuckDBError = gerr.Error()
	}
	rec.OracleError = want.Err
	switch {
	case gerr != nil && want.Err != "":
		rec.LegacyParity = "pass" // both refuse: an out-of-range total is an error, never a wrapped value
	case gerr != nil || want.Err != "":
		rec.LegacyParity = "fail"
		rec.Mismatches = append(rec.Mismatches, fmt.Sprintf("duckdb error %q, oracle error %q", rec.DuckDBError, want.Err))
	default:
		rec.Mismatches = compareSummary(got, want.Summary)
		rec.LegacyParity = map[bool]string{true: "pass", false: "fail"}[len(rec.Mismatches) == 0]
		rec.Totals = map[string]string{}
		for f, name := range oracle.FieldNames {
			rec.Totals[name] = want.Summary.Totals[f].String()
		}
		rec.SPI = strconv.FormatFloat(got.SchedulePerformanceIndex, 'g', -1, 64)
		rec.CPI = strconv.FormatFloat(got.CostPerformanceIndex, 'g', -1, 64)
		again, aerr := rollupOnce(ctx, m)
		rec.SecondCall = map[bool]string{true: "identical", false: "differs"}[aerr == nil && again == got]
		if rec.SecondCall != "identical" {
			rec.LegacyParity = "fail"
		}
	}
	return rec, rec.LegacyParity == "pass", nil
}

// compareSummary checks every field exactly, except the two ratios, which
// the legacy code computes from float64 major units: they must be within
// 2 ULP of the exact ratio's nearest float64 (the documented tolerance).
func compareSummary(got analytics.PortfolioSummary, want *oracle.Summary) []string {
	var out []string
	eqInt := func(name string, g int, w int) {
		if g != w {
			out = append(out, fmt.Sprintf("%s: got %d, want %d", name, g, w))
		}
	}
	eqInt("project_count", got.ProjectCount, want.ProjectCount)
	eqInt("evm_project_count", got.EVMProjectCount, want.EVMProjectCount)
	eqInt("evm_unavailable_project_count", got.EVMUnavailableProjectCount, want.EVMUnavailableProjectCount)
	gm := [5]int64{got.TotalBudgetedCostMinorUnits, got.TotalCommittedCostMinorUnits, got.TotalActualCostMinorUnits, got.TotalEarnedValueMinorUnits, got.TotalPlannedValueMinorUnits}
	gf := [5]float64{got.TotalBudgetedCost, got.TotalCommittedCost, got.TotalActualCost, got.TotalEarnedValue, got.TotalPlannedValue}
	for f, name := range oracle.FieldNames {
		if big.NewInt(gm[f]).Cmp(want.Totals[f]) != 0 {
			out = append(out, fmt.Sprintf("%s_minor_units: got %d, want %s", name, gm[f], want.Totals[f]))
		}
		// The display float is minor/100 in float64.
		if wf := float64(want.Totals[f].Int64()) / 100; gf[f] != wf {
			out = append(out, fmt.Sprintf("%s: got %v, want %v", name, gf[f], wf))
		}
	}
	ratio := func(name string, g, w float64, defined bool) {
		if !defined {
			if g != 0 {
				out = append(out, fmt.Sprintf("%s: got %v, want 0 (n/a)", name, g))
			}
			return
		}
		if d := oracle.ULPDistance(g, w); d > 2 {
			out = append(out, fmt.Sprintf("%s: got %v, want %v (%d ULP)", name, g, w, d))
		}
	}
	ratio("spi", got.SchedulePerformanceIndex, want.SPI, want.SPIDefined)
	ratio("cpi", got.CostPerformanceIndex, want.CPI, want.CPIDefined)
	return out
}

// --- tabular ---

type tabularRecord struct {
	TestIDs []string       `json:"test_ids"`
	Class   fixtures.Class `json:"class"`
	Format  string         `json:"format"`
	Result  canon.Result   `json:"result"`
	// Wire describes json.Marshal(Dataset), the bytes Wails sends to the UI.
	Wire      wireStats `json:"wire"`
	ElapsedNS int64     `json:"elapsed_ns"`
}

type wireStats struct {
	Bytes int `json:"bytes"`
	// JSUnsafeNumbers counts JSON numbers a JavaScript parse would change
	// (integers beyond 2^53, decimals that do not round-trip binary64).
	JSUnsafeNumbers int `json:"js_unsafe_numbers"`
	// ObjectCells counts cells sent as JSON objects (for example a DuckDB
	// Decimal struct), which the UI renders as text "[object Object]".
	ObjectCells int `json:"object_cells"`
	// Error is set when the Dataset cannot cross the JSON boundary at all,
	// for example a NaN or infinite float64 cell. Wails marshals call results
	// with encoding/json and calls os.Exit(1) when that fails
	// (internal/frontend/dispatcher/calls.go, Wails v2.17.0), so this is a
	// failed import in the app, not a measurement gap. The counts above are
	// zero when Error is set.
	Error string `json:"error,omitempty"`
}

func testIDs(format string) []string {
	switch format {
	case "parquet":
		return []string{"DF-001", "DF-012", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
	case "json", "ndjson":
		return []string{"DF-001", "DF-013", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
	}
	return []string{"DF-001", "DF-011", "DF-014", "DF-015", "DF-016", "DF-017", "DF-018"}
}

func tabularCase(ctx context.Context, path string, c fixtures.TabularCase) tabularRecord {
	eng := analytics.New()
	defer func() { _ = eng.Close() }()
	start := time.Now()
	ds, err := eng.ImportTabular(ctx, path)
	elapsed := time.Since(start).Nanoseconds()
	rows := make([][]canon.Cell, len(ds.Rows))
	colTypes := make([]string, len(ds.Columns))
	for i, r := range ds.Rows {
		rows[i] = make([]canon.Cell, len(r))
		for j, v := range r {
			rows[i][j] = toCell(v)
			if j < len(colTypes) && colTypes[j] == "" && v != nil {
				colTypes[j] = fmt.Sprintf("%T", v)
			}
		}
	}
	if err != nil {
		// Paths are environment-specific; keep only the message shape.
		err = errors.New(strings.ReplaceAll(err.Error(), filepath.Dir(path), "<corpus>"))
	}
	res := canon.Compare(c, ds.Columns, colTypes, rows, err)
	rec := tabularRecord{TestIDs: testIDs(c.Format), Class: c.Class, Format: c.Format, Result: res, ElapsedNS: elapsed}
	if err == nil {
		rec.Wire = measureWire(ds)
	}
	return rec
}

// toCell converts a database/sql value from the shipped DuckDB driver.
func toCell(v any) canon.Cell {
	typ := fmt.Sprintf("%T", v)
	mk := func(k fixtures.Kind, s string) canon.Cell {
		return canon.Cell{Cell: fixtures.Cell{K: k, V: s}, EngineType: typ}
	}
	switch t := v.(type) {
	case nil:
		return mk(fixtures.KNull, "")
	case bool:
		return mk(fixtures.KBool, strconv.FormatBool(t))
	case int8, int16, int32, int64, int, uint8, uint16, uint32, uint64, uint:
		return mk(fixtures.KInt, fmt.Sprint(t))
	case *big.Int:
		return mk(fixtures.KInt, t.String())
	case float32:
		return mk(fixtures.KFloat, floatText(float64(t)))
	case float64:
		return mk(fixtures.KFloat, floatText(t))
	case string:
		return mk(fixtures.KStr, t)
	case []byte:
		return mk(fixtures.KBytes, fmt.Sprintf("%x", t))
	case time.Time:
		return mk(fixtures.KTime, t.UTC().Format(time.RFC3339Nano))
	case duckdb.Decimal:
		return mk(fixtures.KDec, decimalText(t))
	case duckdb.UUID:
		return mk(fixtures.KStr, fmt.Sprintf("%x", [16]byte(t)))
	}
	b, err := json.Marshal(jsonable(v))
	if err != nil {
		return mk(fixtures.KStr, fmt.Sprint(v))
	}
	return mk(fixtures.KJSON, string(b))
}

func floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	case f == 0 && math.Signbit(f):
		return "-0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// decimalText renders a DuckDB decimal exactly, keeping its declared scale.
func decimalText(d duckdb.Decimal) string {
	s := new(big.Int).Abs(d.Value).String()
	sign := ""
	if d.Value.Sign() < 0 {
		sign = "-"
	}
	scale := int(d.Scale)
	if scale == 0 {
		return sign + s
	}
	if len(s) <= scale {
		s = strings.Repeat("0", scale-len(s)+1) + s
	}
	return sign + s[:len(s)-scale] + "." + s[len(s)-scale:]
}

// jsonable converts nested driver values (LIST as []any, STRUCT as
// map[string]any, MAP as duckdb.Map) into encoding/json-friendly values with
// the same canonical scalar rendering as top-level cells.
func jsonable(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = jsonable(e)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, e := range t {
			out[k] = jsonable(e)
		}
		return out
	case duckdb.Map:
		out := map[string]any{}
		for k, e := range t {
			out[fmt.Sprint(k)] = jsonable(e)
		}
		return out
	case duckdb.Decimal:
		return json.Number(decimalText(t))
	case *big.Int:
		return json.Number(t.String())
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case []byte:
		return fmt.Sprintf("%x", t)
	}
	return v
}

func measureWire(ds analytics.Dataset) wireStats {
	b, err := json.Marshal(ds)
	if err != nil {
		return wireStats{Error: "json.Marshal: " + err.Error()}
	}
	ws := wireStats{Bytes: len(b)}
	var decoded struct {
		Rows [][]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		ws.Error = "json.Unmarshal: " + err.Error()
		return ws
	}
	for _, r := range decoded.Rows {
		for _, cell := range r {
			s := strings.TrimSpace(string(cell))
			switch {
			case strings.HasPrefix(s, "{"):
				ws.ObjectCells++
				ws.JSUnsafeNumbers += unsafeNumbersIn(s)
			case strings.HasPrefix(s, "["):
				ws.JSUnsafeNumbers += unsafeNumbersIn(s)
			default:
				ws.JSUnsafeNumbers += unsafeNumbersIn(s)
			}
		}
	}
	return ws
}

// unsafeNumbersIn counts numbers in a JSON value whose binary64 parse is
// not exactly the written decimal: JavaScript's JSON.parse would change them.
func unsafeNumbersIn(s string) int {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return 0
	}
	var walk func(any) int
	walk = func(v any) int {
		switch t := v.(type) {
		case json.Number:
			r, ok := new(big.Rat).SetString(t.String())
			if !ok {
				return 0
			}
			f, _ := r.Float64()
			if math.IsInf(f, 0) {
				return 1
			}
			if new(big.Rat).SetFloat64(f).Cmp(r) != 0 {
				// A decimal such as 0.1 is not exact in binary64 either way;
				// count only integers that a parse would change.
				if r.IsInt() {
					return 1
				}
			}
		case []any:
			n := 0
			for _, e := range t {
				n += walk(e)
			}
			return n
		case map[string]any:
			n := 0
			for _, e := range t {
				n += walk(e)
			}
			return n
		}
		return 0
	}
	return walk(v)
}

// --- filesystem and path handling (DF-017, DF-020, DF-A07 baseline) ---

type fsRecord struct {
	TestIDs  []string `json:"test_ids"`
	CaseID   string   `json:"case_id"`
	Note     string   `json:"note"`
	Status   string   `json:"status"` // observed, blocked
	Error    string   `json:"error,omitempty"`
	Columns  []string `json:"columns,omitempty"`
	Rows     int      `json:"rows"`
	Finding  string   `json:"finding,omitempty"`
	Original string   `json:"original_unchanged,omitempty"` // DF-020
}

func filesystemCases(ctx context.Context) []fsRecord {
	dir, err := os.MkdirTemp("", "df0-fs-")
	if err != nil {
		return []fsRecord{{CaseID: "setup", Status: "blocked", Error: err.Error()}}
	}
	defer os.RemoveAll(dir)
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(body), 0o600)
		return p
	}
	imp := func(path string) (analytics.Dataset, error) {
		eng := analytics.New()
		defer func() { _ = eng.Close() }()
		ds, err := eng.ImportTabular(ctx, path)
		if err != nil {
			err = errors.New(strings.ReplaceAll(err.Error(), dir, "<dir>"))
		}
		return ds, err
	}
	var out []fsRecord
	add := func(id, note string, ds analytics.Dataset, err error, finding string) {
		r := fsRecord{TestIDs: []string{"DF-017"}, CaseID: id, Note: note, Status: "observed", Columns: ds.Columns, Rows: len(ds.Rows), Finding: finding}
		if err != nil {
			r.Error = err.Error()
		}
		out = append(out, r)
	}

	ds, err := imp(filepath.Join(dir, "missing.csv"))
	add("fs-missing", "a path that does not exist", ds, err, "")
	sub := filepath.Join(dir, "folder.csv")
	_ = os.Mkdir(sub, 0o700)
	ds, err = imp(sub)
	add("fs-directory", "a directory named like a CSV file", ds, err, "")
	ds, err = imp(write("sheet.xlsx", "PK"))
	add("fs-unsupported-extension", ".xlsx (handled by the frontend reader, not ImportTabular)", ds, err, "")
	ds, err = imp(write("noext", "id\n1\n"))
	add("fs-no-extension", "no extension", ds, err, "")

	// Glob metacharacters: a user-selected file literally named "*.csv"
	// beside two other CSV files.
	globDir := filepath.Join(dir, "glob")
	_ = os.Mkdir(globDir, 0o700)
	_ = os.WriteFile(filepath.Join(globDir, "a.csv"), []byte("id,src\n1,a\n"), 0o600)
	_ = os.WriteFile(filepath.Join(globDir, "b.csv"), []byte("id,src\n2,b\n"), 0o600)
	globFile := filepath.Join(globDir, "*.csv")
	_ = os.WriteFile(globFile, []byte("id,src\n3,selected\n"), 0o600)
	ds, err = imp(globFile)
	finding := ""
	if err == nil && len(ds.Rows) != 1 {
		finding = fmt.Sprintf("the path was expanded as a glob: %d rows from files the user did not select", len(ds.Rows))
	}
	add("fs-glob-star", "selected file named *.csv beside a.csv and b.csv", ds, err, finding)
	_ = os.Mkdir(filepath.Join(globDir, "sub"), 0o700)
	_ = os.WriteFile(filepath.Join(globDir, "sub", "c.csv"), []byte("id,src\n4,sub\n"), 0o600)
	deepFile := filepath.Join(globDir, "**.csv")
	_ = os.WriteFile(deepFile, []byte("id,src\n5,selected\n"), 0o600)
	ds, err = imp(deepFile)
	finding = ""
	if err == nil && len(ds.Rows) != 1 {
		finding = fmt.Sprintf("the path was expanded as a glob: %d rows", len(ds.Rows))
	}
	add("fs-glob-doublestar", "selected file named **.csv beside a subdirectory", ds, err, finding)
	qFile := write("q?.csv", "id,src\n4,selected\n")
	write("q1.csv", "id,src\n5,other\n")
	ds, err = imp(qFile)
	finding = ""
	if err == nil && len(ds.Rows) != 1 {
		finding = fmt.Sprintf("the path was expanded as a glob: %d rows", len(ds.Rows))
	}
	add("fs-glob-question", "selected file q?.csv beside q1.csv", ds, err, finding)

	quote := write("it's.csv", "id\n7\n")
	ds, err = imp(quote)
	add("fs-quote-in-name", "a single quote in the file name (SQL string escaping)", ds, err, "")

	outside, _ := os.MkdirTemp("", "df0-outside-")
	defer os.RemoveAll(outside)
	secret := filepath.Join(outside, "secret.csv")
	_ = os.WriteFile(secret, []byte("id,secret\n1,s3cr3t\n"), 0o600)
	link := filepath.Join(dir, "link.csv")
	_ = os.Symlink(secret, link)
	ds, err = imp(link)
	finding = ""
	if err == nil && len(ds.Rows) == 1 {
		finding = "a symlink to a file outside the selected directory is followed; allowed_directories does not confine it"
	}
	add("fs-symlink-outside", "selected symlink pointing outside its directory", ds, err, finding)

	unreadable := write("noperm.csv", "id\n1\n")
	_ = os.Chmod(unreadable, 0o000)
	ds, err = imp(unreadable)
	rec := fsRecord{TestIDs: []string{"DF-017"}, CaseID: "fs-unreadable", Note: "mode 000 file", Status: "observed", Rows: len(ds.Rows), Columns: ds.Columns}
	if err != nil {
		rec.Error = err.Error()
	}
	if os.Geteuid() == 0 {
		rec.Status, rec.Finding = "blocked", "running as root, which bypasses file permissions; needs an unprivileged run"
	}
	out = append(out, rec)

	// DF-020: the import leaves the source file unchanged.
	src := write("unchanged.csv", "id,name\n1,a\n2,b\n")
	before, _ := os.ReadFile(src) // #nosec G304 -- test file.
	statBefore, _ := os.Stat(src)
	_, err = imp(src)
	after, _ := os.ReadFile(src) // #nosec G304 -- test file.
	statAfter, _ := os.Stat(src)
	r := fsRecord{TestIDs: []string{"DF-020"}, CaseID: "fs-source-unchanged", Note: "bytes and modification time before and after import", Status: "observed"}
	if err != nil {
		r.Error = err.Error()
	}
	r.Original = map[bool]string{true: "unchanged", false: "CHANGED"}[string(before) == string(after) && statBefore.ModTime().Equal(statAfter.ModTime())]
	out = append(out, r)
	return out
}

// --- environment ---

func writeEnvironment(path string) error {
	env := map[string]any{
		"go_version": runtime.Version(),
		"goos":       runtime.GOOS,
		"goarch":     runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"run_at":     time.Now().UTC().Format(time.RFC3339),
		"engine":     "gopmgr/internal/analytics (duckdb build tag), unmodified",
	}
	if head, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		env["source_commit"] = strings.TrimSpace(string(head))
	}
	if st, err := exec.Command("git", "status", "--porcelain", "--", "../..", ":(exclude)evidence").Output(); err == nil {
		env["worktree_dirty"] = len(strings.TrimSpace(string(st))) > 0
	}
	if mods, err := exec.Command("go", "list", "-deps", "-tags", "duckdb", "-f", "{{with .Module}}{{.Path}} {{.Version}}{{end}}", "gopmgr/internal/analytics").Output(); err == nil {
		seen := map[string]bool{}
		var list []string
		for _, l := range strings.Split(strings.TrimSpace(string(mods)), "\n") {
			if l != "" && !seen[l] && !strings.HasPrefix(l, "gopmgr ") {
				seen[l] = true
				list = append(list, l)
			}
		}
		sort.Strings(list)
		env["engine_modules"] = list
	}
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "model name") {
				env["cpu_model"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				break
			}
		}
	}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644) // #nosec G306 -- evidence file.
}
