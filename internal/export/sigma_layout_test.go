// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/sigma/domain"
)

// shownText is one string a PDF draws, placed in mm from the top-left
// corner of its page at the start of its baseline.
type shownText struct {
	page int
	x, y float64
	text string
}

var sigmaPDFTd = regexp.MustCompile(`BT (-?[\d.]+) (-?[\d.]+) Td \(((?:\\.|[^\\)])*)\)\s*Tj`)

// sigmaPDFTextAt returns every string a report draws with its position.
// fpdf writes one content stream per page, in page order, and places
// each string with "BT x y Td (...) Tj" in points from the bottom left.
func sigmaPDFTextAt(t *testing.T, pdf []byte) []shownText {
	t.Helper()
	const ptPerMM = 72 / 25.4
	_, pageH := pdfdoc.New("P", "mm", "A4", "").GetPageSize()
	var shown []shownText
	page := 0
	for _, m := range sigmaPDFStream.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		content, err := io.ReadAll(r)
		if err != nil || !bytes.Contains(content, []byte("BT ")) {
			continue
		}
		page++
		for _, td := range sigmaPDFTd.FindAllSubmatch(content, -1) {
			x, errX := strconv.ParseFloat(string(td[1]), 64)
			y, errY := strconv.ParseFloat(string(td[2]), 64)
			if errX != nil || errY != nil {
				t.Fatalf("unreadable text position %q %q", td[1], td[2])
			}
			shown = append(shown, shownText{page: page, x: x / ptPerMM, y: pageH - y/ptPerMM, text: decodePDFString(td[3])})
		}
	}
	return shown
}

// find returns the drawn strings equal to text.
func find(shown []shownText, text string) []shownText {
	var out []shownText
	for _, s := range shown {
		if s.text == text {
			out = append(out, s)
		}
	}
	return out
}

// sigmaColumns returns each column's left and right edges in mm.
func sigmaColumns(widths []float64) [][2]float64 {
	left, _, _, _ := pdfdoc.New("P", "mm", "A4", "").GetMargins()
	cols := make([][2]float64, len(widths))
	for i, w := range widths {
		cols[i] = [2]float64{left, left + w}
		left += w
	}
	return cols
}

var controlPlanWidths = []float64{35, 30, 25, 25, 30, 45}

func renderControlPlan(t *testing.T, plan []domain.ControlPlanItem) []shownText {
	t.Helper()
	pdf, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3"}, "USD", nil, nil, nil, nil, plan)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	return sigmaPDFTextAt(t, pdf)
}

// Each Control Plan value prints inside its own column. The rows used to
// put columns 3 to 6 on top of columns 2 and 3: after each MultiCell the
// next column started from the left margin instead of the previous column.
func TestSigmaControlPlanValuesStayInTheirOwnColumns(t *testing.T) {
	shown := renderControlPlan(t, []domain.ControlPlanItem{{
		ProcessStep: "STEPA", Metric: "METRB", Specification: "SPECC", Frequency: "FREQD", Owner: "OWNRE", ResponsePlan: "RESPF",
	}})
	cols := sigmaColumns(controlPlanWidths)
	for i, token := range []string{"STEPA", "METRB", "SPECC", "FREQD", "OWNRE", "RESPF"} {
		got := find(shown, token)
		if len(got) != 1 {
			t.Fatalf("%s drawn %d times, want once", token, len(got))
		}
		if x := got[0].x; x < cols[i][0] || x >= cols[i][1] {
			t.Errorf("%s starts at x = %.1f mm, outside column %d (%.0f to %.0f mm)", token, x, i+1, cols[i][0], cols[i][1])
		}
	}
}

