// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

// OpenEncryptedReadOnly opens an existing SQLCipher project for reading.
// It does not create the file, run Migrate, change the journal mode, or
// chmod the file. mode=ro is set on the URI so the driver's READWRITE|CREATE
// open flags do not apply. _query_only is also set, and is not the only control.
func OpenEncryptedReadOnly(path string, dek []byte) (*Database, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("db: read-only project is not a regular file: %s", path)
	}
	dsn, err := readOnlyEncryptedDSN(path, dek)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := applyStandardPragmas(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Database{Conn: conn, Path: path}, nil
}

func readOnlyEncryptedDSN(path string, dek []byte) (string, error) {
	if _, err := encryptedDSN(path, dek); err != nil {
		return "", err
	}
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return readOnlyFileURI(abs, hexKey)
}

// readOnlyFileURI builds the DSN sqlite3_open_v2 receives unchanged because
// it starts with file:. The path is percent-encoded so a space cannot split
// the URI. '?' and '#' are rejected by encryptedDSN before this runs.
func readOnlyFileURI(abs, hexKey string) (string, error) {
	if strings.ContainsAny(abs, "?#") {
		return "", fmt.Errorf("db: project path contains an illegal character: %q", abs)
	}
	slash := filepath.ToSlash(abs)
	var b strings.Builder
	b.WriteString("file://")
	if !strings.HasPrefix(slash, "/") {
		b.WriteByte('/')
	}
	for i := 0; i < len(slash); i++ {
		c := slash[i]
		if isReadOnlyURIChar(c) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	b.WriteString("?mode=ro&_query_only=1&_foreign_keys=on&_busy_timeout=5000&_pragma_key=x'")
	b.WriteString(hexKey)
	b.WriteByte('\'')
	return b.String(), nil
}

func isReadOnlyURIChar(c byte) bool {
	return c == '/' || c == ':' || c == '.' || c == '-' || c == '_' ||
		(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// CountProjectRecords counts rows for one project. A table this build's
// older files do not have counts as zero. The table name is a fixed
// switch so it is never pasted in from outside the program.
func (db *Database) CountProjectRecords(table, projectID string) (int, error) {
	var query string
	switch table {
	case "charts":
		query = `SELECT COUNT(*) FROM charts WHERE project_id = ?`
	case "documents":
		query = `SELECT COUNT(*) FROM documents WHERE project_id = ?`
	default:
		return 0, fmt.Errorf("db: unsupported project count %q", table)
	}
	ok, err := db.tableExists(table)
	if err != nil || !ok {
		return 0, err
	}
	var n int
	if err := db.Conn.QueryRow(query, projectID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (db *Database) tableExists(name string) (bool, error) {
	var n int
	err := db.Conn.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// HasTableColumns reports whether table exists and has every named column.
// Callers use a false result to skip a query that would fail on an older
// file. They still run the query, and return its error, when the result is
// true. A typo in the column list fails on a current database instead of
// being reported as a missing column.
func (db *Database) HasTableColumns(table string, columns ...string) (bool, error) {
	ok, err := db.tableExists(table)
	if err != nil || !ok {
		return false, err
	}
	have, err := db.columnSet(table)
	if err != nil {
		return false, err
	}
	for _, column := range columns {
		if _, found := have[column]; !found {
			return false, nil
		}
	}
	return true, nil
}
