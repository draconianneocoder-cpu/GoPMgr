// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import "testing"

// ----- ParseGenericMatrix -----

func TestParseGenericMatrix_Empty(t *testing.T) {
	doc, err := ParseGenericMatrix("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if len(doc.Rows) != 0 || len(doc.Cols) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseGenericMatrix_EmptyObject(t *testing.T) {
	doc, err := ParseGenericMatrix("{}")
	if err != nil {
		t.Fatalf("unexpected error for {}: %v", err)
	}
	if len(doc.Rows) != 0 {
		t.Error("expected empty doc from {}")
	}
}

func TestParseGenericMatrix_InvalidJSON(t *testing.T) {
	if _, err := ParseGenericMatrix("{nope}"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseGenericMatrix_ValidDocument(t *testing.T) {
	raw := `{
		"title": "Traceability",
		"rows_label": "Requirements",
		"cols_label": "Tests",
		"rows": ["R1", "R2"],
		"cols": ["T1"],
		"cells": [["x"], [""]]
	}`
	doc, err := ParseGenericMatrix(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Traceability" || doc.RowsLabel != "Requirements" || doc.ColsLabel != "Tests" {
		t.Errorf("metadata not parsed: %+v", doc)
	}
	if len(doc.Rows) != 2 || len(doc.Cols) != 1 {
		t.Errorf("rows/cols: got %d/%d, want 2/1", len(doc.Rows), len(doc.Cols))
	}
}

// ----- LayoutGenericMatrix: normalisation -----

func TestLayoutGenericMatrix_PadsShortRows(t *testing.T) {
	// Cells has fewer columns than Cols header; missing cells pad to "".
	doc := GenericMatrixDocument{
		Rows:  []string{"r1", "r2"},
		Cols:  []string{"c1", "c2", "c3"},
		Cells: [][]string{{"a"}, {"b", "c"}},
	}
	layout := LayoutGenericMatrix(doc)
	if len(layout.Cells) != 2 {
		t.Fatalf("expected 2 cell rows, got %d", len(layout.Cells))
	}
	for r, row := range layout.Cells {
		if len(row) != 3 {
			t.Errorf("row %d: got width %d, want 3 (padded to len(Cols))", r, len(row))
		}
	}
	// Spot-check padding values.
	if layout.Cells[0][0] != "a" || layout.Cells[0][1] != "" || layout.Cells[0][2] != "" {
		t.Errorf("row 0 not padded correctly: %v", layout.Cells[0])
	}
	if layout.Cells[1][0] != "b" || layout.Cells[1][1] != "c" || layout.Cells[1][2] != "" {
		t.Errorf("row 1 not padded correctly: %v", layout.Cells[1])
	}
}

func TestLayoutGenericMatrix_TruncatesLongRows(t *testing.T) {
	// A source row wider than len(Cols) is truncated to the header width.
	doc := GenericMatrixDocument{
		Rows:  []string{"r1"},
		Cols:  []string{"c1", "c2"},
		Cells: [][]string{{"a", "b", "EXTRA"}},
	}
	layout := LayoutGenericMatrix(doc)
	if len(layout.Cells[0]) != 2 {
		t.Fatalf("row width: got %d, want 2 (truncated)", len(layout.Cells[0]))
	}
	if layout.Cells[0][0] != "a" || layout.Cells[0][1] != "b" {
		t.Errorf("truncated row contents wrong: %v", layout.Cells[0])
	}
}

func TestLayoutGenericMatrix_FewerCellRowsThanHeaders(t *testing.T) {
	// More Rows headers than Cells rows: the missing rows become all-empty.
	doc := GenericMatrixDocument{
		Rows:  []string{"r1", "r2", "r3"},
		Cols:  []string{"c1"},
		Cells: [][]string{{"only"}},
	}
	layout := LayoutGenericMatrix(doc)
	if len(layout.Cells) != 3 {
		t.Fatalf("expected 3 cell rows (one per Row header), got %d", len(layout.Cells))
	}
	if layout.Cells[0][0] != "only" {
		t.Errorf("row 0: got %q, want only", layout.Cells[0][0])
	}
	if layout.Cells[1][0] != "" || layout.Cells[2][0] != "" {
		t.Errorf("missing rows should be empty, got %v / %v", layout.Cells[1], layout.Cells[2])
	}
}

func TestLayoutGenericMatrix_MetadataPassThrough(t *testing.T) {
	doc := GenericMatrixDocument{
		Title:     "T",
		RowsLabel: "RL",
		ColsLabel: "CL",
		Rows:      []string{"r1"},
		Cols:      []string{"c1"},
		Cells:     [][]string{{"v"}},
	}
	layout := LayoutGenericMatrix(doc)
	if layout.Title != "T" || layout.RowsLabel != "RL" || layout.ColsLabel != "CL" {
		t.Errorf("metadata not passed through: %+v", layout)
	}
}

func TestLayoutGenericMatrix_Empty(t *testing.T) {
	layout := LayoutGenericMatrix(GenericMatrixDocument{})
	if len(layout.Rows) != 0 || len(layout.Cols) != 0 || len(layout.Cells) != 0 {
		t.Errorf("empty doc should yield empty layout, got %+v", layout)
	}
}
