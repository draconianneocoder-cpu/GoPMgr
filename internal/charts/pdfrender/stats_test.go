// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"strings"
	"testing"
)

// --- Pie ---

func TestRenderPie_EmptySlices(t *testing.T) {
	pdf := newTestPDF()
	if err := renderPie(pdf, statsLayout{Kind: "pie"}, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderPie(empty): %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
		t.Error("zero slices did not render the (empty) placeholder")
	}
}

// TestRenderPie_NonPositiveTotal is the pie-engine equivalent of the
// DAG engine's empty-layout guard: renderPie divides each slice's
// share by the sum of all slice values, so a slice set whose values
// sum to <= 0 (all zero, or a mix that cancels out) must be caught by
// the explicit `total <= 0` guard before that division, not silently
// divide by zero.
func TestRenderPie_NonPositiveTotal(t *testing.T) {
	cases := []struct {
		name   string
		slices []pieSlice
	}{
		{"all zero", []pieSlice{{Label: "A", Value: 0}, {Label: "B", Value: 0}}},
		{"negative sum", []pieSlice{{Label: "A", Value: -5}, {Label: "B", Value: 3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newTestPDF()
			l := statsLayout{Kind: "pie", Slices: tc.slices}
			if err := renderPie(pdf, l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
				t.Fatalf("renderPie: %v", err)
			}
			out := outputBytes(t, pdf)
			if !bytes.Contains(out, []byte(pdfEscape("(no values)"))) {
				t.Error("non-positive total did not render the (no values) placeholder")
			}
			if strings.Contains(string(out), "NaN") {
				t.Error("output contains \"NaN\" -- a wedge angle was computed as 0/0")
			}
		})
	}
}

// TestRenderPie_DrawsEverySlice: each wedge is drawn via
// pdf.Polygon(pts, "FD"), fpdf's fill-and-stroke style, which emits
// exactly one bare "B" operator line per wedge (confirmed empirically,
// same technique as the DAG engine's node-box count) -- distinguishing
// "3 wedges drawn" from "3 legend labels drawn but a wedge silently
// skipped", which title-text presence alone cannot.
func TestRenderPie_DrawsEverySlice(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{
		Kind: "pie",
		Slices: []pieSlice{
			{Label: "Alpha", Value: 10, Pct: 40},
			{Label: "Beta", Value: 8, Pct: 32},
			{Label: "Gamma", Value: 7, Pct: 28},
		},
	}
	if err := renderPie(pdf, l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderPie: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Alpha", "Beta", "Gamma"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing slice legend label %q", want)
		}
	}
	gotWedges := strings.Count(string(out), "\nB\n")
	const wantWedges = 3
	if gotWedges != wantWedges {
		t.Errorf("wedge (Polygon fill+stroke) count = %d, want %d", gotWedges, wantWedges)
	}
}

// --- Cartesian ---

