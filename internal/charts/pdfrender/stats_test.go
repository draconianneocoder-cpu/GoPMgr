// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
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

// paletteRGB picks a series' print colour. The expected values are the
// light-theme steps of the app's validated series palette (blue, orange,
// aqua, yellow, magenta, green, violet, red); regressions it guards: the
// former dark-only defaults printed as given (cyan #22d3ee is 1.7:1 on
// white), case-sensitive matching letting "#22D3EE" through, a slot past
// the eighth series panicking or going blank, and a user's own colour
// being replaced.
func TestPaletteRGB(t *testing.T) {
	cases := []struct {
		name    string
		idx     int
		hex     string
		r, g, b int
	}{
		{"unset first slot", 0, "", 42, 120, 214},
		{"unset last slot", 7, "", 227, 73, 72},
		{"unset ninth series repeats the first", 8, "", 42, 120, 214},
		{"former cyan default prints as blue", 3, "#22d3ee", 42, 120, 214},
		{"former default matched in any case", 0, "#F59E0B", 235, 104, 52},
		{"former red default", 0, "#ef4444", 227, 73, 72},
		{"former slate is the neutral reference colour", 0, "#94a3b8", 100, 116, 139},
		{"a user's own colour is kept", 5, "#123456", 0x12, 0x34, 0x56},
		{"malformed colour falls back to the first slot", 5, "#zz", 42, 120, 214},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b := paletteRGB(tc.idx, tc.hex)
			if r != tc.r || g != tc.g || b != tc.b {
				t.Errorf("paletteRGB(%d, %q) = (%d,%d,%d), want (%d,%d,%d)", tc.idx, tc.hex, r, g, b, tc.r, tc.g, tc.b)
			}
		})
	}
}

