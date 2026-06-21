// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"

	"github.com/jung-kurt/gofpdf"
)

// ganttRow / ganttLayout mirror dag.GanttRow / dag.GanttLayout (wire
// format is stable; redeclared to keep the parser dependency-light,
// matching nodeLayout's precedent).
type ganttRow struct {
	ID              string  `json:"id"`
	Label           string  `json:"label"`
	ES              float64 `json:"es"`
	EF              float64 `json:"ef"`
	IsCritical      bool    `json:"is_critical"`
	Milestone       bool    `json:"milestone"`
	PercentComplete float64 `json:"percent_complete"`
	StartDate       string  `json:"start_date,omitempty"`
	FinishDate      string  `json:"finish_date,omitempty"`
}

type ganttLayout struct {
	Rows    []ganttRow `json:"rows"`
	Horizon float64    `json:"horizon"`
}

// renderGantt draws schedule bars over a day-scaled axis: label
// column, critical bars in red, progress as a darker inner bar,
// milestones as diamonds, anchored dates beside the bars.
func renderGantt(pdf *gofpdf.Fpdf, body json.RawMessage, frame Frame) error {
	// The body is the wrapped {layout, doc} form.
	var wrapped struct {
		Layout ganttLayout `json:"layout"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return err
	}
	layout := wrapped.Layout
	if len(layout.Rows) == 0 || layout.Horizon <= 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.Text(frame.X+2, frame.Y+6, "No scheduled tasks.")
		return nil
	}

	const labelW = 45.0
	const rowH = 7.0
	const barH = 4.0

	chartX := frame.X + labelW
	chartW := frame.W - labelW
	scale := chartW / layout.Horizon

	maxRows := int(frame.H / rowH)
	rows := layout.Rows
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}

	// Light vertical day grid (skip when days are too dense).
	if step := pickGridStep(layout.Horizon, chartW); step > 0 {
		pdf.SetDrawColor(226, 232, 240)
		for d := 0.0; d <= layout.Horizon; d += step {
			x := chartX + d*scale
			pdf.Line(x, frame.Y, x, frame.Y+float64(len(rows))*rowH)
		}
	}

	for i, r := range rows {
		y := frame.Y + float64(i)*rowH
		barY := y + (rowH-barH)/2

		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(30, 41, 59)
		label := r.Label
		if len(label) > 28 {
			label = label[:27] + "…"
		}
		pdf.Text(frame.X, y+rowH-2.2, label)

		if r.Milestone {
			// Diamond at ES.
			cx := chartX + r.ES*scale
			cy := barY + barH/2
			s := barH / 2
			pdf.SetFillColor(8, 145, 178)
			pdf.Polygon([]gofpdf.PointType{
				{X: cx, Y: cy - s}, {X: cx + s, Y: cy},
				{X: cx, Y: cy + s}, {X: cx - s, Y: cy},
			}, "F")
		} else {
			x := chartX + r.ES*scale
			w := (r.EF - r.ES) * scale
			if w < 0.8 {
				w = 0.8
			}
			if r.IsCritical {
				pdf.SetFillColor(239, 68, 68)
			} else {
				pdf.SetFillColor(34, 211, 238)
			}
			pdf.Rect(x, barY, w, barH, "F")
			if r.PercentComplete > 0 {
				pct := r.PercentComplete
				if pct > 100 {
					pct = 100
				}
				pdf.SetFillColor(15, 118, 110)
				pdf.Rect(x, barY+barH-1.2, w*pct/100, 1.2, "F")
			}
		}

		if r.StartDate != "" {
			pdf.SetFont("Helvetica", "", 6)
			pdf.SetTextColor(100, 116, 139)
			pdf.Text(chartX+r.EF*scale+1.5, y+rowH-2.5, r.StartDate+" → "+r.FinishDate)
		}
	}

	pdf.SetTextColor(0, 0, 0)
	return nil
}

// pickGridStep chooses a day-grid interval that keeps lines at least
// ~6mm apart; 0 disables the grid.
func pickGridStep(horizon, width float64) float64 {
	if horizon <= 0 || width <= 0 {
		return 0
	}
	for _, step := range []float64{1, 5, 10, 20, 50, 100} {
		if step*width/horizon >= 6 {
			return step
		}
	}
	return 0
}
