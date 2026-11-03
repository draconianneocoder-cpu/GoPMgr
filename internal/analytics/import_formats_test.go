// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Expected values below are written by hand from the fixture files in
// testdata/import, not taken from any engine (ADR-012 DF-A11). The Parquet
// fixtures were written by DuckDB, a different implementation from the Arrow
// reader under test; see testdata/import/README.md.

type importCase struct {
	columns   []string
	rows      [][]any // nil: check only the first and last rows given in edges
	edges     [][]any // first and last row, for large fixtures
	count     int
	truncated bool
	err       string
}

var parquetTypesRows = [][]any{
	{int64(1), "9007199254740993", 1.5, "alpha", true, "2026-01-02", "2026-01-02 03:04:05", "12345.67", "[1,2,3]", `{"k":"v","n":2}`},
	{int64(2), int64(-5), -0.25, "Zoë", false, "1999-12-31", "1999-12-31 23:59:59.5", "-0.01", "[]", `{"k":"w","n":3}`},
	{nil, nil, nil, nil, nil, nil, nil, nil, nil, nil},
}

var parquetTypesColumns = []string{"i32", "i64", "dbl", "txt", "flag", "day", "stamp", "amount", "list", "rec"}

var importCases = map[string]importCase{
	"basic.csv": {columns: []string{"id", "name", "amount"}, rows: [][]any{
		{int64(1), "Alpha", 10.5}, {int64(2), "Beta", 20.0}, {int64(3), "Gamma", -3.25}}},
	"types.csv": {columns: []string{"flag", "count", "ratio", "sci", "day", "stamp", "note"}, rows: [][]any{
		{true, int64(42), 0.5, 1000.0, "2026-01-02", "2026-01-02 03:04:05", "x"},
		{false, int64(-7), 2.25, -0.025, "2025-12-31", "2025-12-31 23:59:59", nil},
		{true, int64(0), 0.0, 0.0, "2024-02-29", "2024-02-29 00:00:00", "z"}}},
	"noheader.csv": {columns: []string{"column0", "column1", "column2"}, rows: [][]any{
		{int64(1), 2.5, int64(3)}, {int64(4), 5.5, int64(6)}, {int64(7), 8.5, int64(9)}}},
	"allstrings.csv": {columns: []string{"city", "country"}, rows: [][]any{{"Paris", "France"}, {"Lima", "Peru"}}},
	"quoted.csv": {columns: []string{"id", "text"}, rows: [][]any{
		{int64(1), "hello, world"}, {int64(2), "line one\nline two"}, {int64(3), `she said "hi"`}}},
	"bom_crlf.csv":     {columns: []string{"id", "name"}, rows: [][]any{{int64(1), "Alpha"}, {int64(2), "Beta"}}},
	"semicolon.csv":    {columns: []string{"id", "name", "value"}, rows: [][]any{{int64(1), "Alpha", "3,5"}, {int64(2), "Beta", "4"}}},
	"pipe.txt":         {columns: []string{"id", "name"}, rows: [][]any{{int64(1), "Alpha"}, {int64(2), "Beta"}}},
	"tabs.tsv":         {columns: []string{"id", "name", "value"}, rows: [][]any{{int64(1), "Alpha", 1.5}, {int64(2), "Beta", 2.5}}},
	"dupnames.csv":     {columns: []string{"a", "a_1", "column2", "b"}, rows: [][]any{{int64(1), int64(2), int64(3), int64(4)}}},
	"ragged.csv":       {err: "line 3 has 2 fields, but the first line has 3"},
	"unicode.csv":      {columns: []string{"name", "city"}, rows: [][]any{{"Zoë", "Zürich"}, {"東京", "日本"}}},
	"leadingzeros.csv": {columns: []string{"code", "zip"}, rows: [][]any{{"007", "02134"}, {"123", "90210"}}},
	"bigint.csv":       {columns: []string{"n"}, rows: [][]any{{"9223372036854775807"}, {"9223372036854775808"}}},
	"empty.csv":        {err: "the file is empty"},
	"header_only.csv":  {columns: []string{"a", "b"}, rows: [][]any{}},
	"many.csv":         {columns: []string{"n"}, edges: [][]any{{int64(0)}, {int64(9999)}}, count: 10000, truncated: true},
	"array.json":       {columns: []string{"a", "b", "c"}, rows: [][]any{{int64(1), "x", nil}, {int64(2), nil, true}}},
	"objects.ndjson":   {columns: []string{"a", "b", "c"}, rows: [][]any{{int64(1), "x", nil}, {int64(2), nil, true}}},
	"nested.json": {columns: []string{"id", "tags", "meta"}, rows: [][]any{
		{int64(1), `["x","y"]`, `{"k":"v","n":2}`}, {int64(2), "[]", nil}}},
	"scalararray.json":   {columns: []string{"json"}, rows: [][]any{{int64(1)}, {int64(2)}, {int64(3)}}},
	"single.json":        {columns: []string{"a", "b"}, rows: [][]any{{int64(1), "only"}}},
	"numbers.json":       {columns: []string{"big", "dec", "neg", "exp"}, rows: [][]any{{"9007199254740993", 0.1, int64(-5), 100.0}}},
	"lines_in_json.json": {columns: []string{"a"}, rows: [][]any{{int64(1)}, {int64(2)}}},
	"nulls.json":         {columns: []string{"a", "b"}, rows: [][]any{{nil, int64(1)}, {"x", nil}}},
	"many.parquet":       {columns: []string{"n"}, edges: [][]any{{int64(0)}, {int64(9999)}}, count: 10000, truncated: true},
}

