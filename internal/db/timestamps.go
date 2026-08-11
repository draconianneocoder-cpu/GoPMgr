// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"fmt"
	"time"
)

// timestampLayout is a fixed-width RFC3339 layout. Unlike time.RFC3339Nano,
// which trims trailing zeros from the fractional-seconds component and
// omits it entirely when exactly zero, this layout's use of '0' (not '9')
// in the reference time's fractional digits forces time.Format to always
// emit exactly nine digits.
//
// Fixed width matters for correctness, not just cosmetics: this package
// stores every created_at/updated_at as a TEXT column and several queries
// do "ORDER BY updated_at DESC" as a plain lexicographic string
// comparison. A trimmed or omitted fraction sorts incorrectly against one
// that IS present in the same second -- e.g. "10:00:00Z" sorts AFTER
// "10:00:00.5Z" even though it is chronologically earlier, since '.'
// (0x2E) sorts before 'Z' (0x5A) in ASCII. This is not limited to the
// whole-second case: ANY two same-second timestamps whose trimmed
// fractional widths differ can invert order the same way (confirmed
// directly with time.Format: "10:00:00.5Z" sorts AFTER
// "10:00:00.50000001Z" despite being chronologically earlier). Fixed
// width closes the hazard for every case, not just the zero-fraction one.
//
// Existing time.Parse(time.RFC3339Nano, ...) call sites need no change:
// the parse layout's '9's are lenient to any digit count, 0 through 9,
// so both this fixed-width format and the legacy trimmed format parse
// identically.
const timestampLayout = "2006-01-02T15:04:05.000000000Z07:00"

// nowTimestamp returns the current UTC time formatted with
// timestampLayout. Use this everywhere a created_at/updated_at/timestamp
// column is populated -- never time.Now().UTC().Format(time.RFC3339Nano)
// directly, which reintroduces the ordering hazard described above.
func nowTimestamp() string {
	return time.Now().UTC().Format(timestampLayout)
}

// fixedWidthLikePattern matches exactly a fixed-width timestampLayout
// value ("2026-08-11T10:00:00.500000000Z", 30 characters, always nine
// fraction digits). Used as a cheap SQL prefilter so retrofitTimestampFormat
// becomes a no-op empty scan once every row has been normalized, rather
// than re-scanning and re-parsing every row on every database open
// indefinitely. SQLite's '_'/'%' LIKE wildcards are plain positional
// wildcards, unaffected by PRAGMA case_sensitive_like (that pragma only
// affects literal ASCII-letter matching), so this is reliable regardless
// of that setting.
const fixedWidthLikePattern = "____-__-__T__:__:__._________Z"

// timestampRetrofitTargets lists every (table, id column, timestamp
// column) whose existing on-disk values need normalizing to
// timestampLayout. Two distinct legacy shapes exist and both are
// normalized here: values written by application code via the old
// trimming time.RFC3339Nano format (variable fraction width, 0-9 digits,
// sometimes omitted entirely), and values written by this schema's SQL
// column DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'), a fixed but
// different 3-digit fraction) -- reachable via a row inserted through
// direct SQL rather than a Save function. Both are subject to the same
// lexicographic ordering hazard against each other and against
// fixed-width values.
//
// audit_events.timestamp_utc is deliberately EXCLUDED, even though it is
// populated the same buggy way: it is one of the fields eventHash hashes
// (see eventHash in audit.go), so rewriting an existing row's value would
// change what VerifyAuditChain recomputes and break the tamper-evident
// hash chain for every database that already has audit history. This
// column is also never used in an ORDER BY, so it was never actually
// subject to the ordering hazard this retrofit exists to fix -- only
// write-side format consistency would be gained, and that is not worth
// breaking the audit chain's actual compliance guarantee. New audit
// events still get the fixed-width format automatically (via
// nowTimestamp()), since each event hashes its own fresh value at write
// time; only retrofitting pre-existing rows is unsafe.
var timestampRetrofitTargets = []struct {
	table, idCol, tsCol string
}{
	{"charts", "id", "created_at"},
	{"charts", "id", "updated_at"},
	{"documents", "id", "created_at"},
	{"documents", "id", "updated_at"},
	{"baselines", "id", "created_at"},
	{"project", "id", "created_at"},
	{"project", "id", "updated_at"},
	{"scenarios", "id", "created_at"},
	{"scenarios", "id", "updated_at"},
	{"scenario_charts", "id", "created_at"},
	{"scenario_charts", "id", "updated_at"},
	{"resource_calendars", "id", "created_at"},
	{"resource_calendars", "id", "updated_at"},
	{"stakeholders", "id", "created_at"},
	{"stakeholders", "id", "updated_at"},
}

// retrofitTimestampFormat normalizes every pre-existing on-disk
// created_at/updated_at value listed in timestampRetrofitTargets to the
// fixed-width timestampLayout. Called on every database open (from
// migrateLegacyColumns), matching this package's existing pattern for
// self-healing legacy data (see the budget_minor_units backfill in the
// same function) -- idempotent and safe to re-run, since the SQL
// prefilter plus the Go-side value comparison mean an already-normalized
// row is never rewritten.
//
// Malformed or empty values are left untouched rather than failing the
// migration: this runs inside Migrate(), and a single bad row must not
// prevent the database from opening at all.
func (db *Database) retrofitTimestampFormat() error {
	for _, target := range timestampRetrofitTargets {
		if err := db.retrofitTimestampColumn(target.table, target.idCol, target.tsCol); err != nil {
			return fmt.Errorf("retrofit %s.%s: %w", target.table, target.tsCol, err)
		}
	}
	return nil
}

// retrofitTimestampColumn normalizes one column. table/idCol/tsCol come
// only from the fixed, hardcoded timestampRetrofitTargets list above,
// never from user input, so building the query with fmt.Sprintf is safe;
// the only runtime value (the LIKE pattern) is passed as a parameter.
func (db *Database) retrofitTimestampColumn(table, idCol, tsCol string) error {
	query := fmt.Sprintf( // #nosec G201 -- identifiers come only from the hardcoded timestampRetrofitTargets list, never user input; the one runtime value is the parameterized LIKE pattern below.
		"SELECT %s, %s FROM %s WHERE %s IS NOT NULL AND %s != '' AND %s NOT LIKE ?",
		idCol, tsCol, table, tsCol, tsCol, tsCol,
	)
	rows, err := db.Conn.Query(query, fixedWidthLikePattern)
	if err != nil {
		return err
	}

	type pending struct{ id, newVal string }
	var updates []pending
	for rows.Next() {
		var id, val string
		if err := rows.Scan(&id, &val); err != nil {
			_ = rows.Close()
			return err
		}
		t, err := time.Parse(time.RFC3339Nano, val)
		if err != nil {
			continue // leave malformed values untouched
		}
		newVal := t.UTC().Format(timestampLayout)
		if newVal != val {
			updates = append(updates, pending{id, newVal})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	updateSQL := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", table, tsCol, idCol) // #nosec G201 -- same fixed, hardcoded identifiers as the query above; both values are parameterized.
	for _, u := range updates {
		if _, err := db.Conn.Exec(updateSQL, u.newVal, u.id); err != nil {
			return err
		}
	}
	return nil
}
