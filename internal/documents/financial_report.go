// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

var (
	financialLedgerTableWidths      = []float64{20, 17, 45, 22, 20, 19, 84, 42}
	financialReserveTableWidths     = []float64{45, 40, 184}
	financialBaselineTableWidths    = []float64{18, 40, 40, 35, 48, 88}
	financialProcurementTableWidths = []float64{20, 55, 30, 50, 40, 25, 20, 29}
	financialQuantityTableWidths    = []float64{120, 40, 60, 49}
)

// FinancialReport is a project-scoped, printable snapshot. Monetary fields
// are canonical decimal strings supplied by the exact-money application layer.
type FinancialReport struct {
	ProjectName  string
	CurrencyCode string
	GeneratedAt  time.Time
	Legacy       FinancialLegacyBudget
	CostControl  FinancialCostControl
}

type FinancialLegacyBudget struct {
	Budget, ContractValue, LabourEstimate, Committed, Remaining string
}

type FinancialCostControl struct {
	Planned            string
	Contingency        string
	CostBaseline       string
	ManagementReserve  string
	AuthorisedFunding  string
	Commitment         string
	Actual             string
	Entries            []FinancialLedgerEntry
	Reserves           []FinancialReserve
	Baselines          []FinancialBaseline
	QuantityAggregates []FinancialQuantityAggregate
}

type FinancialLedgerEntry struct {
	Date, State, Type, Attribution, Behavior, Treatment, Description, Amount string
	ItemName, SKU, SupplierName, InvoiceReference, Quantity, Unit            string
}

// FinancialQuantityAggregate is a display-ready row from
// [gopmgr/internal/db.CostQuantityAggregate]: the summed quantity of a
// given item/unit pair across every ledger entry that carries it.
type FinancialQuantityAggregate struct {
	ItemName, Unit, TotalQuantity string
	EntryCount                    int
}

type FinancialReserve struct {
	Kind        string
	Amount      string
	Description string
}

type FinancialBaseline struct {
	Version           int64
	Planned           string
	Contingency       string
	CostBaseline      string
	ManagementReserve string
	AuthorisedFunding string
	ApprovedBy        string
	ApprovalNote      string
	ApprovedAt        string
}

// RenderFinancialReportPDF makes the legacy Budget and Cost Control sections
// visibly distinct. It never derives a forecast, allocation, or combined total.
func RenderFinancialReportPDF(report FinancialReport) ([]byte, error) {
	pdf := newDocPDF("L")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 14)
	pdf.SetTitle("Project Financial Report", true)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 10, report.ProjectName)
	pdf.Ln(11)
	pdf.SetFont("Helvetica", "", 9)
	pdf.Cell(0, 5, fmt.Sprintf("Project financial report | Reporting currency: %s | As of: %s", report.CurrencyCode, report.GeneratedAt.UTC().Format(time.RFC3339)))
	pdf.Ln(9)

	financialHeading(pdf, "Legacy Budget context")
	financialNote(pdf, "The legacy Budget rollup is shown for context only. It is not included in Cost Control baseline or authorised funding.")
	financialKeyValues(pdf, report.CurrencyCode, [][2]string{{"Budget cap", report.Legacy.Budget}, {"Contracts", report.Legacy.ContractValue}, {"Labour estimate", report.Legacy.LabourEstimate}, {"Committed", report.Legacy.Committed}, {"Remaining", report.Legacy.Remaining}})

	financialHeading(pdf, "Cost Control")
	financialNote(pdf, "Cost Control is a separate project-local ledger. Reserve balances are assessed governance buffers, not posted costs. No forecast, allocation, drawdown, or remaining-spend calculation is shown.")
	financialKeyValues(pdf, report.CurrencyCode, [][2]string{{"Base plan", report.CostControl.Planned}, {"Contingency reserve", report.CostControl.Contingency}, {"Cost baseline", report.CostControl.CostBaseline}, {"Management reserve", report.CostControl.ManagementReserve}, {"Authorised funding", report.CostControl.AuthorisedFunding}, {"Committed", report.CostControl.Commitment}, {"Actual", report.CostControl.Actual}})

	financialHeading(pdf, "Cost Control ledger entries")
	if len(report.CostControl.Entries) == 0 {
		financialNote(pdf, "No Cost Control ledger entries recorded.")
	} else {
		financialTable(pdf, []string{"Date", "State", "Type", "Attribution", "Behaviour", "Treatment", "Description", "Amount"}, financialLedgerTableWidths, financialLedgerRows(report.CostControl.Entries))
	}
	financialHeading(pdf, "Procurement detail")
	procurementRows := financialProcurementRows(report.CostControl.Entries)
	if len(procurementRows) == 0 {
		financialNote(pdf, "No ledger entries carry procurement detail (item, SKU, supplier, invoice reference, or quantity).")
	} else {
		financialTable(pdf, []string{"Date", "Item", "SKU", "Supplier", "Invoice ref", "Quantity", "Unit", "Amount"}, financialProcurementTableWidths, procurementRows)
	}
	financialHeading(pdf, "Quantity by item & unit")
	if len(report.CostControl.QuantityAggregates) == 0 {
		financialNote(pdf, "No same-item/unit quantity aggregation available.")
	} else {
		financialTable(pdf, []string{"Item", "Unit", "Total quantity", "Entries"}, financialQuantityTableWidths, financialQuantityRows(report.CostControl.QuantityAggregates))
	}
	financialHeading(pdf, "Assessed reserve balances")
	if len(report.CostControl.Reserves) == 0 {
		financialNote(pdf, "No reserve balances recorded.")
	} else {
		financialTable(pdf, []string{"Reserve", "Amount", "Basis / owner note"}, financialReserveTableWidths, financialReserveRows(report.CostControl.Reserves))
	}
	financialHeading(pdf, "Immutable baseline history")
	if len(report.CostControl.Baselines) == 0 {
		financialNote(pdf, "No approved Cost Control baseline snapshots recorded.")
	} else {
		financialTable(pdf, []string{"Version", "Cost baseline", "Authorised", "Approved by", "Approved at", "Rationale"}, financialBaselineTableWidths, financialBaselineRows(report.CostControl.Baselines))
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func financialHeading(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(15, 23, 42)
	pdf.Cell(0, 8, title)
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)
}

