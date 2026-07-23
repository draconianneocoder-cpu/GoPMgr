// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

// Tests for wbs.go, fishbone.go, causal_tree.go, layered.go, and
// network.go. PERT tests live in pert_test.go; the `within` helper
// defined there is shared across all files in this package.

import (
	"errors"
	"testing"
)

// ===== WBS (wbs.go) =====

func TestParse_EmptyString(t *testing.T) {
	_, err := Parse("")
	if !errors.Is(err, ErrEmptyTree) {
		t.Errorf("expected ErrEmptyTree, got %v", err)
	}
}

func TestParse_NilRoot(t *testing.T) {
	_, err := Parse(`{"root":null}`)
	if !errors.Is(err, ErrEmptyTree) {
		t.Errorf("expected ErrEmptyTree for null root, got %v", err)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse("{bad json}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParse_ValidDocument(t *testing.T) {
	raw := `{"root":{"id":"r1","title":"Project","children":[{"id":"c1","title":"Phase 1"}]}}`
	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Root == nil {
		t.Fatal("Root should not be nil")
	}
	if doc.Root.ID != "r1" || doc.Root.Title != "Project" {
		t.Errorf("root fields: got {%q, %q}", doc.Root.ID, doc.Root.Title)
	}
	if len(doc.Root.Children) != 1 || doc.Root.Children[0].ID != "c1" {
		t.Errorf("children: got %+v", doc.Root.Children)
	}
}

func TestRenumber_SingleNode(t *testing.T) {
	doc := &WBSDocument{Root: &WBSNode{ID: "r"}}
	Renumber(doc)
	if doc.Root.Number != "1" {
		t.Errorf("root number: got %q, want %q", doc.Root.Number, "1")
	}
}

func TestRenumber_TwoChildren(t *testing.T) {
	doc := &WBSDocument{
		Root: &WBSNode{
			ID: "r",
			Children: []*WBSNode{
				{ID: "c1"},
				{ID: "c2"},
			},
		},
	}
	Renumber(doc)
	if doc.Root.Number != "1" {
		t.Errorf("root: got %q, want %q", doc.Root.Number, "1")
	}
	if doc.Root.Children[0].Number != "1.1" {
		t.Errorf("c1: got %q, want %q", doc.Root.Children[0].Number, "1.1")
	}
	if doc.Root.Children[1].Number != "1.2" {
		t.Errorf("c2: got %q, want %q", doc.Root.Children[1].Number, "1.2")
	}
}

func TestRenumber_DeepHierarchy(t *testing.T) {
	grandchild := &WBSNode{ID: "gc"}
	child := &WBSNode{ID: "c", Children: []*WBSNode{grandchild}}
	root := &WBSNode{ID: "r", Children: []*WBSNode{child}}
	Renumber(&WBSDocument{Root: root})
	if grandchild.Number != "1.1.1" {
		t.Errorf("grandchild number: got %q, want %q", grandchild.Number, "1.1.1")
	}
}

func TestRenumber_NilDoc_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Renumber panicked: %v", r)
		}
	}()
	Renumber(nil)
	Renumber(&WBSDocument{})
}

func TestFlattenLeaves_SingleRoot_IsLeaf(t *testing.T) {
	doc := WBSDocument{Root: &WBSNode{ID: "r"}}
	leaves := FlattenLeaves(doc)
	if len(leaves) != 1 || leaves[0].ID != "r" {
		t.Errorf("expected [r], got %v", leaves)
	}
}

func TestFlattenLeaves_TwoChildren_ParentExcluded(t *testing.T) {
	doc := WBSDocument{
		Root: &WBSNode{
			ID: "r",
			Children: []*WBSNode{
				{ID: "c1"},
				{ID: "c2"},
			},
		},
	}
	leaves := FlattenLeaves(doc)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves, got %d", len(leaves))
	}
	ids := map[string]bool{leaves[0].ID: true, leaves[1].ID: true}
	if !ids["c1"] || !ids["c2"] {
		t.Errorf("expected c1 and c2 as leaves, got %v", leaves)
	}
	for _, l := range leaves {
		if l.ID == "r" {
			t.Error("parent should not appear in leaves")
		}
	}
}

