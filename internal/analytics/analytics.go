// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package analytics is PMForge's optional, in-memory analytical engine.
//
// Design and decision record:
//   - docs/design/duckdb-analytics-engine.md
//   - docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md
//
// SQLCipher remains PMForge's system of record. This package never opens
// the encrypted .pmforge file: callers read rows from SQLCipher (already
// decrypted in process) and hand them to the Engine, which aggregates
// them in memory. The real implementation is DuckDB-backed and compiled
// in under the `duckdb` build tag. Production/package builds set that tag;
// untagged developer builds link the no-op stub in stub.go so analytics
// features degrade gracefully during local experiments.
package analytics

import (
	"context"
	"errors"
)

// ErrAnalyticsUnavailable is returned by every Engine method when PMForge
// was built without the DuckDB analytics engine.
// Callers should treat it as "feature not installed", not as a failure.
var ErrAnalyticsUnavailable = errors.New("analytics: engine not built in (rebuild with -tags duckdb)")

// ProjectMetrics is one project's pre-computed figures, supplied by the
// caller. The engine never reads these from disk — the app loads them
// from SQLCipher and passes them in.
type ProjectMetrics struct {
	ProjectID       string
	Name            string
	BudgetedCost    float64 // BAC
	ActualCost      float64 // AC
	EarnedValue     float64 // EV
	PlannedValue    float64 // PV
	PercentComplete float64 // 0..100
}

// PortfolioSummary is the aggregated result of a portfolio rollup across
// many projects. Index fields use 0 to mean "n/a" (undefined), matching
// the kernel's EVM convention.
type PortfolioSummary struct {
	ProjectCount             int     `json:"project_count"`
	TotalBudgetedCost        float64 `json:"total_budgeted_cost"`
	TotalActualCost          float64 `json:"total_actual_cost"`
	TotalEarnedValue         float64 `json:"total_earned_value"`
	TotalPlannedValue        float64 `json:"total_planned_value"`
	SchedulePerformanceIndex float64 `json:"schedule_performance_index"` // SPI = EV/PV (0 = n/a)
	CostPerformanceIndex     float64 `json:"cost_performance_index"`      // CPI = EV/AC (0 = n/a)
}

// Dataset is a generic tabular result from a local-file import
// (CSV / Parquet / JSON). Rows are row-major; cell types follow the
// engine's inference.
type Dataset struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// Engine is PMForge's optional analytical backend. Implementations are
// in-memory and ephemeral; they receive data the app already decrypted
// and must never open the encrypted .pmforge file. Implementations must
// be safe to Close once and tolerate Close being called on a stub.
type Engine interface {
	// PortfolioRollup aggregates per-project metrics into portfolio totals.
	PortfolioRollup(ctx context.Context, projects []ProjectMetrics) (PortfolioSummary, error)

	// ImportTabular reads a single local CSV/Parquet/JSON file (an explicit,
	// user-chosen path) into a Dataset. Implementations must restrict file
	// access to that path and must not enable network or extension
	// auto-install. .xlsx is intentionally not handled here (see the
	// file-import evaluation in the design doc).
	ImportTabular(ctx context.Context, path string) (Dataset, error)

	// Available reports whether a real engine is compiled in. The UI can
	// use this to show or hide analytics features without provoking an error.
	Available() bool

	// Close releases engine resources. Safe to call on the stub.
	Close() error
}
