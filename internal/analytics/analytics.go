// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package analytics is GoPMgr's optional, in-memory analytical engine.
//
// Design and decision record:
//   - docs/design/duckdb-analytics-engine.md
//   - docs/design/ADR-002-duckdb-vs-sqlcipher-evaluation.md
//
// SQLCipher remains GoPMgr's system of record. This package never opens
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

// ErrAnalyticsUnavailable is returned by every Engine method when GoPMgr
// was built without the DuckDB analytics engine.
// Callers should treat it as "feature not installed", not as a failure.
var ErrAnalyticsUnavailable = errors.New("analytics: engine not built in (rebuild with -tags duckdb)")

// ProjectMetrics is one project's pre-computed figures, supplied by the
// caller. The engine never reads these from disk — the app loads them
// from SQLCipher and passes them in.
type ProjectMetrics struct {
	ProjectID               string
	Name                    string
	BudgetedCost            float64 // approved project budget
	CommittedCost           float64 // contracts + estimated labour
	ActualCost              float64 // EVM AC from the authoritative schedule
	EarnedValue             float64 // EVM EV
	PlannedValue            float64 // EVM PV
	BudgetedCostMinorUnits  int64
	CommittedCostMinorUnits int64
	ActualCostMinorUnits    int64
	EarnedValueMinorUnits   int64
	PlannedValueMinorUnits  int64
	PercentComplete         float64 // EV/BAC, 0..100
	// EVMAvailable distinguishes a valid zero-valued status date from a
	// project whose schedule cannot support EVM. The engine must exclude
	// unavailable rows rather than making missing evidence look like zero.
	EVMAvailable bool
}

// PortfolioSummary is the aggregated result of a portfolio rollup across
// many projects. EVMProjectCount makes partial coverage visible; index fields
// use 0 to mean "n/a" (undefined), matching the kernel's EVM convention.
type PortfolioSummary struct {
	ProjectCount                 int     `json:"project_count"`
	EVMProjectCount              int     `json:"evm_project_count"`
	EVMUnavailableProjectCount   int     `json:"evm_unavailable_project_count"`
	AsOfDate                     string  `json:"as_of_date"`
	TotalBudgetedCost            float64 `json:"total_budgeted_cost"`
	TotalCommittedCost           float64 `json:"total_committed_cost"`
	TotalActualCost              float64 `json:"total_actual_cost"`
	TotalEarnedValue             float64 `json:"total_earned_value"`
	TotalPlannedValue            float64 `json:"total_planned_value"`
	TotalBudgetedCostMinorUnits  int64   `json:"total_budgeted_cost_minor_units"`
	TotalCommittedCostMinorUnits int64   `json:"total_committed_cost_minor_units"`
	TotalActualCostMinorUnits    int64   `json:"total_actual_cost_minor_units"`
	TotalEarnedValueMinorUnits   int64   `json:"total_earned_value_minor_units"`
	TotalPlannedValueMinorUnits  int64   `json:"total_planned_value_minor_units"`
	SchedulePerformanceIndex     float64 `json:"schedule_performance_index"` // SPI = ΣEV/ΣPV (0 = n/a)
	CostPerformanceIndex         float64 `json:"cost_performance_index"`     // CPI = ΣEV/ΣAC (0 = n/a)
}

// Dataset is a generic tabular result from a local-file import
// (CSV / Parquet / JSON). Rows are row-major; cell types follow the
// engine's inference.
type Dataset struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// Engine is GoPMgr's optional analytical backend. Implementations are
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
