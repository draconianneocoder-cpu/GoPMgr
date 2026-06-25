// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"

	"github.com/go-pdf/fpdf"
)

// flowNode mirrors flow.NodeLayout.
type flowNode struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Shape      string  `json:"shape"`
	SwimlaneID string  `json:"swimlane_id,omitempty"`
	Rank       int     `json:"rank"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
}

type flowEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

type flowSwimlane struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type flowPayload struct {
	Nodes     []flowNode     `json:"nodes"`
	Edges     []flowEdge     `json:"edges"`
	Swimlanes []flowSwimlane `json:"swimlanes,omitempty"`
	Width     float64        `json:"width"`
	Height    float64        `json:"height"`
}

func renderFlow(pdf *fpdf.Fpdf, kind string, body json.RawMessage, frame Frame) error {
	var layout flowPayload
	if err := parseBody(body, &layout); err != nil {
		return err
	}
	if len(layout.Nodes) == 0 && len(layout.Swimlanes) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	scale, ox, oy := fit(layout.Width, layout.Height, frame.W, frame.H)

	// Swimlanes (Activity diagrams only) go down first.
	for i, s := range layout.Swimlanes {
		x := frame.X + ox + s.X*scale
		y := frame.Y + oy + s.Y*scale
		w := s.Width * scale
		h := s.Height * scale
		if i%2 == 0 {
			pdf.SetFillColor(15, 23, 42)
		} else {
			pdf.SetFillColor(17, 24, 39)
		}
		pdf.SetDrawColor(51, 65, 85)
		pdf.SetLineWidth(0.2)
		pdf.Rect(x, y, w, h, "FD")
		// Header strip
		pdf.SetFillColor(30, 41, 59)
		pdf.Rect(x, y, w, 6*scale, "FD")
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetTextColor(103, 232, 249)
		pdf.SetXY(x, y+1)
		pdf.CellFormat(w, 4, s.Name, "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	// Edges (orthogonal routing, same as DAG).
	pdf.SetDrawColor(100, 116, 139)
	pdf.SetLineWidth(0.3)
	byID := make(map[string]flowNode, len(layout.Nodes))
	for _, n := range layout.Nodes {
		byID[n.ID] = n
	}
	for _, e := range layout.Edges {
		from, fok := byID[e.From]
		to, tok := byID[e.To]
		if !fok || !tok {
			continue
		}
		x1 := frame.X + ox + (from.X+from.Width/2)*scale
		y1 := frame.Y + oy + (from.Y+from.Height)*scale
		x2 := frame.X + ox + (to.X+to.Width/2)*scale
		y2 := frame.Y + oy + to.Y*scale
		midY := (y1 + y2) / 2
		pdf.Line(x1, y1, x1, midY)
		pdf.Line(x1, midY, x2, midY)
		pdf.Line(x2, midY, x2, y2)
		if e.Label != "" {
			pdf.SetFont("Helvetica", "", 5)
			pdf.SetTextColor(203, 213, 225)
			pdf.SetXY((x1+x2)/2+0.5, midY-1.5)
			pdf.CellFormat(15, 2, e.Label, "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
	}

	// Nodes with shape-by-type rendering.
	for _, n := range layout.Nodes {
		drawFlowNode(pdf, n, frame, scale, ox, oy)
	}
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// drawFlowNode renders one node using the appropriate shape for
// its `shape` field. This is the PDF equivalent of the SVG
// shapePath() function used by the frontend editors.
func drawFlowNode(pdf *fpdf.Fpdf, n flowNode, frame Frame, scale, ox, oy float64) {
	x := frame.X + ox + n.X*scale
	y := frame.Y + oy + n.Y*scale
	w := n.Width * scale
	h := n.Height * scale

	// Pick fill colour by shape.
	switch n.Shape {
	case "start":
		pdf.SetFillColor(22, 163, 74)
	case "end":
		pdf.SetFillColor(127, 29, 29)
	case "decision", "a_decision":
		pdf.SetFillColor(161, 98, 7)
	case "io":
		pdf.SetFillColor(30, 64, 175)
	case "subprocess":
		pdf.SetFillColor(49, 46, 129)
	case "initial", "fork", "join":
		pdf.SetFillColor(241, 245, 249)
	case "final":
		pdf.SetFillColor(30, 41, 59)
	default:
		pdf.SetFillColor(30, 41, 59)
	}
	pdf.SetDrawColor(100, 116, 139)
	pdf.SetLineWidth(0.25)

	switch n.Shape {
	case "start", "end":
		// Oval = rounded rect with rx=h/2
		pdf.RoundedRect(x, y, w, h, h/2, "1234", "FD")
	case "decision", "a_decision":
		// Diamond
		pts := []fpdf.PointType{
			{X: x + w/2, Y: y},
			{X: x + w, Y: y + h/2},
			{X: x + w/2, Y: y + h},
			{X: x, Y: y + h/2},
		}
		drawPolygon(pdf, pts, "FD")
	case "io":
		// Parallelogram (slant = h/3)
		s := h / 3
		pts := []fpdf.PointType{
			{X: x + s, Y: y},
			{X: x + w, Y: y},
			{X: x + w - s, Y: y + h},
			{X: x, Y: y + h},
		}
		drawPolygon(pdf, pts, "FD")
	case "initial":
		// Filled circle
		pdf.Circle(x+w/2, y+h/2, mind(w, h)/2, "FD")
	case "final":
		// Bullseye: outer circle (dark), inner filled circle (light)
		pdf.Circle(x+w/2, y+h/2, mind(w, h)/2, "FD")
		pdf.SetFillColor(241, 245, 249)
		pdf.Circle(x+w/2, y+h/2, mind(w, h)/4, "F")
	case "fork", "join":
		pdf.Rect(x, y, w, h, "F")
	default:
		pdf.RoundedRect(x, y, w, h, 1.0, "1234", "FD")
	}

	// Label (skip for initial/final).
	if n.Shape != "initial" && n.Shape != "final" && n.Label != "" {
		switch n.Shape {
		case "start", "end", "io", "subprocess":
			pdf.SetTextColor(241, 245, 249)
		case "decision", "a_decision":
			pdf.SetTextColor(254, 243, 199)
		default:
			pdf.SetTextColor(241, 245, 249)
		}
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetXY(x, y+h/2-2)
		pdf.CellFormat(w, 4, truncatePDF(n.Label, int(w*1.6)), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
}

// drawPolygon draws a closed filled+stroked polygon. fpdf has
// fpdf.PolyLine for open polylines but not a one-call polygon
// helper, so we trace it manually with Line() and rely on the
// caller's SetFillColor / SetDrawColor.
func drawPolygon(pdf *fpdf.Fpdf, pts []fpdf.PointType, _ string) {
	if len(pts) < 3 {
		return
	}
	// Use the built-in Polygon helper from fpdf when available
	// (it is, via fpdf.Polygon).
	pdf.Polygon(pts, "FD")
}

func mind(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
