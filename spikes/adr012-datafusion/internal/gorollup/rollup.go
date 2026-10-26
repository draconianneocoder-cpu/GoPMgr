// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package gorollup is ADR-012 candidate C's portfolio rollup: native Go,
// in the host, with no engine. It keeps the shipped semantics exactly
// (the legacy float fallback, EVM-filtered sums, float64 display totals and
// indices) and replaces the SQL sums with checked integer addition, so an
// out-of-range total is a typed error instead of a scan failure.
package gorollup

import (
	"errors"
	"fmt"
	"math/bits"

	"gopmgr/internal/analytics"
	"gopmgr/internal/money"
)

// ErrOverflow reports a total outside int64 minor units.
var ErrOverflow = errors.New("portfolio total exceeds int64 minor units")

// add returns a+b, or false when the sum overflows int64.
func add(a, b int64) (int64, bool) {
	s, _ := bits.Add64(uint64(a), uint64(b), 0)
	r := int64(s)
	// Overflow happened when both operands share a sign the result lacks.
	if (a >= 0) == (b >= 0) && (r >= 0) != (a >= 0) {
		return 0, false
	}
	return r, true
}

// legacy applies the shipped normaliseProjectMetricsMoney rule to one field.
func legacy(minor int64, major float64) int64 {
	if minor == 0 && major != 0 {
		return money.FromMajorFloat(major).MinorUnits
	}
	return minor
}

// Rollup computes the portfolio summary.
func Rollup(projects []analytics.ProjectMetrics) (analytics.PortfolioSummary, error) {
	var s analytics.PortfolioSummary
	var totals [5]int64
	names := [5]string{"budgeted_cost", "committed_cost", "actual_cost", "earned_value", "planned_value"}
	for _, p := range projects {
		vals := [5]int64{
			legacy(p.BudgetedCostMinorUnits, p.BudgetedCost),
			legacy(p.CommittedCostMinorUnits, p.CommittedCost),
			legacy(p.ActualCostMinorUnits, p.ActualCost),
			legacy(p.EarnedValueMinorUnits, p.EarnedValue),
			legacy(p.PlannedValueMinorUnits, p.PlannedValue),
		}
		s.ProjectCount++
		if p.EVMAvailable {
			s.EVMProjectCount++
		}
		for f := 0; f < 5; f++ {
			if f >= 2 && !p.EVMAvailable {
				continue // AC, EV and PV count only for projects with EVM
			}
			var ok bool
			if totals[f], ok = add(totals[f], vals[f]); !ok {
				return analytics.PortfolioSummary{}, fmt.Errorf("%w: %s", ErrOverflow, names[f])
			}
		}
	}
	s.EVMUnavailableProjectCount = s.ProjectCount - s.EVMProjectCount
	s.TotalBudgetedCostMinorUnits, s.TotalCommittedCostMinorUnits = totals[0], totals[1]
	s.TotalActualCostMinorUnits, s.TotalEarnedValueMinorUnits, s.TotalPlannedValueMinorUnits = totals[2], totals[3], totals[4]
	s.TotalBudgetedCost = money.Amount{MinorUnits: totals[0]}.MajorFloat()
	s.TotalCommittedCost = money.Amount{MinorUnits: totals[1]}.MajorFloat()
	s.TotalActualCost = money.Amount{MinorUnits: totals[2]}.MajorFloat()
	s.TotalEarnedValue = money.Amount{MinorUnits: totals[3]}.MajorFloat()
	s.TotalPlannedValue = money.Amount{MinorUnits: totals[4]}.MajorFloat()
	if s.TotalPlannedValue != 0 {
		s.SchedulePerformanceIndex = s.TotalEarnedValue / s.TotalPlannedValue
	}
	if s.TotalActualCost != 0 {
		s.CostPerformanceIndex = s.TotalEarnedValue / s.TotalActualCost
	}
	return s, nil
}
