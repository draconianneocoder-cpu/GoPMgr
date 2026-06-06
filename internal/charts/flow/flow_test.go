// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package flow

import (
	"errors"
	"testing"
)

// ----- ParseWorkflow -----

func TestParseWorkflow_Empty(t *testing.T) {
	doc, err := ParseWorkflow("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Nodes) != 0 || len(doc.Edges) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseWorkflow_EmptyObject(t *testing.T) {
	doc, err := ParseWorkflow("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Nodes) != 0 || len(doc.Edges) != 0 {
		t.Error("expected empty doc from {}")
	}
}

func TestParseWorkflow_InvalidJSON(t *testing.T) {
	_, err := ParseWorkflow("{bad}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseWorkflow_ValidDocument(t *testing.T) {
	raw := `{"nodes":[{"id":"A","label":"Start","shape":"start"}],"edges":[{"from":"A","to":"B","label":"yes"}]}`
	doc, err := ParseWorkflow(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}
	if doc.Nodes[0].ID != "A" || doc.Nodes[0].Shape != "start" {
		t.Errorf("node fields: got %+v", doc.Nodes[0])
	}
	if len(doc.Edges) != 1 || doc.Edges[0].Label != "yes" {
		t.Errorf("edge label: got %+v", doc.Edges)
	}
}

// ----- ParseActivity / EncodeActivity -----

func TestParseActivity_Empty(t *testing.T) {
	doc, err := ParseActivity("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Swimlanes) != 0 || len(doc.Nodes) != 0 {
		t.Error("expected empty doc from empty string")
	}
}

func TestParseActivity_InvalidJSON(t *testing.T) {
	_, err := ParseActivity("{bad}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseActivity_ValidDocument(t *testing.T) {
	raw := `{"swimlanes":[{"id":"s1","name":"Dev"},{"id":"s2","name":"QA"}],"nodes":[{"id":"N1","label":"Code","shape":"activity","swimlane_id":"s1"}],"edges":[]}`
	doc, err := ParseActivity(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Swimlanes) != 2 {
		t.Fatalf("expected 2 swimlanes, got %d", len(doc.Swimlanes))
	}
	if len(doc.Nodes) != 1 || doc.Nodes[0].SwimlaneID != "s1" {
		t.Errorf("node swimlane_id: got %+v", doc.Nodes)
	}
}

func TestEncodeWorkflow_RoundTrip(t *testing.T) {
	original := WorkflowDocument{
		Nodes: []Node{{ID: "A", Label: "Start", Shape: ShapeStart}},
		Edges: []Edge{{From: "A", To: "B", Label: "ok"}},
	}
	encoded, err := EncodeWorkflow(original)
	if err != nil {
		t.Fatalf("EncodeWorkflow: %v", err)
	}
	decoded, err := ParseWorkflow(encoded)
	if err != nil {
		t.Fatalf("ParseWorkflow roundtrip: %v", err)
	}
	if len(decoded.Nodes) != 1 || decoded.Nodes[0].ID != "A" {
		t.Errorf("roundtrip nodes: %+v", decoded.Nodes)
	}
	if len(decoded.Edges) != 1 || decoded.Edges[0].Label != "ok" {
		t.Errorf("roundtrip edges: %+v", decoded.Edges)
	}
}

// ----- layerNodes -----

func TestLayerNodes_LinearChain_Ranks(t *testing.T) {
	nodes := []Node{{ID: "A"}, {ID: "B"}, {ID: "C"}}
	edges := []Edge{{From: "A", To: "B"}, {From: "B", To: "C"}}
	ranks, _, ok := layerNodes(nodes, edges)
	if !ok {
		t.Fatal("expected ok=true for acyclic graph")
	}
	if ranks["A"] != 0 {
		t.Errorf("A rank: got %d, want 0", ranks["A"])
	}
	if ranks["B"] != 1 {
		t.Errorf("B rank: got %d, want 1", ranks["B"])
	}
	if ranks["C"] != 2 {
		t.Errorf("C rank: got %d, want 2", ranks["C"])
	}
}

func TestLayerNodes_Diamond_Ranks(t *testing.T) {
	// A → B, A → C, B → D, C → D: D should have rank 2
	nodes := []Node{{ID: "A"}, {ID: "B"}, {ID: "C"}, {ID: "D"}}
	edges := []Edge{
		{From: "A", To: "B"}, {From: "A", To: "C"},
		{From: "B", To: "D"}, {From: "C", To: "D"},
	}
	ranks, _, ok := layerNodes(nodes, edges)
	if !ok {
		t.Fatal("expected ok=true for diamond DAG")
	}
	if ranks["A"] != 0 {
		t.Errorf("A rank: got %d, want 0", ranks["A"])
	}
	if ranks["B"] != 1 || ranks["C"] != 1 {
		t.Errorf("B/C ranks: got %d/%d, want 1/1", ranks["B"], ranks["C"])
	}
	if ranks["D"] != 2 {
		t.Errorf("D rank: got %d, want 2", ranks["D"])
	}
}

func TestLayerNodes_Cycle_ReturnsFalse(t *testing.T) {
	nodes := []Node{{ID: "A"}, {ID: "B"}}
	edges := []Edge{{From: "A", To: "B"}, {From: "B", To: "A"}}
	_, _, ok := layerNodes(nodes, edges)
	if ok {
		t.Error("expected ok=false for cyclic graph")
	}
}

func TestLayerNodes_AlphabeticalLayerOrder(t *testing.T) {
	// Three sources A, B, C at rank 0 — sorted alphabetically in layer.
	nodes := []Node{{ID: "C"}, {ID: "A"}, {ID: "B"}}
	ranks, layers, ok := layerNodes(nodes, nil)
	if !ok {
		t.Fatal("expected ok=true")
	}
	_ = ranks
	if len(layers) == 0 || len(layers[0]) != 3 {
		t.Fatalf("expected 3 nodes in layer 0, got %v", layers)
	}
	if layers[0][0] != "A" || layers[0][1] != "B" || layers[0][2] != "C" {
		t.Errorf("alphabetical order violated in layer 0: %v", layers[0])
	}
}

// ----- resolveWorkflowShape -----

func TestResolveWorkflowShape_KnownShapes(t *testing.T) {
	known := []string{ShapeStart, ShapeEnd, ShapeAction, ShapeDecision, ShapeIO, ShapeSubprocess}
	for _, s := range known {
		if got := resolveWorkflowShape(s); got != s {
			t.Errorf("resolveWorkflowShape(%q) = %q, want %q", s, got, s)
		}
	}
}

func TestResolveWorkflowShape_UnknownDefaultsToAction(t *testing.T) {
	if got := resolveWorkflowShape("xyz"); got != ShapeAction {
		t.Errorf("expected %q, got %q", ShapeAction, got)
	}
}

// ----- resolveActivityShape -----

func TestResolveActivityShape_KnownShapes(t *testing.T) {
	known := []string{ShapeInitial, ShapeFinal, ShapeActivity, ShapeADecision, ShapeFork, ShapeJoin}
	for _, s := range known {
		if got := resolveActivityShape(s); got != s {
			t.Errorf("resolveActivityShape(%q) = %q, want %q", s, got, s)
		}
	}
}

func TestResolveActivityShape_UnknownDefaultsToActivity(t *testing.T) {
	if got := resolveActivityShape("xyz"); got != ShapeActivity {
		t.Errorf("expected %q, got %q", ShapeActivity, got)
	}
}

// ----- activityNodeSize -----

func TestActivityNodeSize_InitialAndFinal(t *testing.T) {
	opt := DefaultOptions()
	for _, shape := range []string{ShapeInitial, ShapeFinal} {
		w, h := activityNodeSize(shape, opt)
		if w != 28 || h != 28 {
			t.Errorf("shape %q: got (%v,%v), want (28,28)", shape, w, h)
		}
	}
}

func TestActivityNodeSize_ForkAndJoin(t *testing.T) {
	opt := DefaultOptions()
	for _, shape := range []string{ShapeFork, ShapeJoin} {
		w, h := activityNodeSize(shape, opt)
		wantW := opt.SwimlaneWidth - 40
		if w != wantW || h != 8 {
			t.Errorf("shape %q: got (%v,%v), want (%v,8)", shape, w, h, wantW)
		}
	}
}

func TestActivityNodeSize_DefaultActivity(t *testing.T) {
	opt := DefaultOptions()
	w, h := activityNodeSize(ShapeActivity, opt)
	if w != opt.NodeWidth-20 || h != opt.NodeHeight {
		t.Errorf("activity shape: got (%v,%v), want (%v,%v)",
			w, h, opt.NodeWidth-20, opt.NodeHeight)
	}
}

// ----- hasDefaultLane -----

func TestHasDefaultLane_AllAssigned_False(t *testing.T) {
	doc := ActivityDocument{
		Swimlanes: []Swimlane{{ID: "s1"}},
		Nodes:     []Node{{ID: "N", SwimlaneID: "s1"}},
	}
	if hasDefaultLane(doc) {
		t.Error("expected false when all nodes have a valid swimlane ID")
	}
}

func TestHasDefaultLane_EmptySwimlaneID_True(t *testing.T) {
	doc := ActivityDocument{
		Swimlanes: []Swimlane{{ID: "s1"}},
		Nodes:     []Node{{ID: "N", SwimlaneID: ""}},
	}
	if !hasDefaultLane(doc) {
		t.Error("expected true when a node has empty SwimlaneID")
	}
}

func TestHasDefaultLane_UnknownSwimlaneID_True(t *testing.T) {
	doc := ActivityDocument{
		Swimlanes: []Swimlane{{ID: "s1"}},
		Nodes:     []Node{{ID: "N", SwimlaneID: "unknown"}},
	}
	if !hasDefaultLane(doc) {
		t.Error("expected true when a node references an undeclared swimlane")
	}
}

// ----- LayoutWorkflow -----

func TestLayoutWorkflow_EmptyNodes(t *testing.T) {
	layout, err := LayoutWorkflow(WorkflowDocument{}, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(layout.Nodes))
	}
}

func TestLayoutWorkflow_SingleNode_Geometry(t *testing.T) {
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "A", Label: "Start", Shape: ShapeAction}},
	}
	opt := DefaultOptions()
	layout, err := LayoutWorkflow(doc, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}
	n := layout.Nodes[0]
	if n.X != 0 || n.Y != 0 {
		t.Errorf("single node position: got (%v,%v), want (0,0)", n.X, n.Y)
	}
	if n.Width != opt.NodeWidth {
		t.Errorf("Width: got %v, want %v", n.Width, opt.NodeWidth)
	}
	if n.Height != opt.NodeHeight {
		t.Errorf("Height: got %v, want %v", n.Height, opt.NodeHeight)
	}
}

