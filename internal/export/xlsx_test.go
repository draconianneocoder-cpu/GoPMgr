// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"testing"

	"gopmgr/internal/kernel"

	"github.com/xuri/excelize/v2"
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
