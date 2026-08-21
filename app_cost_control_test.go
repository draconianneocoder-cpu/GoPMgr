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
	if formatted := formatMoneyDecimal(got); formatted != "90071992547409.91" {
		t.Fatalf("formatted = %q", formatted)
	}
}

func TestCostControlWireUsesSnakeCaseStringMoney(t *testing.T) {
	body, err := json.Marshal(CostSummaryWire{CurrencyCode: "USD", LegacyBudget: "90071992547409.91", CostBaseline: "90071992547409.91"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	if !strings.Contains(encoded, `"currency_code":"USD"`) || !strings.Contains(encoded, `"legacy_budget":"90071992547409.91"`) || !strings.Contains(encoded, `"cost_baseline":"90071992547409.91"`) {
		t.Fatalf("wire JSON = %s", encoded)
	}
	if strings.Contains(encoded, "CurrencyCode") || strings.Contains(encoded, "CostBaseline") || strings.Contains(encoded, `"funding":`) || strings.Contains(encoded, `"remaining_funding":`) {
		t.Fatalf("wire JSON exposes Go field names: %s", encoded)
	}
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
		if got := formatMoneyDecimal(amount); got != total.want {
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
		_, err := classifyCostEntries([]db.CostEntry{{ID: "entry", CostTypeID: "foreign", Kind: "planned", AmountMinorUnits: 1}}, map[string]db.CostType{})
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
		}, map[string]db.CostType{costType.ID: costType})
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("classification overflow error = %v", err)
		}
	})
}
