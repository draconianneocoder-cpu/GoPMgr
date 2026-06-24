// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package flow

import (
	"encoding/json"
	"errors"
)

// Workflow shapes — kept as string constants so the JSON wire format
// is stable and the frontend can switch on them.
const (
	ShapeStart      = "start"      // oval
	ShapeEnd        = "end"        // oval
	ShapeAction     = "action"     // rectangle (default)
	ShapeDecision   = "decision"   // diamond
	ShapeIO         = "io"         // parallelogram (input/output)
	ShapeSubprocess = "subprocess" // rectangle with double vertical bars
)

// WorkflowDocument is the JSON shape stored in db.charts.data for a
// Workflow Diagram.
type WorkflowDocument struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// ErrCycle is returned when the workflow graph contains a cycle. A
// flowchart can legitimately loop, but the layered layout we use here
// requires a DAG. Future work: detect back-edges and route them
// separately so loops can be rendered.
var ErrCycle = errors.New("flow: workflow contains a cycle")

// ParseWorkflow decodes JSON into a WorkflowDocument.
func ParseWorkflow(raw string) (WorkflowDocument, error) {
	if raw == "" || raw == "{}" {
		return WorkflowDocument{}, nil
	}
	var doc WorkflowDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return WorkflowDocument{}, err
	}
	return doc, nil
}

// EncodeWorkflow serialises back to JSON.
func EncodeWorkflow(doc WorkflowDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LayoutWorkflow produces a top-to-bottom flow layout.
//
// Geometry:
//
//   - Each rank (layer) is one row.
//   - Within a row, nodes are centered around x=0 with NodeWidth+ColGap
//     spacing between siblings; the canvas is then shifted so the
//     left edge of the leftmost node is at x=0.
//   - Decision nodes are diamonds, so we expand their bounding box
//     vertically a touch to keep the label readable. The Width still
//     accounts for the diamond's full footprint.
func LayoutWorkflow(doc WorkflowDocument, opt LayoutOptions) (Layout, error) {
	if len(doc.Nodes) == 0 {
		return Layout{}, nil
	}

	_, layers, ok := layerNodes(doc.Nodes, doc.Edges)
	if !ok {
		return Layout{}, ErrCycle
	}

	// Index nodes for quick lookup.
	byID := make(map[string]*Node, len(doc.Nodes))
	for i := range doc.Nodes {
		byID[doc.Nodes[i].ID] = &doc.Nodes[i]
	}

	rowStride := opt.NodeHeight + opt.RowGap
	colStride := opt.NodeWidth + opt.ColGap

	var (
		out  Layout
		minX float64
		maxX float64
		maxY float64
	)
	for li, layer := range layers {
		// Center this row around x = 0.
		offsetX := -(float64(len(layer)-1) * colStride) / 2
		y := float64(li) * rowStride
		for pi, id := range layer {
			n := byID[id]
			x := offsetX + float64(pi)*colStride
			w := opt.NodeWidth
			h := opt.NodeHeight
			// Diamond decision nodes look better square-ish, so we
			// slightly bias the dimensions when shape == decision.
			if n.Shape == ShapeDecision {
				h = opt.NodeHeight + 16
			}
			out.Nodes = append(out.Nodes, NodeLayout{
				ID:     n.ID,
				Label:  n.Label,
				Shape:  resolveWorkflowShape(n.Shape),
				Rank:   li,
				X:      x,
				Y:      y,
				Width:  w,
				Height: h,
			})
			if x < minX {
				minX = x
			}
			if x+w > maxX {
				maxX = x + w
			}
			if y+h > maxY {
				maxY = y + h
			}
		}
	}

	// Shift everything so x >= 0.
	if minX < 0 {
		dx := -minX
		for i := range out.Nodes {
			out.Nodes[i].X += dx
		}
		maxX += dx
	}

	// Pass edges through unchanged. The frontend draws them from each
	// node's bottom-center to the next node's top-center.
	for _, e := range doc.Edges {
		out.Edges = append(out.Edges, EdgeLayout(e))
	}

	out.Width = maxX
	out.Height = maxY
	return out, nil
}

// resolveWorkflowShape canonicalises shape strings. Unknown shapes
// default to "action" so older saved data with new shapes still
// renders rather than crashing.
func resolveWorkflowShape(s string) string {
	switch s {
	case ShapeStart, ShapeEnd, ShapeAction, ShapeDecision, ShapeIO, ShapeSubprocess:
		return s
	default:
		return ShapeAction
	}
}
