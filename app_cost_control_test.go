// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strings"
	"testing"
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
