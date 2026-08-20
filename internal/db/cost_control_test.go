// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
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
