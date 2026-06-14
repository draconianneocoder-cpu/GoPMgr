// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"pmforge/internal/kernel"
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
	m := kernel.ComputeEVM(tasks, 4)
	return ReportPayload{Tasks: tasks, EVM: &m}
}

func TestEVMSummaryLines(t *testing.T) {
	lines := evmSummaryLines(evmPayload(t).EVM)
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
	if evmSummaryLines(nil) != nil {
		t.Error("nil metrics must suppress the section")
	}
	if evmSummaryLines(&kernel.EVMetrics{}) != nil {
		t.Error("zero BAC must suppress the section")
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