func init() {
	for _, codec := range []string{"uncompressed", "snappy", "gzip", "zstd", "brotli", "lz4"} {
		importCases["types-"+codec+".parquet"] = importCase{columns: parquetTypesColumns, rows: parquetTypesRows}
	}
}

func TestImportTabularFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "import")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if _, ok := formatFor(filepath.Ext(name)); !ok || name == "duckdb-parity.json" {
			continue
		}
		if _, ok := importCases[name]; !ok {
			t.Errorf("fixture %s has no expected result", name)
		}
	}
	for name, want := range importCases {
		t.Run(name, func(t *testing.T) {
			ds, err := ImportTabular(context.Background(), filepath.Join(dir, name))
			if want.err != "" {
				if err == nil || !strings.Contains(err.Error(), want.err) {
					t.Fatalf("err = %v, want one containing %q", err, want.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ds.Columns, want.columns) {
				t.Errorf("columns = %q, want %q", ds.Columns, want.columns)
			}
			if ds.Truncated != want.truncated {
				t.Errorf("truncated = %t, want %t", ds.Truncated, want.truncated)
			}
			if want.rows != nil {
				if !reflect.DeepEqual(ds.Rows, want.rows) {
					t.Errorf("rows =\n%#v\nwant\n%#v", ds.Rows, want.rows)
				}
				return
			}
			if len(ds.Rows) != want.count {
				t.Fatalf("%d rows, want %d", len(ds.Rows), want.count)
			}
			if !reflect.DeepEqual(ds.Rows[0], want.edges[0]) || !reflect.DeepEqual(ds.Rows[len(ds.Rows)-1], want.edges[1]) {
				t.Errorf("first/last rows = %#v / %#v, want %#v", ds.Rows[0], ds.Rows[len(ds.Rows)-1], want.edges)
			}
		})
	}
}

// parityDispositions records, for every fixture, whether the Go engine's
// result is the same as the retired DuckDB engine's (captured in
// testdata/import/duckdb-parity.json) and, if not, why. The test fails on an
// undeclared difference and on a declared one that no longer occurs, so
// docs/testing/duckdb-retirement-parity.md stays true.
var parityDispositions = map[string]string{
	"allstrings.csv":     "same",
	"array.json":         "same",
	"basic.csv":          "same",
	"bigint.csv":         "differs: DuckDB read both values as one rounded float (9223372036854776000); Go keeps the exact digits as text",
	"bom_crlf.csv":       "same",
	"dupnames.csv":       "same",
	"empty.csv":          "differs: DuckDB returned an empty column named column0; Go reports that the file is empty",
	"header_only.csv":    "same",
	"leadingzeros.csv":   "same",
	"lines_in_json.json": "same",
	"many.csv":           "same",
	"many.parquet":       "same",
	"nested.json":        "differs: DuckDB returned lists and objects, which the UI showed as text like [object Object]; Go returns compact JSON text",
	"noheader.csv":       "same",
	"nulls.json":         "same",
	"numbers.json":       "differs: DuckDB returned 9007199254740993 as a number, which JavaScript rounds to ...992; Go returns the exact digits as text",
	"objects.ndjson":     "same",
	"pipe.txt":           "same",
	"quoted.csv":         "same",
	"ragged.csv":         "differs: DuckDB read each line as one text column named \"a,b,c\"; Go reports the line with the wrong field count",
	"scalararray.json":   "same",
	"semicolon.csv":      "same",
	"single.json":        "same",
	"tabs.tsv":           "same",
	"types.csv":          "differs: DuckDB returned dates and timestamps as UTC times (shown as 2026-01-02T00:00:00Z); Go keeps the text as written",
	"unicode.csv":        "same",
}

func init() {
	for _, codec := range []string{"uncompressed", "snappy", "gzip", "zstd", "brotli", "lz4"} {
		parityDispositions["types-"+codec+".parquet"] = "differs: DuckDB returned dates and timestamps as UTC times, decimals as objects and nested values as objects; Go returns plain dates, timestamps without an invented zone, exact decimal text and compact JSON, and keeps the 64-bit value beyond 2^53 as text"
	}
}

func TestImportTabularParityWithDuckDB(t *testing.T) {
	dir := filepath.Join("testdata", "import")
	raw, err := os.ReadFile(filepath.Join(dir, "duckdb-parity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var baseline struct {
		Results map[string]struct {
			Error      string `json:"error"`
			Columns    []string
			RowCount   int `json:"row_count"`
			SampleRows [][]struct {
				Value string `json:"value"`
			} `json:"sample_rows"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatal(err)
	}
	var names []string
	for n := range baseline.Results {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) != len(parityDispositions) {
		t.Errorf("%d baseline fixtures, %d dispositions", len(names), len(parityDispositions))
	}
	for _, name := range names {
		duck := baseline.Results[name]
		disposition, ok := parityDispositions[name]
		if !ok {
			t.Errorf("%s: no parity disposition", name)
			continue
		}
		ds, err := ImportTabular(context.Background(), filepath.Join(dir, name))
		same := (err == nil) == (duck.Error == "")
		if same && err == nil {
			same = reflect.DeepEqual(ds.Columns, duck.Columns) && len(ds.Rows) == duck.RowCount
			for i, row := range duck.SampleRows {
				r := i
				if i >= 5 { // the baseline keeps the first five rows and the last
					r = len(ds.Rows) - 1
				}
				if r >= len(ds.Rows) {
					same = false
					break
				}
				for c, cell := range row {
					got, _ := json.Marshal(ds.Rows[r][c])
					if string(got) != cell.Value && !(cell.Value == "null" && ds.Rows[r][c] == nil) && !sameJSONNumber(string(got), cell.Value) {
						same = false
					}
				}
			}
		}
		switch {
		case same && disposition != "same":
			t.Errorf("%s: now matches DuckDB, but the ledger says %q", name, disposition)
		case !same && !strings.HasPrefix(disposition, "differs: "):
			t.Errorf("%s: differs from DuckDB (err %v), but the ledger says same", name, err)
		}
	}
}

// sameJSONNumber treats 20 and 20.0 (int and float renderings) as equal.
func sameJSONNumber(a, b string) bool {
	var x, y float64
	return json.Unmarshal([]byte(a), &x) == nil && json.Unmarshal([]byte(b), &y) == nil && x == y && !strings.Contains(a, `"`) && !strings.Contains(b, `"`)
}
