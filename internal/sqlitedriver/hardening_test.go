// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// The engine is built to distrust the schema of every file it opens and to
// refuse the statements that can corrupt one. Both are compile-time
// defaults (third_party/go-sqlcipher/flags.go), so these tests open
// connections the plain way, with nothing set on them.
func openHardeningDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open(Name, filepath.Join(t.TempDir(), "hardening.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSchemaIsUntrustedByDefault(t *testing.T) {
	t.Parallel()
	db := openHardeningDB(t)
	var trusted int
	if err := db.QueryRow(`PRAGMA trusted_schema`).Scan(&trusted); err != nil {
		t.Fatal(err)
	}
	if trusted != 0 {
		t.Fatalf("PRAGMA trusted_schema = %d on a new connection, want 0", trusted)
	}

	// What that buys: a view planted in a file cannot reach a virtual
	// table that is not marked safe for schemas. pragma_database_list,
	// which reports file paths, is one: queried directly it works,
	// through a view it is refused.
	if _, err := db.Exec(`CREATE VIEW planted AS SELECT name FROM pragma_database_list`); err != nil {
		t.Fatalf("create view: %v", err)
	}
	var direct string
	if err := db.QueryRow(`SELECT name FROM pragma_database_list`).Scan(&direct); err != nil || direct != "main" {
		t.Fatalf("direct query = %q, %v; want main", direct, err)
	}
	var viaView string
	err := db.QueryRow(`SELECT name FROM planted`).Scan(&viaView)
	if err == nil || !strings.Contains(err.Error(), "unsafe use of virtual table") {
		t.Fatalf("query through a view = %q, %v; want an unsafe-use refusal", viaView, err)
	}
}

func TestDefensiveModeIsOnByDefault(t *testing.T) {
	t.Parallel()
	db := openHardeningDB(t)
	if _, err := db.Exec(`CREATE TABLE t(a)`); err != nil {
		t.Fatal(err)
	}
	// In defensive mode writable_schema is ignored, so the schema table
	// stays read-only.
	if _, err := db.Exec(`PRAGMA writable_schema = ON`); err != nil {
		t.Fatalf("PRAGMA writable_schema: %v", err)
	}
	_, err := db.Exec(`UPDATE sqlite_master SET sql = 'CREATE TABLE t(a, b)' WHERE name = 't'`)
	if err == nil || !strings.Contains(err.Error(), "may not be modified") {
		t.Fatalf("schema table update = %v, want it refused", err)
	}
	var on int
	if err := db.QueryRow(`PRAGMA writable_schema`).Scan(&on); err != nil || on != 0 {
		t.Fatalf("PRAGMA writable_schema = %d, %v; want 0", on, err)
	}
}
