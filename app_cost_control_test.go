// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/money"
)

func TestParseMoneyDecimalRejectsNonCanonicalOrUnsafeInput(t *testing.T) {
	for _, input := range []string{"", "1.234", "1e2", "NaN", " 1.00", "9,000.00", "92233720368547758.08"} {
		if _, err := parseMoneyDecimal(input); err == nil {
			t.Errorf("parseMoneyDecimal(%q) accepted invalid input", input)
		}
	}
	got, err := parseMoneyDecimal("90071992547409.91")
	if err != nil || got.MinorUnits != 9007199254740991 {
		t.Fatalf("large exact amount = %+v, %v", got, err)
	}
	if formatted := formatMoneyDecimal(got, "USD"); formatted != "90071992547409.91" {
		t.Fatalf("formatted = %q", formatted)
	}
}

func TestParseQuantityDecimalRejectsNonCanonicalOrUnsafeInput(t *testing.T) {
	for _, input := range []string{"1.2345", "1e2", "NaN", " 1.000", "9,000.000", "-1.000", "-0.001", "01.000"} {
		if _, err := parseQuantityDecimal(input); err == nil {
			t.Errorf("parseQuantityDecimal(%q) accepted invalid input", input)
		}
	}
	if got, err := parseQuantityDecimal(""); err != nil || got != 0 {
		t.Fatalf("parseQuantityDecimal(\"\") = %d, %v, want 0, nil", got, err)
	}
	got, err := parseQuantityDecimal("2.5")
	if err != nil || got != 2_500 {
		t.Fatalf("parseQuantityDecimal(\"2.5\") = %d, %v, want 2500, nil", got, err)
	}
	got, err = parseQuantityDecimal("1000")
	if err != nil || got != 1_000_000 {
		t.Fatalf("parseQuantityDecimal(\"1000\") = %d, %v, want 1000000, nil", got, err)
	}
	if formatted := formatQuantityDecimal(2_500); formatted != "2.500" {
		t.Fatalf("formatQuantityDecimal(2500) = %q, want 2.500", formatted)
	}
	if formatted := formatQuantityDecimal(0); formatted != "" {
		t.Fatalf("formatQuantityDecimal(0) = %q, want empty (unset)", formatted)
	}
	// Round-trip: every value formatQuantityDecimal can produce must parse
	// back to the exact same milli-units -- no float drift at the boundary.
	for _, milli := range []int64{1, 999, 1_000, 2_500, 1_234_567_000} {
		back, err := parseQuantityDecimal(formatQuantityDecimal(milli))
		if err != nil || back != milli {
			t.Fatalf("round-trip %d -> %q -> %d, %v", milli, formatQuantityDecimal(milli), back, err)
		}
	}
}

