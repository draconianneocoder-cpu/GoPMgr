// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"fmt"
	"strings"
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
// timestampLayout. Use this everywhere a value is needed for JSON/audit
// payloads that are never ORDER BY'd (e.g. an embedded approval-checkpoint
// timestamp) -- never time.Now().UTC().Format(time.RFC3339Nano) directly,
// which reintroduces the ordering hazard described above. For a
// created_at/updated_at TABLE COLUMN, use captureTimestamp() instead: it
// also derives the numeric sort key described below, which is what
// actually makes ordering correct (see timestampCapture's doc comment).
func nowTimestamp() string {
	return captureTimestamp().text
}

// timestampCapture pairs the two on-disk representations of one instant:
// the human-readable, fixed-width TEXT form (unchanged JSON/display
// contract, still parsed by every existing time.Parse(time.RFC3339Nano,
// ...) call site) and a UnixNano INTEGER form that is the actual sort key
// for every ORDER BY on a created_at/updated_at column.
//
// Why a second column instead of trusting the fixed-width TEXT format
// forever: fixed-width formatting is a CONVENTION -- it makes ordering
// correct only as long as every write call site remembers to use it
// instead of reaching for time.Now().Format(time.RFC3339Nano) directly,
// and nothing stops a future call site from doing exactly that. Sorting
// on an INTEGER column is a STRUCTURAL property instead: numeric
// comparison cannot be broken by a string-formatting mistake, no matter
// what the TEXT column ever contains. captureTimestamp() takes one
// time.Now() reading and derives both fields from it, so the pair can
// never be constructed from two different instants -- but the Go type
// system does NOT stop a call site from binding only .text and
// forgetting .unixNano (Go has no way to force both fields of a struct
// to be consumed). The real enforcement is
// TestAllTimestampColumns_TextAndUnixNanoAgree in timestamps_test.go,
// which reads persisted state back with raw SQL (bypassing every Save
// function) and fails if the two ever disagree for any row in any of the
// eight tables below -- treat that test as load-bearing, not incidental.
//
// An INTEGER-only column (dropping the TEXT form entirely) was
// considered and rejected: several structs (documents, scenarios,
// charts, project, stakeholders) deliberately use a Go string field
// rather than time.Time so the Wails JSON bridge can round-trip an
// unsaved record's empty timestamp without a parse error. Keeping TEXT
// as the display/JSON representation and adding UnixNano purely as a
// sort key fixes the actual defect (SQL ORDER BY correctness) with zero
// change to the JSON wire contract or any frontend code.
type timestampCapture struct {
	text     string
	unixNano int64
}

