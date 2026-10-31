// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

type fidelityTable struct {
	Columns []string          `json:"columns"`
	Rows    [][]fixtures.Cell `json:"rows"`
}

// This test-side adapter uses the established corpus format. It is not a
// proposed production tabular API or a generic Arrow decoder.
func fidelityCell(a arrow.Array, row int) (fixtures.Cell, error) {
	if a.IsNull(row) {
		return fixtures.N(), nil
	}
	switch a.DataType().ID() {
	case arrow.INT32, arrow.INT64, arrow.UINT64:
		return fixtures.I(a.ValueStr(row)), nil
	case arrow.FLOAT64:
		return fixtures.F(a.ValueStr(row)), nil
	case arrow.DECIMAL128:
		// ValueStr is a display representation and drops trailing zeros.
		values := a.(*array.Decimal128)
		scale := a.DataType().(*arrow.Decimal128Type).Scale
		return fixtures.D(values.Value(row).ToString(scale)), nil
	case arrow.DECIMAL256:
		values := a.(*array.Decimal256)
		scale := a.DataType().(*arrow.Decimal256Type).Scale
		return fixtures.D(values.Value(row).ToString(scale)), nil
	case arrow.BOOL:
		return fixtures.Cell{K: fixtures.KBool, V: a.ValueStr(row)}, nil
	case arrow.STRING, arrow.STRING_VIEW:
		return fixtures.S(a.ValueStr(row)), nil
	case arrow.BINARY:
		return fixtures.Hex(hex.EncodeToString(a.(*array.Binary).Value(row))), nil
	case arrow.BINARY_VIEW:
		return fixtures.Hex(hex.EncodeToString(a.(*array.BinaryView).Value(row))), nil
	case arrow.TIMESTAMP:
		dt := a.DataType().(*arrow.TimestampType)
		text := a.(*array.Timestamp).Value(row).ToTime(dt.Unit).Format(time.RFC3339Nano)
		if dt.TimeZone == "" {
			text = strings.TrimSuffix(text, "Z")
		}
		return fixtures.Tm(text), nil
	case arrow.DATE32:
		return fixtures.Dt(a.ValueStr(row)), nil
	case arrow.MAP:
		m := a.(*array.Map)
		lo, hi := m.ValueOffsets(row)
		values := make(map[string]any, hi-lo)
		for i := lo; i < hi; i++ {
			key := m.Keys().ValueStr(int(i))
			if _, exists := values[key]; exists {
				return fixtures.Cell{}, fmt.Errorf("duplicate fixture map key")
			}
			values[key] = m.Items().GetOneForMarshal(int(i))
		}
		body, err := json.Marshal(values)
		return fixtures.J(string(body)), err
	case arrow.LIST, arrow.STRUCT:
		body, err := json.Marshal(a.GetOneForMarshal(row))
		return fixtures.J(string(body)), err
	default:
		return fixtures.Cell{}, fmt.Errorf("unqualified fixture type %v", a.DataType())
	}
}

func TestNumericWireRejectsDuplicateMapKeys(t *testing.T) {
	b := array.NewMapBuilder(memory.DefaultAllocator, arrow.BinaryTypes.String, arrow.PrimitiveTypes.Int64, false)
	t.Cleanup(b.Release)
	b.Append(true)
	b.KeyBuilder().(*array.StringBuilder).AppendValues([]string{"same", "same"}, nil)
	b.ItemBuilder().(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	a := b.NewMapArray()
	t.Cleanup(a.Release)
	if _, err := fidelityCell(a, 0); err == nil || err.Error() != "duplicate fixture map key" {
		t.Fatalf("silently lost a map entry: %v", err)
	}
}

func fidelityType(name string) arrow.DataType {
	switch name {
	case "i64":
		return arrow.PrimitiveTypes.Int64
	case "i32":
		return arrow.PrimitiveTypes.Int32
	case "u64":
		return arrow.PrimitiveTypes.Uint64
	case "f64":
		return arrow.PrimitiveTypes.Float64
	case "d38_4":
		return &arrow.Decimal128Type{Precision: 38, Scale: 4}
	case "d9_2":
		return &arrow.Decimal128Type{Precision: 9, Scale: 2}
	case "d60_10":
		return &arrow.Decimal256Type{Precision: 60, Scale: 10}
	case "ts_us_utc":
		return &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
	case "ts_ns_naive":
		return &arrow.TimestampType{Unit: arrow.Nanosecond}
	case "ts_ms_ny":
		return &arrow.TimestampType{Unit: arrow.Millisecond, TimeZone: "America/New_York"}
	}
	return nil // Other fixture fields are checked by kind and exact value.
}

func checkFidelityJS(t *testing.T, actual, expected any) error {
	t.Helper()
	body, err := json.Marshal(struct {
		Actual   any `json:"actual"`
		Expected any `json:"expected"`
	}{actual, expected})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join("..", "..", "tools", "check_wire_fidelity.mjs"))
	cmd.Stdin = bytes.NewReader(body)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("JavaScript checker: %w: %s", err, output)
	}
	return nil
}

