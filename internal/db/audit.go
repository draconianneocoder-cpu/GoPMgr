// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"encoding/csv"
	"fmt"
	"os"
	"pmforge/internal/debug"
)

// LogAction writes a row to audit_log using SQLite's strftime() default
// for the timestamp (millisecond-precision UTC). This is the function
// the signature-event and admin workflows call.
func (db *Database) LogAction(actor, action, targetID, details string) error {
	_, err := db.Conn.Exec(
		`INSERT INTO audit_log(actor, action, target_id, details) VALUES (?, ?, ?, ?)`,
		actor, action, targetID, details,
	)
	if err != nil {
		// Re-wrap so callers can attribute audit-log failures via
		// debug.Report(err). Returning a plain error is also acceptable.
		return debug.Wrap(err, "AUDIT_LOG_WRITE_FAILED").ToError()
	}
	return nil
}

// ExportAuditCSV dumps the audit_log table to a CSV file at the given
// path. Used by the `--export-audit` CLI flag.
func (db *Database) ExportAuditCSV(path string) error {
	rows, err := db.Conn.Query(
		`SELECT id, ts, actor, action, target_id, details FROM audit_log ORDER BY id ASC`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"id", "ts", "actor", "action", "target_id", "details"}); err != nil {
		return err
	}

	for rows.Next() {
		var (
			id                                 int64
			ts, actor, action, target, details string
		)
		if err := rows.Scan(&id, &ts, &actor, &action, &target, &details); err != nil {
			return err
		}
		if err := w.Write([]string{fmt.Sprintf("%d", id), ts, actor, action, target, details}); err != nil {
			return err
		}
	}
	return rows.Err()
}
