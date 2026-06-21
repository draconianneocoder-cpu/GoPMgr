// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"

	"github.com/jung-kurt/gofpdf"
)

// nodeLayout mirrors dag.NodeLayout / flow.NodeLayout. We redeclare
// it here (rather than importing) so the JSON parser doesn't pull
// in the dag package's full surface. The wire format is stable.
type nodeLayout struct {
	ID     string  `json:"id"`
	Number string  `json:"number,omitempty"`
	Title  string  `json:"title"`
	Note   string  `json:"note,omitempty"`
	Owner  string  `json:"owner,omitempty"`
	Depth  int     `json:"depth"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type edgeLayout struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type layoutPayload struct {
	Nodes  []nodeLayout `json:"nodes"`
	Edges  []edgeLayout `json:"edges"`
	Width  float64      `json:"width"`
	Height float64      `json:"height"`
}

// renderDAG handles every DAG-family kind (WBS, Network, PERT, CPM,
// Cause-and-Effect). Fishbone has its own renderer because its body
// shape is different.
//
// Behaviour:
//
//   - WBS / Cause-Effect: layout body is a layoutPayload directly.
//   - Network / PERT / CPM: layout body is
//     {"layout": layoutPayload, "doc": LayeredDocument}. We only need
//     the layout for drawing.
//
// We attempt to unmarshal as the wrapped form first; if that fails
// or yields no nodes we fall back to the plain form.
func renderDAG(pdf *gofpdf.Fpdf, kind string, body json.RawMessage, frame Frame) error {
	layout, err := unwrapLayout(body)
	if err != nil {
		return err
	}
	if len(layout.Nodes) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	scale, ox, oy := fit(layout.Width, layout.Height, frame.W, frame.H)

	// Draw edges first so nodes overlay them.
	pdf.SetDrawColor(110, 120, 140)
	pdf.SetLineWidth(0.3)
	byID := indexNodes(layout.Nodes)
	for _, e := range layout.Edges {
		from, fok := byID[e.From]
		to, tok := byID[e.To]
		if !fok || !tok {
			continue
		}
		// Route as an orthogonal connector through the midpoint.
		x1 := frame.X + ox + (from.X+from.Width/2)*scale
		y1 := frame.Y + oy + (from.Y+from.Height)*scale
		x2 := frame.X + ox + (to.X+to.Width/2)*scale
		y2 := frame.Y + oy + to.Y*scale
		midY := (y1 + y2) / 2

		pdf.Line(x1, y1, x1, midY)
		pdf.Line(x1, midY, x2, midY)
		pdf.Line(x2, midY, x2, y2)
	}

	// Nodes.
	for _, n := range layout.Nodes {
		drawDAGNode(pdf, kind, n, frame, scale, ox, oy)
	}
	return nil
}

// unwrapLayout tries the wrapped (layout/doc) form first, falls back
// to the bare layoutPayload form. Returns the layout regardless.
func unwrapLayout(body json.RawMessage) (layoutPayload, error) {
	var wrapped struct {
		Layout layoutPayload `json:"layout"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Layout.Nodes) > 0 {
		return wrapped.Layout, nil
	}
	var plain layoutPayload
	if err := json.Unmarshal(body, &plain); err != nil {
		return layoutPayload{}, err
	}
	return plain, nil
}

func indexNodes(ns []nodeLayout) map[string]nodeLayout {
	m := make(map[string]nodeLayout, len(ns))
	for _, n := range ns {
		m[n.ID] = n
	}
	return m
}

// drawDAGNode renders one node box with its title. WBS-family
// (and Network/CPM/PERT) all use the same box-with-text style; the
// kind argument is reserved for future per-kind styling.
func drawDAGNode(pdf *gofpdf.Fpdf, kind string, n nodeLayout, frame Frame, scale, ox, oy float64) {
	x := frame.X + ox + n.X*scale
	y := frame.Y + oy + n.Y*scale
	w := n.Width * scale
	h := n.Height * scale

	pdf.SetFillColor(30, 41, 59) // slate-800
	pdf.SetDrawColor(100, 116, 139)
	pdf.SetLineWidth(0.25)
	pdf.RoundedRect(x, y, w, h, 1.2, "1234", "FD")

	pdf.SetTextColor(241, 245, 249) // slate-100
	// Number / code label (small)
	if n.Number != "" {
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetXY(x+1, y+1)
		pdf.CellFormat(w-2, 3, n.Number, "", 0, "L", false, 0, "")
	}
	// Main title
	pdf.SetFont("Helvetica", "B", 7)
	title := truncatePDF(n.Title, int(w*1.6))
	pdf.SetXY(x+1, y+h/2-2)
	pdf.CellFormat(w-2, 4, title, "", 0, "L", false, 0, "")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
}

// truncatePDF trims a label to roughly fit a given pixel width in
// the chosen font size. Conservative; favours showing too little
// rather than overflowing the cell.
func truncatePDF(s string, maxChars int) string {
	if maxChars < 4 {
		maxChars = 4
	}
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars-1] + "…"
}

func drawEmptyChartPlaceholder(pdf *gofpdf.Fpdf, frame Frame, label string) {
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(frame.X, frame.Y+frame.H/2-3)
	pdf.CellFormat(frame.W, 6, label, "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}
