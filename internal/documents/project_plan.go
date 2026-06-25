// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// RenderProjectPlanPDF is the bespoke renderer for the Project Plan
// document — the most comprehensive of the 25 kinds. The plan
// references other docs/charts via chart_ref / doc_id fields, so the
// layout has a dedicated "Linked artifacts" panel where each
// reference is rendered as a labelled chip rather than as a raw ID.
//
// Layout:
//
//	Cover page:
//	  Big project name
//	  "Project Plan"
//	  Executive summary block (if present)
//
//	"Linked artifacts" page:
//	  Schedule chart ref, WBS ref, RACI ref + four doc-id refs
//	  (Scope, Budget, Risks, Communication Plan). Each shown as a
//	  bordered card with the ID and a hint.
//
//	"Narrative sections" pages:
//	  One per entry in narrative_sections[]. Heading at H1, body
//	  text wrapped.
//
//	Footer on every page: PMForge generation timestamp.
//
// Both plan_word and plan_excel kinds dispatch here — the schema is
// identical between the two; the .docx vs .xlsx default export
// difference is handled by exportDocumentAs / RenderDocumentDOCX,
// not by this renderer.
func RenderProjectPlanPDF(content map[string]interface{}, projectName string) ([]byte, error) {
	pdf := newDocPDF("P")
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetTitle("Project Plan", true)

	drawProjectPlanCover(pdf, content, projectName)
	drawProjectPlanLinks(pdf, content)
	drawProjectPlanNarratives(pdf, content)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawProjectPlanCover(pdf *fpdf.Fpdf, content map[string]interface{}, projectName string) {
	pdf.AddPage()

	pdf.SetY(60)
	pdf.SetFont("Helvetica", "B", 26)
	pdf.MultiCell(0, 14, getStringP(content, "project_name", projectName), "", "C", false)

	pdf.SetFont("Helvetica", "", 14)
	pdf.SetTextColor(110, 110, 110)
	pdf.Ln(4)
	pdf.MultiCell(0, 7, "Project Plan", "", "C", false)
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	if summary := getStringP(content, "executive_summary", ""); summary != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(0, 80, 130)
		pdf.SetX(20)
		pdf.Cell(0, 6, "Executive summary")
		pdf.Ln(8)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 5, summary, "", "L", false)
	}

	plannedFooter(pdf)
}

func drawProjectPlanLinks(pdf *fpdf.Fpdf, content map[string]interface{}) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 10, "Linked artifacts")
	pdf.Ln(14)

	// Group: chart references
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(0, 80, 130)
	pdf.Cell(0, 6, "Linked charts")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)

	chartRefs := []struct {
		key, label, hint string
	}{
		{"schedule_ref", "Schedule (CPM)", "chart kind: cpm"},
		{"wbs_ref", "Work Breakdown Structure", "chart kind: wbs"},
		{"raci_ref", "RACI matrix", "chart kind: raci"},
	}
	for _, ref := range chartRefs {
		drawRefCard(pdf, ref.label, getStringP(content, ref.key, ""), ref.hint)
	}

	pdf.Ln(5)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(0, 80, 130)
	pdf.Cell(0, 6, "Linked documents")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)

	docRefs := []struct {
		key, label, hint string
	}{
		{"budget_ref", "Budget", "document kind: budget"},
		{"risks_ref", "Risk Register", "document kind: risk_register"},
		{"communication_plan_ref", "Communication Plan", "document kind: communication_plan"},
	}
	for _, ref := range docRefs {
		drawRefCard(pdf, ref.label, getStringP(content, ref.key, ""), ref.hint)
	}
	plannedFooter(pdf)
}

func drawProjectPlanNarratives(pdf *fpdf.Fpdf, content map[string]interface{}) {
	sections := getObjectSliceP(content, "narrative_sections")
	if len(sections) == 0 {
		return
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 10, "Narrative")
	pdf.Ln(14)

	for _, s := range sections {
		heading := getStringP(s, "heading", "")
		body := getStringP(s, "body", "")
		if heading == "" && body == "" {
			continue
		}
		if heading != "" {
			pdf.SetFont("Helvetica", "B", 13)
			pdf.SetTextColor(0, 80, 130)
			pdf.MultiCell(0, 7, heading, "", "L", false)
			pdf.SetTextColor(0, 0, 0)
		}
		if body != "" {
			pdf.SetFont("Helvetica", "", 10)
			pdf.MultiCell(0, 5, body, "", "L", false)
		}
		pdf.Ln(3)
	}
	plannedFooter(pdf)
}

// drawRefCard renders a bordered card with the referenced artifact's
// label, the ID (or "(not linked)" when blank), and a small hint
// caption. Used in the Linked Artifacts panel.
func drawRefCard(pdf *fpdf.Fpdf, label, id, hint string) {
	x := pdf.GetX()
	y := pdf.GetY()
	const w = 170.0
	const h = 13.0

	if id == "" {
		pdf.SetFillColor(248, 250, 252) // slate-50
		pdf.SetDrawColor(203, 213, 225)
	} else {
		pdf.SetFillColor(236, 254, 255) // cyan-50
		pdf.SetDrawColor(8, 145, 178)
	}
	pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(15, 23, 42)
	pdf.SetXY(x+3, y+1.5)
	pdf.CellFormat(120, 4, label, "", 0, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetXY(x+3, y+6)
	pdf.CellFormat(120, 3.5, hint, "", 0, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 9)
	if id == "" {
		pdf.SetTextColor(180, 100, 60)
		pdf.SetXY(x+w-48, y+4)
		pdf.CellFormat(45, 5, "(not linked)", "", 0, "R", false, 0, "")
	} else {
		pdf.SetTextColor(8, 145, 178)
		pdf.SetXY(x+w-58, y+4)
		pdf.CellFormat(55, 5, id, "", 0, "R", false, 0, "")
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetXY(x, y+h+2)
}

func plannedFooter(pdf *fpdf.Fpdf) {
	pdf.SetY(-15)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 5,
		fmt.Sprintf("Generated by PMForge at %s", time.Now().UTC().Format(time.RFC3339Nano)),
		"", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

// Local helpers — same pattern as other bespoke renderers.

func getStringP(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func getObjectSliceP(m map[string]interface{}, key string) []map[string]interface{} {
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(v))
	for _, x := range v {
		if obj, ok := x.(map[string]interface{}); ok {
			out = append(out, obj)
		}
	}
	return out
}
