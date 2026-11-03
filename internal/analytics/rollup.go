// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"errors"
	"fmt"
	"math/bits"

	"gopmgr/internal/money"
)

// ErrRollupOverflow reports a portfolio total that does not fit in int64
// minor units. Totals accumulate in 128 bits, so only a final total that is
// out of range fails, not an intermediate sum.
var ErrRollupOverflow = errors.New("analytics: portfolio total exceeds the int64 minor-unit range")

// acc128 is a signed 128-bit accumulator.
type acc128 struct {
	hi int64
	lo uint64
}

func (a *acc128) add(v int64) {
	var carry uint64
	a.lo, carry = bits.Add64(a.lo, uint64(v), 0)
	a.hi += v>>63 + int64(carry) // v>>63 sign-extends v into the high word
}

func (a acc128) int64() (int64, bool) {
	r := int64(a.lo)
	return r, a.hi == r>>63
}

// minorUnits prefers the exact minor-unit field and falls back to the
// legacy float only when the minor-unit field is unset.
func minorUnits(minor int64, major float64) int64 {
	if minor == 0 && major != 0 {
		return money.FromMajorFloat(major).MinorUnits
	}
	return minor
}

// PortfolioRollup totals per-project figures. Budgeted and committed cost
// count for every project; actual cost, earned value and planned value
// count only for projects whose schedule supports EVM. Index fields are 0
// when undefined (zero denominator), matching the kernel's convention.
func PortfolioRollup(projects []ProjectMetrics) (PortfolioSummary, error) {
	var s PortfolioSummary
	var accs [5]acc128
	names := [5]string{"budgeted cost", "committed cost", "actual cost", "earned value", "planned value"}
	for _, p := range projects {
		vals := [5]int64{
			minorUnits(p.BudgetedCostMinorUnits, p.BudgetedCost),
			minorUnits(p.CommittedCostMinorUnits, p.CommittedCost),
			minorUnits(p.ActualCostMinorUnits, p.ActualCost),
			minorUnits(p.EarnedValueMinorUnits, p.EarnedValue),
			minorUnits(p.PlannedValueMinorUnits, p.PlannedValue),
		}
		s.ProjectCount++
		if p.EVMAvailable {
			s.EVMProjectCount++
		}
		for f := range vals {
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
			return PortfolioSummary{}, fmt.Errorf("%w: %s", ErrRollupOverflow, names[f])
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
