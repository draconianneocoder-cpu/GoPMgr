// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"strings"
	"testing"

	"pmforge/internal/kernel"
)

func reportEVMFixture(t *testing.T) *kernel.EVMetrics {
	t.Helper()
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 4,
			BudgetedCost: 400, PercentComplete: 75, ActualCost: 500},
		"B": {ID: "B", Title: "Build", Duration: 4, Precedents: []string{"A"},
			BudgetedCost: 400},
	}
	if !kernel.CalculateCPM(tasks) {
		t.Fatal("cycle in EVM fixture")
	}
	m := kernel.ComputeEVM(tasks, 4)
	return &m
}

func TestEVMSummaryLinesForDocumentReport(t *testing.T) {
	lines := evmSummaryLines(reportEVMFixture(t))
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"Budget at completion (BAC): 800.00",
		"Planned value (PV): 400.00",
		"Earned value (EV): 300.00",
		"Cost performance index (CPI): 0.60",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestBuildCombinedReportAcceptsResolvedEVMForChartRefs(t *testing.T) {
	content := `{"schedule_ref":"chart-1","accomplishments":["Started design"]}`
	_, err := BuildCombinedReport(ReportSpec{
		ReportTitle: "Weekly Pack",
		ProjectName: "Demo",
		ResolvedEVM: map[string]*kernel.EVMetrics{
			"chart-1": reportEVMFixture(t),
		},
	}, []ResolvedSection{{
		Section: ReportSection{DocumentID: "status", Title: "Status"},
		Kind:    KindStatusReport,
		Content: content,
		Version: 1,
		Status:  "draft",
	}})
	if err != nil {
		t.Fatalf("BuildCombinedReport with resolved EVM: %v", err)
	}
}

func TestStatusReportHasScheduleChartRef(t *testing.T) {
	for _, f := range EffectiveFields(KindStatusReport) {
		if f.Key == "schedule_ref" {
			if f.Type != FieldChartRef || f.ChartKind != "cpm" {
				t.Fatalf("schedule_ref = %+v, want CPM chart_ref", f)
			}
			return
		}
	}
	t.Fatal("Status Report must expose schedule_ref so combined reports can resolve EVM")
}
