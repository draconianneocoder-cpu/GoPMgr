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
// cascading foreign key, a CHECK or UNIQUE constraint, a generated column,
// a computed default, a conflict clause, a collation or a column type that
// GoPMgr never asked for. This file compares such a
// schema with the one GoPMgr itself creates and lists every difference that
// could change what a later write does.
//
// The reference is not a hand-kept list. It is read from a database this
// build creates, so it cannot drift from the schema in sqlite.go and
// wbs_store.go. A file made by an older GoPMgr has fewer columns and tables
// than the reference, never different ones, with the two exceptions in
// legacyForeignKeys and the index rule below; so the test is "nothing in
// the file is outside the reference", not equality.
//
// Two things follow from that and are not checked. A constraint the file
// has dropped (a missing UNIQUE or foreign key) passes, because an older
// file may never have had it; the cost is a later write that errors or a
// rule that is not enforced, never SQL that runs. And a constant default
// may differ from the reference, because ALTER TABLE gave older files
// their own.

// tableShape is what decides how a table behaves on write.
type tableShape struct {
	withoutRowID, strict bool
	columns              map[string]columnShape
	foreignKeys          map[string]bool // "from>table.to on_update on_delete"
	checks               map[string]bool // CHECK expressions, whitespace removed
	clauses              map[string]bool // behaviourClauses words in the CREATE TABLE text
}

type columnShape struct {
	notNull    bool
	primaryKey int
	hidden     int // 0 ordinary; 2 and 3 are generated columns
	affinity   string
	defaultSQL sql.NullString
}

type indexShape struct {
	table   string
	unique  bool
	partial bool
	columns []string // "" for an expression
	// collations holds each key column's collating sequence. A UNIQUE
	// index compares with it, so "the same columns" is not the same rule
	// under another collation.
	collations []string
	sql        string
}

type schemaShape struct {
	tables  map[string]tableShape
	indexes map[string]indexShape
	// irregular lists schema rows whose stored form this file cannot read
	// the way SQLite does. Nothing GoPMgr writes produces one.
	irregular []string
}

// legacyForeignKeys are foreign keys an older GoPMgr created and Migrate
// still knows how to replace. Before migrateSigmaProjectsSchema,
// sigma_projects.id referenced the project row.
var legacyForeignKeys = map[string]map[string]bool{
	"sigma_projects": {"id>project.id NO ACTION CASCADE": true},
}

