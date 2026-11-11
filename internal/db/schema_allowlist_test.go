// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
