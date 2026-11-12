// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

func unexpectedSchemaOf(t *testing.T, path string) []string {
	t.Helper()
	d, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = d.Close() }()
	found, err := d.UnexpectedSchema()
	if err != nil {
		t.Fatalf("UnexpectedSchema: %v", err)
	}
	return found
}

// Files GoPMgr itself wrote, at any age, must pass: a false refusal here
// would lock a user out of their own backup.
func TestUnexpectedSchemaAcceptsEverySchemaGoPMgrHasWritten(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T) string{
		"a new project": func(t *testing.T) string { return plantedProject(t) },
		"a project with the WBS tables": func(t *testing.T) string {
			return plantedProject(t, wbsStorageSchema)
		},
		"sigma_projects before its rebuild": func(t *testing.T) string {
			return newLegacySigmaProjectsFile(t, "legacy-project", true)
		},
		"a project from before cost control": func(t *testing.T) string { return newLegacyCostControlFile(t) },
		"the older shape of the two repointed indexes": func(t *testing.T) string {
			return plantedProject(t,
				`DROP INDEX idx_scenarios_project`,
				`CREATE INDEX idx_scenarios_project ON scenarios(project_id, created_at)`,
				`DROP INDEX idx_scenario_charts_scenario`,
				`CREATE INDEX idx_scenario_charts_scenario ON scenario_charts(scenario_id, updated_at)`)
		},
		"money columns declared REAL, as before 2026-06-26": func(t *testing.T) string {
			return plantedProject(t, append(
				rebuilt("stakeholders", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL, hourly_rate REAL NOT NULL DEFAULT 0, contract_value REAL NOT NULL DEFAULT 0)`),
				`ALTER TABLE project RENAME COLUMN budget TO budget_numeric`,
				`ALTER TABLE project ADD COLUMN budget REAL NOT NULL DEFAULT 0`,
				`ALTER TABLE project DROP COLUMN budget_numeric`)...)
		},
		"a column an older version added with ALTER TABLE": func(t *testing.T) string {
			// The reference declares time_zone inside CREATE TABLE; an
			// upgraded file got it from ALTER TABLE with its own default.
			path := filepath.Join(t.TempDir(), "upgraded.gopmgr")
			conn, err := sql.Open(sqlitedriver.Name, path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			for _, stmt := range []string{
				`CREATE TABLE project (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
				`ALTER TABLE project ADD COLUMN time_zone TEXT NOT NULL DEFAULT 'America/New_York'`,
				`ALTER TABLE project ADD COLUMN country_code TEXT NOT NULL DEFAULT 'US'`,
			} {
				if _, err := conn.Exec(stmt); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
			}
			return path
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := build(t)
			if found := unexpectedSchemaOf(t, path); len(found) != 0 {
				t.Fatalf("a schema GoPMgr wrote was reported as foreign: %v", found)
			}
			if err := CheckForeignSchemaObjects(path, nil); err != nil {
				t.Fatalf("CheckForeignSchemaObjects = %v, want nil", err)
			}
		})
	}
}