func TestLayoutWorkflow_DecisionNode_TallerThanAction(t *testing.T) {
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "D", Shape: ShapeDecision}},
	}
	opt := DefaultOptions()
	layout, err := LayoutWorkflow(doc, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if layout.Nodes[0].Height <= opt.NodeHeight {
		t.Errorf("decision height %v should exceed action height %v",
			layout.Nodes[0].Height, opt.NodeHeight)
	}
}

func TestLayoutWorkflow_LinearChain_Ranks(t *testing.T) {
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "A", Shape: ShapeStart}, {ID: "B", Shape: ShapeEnd}},
		Edges: []Edge{{From: "A", To: "B"}},
	}
	opt := DefaultOptions()
	layout, err := LayoutWorkflow(doc, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := make(map[string]NodeLayout)
	for _, n := range layout.Nodes {
		byID[n.ID] = n
	}
	if byID["A"].Rank != 0 {
		t.Errorf("A rank: got %d, want 0", byID["A"].Rank)
	}
	if byID["B"].Rank != 1 {
		t.Errorf("B rank: got %d, want 1", byID["B"].Rank)
	}
	// B is one rowStride below A.
	rowStride := opt.NodeHeight + opt.RowGap
	if byID["B"].Y != float64(rowStride) {
		t.Errorf("B.Y: got %v, want %v", byID["B"].Y, float64(rowStride))
	}
}