func TestRenderCartesian_EmptyInput(t *testing.T) {
	cases := []struct {
		name string
		l    statsLayout
	}{
		{"no series", statsLayout{Kind: "line", Categories: []string{"W1"}}},
		{"no categories", statsLayout{Kind: "line", Series: []statsSeries{{Name: "S", Values: []float64{1}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newTestPDF()
			if err := renderCartesian(pdf, tc.l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
				t.Fatalf("renderCartesian: %v", err)
			}
			out := outputBytes(t, pdf)
			if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
				t.Error("empty input did not render the (empty) placeholder")
			}
		})
	}
}

// TestRenderCartesian_DegenerateYRange is the highest-priority risk
// this increment targets: scanYBounds anchors the low bound at 0 for
// all-positive data, so an all-zero series produces yMin=yMax=0 before
// the `if yMax <= yMin { yMax = yMin + 1 }` guard runs. Without that
// guard, every tick/gridline/point Y-coordinate divides by
// (yMax-yMin) == 0, producing 0/0 = NaN or a non-zero/0 = +-Inf
// coordinate. Go's float formatting (strconv.FormatFloat, which fpdf
// uses for every coordinate) renders those literally as "NaN"/"+Inf"/
// "-Inf" in the PDF content stream -- confirmed by direct read of
// fpdf's fmtF64 and a throwaway probe -- so their absence is a
// precise, unambiguous proxy for "the guard fired", stronger than
// checking gnum's tick-label text (which has its own NaN-safe
// fallback and would mask a coordinate-level NaN).
func TestRenderCartesian_DegenerateYRange(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{
		Kind:       "line",
		Categories: []string{"W1", "W2", "W3"},
		Series:     []statsSeries{{Name: "Flat", Values: []float64{0, 0, 0}, Type: "line"}},
	}
	if err := renderCartesian(pdf, l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderCartesian: %v", err)
	}
	assertNoNaNOrInf(t, outputBytes(t, pdf))
}

// TestRenderCartesian_ExplicitEqualAxisOverride covers the other path
// to the same degenerate range: an explicit y_axis.min == y_axis.max
// in the chart's config (e.g. malformed or hand-edited persisted data)
// overrides scanYBounds' computed range BEFORE the yMax<=yMin guard
// runs -- confirming the guard's placement (after the override, not
// before it) actually protects this path too, not just the
// all-zero-data path above.
func TestRenderCartesian_ExplicitEqualAxisOverride(t *testing.T) {
	pdf := newTestPDF()
	fixed := 5.0
	l := statsLayout{
		Kind:       "line",
		Categories: []string{"W1", "W2", "W3"},
		YAxis:      axisConfig{Min: &fixed, Max: &fixed},
		Series:     []statsSeries{{Name: "S", Values: []float64{1, 2, 3}, Type: "line"}},
	}
	if err := renderCartesian(pdf, l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderCartesian: %v", err)
	}
	assertNoNaNOrInf(t, outputBytes(t, pdf))
}

func assertNoNaNOrInf(t *testing.T, out []byte) {
	t.Helper()
	s := string(out)
	for _, bad := range []string{"NaN", "+Inf", "-Inf"} {
		if strings.Contains(s, bad) {
			t.Errorf("output contains %q -- a coordinate divided by a zero-width Y range", bad)
		}
	}
}

// TestRenderCartesian_SeriesLongerThanCategories: drawLineSeries caps
// the number of points drawn to min(len(Values), len(Categories)) so a
// series with more data points than the chart currently has categories
// (e.g. categories trimmed without re-simulating series data) doesn't
// draw points past the plot's right edge. Each point is a
// pdf.Circle(..., "F") call, which emits one bare "f" operator line --
// counting them distinguishes "capped correctly" from "drew all 5
// points anyway", which a bare no-panic check cannot. The fixture below
// has no "bar" series: drawBars also emits "F"-style fills (via
// pdf.Rect), which would inflate this count -- if a bar series is ever
// added to this fixture, the expected count must account for it too.
func TestRenderCartesian_SeriesLongerThanCategories(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{
		Kind:       "line",
		Categories: []string{"W1", "W2", "W3"},                                                 // 3 categories
		Series:     []statsSeries{{Name: "S", Values: []float64{1, 2, 3, 4, 5}, Type: "line"}}, // 5 values
	}
	if err := renderCartesian(pdf, l, Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderCartesian: %v", err)
	}
	out := outputBytes(t, pdf)
	gotPoints := strings.Count(string(out), "\nf\n")
	const wantPoints = 3 // capped to len(Categories), not len(Values)
	if gotPoints != wantPoints {
		t.Errorf("point-marker count = %d, want %d (capped to category count)", gotPoints, wantPoints)
	}
}

// --- hexRGB / fmtSscanfHex: Color strings are persisted chart data;
// malformed values (truncated, non-hex, missing '#') must fall back to
// the caller-supplied default rather than panic on a short slice. ---

func TestHexRGB(t *testing.T) {
	const fbR, fbG, fbB = 1, 2, 3
	cases := []struct {
		name                string
		hex                 string
		wantR, wantG, wantB int
	}{
		{"empty falls back", "", fbR, fbG, fbB},
		{"valid with hash", "#ff0080", 255, 0, 128},
		{"valid without hash", "00ff00", 0, 255, 0},
		{"hash only falls back", "#", fbR, fbG, fbB},
		{"too short falls back", "#fff", fbR, fbG, fbB},
		{"too long falls back", "#ff00801234", fbR, fbG, fbB},
		{"non-hex chars fall back", "#zzzzzz", fbR, fbG, fbB},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b := hexRGB(tc.hex, fbR, fbG, fbB)
			if r != tc.wantR || g != tc.wantG || b != tc.wantB {
				t.Errorf("hexRGB(%q) = (%d,%d,%d), want (%d,%d,%d)", tc.hex, r, g, b, tc.wantR, tc.wantG, tc.wantB)
			}
		})
	}
}

// --- Full pipeline: RenderChartToPDF -> charts.Layout -> renderStats.
// The tests above call renderPie/renderCartesian directly to control
// edge cases precisely; these two confirm renderStats's own kind
// dispatch (pie vs. everything else) is reached by a real chart kind's
// wire format, and additionally exercise drawBars (the "bar" kind),
// which shares the same yMax/yMin division as the line-series path
// above but was otherwise unexercised. ---

func TestRenderChartToPDF_Pie(t *testing.T) {
	pdf := newTestPDF()
	data := `{"title":"Defects","slices":[{"label":"Critical","value":5},{"label":"Minor","value":15}]}`
	if err := RenderChartToPDF(pdf, "pie", data, "Defects", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("RenderChartToPDF(pie): %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte("Critical")) || !bytes.Contains(out, []byte("Minor")) {
		t.Error("output missing pie slice labels")
	}
}

func TestRenderChartToPDF_Bar(t *testing.T) {
	pdf := newTestPDF()
	data := `{"categories":["Q1","Q2"],"series":[{"name":"Revenue","values":[100,200]}]}`
	if err := RenderChartToPDF(pdf, "bar", data, "Revenue", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("RenderChartToPDF(bar): %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte("Revenue")) {
		t.Error("output missing bar series legend name")
	}
	assertNoNaNOrInf(t, out)
}
