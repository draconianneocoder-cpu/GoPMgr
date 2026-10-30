// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package gorollup is ADR-012 candidate C's portfolio rollup: native Go,
// in the host, with no engine. It keeps the shipped semantics exactly
// (the legacy float fallback, EVM-filtered sums, float64 display totals and
// indices) and replaces the SQL sums with exact 128-bit sums that are range
// checked once at the end, so an out-of-range total is a typed error instead
// of a scan failure. Like the oracle and DuckDB's HUGEINT SUM, only the final
// total must fit: [MaxInt64, 1, -1] sums to MaxInt64 in any order.
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

// acc128 is an exact signed 128-bit accumulator in two's complement. It
// cannot overflow for fewer than 2^63 int64 addends.
type acc128 struct {
	hi int64
	lo uint64
}

// add adds v exactly.
func (a *acc128) add(v int64) {
	var carry uint64
	a.lo, carry = bits.Add64(a.lo, uint64(v), 0)
	a.hi += v>>63 + int64(carry) // v>>63 sign-extends v into the high word
}

// int64 returns the total, or false when it is outside int64.
func (a acc128) int64() (int64, bool) {
	r := int64(a.lo)
	return r, a.hi == r>>63
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
	var accs [5]acc128
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
			accs[f].add(vals[f])
		}
	}
	var totals [5]int64
	for f := range accs {
		var ok bool
		if totals[f], ok = accs[f].int64(); !ok {
			return analytics.PortfolioSummary{}, fmt.Errorf("%w: %s", ErrOverflow, names[f])
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
