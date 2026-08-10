// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"bytes"
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// --- RACI ---

func TestRenderRACI_EmptyGuard(t *testing.T) {
	cases := []struct {
		name   string
		layout raciLayoutBody
	}{
		{"no roles", raciLayoutBody{Tasks: []raciTask{{ID: "t1", Title: "Deploy"}}}},
		{"no tasks", raciLayoutBody{Roles: []string{"Alice"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newTestPDF()
			if err := renderRACI(pdf, mustJSON(t, tc.layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
				t.Fatalf("renderRACI: %v", err)
			}
			out := outputBytes(t, pdf)
			if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
				t.Error("missing roles/tasks did not render the (empty) placeholder")
			}
		})
	}
}

func TestRenderRACI_DrawsAllRolesAndTasks(t *testing.T) {
	pdf := newTestPDF()
	layout := raciLayoutBody{
		Roles: []string{"Alice", "Bob", "Carol", "Dan"},
		Tasks: []raciTask{{ID: "t1", Title: "Deploy"}, {ID: "t2", Title: "Review"}},
		Cells: []raciCell{
			{TaskID: "t1", Role: "Alice", Value: "R"},
			{TaskID: "t1", Role: "Bob", Value: "A"},
			{TaskID: "t1", Role: "Carol", Value: "C"},
			{TaskID: "t1", Role: "Dan", Value: "I"},
			{TaskID: "t2", Role: "Alice", Value: ""}, // unassigned -> default fill
		},
	}
	if err := renderRACI(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderRACI: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Alice", "Bob", "Carol", "Dan", "Deploy", "Review"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}

// TestRenderRACI_TruncatesOverflowRowsWithAccurateCount verifies the
// "... and N more rows" footer's count arithmetic (len(Tasks)-i), not
// just the footer's presence. An off-by-one here would silently
// misreport how much of a signed report's matrix was cut off.
func TestRenderRACI_TruncatesOverflowRowsWithAccurateCount(t *testing.T) {
	pdf := newTestPDF()
	// rowH=6, maxRows = int((H-rowH)/rowH); H=20 -> maxRows=2.
	frame := Frame{X: 10, Y: 10, W: 260, H: 20}
	layout := raciLayoutBody{
		Roles: []string{"Alice"},
		Tasks: []raciTask{
			{ID: "t1", Title: "One"}, {ID: "t2", Title: "Two"}, {ID: "t3", Title: "Three"},
			{ID: "t4", Title: "Four"}, {ID: "t5", Title: "Five"},
		},
	}
	if err := renderRACI(pdf, mustJSON(t, layout), frame); err != nil {
		t.Fatalf("renderRACI: %v", err)
	}
	out := outputBytes(t, pdf)
	const want = "and 3 more rows" // len(Tasks)=5, maxRows=2 -> 5-2=3
	if !bytes.Contains(out, []byte(want)) {
		t.Errorf("output missing exact truncation count %q", want)
	}
	if bytes.Contains(out, []byte("Five")) {
		t.Error("output contains a task past the truncation point -- footer count and actual row count disagree")
	}
}

// --- SWOT ---

func TestRenderSWOT_EmptyGuard(t *testing.T) {
	pdf := newTestPDF()
	layout := swotLayoutBody{}
	if err := renderSWOT(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderSWOT: %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
		t.Error("zero quadrants did not render the (empty) placeholder")
	}
}

func TestRenderSWOT_DrawsAllQuadrants(t *testing.T) {
	pdf := newTestPDF()
	layout := swotLayoutBody{
		Quadrants: []swotQuadrant{
			{Key: "S", Title: "Strengths", Row: 0, Col: 0, Tone: "positive", Items: []string{"Fast"}},
			{Key: "W", Title: "Weaknesses", Row: 0, Col: 1, Tone: "negative", Items: []string{"Slow"}},
		},
	}
	if err := renderSWOT(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderSWOT: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Strengths", "Weaknesses", "Fast", "Slow"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}

// TestRenderSWOT_TruncationActuallyDropsItems proves the item-overflow
// break genuinely stops drawing later items -- not merely that a "…"
// indicator is painted on top of otherwise-complete content, which a
// presence-only check on the indicator glyph could not distinguish.
func TestRenderSWOT_TruncationActuallyDropsItems(t *testing.T) {
	pdf := newTestPDF()
	// paneH = frame.H/2 = 10; cursor starts at y+9, bottom at y+paneH-2=y+8.
	// cursor(9) + 3.5 > bottom(8) is true immediately, so the very first
	// item triggers the truncation branch before it is drawn.
	frame := Frame{X: 10, Y: 10, W: 260, H: 20}
	layout := swotLayoutBody{
		Quadrants: []swotQuadrant{
			{Key: "S", Title: "Strengths", Row: 0, Col: 0, Items: []string{"FirstItemText", "SecondItemText"}},
		},
	}
	if err := renderSWOT(pdf, mustJSON(t, layout), frame); err != nil {
		t.Fatalf("renderSWOT: %v", err)
	}
	out := outputBytes(t, pdf)
	if bytes.Contains(out, []byte("FirstItemText")) || bytes.Contains(out, []byte("SecondItemText")) {
		t.Error("truncation branch did not actually suppress overflow item text")
	}
}

// --- Generic Matrix ---

func TestRenderGenericMatrix_EmptyGuard(t *testing.T) {
	cases := []struct {
		name   string
		layout genericMatrixBody
	}{
		{"no rows", genericMatrixBody{Cols: []string{"C1"}}},
		{"no cols", genericMatrixBody{Rows: []string{"R1"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newTestPDF()
			if err := renderGenericMatrix(pdf, mustJSON(t, tc.layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
				t.Fatalf("renderGenericMatrix: %v", err)
			}
			out := outputBytes(t, pdf)
			if !bytes.Contains(out, []byte(pdfEscape("(empty)"))) {
				t.Error("missing rows/cols did not render the (empty) placeholder")
			}
		})
	}
}

func TestRenderGenericMatrix_TruncatesOverflowRowsWithAccurateCount(t *testing.T) {
	pdf := newTestPDF()
	frame := Frame{X: 10, Y: 10, W: 260, H: 20} // maxRows=2, same math as RACI
	layout := genericMatrixBody{
		Cols: []string{"C1"},
		Rows: []string{"One", "Two", "Three", "Four", "Five"},
	}
	if err := renderGenericMatrix(pdf, mustJSON(t, layout), frame); err != nil {
		t.Fatalf("renderGenericMatrix: %v", err)
	}
	out := outputBytes(t, pdf)
	const want = "and 3 more rows"
	if !bytes.Contains(out, []byte(want)) {
		t.Errorf("output missing exact truncation count %q", want)
	}
	if bytes.Contains(out, []byte("Five")) {
		t.Error("output contains a row past the truncation point -- footer count and actual row count disagree")
	}
}

// TestRenderGenericMatrix_RaggedCellsDoesNotPanic targets the two
// independent conjuncts of the cell-bounds guard
// `i < len(layout.Cells) && c < len(layout.Cells[i])`. A fixture that
// is merely short in Cells (fewer rows than Rows) only exercises the
// first conjunct; the second subtest below has the right number of
// Cells rows but one row shorter than len(Cols), which is what
// actually panics if only that half of the guard regresses.
func TestRenderGenericMatrix_RaggedCellsDoesNotPanic(t *testing.T) {
	cases := []struct {
		name  string
		cells [][]string
	}{
		{"fewer cell rows than data rows", [][]string{{"a", "b"}}},
		{"a cell row shorter than col count", [][]string{{"a", "b"}, {"c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newTestPDF()
			layout := genericMatrixBody{
				Rows:  []string{"R1", "R2"},
				Cols:  []string{"C1", "C2"},
				Cells: tc.cells,
			}
			if err := renderGenericMatrix(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
				t.Fatalf("renderGenericMatrix: %v", err)
			}
		})
	}
}

// --- Stakeholder ---

// TestRenderStakeholder_EmptyInputDoesNotPanic: unlike RACI/SWOT/
// GenericMatrix, renderStakeholder has no explicit
// len(...)==0 empty-input guard -- it just draws an empty 2x2 grid
// with no quadrant panes and no points. This is a flagged
// inconsistency (see .agent_memory note), not something this test
// pins as intended: it asserts only the safety property (no error,
// no panic, still valid PDF output), not the absence of a
// placeholder.
func TestRenderStakeholder_EmptyInputDoesNotPanic(t *testing.T) {
	pdf := newTestPDF()
	layout := stakeLayoutBody{}
	if err := renderStakeholder(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderStakeholder: %v", err)
	}
	out := outputBytes(t, pdf)
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Error("output is not a valid PDF")
	}
}

func TestRenderStakeholder_DrawsPointsAndQuadrants(t *testing.T) {
	pdf := newTestPDF()
	layout := stakeLayoutBody{
		Quadrants: []quadrantLabel{{Power: "high", Interest: "high", Title: "Manage Closely", Strategy: "Engage"}},
		Points:    []stakePoint{{ID: "p1", Name: "Sponsor", Power: "high", Interest: "high", X: 0.8, Y: 0.2}},
	}
	if err := renderStakeholder(pdf, mustJSON(t, layout), Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("renderStakeholder: %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Manage Closely", "Engage", "Sponsor"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}

// --- Full pipeline: RenderChartToPDF -> charts.Layout (matrix.ParseRACI
// / matrix.LayoutRACI) -> renderRACI. Representative for the matrix
// family's dispatch path; RACI chosen because its wire format is the
// simplest to construct correctly and its entry point is the one
// covered above at the finest granularity. ---

func TestRenderChartToPDF_RACI(t *testing.T) {
	pdf := newTestPDF()
	data := `{
		"roles": ["Alice", "Bob"],
		"tasks": [{"id": "t1", "title": "Deploy"}],
		"assignments": {"t1": {"Alice": "R", "Bob": "A"}}
	}`
	if err := RenderChartToPDF(pdf, "raci", data, "Release RACI", Frame{X: 10, Y: 10, W: 260, H: 180}); err != nil {
		t.Fatalf("RenderChartToPDF(raci): %v", err)
	}
	out := outputBytes(t, pdf)
	for _, want := range []string{"Alice", "Bob", "Deploy"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
}
