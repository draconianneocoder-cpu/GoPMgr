// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"encoding/json"
	"errors"
)

// Cause-and-Effect (generic) Diagram
// ===================================
//
// A more flexible alternative to Fishbone. Where Fishbone forces a
// strict effect → category → cause hierarchy, this kind models the
// problem as a tree of causes of arbitrary depth (e.g., the 5 Whys
// drill-down) growing leftward from the central effect.
//
// Internally it reuses the WBS layout from wbs.go (left-to-right
// rather than top-down), so callers benefit from the same subtree-
// width algorithm.

// CauseNode is one cause in the tree.
type CauseNode struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Note     string       `json:"note,omitempty"`
	Children []*CauseNode `json:"children,omitempty"`
}

// CausalTreeDocument is the JSON shape stored in db.charts.data.
type CausalTreeDocument struct {
	Effect string     `json:"effect"`
	Root   *CauseNode `json:"root"` // root cause; usually one node whose children are first-level causes
}

// ErrNoRoot is returned by ParseCausalTree when the document has no
// root cause node.
var ErrNoRoot = errors.New("dag: causal tree has no root")

// ParseCausalTree decodes a JSON blob into a CausalTreeDocument.
func ParseCausalTree(raw string) (CausalTreeDocument, error) {
	if raw == "" || raw == "{}" {
		return CausalTreeDocument{}, nil
	}
	var doc CausalTreeDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return CausalTreeDocument{}, err
	}
	return doc, nil
}

// EncodeCausalTree serialises back to JSON.
func EncodeCausalTree(doc CausalTreeDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LayoutCausalTree produces a left-to-right tree layout. The root
// (the effect) sits at the far right; causes branch leftward, each
// child node above/below its parent. The layout reuses the WBS
// subtree-width algorithm with axes swapped.
func LayoutCausalTree(doc CausalTreeDocument) (Layout, error) {
	if doc.Root == nil {
		return Layout{}, ErrNoRoot
	}
	opt := DefaultLayoutOptions() // borrowed from WBS

	// We need two passes:
	//   Pass 1 — compute subtreeHeight (vertical extent) of each node.
	//   Pass 2 — assign coordinates with x = depth*column_stride
	//             (mirrored so root is on the right).
	heights := map[string]float64{}
	causalHeight(doc.Root, opt, heights)

	maxDepth := 0
	measureDepth(doc.Root, 0, &maxDepth)

	colStride := opt.NodeWidth + opt.HorizontalSpacing
	canvasWidth := float64(maxDepth+1)*colStride + 60
	canvasHeight := heights[doc.Root.ID]

	var (
		nodes []NodeLayout
		edges []EdgeLayout
	)
	assignCausal(doc.Root, 0, 0, opt, heights, "", canvasWidth, &nodes, &edges)

	return Layout{
		Nodes:  nodes,
		Edges:  edges,
		Width:  canvasWidth,
		Height: canvasHeight,
	}, nil
}

func causalHeight(n *CauseNode, opt LayoutOptions, out map[string]float64) float64 {
	if len(n.Children) == 0 {
		out[n.ID] = opt.NodeHeight
		return out[n.ID]
	}
	var total float64
	for i, c := range n.Children {
		if i > 0 {
			total += opt.VerticalSpacing
		}
		total += causalHeight(c, opt, out)
	}
	if total < opt.NodeHeight {
		total = opt.NodeHeight
	}
	out[n.ID] = total
	return total
}

func measureDepth(n *CauseNode, depth int, max *int) {
	if depth > *max {
		*max = depth
	}
	for _, c := range n.Children {
		measureDepth(c, depth+1, max)
	}
}

func assignCausal(
	n *CauseNode, depth int, topY float64,
	opt LayoutOptions, heights map[string]float64,
	parentID string, canvasWidth float64,
	nodes *[]NodeLayout, edges *[]EdgeLayout,
) {
	h := heights[n.ID]
	// Mirror x so root sits on the right.
	colStride := opt.NodeWidth + opt.HorizontalSpacing
	x := canvasWidth - float64(depth+1)*colStride
	y := topY + h/2 - opt.NodeHeight/2

	*nodes = append(*nodes, NodeLayout{
		ID:     n.ID,
		Title:  n.Label,
		Note:   n.Note,
		Depth:  depth,
		X:      x,
		Y:      y,
		Width:  opt.NodeWidth,
		Height: opt.NodeHeight,
	})
	if parentID != "" {
		*edges = append(*edges, EdgeLayout{From: parentID, To: n.ID})
	}

	cursor := topY
	for _, c := range n.Children {
		ch := heights[c.ID]
		assignCausal(c, depth+1, cursor, opt, heights, n.ID, canvasWidth, nodes, edges)
		cursor += ch + opt.VerticalSpacing
	}
}