func TestTotalEffort_SumOfLeafEfforts(t *testing.T) {
	doc := WBSDocument{
		Root: &WBSNode{
			ID: "r", Effort: 99, // parent effort is not counted
			Children: []*WBSNode{
				{ID: "c1", Effort: 3},
				{ID: "c2", Effort: 7},
			},
		},
	}
	got := TotalEffort(doc)
	within(t, "TotalEffort", got, 10.0)
}

func TestLayoutWBS_NilRoot(t *testing.T) {
	layout := LayoutWBS(WBSDocument{}, DefaultLayoutOptions())
	if len(layout.Nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(layout.Nodes))
	}
}

func TestLayoutWBS_SingleNode(t *testing.T) {
	doc := WBSDocument{Root: &WBSNode{ID: "r", Title: "Root"}}
	opt := DefaultLayoutOptions()
	layout := LayoutWBS(doc, opt)
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}
	n := layout.Nodes[0]
	if n.Depth != 0 {
		t.Errorf("Depth: got %d, want 0", n.Depth)
	}
	if n.X < 0 || n.Y < 0 {
		t.Errorf("X/Y should be non-negative: got (%v, %v)", n.X, n.Y)
	}
	if layout.Width <= 0 || layout.Height <= 0 {
		t.Errorf("Width/Height should be positive: got (%v, %v)", layout.Width, layout.Height)
	}
}

func TestLayoutWBS_ParentChildEdge(t *testing.T) {
	doc := WBSDocument{
		Root: &WBSNode{
			ID:       "r",
			Children: []*WBSNode{{ID: "c1"}, {ID: "c2"}},
		},
	}
	layout := LayoutWBS(doc, DefaultLayoutOptions())
	if len(layout.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(layout.Nodes))
	}
	if len(layout.Edges) != 2 {
		t.Fatalf("expected 2 edges (r→c1, r→c2), got %d", len(layout.Edges))
	}
}

func TestItoa(t *testing.T) {
	cases := []struct {
		in  int
		out string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
	}
	for _, c := range cases {
		if got := itoa(c.in); got != c.out {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.out)
		}
	}
}

// ===== Layered layout (layered.go) =====

func TestParseLayered_Empty(t *testing.T) {
	doc, err := ParseLayered("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Nodes) != 0 || len(doc.Edges) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseLayered_InvalidJSON(t *testing.T) {
	_, err := ParseLayered("{bad}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLayoutLayered_Empty(t *testing.T) {
	layout, err := LayoutLayered(LayeredDocument{}, DefaultLayeredOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(layout.Nodes))
	}
}

func TestLayoutLayered_SingleNode(t *testing.T) {
	doc := LayeredDocument{Nodes: []LayeredNode{{ID: "A", Label: "Start"}}}
	layout, err := LayoutLayered(doc, DefaultLayeredOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}
	n := layout.Nodes[0]
	if n.Depth != 0 {
		t.Errorf("Depth: got %d, want 0", n.Depth)
	}
	if n.Y < 0 {
		t.Errorf("Y should be non-negative, got %v", n.Y)
	}
}

func TestLayoutLayered_LinearChain_DepthIncreases(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A"}, {ID: "B"}},
		Edges: []LayeredEdge{{From: "A", To: "B"}},
	}
	layout, err := LayoutLayered(doc, DefaultLayeredOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := make(map[string]NodeLayout)
	for _, n := range layout.Nodes {
		byID[n.ID] = n
	}
	if byID["A"].Depth != 0 {
		t.Errorf("A Depth: got %d, want 0", byID["A"].Depth)
	}
	if byID["B"].Depth != 1 {
		t.Errorf("B Depth: got %d, want 1", byID["B"].Depth)
	}
	if byID["B"].X <= byID["A"].X {
		t.Errorf("B.X (%v) should be greater than A.X (%v)", byID["B"].X, byID["A"].X)
	}
}

func TestLayoutLayered_Cycle_ReturnsErrCycle(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A"}, {ID: "B"}},
		Edges: []LayeredEdge{{From: "A", To: "B"}, {From: "B", To: "A"}},
	}
	_, err := LayoutLayered(doc, DefaultLayeredOptions())
	if !errors.Is(err, ErrCycle) {
		t.Errorf("expected ErrCycle, got %v", err)
	}
}

