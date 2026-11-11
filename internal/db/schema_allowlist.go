// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// A project file from outside this account (a restored backup) defines its
// own tables. Migrate creates tables with CREATE TABLE IF NOT EXISTS, so a
// table the file already defines is kept as the file wrote it: with a
// cascading foreign key, a CHECK or UNIQUE constraint, a generated column or
// a computed default that GoPMgr never asked for. This file compares such a
// schema with the one GoPMgr itself creates and lists every difference that
// could change what a later write does.
//
// The reference is not a hand-kept list. It is read from a database this
// build creates, so it cannot drift from the schema in sqlite.go and
// wbs_store.go. A file made by an older GoPMgr has fewer columns and tables
// than the reference, never different ones, with the two exceptions in
// legacyForeignKeys and the index rule below; so the test is "nothing in
// the file is outside the reference", not equality.

// tableShape is what decides how a table behaves on write.
type tableShape struct {
	withoutRowID, strict bool
	columns              map[string]columnShape
	foreignKeys          map[string]bool // "from>table.to on_update on_delete"
	checks               map[string]bool // CHECK expressions, whitespace removed
}

type columnShape struct {
	notNull    bool
	primaryKey int
	hidden     int // 0 ordinary; 2 and 3 are generated columns
	defaultSQL sql.NullString
}

type indexShape struct {
	table   string
	unique  bool
	partial bool
	columns []string // "" for an expression
	sql     string
}

type schemaShape struct {
	tables  map[string]tableShape
	indexes map[string]indexShape
}

// legacyForeignKeys are foreign keys an older GoPMgr created and Migrate
// still knows how to replace. Before migrateSigmaProjectsSchema,
// sigma_projects.id referenced the project row.
var legacyForeignKeys = map[string]map[string]bool{
	"sigma_projects": {"id>project.id NO ACTION CASCADE": true},
}

// literalDefault matches a column default that is a constant: NULL, a
// number or a quoted string. A constant default cannot compute anything.
var literalDefault = regexp.MustCompile(`(?i)^\s*(null|true|false|[-+]?[0-9]+(\.[0-9]+)?|'([^']|'')*')\s*$`)

type schemaQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// readSchemaShape describes every table and index in the database through
// SQLite's own pragmas, so it sees what SQLite will enforce and not what
// the stored SQL text appears to say.
func readSchemaShape(q schemaQueryer) (schemaShape, error) {
	shape := schemaShape{tables: map[string]tableShape{}, indexes: map[string]indexShape{}}

	type tableRow struct {
		name, sql            string
		withoutRowID, strict bool
	}
	var tables []tableRow
	rows, err := q.Query(`SELECT l.name, l.wr, l.strict, COALESCE(m.sql, '')
		FROM pragma_table_list AS l LEFT JOIN sqlite_master AS m ON m.type = 'table' AND m.name = l.name
		WHERE l.schema = 'main' AND l.type = 'table' AND l.name NOT LIKE 'sqlite\_%' ESCAPE '\'`)
	if err != nil {
		return shape, fmt.Errorf("list tables: %w", err)
	}
	for rows.Next() {
		var t tableRow
		if err := rows.Scan(&t.name, &t.withoutRowID, &t.strict, &t.sql); err != nil {
			_ = rows.Close()
			return shape, fmt.Errorf("list tables: %w", err)
		}
		tables = append(tables, t)
	}
	if err := closeRows(rows); err != nil {
		return shape, fmt.Errorf("list tables: %w", err)
	}

	for _, t := range tables {
		table := tableShape{
			withoutRowID: t.withoutRowID, strict: t.strict,
			columns: map[string]columnShape{}, foreignKeys: map[string]bool{}, checks: map[string]bool{},
		}
		for _, check := range checkClauses(t.sql) {
			table.checks[check] = true
		}

		rows, err := q.Query(`SELECT name, "notnull", pk, hidden, dflt_value FROM pragma_table_xinfo(?)`, t.name)
		if err != nil {
			return shape, fmt.Errorf("columns of %q: %w", t.name, err)
		}
		for rows.Next() {
			var name string
			var c columnShape
			if err := rows.Scan(&name, &c.notNull, &c.primaryKey, &c.hidden, &c.defaultSQL); err != nil {
				_ = rows.Close()
				return shape, fmt.Errorf("columns of %q: %w", t.name, err)
			}
			table.columns[name] = c
		}
		if err := closeRows(rows); err != nil {
			return shape, fmt.Errorf("columns of %q: %w", t.name, err)
		}

		rows, err = q.Query(`SELECT "from", "table", COALESCE("to", ''), on_update, on_delete FROM pragma_foreign_key_list(?)`, t.name)
		if err != nil {
			return shape, fmt.Errorf("foreign keys of %q: %w", t.name, err)
		}
		for rows.Next() {
			var from, target, to, onUpdate, onDelete string
			if err := rows.Scan(&from, &target, &to, &onUpdate, &onDelete); err != nil {
				_ = rows.Close()
				return shape, fmt.Errorf("foreign keys of %q: %w", t.name, err)
			}
			table.foreignKeys[fmt.Sprintf("%s>%s.%s %s %s", from, target, to, onUpdate, onDelete)] = true
		}
		if err := closeRows(rows); err != nil {
			return shape, fmt.Errorf("foreign keys of %q: %w", t.name, err)
		}
		shape.tables[t.name] = table

		type indexRow struct {
			name            string
			unique, partial bool
		}
		var indexes []indexRow
		rows, err = q.Query(`SELECT name, "unique", partial FROM pragma_index_list(?)`, t.name)
		if err != nil {
			return shape, fmt.Errorf("indexes of %q: %w", t.name, err)
		}
		for rows.Next() {
			var i indexRow
			if err := rows.Scan(&i.name, &i.unique, &i.partial); err != nil {
				_ = rows.Close()
				return shape, fmt.Errorf("indexes of %q: %w", t.name, err)
			}
			indexes = append(indexes, i)
		}
		if err := closeRows(rows); err != nil {
			return shape, fmt.Errorf("indexes of %q: %w", t.name, err)
		}
		for _, i := range indexes {
			index := indexShape{table: t.name, unique: i.unique, partial: i.partial}
			rows, err := q.Query(`SELECT COALESCE(name, '') FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, i.name)
			if err != nil {
				return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
			}
			for rows.Next() {
				var column string
				if err := rows.Scan(&column); err != nil {
					_ = rows.Close()
					return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
				}
				index.columns = append(index.columns, column)
			}
			if err := closeRows(rows); err != nil {
				return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
			}
			rows, err = q.Query(`SELECT COALESCE(sql, '') FROM sqlite_master WHERE type = 'index' AND name = ?`, i.name)
			if err != nil {
				return shape, fmt.Errorf("definition of index %q: %w", i.name, err)
			}
			for rows.Next() {
				if err := rows.Scan(&index.sql); err != nil {
					_ = rows.Close()
					return shape, fmt.Errorf("definition of index %q: %w", i.name, err)
				}
			}
			if err := closeRows(rows); err != nil {
				return shape, fmt.Errorf("definition of index %q: %w", i.name, err)
			}
			shape.indexes[i.name] = index
		}
	}
	return shape, nil
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	return err
}

// checkClauses returns the expression of every CHECK constraint in a
// CREATE TABLE statement, with whitespace outside quotes removed so that
// layout does not matter. Quoted text and comments are skipped, so the word
// in a string or a column name is not mistaken for a constraint.
func checkClauses(createSQL string) []string {
	var out []string
	s := createSQL
	isWord := func(b byte) bool {
		return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
	}
	// skipQuoted returns the index just past the quoted run starting at i.
	skipQuoted := func(i int) int {
		closer := s[i]
		if closer == '[' {
			closer = ']'
		}
		for j := i + 1; j < len(s); j++ {
			if s[j] != closer {
				continue
			}
			if closer != ']' && j+1 < len(s) && s[j+1] == closer {
				j++ // doubled quote
				continue
			}
			return j + 1
		}
		return len(s)
	}
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\'' || c == '"' || c == '`' || c == '[':
			i = skipQuoted(i)
		case c == '-' && strings.HasPrefix(s[i:], "--"):
			if end := strings.IndexByte(s[i:], '\n'); end >= 0 {
				i += end + 1
			} else {
				i = len(s)
			}
		case c == '/' && strings.HasPrefix(s[i:], "/*"):
			if end := strings.Index(s[i+2:], "*/"); end >= 0 {
				i += end + 4
			} else {
				i = len(s)
			}
		case isWord(c):
			start := i
			for i < len(s) && isWord(s[i]) {
				i++
			}
			if !strings.EqualFold(s[start:i], "check") {
				continue
			}
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j >= len(s) || s[j] != '(' {
				continue
			}
			var expr strings.Builder
			depth := 0
			for ; j < len(s); j++ {
				switch s[j] {
				case '\'', '"', '`', '[':
					end := skipQuoted(j)
					expr.WriteString(s[j:end])
					j = end - 1
					continue
				case '(':
					depth++
				case ')':
					depth--
				case ' ', '\t', '\n', '\r':
					continue
				}
				expr.WriteByte(s[j])
				if depth == 0 {
					break
				}
			}
			out = append(out, expr.String())
			i = j + 1
		default:
			i++
		}
	}
	return out
}

var (
	referenceSchemaOnce sync.Once
	referenceSchemaVal  schemaShape
	referenceSchemaErr  error
)

// referenceSchema is the shape of a project file this build creates,
// including the WBS tables that are added on first use.
func referenceSchema() (schemaShape, error) {
	referenceSchemaOnce.Do(func() {
		referenceSchemaVal, referenceSchemaErr = buildReferenceSchema()
	})
	return referenceSchemaVal, referenceSchemaErr
}

func buildReferenceSchema() (shape schemaShape, err error) {
	dir, err := os.MkdirTemp("", "gopmgr-schema-reference-")
	if err != nil {
		return shape, fmt.Errorf("reference schema: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(dir); err == nil && removeErr != nil {
			err = fmt.Errorf("reference schema: %w", removeErr)
		}
	}()
	reference, err := InitDB(filepath.Join(dir, "reference.gopmgr"))
	if err != nil {
		return shape, fmt.Errorf("reference schema: %w", err)
	}
	defer func() {
		if closeErr := reference.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("reference schema: %w", closeErr)
		}
	}()
	if _, err := reference.Conn.Exec(wbsStorageSchema); err != nil {
		return shape, fmt.Errorf("reference schema: %w", err)
	}
	shape, err = readSchemaShape(reference.Conn)
	if err != nil {
		return shape, fmt.Errorf("reference schema: %w", err)
	}
	return shape, nil
}

// UnexpectedSchema lists, sorted, every way the database's tables and
// indexes go beyond what GoPMgr creates. An empty list means a later write
// behaves as it would on a file GoPMgr made. The entries carry names taken
// from the file, so they belong in logs, not in messages shown to the user.
func (db *Database) UnexpectedSchema() ([]string, error) {
	reference, err := referenceSchema()
	if err != nil {
		return nil, err
	}
	actual, err := readSchemaShape(db.Conn)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	return schemaDifferences(actual, reference), nil
}

func schemaDifferences(actual, reference schemaShape) []string {
	var found []string
	note := func(format string, args ...any) { found = append(found, fmt.Sprintf(format, args...)) }

	for name, table := range actual.tables {
		want, known := reference.tables[name]
		if !known {
			note("table %s", name)
			continue
		}
		if table.withoutRowID != want.withoutRowID || table.strict != want.strict {
			note("table %s storage options", name)
		}
		for column, c := range table.columns {
			wantColumn, known := want.columns[column]
			switch {
			case !known:
				note("column %s.%s", name, column)
				continue
			case c.hidden != wantColumn.hidden:
				note("generated column %s.%s", name, column)
			case c.primaryKey != wantColumn.primaryKey:
				note("primary key on %s.%s", name, column)
			case c.notNull && !wantColumn.notNull:
				note("NOT NULL on %s.%s", name, column)
			}
			if c.defaultSQL.Valid && !literalDefault.MatchString(c.defaultSQL.String) &&
				(!wantColumn.defaultSQL.Valid || c.defaultSQL.String != wantColumn.defaultSQL.String) {
				note("computed default on %s.%s", name, column)
			}
		}
		for key := range table.foreignKeys {
			if !want.foreignKeys[key] && !legacyForeignKeys[name][key] {
				note("foreign key on %s (%s)", name, key)
			}
		}
		for check := range table.checks {
			if !want.checks[check] {
				note("CHECK on %s", name)
			}
		}
	}

	// A plain index on existing columns only changes speed, and Migrate
	// replaces two older ones by name, so those pass whatever they are
	// called. An index that can refuse a write (UNIQUE), or that holds an
	// expression or a WHERE clause, must be one GoPMgr creates.
	uniqueInReference := map[string]bool{}
	for _, index := range reference.indexes {
		if index.unique && !index.partial {
			uniqueInReference[index.table+"("+strings.Join(index.columns, ",")+")"] = true
		}
	}
	for name, index := range actual.indexes {
		if _, known := reference.tables[index.table]; !known {
			continue // the table is already reported
		}
		expression := false
		for _, column := range index.columns {
			if column == "" {
				expression = true
			}
		}
		want, sameName := reference.indexes[name]
		switch {
		case expression || index.partial:
			if !sameName || want.table != index.table || want.sql != index.sql {
				note("index %s on %s", name, index.table)
			}
		case index.unique:
			if !uniqueInReference[index.table+"("+strings.Join(index.columns, ",")+")"] {
				note("UNIQUE on %s(%s)", index.table, strings.Join(index.columns, ","))
			}
		}
	}
	sort.Strings(found)
	return found
}
