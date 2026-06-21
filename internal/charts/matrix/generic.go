// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package matrix

import "encoding/json"

// GenericMatrixDocument is the JSON shape stored in db.charts.data
// for a Matrix Diagram chart. It is intentionally non-prescriptive:
// any two-dimensional relationship can be expressed as Rows × Cols
// with text in each cell.
//
// Common uses include:
//
//   - Requirements traceability (rows = requirements, cols = tests)
//   - Prioritization matrices (rows = options, cols = criteria)
//   - Decision matrices (rows = alternatives, cols = factors)
//
// Cells is stored row-major: Cells[r][c] is the value at row r,
// column c. The slice is normalised to the (len(Rows), len(Cols))
// shape during layout so the GUI never has to handle ragged arrays.
type GenericMatrixDocument struct {
	Title     string     `json:"title,omitempty"`
	RowsLabel string     `json:"rows_label,omitempty"` // axis title above row headers
	ColsLabel string     `json:"cols_label,omitempty"` // axis title above column headers
	Rows      []string   `json:"rows"`
	Cols      []string   `json:"cols"`
	Cells     [][]string `json:"cells"`
}

// GenericMatrixLayout is the frontend payload.
type GenericMatrixLayout struct {
	Title     string     `json:"title,omitempty"`
	RowsLabel string     `json:"rows_label,omitempty"`
	ColsLabel string     `json:"cols_label,omitempty"`
	Rows      []string   `json:"rows"`
	Cols      []string   `json:"cols"`
	Cells     [][]string `json:"cells"`
}

// ParseGenericMatrix decodes the JSON blob.
func ParseGenericMatrix(raw string) (GenericMatrixDocument, error) {
	if raw == "" || raw == "{}" {
		return GenericMatrixDocument{}, nil
	}
	var doc GenericMatrixDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return GenericMatrixDocument{}, err
	}
	return doc, nil
}

// LayoutGenericMatrix normalises the cells array to a strict
// rows × cols rectangle, padding short rows with empty strings.
// This lets the frontend assume a uniform grid shape.
func LayoutGenericMatrix(doc GenericMatrixDocument) GenericMatrixLayout {
	rows := append([]string{}, doc.Rows...)
	cols := append([]string{}, doc.Cols...)

	cells := make([][]string, len(rows))
	for r := 0; r < len(rows); r++ {
		row := make([]string, len(cols))
		if r < len(doc.Cells) {
			src := doc.Cells[r]
			for c := 0; c < len(cols); c++ {
				if c < len(src) {
					row[c] = src[c]
				}
			}
		}
		cells[r] = row
	}

	return GenericMatrixLayout{
		Title:     doc.Title,
		RowsLabel: doc.RowsLabel,
		ColsLabel: doc.ColsLabel,
		Rows:      rows,
		Cols:      cols,
		Cells:     cells,
	}
}
