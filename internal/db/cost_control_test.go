// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func newCostControlTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := InitDB(filepath.Join(t.TempDir(), "cost-control.gopmgr"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestCostControlSeedsAndAuditsProjectScopedEntry(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Cost ledger"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) < 8 {
		t.Fatalf("seed types = %d, want at least 8", len(types))
	}
	entry, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Security review", AmountMinorUnits: 125_50})
	if err != nil {
		t.Fatal(err)
	}
	if entry.AmountMinorUnits != 125_50 {
		t.Fatalf("amount = %d", entry.AmountMinorUnits)
	}
	p.CurrencyCode = "EUR"
	if _, err := d.UpsertProject(p); err == nil {
		t.Fatal("currency changed after a cost entry")
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
	verification, err := d.VerifyAuditChain(p.ID)
	if err != nil || !verification.Valid {
		t.Fatalf("audit = %#v, %v", verification, err)
	}
}

func TestProjectReportingCurrencyPolicy(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Currency policy"})
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrencyCode != "USD" {
		t.Fatalf("default currency = %q, want USD", p.CurrencyCode)
	}
	p.CurrencyCode = " eur "
	p, err = d.UpsertProject(p)
	if err != nil {
		t.Fatalf("change empty project currency: %v", err)
	}
	if p.CurrencyCode != "EUR" {
		t.Fatalf("normalised currency = %q, want EUR", p.CurrencyCode)
	}
	p.CurrencyCode = "ZZZ"
	if _, err := d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "unsupported project reporting currency") {
		t.Fatalf("unsupported currency error = %v", err)
	}

	p.CurrencyCode = "EUR"
	p.BudgetMinorUnits = 1
	p, err = d.UpsertProject(p)
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}
	p.CurrencyCode = "USD"
	if _, err := d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "cannot change while a budget") {
		t.Fatalf("currency change after budget error = %v", err)
	}
}

func TestCostEntryRejectsArchivedOrZeroAmount(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Validation"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`UPDATE cost_types SET active=0 WHERE id=?`, types[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-20", Description: "x", AmountMinorUnits: 1}); err == nil {
		t.Fatal("archived type accepted")
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[1].ID, Kind: "planned", CostDate: "2026-08-20", Description: "x"}); err == nil {
		t.Fatal("zero amount accepted")
	}
}

func TestCostReserveUpsertPreservesIdentityAndAudits(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Reserves"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 10_000, Description: "Known risk"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 12_500, Description: "Reassessed"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("reserve ID changed: %q -> %q", first.ID, second.ID)
	}
	reserves, err := d.ListCostReserves(p.ID)
	if err != nil || len(reserves) != 1 || reserves[0].AmountMinorUnits != 12_500 {
		t.Fatalf("reserves = %#v, %v", reserves, err)
	}
	verified, err := d.VerifyAuditChain(p.ID)
	if err != nil || !verified.Valid {
		t.Fatalf("audit = %#v, %v", verified, err)
	}
}