func TestNumericWireJavaScriptRejectsLoss(t *testing.T) {
	expected := fidelityTable{
		Columns: []string{"integer", "decimal", "time", "null", "nested"},
		Rows:    [][]fixtures.Cell{{fixtures.I("9007199254740993"), fixtures.D("-12345678901234567890.2300"), fixtures.Tm("2026-01-02T03:04:05.123456789Z"), fixtures.N(), fixtures.J("[9223372036854775807]")}},
	}
	if err := checkFidelityJS(t, expected, expected); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"integer_number", "integer_rounding", "decimal_scale", "timestamp_precision", "null_string", "nested_rounding", "missing_row", "column_order"} {
		t.Run(fault, func(t *testing.T) {
			body, err := json.Marshal(expected)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := json.Unmarshal(body, &actual); err != nil {
				t.Fatal(err)
			}
			row := actual["rows"].([]any)[0].([]any)
			switch fault {
			case "integer_number":
				row[0].(map[string]any)["v"] = float64(9007199254740993)
			case "integer_rounding":
				row[0].(map[string]any)["v"] = "9007199254740992"
			case "decimal_scale":
				row[1].(map[string]any)["v"] = "-12345678901234567890.23"
			case "timestamp_precision":
				row[2].(map[string]any)["v"] = "2026-01-02T03:04:05.123Z"
			case "null_string":
				row[3] = fixtures.S("null")
			case "nested_rounding":
				row[4].(map[string]any)["v"] = "[9223372036854776000]"
			case "missing_row":
				actual["rows"] = []any{}
			case "column_order":
				cols := actual["columns"].([]any)
				cols[0], cols[1] = cols[1], cols[0]
			}
			err = checkFidelityJS(t, actual, expected)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(err.Error(), "WIRE_MISMATCH") {
				t.Fatalf("fault %s was not rejected by the value checker: %v", fault, err)
			}
		})
	}
}

func TestNativeNumericWireFidelity(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the reviewed native worker")
	}
	type observation struct {
		Result
		FixtureSHA256 string        `json:"fixture_sha256"`
		Types         []string      `json:"arrow_types"`
		Table         fidelityTable `json:"javascript_verified_table"`
	}
	var evidence []observation
	wanted := map[string]bool{"pq-scalars": true, "pq-scalars-nulls": true, "pq-decimals": true, "pq-timestamps": true, "pq-nested": true}
	for _, c := range fixtures.TabularCases() {
		if !wanted[c.ID] {
			continue
		}
		delete(wanted, c.ID)
		for _, mode := range []string{"parquet_range", "parquet_stream"} {
			t.Run(c.ID+"/"+mode, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), c.File)
				if err := c.Write(path); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(data)
				snap, err := source.Capture(path)
				if err != nil {
					t.Fatal(err)
				}
				actual := fidelityTable{Columns: c.Columns, Rows: [][]fixtures.Cell{}}
				var types []string
				verify := func(data []byte) (int, error) {
					rd, err := ipc.NewReader(bytes.NewReader(data))
					if err != nil {
						return 0, err
					}
					defer rd.Release()
					n := 0
					for rd.Next() {
						rec := rd.RecordBatch()
						if rec.NumCols() != int64(len(c.Columns)) {
							return n, fmt.Errorf("wrong column count")
						}
						for j, field := range rec.Schema().Fields() {
							if field.Name != c.Columns[j] {
								return n, fmt.Errorf("wrong column order")
							}
							if want := fidelityType(field.Name); want != nil && !arrow.TypeEqual(field.Type, want) {
								return n, fmt.Errorf("%s type %v, want %v", field.Name, field.Type, want)
							}
						}
						if types == nil {
							for _, field := range rec.Schema().Fields() {
								types = append(types, field.Type.String())
							}
							actual.Columns = make([]string, len(c.Columns))
							for j, field := range rec.Schema().Fields() {
								actual.Columns[j] = field.Name
							}
						}
						for i := 0; i < int(rec.NumRows()); i++ {
							row := make([]fixtures.Cell, len(c.Columns))
							for j := range row {
								row[j], err = fidelityCell(rec.Column(j), i)
								if err != nil {
									return n, err
								}
							}
							if len(actual.Rows) >= c.TotalRows || !reflect.DeepEqual(row, c.Row(len(actual.Rows))) {
								return n, fmt.Errorf("literal golden differs at row %d: %+v", len(actual.Rows), row)
							}
							actual.Rows = append(actual.Rows, row)
							n++
						}
					}
					return n, rd.Err()
				}
				r := Run(Job{Name: c.ID + "/" + mode, Worker: worker, ReadSource: snap, MaxFrame: 64 << 10, Timeout: 30 * time.Second, Verify: verify,
					Execute: &df0pb.Execute{Operation: mode, BatchRows: 2}})
				if r.Outcome != "completed" || r.Rows != uint64(c.TotalRows) || r.Batches < 2 || len(actual.Rows) != c.TotalRows || r.DigestMatch == nil || !*r.DigestMatch || r.SourceMatch == nil || !*r.SourceMatch || r.WorkerExit != "exit 0" || !r.EndpointRemoved {
					t.Fatalf("incomplete numeric pipeline: %+v; %s", r, r.WorkerDiag)
				}
				expected := fidelityTable{Columns: c.Columns, Rows: make([][]fixtures.Cell, c.TotalRows)}
				for i := range expected.Rows {
					expected.Rows[i] = c.Row(i)
				}
				if err := checkFidelityJS(t, actual, expected); err != nil {
					t.Fatal(err)
				}
				evidence = append(evidence, observation{Result: r, FixtureSHA256: hex.EncodeToString(sum[:]), Types: types, Table: actual})
			})
		}
	}
	if len(wanted) != 0 {
		t.Fatalf("missing literal fixtures: %v", wanted)
	}
	if path := os.Getenv("DF0_FIDELITY_EVIDENCE"); path != "" && !t.Failed() {
		body, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
