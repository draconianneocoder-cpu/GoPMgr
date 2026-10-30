// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

// This file is compiled under `-tags duckdb`. It links the DuckDB engine
// (CGO) and provides the real New(). Production/package builds include this
// tag; untagged developer builds use the no-op New() in stub.go.
//
// Design: docs/design/duckdb-analytics-engine.md. Invariants honored:
//   - in-memory only (DSN ""), nothing persisted to disk;
//   - never opens the encrypted .pmforge file — callers pass rows in;
//   - extension autoinstall disabled (bundled reader autoload stays available).
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopmgr/internal/money"

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
		budgeted_cost_minor_units BIGINT,
		committed_cost_minor_units BIGINT,
		actual_cost_minor_units BIGINT,
		earned_value_minor_units BIGINT,
		planned_value_minor_units BIGINT,
		percent_complete DOUBLE,
		evm_available BOOLEAN
	)`); err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: create table: %w", err)
	}

	// duckdb-go is a database/sql driver and uses "?" positional
	// placeholders (verified with `go test -tags duckdb`). If a future
	// driver version ever reports a bind error, this is the one line to
	// switch to "$1..$9".
	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO portfolio VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, p := range projects {
		p = normaliseProjectMetricsMoney(p)
		if _, err := stmt.ExecContext(ctx,
			p.ProjectID, p.Name,
			p.BudgetedCostMinorUnits, p.CommittedCostMinorUnits,
			p.ActualCostMinorUnits,
			p.EarnedValueMinorUnits, p.PlannedValueMinorUnits,
			p.PercentComplete, p.EVMAvailable,
		); err != nil {
			return PortfolioSummary{}, fmt.Errorf("analytics: insert %q: %w", p.ProjectID, err)
		}
	}

	var s PortfolioSummary
	row := conn.QueryRowContext(ctx, `SELECT
		count(*),
		coalesce(sum(CASE WHEN evm_available THEN 1 ELSE 0 END), 0),
		coalesce(sum(budgeted_cost_minor_units), 0),
		coalesce(sum(committed_cost_minor_units), 0),
		coalesce(sum(CASE WHEN evm_available THEN actual_cost_minor_units ELSE 0 END), 0),
		coalesce(sum(CASE WHEN evm_available THEN earned_value_minor_units ELSE 0 END), 0),
		coalesce(sum(CASE WHEN evm_available THEN planned_value_minor_units ELSE 0 END), 0)
	FROM portfolio`)
	if err := row.Scan(
		&s.ProjectCount,
		&s.EVMProjectCount,
		&s.TotalBudgetedCostMinorUnits,
		&s.TotalCommittedCostMinorUnits,
		&s.TotalActualCostMinorUnits,
		&s.TotalEarnedValueMinorUnits,
		&s.TotalPlannedValueMinorUnits,
	); err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: aggregate: %w", err)
	}
	s.EVMUnavailableProjectCount = s.ProjectCount - s.EVMProjectCount
	s.TotalBudgetedCost = money.Amount{MinorUnits: s.TotalBudgetedCostMinorUnits}.MajorFloat()
	s.TotalCommittedCost = money.Amount{MinorUnits: s.TotalCommittedCostMinorUnits}.MajorFloat()
	s.TotalActualCost = money.Amount{MinorUnits: s.TotalActualCostMinorUnits}.MajorFloat()
	s.TotalEarnedValue = money.Amount{MinorUnits: s.TotalEarnedValueMinorUnits}.MajorFloat()
	s.TotalPlannedValue = money.Amount{MinorUnits: s.TotalPlannedValueMinorUnits}.MajorFloat()

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

func normaliseProjectMetricsMoney(p ProjectMetrics) ProjectMetrics {
	if p.BudgetedCostMinorUnits == 0 && p.BudgetedCost != 0 {
		p.BudgetedCostMinorUnits = money.FromMajorFloat(p.BudgetedCost).MinorUnits
	}
	if p.CommittedCostMinorUnits == 0 && p.CommittedCost != 0 {
		p.CommittedCostMinorUnits = money.FromMajorFloat(p.CommittedCost).MinorUnits
	}
	if p.ActualCostMinorUnits == 0 && p.ActualCost != 0 {
		p.ActualCostMinorUnits = money.FromMajorFloat(p.ActualCost).MinorUnits
	}
	if p.EarnedValueMinorUnits == 0 && p.EarnedValue != 0 {
		p.EarnedValueMinorUnits = money.FromMajorFloat(p.EarnedValue).MinorUnits
	}
	if p.PlannedValueMinorUnits == 0 && p.PlannedValue != 0 {
		p.PlannedValueMinorUnits = money.FromMajorFloat(p.PlannedValue).MinorUnits
	}
	return p
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
// extension from the network (harden), rejects glob-bearing paths, and caps
// the row count. `.xlsx` stays on the frontend `read-excel-file` reader.
func (e *duckEngine) ImportTabular(ctx context.Context, path string) (Dataset, error) {
	if e.initErr != nil {
		return Dataset{}, e.initErr
	}

	if strings.ContainsAny(path, "*?[") {
		return Dataset{}, ErrAmbiguousImportPath
	}
	// Resolve before Abs cleans symlink/.., so validation and the reader select
	// the same source. Keep format selection tied to the user's chosen name.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	if strings.ContainsAny(abs, "*?[") {
		return Dataset{}, ErrAmbiguousImportPath
	}
	sourceInfo, statErr := os.Stat(abs)
	if statErr != nil || sourceInfo.IsDir() {
		return Dataset{}, fmt.Errorf("analytics: not a readable file: %s", abs)
	}
	readerPath, err := filepath.Abs(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	if strings.ContainsAny(readerPath, "*?[") {
		return Dataset{}, ErrAmbiguousImportPath
	}
	// Preserve selected-path partition inference only when source identity is
	// confirmed. Cleaning symlink/.. may select another file or a missing one;
	// an identity mismatch or unavailable identity uses the canonical source.
	if selected, err := os.Stat(readerPath); err != nil || !os.SameFile(sourceInfo, selected) {
		readerPath = abs
	}
	reader, ok := tabularReader(filepath.Ext(path))
	if !ok {
		return Dataset{}, fmt.Errorf("analytics: unsupported file type %q (want .csv/.tsv/.parquet/.json)", filepath.Ext(path))
	}

	conn, err := e.db.Conn(ctx)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: acquire conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := harden(ctx, conn); err != nil {
		return Dataset{}, err
	}
	// Retain the best-effort setting, but it does not enforce confinement while
	// external access is enabled. The path checks above prevent glob expansion.
	_, _ = conn.ExecContext(ctx, fmt.Sprintf("SET allowed_directories=['%s']", sqlSingleQuote(filepath.Dir(abs))))

	q := fmt.Sprintf("SELECT * FROM %s('%s') LIMIT %d", reader, sqlSingleQuote(readerPath), maxImportRows)
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
