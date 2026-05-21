// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dag implements the DAG (directed-acyclic-graph) chart engine
// shared by WBS, Network Diagram, PERT, CPM, Fishbone, and
// Cause-and-Effect charts.
//
// The fully-implemented engine in this package is the Work Breakdown
// Structure (WBS): hierarchical decomposition of project scope.
// Other DAG-family kinds reuse this package's NodeLayout/EdgeLayout
// types as their output format so the Svelte frontend has one renderer
// to write.
package dag

import (
	"encoding/json"
	"errors"
	"strings"
)

// WBSNode is one node in a WBS tree. The structure is recursive: each
// node has its own children. PMI's WBS-numbering convention
// ("1", "1.1", "1.1.2") is enforced by the engine via Renumber().
type WBSNode struct {
	ID       string     `json:"id"`
	Number   string     `json:"number,omitempty"`  // dotted WBS code, e.g. "1.2.3"
	Title    string     `json:"title"`
	Note     string     `json:"note,omitempty"`
	Owner    string     `json:"owner,omitempty"`
	Effort   float64    `json:"effort,omitempty"`   // person-days or generic units
	Children []*WBSNode `json:"children,omitempty"`
}

// WBSDocument is the JSON shape stored in db.charts.data for a WBS.
type WBSDocument struct {
	Root *WBSNode `json:"root"`
}

// ErrEmptyTree is returned when a WBSDocument has no root.
var ErrEmptyTree = errors.New("dag: WBS document has no root node")

// Parse decodes a JSON `data` blob from the charts table into a
// WBSDocument. Validates that the tree has a single root.
func Parse(raw string) (WBSDocument, error) {
	if raw == "" {
		return WBSDocument{}, ErrEmptyTree
	}
	var doc WBSDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return WBSDocument{}, err
	}
	if doc.Root == nil {
		return WBSDocument{}, ErrEmptyTree
	}
	return doc, nil
}

// Encode serialises a WBSDocument back into JSON suitable for storage.
func Encode(doc WBSDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Renumber assigns dotted WBS codes ("1", "1.1", "1.2", "1.2.1", ...)
// to every node in the tree, in pre-order. The root is always "1".
func Renumber(doc *WBSDocument) {
	if doc == nil || doc.Root == nil {
		return
	}
	renumberRec(doc.Root, "1")
}

func renumberRec(n *WBSNode, prefix string) {
	n.Number = prefix
	for i, c := range n.Children {
		renumberRec(c, prefix+"."+itoa(i+1))
	}
}

// FlattenLeaves returns every leaf (childless) node in pre-order. PMI
// calls these "work packages" — the level at which effort is estimated
// and tasks scheduled.
func FlattenLeaves(doc WBSDocument) []*WBSNode {
	var out []*WBSNode
	walk(doc.Root, func(n *WBSNode) {
		if len(n.Children) == 0 {
			out = append(out, n)
		}
	})
	return out
}

// TotalEffort returns the sum of Effort across every leaf node.
func TotalEffort(doc WBSDocument) float64 {
	var sum float64
	for _, leaf := range FlattenLeaves(doc) {
		sum += leaf.Effort
	}
	return sum
}

// ----- Layout (used by the Svelte renderer) -----

// NodeLayout is one positioned node ready for the frontend to draw.
type NodeLayout struct {
	ID     string  `json:"id"`
	Number string  `json:"number"`
	Title  string  `json:"title"`
	Note   string  `json:"note,omitempty"`
	Owner  string  `json:"owner,omitempty"`
	Depth  int     `json:"depth"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// EdgeLayout is one parent→child link.
type EdgeLayout struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// LayoutOptions controls the WBS layout.
type LayoutOptions struct {
	HorizontalSpacing float64 `json:"horizontal_spacing"`
	VerticalSpacing   float64 `json:"vertical_spacing"`
	NodeWidth         float64 `json:"node_width"`
	NodeHeight        float64 `json:"node_height"`
}

// DefaultLayoutOptions returns the layout numbers the GUI uses by default.
func DefaultLayoutOptions() LayoutOptions {
	return LayoutOptions{
		HorizontalSpacing: 32,
		VerticalSpacing:   24,
		NodeWidth:         180,
		NodeHeight:        56,
	}
}

// Layout is the result returned to the frontend renderer.
type Layout struct {
	Nodes  []NodeLayout `json:"nodes"`
	Edges  []EdgeLayout `json:"edges"`
	Width  float64      `json:"width"`
	Height float64      `json:"height"`
}

// LayoutWBS produces a top-down hierarchical layout (root at top,
// leaves at the bottom). Sibling nodes are spaced evenly under their
// parent; the overall horizontal extent of a subtree is computed
// recursively so siblings never overlap.
func LayoutWBS(doc WBSDocument, opt LayoutOptions) Layout {
	if doc.Root == nil {
		return Layout{}
	}
	// Pass 1: compute subtree widths.
	widths := map[string]float64{}
	subtreeWidth(doc.Root, opt, widths)

	// Pass 2: assign coordinates depth-first.
	var (
		nodes []NodeLayout
		edges []EdgeLayout
		maxX  float64
		maxY  float64
	)
	assign(doc.Root, 0, 0, opt, widths, "", &nodes, &edges, &maxX, &maxY)

	return Layout{
		Nodes:  nodes,
		Edges:  edges,
		Width:  maxX + opt.NodeWidth,
		Height: maxY + opt.NodeHeight,
	}
}

func subtreeWidth(n *WBSNode, opt LayoutOptions, out map[string]float64) float64 {
	if len(n.Children) == 0 {
		out[n.ID] = opt.NodeWidth
		return out[n.ID]
	}
	var total float64
	for i, c := range n.Children {
		if i > 0 {
			total += opt.HorizontalSpacing
		}
		total += subtreeWidth(c, opt, out)
	}
	if total < opt.NodeWidth {
		total = opt.NodeWidth
	}
	out[n.ID] = total
	return total
}

func assign(
	n *WBSNode, depth int, leftX float64,
	opt LayoutOptions, widths map[string]float64,
	parentID string, nodes *[]NodeLayout, edges *[]EdgeLayout,
	maxX, maxY *float64,
) {
	w := widths[n.ID]
	centerX := leftX + w/2
	y := float64(depth) * (opt.NodeHeight + opt.VerticalSpacing)

	*nodes = append(*nodes, NodeLayout{
		ID:     n.ID,
		Number: n.Number,
		Title:  n.Title,
		Note:   n.Note,
		Owner:  n.Owner,
		Depth:  depth,
		X:      centerX - opt.NodeWidth/2,
		Y:      y,
		Width:  opt.NodeWidth,
		Height: opt.NodeHeight,
	})
	if parentID != "" {
		*edges = append(*edges, EdgeLayout{From: parentID, To: n.ID})
	}

	if centerX+opt.NodeWidth/2 > *maxX {
		*maxX = centerX + opt.NodeWidth/2
	}
	if y > *maxY {
		*maxY = y
	}

	// Recurse over children.
	cursor := leftX
	for _, c := range n.Children {
		cw := widths[c.ID]
		assign(c, depth+1, cursor, opt, widths, n.ID, nodes, edges, maxX, maxY)
		cursor += cw + opt.HorizontalSpacing
	}
}

func walk(n *WBSNode, visit func(*WBSNode)) {
	if n == nil {
		return
	}
	visit(n)
	for _, c := range n.Children {
		walk(c, visit)
	}
}

// itoa avoids importing strconv just for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	b.Write(digits)
	return b.String()
}
