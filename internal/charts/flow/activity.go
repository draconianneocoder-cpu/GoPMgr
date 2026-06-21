// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package flow

import (
	"encoding/json"
	"errors"
)

// Activity Diagram shapes — UML 2.5 vocabulary.
const (
	ShapeInitial  = "initial"  // filled black circle
	ShapeFinal    = "final"    // bullseye (circle within circle)
	ShapeActivity = "activity" // rounded rectangle
	ShapeADecision = "a_decision" // diamond (renamed to avoid clash with workflow)
	ShapeFork     = "fork"     // horizontal bar (also used for join)
	ShapeJoin     = "join"     // horizontal bar
)

// Swimlane is one horizontal partition in the diagram. Activities are
// assigned to a swimlane via their Node.SwimlaneID field.
type Swimlane struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ActivityDocument is the JSON shape stored in db.charts.data for an
// Activity Diagram.
type ActivityDocument struct {
	Swimlanes []Swimlane `json:"swimlanes"`
	Nodes     []Node     `json:"nodes"`
	Edges     []Edge     `json:"edges"`
}

// ErrCycleActivity is returned when the activity graph contains a
// cycle.
var ErrCycleActivity = errors.New("flow: activity diagram contains a cycle")

// ParseActivity decodes JSON into an ActivityDocument.
func ParseActivity(raw string) (ActivityDocument, error) {
	if raw == "" || raw == "{}" {
		return ActivityDocument{}, nil
	}
	var doc ActivityDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return ActivityDocument{}, err
	}
	return doc, nil
}

// EncodeActivity serialises back to JSON.
func EncodeActivity(doc ActivityDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LayoutActivity produces a top-to-bottom activity layout with
// swimlanes drawn as horizontal columns.
//
// Geometry:
//
//   - Each swimlane occupies a vertical column of width SwimlaneWidth.
//     Columns are placed left-to-right in the order they appear in
//     Swimlanes.
//   - Nodes are vertically ranked by longest path from any source
//     (same as Workflow), then horizontally placed at the center of
//     their swimlane's column.
//   - If a node has no SwimlaneID (or an unknown one), it is placed
//     in a "default" column appended at the right.
//   - Initial and Final nodes are sized smaller (circles); Fork/Join
//     bars span the full swimlane width minus a margin.
func LayoutActivity(doc ActivityDocument, opt LayoutOptions) (Layout, error) {
	if len(doc.Nodes) == 0 {
		return Layout{
			Swimlanes: layoutEmptySwimlanes(doc.Swimlanes, opt),
		}, nil
	}

	_, layers, ok := layerNodes(doc.Nodes, doc.Edges)
	if !ok {
		return Layout{}, ErrCycleActivity
	}

	// Index swimlanes; preserve user-supplied order.
	laneIndex := make(map[string]int, len(doc.Swimlanes))
	for i, s := range doc.Swimlanes {
		laneIndex[s.ID] = i
	}
	defaultLaneIdx := len(doc.Swimlanes)
	totalLanes := len(doc.Swimlanes)
	if hasDefaultLane(doc) {
		totalLanes++
	}

	// Index nodes.
	byID := make(map[string]*Node, len(doc.Nodes))
	for i := range doc.Nodes {
		byID[doc.Nodes[i].ID] = &doc.Nodes[i]
	}

	rowStride := opt.NodeHeight + opt.RowGap

	out := Layout{}
	// Swimlane bands first so they render behind nodes on the canvas.
	for i, s := range doc.Swimlanes {
		out.Swimlanes = append(out.Swimlanes, SwimlaneLayout{
			ID:     s.ID,
			Name:   s.Name,
			X:      float64(i) * opt.SwimlaneWidth,
			Y:      0,
			Width:  opt.SwimlaneWidth,
			Height: 0, // patched below once we know total height
		})
	}
	if hasDefaultLane(doc) {
		out.Swimlanes = append(out.Swimlanes, SwimlaneLayout{
			ID:     "",
			Name:   "(unassigned)",
			X:      float64(defaultLaneIdx) * opt.SwimlaneWidth,
			Y:      0,
			Width:  opt.SwimlaneWidth,
			Height: 0,
		})
	}

	var maxY float64
	for li, layer := range layers {
		y := opt.SwimlaneHeaderH + float64(li)*rowStride
		for _, id := range layer {
			n := byID[id]
			laneIdx, ok := laneIndex[n.SwimlaneID]
			if !ok || n.SwimlaneID == "" {
				laneIdx = defaultLaneIdx
			}

			// Node size by shape.
			w, h := activityNodeSize(n.Shape, opt)
			x := float64(laneIdx)*opt.SwimlaneWidth + (opt.SwimlaneWidth-w)/2

			out.Nodes = append(out.Nodes, NodeLayout{
				ID:         n.ID,
				Label:      n.Label,
				Shape:      resolveActivityShape(n.Shape),
				SwimlaneID: n.SwimlaneID,
				Rank:       li,
				X:          x,
				Y:          y,
				Width:      w,
				Height:     h,
			})
			if y+h > maxY {
				maxY = y + h
			}
		}
	}

	for _, e := range doc.Edges {
		out.Edges = append(out.Edges, EdgeLayout{From: e.From, To: e.To, Label: e.Label})
	}

	// Patch swimlane heights.
	bottomY := maxY + opt.RowGap
	for i := range out.Swimlanes {
		out.Swimlanes[i].Height = bottomY
	}

	out.Width = float64(totalLanes) * opt.SwimlaneWidth
	out.Height = bottomY
	return out, nil
}

// activityNodeSize returns (width, height) for a given Activity shape.
func activityNodeSize(shape string, opt LayoutOptions) (float64, float64) {
	switch shape {
	case ShapeInitial, ShapeFinal:
		return 28, 28
	case ShapeFork, ShapeJoin:
		// A horizontal bar — wide and thin.
		return opt.SwimlaneWidth - 40, 8
	case ShapeADecision:
		return opt.NodeWidth - 30, opt.NodeHeight
	default:
		return opt.NodeWidth - 20, opt.NodeHeight
	}
}

func resolveActivityShape(s string) string {
	switch s {
	case ShapeInitial, ShapeFinal, ShapeActivity, ShapeADecision, ShapeFork, ShapeJoin:
		return s
	default:
		return ShapeActivity
	}
}

func hasDefaultLane(doc ActivityDocument) bool {
	known := make(map[string]struct{}, len(doc.Swimlanes))
	for _, s := range doc.Swimlanes {
		known[s.ID] = struct{}{}
	}
	for _, n := range doc.Nodes {
		if _, ok := known[n.SwimlaneID]; !ok || n.SwimlaneID == "" {
			return true
		}
	}
	return false
}

// layoutEmptySwimlanes returns just the swimlane columns when there
// are no nodes yet — useful so the user can see their swimlanes laid
// out even before placing activities.
func layoutEmptySwimlanes(lanes []Swimlane, opt LayoutOptions) []SwimlaneLayout {
	out := make([]SwimlaneLayout, 0, len(lanes))
	for i, s := range lanes {
		out = append(out, SwimlaneLayout{
			ID:     s.ID,
			Name:   s.Name,
			X:      float64(i) * opt.SwimlaneWidth,
			Y:      0,
			Width:  opt.SwimlaneWidth,
			Height: opt.SwimlaneHeaderH + 200,
		})
	}
	return out
}
