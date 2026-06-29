// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"encoding/json"
	"errors"
	"sort"

	"pmforge/internal/kernel"
)

// LayeredNode is the unified node shape used by Network Diagram,
// PERT Chart, and CPM Chart. Fields that aren't relevant to a given
// kind are simply ignored:
//
//   - Network only reads Label and Duration.
//   - PERT additionally reads Optimistic/MostLikely/Pessimistic and
//     writes Expected/Variance/StdDev.
//   - CPM additionally writes ES/EF/LS/LF/Float/IsCritical.
//
// Keeping one struct (rather than three) lets the frontend reuse a
// single LayeredDiagram.svelte component with kind-specific overlays.
type LayeredNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Note  string `json:"note,omitempty"`
	Owner string `json:"owner,omitempty"`

	// Network / CPM input
	Duration float64 `json:"duration,omitempty"`

	// PERT input
	Optimistic  float64 `json:"o,omitempty"`
	MostLikely  float64 `json:"m,omitempty"`
	Pessimistic float64 `json:"p,omitempty"`

	// PERT computed (filled by LayoutPERT)
	Expected float64 `json:"expected,omitempty"`
	Variance float64 `json:"variance,omitempty"`
	StdDev   float64 `json:"std_dev,omitempty"`

	// CPM computed (filled by LayoutCPM)
	ES         float64 `json:"es,omitempty"`
	EF         float64 `json:"ef,omitempty"`
	LS         float64 `json:"ls,omitempty"`
	LF         float64 `json:"lf,omitempty"`
	Float      float64 `json:"float,omitempty"`
	IsCritical bool    `json:"is_critical,omitempty"`

	// CPM calendar-anchored dates (filled by AnchorCPMDates when the
	// project has a start date). YYYY-MM-DD; empty when un-anchored.
	StartDate  string `json:"start_date,omitempty"`
	FinishDate string `json:"finish_date,omitempty"`

	// CPM scheduling constraint (input): ASAP (default), ALAP, SNET,
	// FNLT, MFO; SNET/FNLT/MFO carry ConstraintDate (YYYY-MM-DD) and
	// only take effect when the schedule is calendar-anchored.
	// ConstraintViolated is computed by the kernel.
	Constraint         string `json:"constraint,omitempty"`
	ConstraintDate     string `json:"constraint_date,omitempty"`
	ConstraintViolated bool   `json:"constraint_violated,omitempty"`

	// Progress tracking (input, reporting-only): 0..100 percent
	// complete and an explicit milestone marker, plus observed
	// actual dates (YYYY-MM-DD).
	PercentComplete float64 `json:"percent_complete,omitempty"`
	Milestone       bool    `json:"milestone,omitempty"`
	ActualStart     string  `json:"actual_start,omitempty"`
	ActualFinish    string  `json:"actual_finish,omitempty"`

	// Cost tracking for EVM (input): task budget at completion and
	// actual cost to date.
	BudgetedCost float64 `json:"budgeted_cost,omitempty"`
	ActualCost   float64 `json:"actual_cost,omitempty"`

	// Resource assignments (input) and the computed overallocation
	// flag (set by the CPM layout paths via DetectOverallocations).
	Assignments   []kernel.Assignment `json:"assignments,omitempty"`
	Overallocated bool                `json:"overallocated,omitempty"`
}

// LayeredEdge is one precedence relationship.
type LayeredEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"` // e.g. "FS", "FS+2", "SS-1"
}

// LayeredDocument is the JSON shape stored in db.charts.data for
// Network, PERT, and CPM charts.
type LayeredDocument struct {
	Nodes []LayeredNode `json:"nodes"`
	Edges []LayeredEdge `json:"edges"`
}

// ErrCycle is returned by LayoutLayered when the graph contains a
// cycle. A DAG-family chart with a cycle has no defined layering, so
// the GUI must surface this to the user rather than rendering nonsense.
var ErrCycle = errors.New("dag: graph contains a cycle")

