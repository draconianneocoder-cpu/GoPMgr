// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package db is PMForge's persistence kernel. It wraps a single SQLite
// file (a ".pmforge" project) with WAL journaling, foreign-key
// enforcement, and self-healing helpers.
//
// All multi-statement migrations live in Migrate(); call it once after
// InitDB to bring an existing file up to the current schema.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"pmforge/internal/sqlitedriver"
)

// Database is the canonical handle passed to every service. Keep it
// small; per-domain logic lives in sibling files (settings.go, repair.go,
// backup.go, audit.go).
type Database struct {
	Conn *sql.DB
	Path string
}

// InitDB opens (or creates) a SQLite file at `path`, applies the
// PMForge-standard pragmas, and runs Migrate. The returned Database is
// safe for concurrent use by multiple goroutines because *sql.DB is
// already a connection pool.
func InitDB(path string) (*Database, error) {
	return initDBWithDSN(path, path)
}

func initDBWithDSN(path, dsn string) (*Database, error) {
	conn, err := sql.Open(sqlitedriver.Name, withConnPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	if err := applyStandardPragmas(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	db := &Database{Conn: conn, Path: path}
	if err := db.Migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := ensurePrivateSQLiteFiles(path); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("private database file: %w", err)
	}
	return db, nil
}

// withConnPragmas appends the per-connection pragmas as DSN options so
// EVERY connection the *sql.DB pool opens gets them. A plain
// conn.Exec("PRAGMA foreign_keys = ON") binds only to the one physical
// connection that happens to serve that Exec; with the default unbounded
// pool, a second connection opened under concurrent Wails calls would
// silently run with foreign keys OFF. journal_mode is also included even
// though WAL persists in the file, so brand-new files are WAL from the
// first connection regardless of which one touches them first.
func withConnPragmas(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
}

// applyStandardPragmas applies the pragmas that have no DSN form.
// temp_store is a per-connection performance hint only — correctness
// never depends on it — so best-effort application to the first pooled
// connection is acceptable. It also serves as an early connectivity
// probe (a bad key or unreadable file fails here, before Migrate).
func applyStandardPragmas(conn *sql.DB) error {
	if _, err := conn.Exec("PRAGMA temp_store = MEMORY;"); err != nil {
		return fmt.Errorf("pragma temp_store: %w", err)
	}
	return nil
}

// Migrate creates every table PMForge needs if it does not already exist.
// The schema is intentionally additive: never DROP or ALTER a column in a
// migration that ships to users; introduce a new column with a default.
//
// Tables, grouped by concern:
//
//   - settings, tasks, command_log, audit_log  (V1)
//   - project, charts, documents, templates    (V2 — multi-entity model)
//
// V2 tables are the foundation for the 19 chart types and 25 document
// types: rather than one table per kind, every chart lives in `charts`
// with a `kind` discriminator and a JSON `data` blob whose shape
// depends on the kind. The same pattern applies to documents.
func (db *Database) Migrate() error {
	schema := `
	-- ===========================================================
	-- V1 tables
	-- ===========================================================

	CREATE TABLE IF NOT EXISTS settings (
		id                INTEGER PRIMARY KEY CHECK (id = 1),
		default_password  TEXT NOT NULL DEFAULT '',
		export_theme      TEXT NOT NULL DEFAULT 'modern',
		auto_repair       INTEGER NOT NULL DEFAULT 1,
		cert_path         TEXT NOT NULL DEFAULT '',
		signature_enabled INTEGER NOT NULL DEFAULT 0,
		signature_method  TEXT NOT NULL DEFAULT '',
		gpg_key_id        TEXT NOT NULL DEFAULT '',
		default_font      TEXT NOT NULL DEFAULT '',
		compliance_mode   INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL,
		duration    REAL NOT NULL DEFAULT 0,
		precedents  TEXT NOT NULL DEFAULT '[]',
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);

	CREATE TABLE IF NOT EXISTS command_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ts          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		actor       TEXT NOT NULL,
		command     TEXT NOT NULL,
		payload     TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ts          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		actor       TEXT NOT NULL,
		action      TEXT NOT NULL,
		target_id   TEXT NOT NULL,
		details     TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log(target_id);
	CREATE INDEX IF NOT EXISTS idx_audit_ts     ON audit_log(ts);

	CREATE TABLE IF NOT EXISTS audit_events (
		id                       TEXT PRIMARY KEY,
		project_id               TEXT NOT NULL,
		sequence_number          INTEGER NOT NULL,
		previous_event_hash      TEXT NOT NULL DEFAULT '',
		event_hash               TEXT NOT NULL,
		event_type               TEXT NOT NULL,
		entity_type              TEXT NOT NULL,
		entity_id                TEXT NOT NULL DEFAULT '',
		before_canonical_json    TEXT NOT NULL DEFAULT 'null',
		after_canonical_json     TEXT NOT NULL DEFAULT 'null',
		user_id                  TEXT NOT NULL DEFAULT '',
		session_id               TEXT NOT NULL DEFAULT '',
		timestamp_utc            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		signature_status         TEXT NOT NULL DEFAULT 'unsigned',
		signature_blob_optional  TEXT NOT NULL DEFAULT '',
		UNIQUE(project_id, sequence_number)
	);

	CREATE INDEX IF NOT EXISTS idx_audit_events_project_seq ON audit_events(project_id, sequence_number);
	CREATE INDEX IF NOT EXISTS idx_audit_events_hash ON audit_events(project_id, event_hash);

	-- ===========================================================
	-- V2 tables: project lifecycle, charts, documents, templates
	-- ===========================================================

	-- A .pmforge file currently contains exactly ONE project, but the
	-- table is shaped to support multi-project files later.
	--
	-- Columns industry / sub_category / methodology / country_code
	-- were added in V2.x to support the Project Launchpad. They are
	-- optional (empty string defaults) so older .pmforge files open
	-- without migration. The Launchpad's seeding rules
	-- (internal/templates) read (industry, methodology).
	CREATE TABLE IF NOT EXISTS project (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'planning',
		phase         TEXT NOT NULL DEFAULT 'initiation',
		start_date    TEXT NOT NULL DEFAULT '',
		end_date      TEXT NOT NULL DEFAULT '',
		budget        NUMERIC NOT NULL DEFAULT 0,
		budget_minor_units INTEGER NOT NULL DEFAULT 0,
		owner         TEXT NOT NULL DEFAULT '',
		industry      TEXT NOT NULL DEFAULT '',
		sub_category  TEXT NOT NULL DEFAULT '',
		methodology   TEXT NOT NULL DEFAULT '',
		country_code  TEXT NOT NULL DEFAULT 'US',  -- ISO 3166-1 alpha-2 for rickar/cal holidays
		created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);

	-- For projects created before V2.x, ALTER columns into place.
	-- SQLite ignores duplicate ADD COLUMN; wrapping in idempotent
	-- pragmas keeps migration safe to re-run.
	-- (Older files will silently gain the four new columns on next
	-- open; defaults match the table definition above.)

	-- Charts table. ` + "`kind`" + ` is one of the 19 chart types defined in
	-- internal/charts/registry.go. ` + "`data`" + ` and ` + "`config`" + ` are JSON whose
	-- shape depends on kind.
	CREATE TABLE IF NOT EXISTS charts (
		id           TEXT PRIMARY KEY,
		project_id   TEXT NOT NULL,
		kind         TEXT NOT NULL,
		title        TEXT NOT NULL,
		data         TEXT NOT NULL DEFAULT '{}',
		config       TEXT NOT NULL DEFAULT '{}',
		template_id  TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_charts_project ON charts(project_id);
	CREATE INDEX IF NOT EXISTS idx_charts_kind    ON charts(kind);

	-- Schedule baselines (roadmap item 17). One row per snapshot of a
	-- CPM chart's scheduled task map (opaque JSON of kernel.Task keyed
	-- by task ID); used for planned-vs-baseline variance.
	CREATE TABLE IF NOT EXISTS baselines (
		id           TEXT PRIMARY KEY,
		project_id   TEXT NOT NULL,
		chart_id     TEXT NOT NULL,
		name         TEXT NOT NULL DEFAULT '',
		data         TEXT NOT NULL DEFAULT '{}',
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE,
		FOREIGN KEY (chart_id)   REFERENCES charts(id)  ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_baselines_chart ON baselines(chart_id);

	-- What-if scenario metadata. Scenario task/baseline branching is
	-- introduced in later slices; this table establishes the stable
	-- project-local identity and active-scenario contract.
	CREATE TABLE IF NOT EXISTS scenarios (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL,
		name               TEXT NOT NULL,
		source_baseline_id TEXT NOT NULL DEFAULT '',
		description        TEXT NOT NULL DEFAULT '',
		is_active          INTEGER NOT NULL DEFAULT 0,
		created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_scenarios_project ON scenarios(project_id, created_at);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_scenarios_one_active ON scenarios(project_id) WHERE is_active = 1;

	CREATE TABLE IF NOT EXISTS scenario_charts (
		id                 TEXT PRIMARY KEY,
		scenario_id        TEXT NOT NULL,
		project_id         TEXT NOT NULL,
		source_chart_id    TEXT NOT NULL DEFAULT '',
		source_baseline_id TEXT NOT NULL DEFAULT '',
		kind               TEXT NOT NULL DEFAULT '',
		title              TEXT NOT NULL DEFAULT '',
		data               TEXT NOT NULL DEFAULT '{}',
		config             TEXT NOT NULL DEFAULT '{}',
		baseline_data      TEXT NOT NULL DEFAULT '{}',
		created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE,
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_scenario_charts_scenario ON scenario_charts(scenario_id, updated_at);
	CREATE INDEX IF NOT EXISTS idx_scenario_charts_source ON scenario_charts(project_id, source_chart_id);

	-- Documents table. ` + "`kind`" + ` is one of the 25 document types defined
	-- in internal/documents/registry.go. ` + "`content`" + ` is JSON keyed by the
	-- kind's schema.
	CREATE TABLE IF NOT EXISTS documents (
		id           TEXT PRIMARY KEY,
		project_id   TEXT NOT NULL,
		kind         TEXT NOT NULL,
		title        TEXT NOT NULL,
		content      TEXT NOT NULL DEFAULT '{}',
		template_id  TEXT NOT NULL DEFAULT '',
		version      INTEGER NOT NULL DEFAULT 1,
		status       TEXT NOT NULL DEFAULT 'draft',
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project_id);
	CREATE INDEX IF NOT EXISTS idx_documents_kind    ON documents(kind);

	-- User-defined templates. Built-in templates live in the binary
	-- (see internal/documents/templates.go and internal/charts/templates.go);
	-- this table stores templates a user has saved from a chart or
	-- document they themselves built. is_builtin=0 always.
	CREATE TABLE IF NOT EXISTS templates (
		id           TEXT PRIMARY KEY,
		scope        TEXT NOT NULL,            -- 'chart' or 'document'
		kind         TEXT NOT NULL,            -- the chart/doc kind it templates
		name         TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		defaults     TEXT NOT NULL DEFAULT '{}',
		is_builtin   INTEGER NOT NULL DEFAULT 0,
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);

	CREATE INDEX IF NOT EXISTS idx_templates_kind ON templates(scope, kind);

	-- ===========================================================
	-- Agile Pack tables (V2.x — Kanban, Sprints, DORA)
	-- ===========================================================

	CREATE TABLE IF NOT EXISTS agile_boards (
		id          TEXT PRIMARY KEY,
		project_id  TEXT NOT NULL,
		name        TEXT NOT NULL,
		is_default  INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS agile_columns (
		id         TEXT PRIMARY KEY,
		board_id   TEXT NOT NULL,
		name       TEXT NOT NULL,
		order_idx  INTEGER NOT NULL DEFAULT 0,
		wip_limit  INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (board_id) REFERENCES agile_boards(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_columns_board ON agile_columns(board_id, order_idx);

	-- Work items move between columns (state == column id) and may
	-- belong to a sprint (sprint_id is FK or empty for backlog).
	CREATE TABLE IF NOT EXISTS agile_work_items (
		id          TEXT PRIMARY KEY,
		project_id  TEXT NOT NULL,
		type        TEXT NOT NULL DEFAULT 'story',
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		state       TEXT NOT NULL DEFAULT 'backlog',
		points      REAL NOT NULL DEFAULT 0,
		assignee    TEXT NOT NULL DEFAULT '',
		sprint_id   TEXT NOT NULL DEFAULT '',
		priority    TEXT NOT NULL DEFAULT 'medium',
		order_idx   INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		closed_at   TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_items_state    ON agile_work_items(project_id, state, order_idx);
	CREATE INDEX IF NOT EXISTS idx_items_sprint   ON agile_work_items(project_id, sprint_id);
	CREATE INDEX IF NOT EXISTS idx_items_assignee ON agile_work_items(project_id, assignee);

	CREATE TABLE IF NOT EXISTS agile_sprints (
		id          TEXT PRIMARY KEY,
		project_id  TEXT NOT NULL,
		name        TEXT NOT NULL,
		goal        TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'planning',
		start_date  TEXT NOT NULL DEFAULT '',
		end_date    TEXT NOT NULL DEFAULT '',
		capacity    REAL NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_sprints_status ON agile_sprints(project_id, status);

	-- One row per production deployment, used to compute the four
	-- DORA metrics over a rolling window.
	CREATE TABLE IF NOT EXISTS agile_deployments (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL,
		ts                 TEXT NOT NULL,
		version            TEXT NOT NULL DEFAULT '',
		successful         INTEGER NOT NULL DEFAULT 1,
		lead_time_hours    REAL NOT NULL DEFAULT 0,
		restore_time_hours REAL NOT NULL DEFAULT 0,
		notes              TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_deploys_ts ON agile_deployments(project_id, ts);

	-- Project-level stakeholder address book. Promoted from
	-- per-document strings (Charter, Stakeholder Analysis) to a
	-- shared project resource in V2.x so RACI rows, document fields,
	-- and the budget rollup can all reference the same record.
	CREATE TABLE IF NOT EXISTS stakeholders (
		id              TEXT PRIMARY KEY,
		project_id      TEXT NOT NULL,
		name            TEXT NOT NULL,
		role            TEXT NOT NULL DEFAULT '',
		organisation    TEXT NOT NULL DEFAULT '',
		email           TEXT NOT NULL DEFAULT '',
		phone           TEXT NOT NULL DEFAULT '',
		category        TEXT NOT NULL DEFAULT 'team',  -- team | vendor | sponsor | external
		hourly_rate     NUMERIC NOT NULL DEFAULT 0,
		hourly_rate_minor_units INTEGER NOT NULL DEFAULT 0,
		contract_value  NUMERIC NOT NULL DEFAULT 0,
		contract_value_minor_units INTEGER NOT NULL DEFAULT 0,
		availability    REAL NOT NULL DEFAULT 1, -- resource capacity in units (1 = full-time)
		notes           TEXT NOT NULL DEFAULT '',
		created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_stakeholders_project ON stakeholders(project_id);
	CREATE INDEX IF NOT EXISTS idx_stakeholders_cat     ON stakeholders(project_id, category);

	-- Named resource calendars hold per-resource capacity overrides
	-- for resource levelling and over-allocation checks. Capacity is
	-- stored in units (1.0 = full-time), not money.
	CREATE TABLE IF NOT EXISTS resource_calendars (
		id               TEXT PRIMARY KEY,
		project_id       TEXT NOT NULL,
		resource         TEXT NOT NULL DEFAULT '',
		name             TEXT NOT NULL DEFAULT '',
		default_capacity REAL NOT NULL DEFAULT 1,
		weekly_capacity  TEXT NOT NULL DEFAULT '{}',
		overrides        TEXT NOT NULL DEFAULT '{}',
		skill_tags       TEXT NOT NULL DEFAULT '[]',
		notes            TEXT NOT NULL DEFAULT '{}',
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_resource_calendars_project ON resource_calendars(project_id);
	CREATE INDEX IF NOT EXISTS idx_resource_calendars_resource ON resource_calendars(project_id, resource);

	-- Process Excellence Suite (Six Sigma) — MVP 1
	CREATE TABLE IF NOT EXISTS sigma_projects (
		id             TEXT PRIMARY KEY,
		title          TEXT NOT NULL,
		description    TEXT NOT NULL DEFAULT '',
		belt_level     TEXT NOT NULL DEFAULT 'green',
		phase          TEXT NOT NULL DEFAULT 'define',
		status         TEXT NOT NULL DEFAULT 'active',
		sponsor        TEXT NOT NULL DEFAULT '',
		process_owner  TEXT NOT NULL DEFAULT '',
		belt_lead      TEXT NOT NULL DEFAULT '',
		created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (id) REFERENCES project(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sigma_charters (
		id               TEXT PRIMARY KEY,
		project_id       TEXT NOT NULL UNIQUE,
		problem_statement TEXT NOT NULL DEFAULT '',
		business_case    TEXT NOT NULL DEFAULT '',
		goal_statement   TEXT NOT NULL DEFAULT '',
		scope_in         TEXT NOT NULL DEFAULT '[]',
		scope_out        TEXT NOT NULL DEFAULT '[]',
		ctqs             TEXT NOT NULL DEFAULT '[]',
		sponsor          TEXT NOT NULL DEFAULT '',
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_sigma_projects_phase ON sigma_projects(phase);
	CREATE INDEX IF NOT EXISTS idx_sigma_projects_status ON sigma_projects(status);

	CREATE TABLE IF NOT EXISTS sigma_fishbones (
		id                TEXT PRIMARY KEY,
		project_id        TEXT NOT NULL UNIQUE,
		problem_statement TEXT NOT NULL DEFAULT '',
		data_json         TEXT NOT NULL DEFAULT '{"branches":[]}',
		updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sigma_solutions (
		id         TEXT PRIMARY KEY,
		project_id TEXT NOT NULL UNIQUE,
		data_json  TEXT NOT NULL DEFAULT '[]',
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sigma_control_plans (
		id         TEXT PRIMARY KEY,
		project_id TEXT NOT NULL UNIQUE,
		data_json  TEXT NOT NULL DEFAULT '[]',
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sigma_sipocs (
		id         TEXT PRIMARY KEY,
		project_id TEXT NOT NULL UNIQUE,
		data_json  TEXT NOT NULL DEFAULT '{"elements":[]}',
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sigma_voc (
		id         TEXT PRIMARY KEY,
		project_id TEXT NOT NULL UNIQUE,
		data_json  TEXT NOT NULL DEFAULT '{"entries":[]}',
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (project_id) REFERENCES sigma_projects(id) ON DELETE CASCADE
	);
	`
	if _, err := db.Conn.Exec(schema); err != nil {
		return err
	}
	return db.migrateLegacyColumns()
}

// migrateLegacyColumns folds the V2.x project columns (industry,
// sub_category, methodology, country_code) onto .pmforge files
// created before the Launchpad shipped. SQLite's ALTER TABLE ADD
// COLUMN errors if the column already exists, so we probe first and
// only run the ADD when missing.
//
// All new columns are nullable-with-default at the schema level, so
// once they exist, every row already satisfies them.
func (db *Database) migrateLegacyColumns() error {
	type col struct {
		name string
		ddl  string
	}
	wanted := []col{
		{"industry", "ALTER TABLE project ADD COLUMN industry TEXT NOT NULL DEFAULT ''"},
		{"sub_category", "ALTER TABLE project ADD COLUMN sub_category TEXT NOT NULL DEFAULT ''"},
		{"methodology", "ALTER TABLE project ADD COLUMN methodology TEXT NOT NULL DEFAULT ''"},
		{"country_code", "ALTER TABLE project ADD COLUMN country_code TEXT NOT NULL DEFAULT 'US'"},
		{"budget_minor_units", "ALTER TABLE project ADD COLUMN budget_minor_units INTEGER NOT NULL DEFAULT 0"},
	}
	existing, err := db.columnSet("project")
	if err != nil {
		return err
	}
	for _, c := range wanted {
		if _, ok := existing[c.name]; ok {
			continue
		}
		if _, err := db.Conn.Exec(c.ddl); err != nil {
			return fmt.Errorf("add column %s: %w", c.name, err)
		}
		if c.name == "budget_minor_units" {
			if _, err := db.Conn.Exec(
				`UPDATE project
				 SET budget_minor_units = CAST(ROUND(budget * 100) AS INTEGER)
				 WHERE budget_minor_units = 0 AND budget != 0`,
			); err != nil {
				return fmt.Errorf("backfill budget_minor_units: %w", err)
			}
		}
	}

	// settings columns added after the initial schema shipped.
	settingsCols, err := db.columnSet("settings")
	if err != nil {
		return err
	}
	settingsMigrations := []struct {
		name string
		ddl  string
	}{
		{"default_font", "ALTER TABLE settings ADD COLUMN default_font TEXT NOT NULL DEFAULT ''"},
		{"agile_enabled", "ALTER TABLE settings ADD COLUMN agile_enabled INTEGER NOT NULL DEFAULT 0"},
		{"compliance_mode", "ALTER TABLE settings ADD COLUMN compliance_mode INTEGER NOT NULL DEFAULT 0"},
		{"signature_method", "ALTER TABLE settings ADD COLUMN signature_method TEXT NOT NULL DEFAULT ''"},
		{"gpg_key_id", "ALTER TABLE settings ADD COLUMN gpg_key_id TEXT NOT NULL DEFAULT ''"},
	}
	for _, m := range settingsMigrations {
		if _, ok := settingsCols[m.name]; ok {
			continue
		}
		if _, err := db.Conn.Exec(m.ddl); err != nil {
			return fmt.Errorf("add column %s: %w", m.name, err)
		}
	}

	// stakeholders columns added after the initial schema shipped
	// (roadmap item 19: availability = resource capacity in units).
	stakeholderCols, err := db.columnSet("stakeholders")
	if err != nil {
		return err
	}
	if _, ok := stakeholderCols["availability"]; !ok {
		if _, err := db.Conn.Exec(
			"ALTER TABLE stakeholders ADD COLUMN availability REAL NOT NULL DEFAULT 1",
		); err != nil {
			return fmt.Errorf("add column availability: %w", err)
		}
	}
	stakeholderMoneyMigrations := []struct {
		name     string
		ddl      string
		backfill string
	}{
		{
			name:     "hourly_rate_minor_units",
			ddl:      "ALTER TABLE stakeholders ADD COLUMN hourly_rate_minor_units INTEGER NOT NULL DEFAULT 0",
			backfill: `UPDATE stakeholders SET hourly_rate_minor_units = CAST(ROUND(hourly_rate * 100) AS INTEGER) WHERE hourly_rate_minor_units = 0 AND hourly_rate != 0`,
		},
		{
			name:     "contract_value_minor_units",
			ddl:      "ALTER TABLE stakeholders ADD COLUMN contract_value_minor_units INTEGER NOT NULL DEFAULT 0",
			backfill: `UPDATE stakeholders SET contract_value_minor_units = CAST(ROUND(contract_value * 100) AS INTEGER) WHERE contract_value_minor_units = 0 AND contract_value != 0`,
		},
	}
	for _, m := range stakeholderMoneyMigrations {
		if _, ok := stakeholderCols[m.name]; ok {
			continue
		}
		if _, err := db.Conn.Exec(m.ddl); err != nil {
			return fmt.Errorf("add column %s: %w", m.name, err)
		}
		if _, err := db.Conn.Exec(m.backfill); err != nil {
			return fmt.Errorf("backfill %s: %w", m.name, err)
		}
	}
	return nil
}

// columnSet returns the column names of a given table as a set.
// Used by migrateLegacyColumns to make ADD COLUMN idempotent.
func (db *Database) columnSet(table string) (map[string]struct{}, error) {
	rows, err := db.Conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]struct{})
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}

