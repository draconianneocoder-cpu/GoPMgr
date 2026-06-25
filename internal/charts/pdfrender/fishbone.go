// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"

	"github.com/go-pdf/fpdf"
)

// fishboneNode mirrors dag.FishboneNode. We re-declare locally so
// the parse step doesn't pull in the dag package.
type fishboneNode struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"` // "effect" | "category" | "cause"
	Label  string  `json:"label"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Side   string  `json:"side,omitempty"`
}

type fishboneEdge struct {
	X1, Y1, X2, Y2 float64
	Kind           string `json:"kind"` // "spine" | "bone" | "cause"
}

type fishbonePayload struct {
	Nodes  []fishboneNode `json:"nodes"`
	Edges  []fishboneEdge `json:"edges"`
	Width  float64        `json:"width"`
	Height float64        `json:"height"`
}

func renderFishbone(pdf *fpdf.Fpdf, body json.RawMessage, frame Frame) error {
	var layout fishbonePayload
	if err := parseBody(body, &layout); err != nil {
		return err
	}
	if len(layout.Nodes) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	scale, ox, oy := fit(layout.Width, layout.Height, frame.W, frame.H)

	// Edges in three weights so the spine reads as the backbone, the
	// bones as primary structure, and the cause strokes as fine detail.
	for _, e := range layout.Edges {
		switch e.Kind {
		case "spine":
			pdf.SetDrawColor(34, 211, 238) // cyan
			pdf.SetLineWidth(0.6)
		case "bone":
			pdf.SetDrawColor(148, 163, 184)
			pdf.SetLineWidth(0.4)
		default:
			pdf.SetDrawColor(100, 116, 139)
			pdf.SetLineWidth(0.2)
		}
		pdf.Line(
			frame.X+ox+e.X1*scale, frame.Y+oy+e.Y1*scale,
			frame.X+ox+e.X2*scale, frame.Y+oy+e.Y2*scale,
		)
	}

	// Nodes by type. Effect is a filled box; category labels are
	// emphasised; causes are plain text.
	for _, n := range layout.Nodes {
		x := frame.X + ox + n.X*scale
		y := frame.Y + oy + n.Y*scale
		w := n.Width * scale
		h := n.Height * scale

		switch n.Type {
		case "effect":
			pdf.SetFillColor(14, 116, 144) // cyan-700
			pdf.SetDrawColor(34, 211, 238)
			pdf.SetLineWidth(0.4)
			pdf.RoundedRect(x, y, w, h, 1.2, "1234", "FD")
			pdf.SetFont("Helvetica", "B", 8)
			pdf.SetTextColor(241, 245, 249)
			pdf.SetXY(x, y+h/2-2)
			pdf.CellFormat(w, 4, truncatePDF(n.Label, int(w*1.6)), "", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		case "category":
			pdf.SetFont("Helvetica", "B", 7)
			pdf.SetTextColor(103, 232, 249)
			pdf.SetXY(x, y+h/2-2)
			pdf.CellFormat(w, 4, n.Label, "", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		default: // cause
			pdf.SetFont("Helvetica", "", 6)
			pdf.SetTextColor(203, 213, 225)
			pdf.SetXY(x, y+h/2-1.5)
			align := "R"
			pdf.CellFormat(w, 3, n.Label, "", 0, align, false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
	}
	pdf.SetFillColor(255, 255, 255)
	return nil
}
