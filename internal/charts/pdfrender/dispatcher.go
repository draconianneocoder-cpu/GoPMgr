// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pdfrender renders PMForge charts directly into a gofpdf
// document using gofpdf's vector primitives (no PNG intermediate, no
// headless browser).
//
// All renderers operate within a caller-supplied Frame — an (x, y,
// width, height) rectangle in PDF page coordinates — and do not
// touch the page state outside that frame. Callers are responsible
// for creating pages and adding titles.
//
// Dispatch is by chart kind, which the caller pulls from the
// db.charts row. The kind selects one of the five engine-specific
// renderers (dag, fishbone, flow, matrix, stats) defined in sibling
// files.
package pdfrender

import (
	"encoding/json"
	"fmt"

	"github.com/jung-kurt/gofpdf"

	"pmforge/internal/charts"
)

// Frame is the bounding box a renderer paints into. Coordinates are
// in millimetres (gofpdf's default unit in PMForge).
type Frame struct {
	X, Y, W, H float64
}

// RenderChartToPDF lays out the given chart, then dispatches to the
// engine-specific renderer.
//
//	pdf       the in-progress PDF document
//	kind      the chart kind (e.g. "wbs", "raci", "line")
//	data      the raw JSON from db.charts.data
//	title     a heading to draw above the chart inside the frame
//	frame     the rectangle to paint into
//
// Returns ErrUnsupportedKind if the kind has no PDF renderer yet
// (currently every kind in the V2 taxonomy has one); the caller can
// choose to fall through to a textual placeholder.
func RenderChartToPDF(pdf *gofpdf.Fpdf, kind string, data string, title string, frame Frame) error {
	result, err := charts.Layout(charts.Kind(kind), data)
	if err != nil && !isEngineNotImpl(err) {
		return fmt.Errorf("pdfrender: layout %s: %w", kind, err)
	}

	// Reserve space for a title strip at the top of the frame.
	const titleH = 8.0
	body := Frame{
		X: frame.X,
		Y: frame.Y + titleH,
		W: frame.W,
		H: frame.H - titleH,
	}
	if title != "" {
		drawTitle(pdf, frame.X, frame.Y, frame.W, titleH, title, result.Title)
	}

	switch result.Engine {
	case charts.EngineDAG:
		// Fishbone uses a bespoke layout (effect + bones + causes)
		// rather than the shared NodeLayout/EdgeLayout shape.
		if kind == string(charts.KindFishbone) {
			return renderFishbone(pdf, result.Body, body)
		}
		return renderDAG(pdf, kind, result.Body, body)
	case charts.EngineMatrix:
		return renderMatrix(pdf, kind, result.Body, body)
	case charts.EngineFlow:
		return renderFlow(pdf, kind, result.Body, body)
	case charts.EngineStats:
		return renderStats(pdf, kind, result.Body, body)
	}
	return ErrUnsupportedKind
}

// ErrUnsupportedKind is returned by RenderChartToPDF when the chart
// kind has no registered PDF renderer.
var ErrUnsupportedKind = fmt.Errorf("pdfrender: unsupported chart kind")

func isEngineNotImpl(err error) bool {
	return err != nil && err.Error() == "charts: engine renderer not yet implemented"
}

func drawTitle(pdf *gofpdf.Fpdf, x, y, w, h float64, sectionLabel, chartKindName string) {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(0, 80, 130)
	pdf.CellFormat(w, h, sectionLabel, "", 0, "L", false, 0, "")
	if chartKindName != "" {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(x, y+h-3)
		pdf.CellFormat(w, 3, chartKindName, "", 0, "R", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
}

// parseBody is a small helper used by every engine renderer to turn
// the JSON.RawMessage body into a strongly-typed struct.
func parseBody(body json.RawMessage, out interface{}) error {
	if len(body) == 0 {
		return fmt.Errorf("pdfrender: empty layout body")
	}
	return json.Unmarshal(body, out)
}

// fit scales a Layout's natural width × height to fit inside the
// frame while preserving aspect ratio. Returns the scale factor and
// the centred offsets so callers can map layout coordinates to PDF
// coordinates with:
//
//	pdfX = frame.X + ox + layoutX*scale
//	pdfY = frame.Y + oy + layoutY*scale
func fit(layoutW, layoutH, frameW, frameH float64) (scale, ox, oy float64) {
	if layoutW <= 0 || layoutH <= 0 {
		return 1, 0, 0
	}
	sx := frameW / layoutW
	sy := frameH / layoutH
	scale = sx
	if sy < scale {
		scale = sy
	}
	if scale > 1 {
		scale = 1 // never upscale; charts look better small than blown up
	}
	ox = (frameW - layoutW*scale) / 2
	oy = (frameH - layoutH*scale) / 2
	return
}
