// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopmgr/internal/documents"
	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/pdfmeta"
	"gopmgr/internal/sigma/domain"
)

// sigmaCostField describes the solution matrix's Cost column, an amount in
// the project's reporting currency.
var sigmaCostField = documents.Field{Label: "Cost", Money: true}

// GenerateSigmaReportPDF renders the Six Sigma phase deliverables report to
// PDF bytes and a suggested filename, with no filesystem access -- the
// caller chooses the destination (typically via a save dialog) and
// publishes the bytes itself. currency is the project's reporting currency,
// which labels and formats solution costs.
func GenerateSigmaReportPDF(
	project domain.Project,
	currency string,
	charter *domain.Charter,
	sipoc *domain.SIPOCData,
	fishbone *domain.FishboneData,
	solutions []domain.Solution,
	controlPlan []domain.ControlPlanItem,
) (pdfBytes []byte, filename string, err error) {
	if !documents.IsCurrencyCode(currency) {
		return nil, "", fmt.Errorf("sigma report: invalid reporting currency %q", currency)
	}
	pdf := pdfdoc.New("P", "mm", "A4", "")
	registerPDFABaseline(pdf)
	pdf.SetTitle("Six Sigma Project Report: "+project.Title, true)
	pdf.SetAuthor("GoPMgr", true)
	pdf.SetCreator("GoPMgr Sigma Report Generator", true)

	generateReportCover(pdf, project)
	generateCharterSection(pdf, charter)
	generateSIPOCSection(pdf, sipoc)
	generateFishboneSection(pdf, fishbone)
	generateSolutionSection(pdf, solutions, currency)
	generateControlPlanSection(pdf, controlPlan)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", fmt.Errorf("sigma report: %w", err)
	}

	pdfBytes = tagPDFA(buf.Bytes(), pdfmeta.XMPSpec{
		Title:       "Six Sigma Project Report: " + project.Title,
		Author:      "GoPMgr",
		CreatorTool: "GoPMgr Sigma Report Generator",
	})

	filename = fmt.Sprintf("sigma_report_%s_%s.pdf", sanitizeFilename(project.Title), time.Now().UTC().Format("20060102_150405"))
	return pdfBytes, filename, nil
}

func generateReportCover(pdf *pdfdoc.PDF, project domain.Project) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 24)
	pdf.Ln(40)
	pdf.Cell(0, 15, "Six Sigma Project Report")
	pdf.Ln(20)

	pdf.SetFont("Helvetica", "", 16)
	pdf.MultiCell(0, 10, project.Title, "", "L", false)
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", 12)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 8, fmt.Sprintf("Belt Level: %s", project.BeltLevel))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Phase: %s", project.Phase))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Status: %s", project.Status))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Generated: %s", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))
	pdf.Ln(8)

	if project.Description != "" {
		pdf.Ln(10)
		pdf.SetFont("Helvetica", "I", 11)
		pdf.MultiCell(0, 6, project.Description, "", "L", false)
	}
}

func generateCharterSection(pdf *pdfdoc.PDF, charter *domain.Charter) {
	if charter == nil || charter.ProblemStatement == "" {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Project Charter")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 8, "Problem Statement:")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 5, charter.ProblemStatement, "", "L", false)
	pdf.Ln(4)

	if charter.BusinessCase != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 8, "Business Case:")
		pdf.Ln(7)
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 5, charter.BusinessCase, "", "L", false)
		pdf.Ln(4)
	}

	if charter.GoalStatement != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 8, "Goal Statement:")
		pdf.Ln(7)
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 5, charter.GoalStatement, "", "L", false)
		pdf.Ln(4)
	}

	if charter.Sponsor != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.MultiCell(0, 8, fmt.Sprintf("Sponsor: %s", charter.Sponsor), "", "L", false)
		pdf.Ln(2)
	}

	if len(charter.ScopeIn) > 0 {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 8, "In Scope:")
		pdf.Ln(7)
		pdf.SetFont("Helvetica", "", 10)
		for _, item := range charter.ScopeIn {
			bulletItem(pdf, "- ", 5, 5, item)
			pdf.Ln(1)
		}
	}
}