// ParseLayered decodes a JSON blob into a LayeredDocument. Empty input
// is treated as an empty (but valid) document so brand-new charts
// don't crash the layout pass.
func ParseLayered(raw string) (LayeredDocument, error) {
	if raw == "" || raw == "{}" {
		return LayeredDocument{}, nil
	}
	var doc LayeredDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return LayeredDocument{}, err
	}
	return doc, nil
}

// EncodeLayered serialises a LayeredDocument back to JSON.
func EncodeLayered(doc LayeredDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LayeredLayoutOptions controls visual spacing.
type LayeredLayoutOptions struct {
	NodeWidth        float64 `json:"node_width"`
	NodeHeight       float64 `json:"node_height"`
	ColumnGap        float64 `json:"column_gap"`
	RowGap           float64 `json:"row_gap"`
	BarycenterPasses int     `json:"barycenter_passes"`
}

// DefaultLayeredOptions returns the spacing the GUI uses by default.
//
// NodeHeight defaults to 80 so PERT/CPM nodes have room for three
// rows of small text below the label. Network ignores the extra room.
func DefaultLayeredOptions() LayeredLayoutOptions {
	return LayeredLayoutOptions{
		NodeWidth:        160,
		NodeHeight:       80,
		ColumnGap:        56,
		RowGap:           28,
		BarycenterPasses: 6,
	}
}

// LayoutLayered computes (a) a layer index for each node via the
// longest-path-from-source algorithm, and (b) deterministic
// coordinates within each layer using a barycenter heuristic that
// reduces edge crossings.
//
// Returns ErrCycle if the graph is not acyclic.
func LayoutLayered(doc LayeredDocument, opt LayeredLayoutOptions) (Layout, error) {
	if len(doc.Nodes) == 0 {
		return Layout{}, nil
	}

	// Index by ID for fast lookup.
	byID := make(map[string]*LayeredNode, len(doc.Nodes))
	for i := range doc.Nodes {
		byID[doc.Nodes[i].ID] = &doc.Nodes[i]
	}

	// Build successor / predecessor adjacency.
	succ := make(map[string][]string, len(doc.Nodes))
	pred := make(map[string][]string, len(doc.Nodes))
	for _, e := range doc.Edges {
		if _, ok := byID[e.From]; !ok {
			continue
		}
		if _, ok := byID[e.To]; !ok {
			continue
		}
		succ[e.From] = append(succ[e.From], e.To)
		pred[e.To] = append(pred[e.To], e.From)
	}

	// Topological order via Kahn's algorithm.
	indeg := make(map[string]int, len(doc.Nodes))
	for _, n := range doc.Nodes {
		indeg[n.ID] = len(pred[n.ID])
	}
	var queue []string
	for id, d := range indeg {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	topo := make([]string, 0, len(doc.Nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		topo = append(topo, id)
		for _, s := range succ[id] {
			indeg[s]--
			if indeg[s] == 0 {
				queue = append(queue, s)
				sort.Strings(queue)
			}
		}
	}
	if len(topo) != len(doc.Nodes) {
		return Layout{}, ErrCycle
	}

	// Longest-path layering: layer[v] = max over predecessors u of (layer[u] + 1).
	layer := make(map[string]int, len(doc.Nodes))
	for _, id := range topo {
		max := 0
		for _, p := range pred[id] {
			if layer[p]+1 > max {
				max = layer[p] + 1
			}
		}
		layer[id] = max
	}

	// Bucket nodes by layer.
	maxLayer := 0
	for _, l := range layer {
		if l > maxLayer {
			maxLayer = l
		}
	}
	layers := make([][]string, maxLayer+1)
	for id, l := range layer {
		layers[l] = append(layers[l], id)
	}
	for i := range layers {
		sort.Strings(layers[i])
	}

	// Barycenter ordering passes: alternate forward/backward sweeps.
	// Each forward pass orders a layer by the mean position of its
	// predecessors in the previous layer; backward pass mirrors that
	// with successors. Convergence is typically 2-3 iterations; we
	// run BarycenterPasses to be safe.
	pos := make(map[string]int) // position-within-layer index
	for _, layerNodes := range layers {
		for i, id := range layerNodes {
			pos[id] = i
		}
	}
	if opt.BarycenterPasses < 1 {
		opt.BarycenterPasses = 1
	}
	for pass := 0; pass < opt.BarycenterPasses; pass++ {
		// Forward: order each layer by predecessor barycenter.
		for li := 1; li < len(layers); li++ {
			sort.SliceStable(layers[li], func(i, j int) bool {
				return barycenter(layers[li][i], pred, pos) <
					barycenter(layers[li][j], pred, pos)
			})
			for i, id := range layers[li] {
				pos[id] = i
			}
		}
		// Backward: order each layer by successor barycenter.
		for li := len(layers) - 2; li >= 0; li-- {
			sort.SliceStable(layers[li], func(i, j int) bool {
				return barycenter(layers[li][i], succ, pos) <
					barycenter(layers[li][j], succ, pos)
			})
			for i, id := range layers[li] {
				pos[id] = i
			}
		}
	}

	// Coordinate assignment.
	colStride := opt.NodeWidth + opt.ColumnGap
	rowStride := opt.NodeHeight + opt.RowGap

	var (
		out  Layout
		maxY float64
		maxX float64
	)
	for li, layerNodes := range layers {
		x := float64(li) * colStride
		// Vertically center each layer around y=0 so the diagram is
		// balanced when layers have different node counts.
		offsetY := -(float64(len(layerNodes)-1) * rowStride) / 2
		for pi, id := range layerNodes {
			node := byID[id]
			y := offsetY + float64(pi)*rowStride
			out.Nodes = append(out.Nodes, NodeLayout{
				ID:     node.ID,
				Title:  node.Label,
				Note:   node.Note,
				Owner:  node.Owner,
				Depth:  li,
				X:      x,
				Y:      y,
				Width:  opt.NodeWidth,
				Height: opt.NodeHeight,
			})
			if x+opt.NodeWidth > maxX {
				maxX = x + opt.NodeWidth
			}
			if y+opt.NodeHeight > maxY {
				maxY = y + opt.NodeHeight
			}
		}
	}

	// Edges pass through unchanged. The frontend renders the routing
	// (orthogonal or curved) based on the node positions.
	for _, e := range doc.Edges {
		out.Edges = append(out.Edges, EdgeLayout{From: e.From, To: e.To})
	}

	// Shift everything so coordinates are non-negative.
	if minY := findMinY(out.Nodes); minY < 0 {
		shiftY(&out, -minY)
		maxY += -minY
	}

	out.Width = maxX
	out.Height = maxY
	return out, nil
}

// barycenter returns the mean position-within-layer of `id`'s
// neighbours (predecessors on a forward pass, successors on backward).
// Nodes with no neighbours stay where they are (returns their own pos).
func barycenter(id string, neighbours map[string][]string, pos map[string]int) float64 {
	ns := neighbours[id]
	if len(ns) == 0 {
		return float64(pos[id])
	}
	sum := 0
	for _, n := range ns {
		sum += pos[n]
	}
	return float64(sum) / float64(len(ns))
}

func findMinY(nodes []NodeLayout) float64 {
	if len(nodes) == 0 {
		return 0
	}
	min := nodes[0].Y
	for _, n := range nodes[1:] {
		if n.Y < min {
			min = n.Y
		}
	}
	return min
}

func shiftY(l *Layout, dy float64) {
	for i := range l.Nodes {
		l.Nodes[i].Y += dy
	}
}

// NewLayeredNode is a convenience constructor used by the kind-specific
// wrappers to create a node with a fresh ID and a sensible label.
func NewLayeredNode(id, label string) LayeredNode {
	return LayeredNode{ID: id, Label: label}
}