func captureTimestamp() timestampCapture {
	t := time.Now().UTC()
	return timestampCapture{text: t.Format(timestampLayout), unixNano: t.UnixNano()}
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

// unixNanoIndexTargets lists the (index name, table, id column, sort
// column, other index columns) for every index that sorts on a
// created_at/updated_at TEXT column. Each entry's index must be
// repointed at the column's _unixnano sibling: SQLite's numeric
// comparison on that column is what actually makes the ordering
// hazard structurally impossible (see timestampCapture's doc comment)
// -- an index still sorting on the TEXT column would silently defeat
// that guarantee by forcing a full sort instead of failing loudly.
var unixNanoIndexTargets = []struct {
	indexName, table, sortCol string
	otherCols                 []string
}{
	{"idx_scenarios_project", "scenarios", "created_at", []string{"project_id"}},
	{"idx_scenario_charts_scenario", "scenario_charts", "updated_at", []string{"scenario_id"}},
}

// retrofitTimestampUnixNano is the structural half of the ordering fix:
// it adds a `<col>_unixnano INTEGER NOT NULL DEFAULT 0` sibling for
// every column in timestampRetrofitTargets, backfills it from the
// (already TEXT-normalized, by the time this runs) existing value, and
// repoints the two indexes in unixNanoIndexTargets so list queries keep
// using an index for their sort instead of falling back to a full scan.
// Called after retrofitTimestampFormat in migrateLegacyColumns, on every
// database open -- idempotent, matching this package's established
// self-healing migration pattern (see the budget_minor_units backfill
// and retrofitTimestampFormat itself).
//
// audit_events.timestamp_utc is excluded for the same reason as the
// TEXT retrofit: it is never used in an ORDER BY, and it participates in
// eventHash's tamper-evident chain, so no code should ever touch it
// outside of appendAuditEventTx writing a fresh event's own value.
func (db *Database) retrofitTimestampUnixNano() error {
	for _, target := range timestampRetrofitTargets {
		if err := db.addUnixNanoColumn(target.table, target.tsCol); err != nil {
			return fmt.Errorf("add %s.%s_unixnano column: %w", target.table, target.tsCol, err)
		}
	}
	for _, target := range timestampRetrofitTargets {
		if err := db.backfillUnixNanoColumn(target.table, target.idCol, target.tsCol); err != nil {
			return fmt.Errorf("backfill %s.%s_unixnano: %w", target.table, target.tsCol, err)
		}
	}
	for _, idx := range unixNanoIndexTargets {
		if err := db.repointUnixNanoIndex(idx.indexName, idx.table, idx.sortCol, idx.otherCols); err != nil {
			return fmt.Errorf("repoint index %s: %w", idx.indexName, err)
		}
	}
	return nil
}

// addUnixNanoColumn idempotently adds table.<tsCol>_unixnano, matching
// this package's established columnSet-probe-then-ALTER convention (see
// the budget_minor_units and stakeholder money-column migrations in
// sqlite.go). table/tsCol come only from the hardcoded
// timestampRetrofitTargets list, never user input.
func (db *Database) addUnixNanoColumn(table, tsCol string) error {
	cols, err := db.columnSet(table)
	if err != nil {
		return err
	}
	unixCol := tsCol + "_unixnano"
	if _, ok := cols[unixCol]; ok {
		return nil
	}
	ddl := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s INTEGER NOT NULL DEFAULT 0", table, unixCol) // #nosec G201 -- table/unixCol come only from the hardcoded timestampRetrofitTargets list, never user input.
	_, err = db.Conn.Exec(ddl)
	return err
}

// backfillUnixNanoColumn populates table.<tsCol>_unixnano for every row
// where it is still the ADD COLUMN default of 0, by parsing the row's
// existing tsCol value. 0 is an unambiguous "not yet backfilled" sentinel
// here: every created_at/updated_at column is declared TEXT NOT NULL
// DEFAULT (strftime(...)) in the schema and every Go write path binds a
// real, non-empty value (verified across all 8 target tables' INSERT/
// UPDATE statements before this was written) -- there is no code path
// that can persist an empty or NULL value, so 0 (1970-01-01T00:00:00Z)
// can never collide with a genuine "unknown timestamp" case, and every
// row is guaranteed to have a parseable source value to derive from.
// Safe to re-run: already-backfilled rows (unixnano != 0) are excluded
// by the WHERE clause, so a normalized database is a no-op scan.
func (db *Database) backfillUnixNanoColumn(table, idCol, tsCol string) error {
	unixCol := tsCol + "_unixnano"
	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = 0", idCol, tsCol, table, unixCol) // #nosec G201 -- identifiers come only from the hardcoded timestampRetrofitTargets list, never user input.
	rows, err := db.Conn.Query(query)
	if err != nil {
		return err
	}

	type pending struct {
		id       string
		unixNano int64
	}
	var updates []pending
	for rows.Next() {
		var id, val string
		if err := rows.Scan(&id, &val); err != nil {
			_ = rows.Close()
			return err
		}
		t, err := time.Parse(time.RFC3339Nano, val)
		if err != nil {
			continue // leave malformed values at the 0 sentinel untouched
		}
		updates = append(updates, pending{id, t.UnixNano()})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	updateSQL := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", table, unixCol, idCol) // #nosec G201 -- same fixed, hardcoded identifiers as above; both values are parameterized.
	for _, u := range updates {
		if _, err := db.Conn.Exec(updateSQL, u.unixNano, u.id); err != nil {
			return err
		}
	}
	return nil
}

// indexColumns returns indexName's columns in definition order via
// PRAGMA index_info, or nil if the index doesn't exist (SQLite returns
// zero rows for an unknown index name, not an error).
func (db *Database) indexColumns(indexName string) ([]string, error) {
	rows, err := db.Conn.Query("PRAGMA index_info(" + indexName + ")") // #nosec G201 -- indexName comes only from the hardcoded unixNanoIndexTargets list, never user input.
	if err != nil {
		return nil, err
	}
	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cols = append(cols, name.String)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return cols, rows.Close()
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// repointUnixNanoIndex checks whether indexName's on-disk column list is
// still the legacy TEXT-column shape (otherCols + sortCol) instead of
// the repointed shape (otherCols + sortCol_unixnano), and if so drops
// and recreates it. The static schema's CREATE INDEX statement
// intentionally still names the TEXT column (Migrate runs the static
// schema, which creates every table AND index, before this migration
// step ever runs -- referencing a not-yet-added _unixnano column there
// would fail on a brand-new database with "no such column"). So this
// migration step is not a defensive fallback for stale on-disk
// databases; it is THE mechanism, unconditionally, for every database,
// new or old, run every time via the same probe-then-act idiom as
// addUnixNanoColumn -- it runs after addUnixNanoColumn/
// backfillUnixNanoColumn in retrofitTimestampUnixNano, so the target
// column always exists by the time this repoints the index to it.
//
// otherCols is a hand-maintained restatement of unixNanoIndexTargets'
// entry, not derived from the live schema -- so the observed column set
// is validated against it before anything is dropped: if the static
// schema's CREATE INDEX in sqlite.go is ever changed (e.g. a third
// column added) without updating this list, an unrecognized column set
// fails loudly here instead of silently rebuilding a NARROWER index than
// the one actually in use.
//
// DROP + CREATE run in one transaction, not two independent Exec calls:
// a process death between them would otherwise leave an existing
// database with no index at all on next open -- and since an empty
// column set doesn't match either expected shape, this function treats
// "index missing" as "must (re)create," not "nothing to do."
func (db *Database) repointUnixNanoIndex(indexName, table, sortCol string, otherCols []string) error {
	unixCol := sortCol + "_unixnano"
	repointedCols := append(append([]string{}, otherCols...), unixCol)

	observed, err := db.indexColumns(indexName)
	if err != nil {
		return err
	}
	if stringSlicesEqual(observed, repointedCols) {
		return nil // a prior run already repointed it
	}
	if len(observed) > 0 {
		legacyCols := append(append([]string{}, otherCols...), sortCol)
		if !stringSlicesEqual(observed, legacyCols) {
			return fmt.Errorf(
				"index %s has unexpected columns %v (want %v or %v) -- refusing to drop and rebuild an index whose on-disk shape doesn't match unixNanoIndexTargets; update that hardcoded definition first",
				indexName, observed, legacyCols, repointedCols,
			)
		}
	}
	// observed is either empty (index missing -- e.g. a crash left a
	// prior repoint half-done) or exactly the legacy TEXT-column shape.
	// Either way, (re)create it with the correct shape.

	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if len(observed) > 0 {
		if _, err = tx.Exec("DROP INDEX " + indexName); err != nil { // #nosec G201 -- indexName comes only from the hardcoded unixNanoIndexTargets list, never user input.
			return err
		}
	}
	createSQL := fmt.Sprintf("CREATE INDEX %s ON %s(%s)", indexName, table, strings.Join(repointedCols, ", ")) // #nosec G201 -- indexName/table/repointedCols come only from the hardcoded unixNanoIndexTargets list, never user input.
	if _, err = tx.Exec(createSQL); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}
