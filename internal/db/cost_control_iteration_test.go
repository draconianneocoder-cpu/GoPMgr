// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"strings"
	"testing"
)

// failRowDuringRead replaces table with a view of the same rows in which
// reading column fails, with SQLite's "integer overflow", on the rows that
// satisfy failWhen and on no others. A query against the name then starts
// normally and fails while its rows are being read, which is how a damaged
// page or an I/O error appears to database/sql: Next returns false and only
// Rows.Err reports why.
func failRowDuringRead(t *testing.T, d *Database, table, column, failWhen string) {
	t.Helper()
	columns := tableColumns(t, d, table)
	for i, name := range columns {
		if name == column {
			columns[i] = `CASE WHEN abs(-9223372036854775807 - (` + failWhen + `)) THEN ` + name + ` END AS ` + name
		}
	}
	for _, statement := range []string{
		`ALTER TABLE ` + table + ` RENAME TO ` + table + `_stored`,
		`CREATE VIEW ` + table + ` AS SELECT ` + strings.Join(columns, ", ") + ` FROM ` + table + `_stored ORDER BY rowid`,
	} {
		if _, err := d.Conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func tableColumns(t *testing.T, d *Database, table string) []string {
	t.Helper()
	rows, err := d.Conn.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(columns) == 0 {
		t.Fatalf("table %s has no columns", table)
	}
	return columns
}

// A baseline is an approved, permanent record. If reading the plan or the
// reserves stops part-way, approving must fail: a snapshot of the rows read
// so far would understate the baseline and look complete.
func TestApproveCostBaselineRefusesAnInterruptedRead(t *testing.T) {
	for _, tc := range []struct {
		name, table, column, failWhen string
	}{
		{"planned entries", "cost_entries", "amount_minor_units", "description = 'second'"},
		{"reserves", "cost_reserves", "amount_minor_units", "kind = 'management'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newCostControlTestDB(t)
			p, err := d.UpsertProject(Project{Name: "Interrupted baseline"})
			if err != nil {
				t.Fatal(err)
			}
			types, err := d.ListCostTypes(p.ID)
			if err != nil || len(types) == 0 {
				t.Fatalf("cost types = %d, %v", len(types), err)
			}
			for _, description := range []string{"first", "second"} {
				if _, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-11-13", Description: description, AmountMinorUnits: 10_000}); err != nil {
					t.Fatal(err)
				}
			}
			for _, kind := range []string{"contingency", "management"} {
				if _, err := d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: kind, AmountMinorUnits: 1_000, Description: kind}); err != nil {
					t.Fatal(err)
				}
			}
			// Control: the same data approves when every row can be read.
			complete, err := d.ApproveCostBaseline(p.ID, "alice", "complete read")
			if err != nil {
				t.Fatalf("approval with readable rows: %v", err)
			}
			if complete.PlannedMinorUnits != 20_000 || complete.ContingencyMinorUnits != 1_000 || complete.ManagementReserveMinorUnits != 1_000 {
				t.Fatalf("complete snapshot = %+v, want planned 20000 and both reserves 1000", complete)
			}

			failRowDuringRead(t, d, tc.table, tc.column, tc.failWhen)

			got, err := d.ApproveCostBaseline(p.ID, "alice", "interrupted read")
			if err == nil {
				t.Fatalf("approval after an interrupted read succeeded with %+v; a partial sum was recorded as the baseline", got)
			}
			if !strings.Contains(err.Error(), "integer overflow") {
				t.Fatalf("approval error = %v, want the read failure", err)
			}
			baselines, err := d.ListCostBaselines(p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(baselines) != 1 {
				t.Fatalf("baselines after the refused approval = %d, want only the complete one", len(baselines))
			}
		})
	}
}
