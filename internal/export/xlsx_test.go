// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"gopmgr/internal/kernel"
)

func TestRenderXLSXStoresFormulaLikeTitlesAsText(t *testing.T) {
	const title = "=1+1"
	out, err := renderXLSX(ReportPayload{
		Tasks: map[string]*kernel.Task{
			"task-1": {ID: "task-1", Title: title},
		},
	}, ExportOptions{})
	if err != nil {
		t.Fatalf("renderXLSX: %v", err)
	}

	book, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = book.Close() })

	formula, err := book.GetCellFormula("Schedule", "B2")
	if err != nil {
		t.Fatalf("GetCellFormula: %v", err)
	}
	if formula != "" {
		t.Errorf("formula-like title became a formula %q", formula)
	}
	value, err := book.GetCellValue("Schedule", "B2")
	if err != nil {
		t.Fatalf("GetCellValue: %v", err)
	}
	if value != title {
		t.Errorf("title = %q, want %q", value, title)
	}
}

func TestRenderXLSXPreservesNumericPrecision(t *testing.T) {
	out, err := renderXLSX(ReportPayload{Tasks: map[string]*kernel.Task{
		"a": {ID: "a", Duration: 0.3333333333333333, ES: -0.3333333333333333,
			EF: 1234567890123456, LS: 123.00000000000003, LF: 1e-16, Float: -1234567890.1234567},
		"b": {ID: "b", Duration: math.Copysign(0, -1), ES: math.SmallestNonzeroFloat64,
			EF: math.MaxFloat64, LF: math.Inf(1), Float: math.NaN()},
	}}, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	var sheet struct {
		Cells []struct {
			Reference string `xml:"r,attr"`
			Type      string `xml:"t,attr"`
			Value     string `xml:"v"`
		} `xml:"sheetData>row>c"`
	}
	r, err := archive.Open("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := xml.NewDecoder(r).Decode(&sheet); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"C2": "0.3333333333333333", "D2": "-0.3333333333333333",
		"E2": "1234567890123456", "F2": "123.00000000000003",
		"G2": "0.0000000000000001", "H2": "-1234567890.1234567",
		"C3": "-0", "D3": "0." + strings.Repeat("0", 323) + "5",
		"E3": "17976931348623157" + strings.Repeat("0", 292), "F3": "0",
	}
	got := make(map[string]string)
	for _, cell := range sheet.Cells {
		if _, numeric := want[cell.Reference]; numeric {
			if cell.Type != "" && cell.Type != "n" {
				t.Errorf("%s type = %q, want numeric", cell.Reference, cell.Type)
			}
			got[cell.Reference] = cell.Value
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("raw numeric cells = %v, want %v", got, want)
	}
	book, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	for cell, want := range map[string]string{"G3": "+Inf", "H3": "NaN"} {
		if got, err := book.GetCellValue("Schedule", cell); err != nil || got != want {
			t.Errorf("non-finite %s = %q, %v; want %q", cell, got, err, want)
		}
		if got, err := book.GetCellType("Schedule", cell); err != nil || got != excelize.CellTypeSharedString {
			t.Errorf("non-finite %s type = %v, %v; want text", cell, got, err)
		}
	}
}

func TestRenderXLSXWorkbookContract(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "tasks", true: "empty"}[empty], func(t *testing.T) {
			tasks := map[string]*kernel.Task{
				"z": {ID: "z", Title: "=1+1", Duration: 1.25, IsCritical: true},
				"a": {ID: "a", Title: "日本語 & <draft>", Duration: -0.5},
			}
			if empty {
				tasks = nil
			}
			start := time.Now().UTC()
			out, err := renderXLSX(ReportPayload{Tasks: tasks}, ExportOptions{Title: "=project"})
			end := time.Now().UTC()
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = book.Close() })
			if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"Schedule", "Meta"}) {
				t.Fatalf("sheets = %v", got)
			}
			rows, err := book.GetRows("Schedule")
			if err != nil {
				t.Fatal(err)
			}
			want := [][]string{{"ID", "Title", "Duration", "ES", "EF", "LS", "LF", "Float", "Critical"}}
			if !empty {
				want = append(want, []string{"a", "日本語 & <draft>", "-0.5", "0", "0", "0", "0", "0", "FALSE"},
					[]string{"z", "=1+1", "1.25", "0", "0", "0", "0", "0", "TRUE"})
			}
			if !reflect.DeepEqual(rows, want) {
				t.Errorf("schedule rows = %v, want %v", rows, want)
			}
			for cell, want := range map[string]string{"A1": "Title", "B1": "=project", "A2": "GeneratedAt", "A3": "AppVersion", "B3": exportVersion()} {
				if got, err := book.GetCellValue("Meta", cell); err != nil || got != want {
					t.Errorf("Meta %s = %q, %v; want %q", cell, got, err, want)
				}
			}
			stamp, err := book.GetCellValue("Meta", "B2")
			if err != nil {
				t.Fatal(err)
			}
			generated, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil || generated.Location() != time.UTC || generated.Before(start) || generated.After(end) {
				t.Errorf("GeneratedAt = %q, %v; want UTC time within invocation", stamp, err)
			}
			for _, sheet := range book.GetSheetList() {
				rows, err := book.GetRows(sheet)
				if err != nil {
					t.Fatal(err)
				}
				for row, values := range rows {
					for col := range values {
						cell, err := excelize.CoordinatesToCellName(col+1, row+1)
						if err != nil {
							t.Fatal(err)
						}
						if formula, err := book.GetCellFormula(sheet, cell); err != nil || formula != "" {
							t.Errorf("%s %s formula = %q, %v", sheet, cell, formula, err)
						}
					}
				}
			}
			if !empty {
				for _, cell := range []string{"I2", "I3"} {
					if got, err := book.GetCellType("Schedule", cell); err != nil || got != excelize.CellTypeBool {
						t.Errorf("%s type = %v, %v; want boolean", cell, got, err)
					}
				}
			}
		})
	}
}

func TestXLSXDependencyRejectsOverflowingColumn(t *testing.T) {
	for name, want := range map[string]int{"A": 1, "XFD": 16384} {
		if got, err := excelize.ColumnNameToNumber(name); err != nil || got != want {
			t.Errorf("column %q = %d, %v; want %d", name, got, err, want)
		}
	}
	if _, err := excelize.ColumnNameToNumber("CRPXNLSKVLJFHH"); !errors.Is(err, excelize.ErrColumnNumber) {
		t.Fatalf("overflow error = %v, want ErrColumnNumber", err)
	}
}