func TestLayoutWorkflow_Cycle_ReturnsErrCycle(t *testing.T) {
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "A"}, {ID: "B"}},
		Edges: []Edge{{From: "A", To: "B"}, {From: "B", To: "A"}},
	}
	_, err := LayoutWorkflow(doc, DefaultOptions())
	if !errors.Is(err, ErrCycle) {
		t.Errorf("expected ErrCycle, got %v", err)
	}
}

func TestLayoutWorkflow_AllXNonNegative(t *testing.T) {
	// Three parallel source nodes — left node would be at negative X before shift.
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
	}
	layout, err := LayoutWorkflow(doc, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range layout.Nodes {
		if n.X < 0 {
			t.Errorf("node %q has negative X=%v", n.ID, n.X)
		}
	}
}

func TestLayoutWorkflow_EdgePassthrough(t *testing.T) {
	doc := WorkflowDocument{
		Nodes: []Node{{ID: "A"}, {ID: "B"}},
		Edges: []Edge{{From: "A", To: "B", Label: "approved"}},
	}
	layout, err := LayoutWorkflow(doc, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(layout.Edges))
	}
	e := layout.Edges[0]
	if e.From != "A" || e.To != "B" || e.Label != "approved" {
		t.Errorf("edge: got %+v", e)
	}
}