// A row is as tall as its tallest cell, so a wrapped response plan stays in
// its column and the next row starts below it. Row height used to be
// measured with the first column's width for every cell.
func TestSigmaControlPlanRowIsAsTallAsItsTallestCell(t *testing.T) {
	plan := "Stop the line, quarantine the last two hours of output, recalibrate the torque driver, and notify the process owner before restarting."
	shown := renderControlPlan(t, []domain.ControlPlanItem{
		{ProcessStep: "Torque", Metric: "Nm", Specification: "12 ± 0.5", Frequency: "Hourly", Owner: "Cell lead", ResponsePlan: plan},
		{ProcessStep: "NEXTROW", Metric: "m", Specification: "s", Frequency: "f", Owner: "o", ResponsePlan: "r"},
	})
	col := sigmaColumns(controlPlanWidths)[5]
	next := find(shown, "NEXTROW")
	if len(next) != 1 {
		t.Fatalf("second row drawn %d times, want once", len(next))
	}

	var lines []string
	for _, s := range shown {
		if s.page == next[0].page && s.x >= col[0] && s.x < col[1] && s.y < next[0].y {
			lines = append(lines, s.text)
		}
	}
	// The column also holds the header; drop it.
	lines = slices.DeleteFunc(lines, func(s string) bool { return s == "Response Plan" })
	if len(lines) < 3 {
		t.Fatalf("response plan drew %d lines above the next row, want it wrapped onto 3 or more: %q", len(lines), lines)
	}
	if got := strings.Join(lines, " "); got != plan {
		t.Errorf("response plan column reads %q, want %q", got, plan)
	}
}

// A long solution title wraps inside the Solution column. It used to be
// one centered line that started off the left edge of the page.
func TestSigmaSolutionTitleWrapsInsideItsColumn(t *testing.T) {
	title := "Replace the manual torque check with an inline sensor that rejects out-of-spec assemblies"
	pdf, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3"}, "EUR", nil, nil, nil,
		[]domain.Solution{{Title: title, Impact: 8, Effort: 3, Risk: 2, Cost: 1284500, Status: "pilot"}}, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	shown := sigmaPDFTextAt(t, pdf)
	cost := find(shown, "1,284,500.00")
	if len(cost) != 1 {
		t.Fatalf("cost drawn %d times, want once", len(cost))
	}
	col := sigmaColumns([]float64{45, 20, 20, 20, 30, 25, 20})[0]

	var lines []string
	for _, s := range shown {
		if s.page == cost[0].page && s.x < col[1] && s.y >= cost[0].y {
			if s.x < col[0] {
				t.Errorf("title line %q starts at x = %.1f mm, left of its column (%.0f mm)", s.text, s.x, col[0])
			}
			lines = append(lines, s.text)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("title drew %d lines, want it wrapped: %q", len(lines), lines)
	}
	if got := strings.Join(lines, " "); got != title {
		t.Errorf("Solution column reads %q, want %q", got, title)
	}
}

// A table longer than a page continues on the next page under its header,
// and no row crosses the bottom margin.
func TestSigmaTablesContinueUnderTheirHeaderOnANewPage(t *testing.T) {
	var plan []domain.ControlPlanItem
	for i := range 60 {
		plan = append(plan, domain.ControlPlanItem{
			ProcessStep: fmt.Sprintf("STEP%02d", i), Metric: "Defects per unit", Specification: "≤ 0.5%",
			Frequency: "Each shift", Owner: "Quality", ResponsePlan: "Hold the lot and inform the process owner.",
		})
	}
	shown := renderControlPlan(t, plan)
	headers := find(shown, "Response Plan")
	if len(headers) < 2 {
		t.Fatalf("header drawn %d times, want it repeated on each page of the table", len(headers))
	}
	for i := 1; i < len(headers); i++ {
		if headers[i].page == headers[i-1].page {
			t.Errorf("header drawn twice on page %d", headers[i].page)
		}
	}
	assertInsideTheBodyArea(t, shown)
	for i := range 60 {
		if len(find(shown, fmt.Sprintf("STEP%02d", i))) != 1 {
			t.Errorf("row STEP%02d drawn %d times, want once", i, len(find(shown, fmt.Sprintf("STEP%02d", i))))
		}
	}
}

// A cell too long for one page ends in an ellipsis, so its row still fits
// on a page and the rows after it still print.
func TestSigmaTableRowNeverRunsTallerThanAPage(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("Escalate to the plant manager and record the event. ", 200))
	shown := renderControlPlan(t, []domain.ControlPlanItem{
		{ProcessStep: "Torque", Metric: "Nm", Specification: "12", Frequency: "Hourly", Owner: "Lead", ResponsePlan: long},
		{ProcessStep: "AFTER", Metric: "m", Specification: "s", Frequency: "f", Owner: "o", ResponsePlan: "r"},
	})
	assertInsideTheBodyArea(t, shown)
	ellipsis := 0
	for _, s := range shown {
		if strings.HasSuffix(s.text, "…") {
			ellipsis++
		}
	}
	if ellipsis != 1 {
		t.Errorf("%d lines end in an ellipsis, want 1 for the shortened cell", ellipsis)
	}
	if len(find(shown, "AFTER")) != 1 {
		t.Error("the row after the long cell is missing")
	}
}

