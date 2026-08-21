// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"gopmgr/internal/analytics"
	"gopmgr/internal/money"
)

// PortfolioSummaryWire is the Wails-safe portfolio rollup boundary. Monetary
// totals are canonical decimal strings, so the renderer cannot round a large
// aggregate or derive a money value with JavaScript arithmetic.
type PortfolioSummaryWire struct {
	ProjectCount               int     `json:"project_count"`
	EVMProjectCount            int     `json:"evm_project_count"`
	EVMUnavailableProjectCount int     `json:"evm_unavailable_project_count"`
	AsOfDate                   string  `json:"as_of_date"`
	CurrencyCode               string  `json:"currency_code"`
	TotalBudgetedCost          string  `json:"total_budgeted_cost"`
	TotalCommittedCost         string  `json:"total_committed_cost"`
	TotalActualCost            string  `json:"total_actual_cost"`
	TotalEarnedValue           string  `json:"total_earned_value"`
	TotalPlannedValue          string  `json:"total_planned_value"`
	Remaining                  string  `json:"remaining"`
	SchedulePerformanceIndex   float64 `json:"schedule_performance_index"`
	CostPerformanceIndex       float64 `json:"cost_performance_index"`
}

func portfolioSummaryWire(summary analytics.PortfolioSummary, currencyCode string) (PortfolioSummaryWire, error) {
	budget := money.Amount{MinorUnits: summary.TotalBudgetedCostMinorUnits}
	committed := money.Amount{MinorUnits: summary.TotalCommittedCostMinorUnits}
	remaining, err := budget.Sub(committed)
	if err != nil {
		return PortfolioSummaryWire{}, fmt.Errorf("portfolio remaining: %w", err)
	}
	return PortfolioSummaryWire{
		ProjectCount:               summary.ProjectCount,
		EVMProjectCount:            summary.EVMProjectCount,
		EVMUnavailableProjectCount: summary.EVMUnavailableProjectCount,
		AsOfDate:                   summary.AsOfDate,
		CurrencyCode:               currencyCode,
		TotalBudgetedCost:          budget.Decimal(),
		TotalCommittedCost:         committed.Decimal(),
		TotalActualCost:            money.Amount{MinorUnits: summary.TotalActualCostMinorUnits}.Decimal(),
		TotalEarnedValue:           money.Amount{MinorUnits: summary.TotalEarnedValueMinorUnits}.Decimal(),
		TotalPlannedValue:          money.Amount{MinorUnits: summary.TotalPlannedValueMinorUnits}.Decimal(),
		Remaining:                  remaining.Decimal(),
		SchedulePerformanceIndex:   summary.SchedulePerformanceIndex,
		CostPerformanceIndex:       summary.CostPerformanceIndex,
	}, nil
}