// ----- LayoutActivity -----

func TestLayoutActivity_EmptyNodes_ReturnsSwimlanes(t *testing.T) {
	doc := ActivityDocument{
		Swimlanes: []Swimlane{{ID: "s1", Name: "Dev"}, {ID: "s2", Name: "QA"}},
	}
	opt := DefaultOptions()
	layout, err := LayoutActivity(doc, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layout.Nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(layout.Nodes))
	}
	if len(layout.Swimlanes) != 2 {
		t.Errorf("expected 2 swimlanes, got %d", len(layout.Swimlanes))
	}
	if layout.Swimlanes[0].X != 0 {
		t.Errorf("first swimlane X: got %v, want 0", layout.Swimlanes[0].X)
	}
	if layout.Swimlanes[1].X != opt.SwimlaneWidth {
		t.Errorf("second swimlane X: got %v, want %v",
			layout.Swimlanes[1].X, opt.SwimlaneWidth)
	}
}

func TestLayoutActivity_Cycle_ReturnsErrCycleActivity(t *testing.T) {
	doc := ActivityDocument{
		Nodes: []Node{{ID: "X"}, {ID: "Y"}},
		Edges: []Edge{{From: "X", To: "Y"}, {From: "Y", To: "X"}},
	}
	_, err := LayoutActivity(doc, DefaultOptions())
	if !errors.Is(err, ErrCycleActivity) {
		t.Errorf("expected ErrCycleActivity, got %v", err)
	}
}

func TestLayoutActivity_UnassignedNode_AddsDefaultLane(t *testing.T) {
	doc := ActivityDocument{
		Swimlanes: []Swimlane{{ID: "s1", Name: "Dev"}},
		Nodes:     []Node{{ID: "N1", SwimlaneID: ""}},
	}
	layout, err := LayoutActivity(doc, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The layout should have two swimlanes: s1 + "(unassigned)".
	if len(layout.Swimlanes) != 2 {
		t.Errorf("expected 2 swimlanes (declared + default), got %d", len(layout.Swimlanes))
	}
	found := false
	for _, s := range layout.Swimlanes {
		if s.ID == "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unassigned (ID='') swimlane in output")
	}
}