func generateSIPOCSection(pdf *pdfdoc.PDF, sipoc *domain.SIPOCData) {
	if sipoc == nil || len(sipoc.Elements) == 0 {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "SIPOC Diagram")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	if sipoc.ProcessName != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.MultiCell(0, 8, fmt.Sprintf("Process: %s", sipoc.ProcessName), "", "L", false)
	}

	categories := []string{"supplier", "input", "process", "output", "customer"}
	labels := map[string]string{
		"supplier": "Suppliers",
		"input":    "Inputs",
		"process":  "Process Steps",
		"output":   "Outputs",
		"customer": "Customers",
	}

	for _, cat := range categories {
		var items []domain.SIPOCElement
		for _, el := range sipoc.Elements {
			if el.Category == cat {
				items = append(items, el)
			}
		}

		if len(items) == 0 {
			continue
		}

		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetFillColor(240, 240, 240)
		pdf.MultiCell(0, 8, labels[cat], "1", "L", true)

		pdf.SetFont("Helvetica", "", 10)
		for _, item := range items {
			bulletItem(pdf, "- ", 5, 6, item.Description)
			pdf.Ln(1)
		}
		pdf.Ln(2)
	}
}

func generateFishboneSection(pdf *pdfdoc.PDF, fishbone *domain.FishboneData) {
	if fishbone == nil || len(fishbone.Branches) == 0 {
		return
	}

	hasCauses := false
	for _, b := range fishbone.Branches {
		if len(b.Causes) > 0 {
			hasCauses = true
			break
		}
	}
	if !hasCauses {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Fishbone Diagram (Ishikawa)")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	if fishbone.ProblemStatement != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 8, "Problem:")
		pdf.Ln(7)
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 5, fishbone.ProblemStatement, "", "L", false)
		pdf.Ln(4)
	}

	for _, branch := range fishbone.Branches {
		if len(branch.Causes) == 0 {
			continue
		}

		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetFillColor(240, 240, 240)
		pdf.MultiCell(0, 8, branch.Category, "1", "L", true)

		pdf.SetFont("Helvetica", "", 10)
		for _, cause := range branch.Causes {
			bulletItem(pdf, "- ", 5, 6, cause.Description)
			pdf.Ln(1)

			if len(cause.FiveWhys) > 0 {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.SetTextColor(100, 100, 100)
				for i, why := range cause.FiveWhys {
					label := fmt.Sprintf("  Why %d:", i+1)
					bulletItem(pdf, label, max(10, pdf.GetStringWidth(label)+2*pdf.GetCellMargin()), 5, why)
					pdf.Ln(1)
				}
				pdf.SetTextColor(0, 0, 0)
				pdf.SetFont("Helvetica", "", 10)
			}
		}
		pdf.Ln(2)
	}
}

func generateSolutionSection(pdf *pdfdoc.PDF, solutions []domain.Solution, currency string) {
	if len(solutions) == 0 {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Solution Selection Matrix")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	table := sigmaTable{
		headers:  sigmaSolutionHeaders(currency),
		widths:   []float64{45, 20, 20, 20, 30, 25, 20},
		aligns:   []string{"L", "C", "C", "C", "C", "C", "C"},
		fontSize: 9,
		lineH:    6,
	}
	table.drawHeader(pdf)
	for _, sol := range solutions {
		table.drawRow(pdf, sigmaSolutionRow(sol, currency))
	}
}

// sigmaSolutionHeaders names the solution matrix columns; the Cost column
// names the reporting currency, as document amount labels do.
func sigmaSolutionHeaders(currency string) []string {
	return []string{"Solution", "Impact", "Effort", "Risk", sigmaCostField.LabelIn(currency), "Status", "Selected"}
}

// sigmaSolutionRow is one solution's matrix cells. Its cost prints with
// thousands separators in the reporting currency (whole yen for JPY).
func sigmaSolutionRow(sol domain.Solution, currency string) []string {
	selected := ""
	if sol.Selected {
		selected = "YES"
	}
	return []string{
		sol.Title,
		fmt.Sprintf("%d", sol.Impact),
		fmt.Sprintf("%d", sol.Effort),
		fmt.Sprintf("%d", sol.Risk),
		documents.FormatNumberField(sigmaCostField, sol.Cost, currency),
		sol.Status,
		selected,
	}
}

func generateControlPlanSection(pdf *pdfdoc.PDF, controlPlan []domain.ControlPlanItem) {
	if len(controlPlan) == 0 {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Control Plan")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	table := sigmaTable{
		headers:  []string{"Process Step", "Metric", "Spec", "Frequency", "Owner", "Response Plan"},
		widths:   []float64{35, 30, 25, 25, 30, 45},
		aligns:   []string{"L", "L", "L", "L", "L", "L"},
		fontSize: 8,
		lineH:    5,
	}
	table.drawHeader(pdf)
	for _, item := range controlPlan {
		table.drawRow(pdf, []string{
			item.ProcessStep,
			item.Metric,
			item.Specification,
			item.Frequency,
			item.Owner,
			item.ResponsePlan,
		})
	}
}