func TestLayoutLayered_AllYNonNegative(t *testing.T) {
	// Two nodes with no edges land in the same layer. The offset is
	// negative before the shiftY pass — verify that pass applies.
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A"}, {ID: "B"}},
	}
	layout, err := LayoutLayered(doc, DefaultLayeredOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range layout.Nodes {
		if n.Y < 0 {
			t.Errorf("node %q has negative Y=%v", n.ID, n.Y)
		}
	}
}

func TestBarycenter_NoNeighbours_ReturnsSelfPos(t *testing.T) {
	pos := map[string]int{"A": 3}
	got := barycenter("A", nil, pos)
	within(t, "barycenter no neighbours", got, 3.0)
}

func TestBarycenter_WithNeighbours_ReturnsMean(t *testing.T) {
	pos := map[string]int{"B": 0, "C": 4}
	neighbours := map[string][]string{"A": {"B", "C"}}
	got := barycenter("A", neighbours, pos)
	within(t, "barycenter mean", got, 2.0)
}

func TestFindMinY_Empty_ReturnsZero(t *testing.T) {
	got := findMinY(nil)
	within(t, "findMinY empty", got, 0.0)
}

func TestFindMinY_NegativeY_ReturnsMin(t *testing.T) {
	nodes := []NodeLayout{{Y: 5}, {Y: -3}, {Y: 2}}
	got := findMinY(nodes)
	within(t, "findMinY negative", got, -3.0)
}

// ===== Fishbone (fishbone.go) =====

