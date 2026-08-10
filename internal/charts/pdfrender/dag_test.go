// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

// newTestPDF returns a page with compression off, so the content
// stream is greppable for assertions on what was actually drawn --
// not just that no error occurred.
func newTestPDF() *fpdf.Fpdf {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.AddPage()
	return pdf
}

func outputBytes(t *testing.T, pdf *fpdf.Fpdf) []byte {
	t.Helper()
	if pdf.Err() {
		t.Fatalf("fpdf error state: %v", pdf.Error())
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output: %v", err)
	}
	return out.Bytes()
}

// pdfEscape mirrors fpdf's PDF string-literal escaping: '(', ')', and
// '\' are backslash-escaped inside a PDF string, so text drawn via
// CellFormat containing those characters does not appear byte-for-byte
// in the content stream. Plain alphanumeric labels (the node titles
// used elsewhere in this file) don't need this, but the "(empty)"
// placeholder does.
func pdfEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}

// --- unwrapLayout: pure, no fpdf involved ---

func TestUnwrapLayout(t *testing.T) {
	t.Run("wrapped form with nodes returns the wrapped layout", func(t *testing.T) {
		body := json.RawMessage(`{
			"layout": {"nodes":[{"id":"A","title":"Alpha"}],"edges":[],"width":100,"height":50},
			"doc": {"unrelated":"ignored"}
		}`)
		got, err := unwrapLayout(body)
		if err != nil {
			t.Fatalf("unwrapLayout: %v", err)
		}
		if len(got.Nodes) != 1 || got.Nodes[0].Title != "Alpha" {
			t.Fatalf("Nodes = %+v, want one node titled Alpha", got.Nodes)
		}
		if got.Width != 100 || got.Height != 50 {
			t.Fatalf("Width/Height = %v/%v, want 100/50", got.Width, got.Height)
		}
	})

	t.Run("bare form with nodes returns the plain layout", func(t *testing.T) {
		body := json.RawMessage(`{"nodes":[{"id":"A","title":"Alpha"},{"id":"B","title":"Beta"}],"edges":[],"width":200,"height":80}`)
		got, err := unwrapLayout(body)
		if err != nil {
			t.Fatalf("unwrapLayout: %v", err)
		}
		if len(got.Nodes) != 2 {
			t.Fatalf("Nodes = %+v, want 2", got.Nodes)
		}
		if got.Width != 200 || got.Height != 80 {
			t.Fatalf("Width/Height = %v/%v, want 200/80", got.Width, got.Height)
		}
	})

	// A wrapped body whose layout legitimately has zero nodes (e.g. a
	// Network/PERT/CPM chart with no tasks yet) fails the wrapped
	// branch's `len(...) > 0` guard and falls through to the bare
	// unmarshal of the SAME raw body. Since the bare shape has no
	// top-level "nodes"/"width"/"height" keys in a wrapped body, that
	// produces a zero-value layoutPayload, not the (also-empty) nested
	// layout's Width/Height. This is a real, currently-benign contract
	// quirk: renderDAG only checks len(Nodes)==0 today, so the result is
	// the same placeholder either way -- but any future caller reading
	// Width/Height for an empty wrapped chart would silently get 0
	// instead of the nested layout's real (possibly nonzero) dimensions.
	// Pinned here so a change to that fallback is a deliberate decision,
	// not a silent regression.
	t.Run("wrapped form with zero nodes falls through to a zero-value bare layout", func(t *testing.T) {
		body := json.RawMessage(`{"layout":{"nodes":[],"edges":[],"width":300,"height":150},"doc":{}}`)
		got, err := unwrapLayout(body)
		if err != nil {
			t.Fatalf("unwrapLayout: %v", err)
		}
		if len(got.Nodes) != 0 {
			t.Fatalf("Nodes = %+v, want empty", got.Nodes)
		}
		if got.Width != 0 || got.Height != 0 {
			t.Fatalf("Width/Height = %v/%v, want 0/0 (the nested layout's 300/150 must NOT leak through the fallback)", got.Width, got.Height)
		}
	})

	t.Run("malformed JSON returns an error", func(t *testing.T) {
		if _, err := unwrapLayout(json.RawMessage(`{not valid json`)); err == nil {
			t.Fatal("unwrapLayout(malformed) returned nil error, want error")
		}
	})
}

// --- renderDAG: exercised directly (unexported, same package) so edge
// cases don't depend on fighting the upstream charts.Layout validators. ---