// The migrated form of each legacy file must pass too: this is what a
// backup taken after an upgrade contains.
func TestUnexpectedSchemaAcceptsLegacyFilesAfterMigration(t *testing.T) {
	t.Parallel()
	for name, path := range map[string]string{
		"sigma":        newLegacySigmaProjectsFile(t, "legacy-project", true),
		"cost control": newLegacyCostControlFile(t),
	} {
		d, err := InitDB(path)
		if err != nil {
			t.Fatalf("%s: migrate: %v", name, err)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if found := unexpectedSchemaOf(t, path); len(found) != 0 {
			t.Errorf("%s after migration reported as foreign: %v", name, found)
		}
	}
}

// rebuilt replaces table's definition with definition, keeping the name.
func rebuilt(table, definition string) []string {
	return []string{`PRAGMA foreign_keys = OFF`, `DROP TABLE ` + table, `CREATE TABLE ` + table + definition}
}

func TestUnexpectedSchemaReportsWhatGoPMgrNeverCreates(t *testing.T) {
	t.Parallel()
	const stakeholders = `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL`
	cases := []struct {
		name       string
		statements []string
		want       []string
	}{
		{"an extra table", []string{`CREATE TABLE loot(a)`}, []string{"table loot"}},
		{"an extra column", []string{`ALTER TABLE project ADD COLUMN planted TEXT`}, []string{"column project.planted"}},
		{"a generated column", []string{`ALTER TABLE project ADD COLUMN name_copy TEXT GENERATED ALWAYS AS (name) VIRTUAL`},
			[]string{"column project.name_copy"}},
		{"a known column made generated", rebuilt("stakeholders", stakeholders+`, role TEXT GENERATED ALWAYS AS (name) VIRTUAL)`),
			[]string{"generated column stakeholders.role"}},
		{"a cascading foreign key", rebuilt("stakeholders", stakeholders+`, FOREIGN KEY(name) REFERENCES documents(id) ON DELETE CASCADE)`),
			[]string{"foreign key on stakeholders (name>documents.id NO ACTION CASCADE)"}},
		{"a foreign key GoPMgr has, with a different action", rebuilt("stakeholders", stakeholders+`, FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE SET NULL)`),
			[]string{"foreign key on stakeholders (project_id>project.id NO ACTION SET NULL)"}},
		{"a CHECK constraint", rebuilt("stakeholders", stakeholders+`, CHECK (length(name) < 3))`), []string{"CHECK on stakeholders"}},
		{"a CHECK written to look like text", rebuilt("stakeholders", stakeholders+` DEFAULT 'check (1)' cHeCk(name<>'x'))`), []string{"CHECK on stakeholders"}},
		{"a UNIQUE constraint", rebuilt("stakeholders", stakeholders+`, UNIQUE(name))`), []string{"UNIQUE on stakeholders(name)"}},
		{"a unique index", []string{`CREATE UNIQUE INDEX planted ON stakeholders(name)`}, []string{"UNIQUE on stakeholders(name)"}},
		{"an expression index", []string{`CREATE INDEX planted ON stakeholders(lower(name))`}, []string{"index planted on stakeholders"}},
		{"a partial index", []string{`CREATE INDEX planted ON stakeholders(name) WHERE name <> ''`}, []string{"index planted on stakeholders"}},
		{"GoPMgr's partial index with another condition", []string{
			`DROP INDEX idx_scenarios_one_active`,
			`CREATE UNIQUE INDEX idx_scenarios_one_active ON scenarios(project_id) WHERE is_active = 0`,
		}, []string{"index idx_scenarios_one_active on scenarios"}},
		{"a computed default", rebuilt("stakeholders", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT (hex(randomblob(4))))`),
			[]string{"computed default on stakeholders.name"}},
		{"NOT NULL where GoPMgr allows none", rebuilt("wbs_records", `(chart_id TEXT PRIMARY KEY NOT NULL REFERENCES charts(id), revision TEXT NOT NULL)`),
			[]string{"NOT NULL on wbs_records.chart_id"}},
		{"a moved primary key", rebuilt("stakeholders", `(id TEXT, project_id TEXT NOT NULL, name TEXT NOT NULL PRIMARY KEY)`),
			[]string{"UNIQUE on stakeholders(name)", "primary key on stakeholders.id", "primary key on stakeholders.name"}},
		{"WITHOUT ROWID", rebuilt("stakeholders", stakeholders+`) WITHOUT ROWID`), []string{"NOT NULL on stakeholders.id", "table stakeholders storage options"}},
		{"STRICT", rebuilt("stakeholders", stakeholders+`) STRICT`), []string{"NOT NULL on stakeholders.id", "table stakeholders storage options"}},
		// No pragma reports the next clauses; they are read from the table's
		// own text. Each silently changes what a write or a comparison does.
		{"a primary key that replaces on conflict", rebuilt("stakeholders", `(id TEXT PRIMARY KEY ON CONFLICT REPLACE, project_id TEXT NOT NULL, name TEXT NOT NULL)`),
			[]string{"CONFLICT clause on stakeholders"}},
		{"NOT NULL that ignores the row on conflict", rebuilt("stakeholders", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL on	conflict IGNORE, name TEXT NOT NULL)`),
			[]string{"CONFLICT clause on stakeholders"}},
		{"a column collation", rebuilt("stakeholders", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL COLLATE NOCASE)`),
			[]string{"COLLATE clause on stakeholders"}},
		{"a case-blind primary key", rebuilt("stakeholders", `(id TEXT PRIMARY KEY COLLATE NOCASE, project_id TEXT NOT NULL, name TEXT NOT NULL)`),
			[]string{"COLLATE clause on stakeholders", "UNIQUE on stakeholders(id)"}},
		{"AUTOINCREMENT where GoPMgr has none", rebuilt("stakeholders", `(id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, name TEXT NOT NULL)`),
			[]string{"AUTOINCREMENT clause on stakeholders", "type of stakeholders.id"}},
		{"a deferred foreign key", rebuilt("stakeholders", stakeholders+`, FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED)`),
			[]string{"DEFERRABLE clause on stakeholders", "DEFERRED clause on stakeholders"}},
		{"a text key turned into the rowid", rebuilt("stakeholders", `(id INTEGER PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL)`),
			[]string{"type of stakeholders.id"}},
		{"a column of another type", rebuilt("stakeholders", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name INTEGER NOT NULL)`),
			[]string{"type of stakeholders.name"}},
		// The table lost its UNIQUE constraint and an index of the same
		// columns took its place, comparing without regard to case.
		{"GoPMgr's unique rule under another collation", append(
			rebuilt("sigma_solutions", `(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, data_json TEXT NOT NULL DEFAULT '[]', FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE)`),
			`CREATE UNIQUE INDEX planted ON sigma_solutions(project_id COLLATE NOCASE)`),
			[]string{"UNIQUE on sigma_solutions(project_id)"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			statements := c.statements
			if strings.Contains(strings.Join(statements, " "), "wbs_records") {
				statements = append([]string{wbsStorageSchema}, statements...)
			}
			path := plantedProject(t, statements...)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := unexpectedSchemaOf(t, path); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("UnexpectedSchema = %q, want %q", got, c.want)
			}
			err = CheckForeignSchemaObjects(path, nil)
			var foreign *ForeignSchemaError
			if !errors.Is(err, ErrForeignSchemaObject) || !errors.As(err, &foreign) || !reflect.DeepEqual(foreign.Objects, c.want) {
				t.Fatalf("CheckForeignSchemaObjects = %v, want ErrForeignSchemaObject listing %q", err, c.want)
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
				t.Errorf("the check changed the file it inspected (err %v)", err)
			}
		})
	}
}