// assertInsideTheBodyArea fails for text left of the left margin or below
// the bottom margin.
func assertInsideTheBodyArea(t *testing.T, shown []shownText) {
	t.Helper()
	ref := pdfdoc.New("P", "mm", "A4", "")
	left, _, _, _ := ref.GetMargins()
	_, pageH := ref.GetPageSize()
	_, bottom := ref.GetAutoPageBreak()
	for _, s := range shown {
		if s.x < left || s.y > pageH-bottom {
			t.Errorf("page %d: %q at (%.1f, %.1f) mm is outside the body area", s.page, s.text, s.x, s.y)
		}
	}
}

// wrapCellText keeps every line inside the cell and loses no text.
func TestWrapCellText(t *testing.T) {
	pdf := pdfdoc.New("P", "mm", "A4", "")
	registerPDFABaseline(pdf)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 9)
	avail := func(w float64) float64 { return w - 2*pdf.GetCellMargin() }

	cases := []struct {
		name  string
		text  string
		width float64
		want  []string // nil: check the invariants only
	}{
		{"empty", "", 45, []string{""}},
		{"short", "Inline sensor", 45, []string{"Inline sensor"}},
		{"trimmed", "  Inline sensor \n", 45, []string{"Inline sensor"}},
		{"line breaks", "First step\nSecond step", 45, []string{"First step", "Second step"}},
		{"sentence", "Replace the manual torque check with an inline sensor that rejects out-of-spec assemblies", 45, nil},
		{"one long word", strings.Repeat("Ab", 40), 20, nil},
		{"non-Latin text", "トルク検査をインラインセンサーに置き換える 품질 관리 계획", 25, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines := wrapCellText(pdf, c.text, c.width)
			if c.want != nil && !slices.Equal(lines, c.want) {
				t.Fatalf("lines = %q, want %q", lines, c.want)
			}
			for _, line := range lines {
				if w := pdf.GetStringWidth(line); w > avail(c.width) {
					t.Errorf("line %q is %.1f mm wide, wider than the %.1f mm the cell holds", line, w, avail(c.width))
				}
			}
			if strings.Join(strings.Fields(strings.Join(lines, " ")), "") != strings.Join(strings.Fields(c.text), "") {
				t.Errorf("lines %q lose or change text from %q", lines, c.text)
			}
			if c.want == nil && len(lines) < 2 {
				t.Errorf("lines = %q, want the text wrapped", lines)
			}
		})
	}
}