func TestParseFishbone_Empty(t *testing.T) {
	doc, err := ParseFishbone("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Effect != "" || len(doc.Categories) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseFishbone_InvalidJSON(t *testing.T) {
	_, err := ParseFishbone("{bad}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLayoutFishbone_NoCategories_JustEffect(t *testing.T) {
	doc := FishboneDocument{Effect: "High defect rate"}
	layout := LayoutFishbone(doc, DefaultFishboneOptions())
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node (the effect), got %d", len(layout.Nodes))
	}
	if layout.Nodes[0].Type != "effect" {
		t.Errorf("node type: got %q, want %q", layout.Nodes[0].Type, "effect")
	}
	if layout.Nodes[0].Label != "High defect rate" {
		t.Errorf("effect label: got %q", layout.Nodes[0].Label)
	}
}

func TestLayoutFishbone_WithCategory_EffectPresent(t *testing.T) {
	doc := FishboneDocument{
		Effect: "Defect",
		Categories: []FishboneCategory{
			{Name: "People", Causes: []string{"Untrained", "Overworked"}},
		},
	}
	layout := LayoutFishbone(doc, DefaultFishboneOptions())

	var effectNode *FishboneNode
	for i := range layout.Nodes {
		if layout.Nodes[i].Type == "effect" {
			effectNode = &layout.Nodes[i]
			break
		}
	}
	if effectNode == nil {
		t.Fatal("effect node not found in layout")
		return
	}
	if effectNode.Label != "Defect" {
		t.Errorf("effect label: got %q", effectNode.Label)
	}
}

func TestLayoutFishbone_NodeCounts(t *testing.T) {
	// 1 category with 2 causes → effect(1) + category(1) + cause(2) = 4 nodes
	doc := FishboneDocument{
		Effect: "E",
		Categories: []FishboneCategory{
			{Name: "Cat1", Causes: []string{"C1", "C2"}},
		},
	}
	layout := LayoutFishbone(doc, DefaultFishboneOptions())
	if len(layout.Nodes) != 4 {
		t.Errorf("expected 4 nodes (1 effect + 1 category + 2 causes), got %d", len(layout.Nodes))
	}
}

func TestLayoutFishbone_CanvasSizePositive(t *testing.T) {
	doc := FishboneDocument{
		Effect:     "Problem",
		Categories: []FishboneCategory{{Name: "Process"}},
	}
	layout := LayoutFishbone(doc, DefaultFishboneOptions())
	if layout.Width <= 0 || layout.Height <= 0 {
		t.Errorf("canvas should be positive, got (%v, %v)", layout.Width, layout.Height)
	}
}

// ===== Causal Tree (causal_tree.go) =====

func TestParseCausalTree_Empty(t *testing.T) {
	doc, err := ParseCausalTree("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Root != nil || doc.Effect != "" {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseCausalTree_InvalidJSON(t *testing.T) {
	_, err := ParseCausalTree("{bad}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLayoutCausalTree_NilRoot_ReturnsErrNoRoot(t *testing.T) {
	_, err := LayoutCausalTree(CausalTreeDocument{Effect: "Problem"})
	if !errors.Is(err, ErrNoRoot) {
		t.Errorf("expected ErrNoRoot, got %v", err)
	}
}

func TestLayoutCausalTree_SingleNode(t *testing.T) {
	doc := CausalTreeDocument{
		Effect: "Failure",
		Root:   &CauseNode{ID: "root", Label: "Root Cause"},
	}
	layout, err := LayoutCausalTree(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}
	if len(layout.Edges) != 0 {
		t.Errorf("expected no edges for single node, got %d", len(layout.Edges))
	}
	if layout.Width <= 0 || layout.Height <= 0 {
		t.Errorf("canvas should be positive, got (%v, %v)", layout.Width, layout.Height)
	}
}

func TestLayoutCausalTree_RootWithChildren_HasEdges(t *testing.T) {
	doc := CausalTreeDocument{
		Effect: "Failure",
		Root: &CauseNode{
			ID: "root",
			Children: []*CauseNode{
				{ID: "c1", Label: "Cause A"},
				{ID: "c2", Label: "Cause B"},
			},
		},
	}
	layout, err := LayoutCausalTree(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(layout.Nodes))
	}
	if len(layout.Edges) != 2 {
		t.Fatalf("expected 2 edges (root→c1, root→c2), got %d", len(layout.Edges))
	}
}

// ===== Encode round-trips (Encode/EncodeLayered/EncodeFishbone/EncodeCausalTree) =====
//
// Each Encode is the inverse of its Parse. A round-trip both exercises
// the encoder and drives the Parse success path (valid non-empty JSON),
// which the existing empty/invalid-only Parse tests leave uncovered.

func TestEncodeWBS_RoundTrip(t *testing.T) {
	doc := WBSDocument{Root: &WBSNode{
		ID: "r", Title: "Project", Children: []*WBSNode{
			{ID: "a", Title: "Phase A", Effort: 3},
		},
	}}
	raw, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(Encode(doc)): %v", err)
	}
	if got.Root == nil || got.Root.Title != "Project" {
		t.Fatalf("round-trip lost root: %+v", got.Root)
	}
	if len(got.Root.Children) != 1 || got.Root.Children[0].ID != "a" {
		t.Errorf("round-trip lost children: %+v", got.Root.Children)
	}
}

func TestEncodeLayered_RoundTrip(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A", Label: "Start", Duration: 2}},
		Edges: []LayeredEdge{{From: "A", To: "B", Label: "FS"}},
	}
	raw, err := EncodeLayered(doc)
	if err != nil {
		t.Fatalf("EncodeLayered: %v", err)
	}
	got, err := ParseLayered(raw)
	if err != nil {
		t.Fatalf("ParseLayered(EncodeLayered(doc)): %v", err)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].ID != "A" {
		t.Errorf("round-trip lost nodes: %+v", got.Nodes)
	}
	if len(got.Edges) != 1 || got.Edges[0].Label != "FS" {
		t.Errorf("round-trip lost edges: %+v", got.Edges)
	}
}

func TestEncodeFishbone_RoundTrip(t *testing.T) {
	doc := FishboneDocument{
		Effect: "Defects",
		Categories: []FishboneCategory{
			{Name: "People", Causes: []string{"training", "fatigue"}},
		},
	}
	raw, err := EncodeFishbone(doc)
	if err != nil {
		t.Fatalf("EncodeFishbone: %v", err)
	}
	got, err := ParseFishbone(raw)
	if err != nil {
		t.Fatalf("ParseFishbone(EncodeFishbone(doc)): %v", err)
	}
	if got.Effect != "Defects" {
		t.Errorf("round-trip lost effect: %q", got.Effect)
	}
	if len(got.Categories) != 1 || len(got.Categories[0].Causes) != 2 {
		t.Errorf("round-trip lost categories: %+v", got.Categories)
	}
}