// truncatePDF shortens labels to fit a box. It cut by bytes, so a label
// with accented letters could be split inside a character and print as
// U+FFFD (pdfdoc.SafeText replaces invalid UTF-8).
func TestTruncatePDF(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short label unchanged", "Design", 10, "Design"},
		{"exactly at the limit unchanged", "Überprüfen", 10, "Überprüfen"},
		{"cut between accented letters, not inside one", "Überprüfung der Lieferkette", 8, "Überprü…"},
		{"limit below four is raised to four", "Kostenplanung", 2, "Kos…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncatePDF(tc.in, tc.max)
			if got != tc.want {
				t.Errorf("truncatePDF(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncatePDF(%q, %d) = %q is not valid UTF-8", tc.in, tc.max, got)
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

// --- Deferred helpers: drawStackedAreas, drawRightAxisLine,
// drawRightAxisTicks, drawDashedLine (increment 2's memory note named
// these as deliberately deferred -- lower-frequency chart features
// protected by the same yMax<=yMin guard already proven above, but
// unverified as call sites in their own right). Tested directly
// (white-box, in-package) rather than through the full renderCartesian
// pipeline, since driving each one through specific statsLayout wiring
// while isolating its own content-stream signature from gridlines/axis
// labels/other series is simpler and more precise than differencing a
// full-chart render. ---

// TestDrawStackedAreas_FillsOnePolygonPerSeries: each series in a
// stacked area chart is drawn as its own semi-transparent filled
// polygon (SetAlpha(0.55, ...) around the fill, confirmed via probe to
// be the only SetAlpha call site in this package, so counting the
// "/GS1 gs" ExtGState reference it emits is a precise, unambiguous
// signal distinct from any other drawing operation). Two series must
// produce two polygons, not one merged shape or a silently-dropped
// second series.
func TestDrawStackedAreas_FillsOnePolygonPerSeries(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{
		Categories: []string{"W1", "W2", "W3"},
		Series: []statsSeries{
			{Name: "Done", Values: []float64{1, 2, 3}},
			{Name: "Doing", Values: []float64{2, 1, 1}},
		},
	}
	drawStackedAreas(pdf, l, Frame{X: 10, Y: 10, W: 100, H: 60}, 0, 6)
	out := outputBytes(t, pdf)
	if got := bytes.Count(out, []byte("/GS1 gs")); got != 2 {
		t.Errorf("alpha-blended fill count = %d, want 2 (one per series)", got)
	}
	if got := countBareOp(out, "f"); got != 2 {
		t.Errorf("bare fill op count = %d, want 2 (one polygon per series)", got)
	}
	assertNoNaNOrInf(t, out)
}

// TestDrawStackedAreas_EmptyCategoriesDrawsNothing covers the `n == 0`
// guard: with zero categories, colW would be plot.W/0 and every series
// would still enter SetAlpha/Polygon/SetAlpha around an empty (nil)
// point list if the guard were missing -- confirmed by fault-seeding
// the guard away and observing "/GS1 gs" appear once per series with
// no matching geometry, not a crash. The guard's job is precisely to
// prevent that degenerate no-op-but-not-actually-empty draw.
func TestDrawStackedAreas_EmptyCategoriesDrawsNothing(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{
		Categories: []string{},
		Series:     []statsSeries{{Name: "Done", Values: []float64{1, 2, 3}}},
	}
	drawStackedAreas(pdf, l, Frame{X: 10, Y: 10, W: 100, H: 60}, 0, 6)
	out := outputBytes(t, pdf)
	if got := bytes.Count(out, []byte("/GS1 gs")); got != 0 {
		t.Errorf("alpha-blended fill count = %d, want 0 for zero categories", got)
	}
}

// TestDrawRightAxisLine_DrawsPointsAndConnectingSegments: a 3-point
// series draws 3 circle point-markers (bare "f" fill, same technique
// established for the DAG/Pie/Flow engines' node/wedge/point counts)
// connected by 2 line segments (the plain, non-dashed "l S" pattern --
// distinct from drawDashedLine's many-short-segments signature tested
// separately below).
func TestDrawRightAxisLine_DrawsPointsAndConnectingSegments(t *testing.T) {
	pdf := newTestPDF()
	l := statsLayout{Categories: []string{"A", "B", "C"}}
	s := statsSeries{Name: "Cum%", Values: []float64{50, 80, 100}}
	drawRightAxisLine(pdf, l, s, Frame{X: 10, Y: 10, W: 100, H: 60})
	out := outputBytes(t, pdf)
	if got := countBareOp(out, "f"); got != 3 {
		t.Errorf("circle-marker fill count = %d, want 3 (one per point)", got)
	}
	if got := bytes.Count(out, []byte(" l ")); got != 2 {
		t.Errorf("connecting-segment count = %d, want 2 (n-1 segments for 3 points)", got)
	}
	assertNoNaNOrInf(t, out)
}

// TestDrawRightAxisLine_GuardsShortSeriesAndNoCategories covers both of
// drawRightAxisLine's early returns. Fault-seeding each independently
// found they are NOT equally load-bearing: `n < 2` is real (disabling
// it alone draws a single orphan point-marker) but `nCats == 0` is an
// equivalent mutation -- disabling it alone changes nothing observable,
// because the draw loop's own bound (`i < n && i < nCats`) already
// excludes every iteration when nCats is 0, the same class of finding
// this repo's ledger already documents for other guards (e.g.
// SaveScenarioChart's ID guard in internal/db). Kept in production code
// (defensive depth, avoids computing colW as +Inf) but not proven
// necessary by this test's evidence.
func TestDrawRightAxisLine_GuardsShortSeriesAndNoCategories(t *testing.T) {
	t.Run("fewer than two points", func(t *testing.T) {
		pdf := newTestPDF()
		l := statsLayout{Categories: []string{"A", "B", "C"}}
		s := statsSeries{Name: "Cum%", Values: []float64{50}}
		drawRightAxisLine(pdf, l, s, Frame{X: 10, Y: 10, W: 100, H: 60})
		out := outputBytes(t, pdf)
		if got := countBareOp(out, "f"); got != 0 {
			t.Errorf("fill count = %d, want 0 for a single-point series", got)
		}
	})
	t.Run("zero categories", func(t *testing.T) {
		pdf := newTestPDF()
		l := statsLayout{Categories: []string{}}
		s := statsSeries{Name: "Cum%", Values: []float64{50, 80, 100}}
		drawRightAxisLine(pdf, l, s, Frame{X: 10, Y: 10, W: 100, H: 60})
		out := outputBytes(t, pdf)
		if got := countBareOp(out, "f"); got != 0 {
			t.Errorf("fill count = %d, want 0 for zero categories", got)
		}
	})
}

// TestDrawRightAxisTicks_LabelsMinMidAndMax: three ticks at min/mid/max
// of the axis range, formatted as "<value>%" -- the default (nil Min/Max
// fields) is 0/50/100; a custom range must actually be used, not
// silently fall back to the default.
func TestDrawRightAxisTicks_LabelsMinMidAndMax(t *testing.T) {
	t.Run("default 0-100", func(t *testing.T) {
		pdf := newTestPDF()
		drawRightAxisTicks(pdf, Frame{X: 10, Y: 10, W: 100, H: 60}, &axisConfig{})
		out := outputBytes(t, pdf)
		for _, want := range []string{"(0%)", "(50%)", "(100%)"} {
			if !bytes.Contains(out, []byte(want)) {
				t.Errorf("output missing tick label %q", want)
			}
		}
	})
	t.Run("custom min/max", func(t *testing.T) {
		pdf := newTestPDF()
		min, max := 10.0, 50.0
		drawRightAxisTicks(pdf, Frame{X: 10, Y: 10, W: 100, H: 60}, &axisConfig{Min: &min, Max: &max})
		out := outputBytes(t, pdf)
		for _, want := range []string{"(10%)", "(30%)", "(50%)"} {
			if !bytes.Contains(out, []byte(want)) {
				t.Errorf("output missing tick label %q -- custom Min/Max not applied", want)
			}
		}
		if bytes.Contains(out, []byte("(0%)")) || bytes.Contains(out, []byte("(100%)")) {
			t.Error("output contains a default-range label -- custom Min/Max was ignored")
		}
	})
}

// TestDrawDashedLine_DrawsMultipleShortSegments: a dashed line is drawn
// as a series of short pdf.Line segments, not one continuous stroke --
// distinguishing it from a solid annotation line requires counting
// segments, not just checking a line was drawn at all. For a 50mm
// horizontal line with dash=1.2/gap=0.8, the function's own formula
// (steps := int(length/(dash+gap)); loop runs 0..steps inclusive) gives
// int(50/2.0)+1 = 26 segments -- confirmed by direct probe before
// writing this assertion, not assumed from reading the loop.
func TestDrawDashedLine_DrawsMultipleShortSegments(t *testing.T) {
	pdf := newTestPDF()
	drawDashedLine(pdf, 10, 10, 60, 10, 1.2, 0.8)
	out := outputBytes(t, pdf)
	if got := bytes.Count(out, []byte(" l ")); got != 26 {
		t.Errorf("dash segment count = %d, want 26 (int(50/(1.2+0.8))+1, confirmed by probe)", got)
	}
}

// TestDrawDashedLine_ZeroLengthDoesNotProduceNaN covers the `length ==
// 0` guard: without it, t1 := 0*stepLen/0 is a 0/0 division producing
// NaN, which -- same failure mode already established for this
// package's other unguarded-division risks (scanYBounds, the pie
// wedge-angle calc) -- Go's float formatting renders literally as
// "NaN" in the PDF content stream. Confirmed by fault-seeding the guard
// away before writing this test: the seeded version produced exactly
// this "NaN" string, not a panic and not a silently-wrong coordinate.
func TestDrawDashedLine_ZeroLengthDoesNotProduceNaN(t *testing.T) {
	pdf := newTestPDF()
	drawDashedLine(pdf, 10, 10, 10, 10, 1.2, 0.8)
	out := outputBytes(t, pdf)
	assertNoNaNOrInf(t, out)
	if got := bytes.Count(out, []byte(" l ")); got != 0 {
		t.Errorf("segment count = %d, want 0 for a zero-length line", got)
	}
}

// Exported charts print on white paper. Every text colour the stats
// renderers set must reach WCAG AA (4.5:1) against white: pie legend
// labels were drawn near-white (241,245,249; 1.1:1), legend names in
// 203,213,225 (1.5:1) and ticks in 148,163,184 (2.6:1). The oracle reads
// the colour of every text object from the content stream (textColours).
func TestStatsTextIsLegibleOnWhitePaper(t *testing.T) {
	charts := map[string]string{
		"pie":     `{"slices":[{"label":"Design","value":5},{"label":"Build","value":3}]}`,
		"bar":     `{"categories":["Q1","Q2"],"series":[{"name":"Revenue","values":[100,200]},{"name":"Cost","values":[80,90]}]}`,
		"control": `{"x":[1,2,3,4,5,6],"y":[5,6,4,9,5,6]}`,
		"pareto":  `{"items":[{"label":"A","count":30},{"label":"B","count":10}]}`,
	}
	for kind, data := range charts {
		pdf := newTestPDF()
		if err := RenderChartToPDF(pdf, kind, data, kind, Frame{X: 20, Y: 20, W: 170, H: 180}); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		texts := textColours(outputBytes(t, pdf))
		if len(texts) == 0 {
			t.Fatalf("%s: no text found", kind)
		}
		for _, d := range texts {
			if ratio := contrastOnWhite(d.r, d.g, d.b); ratio < 4.5 {
				t.Errorf("%s: %q is %.2f:1 on white, want >= 4.5", kind, d.text, ratio)
			}
		}
	}
}

// Diagram labels drawn straight on the paper, not inside a filled shape,
// must also reach 4.5:1: fishbone categories (103,232,249) and causes
// (203,213,225) and workflow branch labels (203,213,225) were 1.5:1.
// Labels inside dark node boxes are light on purpose and are not checked.
func TestDiagramLabelsOnPaperAreLegible(t *testing.T) {
	cases := []struct {
		kind, data string
		labels     []string
	}{
		{"fishbone", `{"effect":"Late","categories":[{"name":"People","causes":["Training gaps"]}]}`,
			[]string{"People", "Training gaps"}},
		{"workflow", `{"nodes":[{"id":"s","label":"Request","shape":"start"},{"id":"e","label":"Done","shape":"end"}],"edges":[{"from":"s","to":"e","label":"Yes"}]}`,
			[]string{"Yes"}},
	}
	for _, tc := range cases {
		pdf := newTestPDF()
		if err := RenderChartToPDF(pdf, tc.kind, tc.data, tc.kind, Frame{X: 20, Y: 20, W: 170, H: 180}); err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		out := outputBytes(t, pdf)
		for _, label := range tc.labels {
			r, g, b, ok := textColour(out, label)
			if !ok {
				t.Errorf("%s: label %q not found", tc.kind, label)
				continue
			}
			if ratio := contrastOnWhite(r, g, b); ratio < 4.5 {
				t.Errorf("%s: label %q is %.2f:1 on white, want >= 4.5", tc.kind, label, ratio)
			}
		}
	}
}

var (
	fillRGB   = regexp.MustCompile(`^([0-9.]+) ([0-9.]+) ([0-9.]+) rg$`)
	fillGrey  = regexp.MustCompile(`^([0-9.]+) g$`)
	textInset = regexp.MustCompile(`^q ([0-9.]+)(?: ([0-9.]+) ([0-9.]+) rg| g) BT `)
)

// drawnText is one text object and the colour fpdf drew it in.
type drawnText struct {
	text    string
	r, g, b float64
}

var shownText = regexp.MustCompile(`\((.*?)\)Tj`)

// textColours reads every text object from an uncompressed content stream
// with its colour. fpdf wraps text in "q <colour> BT ... ET Q" when the
// text colour differs from the fill colour, and otherwise draws it in the
// current fill colour, so both forms are followed.
func textColours(out []byte) []drawnText {
	parse := func(m [][]byte) (float64, float64, float64) {
		r, _ := strconv.ParseFloat(string(m[1]), 64)
		g, b := r, r
		if len(m) > 2 && len(m[2]) > 0 {
			g, _ = strconv.ParseFloat(string(m[2]), 64)
			b, _ = strconv.ParseFloat(string(m[3]), 64)
		}
		return r, g, b
	}
	var fr, fg, fb float64 // fpdf's initial fill colour is black
	var texts []drawnText
	for _, line := range bytes.Split(out, []byte("\n")) {
		if m := fillRGB.FindSubmatch(line); m != nil {
			fr, fg, fb = parse(m)
			continue
		}
		if m := fillGrey.FindSubmatch(line); m != nil {
			fr, fg, fb = parse(m)
			continue
		}
		shown := shownText.FindSubmatch(line)
		if shown == nil {
			continue
		}
		r, g, b := fr, fg, fb
		if m := textInset.FindSubmatch(line); m != nil {
			r, g, b = parse(m)
		}
		texts = append(texts, drawnText{string(shown[1]), r, g, b})
	}
	return texts
}

// textColour returns the colour of the first text object showing label.
func textColour(out []byte, label string) (r, g, b float64, ok bool) {
	for _, d := range textColours(out) {
		if d.text == label {
			return d.r, d.g, d.b, true
		}
	}
	return 0, 0, 0, false
}

// contrastOnWhite is the WCAG 2.1 contrast ratio of an sRGB colour
// (channels 0..1) against white.
func contrastOnWhite(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	l := 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
	return 1.05 / (l + 0.05)
}

// A reference line outside the data must still print: the control
// chart's UCL above every sample was skipped, and Pareto's 80% line was
// placed against the count axis instead of the percentage axis.
func TestStatsReferenceLinesPrintOutsideTheData(t *testing.T) {
	for kind, data := range map[string]string{
		// mean 5.83, UCL above the highest sample (9)
		"control": `{"x":[1,2,3,4,5,6],"y":[5,6,4,9,5,6]}`,
		// counts up to 30; the 80% line is on the percentage axis
		"pareto": `{"items":[{"label":"A","count":30},{"label":"B","count":10}]}`,
	} {
		pdf := newTestPDF()
		if err := RenderChartToPDF(pdf, kind, data, kind, Frame{X: 20, Y: 20, W: 170, H: 180}); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		out := outputBytes(t, pdf)
		want := map[string]string{"control": "(UCL)Tj", "pareto": "(80%)Tj"}[kind]
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("%s: output has no %q", kind, want)
		}
	}
}
