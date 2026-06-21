// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"encoding/csv"
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