// legacyAffinities are column types an older GoPMgr declared. Three money
// columns were REAL until 2026-06-26 and NUMERIC afterwards; nothing
// rebuilds those tables, so a file from before that date still says REAL.
var legacyAffinities = map[string]string{
	"project.budget":              "REAL",
	"stakeholders.hourly_rate":    "REAL",
	"stakeholders.contract_value": "REAL",
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

	// SQLite reads a schema row's type without regard to case, and the
	// queries below must find the same rows it does. A row typed "TABLE"
	// would otherwise keep its table but hide its text from the CHECK and
	// clause rules. GoPMgr never writes such a row, so one is reported.
	odd, err := q.Query(`SELECT COALESCE(type, ''), COALESCE(name, '') FROM sqlite_master
		WHERE type IS NULL OR type NOT IN ('table', 'index', 'trigger', 'view')`)
	if err != nil {
		return shape, fmt.Errorf("list schema rows: %w", err)
	}
	for odd.Next() {
		var typ, name string
		if err := odd.Scan(&typ, &name); err != nil {
			_ = odd.Close()
			return shape, fmt.Errorf("list schema rows: %w", err)
		}
		shape.irregular = append(shape.irregular, fmt.Sprintf("schema row %s of type %q", name, typ))
	}
	if err := closeRows(odd); err != nil {
		return shape, fmt.Errorf("list schema rows: %w", err)
	}

	type tableRow struct {
		name, sql            string
		withoutRowID, strict bool
	}
	var tables []tableRow
	rows, err := q.Query(`SELECT l.name, l.wr, l.strict, COALESCE(m.sql, '')
		FROM pragma_table_list AS l LEFT JOIN sqlite_master AS m ON lower(m.type) = 'table' AND m.name = l.name
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
		if strings.TrimSpace(t.sql) == "" {
			shape.irregular = append(shape.irregular, fmt.Sprintf("table %s without a definition", t.name))
		}
		table := tableShape{
			withoutRowID: t.withoutRowID, strict: t.strict,
			columns: map[string]columnShape{}, foreignKeys: map[string]bool{}, checks: map[string]bool{},
		}
		for _, check := range checkClauses(t.sql) {
			table.checks[check] = true
		}
		table.clauses = behaviourClausesIn(t.sql)

		rows, err := q.Query(`SELECT name, "notnull", pk, hidden, dflt_value, COALESCE(type, '') FROM pragma_table_xinfo(?)`, t.name)
		if err != nil {
			return shape, fmt.Errorf("columns of %q: %w", t.name, err)
		}
		for rows.Next() {
			var name, declaredType string
			var c columnShape
			if err := rows.Scan(&name, &c.notNull, &c.primaryKey, &c.hidden, &c.defaultSQL, &declaredType); err != nil {
				_ = rows.Close()
				return shape, fmt.Errorf("columns of %q: %w", t.name, err)
			}
			c.affinity = typeAffinity(declaredType)
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
			rows, err := q.Query(`SELECT COALESCE(name, ''), COALESCE(coll, '') FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, i.name)
			if err != nil {
				return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
			}
			for rows.Next() {
				var column, collation string
				if err := rows.Scan(&column, &collation); err != nil {
					_ = rows.Close()
					return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
				}
				index.columns = append(index.columns, column)
				index.collations = append(index.collations, strings.ToUpper(collation))
			}
			if err := closeRows(rows); err != nil {
				return shape, fmt.Errorf("columns of index %q: %w", i.name, err)
			}
			rows, err = q.Query(`SELECT COALESCE(sql, '') FROM sqlite_master WHERE lower(type) = 'index' AND name = ?`, i.name)
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
	isWord := isSQLWordByte
	skipQuoted := func(i int) int { return skipQuotedSQL(s, i) }
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

func isSQLWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// skipQuotedSQL returns the index just past the quoted run that starts at
// s[i], which is one of ' " ` or [.
func skipQuotedSQL(s string, i int) int {
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

// behaviourClauses are the CREATE TABLE keywords that change what a write
// or a comparison does and that no pragma reports: a conflict clause
// (PRIMARY KEY ON CONFLICT REPLACE silently deletes the row it collides
// with), a column collation, AUTOINCREMENT, and a deferred foreign key.
// A table may use one only if the same table in the reference does.
var behaviourClauses = map[string]bool{
	"conflict": true, "collate": true, "autoincrement": true, "deferrable": true, "deferred": true,
}

// behaviourClausesIn returns the behaviourClauses words that appear in a
// CREATE TABLE statement outside quoted text and comments.
func behaviourClausesIn(createSQL string) map[string]bool {
	found := map[string]bool{}
	s := createSQL
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\'' || c == '"' || c == '`' || c == '[':
			i = skipQuotedSQL(s, i)
		case c == '-' && strings.HasPrefix(s[i:], "--"):
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				return found
			}
			i += end + 1
		case c == '/' && strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return found
			}
			i += end + 4
		case isSQLWordByte(c):
			start := i
			for i < len(s) && isSQLWordByte(s[i]) {
				i++
			}
			if word := strings.ToLower(s[start:i]); behaviourClauses[word] {
				found[word] = true
			}
		default:
			i++
		}
	}
	return found
}

// typeAffinity applies SQLite's rules for turning a declared column type
// into a storage affinity. Affinity decides how a written value is stored
// and compared, and INTEGER on a primary key makes the column the rowid,
// which refuses the text identifiers GoPMgr writes.
func typeAffinity(declared string) string {
	// SQLite folds only ASCII letters. strings.ToUpper would also fold
	// "ı" to "I" and see INT where SQLite does not.
	d := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}, declared)
	switch {
	case strings.Contains(d, "INT"):
		return "INTEGER"
	case strings.Contains(d, "CHAR"), strings.Contains(d, "CLOB"), strings.Contains(d, "TEXT"):
		return "TEXT"
	case d == "", strings.Contains(d, "BLOB"):
		return "BLOB"
	case strings.Contains(d, "REAL"), strings.Contains(d, "FLOA"), strings.Contains(d, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
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
	found = append(found, actual.irregular...)

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
			if c.affinity != wantColumn.affinity && c.affinity != legacyAffinities[name+"."+column] {
				note("type of %s.%s", name, column)
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
		for clause := range table.clauses {
			if !want.clauses[clause] {
				note("%s clause on %s", strings.ToUpper(clause), name)
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
			uniqueInReference[uniqueRule(index)] = true
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
			if !uniqueInReference[uniqueRule(index)] {
				note("UNIQUE on %s(%s)", index.table, strings.Join(index.columns, ","))
			}
		}
	}
	sort.Strings(found)
	return found
}

// uniqueRule names what a UNIQUE index enforces: its table, its columns in
// order, and the collation each is compared with.
func uniqueRule(index indexShape) string {
	return index.table + "(" + strings.Join(index.columns, ",") + ") " + strings.Join(index.collations, ",")
}
