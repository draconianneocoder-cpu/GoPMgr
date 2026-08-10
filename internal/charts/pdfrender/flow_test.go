// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"encoding/json"
	"testing"
)

func mustFlowJSON(t *testing.T, v flowPayload) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// TestRenderFlow_EmptyGuardIsConjunctive: unlike every other engine's
// empty guard (a single len(...)==0 check), renderFlow's guard is
// len(Nodes)==0 && len(Swimlanes)==0 -- deliberately, because a
// Workflow diagram legitimately has zero swimlanes (nodes only) and
// an Activity diagram in an unusual but valid state could have
// swimlanes with no nodes yet assigned. Both non-empty branches of
// the conjunction must render normally, not fall into the
// placeholder; only true double-emptiness should.
func TestRenderFlow_EmptyGuardIsConjunctive(t *testing.T) {
	t.Run("both empty renders placeholder", func(t *testing.T) {
		pdf := newTestPDF()
		if err := renderFlow(pdf, "workflow", mustFlowJSON(t, flowPayload{}), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("renderFlow: %v", err)
		}
		out := outputBytes(t, pdf)
		if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
			t.Error("zero nodes and zero swimlanes did not render the (empty) placeholder")
		}
	})
	t.Run("nodes with no swimlanes renders normally", func(t *testing.T) {
		pdf := newTestPDF()
		layout := flowPayload{
			Width: 40, Height: 20,
			Nodes: []flowNode{{ID: "n1", Label: "Step", Shape: "start", X: 5, Y: 5, Width: 20, Height: 10}},
		}
		if err := renderFlow(pdf, "workflow", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("renderFlow: %v", err)
		}
		out := outputBytes(t, pdf)
		if bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
			t.Error("nodes-only workflow (no swimlanes) wrongly fell into the empty placeholder")
		}
		if !bytes.Contains(out, []byte("Step")) {
			t.Error("output missing node label")
		}
	})
	t.Run("swimlanes with no nodes renders normally", func(t *testing.T) {
		pdf := newTestPDF()
		layout := flowPayload{
			Width: 40, Height: 20,
			Swimlanes: []flowSwimlane{{ID: "s1", Name: "Dev Lane", X: 0, Y: 0, Width: 40, Height: 20}},
		}
		if err := renderFlow(pdf, "activity", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("renderFlow: %v", err)
		}
		out := outputBytes(t, pdf)
		if bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
			t.Error("swimlanes-only activity diagram (no nodes yet) wrongly fell into the empty placeholder")
		}
		if !bytes.Contains(out, []byte("Dev Lane")) {
			t.Error("output missing swimlane name")
		}
	})
}

// TestRenderFlow_SkipsEdgesWithMissingNodeReferences mirrors the DAG
// engine's dangling-edge guard: an edge referencing a node ID that
// doesn't exist in this diagram (e.g. a node deleted without its
// edges being cleaned up) must be skipped, not drawn against a
// zero-value node at the origin. Each surviving edge draws exactly
// three Line() calls (orthogonal routing: down, across, down), each
// emitting one " l S" stroke operator.
func TestRenderFlow_SkipsEdgesWithMissingNodeReferences(t *testing.T) {
	pdf := newTestPDF()
	layout := flowPayload{
		Width: 100, Height: 60,
		Nodes: []flowNode{
			{ID: "A", Label: "A", Shape: "start", X: 10, Y: 5, Width: 20, Height: 10},
			{ID: "B", Label: "B", Shape: "end", X: 10, Y: 40, Width: 20, Height: 10},
		},
		Edges: []flowEdge{
			{From: "A", To: "B"},       // valid
			{From: "A", To: "missing"}, // dangling "to"
			{From: "missing", To: "B"}, // dangling "from"
		},
	}
	if err := renderFlow(pdf, "workflow", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderFlow: %v", err)
	}
	out := outputBytes(t, pdf)
	gotStrokes := bytesCountLS(out)
	const wantStrokes = 3 // one valid edge x 3 Line() segments, not 9
	if gotStrokes != wantStrokes {
		t.Errorf("edge stroke ( l S) count = %d, want %d (2 dangling edges must be skipped)", gotStrokes, wantStrokes)
	}
}

func bytesCountLS(out []byte) int {
	return bytes.Count(out, []byte(" l S"))
}