func TestCostControlWireUsesSnakeCaseStringMoney(t *testing.T) {
	body, err := json.Marshal(CostSummaryWire{CurrencyCode: "USD", LegacyBudget: "90071992547409.91", CostBaseline: "90071992547409.91"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	if !strings.Contains(encoded, `"currency_code":"USD"`) || !strings.Contains(encoded, `"mutation_disabled_reason":""`) || !strings.Contains(encoded, `"legacy_budget":"90071992547409.91"`) || !strings.Contains(encoded, `"cost_baseline":"90071992547409.91"`) {
		t.Fatalf("wire JSON = %s", encoded)
	}
	if strings.Contains(encoded, "CurrencyCode") || strings.Contains(encoded, "CostBaseline") || strings.Contains(encoded, `"funding":`) || strings.Contains(encoded, `"remaining_funding":`) {
		t.Fatalf("wire JSON exposes Go field names: %s", encoded)
	}
}

// An older JPY project keeps its stored values exactly: amounts are stored
// in hundredths for every currency, so 12,345 still reads as 123.45 yen.
// Whole-yen amounts read without decimals, and the project is now editable
// (ADR-009).
func TestLegacyJPYCostControlKeepsValuesAndAcceptsEdits(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Legacy JPY")
	d := app.requireDB()
	p, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err = d.Conn.Exec(`UPDATE project SET currency_code='JPY' WHERE id=?`, p.ID); err != nil {
		t.Fatalf("seed legacy JPY project: %v", err)
	}
	p, err = d.GetProject()
	if err != nil {
		t.Fatalf("reload legacy JPY project: %v", err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	if _, err = d.SaveCostEntry(db.CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Existing legacy value", AmountMinorUnits: 12_345}); err != nil {
		t.Fatalf("seed legacy entry: %v", err)
	}
	if _, err = d.SaveCostReserve(db.CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 500, Description: "Existing legacy reserve"}); err != nil {
		t.Fatalf("seed legacy reserve: %v", err)
	}

	summary, err := app.ComputeCostSummary()
	if err != nil {
		t.Fatalf("ComputeCostSummary: %v", err)
	}
	if summary.CurrencyCode != "JPY" || summary.Planned != "123.45" || summary.Contingency != "5" || summary.MutationDisabledReason != "" {
		t.Fatalf("legacy JPY summary = %#v", summary)
	}

	beforeEntries, beforeReserves, beforeBaselines, beforeAuditEvents := countCostControlRows(t, d)
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-10-23", Description: "Venue deposit", Amount: "150000"}); err != nil {
		t.Fatalf("SaveCostEntry in yen: %v", err)
	}
	if _, err := app.ApproveCostBaseline("Kick-off"); err != nil {
		t.Fatalf("ApproveCostBaseline: %v", err)
	}
	afterEntries, afterReserves, afterBaselines, afterAuditEvents := countCostControlRows(t, d)
	if afterEntries != beforeEntries+1 || afterReserves != beforeReserves || afterBaselines != beforeBaselines+1 || afterAuditEvents <= beforeAuditEvents {
		t.Fatalf("yen edits wrote rows %d/%d/%d/%d, want one entry and one baseline more than %d/%d/%d/%d, and audit events", afterEntries, afterReserves, afterBaselines, afterAuditEvents, beforeEntries, beforeReserves, beforeBaselines, beforeAuditEvents)
	}
	entries, err := app.ListCostEntries()
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
	amounts := map[string]bool{}
	for _, e := range entries {
		amounts[e.Amount] = true
	}
	if !amounts["123.45"] || !amounts["150000"] {
		t.Fatalf("entry amounts = %v, want the legacy 123.45 and the new 150000", amounts)
	}
	if baselines, err := app.ListCostBaselines(); err != nil || len(baselines) != 1 || baselines[0].CostBaseline != "128.45" {
		t.Fatalf("baselines = %#v, %v", baselines, err)
	}
}

// Reserves read like entries: whole yen for a yen project, two decimals
// otherwise, and an older fractional value exactly as stored (ADR-009).
func TestCostReservesReadInTheProjectCurrency(t *testing.T) {
	for _, c := range []struct{ currency, saved, legacy string }{
		{"JPY", "2000000", "123.45"},
		{"USD", "2000000.00", "123.45"},
	} {
		t.Run(c.currency, func(t *testing.T) {
			app := newEncryptionProjectTestApp(t)
			if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
				t.Fatalf("CreateAccount: %v", err)
			}
			mustOpenProject(t, app, "Reserves "+c.currency)
			d := app.requireDB()
			p, err := d.GetProject()
			if err != nil {
				t.Fatalf("GetProject: %v", err)
			}
			if _, err = d.Conn.Exec(`UPDATE project SET currency_code=? WHERE id=?`, c.currency, p.ID); err != nil {
				t.Fatalf("set currency: %v", err)
			}
			if _, err = d.SaveCostReserve(db.CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 12_345, Description: "Older reserve"}); err != nil {
				t.Fatalf("seed reserve: %v", err)
			}

			saved, err := app.SaveCostReserve(CostReserveWire{Kind: "management", Amount: "2000000", Description: "Management reserve"})
			if err != nil {
				t.Fatalf("SaveCostReserve: %v", err)
			}
			if saved.Amount != c.saved {
				t.Errorf("saved reserve = %q, want %q", saved.Amount, c.saved)
			}

			reserves, err := app.ListCostReserves()
			if err != nil {
				t.Fatalf("ListCostReserves: %v", err)
			}
			got := map[string]string{}
			for _, r := range reserves {
				got[r.Kind] = r.Amount
			}
			if got["management"] != c.saved || got["contingency"] != c.legacy {
				t.Errorf("reserves = %v, want management %q and contingency %q", got, c.saved, c.legacy)
			}
		})
	}
}

