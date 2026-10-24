// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"gopmgr/internal/kernel"
)

func evmPayload(t *testing.T) ReportPayload {
	t.Helper()
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 4,
			BudgetedCost: 400, PercentComplete: 75, ActualCost: 500},
		"B": {ID: "B", Title: "Build", Duration: 4, Precedents: []string{"A"},
			BudgetedCost: 400},
	}
	if !kernel.CalculateCPM(tasks) {
		t.Fatal("cycle in fixture")
	}
	m, err := kernel.ComputeEVM(tasks, 4)
	if err != nil {
		t.Fatal(err)
	}
	return ReportPayload{Tasks: tasks, EVM: &m, Currency: "USD"}
}

func TestEVMSummaryLines(t *testing.T) {
	lines, err := evmSummaryLines(evmPayload(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 11 {
		t.Fatalf("lines = %d, want 11", len(lines))
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"BAC): 800.00",
		"PV): 400.00",
		"EV): 300.00",
		"AC): 500.00",
		"SV): -100.00",
		"CV): -200.00",
		"SPI): 0.75",
		"CPI): 0.60",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestEVMSummaryLinesSuppressed(t *testing.T) {
	// A suppressed section needs no currency.
	if lines, err := evmSummaryLines(ReportPayload{}); lines != nil || err != nil {
		t.Errorf("nil metrics: lines %q, error %v; want the section suppressed", lines, err)
	}
	if lines, err := evmSummaryLines(ReportPayload{EVM: &kernel.EVMetrics{}}); lines != nil || err != nil {
		t.Errorf("zero BAC: lines %q, error %v; want the section suppressed", lines, err)
	}
}

func TestScheduleReportsCarryEVMSection(t *testing.T) {
	payload := evmPayload(t)

	// ODT content.xml is plain text inside the zip; assert on the body
	// builder directly.
	body, err := renderODTReportBody(payload, "Demo")
	if err != nil {
		t.Fatalf("renderODTReportBody: %v", err)
	}
	if !strings.Contains(body, "Earned Value") || !strings.Contains(body, "BAC): 800.00") {
		t.Errorf("ODT body missing EVM section:\n%s", body)
	}

	// PDF and DOCX are binary; smoke-test that rendering succeeds with
	// the section enabled.
	for _, format := range []ExportFormat{FormatPDF, FormatDOCX} {
		if _, err := GenerateArchivalReport(payload, ExportOptions{Format: format, Title: "Demo"}); err != nil {
			t.Errorf("GenerateArchivalReport(%s) with EVM: %v", format, err)
		}
	}
}

func TestScheduleReportsWithoutEVMUnchanged(t *testing.T) {
	payload := evmPayload(t)
	payload.EVM = nil

	body, err := renderODTReportBody(payload, "Demo")
	if err != nil {
		t.Fatalf("renderODTReportBody: %v", err)
	}
	if strings.Contains(body, "Earned Value") {
		t.Error("EVM section must be absent when metrics are nil")
	}
}

// largeEVMPayload is a schedule whose amounts need thousands separators:
// BAC 1,284,500, PV and EV 642,250, AC 700,000, CV -57,750.
func largeEVMPayload(t *testing.T, currency string) ReportPayload {
	t.Helper()
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 4,
			BudgetedCost: 642250, PercentComplete: 100, ActualCost: 700000},
		"B": {ID: "B", Title: "Build", Duration: 4, Precedents: []string{"A"},
			BudgetedCost: 642250},
	}
	if !kernel.CalculateCPM(tasks) {
		t.Fatal("cycle in fixture")
	}
	m, err := kernel.ComputeEVM(tasks, 4)
	if err != nil {
		t.Fatal(err)
	}
	return ReportPayload{Tasks: tasks, EVM: &m, Currency: currency}
}

// scheduleReportText renders a schedule report and returns its text: the
// decoded PDF strings, DOCX document.xml, the ODT body, or the HTML.
func scheduleReportText(t *testing.T, payload ReportPayload, format ExportFormat) string {
	t.Helper()
	if format == FormatODT {
		body, err := renderODTReportBody(payload, "Demo")
		if err != nil {
			t.Fatalf("renderODTReportBody: %v", err)
		}
		return body
	}
	raw, err := GenerateArchivalReport(payload, ExportOptions{Format: format, Title: "Demo"})
	if err != nil {
		t.Fatalf("GenerateArchivalReport(%s): %v", format, err)
	}
	switch format {
	case FormatPDF:
		return sigmaPDFText(t, raw)
	case FormatDOCX:
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatalf("open DOCX: %v", err)
		}
		for _, f := range zr.File {
			if f.Name != "word/document.xml" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open document.xml: %v", err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read document.xml: %v", err)
			}
			return string(b)
		}
		t.Fatal("DOCX has no word/document.xml")
	}
	return string(raw)
}

// Schedule reports print earned value in the project's reporting
// currency, as combined reports do: thousands separators, and whole yen.
// They used to print "%.2f" for every currency.
func TestScheduleReportsPrintEarnedValueInTheReportingCurrency(t *testing.T) {
	cases := []struct {
		currency string
		want     []string
		notWant  []string
	}{
		{"JPY", []string{
			"Budget at completion (BAC): 1,284,500",
			"Planned value (PV): 642,250",
			"Actual cost (AC): 700,000",
			"Cost variance (CV): -57,750",
		}, []string{"1,284,500.00", "700,000.00"}},
		{"EUR", []string{
			"Budget at completion (BAC): 1,284,500.00",
			"Actual cost (AC): 700,000.00",
			"Cost variance (CV): -57,750.00",
		}, nil},
	}
	for _, c := range cases {
		for _, format := range []ExportFormat{FormatPDF, FormatDOCX, FormatODT, FormatHTML} {
			t.Run(c.currency+"/"+string(format), func(t *testing.T) {
				text := scheduleReportText(t, largeEVMPayload(t, c.currency), format)
				for _, w := range c.want {
					if !strings.Contains(text, w) {
						t.Errorf("missing %q", w)
					}
				}
				for _, w := range append(c.notWant, "1284500") {
					if strings.Contains(text, w) {
						t.Errorf("prints %q", w)
					}
				}
			})
		}
	}
}

// A malformed currency fails the export when the earned-value section
// prints, as documents refuse one; formats without that section, and a
// schedule with no cost data, need no currency.
func TestScheduleReportsRefuseAMalformedCurrencyForEarnedValue(t *testing.T) {
	for _, currency := range []string{"", "eur", "EURO"} {
		for _, format := range []ExportFormat{FormatPDF, FormatDOCX, FormatODT, FormatHTML} {
			_, err := GenerateArchivalReport(largeEVMPayload(t, currency), ExportOptions{Format: format, Title: "Demo"})
			if err == nil || !strings.Contains(err.Error(), "reporting currency") {
				t.Errorf("%s with currency %q: error = %v, want a reporting-currency error", format, currency, err)
			}
		}
		if _, err := GenerateArchivalReport(largeEVMPayload(t, currency), ExportOptions{Format: FormatCSV, Title: "Demo"}); err != nil {
			t.Errorf("CSV with currency %q: %v; CSV prints no earned value", currency, err)
		}
		noCost := largeEVMPayload(t, currency)
		noCost.EVM = &kernel.EVMetrics{}
		if _, err := GenerateArchivalReport(noCost, ExportOptions{Format: FormatPDF, Title: "Demo"}); err != nil {
			t.Errorf("PDF with no cost data and currency %q: %v", currency, err)
		}
	}
}
