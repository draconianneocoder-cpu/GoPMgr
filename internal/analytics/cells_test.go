// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal256"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// Wails exits the application when a call result cannot be encoded as JSON
// (legacy defect LD-10), so no cell may be NaN or infinite.
func TestCellsAlwaysEncode(t *testing.T) {
	b := array.NewFloat64Builder(memory.DefaultAllocator)
	defer b.Release()
	b.AppendValues([]float64{math.NaN(), math.Inf(1), math.Inf(-1), 1.5}, nil)
	floats := b.NewFloat64Array()
	defer floats.Release()

	want := []any{"NaN", "+Inf", "-Inf", 1.5}
	row := make([]any, 0, len(want))
	for i, w := range want {
		got := arrowCell(floats, i)
		if got != w {
			t.Errorf("float cell %d = %#v, want %#v", i, got, w)
		}
		row = append(row, got)
	}

	// JSON 1e400 overflows float64 (LD-3); keep the text.
	cell, err := jsonCell(json.RawMessage("1e400"))
	if err != nil || cell != "1e400" {
		t.Errorf("jsonCell(1e400) = %#v, %v; want the text", cell, err)
	}
	row = append(row, cell)

	if _, err := json.Marshal(Dataset{Columns: make([]string, len(row)), Rows: [][]any{row}}); err != nil {
		t.Fatalf("dataset does not encode: %v", err)
	}
}

// A wide decimal must keep its exact digits (LD-1: DuckDB returned an
// unrelated float for decimal256(60,10)).
func TestDecimal256KeepsExactDigits(t *testing.T) {
	dt := &arrow.Decimal256Type{Precision: 60, Scale: 10}
	b := array.NewDecimal256Builder(memory.DefaultAllocator, dt)
	defer b.Release()
	v, err := decimal256.FromString("12300000000000000000000000000000000000000000000000.0000000001", 60, 10)
	if err != nil {
		t.Fatal(err)
	}
	b.Append(v)
	arr := b.NewDecimal256Array()
	defer arr.Release()

	if got, want := arrowCell(arr, 0), "12300000000000000000000000000000000000000000000000.0000000001"; got != want {
		t.Errorf("decimal256 cell = %#v, want %q", got, want)
	}
}
