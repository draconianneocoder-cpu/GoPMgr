// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/sigma/domain"
)

// GenerateSigmaReportPDF is what the app exports through the save dialog:
// it returns PDF bytes and a suggested name and touches no files.
func TestGenerateSigmaReportPDFReturnsAPDFAndItsName(t *testing.T) {
	pdfBytes, filename, err := GenerateSigmaReportPDF(
		domain.Project{Title: "Line 3 / Scrap", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive},
		"USD", nil, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("report does not start with a PDF header: %q", pdfBytes[:min(len(pdfBytes), 8)])
	}
	if !strings.HasPrefix(filename, "sigma_report_") || !strings.HasSuffix(filename, ".pdf") || strings.ContainsAny(filename, `/\`) {
		t.Fatalf("filename = %q, want sigma_report_<title>_<time>.pdf with no path separators", filename)
	}
}

// The solution matrix's Cost column names and formats the project's
// reporting currency, as documents do; it used to print "$" for every
// project.
func TestSigmaSolutionRowsUseTheReportingCurrency(t *testing.T) {
	cases := []struct {
		currency string
		cost     float64
		header   string
		cell     string
	}{
		{"EUR", 1284500, "Cost (EUR)", "1,284,500.00"},
		{"USD", 1284500, "Cost (USD)", "1,284,500.00"},
		{"JPY", 1284500, "Cost (JPY)", "1,284,500"},
		{"JPY", 1234.5, "Cost (JPY)", "1,234.50"},
		{"GBP", 0, "Cost (GBP)", "0.00"},
	}
	for _, c := range cases {
		headers := sigmaSolutionHeaders(c.currency)
		row := sigmaSolutionRow(domain.Solution{Title: "Fixture", Impact: 8, Effort: 3, Risk: 2, Cost: c.cost, Status: "pilot", Selected: true}, c.currency)
		if len(row) != len(headers) {
			t.Fatalf("%s: %d cells for %d headers", c.currency, len(row), len(headers))
		}
		if headers[4] != c.header || row[4] != c.cell {
			t.Errorf("%s %v: cost column %q / %q, want %q / %q", c.currency, c.cost, headers[4], row[4], c.header, c.cell)
		}
		if strings.Contains(strings.Join(row, " "), "$") {
			t.Errorf("%s: row %q prints a dollar sign", c.currency, row)
		}
		if row[0] != "Fixture" || row[1] != "8" || row[5] != "pilot" || row[6] != "YES" {
			t.Errorf("%s: other cells changed: %q", c.currency, row)
		}
	}
}

var (
	sigmaPDFStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	sigmaPDFTj     = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\)\s*Tj`)
)

// sigmaPDFText returns a report's shown text: each "(...) Tj" string in an
// inflated content stream, decoded from the UTF-16BE fpdf writes for the
// embedded IBM Plex fonts.
func sigmaPDFText(t *testing.T, pdf []byte) string {
	t.Helper()
	var shown []string
	for _, m := range sigmaPDFStream.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // not a compressed content stream
		}
		content, err := io.ReadAll(r)
		if err != nil {
			continue
		}
		for _, tj := range sigmaPDFTj.FindAllSubmatch(content, -1) {
			shown = append(shown, decodePDFString(tj[1]))
		}
	}
	return strings.Join(shown, " ")
}

