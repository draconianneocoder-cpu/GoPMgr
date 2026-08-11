// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"encoding/csv"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExportAuditCSVWritesPrivateCompleteCSV(t *testing.T) {
	d := newBackupTestDB(t)
	if err := d.LogAction("owner", "export", "target-1", "contains, comma\nand newline"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "audit.csv")
	if err := d.ExportAuditCSV(outPath); err != nil {
		t.Fatalf("ExportAuditCSV: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat audit CSV: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("audit CSV mode = %o, want 600", mode)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open audit CSV: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read audit CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[0][0] != "id" || rows[0][5] != "details" {
		t.Fatalf("header = %#v", rows[0])
	}
	if rows[1][1] == "" || rows[1][2] != "owner" || rows[1][3] != "export" || rows[1][4] != "target-1" {
		t.Fatalf("audit row = %#v", rows[1])
	}
	if rows[1][5] != "contains, comma\nand newline" {
		t.Fatalf("details = %q", rows[1][5])
	}
}

// TestExportAuditCSVNeutralizesFormulaInjectionOnEveryColumn proves the
// call site actually applies exportsafe.Cell to all four user-influenced
// columns (actor, action, target_id, details), not just that Cell's own
// escaping logic works (that's covered by
// internal/exportsafe.TestCellNeutralizesFormulaTriggers). This is the
// specific defense the function's own inline comment cites CWE-1236 for;
// dropping the wrapper on any one column would leave a spreadsheet
// formula-injection hole in an audit export a compliance reviewer opens
// in Excel.
func TestExportAuditCSVNeutralizesFormulaInjectionOnEveryColumn(t *testing.T) {
	d := newBackupTestDB(t)
	if err := d.LogAction("=cmd|'/c calc'!A1", "+SUM(A1)", "-1+1", "@SUM(A1)"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "audit.csv")
	if err := d.ExportAuditCSV(outPath); err != nil {
		t.Fatalf("ExportAuditCSV: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open audit CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read audit CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	actor, action, target, details := rows[1][2], rows[1][3], rows[1][4], rows[1][5]
	for name, got := range map[string]string{
		"actor": actor, "action": action, "target_id": target, "details": details,
	} {
		if len(got) == 0 || got[0] != '\'' {
			t.Errorf("%s = %q, want a leading single-quote escape (formula-injection guard did not fire)", name, got)
		}
	}
}

// TestExportAuditCSVOrdersByIDAscending proves multi-row exports preserve
// insertion order (ORDER BY id ASC), not merely that a single row
// round-trips.
func TestExportAuditCSVOrdersByIDAscending(t *testing.T) {
	d := newBackupTestDB(t)
	for _, actor := range []string{"first", "second", "third"} {
		if err := d.LogAction(actor, "export", "target", ""); err != nil {
			t.Fatalf("LogAction(%s): %v", actor, err)
		}
	}

	outPath := filepath.Join(t.TempDir(), "audit.csv")
	if err := d.ExportAuditCSV(outPath); err != nil {
		t.Fatalf("ExportAuditCSV: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open audit CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read audit CSV: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("row count = %d, want 4 (header + 3)", len(rows))
	}
	wantOrder := []string{"first", "second", "third"}
	for i, want := range wantOrder {
		if got := rows[i+1][2]; got != want {
			t.Errorf("row %d actor = %q, want %q (full order: %v)", i, got, want,
				[]string{rows[1][2], rows[2][2], rows[3][2]})
		}
	}
}

// TestExportAuditCSVWritesHeaderOnlyForEmptyLog proves the empty-input
// boundary doesn't error or omit the header a downstream CSV parser
// expects.
func TestExportAuditCSVWritesHeaderOnlyForEmptyLog(t *testing.T) {
	d := newBackupTestDB(t)

	outPath := filepath.Join(t.TempDir(), "audit.csv")
	if err := d.ExportAuditCSV(outPath); err != nil {
		t.Fatalf("ExportAuditCSV: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open audit CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read audit CSV: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1 (header only)", len(rows))
	}
	if rows[0][0] != "id" || rows[0][5] != "details" {
		t.Fatalf("header = %#v", rows[0])
	}
}

// TestExportAuditCSVFailsCleanlyWhenDestinationCannotBeOpened covers the
// os.OpenFile failure path: the query against audit_log has already
// succeeded and opened a *sql.Rows cursor by the time OpenFile is
// attempted, so this also proves that cursor is not leaked on the
// early-return error path (via db.Conn.Stats().InUse, confirmed by a
// throwaway probe to actually discriminate a leaked cursor -- InUse
// went 0 -> 1 while a cursor was held open and unclosed, then back to 0
// after Close -- not assumed from reading database/sql's docs).
func TestExportAuditCSVFailsCleanlyWhenDestinationCannotBeOpened(t *testing.T) {
	d := newBackupTestDB(t)
	if err := d.LogAction("owner", "export", "target-1", "irrelevant"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	before := d.Conn.Stats().InUse
	outPath := filepath.Join(t.TempDir(), "no-such-subdir", "audit.csv")

	if err := d.ExportAuditCSV(outPath); err == nil {
		t.Fatal("ExportAuditCSV into a nonexistent directory = nil error, want a failure")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ExportAuditCSV error = %v, want fs.ErrNotExist", err)
	}

	if _, statErr := os.Stat(outPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("stat(%s) = %v, want the file to not exist", outPath, statErr)
	}

	after := d.Conn.Stats().InUse
	if after != before {
		t.Errorf("Conn.Stats().InUse = %d after a failed export, want %d (the audit_log query cursor must be closed, not leaked, on the OpenFile error path)", after, before)
	}
}
