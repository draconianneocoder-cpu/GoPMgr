// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"
	"fmt"

	"github.com/go-pdf/fpdf"

	"pmforge/internal/charts"
)

// renderMatrix dispatches on kind because each matrix kind has its
// own layout payload shape (RACI cells, SWOT quadrants, Stakeholder
// plot points, generic m×n grid).
func renderMatrix(pdf *fpdf.Fpdf, kind string, body json.RawMessage, frame Frame) error {
	switch charts.Kind(kind) {
	case charts.KindRACI:
		return renderRACI(pdf, body, frame)
	case charts.KindSWOT:
		return renderSWOT(pdf, body, frame)
	case charts.KindStakeholder:
		return renderStakeholder(pdf, body, frame)
	case charts.KindMatrixDiagram:
		return renderGenericMatrix(pdf, body, frame)
	}
	return ErrUnsupportedKind
}

// ---------- RACI ----------

type raciTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
type raciCell struct {
	TaskID string `json:"task_id"`
	Role   string `json:"role"`
	Value  string `json:"value"`
}
type raciLayoutBody struct {
	Roles []string   `json:"roles"`
	Tasks []raciTask `json:"tasks"`
	Cells []raciCell `json:"cells"`
}

func renderRACI(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
	var layout raciLayoutBody
	if err := parseBody(body, &layout); err != nil {
		return err
	}
	if len(layout.Roles) == 0 || len(layout.Tasks) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	// Index cells for O(1) lookup.
	cellMap := make(map[string]string, len(layout.Cells))
	for _, c := range layout.Cells {
		cellMap[c.TaskID+"|"+c.Role] = c.Value
	}

	// Column widths: first column for task title (40% of frame), rest
	// shared evenly among roles.
	firstColW := frame.W * 0.4
	if firstColW < 40 {
		firstColW = 40
	}
	roleColW := (frame.W - firstColW) / float64(len(layout.Roles))
	if roleColW < 12 {
		roleColW = 12
	}
	rowH := 6.0
	maxRows := int((frame.H - rowH) / rowH) // -rowH for header row
	if maxRows < 1 {
		maxRows = 1
	}

	// Header row.
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetFillColor(30, 41, 59)
	pdf.SetTextColor(241, 245, 249)
	pdf.SetDrawColor(100, 116, 139)
	pdf.SetLineWidth(0.2)
	pdf.SetXY(frame.X, frame.Y)
	pdf.CellFormat(firstColW, rowH, "Task", "1", 0, "L", true, 0, "")
	for _, role := range layout.Roles {
		pdf.CellFormat(roleColW, rowH, role, "1", 0, "C", true, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)

	// Body rows.
	pdf.SetFont("Helvetica", "", 7)
	y := frame.Y + rowH
	for i, t := range layout.Tasks {
		if i >= maxRows {
			// Footer indicator that we ran out of room.
			pdf.SetXY(frame.X, y)
			pdf.SetFont("Helvetica", "I", 7)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(frame.W, rowH, fmt.Sprintf("… and %d more rows", len(layout.Tasks)-i),
				"", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			break
		}
		pdf.SetXY(frame.X, y)
		pdf.CellFormat(firstColW, rowH, truncatePDF(t.Title, int(firstColW*0.6)), "1", 0, "L", false, 0, "")
		for _, role := range layout.Roles {
			v := cellMap[t.ID+"|"+role]
			fillRACICell(pdf, v)
			pdf.CellFormat(roleColW, rowH, v, "1", 0, "C", true, 0, "")
		}
		y += rowH
	}
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// fillRACICell sets the cell fill colour based on the assignment
// letter so the printed matrix matches the on-screen tint scheme.
func fillRACICell(pdf *fpdf.Fpdf, v string) {
	switch v {
	case "R":
		pdf.SetFillColor(22, 78, 99) // cyan-900
	case "A":
		pdf.SetFillColor(6, 78, 59) // emerald-900
	case "C":
		pdf.SetFillColor(120, 53, 15) // amber-900
	case "I":
		pdf.SetFillColor(51, 65, 85)
	default:
		pdf.SetFillColor(15, 23, 42)
	}
}

// ---------- SWOT ----------

type swotQuadrant struct {
	Key   string   `json:"key"`
	Title string   `json:"title"`
	Items []string `json:"items"`
	Row   int      `json:"row"`
	Col   int      `json:"col"`
	Tone  string   `json:"tone"`
}
type swotLayoutBody struct {
	Quadrants []swotQuadrant `json:"quadrants"`
}

func renderSWOT(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
	var layout swotLayoutBody
	if err := parseBody(body, &layout); err != nil {
		return err
	}
	if len(layout.Quadrants) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	paneW := frame.W / 2
	paneH := frame.H / 2

	for _, q := range layout.Quadrants {
		x := frame.X + float64(q.Col)*paneW
		y := frame.Y + float64(q.Row)*paneH

		// Tone → fill colour.
		switch q.Tone {
		case "positive":
			pdf.SetFillColor(6, 78, 59)
		case "negative":
			pdf.SetFillColor(127, 29, 29)
		case "external_positive":
			pdf.SetFillColor(7, 89, 133)
		case "external_negative":
			pdf.SetFillColor(120, 53, 15)
		default:
			pdf.SetFillColor(30, 41, 59)
		}
		pdf.SetDrawColor(100, 116, 139)
		pdf.SetLineWidth(0.25)
		pdf.Rect(x, y, paneW, paneH, "FD")

		// Pane heading
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(241, 245, 249)
		pdf.SetXY(x+2, y+2)
		pdf.CellFormat(paneW-4, 5, q.Key+" · "+q.Title, "", 0, "L", false, 0, "")

		// Items, bullet-style.
		pdf.SetFont("Helvetica", "", 7)
		cursor := y + 9
		bottom := y + paneH - 2
		for _, item := range q.Items {
			if cursor+3.5 > bottom {
				// Truncation indicator
				pdf.SetXY(x+2, bottom-3)
				pdf.SetFont("Helvetica", "I", 6)
				pdf.SetTextColor(203, 213, 225)
				pdf.CellFormat(paneW-4, 3, "…", "", 0, "L", false, 0, "")
				break
			}
			pdf.SetXY(x+2, cursor)
			pdf.SetFont("Helvetica", "", 7)
			pdf.SetTextColor(241, 245, 249)
			pdf.MultiCell(paneW-4, 3.5, "· "+item, "", "L", false)
			cursor = pdf.GetY()
		}
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// ---------- Stakeholder Analysis ----------

type stakePoint struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Power    string  `json:"power"`
	Interest string  `json:"interest"`
	Strategy string  `json:"strategy"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}
type quadrantLabel struct {
	Power    string `json:"power"`
	Interest string `json:"interest"`
	Title    string `json:"title"`
	Strategy string `json:"strategy"`
}
type stakeLayoutBody struct {
	Points    []stakePoint    `json:"points"`
	Quadrants []quadrantLabel `json:"quadrants"`
}

func renderStakeholder(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
	var layout stakeLayoutBody
	if err := parseBody(body, &layout); err != nil {
		return err
	}

	// 2×2 quadrant grid in the frame.
	paneW := frame.W / 2
	paneH := frame.H / 2

	for _, q := range layout.Quadrants {
		// Map (power, interest) to (col, row). The chart spec puts:
		//   low power, low interest  → bottom-left   (col=0,row=1)
		//   low power, high interest → bottom-right  (col=1,row=1)
		//   high power, low interest → top-left      (col=0,row=0)
		//   high power, high interest→ top-right     (col=1,row=0)
		col := 0
		if q.Interest == "high" {
			col = 1
		}
		row := 1
		if q.Power == "high" {
			row = 0
		}
		x := frame.X + float64(col)*paneW
		y := frame.Y + float64(row)*paneH

		// Quadrant fill by intensity.
		if q.Power == "high" && q.Interest == "high" {
			pdf.SetFillColor(127, 29, 29) // red
		} else if q.Power == "high" {
			pdf.SetFillColor(120, 53, 15) // amber
		} else if q.Interest == "high" {
			pdf.SetFillColor(22, 78, 99) // cyan
		} else {
			pdf.SetFillColor(30, 41, 59)
		}
		pdf.SetDrawColor(100, 116, 139)
		pdf.SetLineWidth(0.25)
		pdf.Rect(x, y, paneW, paneH, "FD")

		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(241, 245, 249)
		pdf.SetXY(x+2, y+2)
		pdf.CellFormat(paneW-4, 4, q.Strategy, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 6)
		pdf.SetTextColor(203, 213, 225)
		pdf.SetXY(x+2, y+6)
		pdf.CellFormat(paneW-4, 3, q.Title, "", 0, "L", false, 0, "")
	}

	// Points: backend emits x/y in 0..1; we scale to the frame.
	for _, p := range layout.Points {
		px := frame.X + p.X*frame.W
		py := frame.Y + p.Y*frame.H
		// Draw a small circle.
		pdf.SetFillColor(14, 116, 144)
		pdf.SetDrawColor(34, 211, 238)
		pdf.SetLineWidth(0.3)
		pdf.Circle(px, py, 1.6, "FD")
		// Label below.
		pdf.SetFont("Helvetica", "", 6)
		pdf.SetTextColor(203, 213, 225)
		pdf.SetXY(px-15, py+1.8)
		pdf.CellFormat(30, 2.5, truncatePDF(p.Name, 18), "", 0, "C", false, 0, "")
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// ---------- Generic Matrix ----------

type genericMatrixBody struct {
	Title     string     `json:"title,omitempty"`
	RowsLabel string     `json:"rows_label,omitempty"`
	ColsLabel string     `json:"cols_label,omitempty"`
	Rows      []string   `json:"rows"`
	Cols      []string   `json:"cols"`
	Cells     [][]string `json:"cells"`
}

func renderGenericMatrix(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
	var layout genericMatrixBody
	if err := parseBody(body, &layout); err != nil {
		return err
	}
	if len(layout.Rows) == 0 || len(layout.Cols) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	firstColW := frame.W * 0.3
	if firstColW < 30 {
		firstColW = 30
	}
	colW := (frame.W - firstColW) / float64(len(layout.Cols))
	if colW < 14 {
		colW = 14
	}
	rowH := 6.0
	maxRows := int((frame.H - rowH) / rowH)
	if maxRows < 1 {
		maxRows = 1
	}

	// Header row.
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetFillColor(30, 41, 59)
	pdf.SetTextColor(241, 245, 249)
	pdf.SetDrawColor(100, 116, 139)
	pdf.SetLineWidth(0.2)
	pdf.SetXY(frame.X, frame.Y)
	header := layout.RowsLabel + " \\ " + layout.ColsLabel
	pdf.CellFormat(firstColW, rowH, header, "1", 0, "L", true, 0, "")
	for _, c := range layout.Cols {
		pdf.CellFormat(colW, rowH, c, "1", 0, "C", true, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont("Helvetica", "", 7)
	y := frame.Y + rowH
	for i, r := range layout.Rows {
		if i >= maxRows {
			pdf.SetXY(frame.X, y)
			pdf.SetFont("Helvetica", "I", 7)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(frame.W, rowH, fmt.Sprintf("… and %d more rows", len(layout.Rows)-i),
				"", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			break
		}
		pdf.SetXY(frame.X, y)
		pdf.SetFillColor(15, 23, 42)
		pdf.SetTextColor(241, 245, 249)
		pdf.CellFormat(firstColW, rowH, truncatePDF(r, int(firstColW*0.6)), "1", 0, "L", true, 0, "")
		pdf.SetTextColor(0, 0, 0)
		for c := range layout.Cols {
			cell := ""
			if i < len(layout.Cells) && c < len(layout.Cells[i]) {
				cell = layout.Cells[i][c]
			}
			pdf.SetFont("Helvetica", "", 7)
			pdf.CellFormat(colW, rowH, truncatePDF(cell, int(colW*0.6)), "1", 0, "C", false, 0, "")
		}
		y += rowH
	}
	pdf.SetFillColor(255, 255, 255)
	return nil
}