// decodePDFString decodes a "(...)" string operand: UTF-16BE for the
// embedded IBM Plex fonts, or the bytes as written otherwise.
func decodePDFString(operand []byte) string {
	raw := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`, `\r`, "\r").Replace(string(operand))
	if len(raw)%2 != 0 {
		return raw
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[2*i])<<8 | uint16(raw[2*i+1])
	}
	return string(utf16.Decode(units))
}

func TestGenerateSigmaReportPDF_PrintsCostsInTheReportingCurrency(t *testing.T) {
	project := domain.Project{Title: "Line 3", BeltLevel: domain.BeltGreen, Phase: domain.PhaseImprove, Status: domain.StatusActive}
	solutions := []domain.Solution{{Title: "New fixture", Impact: 8, Effort: 3, Risk: 2, Cost: 1284500, Status: "pilot"}}
	for _, c := range []struct{ currency, header, cell string }{
		{"EUR", "Cost (EUR)", "1,284,500.00"},
		{"JPY", "Cost (JPY)", "1,284,500"},
	} {
		t.Run(c.currency, func(t *testing.T) {
			pdf, _, err := GenerateSigmaReportPDF(project, c.currency, nil, nil, nil, solutions, nil)
			if err != nil {
				t.Fatalf("GenerateSigmaReportPDF: %v", err)
			}
			text := sigmaPDFText(t, pdf)
			if !strings.Contains(text, "Solution Selection Matrix") {
				t.Fatalf("could not read the report's text; got %q", text)
			}
			if !strings.Contains(text, c.header) || !strings.Contains(text, c.cell) {
				t.Errorf("report lacks %q and %q", c.header, c.cell)
			}
			if strings.Contains(text, "$") {
				t.Errorf("a %s report still prints a dollar sign", c.currency)
			}
			if c.currency == "JPY" && strings.Contains(text, "1,284,500.00") {
				t.Errorf("a yen cost printed decimals")
			}
		})
	}
}

func TestGenerateSigmaReportPDF_RejectsAMalformedCurrency(t *testing.T) {
	for _, code := range []string{"", "eur", "EURO", "$"} {
		if _, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3"}, code, nil, nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "reporting currency") {
			t.Errorf("GenerateSigmaReportPDF(currency %q) error = %v, want a reporting currency error", code, err)
		}
	}
}

// Every charter and fishbone section the user filled in prints, and a
// fishbone category with no causes is left out.
func TestGenerateSigmaReportPDF_PrintsEverySection(t *testing.T) {
	pdf, _, err := GenerateSigmaReportPDF(
		domain.Project{Title: "Line 3", Description: "Stamping scrap pilot"}, "USD",
		&domain.Charter{ProblemStatement: "Scrap is 8%.", BusinessCase: "Saves 40 hours a month.", GoalStatement: "Scrap under 3% by Q4."},
		nil,
		&domain.FishboneData{ProblemStatement: "Why is scrap high?", Branches: []domain.FishboneBranch{
			{Category: "Material"},
			{Category: "Machine", Causes: []domain.Cause{{Description: "Worn die"}}},
		}},
		nil, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	text := sigmaPDFText(t, pdf)
	for _, want := range []string{
		"Stamping scrap pilot",
		"Business Case:", "Saves 40 hours a month.",
		"Goal Statement:", "Scrap under 3% by Q4.",
		"Problem:", "Why is scrap high?",
		"Machine", "Worn die",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report is missing %q", want)
		}
	}
	if strings.Contains(text, "Material") {
		t.Error("report prints a fishbone category that has no causes")
	}
}

// A fishbone with categories but no causes gets no page at all.
func TestGenerateSigmaReportPDF_SkipsAnEmptyFishbone(t *testing.T) {
	pdf, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3"}, "USD", nil, nil,
		&domain.FishboneData{ProblemStatement: "Why?", Branches: []domain.FishboneBranch{{Category: "Method"}}}, nil, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	if text := sigmaPDFText(t, pdf); strings.Contains(text, "Fishbone Diagram") {
		t.Error("report draws a fishbone page with no causes")
	}
}

// withEllipsis shortens a full line so the line and its ellipsis still
// fit the cell.
func TestWithEllipsisKeepsTheLineInsideTheCell(t *testing.T) {
	pdf := pdfdoc.New("P", "mm", "A4", "")
	registerPDFABaseline(pdf)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 8)
	const width = 30.0
	avail := width - 2*pdf.GetCellMargin()
	full := wrapCellText(pdf, strings.Repeat("Escalate to the plant manager ", 4), width)[0]
	got := withEllipsis(pdf, full, width)
	if !strings.HasSuffix(got, "…") || pdf.GetStringWidth(got) > avail || len(got) >= len(full)+len("…") {
		t.Errorf("withEllipsis(%q) = %q (%.1f of %.1f mm)", full, got, pdf.GetStringWidth(got), avail)
	}
	if got := withEllipsis(pdf, "", width); got != "…" {
		t.Errorf("withEllipsis(\"\") = %q, want a lone ellipsis", got)
	}
}