// SQLite reads a schema row typed "TABLE" as a table. The rules that read
// a table's text must find that row too, or its CHECK and conflict clauses
// are invisible. The row is changed in the file's bytes: the engine's
// defensive mode refuses to edit the schema table.
func TestUnexpectedSchemaSeesATableWhoseSchemaRowIsRetyped(t *testing.T) {
	t.Parallel()
	path := plantedProject(t, rebuilt("stakeholders",
		`(id TEXT PRIMARY KEY ON CONFLICT REPLACE, project_id TEXT NOT NULL, name TEXT NOT NULL CHECK (length(name) < 3))`)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A schema row stores its type, name and table name back to back. The
	// dropped table's old row can linger in free space, so every copy is
	// changed; only the live one is read.
	const row = "tablestakeholdersstakeholders"
	if !strings.Contains(string(data), row) {
		t.Fatal("no schema row for stakeholders found in the file")
	}
	data = []byte(strings.ReplaceAll(string(data), row, "TABLEstakeholdersstakeholders"))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// The engine still treats it as the table, constraints included.
	d, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	var columns int
	err = d.Conn.QueryRow(`SELECT count(*) FROM pragma_table_xinfo('stakeholders')`).Scan(&columns)
	_ = d.Close()
	if err != nil || columns != 3 {
		t.Fatalf("the retyped table is not readable as a table: %d columns, %v", columns, err)
	}

	want := []string{
		"CHECK on stakeholders",
		"CONFLICT clause on stakeholders",
		`schema row stakeholders of type "TABLE"`,
	}
	if got := unexpectedSchemaOf(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("UnexpectedSchema = %q, want %q", got, want)
	}
	if err := CheckForeignSchemaObjects(path, nil); !errors.Is(err, ErrForeignSchemaObject) {
		t.Fatalf("CheckForeignSchemaObjects = %v, want ErrForeignSchemaObject", err)
	}
}

func TestSchemaDifferencesReportsIrregularRows(t *testing.T) {
	t.Parallel()
	reference := schemaShape{tables: map[string]tableShape{}, indexes: map[string]indexShape{}}
	actual := schemaShape{
		tables: map[string]tableShape{}, indexes: map[string]indexShape{},
		irregular: []string{"table t without a definition"},
	}
	if got := schemaDifferences(actual, reference); !reflect.DeepEqual(got, []string{"table t without a definition"}) {
		t.Fatalf("schemaDifferences = %q", got)
	}
}

func TestBehaviourClausesIn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sql  string
		want []string
	}{
		{`CREATE TABLE t(a TEXT PRIMARY KEY)`, nil},
		{`CREATE TABLE t(a PRIMARY KEY ON CONFLICT REPLACE, b COLLATE nocase, c INTEGER)`, []string{"collate", "conflict"}},
		{"CREATE TABLE t(a INTEGER PRIMARY KEY\nAutoIncrement)", []string{"autoincrement"}},
		{`CREATE TABLE t(a REFERENCES u(a) DEFERRABLE INITIALLY DEFERRED)`, []string{"deferrable", "deferred"}},
		// Names and text that only look like the clause.
		{`CREATE TABLE t("conflict" TEXT DEFAULT 'on conflict replace', [collate] INT, ` + "`autoincrement`" + ` INT)`, nil},
		{"CREATE TABLE t(a -- on conflict replace\n, b /* collate nocase */)", nil},
		{`CREATE TABLE t(conflicts INT, collated INT, undeferred INT)`, nil},
		// An unterminated comment hides nothing that SQLite would run.
		{`CREATE TABLE t(a /* collate nocase`, nil},
	}
	for _, c := range cases {
		var got []string
		for word := range behaviourClausesIn(c.sql) {
			got = append(got, word)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("behaviourClausesIn(%q) = %q, want %q", c.sql, got, c.want)
		}
	}
}