// TestDrawFlowNode_ShapeDispatch verifies drawFlowNode picks a
// distinct drawing primitive per shape -- the entire point of the
// function -- using isolated single-node fixtures (no swimlanes, no
// edges) so each fixture's operator counts are attributable only to
// that one node, not polluted by other elements. Confirmed
// empirically via a throwaway probe before writing these assertions
// (same discipline as the DAG/stats increments): fpdf's Rect(...,"F")
// emits its fill operator inline ("re f", not a bare "f" line, unlike
// Circle(...,"F")); RoundedRect/Circle("FD") both emit a bezier curve
// operator (" c ") for their rounded corners, while Polygon("FD")
// (the diamond/parallelogram shapes) does not -- it's straight
// moveto/lineto only -- so curve-operator absence distinguishes a
// polygon shape from a rounded-box or circle shape.
func TestDrawFlowNode_ShapeDispatch(t *testing.T) {
	t.Run("decision renders as a diamond (Polygon), not a rounded box", func(t *testing.T) {
		pdf := newTestPDF()
		layout := flowPayload{
			Width: 40, Height: 20,
			Nodes: []flowNode{{ID: "d1", Label: "Q", Shape: "decision", X: 5, Y: 5, Width: 20, Height: 10}},
		}
		if err := renderFlow(pdf, "workflow", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 100, H: 60}); err != nil {
			t.Fatalf("renderFlow: %v", err)
		}
		out := outputBytes(t, pdf)
		if bytes.Contains(out, []byte(" c ")) {
			t.Error("decision node output contains a curve operator -- rendered as a rounded box, not a diamond polygon")
		}
	})
	t.Run("fork renders as a fill-only rect, not a stroked box", func(t *testing.T) {
		pdf := newTestPDF()
		layout := flowPayload{
			Width: 40, Height: 20,
			Nodes: []flowNode{{ID: "f1", Shape: "fork", X: 5, Y: 5, Width: 20, Height: 10}},
		}
		if err := renderFlow(pdf, "workflow", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 100, H: 60}); err != nil {
			t.Fatalf("renderFlow: %v", err)
		}
		out := outputBytes(t, pdf)
		if !bytes.Contains(out, []byte("re f\n")) {
			t.Error("fork node did not emit a fill-only rect (Rect \"F\")")
		}
	})
	t.Run("initial draws one filled+stroked circle, final draws two (bullseye)", func(t *testing.T) {
		initial := renderSingleShapeNode(t, "initial")
		final := renderSingleShapeNode(t, "final")

		if got := countBareOp(initial, "B"); got != 1 {
			t.Errorf("initial: bare B (Circle FD) count = %d, want 1", got)
		}
		if got := countBareOp(initial, "f"); got != 0 {
			t.Errorf("initial: bare f (Circle F) count = %d, want 0 -- initial should not draw an inner circle", got)
		}
		if got := countBareOp(final, "B"); got != 1 {
			t.Errorf("final: bare B (Circle FD) count = %d, want 1", got)
		}
		if got := countBareOp(final, "f"); got != 1 {
			t.Errorf("final: bare f (Circle F) count = %d, want 1 -- final's bullseye needs a second, inner circle", got)
		}
	})
	t.Run("initial and final nodes suppress their label", func(t *testing.T) {
		for _, shape := range []string{"initial", "final"} {
			out := renderSingleShapeNode(t, shape)
			if bytes.Contains(out, []byte("(Q)Tj")) {
				t.Errorf("%s node drew its label text; initial/final should suppress it", shape)
			}
		}
	})
}

func renderSingleShapeNode(t *testing.T, shape string) []byte {
	t.Helper()
	pdf := newTestPDF()
	layout := flowPayload{
		Width: 40, Height: 20,
		Nodes: []flowNode{{ID: "n1", Label: "Q", Shape: shape, X: 5, Y: 5, Width: 20, Height: 10}},
	}
	if err := renderFlow(pdf, "workflow", mustFlowJSON(t, layout), Frame{X: 10, Y: 10, W: 100, H: 60}); err != nil {
		t.Fatalf("renderFlow(%s): %v", shape, err)
	}
	return outputBytes(t, pdf)
}

func countBareOp(out []byte, op string) int {
	return bytes.Count(out, []byte("\n"+op+"\n"))
}

// --- Full pipeline: RenderChartToPDF -> charts.Layout
// (flow.ParseWorkflow/LayoutWorkflow) -> renderFlow. Representative
// for the flow family's dispatch path (EngineFlow), mirroring the
// matrix increment's single-representative-kind pattern; Workflow
// chosen over Activity because it needs no swimlane fixture data. ---

func TestRenderChartToPDF_Workflow(t *testing.T) {
	pdf := newTestPDF()
	data := `{"nodes":[{"id":"A","label":"Start","shape":"start"},{"id":"B","label":"Ship It","shape":"end"}],"edges":[{"from":"A","to":"B","label":"done"}]}`
	if err := RenderChartToPDF(pdf, "workflow", data, "Release Flow", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("RenderChartToPDF(workflow): %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Start", "Ship It"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}
