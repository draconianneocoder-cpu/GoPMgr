// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFinancialCellTextFitsCellWidth(t *testing.T) {
	pdf := newDocPDF("L")
	pdf.SetFont("Helvetica", "", 7)

	const width = 20.0
	got := financialCellText(pdf, width, "Capital equipment & licensing")
	if got == "Capital equipment & licensing" {
		t.Fatal("financialCellText did not truncate an overflowing value")
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("financialCellText suffix = %q, want ellipsis", got)
	}
	if renderedWidth := pdf.GetStringWidth(got); renderedWidth > width-2*pdf.GetCellMargin() {
		t.Errorf("financialCellText width = %v, want <= %v", renderedWidth, width-2*pdf.GetCellMargin())
	}
}

func TestFinancialCellTextPreservesCanonicalInt64Amount(t *testing.T) {
	pdf := newDocPDF("L")
	pdf.SetFont("Helvetica", "", 7)

	const amount = "-92233720368547758.08"
	for _, width := range []float64{
		financialLedgerTableWidths[7],
		financialReserveTableWidths[1],
		financialBaselineTableWidths[1],
		financialBaselineTableWidths[2],
	} {
		got := financialCellText(pdf, width, amount)
		if got != amount {
			t.Errorf("financialCellText(%q, width=%v) = %q, want exact amount", amount, width, got)
		}
	}
}

func TestFinancialTableWidthsAndCellsFitLandscapeContent(t *testing.T) {
	pdf := newDocPDF("L")
	pdf.SetMargins(14, 14, 14)
	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	contentWidth := pageWidth - left - right

	for name, widths := range map[string][]float64{
		"ledger":      financialLedgerTableWidths,
		"reserves":    financialReserveTableWidths,
		"baselines":   financialBaselineTableWidths,
		"procurement": financialProcurementTableWidths,
		"quantity":    financialQuantityTableWidths,
	} {
		var total float64
		for _, width := range widths {
			total += width
		}
		if total > contentWidth {
			t.Errorf("%s table width = %vmm, exceeds printable width %vmm", name, total, contentWidth)
		}
	}

	for _, table := range []struct {
		name    string
		widths  []float64
		headers []string
		row     []string
	}{
		{
			name:    "ledger",
			widths:  financialLedgerTableWidths,
			headers: []string{"Date", "State", "Type", "Attribution", "Behaviour", "Treatment", "Description", "Amount"},
			row:     []string{"2026-08-21", "commitment", "Capital equipment & licensing", "direct", "fixed", "capex", "Long project-provided description", "-92233720368547758.08"},
		},
		{
			name:    "reserves",
			widths:  financialReserveTableWidths,
			headers: []string{"Reserve", "Amount", "Basis / owner note"},
			row:     []string{"management", "-92233720368547758.08", "Long project-provided reserve basis"},
		},
		{
			name:    "baselines",
			widths:  financialBaselineTableWidths,
			headers: []string{"Version", "Cost baseline", "Authorised", "Approved by", "Approved at", "Rationale"},
			row:     []string{"v1", "-92233720368547758.08", "-92233720368547758.08", "local administrator", "2026-08-21T21:00:41.472810000Z", "Long project-provided approval rationale"},
		},
		{
			name:    "procurement",
			widths:  financialProcurementTableWidths,
			headers: []string{"Date", "Item", "SKU", "Supplier", "Invoice ref", "Quantity", "Unit", "Amount"},
			row:     []string{"2026-08-21", "Structural steel beam, hot-rolled", "SKU-STEEL-BEAM-12345", "Acme Structural Supply Co.", "INV-2026-0821-00123", "12345.678", "each", "-92233720368547758.08"},
		},
		{
			name:    "quantity",
			widths:  financialQuantityTableWidths,
			headers: []string{"Item", "Unit", "Total quantity", "Entries"},
			row:     []string{"Structural steel beam, hot-rolled", "each", "12345.678", "42"},
		},
	} {
		for _, font := range []struct {
			style  string
			size   float64
			values []string
		}{
			{style: "B", size: 8, values: table.headers},
			{style: "", size: 7, values: table.row},
		} {
			pdf.SetFont("Helvetica", font.style, font.size)
			for i, value := range font.values {
				got := financialCellText(pdf, table.widths[i], value)
				if renderedWidth := pdf.GetStringWidth(got); renderedWidth > table.widths[i]-2*pdf.GetCellMargin() {
					t.Errorf("%s %q font cell width = %v, want <= %v", table.name, font.style, renderedWidth, table.widths[i]-2*pdf.GetCellMargin())
				}
			}
		}
	}
}

func TestRenderFinancialReportPDFHandlesWideTypeAndFullRangeAmount(t *testing.T) {
	report := FinancialReport{
		ProjectName:  "Financial evidence",
		CurrencyCode: "USD",
		GeneratedAt:  time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		CostControl: FinancialCostControl{Entries: []FinancialLedgerEntry{{
			Date:        "2026-08-21",
			State:       "actual",
			Type:        "Capital equipment & licensing",
			Attribution: "direct",
			Behavior:    "fixed",
			Treatment:   "capex",
			Description: "Full-range exact amount",
			Amount:      "-92233720368547758.08",
		}}},
	}

	pdf, err := RenderFinancialReportPDF(report)
	if err != nil {
		t.Fatalf("RenderFinancialReportPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("RenderFinancialReportPDF did not return a PDF")
	}
}