func TestTypeAffinity(t *testing.T) {
	t.Parallel()
	for declared, want := range map[string]string{
		"TEXT": "TEXT", "text": "TEXT", "VARCHAR(20)": "TEXT", "CLOB": "TEXT",
		"INTEGER": "INTEGER", "BIGINT": "INTEGER", "POINT": "INTEGER", "CHARINT": "INTEGER",
		"": "BLOB", "BLOB": "BLOB",
		"REAL": "REAL", "DOUBLE PRECISION": "REAL", "FLOAT": "REAL",
		"NUMERIC": "NUMERIC", "DECIMAL(10,5)": "NUMERIC", "DATETIME": "NUMERIC", "BOOLEAN": "NUMERIC",
		// Precedence when a type names two: INT, then text, then BLOB, then REAL.
		"BLOBTEXT": "TEXT", "REALBLOB": "BLOB", "integer": "INTEGER",
		// SQLite folds ASCII only: a dotless i does not spell INT.
		"\u0131NTEGER": "NUMERIC",
	} {
		if got := typeAffinity(declared); got != want {
			t.Errorf("typeAffinity(%q) = %q, want %q", declared, got, want)
		}
	}
}

func TestCheckClauses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sql  string
		want []string
	}{
		{`CREATE TABLE t(a)`, nil},
		{`CREATE TABLE t(a CHECK (a IN ('x', 'y')), b CHECK( b >= 0 ))`, []string{`(aIN('x','y'))`, `(b>=0)`}},
		{"CREATE TABLE t(a,\n  CHECK\n\t( (a > 1) AND a < (2 + 3) ))", []string{`((a>1)ANDa<(2+3))`}},
		{`CREATE TABLE t("check" TEXT DEFAULT 'check (1)', [check (2)] INT, ` + "`check(3)`" + ` INT)`, nil},
		{"CREATE TABLE t(a -- check (9)\n, b /* check (8) */ CHECK (b = ')'))", []string{`(b=')')`}},
		{`CREATE TABLE t(a CHECK (a = 'it''s (' ))`, []string{`(a='it''s (')`}},
		{`CREATE TABLE t(checked INT, uncheck INT, a ChEcK(a))`, []string{`(a)`}},
		{`CREATE TABLE t(a CHECK (a`, []string{`(a`}},
	}
	for _, c := range cases {
		if got := checkClauses(c.sql); !reflect.DeepEqual(got, c.want) {
			t.Errorf("checkClauses(%q) = %q, want %q", c.sql, got, c.want)
		}
	}
}