func countCostControlRows(t *testing.T, d *db.Database) (entries, reserves, baselines, auditEvents int) {
	t.Helper()
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM cost_entries`).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM cost_reserves`).Scan(&reserves); err != nil {
		t.Fatal(err)
	}
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM cost_baseline_snapshots`).Scan(&baselines); err != nil {
		t.Fatal(err)
	}
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&auditEvents); err != nil {
		t.Fatal(err)
	}
	return entries, reserves, baselines, auditEvents
}

func TestCostClassificationWireUsesSnakeCaseStringMoney(t *testing.T) {
	body, err := json.Marshal(CostClassificationSummaryWire{Attribution: []CostClassificationRowWire{{Value: "direct", Planned: "90071992547409.91", Commitment: "0.00", Actual: "0.00"}}})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	if !strings.Contains(encoded, `"attribution":[{"value":"direct","planned":"90071992547409.91","commitment":"0.00","actual":"0.00"}]`) {
		t.Fatalf("classification wire JSON = %s", encoded)
	}
	if strings.Contains(encoded, "Attribution") || strings.Contains(encoded, "9007199254740991") {
		t.Fatalf("classification wire does not preserve decimal-string transport: %s", encoded)
	}
}

func TestComputeCostSummarySeparatesLegacyBudgetFromCostControl(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Cost summary semantics")
	d := app.requireDB()
	p, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	p.BudgetMinorUnits = 100_000
	if _, err := d.UpsertProject(p); err != nil {
		t.Fatalf("set legacy budget: %v", err)
	}
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	for _, entry := range []CostEntryWire{
		{CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-20", Description: "Base plan", Amount: "800.00"},
		{CostTypeID: types[0].ID, Kind: "commitment", CostDate: "2026-08-20", Description: "Purchase order", Amount: "300.00"},
		{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Invoice", Amount: "200.00"},
	} {
		if _, err := app.SaveCostEntry(entry); err != nil {
			t.Fatalf("SaveCostEntry(%s): %v", entry.Kind, err)
		}
	}
	for _, reserve := range []CostReserveWire{
		{Kind: "contingency", Amount: "100.00", Description: "Known supplier risk"},
		{Kind: "management", Amount: "50.00", Description: "Unknown strategic risk"},
	} {
		if _, err := app.SaveCostReserve(reserve); err != nil {
			t.Fatalf("SaveCostReserve(%s): %v", reserve.Kind, err)
		}
	}

	summary, err := app.ComputeCostSummary()
	if err != nil {
		t.Fatalf("ComputeCostSummary: %v", err)
	}
	if summary != (CostSummaryWire{CurrencyCode: "USD", LegacyBudget: "1000.00", Planned: "800.00", Contingency: "100.00", CostBaseline: "900.00", ManagementReserve: "50.00", AuthorisedFunding: "950.00", Commitment: "300.00", Actual: "200.00"}) {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestSaveCostEntryWiresStructuredFieldsRoundTrip(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Structured wire")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	saved, err := app.SaveCostEntry(CostEntryWire{
		CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Rebar",
		Amount: "500.00", Quantity: "2.500", Unit: "kg", ItemName: "Rebar 10mm", SKU: "RB-10",
		SupplierName: "Acme Steel", InvoiceReference: "INV-1",
	})
	if err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}
	want := CostEntryWire{ID: saved.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Rebar", Amount: "500.00", Quantity: "2.500", Unit: "kg", ItemName: "Rebar 10mm", SKU: "RB-10", SupplierName: "Acme Steel", InvoiceReference: "INV-1"}
	if saved != want {
		t.Fatalf("SaveCostEntry round trip = %#v, want %#v", saved, want)
	}
	listed, err := app.ListCostEntries()
	if err != nil || len(listed) != 1 || listed[0] != want {
		t.Fatalf("ListCostEntries = %#v, %v, want [%#v]", listed, err, want)
	}
}

func TestSearchCostEntriesApp(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Search app")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Server rack", Amount: "10.00", SupplierName: "Acme Hardware"}); err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}
	if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Office chairs", Amount: "20.00"}); err != nil {
		t.Fatalf("SaveCostEntry: %v", err)
	}
	got, err := app.SearchCostEntries("acme")
	if err != nil {
		t.Fatalf("SearchCostEntries: %v", err)
	}
	if len(got) != 1 || got[0].Description != "Server rack" {
		t.Fatalf("SearchCostEntries(acme) = %#v", got)
	}
	all, err := app.SearchCostEntries("")
	if err != nil || len(all) != 2 {
		t.Fatalf("SearchCostEntries(\"\") = %#v, %v, want 2 entries", all, err)
	}
}

func TestAggregateCostEntryQuantitiesApp(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Aggregate app")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	for _, qty := range []string{"2.500", "1.500"} {
		if _, err := app.SaveCostEntry(CostEntryWire{CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Rebar", Amount: "10.00", Quantity: qty, Unit: "kg", ItemName: "Rebar 10mm"}); err != nil {
			t.Fatalf("SaveCostEntry: %v", err)
		}
	}
	got, err := app.AggregateCostEntryQuantities()
	if err != nil {
		t.Fatalf("AggregateCostEntryQuantities: %v", err)
	}
	want := []CostQuantityAggregateWire{{ItemName: "Rebar 10mm", Unit: "kg", TotalQuantity: "4.000", EntryCount: 2}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("AggregateCostEntryQuantities = %#v, want %#v", got, want)
	}
}

func TestComputeCostClassificationSummaryReconcilesIndependentLenses(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Cost classifications")
	types, err := app.ListCostTypes()
	if err != nil {
		t.Fatalf("ListCostTypes: %v", err)
	}
	typeByCode := make(map[string]db.CostType, len(types))
	for _, costType := range types {
		typeByCode[costType.Code] = costType
	}
	for _, entry := range []CostEntryWire{
		{CostTypeID: typeByCode["labor"].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Large direct plan", Amount: "90071992547409.91"},
		{CostTypeID: typeByCode["facilities"].ID, Kind: "commitment", CostDate: "2026-08-21", Description: "Indirect commitment", Amount: "3.00"},
		{CostTypeID: typeByCode["capital"].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Capital actual", Amount: "4.00"},
	} {
		if _, err := app.SaveCostEntry(entry); err != nil {
			t.Fatalf("SaveCostEntry(%q): %v", entry.Description, err)
		}
	}

	got, err := app.ComputeCostClassificationSummary()
	if err != nil {
		t.Fatalf("ComputeCostClassificationSummary: %v", err)
	}
	want := CostClassificationSummaryWire{
		Attribution: []CostClassificationRowWire{
			{Value: "direct", Planned: "90071992547409.91", Commitment: "0.00", Actual: "4.00"},
			{Value: "indirect", Planned: "0.00", Commitment: "3.00", Actual: "0.00"},
		},
		Behavior: []CostClassificationRowWire{
			{Value: "fixed", Planned: "0.00", Commitment: "3.00", Actual: "4.00"},
			{Value: "variable", Planned: "90071992547409.91", Commitment: "0.00", Actual: "0.00"},
		},
		Treatment: []CostClassificationRowWire{
			{Value: "capex", Planned: "0.00", Commitment: "0.00", Actual: "4.00"},
			{Value: "opex", Planned: "90071992547409.91", Commitment: "3.00", Actual: "0.00"},
			{Value: "not_applicable", Planned: "0.00", Commitment: "0.00", Actual: "0.00"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classification = %#v, want %#v", got, want)
	}
	summary, err := app.ComputeCostSummary()
	if err != nil {
		t.Fatalf("ComputeCostSummary: %v", err)
	}
	for _, lens := range [][]CostClassificationRowWire{got.Attribution, got.Behavior, got.Treatment} {
		assertClassificationLensReconciles(t, lens, summary)
	}
}

func assertClassificationLensReconciles(t *testing.T, rows []CostClassificationRowWire, summary CostSummaryWire) {
	t.Helper()
	for _, total := range []struct {
		name string
		want string
		get  func(CostClassificationRowWire) string
	}{
		{name: "planned", want: summary.Planned, get: func(row CostClassificationRowWire) string { return row.Planned }},
		{name: "commitment", want: summary.Commitment, get: func(row CostClassificationRowWire) string { return row.Commitment }},
		{name: "actual", want: summary.Actual, get: func(row CostClassificationRowWire) string { return row.Actual }},
	} {
		var accumulator money.Accumulator
		for _, row := range rows {
			amount, err := parseMoneyDecimal(total.get(row))
			if err != nil {
				t.Fatalf("parse %s classification %q: %v", total.name, row.Value, err)
			}
			accumulator.Add(amount)
		}
		amount, err := accumulator.Amount()
		if err != nil {
			t.Fatalf("%s classification accumulation: %v", total.name, err)
		}
		if got := formatMoneyDecimal(amount, "USD"); got != total.want {
			t.Fatalf("%s classification total = %q, want summary %q", total.name, got, total.want)
		}
	}
}

func TestCostBaselineWireRejectsOverflow(t *testing.T) {
	maxInt64 := int64(^uint64(0) >> 1)
	if _, err := costBaselineWire(db.CostBaselineSnapshot{ID: "overflow", PlannedMinorUnits: maxInt64, ContingencyMinorUnits: 1}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("costBaselineWire overflow error = %v", err)
	}
}

func TestClassifyCostEntriesFailsClosedForInvalidTypeOrOverflow(t *testing.T) {
	t.Run("cost type outside the open project", func(t *testing.T) {
		_, err := classifyCostEntries([]db.CostEntry{{ID: "entry", CostTypeID: "foreign", Kind: "planned", AmountMinorUnits: 1}}, map[string]db.CostType{}, "USD")
		if err == nil || !strings.Contains(err.Error(), "outside the open project") {
			t.Fatalf("classification error = %v", err)
		}
	})

	t.Run("bucket total overflow", func(t *testing.T) {
		maxInt64 := int64(^uint64(0) >> 1)
		costType := db.CostType{ID: "labor", Attribution: "direct", Behavior: "variable", Treatment: "opex"}
		_, err := classifyCostEntries([]db.CostEntry{
			{ID: "first", CostTypeID: costType.ID, Kind: "planned", AmountMinorUnits: maxInt64},
			{ID: "second", CostTypeID: costType.ID, Kind: "planned", AmountMinorUnits: 1},
		}, map[string]db.CostType{costType.ID: costType}, "USD")
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("classification overflow error = %v", err)
		}
	})
}
