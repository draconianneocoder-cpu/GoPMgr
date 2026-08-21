// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"testing"
	"time"
)

func TestFinancialReportSnapshotSeparatesLegacyBudgetFromCostControl(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	mustOpenProject(t, app, "Financial Report Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatal(err)
	}
	project.Budget = "1000.00"
	if _, err := app.UpdateProjectMeta(project); err != nil {
		t.Fatal(err)
	}
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Project-only cost", Amount: "800.00"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostReserve(CostReserveWire{Kind: "contingency", Amount: "100.00", Description: "Known risk"}); err != nil {
		t.Fatal(err)
	}
	report, err := buildFinancialReportSnapshot(app.requireDB(), time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if report.Legacy.Budget != "1000.00" {
		t.Fatalf("legacy budget = %q", report.Legacy.Budget)
	}
	if report.CostControl.Planned != "800.00" || report.CostControl.CostBaseline != "900.00" {
		t.Fatalf("cost control = %+v", report.CostControl)
	}
	if len(report.CostControl.Entries) != 1 || report.CostControl.Entries[0].Description != "Project-only cost" {
		t.Fatalf("entries = %+v", report.CostControl.Entries)
	}
	path, err := app.ExportFinancialReportPDF()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes) < 5 || string(bytes[:5]) != "%PDF-" {
		t.Fatalf("financial report is not a PDF")
	}
}