// CheckIntegrity runs SQLite's PRAGMA integrity_check. Returns true only
// if the engine reports "ok" verbatim. Any other value indicates the
// file is at least partially corrupt and InformativeSelfHeal should run.
func (db *Database) CheckIntegrity() (bool, error) {
	var result string
	err := db.Conn.QueryRow("PRAGMA integrity_check;").Scan(&result)
	if err != nil {
		return false, err
	}
	return result == "ok", nil
}

// CreateSnapshot uses VACUUM INTO to copy the live database to a new
// file in a transactionally-consistent way. This is the preferred way to
// take a backup while the application is running.
//
// NOTE: VACUUM INTO is rejected if targetPath already exists. Callers
// should remove or rename existing files first.
func (db *Database) CreateSnapshot(targetPath string) error {
	_, err := db.Conn.Exec("VACUUM INTO ?", targetPath)
	return err
}

// Vacuum reclaims free pages in the live database.
func (db *Database) Vacuum() error {
	_, err := db.Conn.Exec("VACUUM;")
	return err
}

// Close releases the underlying connection pool.
func (db *Database) Close() error {
	if db == nil || db.Conn == nil {
		return nil
	}
	return db.Conn.Close()
}

func ensurePrivateSQLiteFiles(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err := os.Chmod(sidecar, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