func TestEncodeCausalTree_RoundTrip(t *testing.T) {
	doc := CausalTreeDocument{
		Effect: "Outage",
		Root:   &CauseNode{ID: "r", Label: "Root", Children: []*CauseNode{{ID: "c", Label: "Cause"}}},
	}
	raw, err := EncodeCausalTree(doc)
	if err != nil {
		t.Fatalf("EncodeCausalTree: %v", err)
	}
	got, err := ParseCausalTree(raw)
	if err != nil {
		t.Fatalf("ParseCausalTree(EncodeCausalTree(doc)): %v", err)
	}
	if got.Effect != "Outage" || got.Root == nil {
		t.Fatalf("round-trip lost effect/root: %+v", got)
	}
	if len(got.Root.Children) != 1 || got.Root.Children[0].ID != "c" {
		t.Errorf("round-trip lost children: %+v", got.Root.Children)
	}
}

// ===== Kind-specific layout wrappers (network.go, pert.go, cpm.go) =====

func TestNewLayeredNode(t *testing.T) {
	n := NewLayeredNode("n1", "Task One")
	if n.ID != "n1" || n.Label != "Task One" {
		t.Errorf("NewLayeredNode: got %+v", n)
	}
}

func TestLayoutNetwork_LinearChain(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A", Label: "A"}, {ID: "B", Label: "B"}},
		Edges: []LayeredEdge{{From: "A", To: "B"}},
	}
	layout, err := LayoutNetwork(doc)
	if err != nil {
		t.Fatalf("LayoutNetwork: %v", err)
	}
	if len(layout.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(layout.Nodes))
	}
}

func TestLayoutNetwork_Cycle_ReturnsErrCycle(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A"}, {ID: "B"}},
		Edges: []LayeredEdge{{From: "A", To: "B"}, {From: "B", To: "A"}},
	}
	if _, err := LayoutNetwork(doc); !errors.Is(err, ErrCycle) {
		t.Errorf("expected ErrCycle, got %v", err)
	}
}

func TestLayoutPERT_FillsExpected(t *testing.T) {
	nodes := []LayeredNode{{ID: "A", Label: "A", Optimistic: 2, MostLikely: 4, Pessimistic: 12}}
	doc := LayeredDocument{Nodes: nodes}
	layout, err := LayoutPERT(doc)
	if err != nil {
		t.Fatalf("LayoutPERT: %v", err)
	}
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}
	// LayoutPERT mutates the node slice in place: E = (2 + 4*4 + 12)/6 = 5.
	within(t, "A.Expected", nodes[0].Expected, 5)
	if nodes[0].Duration != nodes[0].Expected {
		t.Errorf("Duration (%v) should equal Expected (%v)", nodes[0].Duration, nodes[0].Expected)
	}
}

func TestLayoutCPM_LinearChain_MarksCritical(t *testing.T) {
	nodes := []LayeredNode{
		{ID: "A", Label: "A", Duration: 3},
		{ID: "B", Label: "B", Duration: 2},
	}
	doc := LayeredDocument{
		Nodes: nodes,
		Edges: []LayeredEdge{{From: "A", To: "B"}},
	}
	layout, err := LayoutCPM(doc)
	if err != nil {
		t.Fatalf("LayoutCPM: %v", err)
	}
	if len(layout.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(layout.Nodes))
	}
	// A linear chain has zero float throughout: every node is critical.
	// LayoutCPM writes results back into the shared node slice.
	for _, n := range nodes {
		if !n.IsCritical {
			t.Errorf("node %s should be on the critical path", n.ID)
		}
	}
	within(t, "A.EF", nodes[0].EF, 3)
}

func TestLayoutCPM_Cycle_ReturnsErrCycle(t *testing.T) {
	doc := LayeredDocument{
		Nodes: []LayeredNode{{ID: "A", Duration: 1}, {ID: "B", Duration: 1}},
		Edges: []LayeredEdge{{From: "A", To: "B"}, {From: "B", To: "A"}},
	}
	if _, err := LayoutCPM(doc); !errors.Is(err, ErrCycle) {
		t.Errorf("expected ErrCycle, got %v", err)
	}
}

// TestWalk_NilNode covers the nil guard in walk (reached when a tree
// contains a nil child pointer or walk is seeded with nil).
func TestWalk_NilNode(t *testing.T) {
	count := 0
	walk(nil, func(*WBSNode) { count++ })
	if count != 0 {
		t.Errorf("walk(nil) visited %d nodes, want 0", count)
	}
}