// TestRenderDAG_DrawsEveryNode checks two independent things a node can
// fail at: the title text surviving truncatePDF, and the node's box
// actually being drawn. Title presence alone is coupled to
// truncatePDF's threshold (today ~32 chars for a 20mm-wide, unscaled
// box) and would pass even if a node's box were skipped while some
// other node happened to render text containing the same substring, or
// fail for reasons unrelated to node-dropping if the truncation
// threshold changes. drawDAGNode calls pdf.RoundedRect(..., "FD") --
// fpdf's fill-and-stroke style -- exactly once per node, emitting one
// bare "B" operator line; counting "\nB\n" occurrences is a precise,
// content-shape-independent proxy for "how many node boxes got drawn".
func TestRenderDAG_DrawsEveryNode(t *testing.T) {
	pdf := newTestPDF()
	body := json.RawMessage(`{
		"nodes": [
			{"id":"A","title":"Alpha","x":0,"y":0,"width":20,"height":10},
			{"id":"B","title":"Beta","x":40,"y":0,"width":20,"height":10},
			{"id":"C","title":"Gamma","x":80,"y":0,"width":20,"height":10}
		],
		"edges": [{"from":"A","to":"B"},{"from":"B","to":"C"}],
		"width": 120, "height": 30
	}`)
	if err := renderDAG(pdf, "network", body, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderDAG: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Alpha", "Beta", "Gamma"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing node title %q -- a node was silently dropped", want)
		}
	}
	gotBoxes := strings.Count(string(out), "\nB\n")
	const wantBoxes = 3
	if gotBoxes != wantBoxes {
		t.Errorf("node-box (RoundedRect fill+stroke) count = %d, want %d", gotBoxes, wantBoxes)
	}
}

func TestRenderDAG_EmptyRendersPlaceholderNotPanic(t *testing.T) {
	pdf := newTestPDF()
	body := json.RawMessage(`{"nodes":[],"edges":[],"width":0,"height":0}`)
	if err := renderDAG(pdf, "network", body, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderDAG(empty): %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
		t.Error("empty layout did not render the placeholder label")
	}
}

// TestRenderDAG_SkipsEdgesWithMissingNodeReferences is R3: an edge whose
// from/to ID isn't in the node set must be skipped, not drawn against a
// zero-value node (which would silently draw a stray connector to the
// frame's corner rather than failing loudly). renderDAG draws each valid
// edge as three fpdf.Line calls (vertical, horizontal, vertical), and
// each Line() call emits exactly one " l S" stroke operator -- so
// counting them is a precise proxy for how many edges were actually
// drawn, not just whether the call errored.
func TestRenderDAG_SkipsEdgesWithMissingNodeReferences(t *testing.T) {
	pdf := newTestPDF()
	body := json.RawMessage(`{
		"nodes": [
			{"id":"A","title":"Alpha","x":0,"y":0,"width":20,"height":10},
			{"id":"B","title":"Beta","x":40,"y":0,"width":20,"height":10}
		],
		"edges": [
			{"from":"A","to":"B"},
			{"from":"A","to":"ghost-node-that-does-not-exist"}
		],
		"width": 60, "height": 30
	}`)
	if err := renderDAG(pdf, "network", body, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderDAG: %v", err)
	}
	out := outputBytes(t, pdf)
	gotLines := strings.Count(string(out), " l S")
	const wantLines = 3 // one valid edge x 3 Line() calls each
	if gotLines != wantLines {
		t.Errorf("stroke operator count = %d, want %d (the dangling edge must be skipped, not drawn)", gotLines, wantLines)
	}
}

// TestRenderDAG_BareAndWrappedFormsAgree exercises the full
// RenderChartToPDF -> charts.Layout -> renderDAG pipeline for one bare
// kind (wbs) and one wrapped kind (network), confirming both real
// chart-kind shapes reach the renderer and produce a node's title in
// the output -- not just the hand-built fixtures above.
func TestRenderDAG_BareAndWrappedFormsAgree(t *testing.T) {
	t.Run("wbs (bare form)", func(t *testing.T) {
		pdf := newTestPDF()
		data := `{"root":{"id":"1","title":"Project Root","children":[{"id":"2","title":"Design Phase"}]}}`
		if err := RenderChartToPDF(pdf, "wbs", data, "WBS", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("RenderChartToPDF(wbs): %v", err)
		}
		out := outputBytes(t, pdf)
		if !bytes.Contains(out, []byte("Project Root")) {
			t.Error("output missing WBS root title")
		}
	})

	t.Run("network (wrapped form)", func(t *testing.T) {
		pdf := newTestPDF()
		data := `{"nodes":[{"id":"A","label":"Kickoff","duration":2}],"edges":[]}`
		if err := RenderChartToPDF(pdf, "network", data, "Network", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("RenderChartToPDF(network): %v", err)
		}
		out := outputBytes(t, pdf)
		if !bytes.Contains(out, []byte("Kickoff")) {
			t.Error("output missing network node label")
		}
	})

	t.Run("network with zero tasks reaches the wrapped-form empty path", func(t *testing.T) {
		pdf := newTestPDF()
		if err := RenderChartToPDF(pdf, "network", `{"nodes":[],"edges":[]}`, "Network", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
			t.Fatalf("RenderChartToPDF(network, empty): %v", err)
		}
		out := outputBytes(t, pdf)
		if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
			t.Error("empty network chart did not render the placeholder")
		}
	})
}
