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
	"errors"
	"fmt"

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

// harden disables extension autoinstall/autoload so a session can never
// reach out to the network to fetch an extension. These are documented
// runtime-settable options ("Securing DuckDB").
func harden(ctx context.Context, conn *sql.Conn) error {
	for _, q := range []string{
		"SET autoinstall_known_extensions=false",
		"SET autoload_known_extensions=false",
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("analytics: harden %q: %w", q, err)
		}
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
	defer conn.Close()

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

	// NOTE (verify on Mac): duckdb-go is a database/sql driver and uses
	// "?" positional placeholders. If a build ever reports a bind error,
	// this is the one line to switch to "$1..$7".
	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO portfolio VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return PortfolioSummary{}, fmt.Errorf("analytics: prepare insert: %w", err)
	}
	defer stmt.Close()

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

// ImportTabular lands in Phase D (CSV/Parquet/JSON local-file ingestion),
// where the file-access hardening (allowed_paths) must be implemented and
// verified carefully. Until then the real engine reports it as pending.
func (e *duckEngine) ImportTabular(ctx context.Context, path string) (Dataset, error) {
	if e.initErr != nil {
		return Dataset{}, e.initErr
	}
	return Dataset{}, errors.New("analytics: ImportTabular is not implemented yet (Phase D)")
}
