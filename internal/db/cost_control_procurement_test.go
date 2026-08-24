// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

func TestSaveCostEntryStructuredFieldsAreOptionalAndSnapshotted(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Procurement ledger"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Structured fields absent: an ordinary entry remains valid, matching
	// pre-extension behavior exactly.
	plain, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Consulting", AmountMinorUnits: 500_00})
	if err != nil {
		t.Fatalf("SaveCostEntry without structured fields: %v", err)
	}
	if plain.QuantityMilliUnits != 0 || plain.Unit != "" || plain.ItemName != "" || plain.SKU != "" || plain.SupplierName != "" || plain.InvoiceReference != "" {
		t.Fatalf("plain entry structured fields = %#v, want all zero/blank", plain)
	}

	// Structured fields present: stored verbatim (trimmed), and re-reading
	// the row returns the same snapshot -- no live reference is resolved.
	entry, err := d.SaveCostEntry(CostEntry{
		ProjectID: p.ID, CostTypeID: types[1].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Rebar delivery",
		AmountMinorUnits: 12_345, QuantityMilliUnits: 2_500, Unit: " kg ", ItemName: " Rebar 10mm ", SKU: " RB-10-KG ",
		SupplierName: " Acme Steel ", InvoiceReference: " INV-9911 ",
	})
	if err != nil {
		t.Fatalf("SaveCostEntry with structured fields: %v", err)
	}
	if entry.Unit != "kg" || entry.ItemName != "Rebar 10mm" || entry.SKU != "RB-10-KG" || entry.SupplierName != "Acme Steel" || entry.InvoiceReference != "INV-9911" {
		t.Fatalf("structured fields not trimmed as expected: %#v", entry)
	}
	if entry.QuantityMilliUnits != 2_500 {
		t.Fatalf("QuantityMilliUnits = %d, want 2500", entry.QuantityMilliUnits)
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var reread CostEntry
	for _, e := range entries {
		if e.ID == entry.ID {
			reread = e
		}
	}
	if reread.ID == "" {
		t.Fatal("saved entry not found on re-list")
	}
	if reread.ItemName != "Rebar 10mm" || reread.SKU != "RB-10-KG" || reread.SupplierName != "Acme Steel" {
		t.Fatalf("re-read snapshot = %#v, want unchanged from save", reread)
	}
}

func TestSaveCostEntryRejectsInvalidStructuredFields(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Validation"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	base := CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "x", AmountMinorUnits: 100}

	t.Run("quantity without unit", func(t *testing.T) {
		e := base
		e.QuantityMilliUnits = 1_000
		if _, err := d.SaveCostEntry(e); err == nil || !strings.Contains(err.Error(), "quantity requires a unit") {
			t.Fatalf("err = %v, want quantity-requires-unit", err)
		}
	})
	t.Run("negative quantity", func(t *testing.T) {
		e := base
		e.QuantityMilliUnits = -1
		e.Unit = "kg"
		if _, err := d.SaveCostEntry(e); err == nil {
			t.Fatal("expected error for negative quantity")
		}
	})
	t.Run("quantity above bound", func(t *testing.T) {
		e := base
		e.QuantityMilliUnits = maxQuantityMilliUnits + 1
		e.Unit = "kg"
		if _, err := d.SaveCostEntry(e); err == nil {
			t.Fatal("expected error for out-of-range quantity")
		}
	})
	t.Run("field too long", func(t *testing.T) {
		e := base
		e.ItemName = strings.Repeat("x", maxLedgerFieldLength+1)
		if _, err := d.SaveCostEntry(e); err == nil {
			t.Fatal("expected error for oversized item name")
		}
	})
}

func TestSearchCostEntries(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Search"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Office chairs", AmountMinorUnits: 100, ItemName: "Ergonomic chair", SKU: "CHR-100", Unit: "ea", QuantityMilliUnits: 4_000}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-21", Description: "Server rack", AmountMinorUnits: 200, SupplierName: "Acme Hardware", InvoiceReference: "INV-42"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"empty returns all", "", 2},
		{"matches description", "server", 1},
		{"matches item name", "ergonomic", 1},
		{"matches sku", "chr-100", 1},
		{"matches supplier", "acme", 1},
		{"matches invoice reference", "inv-42", 1},
		{"no match", "nonexistent-widget", 0},
		{"escapes LIKE wildcards literally", "%", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.SearchCostEntries(p.ID, tt.query)
			if err != nil {
				t.Fatalf("SearchCostEntries(%q): %v", tt.query, err)
			}
			if len(got) != tt.want {
				t.Fatalf("SearchCostEntries(%q) = %d results, want %d", tt.query, len(got), tt.want)
			}
		})
	}
}