// Long text in every section wraps inside the page: the cover title, the
// sponsor, scope items, the SIPOC process and items, fishbone categories,
// causes, and whys. Each used to be one line that ran off the right edge.
// List items wrap under their own text, not under the bullet.
func TestSigmaReportWrapsLongTextInsideThePage(t *testing.T) {
	words := func(tag string, n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf("%s%02d", tag, i)
		}
		return strings.Join(parts, " ")
	}
	title, sponsor, scope := words("title", 20), words("sponsor", 25), words("scope", 40)
	process, supplier := words("process", 25), words("supplier", 40)
	category, cause, why := words("category", 30), words("cause", 40), words("why", 40)

	pdf, _, err := GenerateSigmaReportPDF(domain.Project{Title: title}, "USD",
		&domain.Charter{ProblemStatement: "Scrap is high.", Sponsor: sponsor, ScopeIn: []string{scope}},
		&domain.SIPOCData{ProcessName: process, Elements: []domain.SIPOCElement{{Category: "supplier", Description: supplier}}},
		&domain.FishboneData{Branches: []domain.FishboneBranch{{Category: category, Causes: []domain.Cause{{Description: cause, FiveWhys: []string{why}}}}}},
		nil, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	shown := sigmaPDFTextAt(t, pdf)
	assertInsideTheBodyArea(t, shown)

	ref := pdfdoc.New("P", "mm", "A4", "")
	registerPDFABaseline(ref)
	ref.AddPage()
	pageW, _ := ref.GetPageSize()
	left, _, right, _ := ref.GetMargins()

	cases := []struct {
		tag, text, prefix string
		style             string
		size              float64
		listItem          bool
	}{
		{"title", title, "", "", 16, false},
		{"sponsor", sponsor, "Sponsor: ", "B", 11, false},
		{"scope", scope, "", "", 10, true},
		{"process", process, "Process: ", "B", 11, false},
		{"supplier", supplier, "", "", 10, true},
		{"category", category, "", "B", 11, false},
		{"cause", cause, "", "", 10, true},
		{"why", why, "", "I", 9, true},
	}
	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			ownWord := regexp.MustCompile(`(^|\s)` + c.tag + `\d{2}(\s|$)`)
			var lines []shownText
			for _, s := range shown {
				if ownWord.MatchString(s.text) {
					lines = append(lines, s)
				}
			}
			if len(lines) < 2 {
				t.Fatalf("drawn as %d lines, want it wrapped", len(lines))
			}
			var texts []string
			ref.SetFont("Helvetica", c.style, c.size)
			for _, l := range lines {
				texts = append(texts, l.text)
				if end := l.x + ref.GetStringWidth(l.text); end > pageW-right+0.1 {
					t.Errorf("line %q ends at x = %.1f mm, past the right margin (%.1f mm)", l.text, end, pageW-right)
				}
				if c.listItem && (l.x != lines[0].x || l.x < left+4) {
					t.Errorf("line %q starts at x = %.1f mm; want every line of the item at its text's indent (%.1f mm)", l.text, l.x, lines[0].x)
				}
			}
			if got := strings.Join(texts, " "); got != c.prefix+c.text {
				t.Errorf("reads %q, want %q", got, c.prefix+c.text)
			}
		})
	}
}

// A list long enough to run onto more pages keeps every wrapped line at
// the item's text indent, including the lines after a page break.
func TestSigmaListItemsKeepTheirIndentAcrossPages(t *testing.T) {
	var elements []domain.SIPOCElement
	for i := range 40 {
		elements = append(elements, domain.SIPOCElement{Category: "supplier",
			Description: fmt.Sprintf("item%02d supplies coil steel, fasteners, and the torque drivers that the stamping line and the weld cell need every shift", i)})
	}
	pdf, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3"}, "USD", nil,
		&domain.SIPOCData{Elements: elements}, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	shown := sigmaPDFTextAt(t, pdf)
	assertInsideTheBodyArea(t, shown)

	heading := find(shown, "SIPOC Diagram")
	if len(heading) != 1 {
		t.Fatalf("SIPOC heading drawn %d times, want once", len(heading))
	}
	var indent float64
	pages := map[int]bool{}
	for _, s := range shown {
		if s.page < heading[0].page || slices.Contains([]string{"SIPOC Diagram", "Suppliers", "- "}, s.text) {
			continue
		}
		if indent == 0 {
			indent = s.x
		}
		if s.x != indent {
			t.Errorf("page %d: %q starts at x = %.1f mm, want the item indent %.1f mm", s.page, s.text, s.x, indent)
		}
		pages[s.page] = true
	}
	if len(pages) < 2 {
		t.Fatalf("list drawn on %d page(s), want it to run onto a second page", len(pages))
	}
}
