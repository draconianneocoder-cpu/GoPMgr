// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package analytics is GoPMgr's in-memory analytics engine, written in Go.
//
// It does two things: PortfolioRollup totals per-project figures with exact
// minor-unit arithmetic, and ImportTabular reads one user-selected CSV, TSV,
// TXT, JSON, NDJSON or Parquet file into a bounded Dataset for display.
//
// SQLCipher remains GoPMgr's system of record. This package never opens the
// encrypted project file: callers read rows from SQLCipher and pass them in.
// It replaced the DuckDB engine on 2026-11-02; see
// docs/review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md and the parity
// record in docs/testing/duckdb-retirement-parity.md.
package analytics

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

// Dataset is a tabular result from ImportTabular. Rows are row-major. A
// cell is nil (empty), a bool, an int64, a float64 or a string; values that
// JSON or JavaScript cannot carry exactly (integers beyond 2^53, decimals,
// dates, nested values, NaN) are strings.
type Dataset struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
	// Truncated is true when the file holds more than MaxImportRows rows and
	// only the first MaxImportRows were read.
	Truncated bool `json:"truncated"`
}
