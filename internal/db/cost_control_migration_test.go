// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

const legacyCostControlProjectID = "legacy-cost-control-project"

// newLegacyCostControlFile creates the pre-Cost-Control project shape directly
// through the SQLite driver. In particular, it lacks budget_minor_units and
// currency_code, so InitDB is the only code that can add/backfill them.
func newLegacyCostControlFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy-cost-control.pmforge")
	conn, err := sql.Open(sqlitedriver.Name, path)
	if err != nil {
		t.Fatalf("sql.Open legacy fixture: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = conn.Exec(`
		CREATE TABLE project (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT 'planning',
			phase       TEXT NOT NULL DEFAULT 'initiation',
			start_date  TEXT NOT NULL DEFAULT '',
			end_date    TEXT NOT NULL DEFAULT '',
			budget      NUMERIC NOT NULL DEFAULT 0,
			owner       TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create legacy project table: %v", err)
	}
	if _, err = conn.Exec(`INSERT INTO project (
		id,name,description,status,phase,start_date,end_date,budget,owner,created_at,updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, legacyCostControlProjectID, "Legacy Cost Project", "pre-Cost-Control metadata", "active", "execution", "2026-01-01", "2026-12-31", 123.45, "legacy-owner", "2026-01-02T03:04:05.000000000Z", "2026-01-03T04:05:06.000000000Z"); err != nil {
		t.Fatalf("insert legacy project row: %v", err)
	}
	return path
}

func TestCostControlMigrationPreservesLegacyProjectAndIsIdempotent(t *testing.T) {
	path := newLegacyCostControlFile(t)
	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB legacy project: %v", err)
	}
	assertFreshCostControlMigration(t, d)
	exerciseMigratedCostControl(t, d)
	if err := d.Close(); err != nil {
		t.Fatalf("close after first migration: %v", err)
	}

	for open := 2; open <= 3; open++ {
		d, err = InitDB(path)
		if err != nil {
			t.Fatalf("InitDB repeated open %d: %v", open, err)
		}
		assertPersistedMigratedCostControl(t, d)
		if err := d.Close(); err != nil {
			t.Fatalf("close after repeated open %d: %v", open, err)
		}
	}
}

func TestCostControlMigrationThenEncryptionPreservesLegacyProject(t *testing.T) {
	path := newLegacyCostControlFile(t)
	dek := testDEK(t, 0x5c)
	backupPath, err := MigratePlaintextToEncrypted(path, dek)
	if err != nil {
		t.Fatalf("MigratePlaintextToEncrypted legacy project: %v", err)
	}
	if backupPath != path+".pre-encryption.bak" {
		t.Fatalf("backup path = %q, want %q", backupPath, path+".pre-encryption.bak")
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat plaintext backup: %v", err)
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("plaintext backup mode = %o, want 600", backupInfo.Mode().Perm())
	}
	backupEncrypted, err := IsEncryptedFile(backupPath)
	if err != nil {
		t.Fatalf("inspect plaintext backup header: %v", err)
	}
	if backupEncrypted {
		t.Fatal("plaintext backup has an encrypted SQLite header")
	}
	backup, err := sql.Open(sqlitedriver.Name, backupPath)
	if err != nil {
		t.Fatalf("open migrated plaintext backup: %v", err)
	}
	defer func() { _ = backup.Close() }()
	var backupCurrency string
	var backupMinorUnits int64
	if err = backup.QueryRow(`SELECT currency_code,budget_minor_units FROM project WHERE id=?`, legacyCostControlProjectID).Scan(&backupCurrency, &backupMinorUnits); err != nil {
		t.Fatalf("read migrated plaintext backup project: %v", err)
	}
	if backupCurrency != "USD" || backupMinorUnits != 12_345 {
		t.Fatalf("migrated plaintext backup money/currency = %d/%q, want 12345/USD", backupMinorUnits, backupCurrency)
	}
	var baselineTableCount int
	if err = backup.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cost_baseline_snapshots'`).Scan(&baselineTableCount); err != nil {
		t.Fatalf("find Cost Control table in plaintext backup: %v", err)
	}
	if baselineTableCount != 1 {
		t.Fatalf("Cost Control table count in plaintext backup = %d, want 1", baselineTableCount)
	}
	requireEncryptedHeader(t, path)

	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB migrated legacy project: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	assertFreshCostControlMigration(t, d)
	exerciseMigratedCostControl(t, d)
	requireCipherIntegrity(t, d)
}

func assertFreshCostControlMigration(t *testing.T, d *Database) {
	t.Helper()
	p, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject after migration: %v", err)
	}
	if p.ID != legacyCostControlProjectID || p.Name != "Legacy Cost Project" || p.Description != "pre-Cost-Control metadata" || p.Owner != "legacy-owner" || p.Status != "active" || p.Phase != "execution" || p.StartDate != "2026-01-01" || p.EndDate != "2026-12-31" || p.CreatedAt != "2026-01-02T03:04:05.000000000Z" || p.UpdatedAt != "2026-01-03T04:05:06.000000000Z" {
		t.Fatalf("legacy project metadata = %#v", p)
	}
	if p.BudgetMinorUnits != 12_345 || p.Budget != 123.45 || p.CurrencyCode != "USD" {
		t.Fatalf("legacy money/currency = budget %v minor %d currency %q, want 123.45/12345/USD", p.Budget, p.BudgetMinorUnits, p.CurrencyCode)
	}
	for _, table := range []string{"cost_types", "cost_entries", "cost_reserves", "cost_baseline_snapshots"} {
		var count int
		if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
			t.Fatalf("find %s table: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s table count = %d, want 1", table, count)
		}
		if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("count fresh %s rows: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("fresh %s rows = %d, want 0", table, count)
		}
	}
	for _, index := range []string{"idx_cost_entries_project_date", "idx_cost_baselines_project_version"} {
		var count int
		if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count); err != nil {
			t.Fatalf("find %s index: %v", index, err)
		}
		if count != 1 {
			t.Fatalf("%s index count = %d, want 1", index, count)
		}
	}
	var events int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=?`, p.ID).Scan(&events); err != nil {
		t.Fatalf("count audit events after migration: %v", err)
	}
	if events != 0 {
		t.Fatalf("migration audit events = %d, want 0", events)
	}
	if _, err := d.ApproveCostBaseline(p.ID, "alice", "Legacy Budget must not become a Cost Control baseline"); err == nil || !strings.Contains(err.Error(), "cost baseline must be positive") {
		t.Fatalf("approve baseline from legacy Budget error = %v", err)
	}
	assertCostControlMigrationIntegrity(t, d)
}

func exerciseMigratedCostControl(t *testing.T, d *Database) {
	t.Helper()
	types, err := d.ListCostTypes(legacyCostControlProjectID)
	if err != nil {
		t.Fatalf("ListCostTypes after migration: %v", err)
	}
	if len(types) != 8 {
		t.Fatalf("seeded cost types = %d, want 8", len(types))
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: legacyCostControlProjectID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-20", Description: "Migrated plan", AmountMinorUnits: 10_000}); err != nil {
		t.Fatalf("SaveCostEntry after migration: %v", err)
	}
	if _, err = d.SaveCostReserve(CostReserve{ProjectID: legacyCostControlProjectID, Kind: "contingency", AmountMinorUnits: 1_000, Description: "Migrated known risk"}); err != nil {
		t.Fatalf("SaveCostReserve after migration: %v", err)
	}
	if _, err = d.ApproveCostBaseline(legacyCostControlProjectID, "alice", "Post-migration baseline"); err != nil {
		t.Fatalf("ApproveCostBaseline after migration: %v", err)
	}
	assertPersistedMigratedCostControl(t, d)
}

func assertPersistedMigratedCostControl(t *testing.T, d *Database) {
	t.Helper()
	p, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject persisted migration state: %v", err)
	}
	if p.BudgetMinorUnits != 12_345 || p.CurrencyCode != "USD" {
		t.Fatalf("persisted money/currency = %d/%q, want 12345/USD", p.BudgetMinorUnits, p.CurrencyCode)
	}
	if types, err := d.ListCostTypes(p.ID); err != nil || len(types) != 8 {
		t.Fatalf("persisted cost types = %d, %v, want 8", len(types), err)
	}
	if entries, err := d.ListCostEntries(p.ID); err != nil || len(entries) != 1 || entries[0].AmountMinorUnits != 10_000 {
		t.Fatalf("persisted cost entries = %#v, %v", entries, err)
	}
	if reserves, err := d.ListCostReserves(p.ID); err != nil || len(reserves) != 1 || reserves[0].AmountMinorUnits != 1_000 {
		t.Fatalf("persisted reserves = %#v, %v", reserves, err)
	}
	if baselines, err := d.ListCostBaselines(p.ID); err != nil || len(baselines) != 1 || baselines[0].Version != 1 || baselines[0].PlannedMinorUnits != 10_000 || baselines[0].ContingencyMinorUnits != 1_000 {
		t.Fatalf("persisted baselines = %#v, %v", baselines, err)
	}
	assertCostControlMigrationIntegrity(t, d)
}

func assertCostControlMigrationIntegrity(t *testing.T, d *Database) {
	t.Helper()
	var integrity string
	if err := d.Conn.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q, want ok", integrity)
	}
	rows, err := d.Conn.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check reported a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign_key_check rows: %v", err)
	}
}