func TestAggregateCostEntryQuantities(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Aggregation"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	rows := []CostEntry{
		{Kind: "actual", CostDate: "2026-08-01", Description: "a", AmountMinorUnits: 100, ItemName: "Rebar 10mm", Unit: "kg", QuantityMilliUnits: 2_500},
		{Kind: "actual", CostDate: "2026-08-02", Description: "b", AmountMinorUnits: 100, ItemName: "Rebar 10mm", Unit: "kg", QuantityMilliUnits: 1_500},
		{Kind: "actual", CostDate: "2026-08-03", Description: "c", AmountMinorUnits: 100, ItemName: "Rebar 10mm", Unit: "ton", QuantityMilliUnits: 1_000}, // different unit: separate bucket
		{Kind: "actual", CostDate: "2026-08-04", Description: "d", AmountMinorUnits: 100},                                                                 // no item/unit: excluded
	}
	for i := range rows {
		rows[i].ProjectID = p.ID
		rows[i].CostTypeID = types[0].ID
		if _, err := d.SaveCostEntry(rows[i]); err != nil {
			t.Fatalf("seed entry %d: %v", i, err)
		}
	}
	got, err := d.AggregateCostEntryQuantities(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("aggregate buckets = %d, want 2: %#v", len(got), got)
	}
	want := map[string]struct {
		total int64
		count int
	}{
		"Rebar 10mm|kg":  {4_000, 2},
		"Rebar 10mm|ton": {1_000, 1},
	}
	for _, a := range got {
		key := a.ItemName + "|" + a.Unit
		w, ok := want[key]
		if !ok {
			t.Fatalf("unexpected aggregate bucket %q", key)
		}
		if a.TotalQuantityMilliUnits != w.total || a.EntryCount != w.count {
			t.Fatalf("bucket %q = %+v, want total=%d count=%d", key, a, w.total, w.count)
		}
	}
}