func financialNote(pdf *fpdf.Fpdf, note string) {
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(0, 5, note)
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)
}

func financialKeyValues(pdf *fpdf.Fpdf, currency string, values [][2]string) {
	pdf.SetFont("Helvetica", "", 9)
	for _, row := range values {
		pdf.CellFormat(48, 6, row[0], "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, currency+" "+row[1], "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}

func financialTable(pdf *fpdf.Fpdf, headers []string, widths []float64, rows [][]string) {
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(30, 41, 59)
	pdf.SetTextColor(241, 245, 249)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 6, financialCellText(pdf, widths[i], header), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 7)
	for _, row := range rows {
		for i, value := range row {
			pdf.CellFormat(widths[i], 5, financialCellText(pdf, widths[i], value), "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}

// financialCellText fits untrusted project data within the visible bounds of
// an fpdf table cell. CellFormat does not clip overflowing text, so a
// character-count cap alone can let a wide string paint into the next column.
func financialCellText(pdf *fpdf.Fpdf, width float64, value string) string {
	const suffix = "..."
	available := width - 2*pdf.GetCellMargin()
	if available <= 0 {
		return ""
	}
	if pdf.GetStringWidth(value) <= available {
		return value
	}
	suffixWidth := pdf.GetStringWidth(suffix)
	if available <= suffixWidth {
		return ""
	}
	runes := []rune(value)
	low, high := 0, len(runes)
	for low < high {
		middle := low + (high-low+1)/2
		candidate := string(runes[:middle]) + suffix
		if pdf.GetStringWidth(candidate) <= available {
			low = middle
		} else {
			high = middle - 1
		}
	}
	if low == 0 {
		return suffix
	}
	return string(runes[:low]) + suffix
}

func financialLedgerRows(entries []FinancialLedgerEntry) [][]string {
	out := make([][]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, []string{e.Date, e.State, e.Type, e.Attribution, e.Behavior, e.Treatment, e.Description, e.Amount})
	}
	return out
}

// financialProcurementRows lists only the entries that carry procurement
// detail. An entry with no item, SKU, supplier, invoice reference, or
// quantity is a plain financial-only row and is omitted here — it is
// already shown in the Cost Control ledger entries table.
func financialProcurementRows(entries []FinancialLedgerEntry) [][]string {
	out := make([][]string, 0, len(entries))
	for _, e := range entries {
		if e.ItemName == "" && e.SKU == "" && e.SupplierName == "" && e.InvoiceReference == "" && e.Quantity == "" {
			continue
		}
		out = append(out, []string{e.Date, e.ItemName, e.SKU, e.SupplierName, e.InvoiceReference, e.Quantity, e.Unit, e.Amount})
	}
	return out
}

func financialQuantityRows(aggregates []FinancialQuantityAggregate) [][]string {
	out := make([][]string, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, []string{a.ItemName, a.Unit, a.TotalQuantity, fmt.Sprintf("%d", a.EntryCount)})
	}
	return out
}

func financialReserveRows(reserves []FinancialReserve) [][]string {
	out := make([][]string, 0, len(reserves))
	for _, r := range reserves {
		out = append(out, []string{r.Kind, r.Amount, r.Description})
	}
	return out
}

func financialBaselineRows(baselines []FinancialBaseline) [][]string {
	out := make([][]string, 0, len(baselines))
	for _, b := range baselines {
		out = append(out, []string{fmt.Sprintf("v%d", b.Version), b.CostBaseline, b.AuthorisedFunding, b.ApprovedBy, b.ApprovedAt, b.ApprovalNote})
	}
	return out
}
