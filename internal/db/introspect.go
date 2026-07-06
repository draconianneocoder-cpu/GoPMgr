// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "strings"

// DumpSchema returns the project database's SQL schema — the CREATE
// statements for its tables, indexes, triggers, and views — as one string
// with tables first. Internal `sqlite_*` objects are omitted. It is
// read-only and backs the `--schema-dump` CLI flag.
func (db *Database) DumpSchema() (string, error) {
	rows, err := db.Conn.Query(
		`SELECT sql FROM sqlite_master
		 WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		 ORDER BY (type = 'table') DESC, name ASC`,
	)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	for rows.Next() {
		var stmt string
		if err := rows.Scan(&stmt); err != nil {
			return "", err
		}
		b.WriteString(stmt)
		b.WriteString(";\n\n")
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}
