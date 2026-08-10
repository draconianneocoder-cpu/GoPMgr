// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderFishbone_EmptyRendersPlaceholderNotPanic(t *testing.T) {
	pdf := newTestPDF()
	layout := fishbonePayload{}
	b, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := renderFishbone(pdf, b, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderFishbone(empty): %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
		t.Error("zero nodes did not render the (empty) placeholder")
	}
}

// TestRenderFishbone_DrawsEveryNode: the effect node is drawn as a
// RoundedRect("FD"), which -- same technique established for the DAG
// engine -- emits a bare "B" fill+stroke operator on its own
// content-stream line, letting node-box presence be checked
// independently of label text (a dropped effect box with its label
// text happening to still render elsewhere would not be caught by a
// text-only assertion).
func TestRenderFishbone_DrawsEveryNode(t *testing.T) {
	pdf := newTestPDF()
	layout := fishbonePayload{
		Width: 100, Height: 60,
		Nodes: []fishboneNode{
			{ID: "e", Type: "effect", Label: "Late Delivery", X: 80, Y: 25, Width: 20, Height: 10},
			{ID: "c1", Type: "category", Label: "Process", X: 10, Y: 5, Width: 15, Height: 5},
			{ID: "cause1", Type: "cause", Label: "Slow Handoff", X: 10, Y: 12, Width: 15, Height: 3},
		},
		Edges: []fishboneEdge{
			{X1: 0, Y1: 30, X2: 100, Y2: 30, Kind: "spine"},
		},
	}
	b, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := renderFishbone(pdf, b, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderFishbone: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Late Delivery", "Process", "Slow Handoff"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
	gotBoxes := strings.Count(string(out), "\nB\n")
	const wantBoxes = 1 // only the effect node draws a RoundedRect box
	if gotBoxes != wantBoxes {
		t.Errorf("effect node box (RoundedRect fill+stroke) count = %d, want %d", gotBoxes, wantBoxes)
	}
}

// --- Full pipeline: RenderChartToPDF -> charts.Layout
// (dag.ParseFishbone/LayoutFishbone) -> renderFishbone. Fishbone is
// dispatched through a special case inside the EngineDAG branch of
// dispatcher.go's RenderChartToPDF switch (kind == KindFishbone),
// not through EngineMatrix/EngineFlow/EngineStats like every other
// engine -- this test is what actually protects that dispatch fork
// from silently breaking if the EngineDAG branch is ever reordered
// or the fishbone special-case is dropped. ---

func TestRenderChartToPDF_Fishbone(t *testing.T) {
	pdf := newTestPDF()
	data := `{"effect":"Defect Rate","categories":[{"name":"People","causes":["Untrained","Overworked"]}]}`
	if err := RenderChartToPDF(pdf, "fishbone", data, "Root Cause", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("RenderChartToPDF(fishbone): %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Defect Rate", "People", "Untrained", "Overworked"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}
