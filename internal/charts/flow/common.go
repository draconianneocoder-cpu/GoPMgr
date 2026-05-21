// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package flow implements PMForge's Flow-family chart engine.
//
// The family currently covers two kinds:
//
//   - Workflow Diagram — classic process flowchart with multiple node
//     shapes (oval, rectangle, diamond, parallelogram, subprocess).
//   - Activity Diagram — UML-style flow with horizontal swimlanes
//     partitioning the diagram by actor / role.
//
// Both kinds share a top-to-bottom layered layout (rank = longest
// path from any source). Activity additionally constrains each node
// to its swimlane's horizontal band. The shared layering math lives
// in this file; per-kind specifics live in workflow.go / activity.go.
package flow

import "sort"

// Node is the engine-agnostic node primitive. Fields not relevant to a
// given kind are simply ignored:
//
//   - Workflow uses Shape (start/end/action/decision/io/subprocess).
//   - Activity uses Shape (initial/final/activity/decision/fork/join)
//     and SwimlaneID.
type Node struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Shape      string `json:"shape"`
	SwimlaneID string `json:"swimlane_id,omitempty"`
}

// Edge connects two nodes. Label is optional and drawn near the
// midpoint of the connector — used for decision branches ("Yes"/"No",
// "[approved]", etc.).
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// NodeLayout is the positioned, rendering-ready version of a Node.
type NodeLayout struct {
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

// EdgeLayout is one connector with the from/to IDs preserved.
type EdgeLayout struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// SwimlaneLayout positions one horizontal band on the canvas.
type SwimlaneLayout struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Layout is the full rendering-ready output.
type Layout struct {
	Nodes     []NodeLayout     `json:"nodes"`
	Edges     []EdgeLayout     `json:"edges"`
	Swimlanes []SwimlaneLayout `json:"swimlanes,omitempty"`
	Width     float64          `json:"width"`
	Height    float64          `json:"height"`
}

// LayoutOptions controls visual spacing for the top-down flow layout.
type LayoutOptions struct {
	NodeWidth  float64
	NodeHeight float64
	RowGap     float64
	ColGap     float64

	// Swimlane-specific. Ignored when SwimlaneCount == 0.
	SwimlaneWidth   float64
	SwimlaneHeaderH float64
}

// DefaultOptions returns the spacing the GUI uses by default.
func DefaultOptions() LayoutOptions {
	return LayoutOptions{
		NodeWidth:       150,
		NodeHeight:      60,
		RowGap:          40,
		ColGap:          40,
		SwimlaneWidth:   200,
		SwimlaneHeaderH: 30,
	}
}

// layerNodes performs a longest-path-from-sources layering over the
// given (nodes, edges) DAG. Returns:
//
//   - ranks   map[nodeID]rank
//   - layers  []layer, each a sorted slice of node IDs at that rank
//   - ok      false if the graph has a cycle
func layerNodes(nodes []Node, edges []Edge) (ranks map[string]int, layers [][]string, ok bool) {
	ranks = make(map[string]int, len(nodes))

	// Build adjacency (successor + predecessor) limited to known nodes.
	known := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		known[n.ID] = struct{}{}
	}
	succ := make(map[string][]string, len(nodes))
	pred := make(map[string][]string, len(nodes))
	for _, e := range edges {
		if _, k1 := known[e.From]; !k1 {
			continue
		}
		if _, k2 := known[e.To]; !k2 {
			continue
		}
		succ[e.From] = append(succ[e.From], e.To)
		pred[e.To] = append(pred[e.To], e.From)
	}

	// Kahn's algorithm — collect nodes in topological order, then
	// assign rank = max(pred.rank) + 1.
	indeg := make(map[string]int, len(nodes))
	for _, n := range nodes {
		indeg[n.ID] = len(pred[n.ID])
		ranks[n.ID] = 0
	}

	var queue []string
	for id, d := range indeg {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++

		// Rank: max predecessor rank + 1, or 0 if no predecessors.
		maxR := 0
		hasPred := false
		for _, p := range pred[id] {
			hasPred = true
			if ranks[p]+1 > maxR {
				maxR = ranks[p] + 1
			}
		}
		if hasPred {
			ranks[id] = maxR
		}

		for _, s := range succ[id] {
			indeg[s]--
			if indeg[s] == 0 {
				queue = append(queue, s)
				sort.Strings(queue)
			}
		}
	}

	if visited != len(nodes) {
		return nil, nil, false // cycle
	}

	// Bucket by rank.
	maxRank := 0
	for _, r := range ranks {
		if r > maxRank {
			maxRank = r
		}
	}
	layers = make([][]string, maxRank+1)
	for id, r := range ranks {
		layers[r] = append(layers[r], id)
	}
	for i := range layers {
		sort.Strings(layers[i])
	}
	return ranks, layers, true
}
