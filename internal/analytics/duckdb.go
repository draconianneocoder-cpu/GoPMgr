// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

// This file is compiled ONLY under `-tags duckdb`. It links the DuckDB
// engine (CGO) and provides the real New(). The default build uses the
// no-op New() in stub.go (//go:build !duckdb), so a standard PMForge
// download carries no DuckDB weight.
//
// Design: docs/design/duckdb-analytics-engine.md. Invariants honored:
//   - in-memory only (DSN ""), nothing persisted to disk;
//   - never opens the encrypted .pmforge file — callers pass rows in;
//   - extension autoinstall/autoload disabled (no network fetch).
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Registers the "duckdb" database/sql driver.
	_ "github.com/duckdb/duckdb-go/v2"
)

// duckEngine is the DuckDB-backed Engine. The whole analytics surface is
// ephemeral: an in-memory DuckDB shared over a single pooled connection.
type duckEngine struct {
	db      *sql.DB
	initErr error
}

// New opens an in-memory DuckDB engine. It never returns nil; if the open
// fails, the returned engine reports the error from every method so the
// caller can degrade gracefully (same contract as the stub).
func New() Engine {
	db, err := sql.Open("duckdb", "") // "" = in-memory, nothing on disk
	if err != nil {
		return &duckEngine{initErr: fmt.Errorf("analytics: open duckdb: %w", err)}
	}
	// Pin to a single connection so the in-memory database is consistent
	// across operations regardless of how the driver scopes ":memory:".
	db.SetMaxOpenConns(1)
	return &duckEngine{db: db}
}

func (e *duckEngine) Available() bool { return e.initErr == nil }

func (e *duckEngine) Close() error {
	if e.db == nil {
		return nil
	}
	return e.db.Close()
}

// harden prevents any network extension *download* (autoinstall). Auto-LOAD
// of already-bundled extensions (parquet/json) stays enabled — that is
// local-only, no network — so tabular import can use them fully offline.
func harden(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "SET autoinstall_known_extensions=false"); err != nil {
		return fmt.Errorf("analytics: harden: %w", err)
	}
	return nil
}

// PortfolioRollup loads the caller-supplied per-project metrics into an
// ephemeral in-memory table and lets DuckDB aggregate them. No files are
// touched; the data is whatever the app already read from SQLCipher.
func (e *duckEngine) PortfolioRollup(ctx context.Context, projects []ProjectMetrics) (PortfolioSummary, error) {
	if e.initErr != nil {
		return PortfolioSummary{}, e.initErr
	}

	conn, err := e.db.Conn(ctx)
	if err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: acquire conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := harden(ctx, conn); err != nil {
		return PortfolioSummary{}, err
	}

	if _, err := conn.ExecContext(ctx, `CREATE OR REPLACE TEMP TABLE portfolio (
		project_id       VARCHAR,
		name             VARCHAR,
		budgeted_cost    DOUBLE,
		actual_cost      DOUBLE,
		earned_value     DOUBLE,
		planned_value    DOUBLE,
		percent_complete DOUBLE
	)`); err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: create table: %w", err)
	}

	// duckdb-go is a database/sql driver and uses "?" positional
	// placeholders (verified with `go test -tags duckdb`). If a future
	// driver version ever reports a bind error, this is the one line to
	// switch to "$1..$7".
	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO portfolio VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, p := range projects {
		if _, err := stmt.ExecContext(ctx,
			p.ProjectID, p.Name, p.BudgetedCost, p.ActualCost,
			p.EarnedValue, p.PlannedValue, p.PercentComplete,
		); err != nil {
			return PortfolioSummary{}, fmt.Errorf("analytics: insert %q: %w", p.ProjectID, err)
		}
	}

	var s PortfolioSummary
	row := conn.QueryRowContext(ctx, `SELECT
		count(*),
		coalesce(sum(budgeted_cost), 0),
		coalesce(sum(actual_cost), 0),
		coalesce(sum(earned_value), 0),
		coalesce(sum(planned_value), 0)
	FROM portfolio`)
	if err := row.Scan(
		&s.ProjectCount,
		&s.TotalBudgetedCost,
		&s.TotalActualCost,
		&s.TotalEarnedValue,
		&s.TotalPlannedValue,
	); err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: aggregate: %w", err)
	}

	// Portfolio-level EVM indices; 0 means "n/a" (undefined), matching the
	// kernel's convention. The kernel stays the source of truth for
	// per-task scheduling math — this is only a cross-project rollup.
	if s.TotalPlannedValue != 0 {
		s.SchedulePerformanceIndex = s.TotalEarnedValue / s.TotalPlannedValue
	}
	if s.TotalActualCost != 0 {
		s.CostPerformanceIndex = s.TotalEarnedValue / s.TotalActualCost
	}
	return s, nil
}

// maxImportRows caps how many rows ImportTabular materialises into a
// Dataset, so a huge file can't blow up memory or the Wails bridge payload.
const maxImportRows = 10000

// tabularReader maps a file extension to the DuckDB reader table function.
// parquet/json are built-in (Primary-tier) extensions in standard DuckDB
// builds, so they read offline with autoinstall disabled.
func tabularReader(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case ".csv", ".tsv", ".txt":
		return "read_csv_auto", true
	case ".parquet":
		return "read_parquet", true
	case ".json", ".ndjson":
		return "read_json_auto", true
	}
	return "", false
}

// sqlSingleQuote escapes a value for use inside a single-quoted DuckDB
// string literal.
func sqlSingleQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

// ImportTabular reads a single local CSV/Parquet/JSON file (an explicit,
// user-chosen path) into an in-memory Dataset. It never installs an
// extension from the network (harden), scopes file access to the file's
// directory as defense-in-depth, and caps the row count. `.xlsx` is not
// handled here — that stays on the frontend `read-excel-file` reader.
func (e *duckEngine) ImportTabular(ctx context.Context, path string) (Dataset, error) {
	if e.initErr != nil {
		return Dataset{}, e.initErr
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	if fi, statErr := os.Stat(abs); statErr != nil || fi.IsDir() {
		return Dataset{}, fmt.Errorf("analytics: not a readable file: %s", abs)
	}
	reader, ok := tabularReader(filepath.Ext(abs))
	if !ok {
		return Dataset{}, fmt.Errorf("analytics: unsupported file type %q (want .csv/.tsv/.parquet/.json)", filepath.Ext(abs))
	}

	conn, err := e.db.Conn(ctx)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: acquire conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := harden(ctx, conn); err != nil {
		return Dataset{}, err
	}
	// Defense-in-depth: scope file access to the chosen file's directory.
	// Best-effort (the exact option name/semantics vary by DuckDB version);
	// the hard guarantees are that only `abs` is ever read, no network
	// extension can install (harden), and the database is in-memory.
	_, _ = conn.ExecContext(ctx, fmt.Sprintf("SET allowed_directories=['%s']", sqlSingleQuote(filepath.Dir(abs))))

	q := fmt.Sprintf("SELECT * FROM %s('%s') LIMIT %d", reader, sqlSingleQuote(abs), maxImportRows)
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: read file: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: columns: %w", err)
	}
	ds := Dataset{Columns: cols, Rows: [][]any{}}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return Dataset{}, fmt.Errorf("analytics: scan: %w", err)
		}
		ds.Rows = append(ds.Rows, vals)
	}
	if err := rows.Err(); err != nil {
		return Dataset{}, fmt.Errorf("analytics: rows: %w", err)
	}
	return ds, nil
}
