// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/export"
	"gopmgr/internal/money"
	"gopmgr/internal/rfc3161"
)

func TestResolvedEVMForChartsComputesCPMReferences(t *testing.T) {
	charts := map[string]documents.ResolvedChart{
		"chart-1": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"A","label":"Design","duration":4,"budgeted_cost":400,"actual_cost":500,"percent_complete":75},
				{"id":"B","label":"Build","duration":4,"budgeted_cost":400}
			],"edges":[{"from":"A","to":"B"}]}`,
		},
	}
	proj := db.Project{StartDate: "2026-06-01", CountryCode: "US"}
	asOf := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	resolved, err := resolvedEVMForCharts(proj, charts, asOf)
	if err != nil {
		t.Fatal(err)
	}

	metrics := resolved["chart-1"]
	if metrics == nil {
		t.Fatal("expected EVM metrics for CPM chart reference")
		return
	}
	if metrics.BAC != 800 || metrics.PV != 500 || metrics.EV != 300 || metrics.AC != 500 {
		t.Fatalf("metrics = %+v, want BAC=800 PV=500 EV=300 AC=500", metrics)
	}
}

func TestResolvedEVMForChartsReportsOverflow(t *testing.T) {
	charts := map[string]documents.ResolvedChart{
		"z-last": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"A","duration":1,"budgeted_cost_minor_units":9223372036854775807},
				{"id":"B","duration":1,"budgeted_cost_minor_units":1}
			],"edges":[]}`,
		},
		"a-first": {
			Kind: "cpm",
			Data: `{"nodes":[
				{"id":"C","duration":1,"budgeted_cost_minor_units":9223372036854775807},
				{"id":"D","duration":1,"budgeted_cost_minor_units":1}
			],"edges":[]}`,
		},
	}
	proj := db.Project{StartDate: "2026-06-01", CountryCode: "US"}

	_, err := resolvedEVMForCharts(proj, charts, time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("resolvedEVMForCharts error = %v, want ErrOverflow", err)
	}
	if !strings.Contains(err.Error(), `"a-first"`) {
		t.Fatalf("resolvedEVMForCharts error = %v, want lexical first chart", err)
	}
}

// TestExportCombinedReportSignedWithRuntimePropagatesEVMOverflow proves the
// caller in exportCombinedReportSignedWithRuntime actually checks and
// propagates resolvedEVMForCharts' overflow error rather than silently
// continuing to sign a report with wrong or missing EVM data.
// resolvedEVMForCharts' own overflow return is covered directly above; this
// exercises the thin pass-through wiring one level up, through the full
// App-level export path a real signed-report request takes.
func TestExportCombinedReportSignedWithRuntimePropagatesEVMOverflow(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Overflow Schedule Plan")

	proj, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	// resolvedEVMForCharts only attempts EVM at all once the project is
	// date-anchored (see its own StartDate/DayOffset checks).
	proj.StartDate = "2026-06-01"
	proj.CountryCode = "US"
	if _, err := app.UpdateProjectMeta(proj); err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}

	chart, err := app.SaveChart(db.Chart{
		Kind:  "cpm",
		Title: "Overflow Schedule",
		Data: `{"nodes":[
			{"id":"A","duration":1,"budgeted_cost_minor_units":9223372036854775807},
			{"id":"B","duration":1,"budgeted_cost_minor_units":1}
		],"edges":[]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	// KindProjectSchedule ("schedule") has a required schedule_ref
	// chart_ref field of kind cpm — the only wiring needed for
	// reporting.ChartReferences to surface this chart to
	// resolvedEVMForCharts.
	doc, err := app.NewDocument("schedule", "Overflow Schedule Doc")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	doc.Content = fmt.Sprintf(`{"schedule_ref":%q}`, chart.ID)
	if _, err := app.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	sections := []documents.ReportSection{{DocumentID: doc.ID, Title: doc.Title}}
	generatedAt := time.Now().UTC().Truncate(time.Second)
	_, err = app.exportCombinedReportSignedWithRuntime(
		"Overflow Report",
		"Fixture",
		sections,
		"test-signer.p12",
		"test-password",
		newAppPAdESTestRuntime(t, rfc3161.TrustVerified, generatedAt),
	)
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("exportCombinedReportSignedWithRuntime error = %v, want ErrOverflow", err)
	}
}

// TestExportScheduleReportPropagatesEVMOverflow proves
// exportScheduleReportAs checks and propagates kernel.ComputeEVM's
// overflow error rather than silently writing a schedule report file with
// missing or wrong EVM data.
func TestExportScheduleReportPropagatesEVMOverflow(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Overflow Schedule Report Plan")

	proj, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	proj.StartDate = "2026-06-01"
	proj.CountryCode = "US"
	if _, err := app.UpdateProjectMeta(proj); err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}

	if _, err := app.SaveChart(db.Chart{
		Kind:  "cpm",
		Title: "Overflow Schedule",
		Data: `{"nodes":[
			{"id":"A","duration":1,"budgeted_cost_minor_units":9223372036854775807},
			{"id":"B","duration":1,"budgeted_cost_minor_units":1}
		],"edges":[]}`,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if _, err := app.exportScheduleReportAs(export.FormatCSV); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("exportScheduleReportAs error = %v, want ErrOverflow", err)
	}
}
