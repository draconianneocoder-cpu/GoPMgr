// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package rollupwire is the host side of candidate B's portfolio rollup:
// the host normalizes money before the worker sees it (DF-06), and turns
// the worker's exact totals back into the shipped summary type. DF0-B's
// CLI runs and DF0-C's gRPC runs share it.
package rollupwire

import (
	"fmt"
	"strconv"

	"gopmgr/internal/analytics"
	"gopmgr/internal/money"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

// Result is the worker's rollup output.
type Result struct {
	ProjectCount               int      `json:"project_count"`
	EVMProjectCount            int      `json:"evm_project_count"`
	EVMUnavailableProjectCount int      `json:"evm_unavailable_project_count"`
	Totals                     []string `json:"totals"`
	SPI                        float64  `json:"spi"`
	CPI                        float64  `json:"cpi"`
	Error                      string   `json:"error"`
}

// Normalise applies the shipped legacy rule: a zero minor-unit amount with
// a nonzero major-unit float uses the rounded float.
func Normalise(p fixtures.Project) map[string]any {
	leg := func(minor int64, major float64) string {
		if minor == 0 && major != 0 {
			return strconv.FormatInt(money.FromMajorFloat(major).MinorUnits, 10)
		}
		return strconv.FormatInt(minor, 10)
	}
	return map[string]any{
		"budgeted_cost_minor_units":  leg(p.BudgetedCostMinorUnits, p.BudgetedCost),
		"committed_cost_minor_units": leg(p.CommittedCostMinorUnits, p.CommittedCost),
		"actual_cost_minor_units":    leg(p.ActualCostMinorUnits, p.ActualCost),
		"earned_value_minor_units":   leg(p.EarnedValueMinorUnits, p.EarnedValue),
		"planned_value_minor_units":  leg(p.PlannedValueMinorUnits, p.PlannedValue),
		"evm_available":              p.EVMAvailable,
	}
}

// Summary builds the analytics summary the host would return from the
// worker's totals: display floats as minor/100, indices as the worker
// computed them.
func Summary(r Result) (analytics.PortfolioSummary, error) {
	if len(r.Totals) != 5 {
		return analytics.PortfolioSummary{}, fmt.Errorf("want 5 totals, got %d", len(r.Totals))
	}
	var t [5]int64
	for i, s := range r.Totals {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return analytics.PortfolioSummary{}, fmt.Errorf("total %d: %w", i, err)
		}
		t[i] = v
	}
	f := func(m int64) float64 { return money.Amount{MinorUnits: m}.MajorFloat() }
	return analytics.PortfolioSummary{
		ProjectCount: r.ProjectCount, EVMProjectCount: r.EVMProjectCount, EVMUnavailableProjectCount: r.EVMUnavailableProjectCount,
		TotalBudgetedCostMinorUnits: t[0], TotalCommittedCostMinorUnits: t[1], TotalActualCostMinorUnits: t[2],
		TotalEarnedValueMinorUnits: t[3], TotalPlannedValueMinorUnits: t[4],
		TotalBudgetedCost: f(t[0]), TotalCommittedCost: f(t[1]), TotalActualCost: f(t[2]), TotalEarnedValue: f(t[3]), TotalPlannedValue: f(t[4]),
		SchedulePerformanceIndex: r.SPI, CostPerformanceIndex: r.CPI,
	}, nil
}