// newPreProcurementCostEntriesFile creates the shape cost_entries had
// immediately before project-cost-ledger-scope.md item 3 shipped: no
// quantity/unit/item/sku/supplier/invoice columns. This exercises
// migrateLegacyColumns' ALTER TABLE path directly, unlike
// newLegacyCostControlFile (which lacks cost_entries entirely and so only
// ever exercises the fresh CREATE TABLE path).
func newPreProcurementCostEntriesFile(t *testing.T) (path, entryID string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "pre-procurement.gopmgr")
	conn, err := sql.Open(sqlitedriver.Name, path)
	if err != nil {
		t.Fatalf("sql.Open pre-procurement fixture: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = conn.Exec(`
		CREATE TABLE project (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'planning', phase TEXT NOT NULL DEFAULT 'initiation',
			start_date TEXT NOT NULL DEFAULT '', end_date TEXT NOT NULL DEFAULT '',
			budget NUMERIC NOT NULL DEFAULT 0, budget_minor_units INTEGER NOT NULL DEFAULT 0,
			currency_code TEXT NOT NULL DEFAULT 'USD', owner TEXT NOT NULL DEFAULT '',
			industry TEXT NOT NULL DEFAULT '', sub_category TEXT NOT NULL DEFAULT '',
			methodology TEXT NOT NULL DEFAULT '', country_code TEXT NOT NULL DEFAULT 'US',
			time_zone TEXT NOT NULL DEFAULT 'America/New_York',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE cost_types (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, code TEXT NOT NULL, name TEXT NOT NULL,
			attribution TEXT NOT NULL, behavior TEXT NOT NULL, treatment TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 1, UNIQUE(project_id, code)
		);
		CREATE TABLE cost_entries (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, cost_type_id TEXT NOT NULL,
			kind TEXT NOT NULL, amount_minor_units INTEGER NOT NULL, cost_date TEXT NOT NULL,
			description TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE cost_reserves (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL,
			amount_minor_units INTEGER NOT NULL, description TEXT NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(project_id, kind)
		);
		CREATE TABLE cost_baseline_snapshots (
			id TEXT PRIMARY KEY, project_id TEXT NOT NULL, version INTEGER NOT NULL,
			currency_code TEXT NOT NULL, planned_minor_units INTEGER NOT NULL,
			contingency_minor_units INTEGER NOT NULL, management_reserve_minor_units INTEGER NOT NULL,
			approved_by TEXT NOT NULL, approval_note TEXT NOT NULL,
			approved_at TEXT NOT NULL, approved_at_unixnano INTEGER NOT NULL, UNIQUE(project_id, version)
		);
	`); err != nil {
		t.Fatalf("create pre-procurement schema: %v", err)
	}
	const projectID = "pre-procurement-project"
	if _, err = conn.Exec(`INSERT INTO project (id,name,created_at,updated_at) VALUES (?,?,?,?)`,
		projectID, "Pre-Procurement Project", "2026-01-02T03:04:05.000000000Z", "2026-01-02T03:04:05.000000000Z"); err != nil {
		t.Fatalf("insert pre-procurement project row: %v", err)
	}
	const costTypeID = "pre-procurement-type"
	if _, err = conn.Exec(`INSERT INTO cost_types (id,project_id,code,name,attribution,behavior,treatment,active) VALUES (?,?,?,?,?,?,?,1)`,
		costTypeID, projectID, "materials", "Materials & equipment", "direct", "variable", "opex"); err != nil {
		t.Fatalf("insert pre-procurement cost type: %v", err)
	}
	entryID = "pre-procurement-entry"
	if _, err = conn.Exec(`INSERT INTO cost_entries (id,project_id,cost_type_id,kind,amount_minor_units,cost_date,description,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		entryID, projectID, costTypeID, "actual", 5_000, "2026-08-01", "Pre-existing entry", "2026-08-01T00:00:00.000000000Z", "2026-08-01T00:00:00.000000000Z"); err != nil {
		t.Fatalf("insert pre-procurement cost entry: %v", err)
	}
	return path, entryID
}

func TestCostEntriesProcurementColumnMigrationIsIdempotentAndPreservesRows(t *testing.T) {
	path, entryID := newPreProcurementCostEntriesFile(t)

	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB pre-procurement file: %v", err)
	}
	entries, err := d.ListCostEntries("pre-procurement-project")
	if err != nil {
		t.Fatalf("ListCostEntries after migration: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entryID {
		t.Fatalf("entries after migration = %#v", entries)
	}
	e := entries[0]
	if e.AmountMinorUnits != 5_000 || e.Description != "Pre-existing entry" {
		t.Fatalf("pre-existing entry money/description changed by migration: %#v", e)
	}
	if e.QuantityMilliUnits != 0 || e.Unit != "" || e.ItemName != "" || e.SKU != "" || e.SupplierName != "" || e.InvoiceReference != "" {
		t.Fatalf("backfilled procurement columns = %#v, want all zero/blank", e)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close after migration: %v", err)
	}

	// Idempotent: reopening (which re-runs migrateLegacyColumns) must not
	// error on "duplicate column" and must not touch the existing row.
	for open := 2; open <= 3; open++ {
		d, err = InitDB(path)
		if err != nil {
			t.Fatalf("InitDB repeated open %d: %v", open, err)
		}
		entries, err = d.ListCostEntries("pre-procurement-project")
		if err != nil || len(entries) != 1 {
			t.Fatalf("open %d: entries = %#v, %v", open, entries, err)
		}
		if err := d.Close(); err != nil {
			t.Fatalf("close after open %d: %v", open, err)
		}
	}

	// The migrated database must also accept a new structured entry, proving
	// the ALTER'd columns are fully writable, not just present.
	d, err = InitDB(path)
	if err != nil {
		t.Fatalf("InitDB final open: %v", err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.SaveCostEntry(CostEntry{ProjectID: "pre-procurement-project", CostTypeID: entries[0].CostTypeID, Kind: "actual", CostDate: "2026-08-05", Description: "Post-migration structured entry", AmountMinorUnits: 100, ItemName: "Widget", Unit: "ea", QuantityMilliUnits: 3_000}); err != nil {
		t.Fatalf("SaveCostEntry with structured fields after migration: %v", err)
	}
}
