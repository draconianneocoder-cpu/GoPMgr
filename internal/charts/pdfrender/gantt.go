// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"

	"github.com/go-pdf/fpdf"
)

// ganttRow / ganttLayout mirror dag.GanttRow / dag.GanttLayout (wire
// format is stable; redeclared to keep the parser dependency-light,
// matching nodeLayout's precedent).
type ganttRow struct {
	ID              string         `json:"id"`
	Label           string         `json:"label"`
	ES              float64        `json:"es"`
	EF              float64        `json:"ef"`
	IsCritical      bool           `json:"is_critical"`
	Milestone       bool           `json:"milestone"`
	PercentComplete float64        `json:"percent_complete"`
	StartDate       string         `json:"start_date,omitempty"`
	FinishDate      string         `json:"finish_date,omitempty"`
	WorkSegments    []ganttSegment `json:"work_segments,omitempty"`
}

// ganttSegment is one absolute working-day run of a split task (same axis
// as ES/EF).
type ganttSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// rightEdge is the row's rightmost occupied offset: EF, or the last split
// segment end when the task is interrupted past its contiguous finish.
func (r ganttRow) rightEdge() float64 {
	edge := r.EF
	for _, s := range r.WorkSegments {
		if s.End > edge {
			edge = s.End
		}
	}
	return edge
}

type ganttLayout struct {
	Rows    []ganttRow `json:"rows"`
	Horizon float64    `json:"horizon"`
}

// renderGantt draws schedule bars over a day-scaled axis: label
// column, critical bars in red, progress as a darker inner bar,
// milestones as diamonds, anchored dates beside the bars.
func renderGantt(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
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
			pdf.Polygon([]fpdf.PointType{
				{X: cx, Y: cy - s}, {X: cx + s, Y: cy},
				{X: cx, Y: cy + s}, {X: cx - s, Y: cy},
			}, "F")
		} else if len(r.WorkSegments) > 0 {
			// Split (interrupted) task: one bar per working-day run, joined
			// by a dashed connector across the gaps.
			if r.IsCritical {
				pdf.SetFillColor(239, 68, 68)
				pdf.SetDrawColor(239, 68, 68)
			} else {
				pdf.SetFillColor(34, 211, 238)
				pdf.SetDrawColor(34, 211, 238)
			}
			connY := barY + barH/2
			pdf.SetDashPattern([]float64{0.6, 0.6}, 0)
			pdf.SetLineWidth(0.2)
			pdf.Line(chartX+r.WorkSegments[0].Start*scale, connY,
				chartX+r.WorkSegments[len(r.WorkSegments)-1].End*scale, connY)
			pdf.SetDashPattern([]float64{}, 0)
			for _, s := range r.WorkSegments {
				x := chartX + s.Start*scale
				w := (s.End - s.Start) * scale
				if w < 0.8 {
					w = 0.8
				}
				pdf.Rect(x, barY, w, barH, "F")
			}
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
			pdf.Text(chartX+r.rightEdge()*scale+1.5, y+rowH-2.5, r.StartDate+" → "+r.FinishDate)
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