// sigmaTable is one of the report's ruled tables. Each cell wraps inside
// its own column and a row is as tall as its tallest cell. A row that
// would cross the bottom margin starts a new page under a repeated header.
type sigmaTable struct {
	headers  []string
	widths   []float64 // mm, one per column
	aligns   []string  // "L" or "C", one per column
	fontSize float64   // body text, in points
	lineH    float64   // mm per wrapped line
}

// sigmaTableHeaderH is the header row's height in mm.
const sigmaTableHeaderH = 7

func (t sigmaTable) drawHeader(pdf *pdfdoc.PDF) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range t.headers {
		pdf.CellFormat(t.widths[i], sigmaTableHeaderH, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
}

func (t sigmaTable) drawRow(pdf *pdfdoc.PDF, cells []string) {
	pdf.SetFont("Helvetica", "", t.fontSize)
	_, pageH := pdf.GetPageSize()
	_, bottom := pdf.GetAutoPageBreak()
	left, top, _, _ := pdf.GetMargins()
	// A row never runs taller than a page; a longer cell ends in an ellipsis.
	maxLines := max(1, int((pageH-bottom-top-sigmaTableHeaderH)/t.lineH))

	lines := make([][]string, len(cells))
	n := 1
	for i, c := range cells {
		lines[i] = wrapCellText(pdf, c, t.widths[i])
		if len(lines[i]) > maxLines {
			kept := lines[i][:maxLines]
			kept[maxLines-1] = withEllipsis(pdf, kept[maxLines-1], t.widths[i])
			lines[i] = kept
		}
		n = max(n, len(lines[i]))
	}
	h := float64(n) * t.lineH

	if pdf.GetY()+h > pageH-bottom {
		pdf.AddPage()
		t.drawHeader(pdf)
		pdf.SetFont("Helvetica", "", t.fontSize)
	}
	x, y := left, pdf.GetY()
	for i := range cells {
		pdf.Rect(x, y, t.widths[i], h, "D")
		for j, line := range lines[i] {
			pdf.SetXY(x, y+float64(j)*t.lineH)
			pdf.CellFormat(t.widths[i], t.lineH, line, "", 0, t.aligns[i], false, 0, "")
		}
		x += t.widths[i]
	}
	pdf.SetXY(left, y+h)
}

// wrapCellText splits text into lines that fit a cell of width mm in the
// current font, breaking at spaces and at line breaks. A word wider than
// the cell breaks between characters. It always returns at least one line.
func wrapCellText(pdf *pdfdoc.PDF, text string, width float64) []string {
	avail := width - 2*pdf.GetCellMargin()
	var lines []string
	for _, para := range strings.Split(pdfdoc.SafeText(strings.TrimSpace(text)), "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if pdf.GetStringWidth(candidate) <= avail {
				line = candidate
				continue
			}
			if line != "" {
				lines = append(lines, line)
			}
			line = ""
			for _, r := range word {
				if line != "" && pdf.GetStringWidth(line+string(r)) > avail {
					lines = append(lines, line)
					line = ""
				}
				line += string(r)
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// withEllipsis shortens line until it and a trailing ellipsis fit width mm.
func withEllipsis(pdf *pdfdoc.PDF, line string, width float64) string {
	avail := width - 2*pdf.GetCellMargin()
	runes := []rune(line)
	for len(runes) > 0 && pdf.GetStringWidth(string(runes)+"…") > avail {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRight(string(runes), " ") + "…"
}

// bulletItem prints marker, then text wrapped under itself rather than
// under the marker, so a long item stays inside the margins. MultiCell
// starts each wrapped line at the x it began from, on a new page too, and
// ends back at the left margin.
func bulletItem(pdf *pdfdoc.PDF, marker string, markerW, lineH float64, text string) {
	left, _, _, _ := pdf.GetMargins()
	pdf.SetX(left)
	pdf.Cell(markerW, lineH, marker)
	pdf.MultiCell(0, lineH, text, "", "L", false)
}

func sanitizeFilename(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else {
			result += "_"
		}
	}
	return result
}
