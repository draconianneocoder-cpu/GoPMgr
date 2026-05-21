// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"
	"math"

	"github.com/jung-kurt/gofpdf"
)

// Local mirrors of stats.StatsLayout so the PDF renderer doesn't
// depend on the stats package directly.
type statsSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	Type   string    `json:"type,omitempty"`
	Color  string    `json:"color,omitempty"`
	YAxis  string    `json:"y_axis,omitempty"`
	Dashed bool      `json:"dashed,omitempty"`
}
type axisConfig struct {
	Label string   `json:"label,omitempty"`
	Type  string   `json:"type,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}
type annotation struct {
	Type   string  `json:"type"`
	Value  float64 `json:"value"`
	Label  string  `json:"label,omitempty"`
	Color  string  `json:"color,omitempty"`
	Dashed bool    `json:"dashed,omitempty"`
}
type pointFlag struct {
	Series int    `json:"series"`
	Point  int    `json:"point"`
	Color  string `json:"color"`
	Reason string `json:"reason,omitempty"`
}
type pieSlice struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Pct   float64 `json:"pct"`
	Color string  `json:"color,omitempty"`
}
type statsLayout struct {
	Kind        string        `json:"kind"`
	Title       string        `json:"title,omitempty"`
	XAxis       axisConfig    `json:"x_axis"`
	YAxis       axisConfig    `json:"y_axis"`
	YAxisRight  *axisConfig   `json:"y_axis_right,omitempty"`
	Categories  []string      `json:"categories,omitempty"`
	Series      []statsSeries `json:"series,omitempty"`
	Stacked     bool          `json:"stacked,omitempty"`
	Slices      []pieSlice    `json:"slices,omitempty"`
	Annotations []annotation  `json:"annotations,omitempty"`
	Flags       []pointFlag   `json:"flags,omitempty"`
}

func renderStats(pdf *gofpdf.Fpdf, kind string, body json.RawMessage, frame Frame) error {
	var l statsLayout
	if err := parseBody(body, &l); err != nil {
		return err
	}
	if kind == "pie" {
		return renderPie(pdf, l, frame)
	}
	return renderCartesian(pdf, l, frame)
}

// ---------- Pie ----------

func renderPie(pdf *gofpdf.Fpdf, l statsLayout, frame Frame) error {
	if len(l.Slices) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}
	var total float64
	for _, s := range l.Slices {
		total += s.Value
	}
	if total <= 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(no values)")
		return nil
	}

	// Pie circle on the left half, legend on the right.
	pieW := frame.W * 0.55
	legW := frame.W - pieW
	cx := frame.X + pieW/2
	cy := frame.Y + frame.H/2
	r := math.Min(pieW, frame.H) / 2 * 0.85

	// Approximate wedges with thin triangular polygons. 60 segments
	// per wedge is plenty smooth at typical print sizes.
	startAngle := -math.Pi / 2 // start at 12 o'clock
	pdf.SetDrawColor(15, 23, 42)
	pdf.SetLineWidth(0.1)
	for i, s := range l.Slices {
		fr, fg, fb := paletteRGB(i, s.Color)
		pdf.SetFillColor(fr, fg, fb)
		sweep := (s.Value / total) * 2 * math.Pi
		drawPieWedge(pdf, cx, cy, r, startAngle, startAngle+sweep)
		startAngle += sweep
	}

	// Legend.
	pdf.SetFont("Helvetica", "", 7)
	yCursor := frame.Y + 4
	for i, s := range l.Slices {
		fr, fg, fb := paletteRGB(i, s.Color)
		pdf.SetFillColor(fr, fg, fb)
		pdf.Rect(frame.X+pieW+2, yCursor, 3, 3, "F")
		pdf.SetTextColor(241, 245, 249)
		pdf.SetXY(frame.X+pieW+7, yCursor-0.5)
		label := s.Label
		if len(label) > 22 {
			label = label[:21] + "…"
		}
		pdf.CellFormat(legW-10, 3, label, "", 0, "L", false, 0, "")
		pdf.SetXY(frame.X+pieW+7, yCursor+2.5)
		pdf.SetFont("Helvetica", "I", 6)
		pdf.SetTextColor(148, 163, 184)
		pdf.CellFormat(legW-10, 2.5,
			gpct(s.Pct), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		yCursor += 7
		if yCursor > frame.Y+frame.H-4 {
			break
		}
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// drawPieWedge draws a single pie wedge as a many-sided polygon
// going from `start` to `end` (radians, 0 = 3 o'clock).
func drawPieWedge(pdf *gofpdf.Fpdf, cx, cy, r, start, end float64) {
	const segs = 48
	pts := []gofpdf.PointType{{X: cx, Y: cy}}
	step := (end - start) / segs
	for i := 0; i <= segs; i++ {
		a := start + step*float64(i)
		pts = append(pts, gofpdf.PointType{
			X: cx + math.Cos(a)*r,
			Y: cy + math.Sin(a)*r,
		})
	}
	pdf.Polygon(pts, "FD")
}

// ---------- Cartesian (line / bar / pareto / burnup / burndown /
// cumulative_flow / control) ----------

func renderCartesian(pdf *gofpdf.Fpdf, l statsLayout, frame Frame) error {
	if len(l.Series) == 0 || len(l.Categories) == 0 {
		drawEmptyChartPlaceholder(pdf, frame, "(empty)")
		return nil
	}

	// Plot area: leave room for axis labels.
	padL, padR, padT, padB := 12.0, 8.0, 4.0, 10.0
	if l.YAxisRight != nil {
		padR = 14
	}
	plot := Frame{
		X: frame.X + padL,
		Y: frame.Y + padT,
		W: frame.W - padL - padR,
		H: frame.H - padT - padB,
	}

	// Y-axis bounds: scan all left-axis series.
	yMin, yMax := scanYBounds(l.Series, l.Stacked, "left")
	if l.YAxis.Min != nil {
		yMin = *l.YAxis.Min
	}
	if l.YAxis.Max != nil {
		yMax = *l.YAxis.Max
	}
	if yMax <= yMin {
		yMax = yMin + 1
	}

	// Plot frame
	pdf.SetDrawColor(51, 65, 85)
	pdf.SetLineWidth(0.2)
	pdf.Rect(plot.X, plot.Y, plot.W, plot.H, "D")

	// Y-axis ticks (4 divisions).
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetTextColor(148, 163, 184)
	for i := 0; i <= 4; i++ {
		v := yMin + (yMax-yMin)*float64(i)/4
		y := plot.Y + plot.H - (v-yMin)/(yMax-yMin)*plot.H
		pdf.SetDrawColor(30, 41, 59)
		pdf.Line(plot.X, y, plot.X+plot.W, y) // gridline
		pdf.SetXY(frame.X, y-1.5)
		pdf.CellFormat(padL-1, 3, gnum(v), "", 0, "R", false, 0, "")
	}

	// X-axis category labels (every ceil(n/10)th label to avoid clutter).
	n := len(l.Categories)
	if n > 0 {
		stride := 1
		if n > 10 {
			stride = (n + 9) / 10
		}
		colW := plot.W / float64(n)
		for i, c := range l.Categories {
			if i%stride != 0 && i != n-1 {
				continue
			}
			x := plot.X + (float64(i)+0.5)*colW
			pdf.SetXY(x-6, plot.Y+plot.H+0.5)
			pdf.CellFormat(12, 3, c, "", 0, "C", false, 0, "")
		}
	}

	// Bar series: drawn first so lines lay on top.
	hasBars := false
	for _, s := range l.Series {
		if s.Type == "bar" {
			hasBars = true
		}
	}
	if hasBars {
		drawBars(pdf, l, plot, yMin, yMax)
	}

	// Stacked area: collapse all series into one stack and fill.
	if l.Stacked {
		drawStackedAreas(pdf, l, plot, yMin, yMax)
	}

	// Line series.
	for i, s := range l.Series {
		if s.Type == "bar" {
			continue
		}
		if s.YAxis == "right" {
			drawRightAxisLine(pdf, l, s, plot)
			continue
		}
		drawLineSeries(pdf, s, i, l.Flags, plot, yMin, yMax, len(l.Categories))
	}

	// Annotations (Mean / UCL / LCL etc.).
	for _, a := range l.Annotations {
		if a.Type != "horizontal_line" {
			continue
		}
		if a.Value < yMin || a.Value > yMax {
			continue
		}
		y := plot.Y + plot.H - (a.Value-yMin)/(yMax-yMin)*plot.H
		r, g, b := hexRGB(a.Color, 148, 163, 184)
		pdf.SetDrawColor(r, g, b)
		pdf.SetLineWidth(0.25)
		if a.Dashed {
			drawDashedLine(pdf, plot.X, y, plot.X+plot.W, y, 1.2, 0.8)
		} else {
			pdf.Line(plot.X, y, plot.X+plot.W, y)
		}
		if a.Label != "" {
			pdf.SetFont("Helvetica", "", 5)
			pdf.SetTextColor(r, g, b)
			pdf.SetXY(plot.X+0.5, y-2.2)
			pdf.CellFormat(20, 2, a.Label, "", 0, "L", false, 0, "")
		}
	}

	// Right-axis (Pareto cumulative %): light grey ticks at 50/100.
	if l.YAxisRight != nil {
		drawRightAxisTicks(pdf, plot, l.YAxisRight)
	}

	// Legend (top of plot, small).
	drawStatsLegend(pdf, l.Series, frame)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
	return nil
}

// ---------- Series drawing helpers ----------

func drawBars(pdf *gofpdf.Fpdf, l statsLayout, plot Frame, yMin, yMax float64) {
	n := len(l.Categories)
	if n == 0 {
		return
	}
	colW := plot.W / float64(n)

	// Count bar series; multi-series bars get split into sub-bars.
	var barSeries []statsSeries
	var barSeriesIdx []int
	for i, s := range l.Series {
		if s.Type == "bar" {
			barSeries = append(barSeries, s)
			barSeriesIdx = append(barSeriesIdx, i)
		}
	}
	if len(barSeries) == 0 {
		return
	}
	groupPad := colW * 0.1
	barW := (colW - 2*groupPad) / float64(len(barSeries))

	zeroY := plot.Y + plot.H - (math.Max(0, -yMin))/(yMax-yMin)*plot.H

	for si, s := range barSeries {
		fr, fg, fb := paletteRGB(barSeriesIdx[si], s.Color)
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(fr, fg, fb)
		for i, v := range s.Values {
			if i >= n {
				break
			}
			x := plot.X + float64(i)*colW + groupPad + float64(si)*barW
			y := plot.Y + plot.H - (v-yMin)/(yMax-yMin)*plot.H
			h := zeroY - y
			if h < 0 {
				h = -h
				y = zeroY
			}
			pdf.Rect(x, y, barW*0.85, h, "F")
		}
	}
}

func drawStackedAreas(pdf *gofpdf.Fpdf, l statsLayout, plot Frame, yMin, yMax float64) {
	n := len(l.Categories)
	if n == 0 {
		return
	}
	colW := plot.W / float64(n)
	// Running totals per point.
	stack := make([]float64, n)
	for i := range stack {
		stack[i] = 0
	}
	for si, s := range l.Series {
		fr, fg, fb := paletteRGB(si, s.Color)
		// Build polygon: bottom of stack, then top.
		var pts []gofpdf.PointType
		for i := 0; i < n && i < len(s.Values); i++ {
			x := plot.X + (float64(i)+0.5)*colW
			y := plot.Y + plot.H - (stack[i]-yMin)/(yMax-yMin)*plot.H
			pts = append(pts, gofpdf.PointType{X: x, Y: y})
		}
		for i := n - 1; i >= 0; i-- {
			if i >= len(s.Values) {
				continue
			}
			top := stack[i] + s.Values[i]
			x := plot.X + (float64(i)+0.5)*colW
			y := plot.Y + plot.H - (top-yMin)/(yMax-yMin)*plot.H
			pts = append(pts, gofpdf.PointType{X: x, Y: y})
		}
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(fr, fg, fb)
		pdf.SetAlpha(0.55, "Normal")
		pdf.Polygon(pts, "F")
		pdf.SetAlpha(1.0, "Normal")
		// Update running stack.
		for i := 0; i < n && i < len(s.Values); i++ {
			stack[i] += s.Values[i]
		}
	}
}

func drawLineSeries(pdf *gofpdf.Fpdf, s statsSeries, idx int, flags []pointFlag, plot Frame, yMin, yMax float64, nCats int) {
	fr, fg, fb := paletteRGB(idx, s.Color)
	pdf.SetDrawColor(fr, fg, fb)
	pdf.SetLineWidth(0.5)
	n := len(s.Values)
	if n > nCats {
		n = nCats
	}
	if n < 2 {
		return
	}
	colW := plot.W / float64(nCats)
	var prevX, prevY float64
	for i := 0; i < n; i++ {
		x := plot.X + (float64(i)+0.5)*colW
		y := plot.Y + plot.H - (s.Values[i]-yMin)/(yMax-yMin)*plot.H
		if i > 0 {
			if s.Dashed {
				drawDashedLine(pdf, prevX, prevY, x, y, 1.2, 0.8)
			} else {
				pdf.Line(prevX, prevY, x, y)
			}
		}
		prevX, prevY = x, y
		// Point marker; red for flagged points (Control chart).
		flagged := false
		for _, f := range flags {
			if f.Series == idx && f.Point == i {
				flagged = true
				break
			}
		}
		if flagged {
			pdf.SetFillColor(239, 68, 68)
		} else {
			pdf.SetFillColor(fr, fg, fb)
		}
		pdf.Circle(x, y, 0.8, "F")
	}
}

// drawRightAxisLine draws a series mapped onto a 0..100 right axis
// (Pareto cumulative percentage).
func drawRightAxisLine(pdf *gofpdf.Fpdf, l statsLayout, s statsSeries, plot Frame) {
	rMin := 0.0
	rMax := 100.0
	if l.YAxisRight != nil {
		if l.YAxisRight.Min != nil {
			rMin = *l.YAxisRight.Min
		}
		if l.YAxisRight.Max != nil {
			rMax = *l.YAxisRight.Max
		}
	}
	fr, fg, fb := paletteRGB(99, s.Color) // unique idx → custom palette
	if s.Color == "" {
		fr, fg, fb = 239, 68, 68
	}
	pdf.SetDrawColor(fr, fg, fb)
	pdf.SetLineWidth(0.5)
	n := len(s.Values)
	if n < 2 {
		return
	}
	nCats := len(l.Categories)
	if nCats == 0 {
		return
	}
	colW := plot.W / float64(nCats)
	var prevX, prevY float64
	for i := 0; i < n && i < nCats; i++ {
		x := plot.X + (float64(i)+0.5)*colW
		y := plot.Y + plot.H - (s.Values[i]-rMin)/(rMax-rMin)*plot.H
		if i > 0 {
			pdf.Line(prevX, prevY, x, y)
		}
		prevX, prevY = x, y
		pdf.SetFillColor(fr, fg, fb)
		pdf.Circle(x, y, 0.8, "F")
	}
}

func drawRightAxisTicks(pdf *gofpdf.Fpdf, plot Frame, ax *axisConfig) {
	min := 0.0
	max := 100.0
	if ax.Min != nil {
		min = *ax.Min
	}
	if ax.Max != nil {
		max = *ax.Max
	}
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetTextColor(148, 163, 184)
	for i := 0; i <= 2; i++ {
		v := min + (max-min)*float64(i)/2
		y := plot.Y + plot.H - (v-min)/(max-min)*plot.H
		pdf.SetXY(plot.X+plot.W+0.5, y-1.5)
		pdf.CellFormat(12, 3, gnum(v)+"%", "", 0, "L", false, 0, "")
	}
}

func drawStatsLegend(pdf *gofpdf.Fpdf, series []statsSeries, frame Frame) {
	pdf.SetFont("Helvetica", "", 6)
	cursor := frame.X + 2
	yLegend := frame.Y + frame.H - 3.5
	for i, s := range series {
		fr, fg, fb := paletteRGB(i, s.Color)
		pdf.SetFillColor(fr, fg, fb)
		pdf.Rect(cursor, yLegend, 2.5, 2.5, "F")
		pdf.SetTextColor(203, 213, 225)
		pdf.SetXY(cursor+3, yLegend-0.4)
		w := pdf.GetStringWidth(s.Name) + 4
		pdf.CellFormat(w, 3, s.Name, "", 0, "L", false, 0, "")
		cursor += w + 4
		if cursor > frame.X+frame.W-20 {
			break
		}
	}
	pdf.SetTextColor(0, 0, 0)
}

// ---------- helpers ----------

func scanYBounds(series []statsSeries, stacked bool, side string) (lo, hi float64) {
	lo, hi = 0, 0
	first := true
	if stacked {
		// For stacked series, the y-bound is the per-point sum.
		var sums []float64
		for _, s := range series {
			if s.YAxis == "right" {
				continue
			}
			for i, v := range s.Values {
				if i >= len(sums) {
					sums = append(sums, 0)
				}
				sums[i] += v
			}
		}
		for _, v := range sums {
			if first || v < lo {
				lo = v
			}
			if first || v > hi {
				hi = v
			}
			first = false
		}
	} else {
		for _, s := range series {
			if (side == "left" && s.YAxis == "right") || (side == "right" && s.YAxis != "right") {
				continue
			}
			for _, v := range s.Values {
				if first || v < lo {
					lo = v
				}
				if first || v > hi {
					hi = v
				}
				first = false
			}
		}
	}
	if first {
		return 0, 1
	}
	if lo > 0 {
		lo = 0 // anchor y=0 when all values are positive
	}
	return lo, hi
}

func paletteRGB(idx int, hex string) (int, int, int) {
	if hex != "" {
		return hexRGB(hex, 34, 211, 238)
	}
	pal := [][3]int{
		{34, 211, 238},   // cyan
		{245, 158, 11},   // amber
		{34, 197, 94},    // green
		{168, 85, 247},   // purple
		{239, 68, 68},    // red
		{14, 165, 233},   // sky
		{234, 179, 8},    // yellow
		{148, 163, 184},  // slate
	}
	c := pal[idx%len(pal)]
	return c[0], c[1], c[2]
}

// hexRGB parses "#RRGGBB". On parse failure it returns the fallback
// triple.
func hexRGB(hex string, fr, fg, fb int) (int, int, int) {
	if hex == "" {
		return fr, fg, fb
	}
	s := hex
	if s[0] == '#' {
		s = s[1:]
	}
	if len(s) != 6 {
		return fr, fg, fb
	}
	var r, g, b int
	if _, err := fmtSscanfHex(s, &r, &g, &b); err != nil {
		return fr, fg, fb
	}
	return r, g, b
}

// fmtSscanfHex parses a 6-char hex string into three ints. We avoid
// the strconv/fmt boilerplate of three separate parses.
func fmtSscanfHex(s string, r, g, b *int) (int, error) {
	parse := func(t string) (int, error) {
		var v int
		for _, c := range t {
			v *= 16
			switch {
			case c >= '0' && c <= '9':
				v += int(c - '0')
			case c >= 'a' && c <= 'f':
				v += int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v += int(c-'A') + 10
			default:
				return 0, errInvalidHex
			}
		}
		return v, nil
	}
	rr, err := parse(s[0:2])
	if err != nil {
		return 0, err
	}
	gg, err := parse(s[2:4])
	if err != nil {
		return 0, err
	}
	bb, err := parse(s[4:6])
	if err != nil {
		return 0, err
	}
	*r, *g, *b = rr, gg, bb
	return 3, nil
}

type errHexT struct{}

func (errHexT) Error() string { return "invalid hex" }

var errInvalidHex = errHexT{}

func gnum(v float64) string {
	// Compact representation: drop trailing zeros, max 2 decimals.
	if v == math.Trunc(v) {
		// Integer
		i := int64(v)
		return itoa64(i)
	}
	// One decimal usually fits axis tick width.
	return fmtFloat1(v)
}

func gpct(v float64) string {
	return fmtFloat1(v) + "%"
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func fmtFloat1(v float64) string {
	// One decimal place; avoid strconv to keep this file self-contained.
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	scaled := math.Round(v * 10)
	intPart := int64(scaled / 10)
	frac := int64(math.Abs(scaled)) - int64(math.Abs(float64(intPart)))*10
	if v < 0 && intPart == 0 {
		return "-0." + itoa64(frac)
	}
	return itoa64(intPart) + "." + itoa64(frac)
}

func drawDashedLine(pdf *gofpdf.Fpdf, x1, y1, x2, y2, dash, gap float64) {
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	stepLen := dash + gap
	steps := int(length / stepLen)
	for i := 0; i <= steps; i++ {
		t1 := float64(i) * stepLen / length
		t2 := math.Min(t1+dash/length, 1)
		pdf.Line(x1+dx*t1, y1+dy*t1, x1+dx*t2, y1+dy*t2)
	}
}
