// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"

	"pmforge/internal/pdfmeta"
	"pmforge/internal/sigma/domain"
)

// GenerateSigmaReport produces a PDF report of all Six Sigma phase deliverables.
func GenerateSigmaReport(
	project domain.Project,
	charter *domain.Charter,
	sipoc *domain.SIPOCData,
	fishbone *domain.FishboneData,
	solutions []domain.Solution,
	controlPlan []domain.ControlPlanItem,
) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Six Sigma Project Report: "+project.Title, true)
	pdf.SetAuthor("PMForge", true)
	pdf.SetCreator("PMForge Sigma Report Generator", true)

	generateReportCover(pdf, project)
	generateCharterSection(pdf, charter)
	generateSIPOCSection(pdf, sipoc)
	generateFishboneSection(pdf, fishbone)
	generateSolutionSection(pdf, solutions)
	generateControlPlanSection(pdf, controlPlan)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return "", fmt.Errorf("sigma report: %w", err)
	}

	pdfBytes := buf.Bytes()
	if pdfmeta.HasDefaultICC() {
		icc := pdfmeta.DefaultICCProfile()
		spec := pdfmeta.XMPSpec{
			Title:       "Six Sigma Project Report: " + project.Title,
			Author:      "PMForge",
			CreatorTool: "PMForge Sigma Report Generator",
		}
		pdfaBytes, err := pdfmeta.MakePDFA3(pdfBytes, spec, icc)
		if err == nil {
			pdfBytes = pdfaBytes
		}
	}

	outputDir, err := getExportDir()
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("sigma_report_%s_%s.pdf", sanitizeFilename(project.Title), time.Now().UTC().Format("20060102_150405"))
	outputPath := filepath.Join(outputDir, filename)

	if err := os.WriteFile(outputPath, pdfBytes, 0o600); err != nil {
		return "", fmt.Errorf("sigma report write: %w", err)
	}

	return outputPath, nil
}

func generateReportCover(pdf *gofpdf.Fpdf, project domain.Project) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 24)
	pdf.Ln(40)
	pdf.Cell(0, 15, "Six Sigma Project Report")
	pdf.Ln(20)

	pdf.SetFont("Helvetica", "", 16)
	pdf.Cell(0, 10, project.Title)
	pdf.Ln(12)

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

func generateCharterSection(pdf *gofpdf.Fpdf, charter *domain.Charter) {
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
		pdf.Cell(0, 8, fmt.Sprintf("Sponsor: %s", charter.Sponsor))
		pdf.Ln(10)
	}

	if len(charter.ScopeIn) > 0 {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 8, "In Scope:")
		pdf.Ln(7)
		pdf.SetFont("Helvetica", "", 10)
		for _, item := range charter.ScopeIn {
			pdf.Cell(5, 5, "- ")
			pdf.Cell(0, 5, item)
			pdf.Ln(6)
		}
	}
}

func generateSIPOCSection(pdf *gofpdf.Fpdf, sipoc *domain.SIPOCData) {
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
		pdf.Cell(0, 8, fmt.Sprintf("Process: %s", sipoc.ProcessName))
		pdf.Ln(8)
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
		pdf.CellFormat(0, 8, labels[cat], "1", 1, "L", true, 0, "")

		pdf.SetFont("Helvetica", "", 10)
		for _, item := range items {
			pdf.Cell(5, 6, "- ")
			pdf.Cell(0, 6, item.Description)
			pdf.Ln(7)
		}
		pdf.Ln(2)
	}
}

func generateFishboneSection(pdf *gofpdf.Fpdf, fishbone *domain.FishboneData) {
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
		pdf.CellFormat(0, 8, branch.Category, "1", 1, "L", true, 0, "")

		pdf.SetFont("Helvetica", "", 10)
		for _, cause := range branch.Causes {
			pdf.Cell(5, 6, "- ")
			pdf.Cell(0, 6, cause.Description)
			pdf.Ln(7)

			if len(cause.FiveWhys) > 0 {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.SetTextColor(100, 100, 100)
				for i, why := range cause.FiveWhys {
					pdf.Cell(10, 5, fmt.Sprintf("  Why %d:", i+1))
					pdf.Cell(0, 5, why)
					pdf.Ln(6)
				}
				pdf.SetTextColor(0, 0, 0)
				pdf.SetFont("Helvetica", "", 10)
			}
		}
		pdf.Ln(2)
	}
}

func generateSolutionSection(pdf *gofpdf.Fpdf, solutions []domain.Solution) {
	if len(solutions) == 0 {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Solution Selection Matrix")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	headers := []string{"Solution", "Impact", "Effort", "Risk", "Cost", "Status", "Selected"}
	widths := []float64{50, 20, 20, 20, 25, 25, 20}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for _, sol := range solutions {
		selected := ""
		if sol.Selected {
			selected = "YES"
		}
		row := []string{
			sol.Title,
			fmt.Sprintf("%d", sol.Impact),
			fmt.Sprintf("%d", sol.Effort),
			fmt.Sprintf("%d", sol.Risk),
			fmt.Sprintf("$%.2f", sol.Cost),
			sol.Status,
			selected,
		}
		for i, val := range row {
			pdf.CellFormat(widths[i], 6, val, "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
	}
}

func generateControlPlanSection(pdf *gofpdf.Fpdf, controlPlan []domain.ControlPlanItem) {
	if len(controlPlan) == 0 {
		return
	}

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(0, 100, 150)
	pdf.Cell(0, 10, "Control Plan")
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)

	headers := []string{"Process Step", "Metric", "Spec", "Frequency", "Owner", "Response Plan"}
	widths := []float64{35, 30, 25, 25, 30, 45}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 8)
	for _, item := range controlPlan {
		row := []string{
			item.ProcessStep,
			item.Metric,
			item.Specification,
			item.Frequency,
			item.Owner,
			item.ResponsePlan,
		}
		yStart := pdf.GetY()
		maxLines := 1
		for _, val := range row {
			lines := pdf.SplitLines([]byte(val), widths[0]-4)
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		rowHeight := float64(maxLines) * 5

		for i, val := range row {
			pdf.MultiCell(widths[i], 5, val, "1", "L", false)
			pdf.SetXY(pdf.GetX()+widths[i], yStart)
		}
		pdf.SetY(yStart + rowHeight)
	}
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

func getExportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("sigma report: %w", err)
	}
	dir := filepath.Join(home, "PMForge", "exports")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("sigma report mkdir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- this is a private directory mode, not a file mode.
		return "", fmt.Errorf("sigma report chmod export dir: %w", err)
	}
	return dir, nil
}
