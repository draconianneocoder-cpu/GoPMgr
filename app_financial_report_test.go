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

// The financial report shows a yen project's amounts as whole yen, from the
// same stored hundredths as every other currency (ADR-009).
func TestFinancialReportSnapshotShowsYenAsWholeNumbers(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	mustOpenProject(t, app, "Yen Financial Report")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatal(err)
	}
	project.CurrencyCode = "JPY"
	project.Budget = "5000000"
	if _, err := app.UpdateProjectMeta(project); err != nil {
		t.Fatalf("switch the empty project to yen: %v", err)
	}
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-10-23", Description: "Venue", Amount: "1500000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostReserve(CostReserveWire{Kind: "contingency", Amount: "300000", Description: "Known risk"}); err != nil {
		t.Fatal(err)
	}

	report, err := buildFinancialReportSnapshot(app.requireDB(), time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if report.CurrencyCode != "JPY" || report.Legacy.Budget != "5000000" {
		t.Errorf("report currency %q, legacy budget %q; want JPY and 5000000", report.CurrencyCode, report.Legacy.Budget)
	}
	cc := report.CostControl
	if cc.Planned != "1500000" || cc.Contingency != "300000" || cc.CostBaseline != "1800000" || cc.Actual != "0" {
		t.Errorf("cost control = %+v, want whole yen", cc)
	}
	if len(cc.Entries) != 1 || cc.Entries[0].Amount != "1500000" {
		t.Errorf("entries = %+v", cc.Entries)
	}
	if len(cc.Reserves) != 1 || cc.Reserves[0].Amount != "300000" {
		t.Errorf("reserves = %+v", cc.Reserves)
	}
}

func TestFinancialReportSnapshotCarriesProcurementDetailAndQuantityAggregates(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	mustOpenProject(t, app, "Financial Report Procurement")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Steel beams, batch 1", Amount: "800.00", Quantity: "10.000", Unit: "each", ItemName: "Structural steel beam", SKU: "SKU-BEAM-1", SupplierName: "Acme Structural Supply", InvoiceReference: "INV-1001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-22", Description: "Steel beams, batch 2", Amount: "400.00", Quantity: "5.000", Unit: "each", ItemName: "Structural steel beam", SKU: "SKU-BEAM-1", SupplierName: "Acme Structural Supply", InvoiceReference: "INV-1002"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-23", Description: "Plain cost with no procurement detail", Amount: "50.00"}); err != nil {
		t.Fatal(err)
	}
	report, err := buildFinancialReportSnapshot(app.requireDB(), time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CostControl.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(report.CostControl.Entries))
	}
	var withDetail int
	for _, e := range report.CostControl.Entries {
		if e.ItemName == "Structural steel beam" {
			withDetail++
			if e.SKU != "SKU-BEAM-1" || e.SupplierName != "Acme Structural Supply" {
				t.Fatalf("ledger entry missing procurement detail: %+v", e)
			}
			if e.Quantity != "10.000" && e.Quantity != "5.000" {
				t.Fatalf("ledger entry quantity = %q", e.Quantity)
			}
		}
	}
	if withDetail != 2 {
		t.Fatalf("entries carrying procurement detail = %d, want 2", withDetail)
	}
	if len(report.CostControl.QuantityAggregates) != 1 {
		t.Fatalf("quantity aggregates = %+v, want exactly one item/unit group", report.CostControl.QuantityAggregates)
	}
	agg := report.CostControl.QuantityAggregates[0]
	if agg.ItemName != "Structural steel beam" || agg.Unit != "each" || agg.TotalQuantity != "15.000" || agg.EntryCount != 2 {
		t.Fatalf("quantity aggregate = %+v, want steel beam/each 15.000 x2", agg)
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
