// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package oracle computes the expected portfolio rollup independently of any
// engine: exact big-integer sums, exact rational indices, and the legacy
// float-fallback rule written from its specification rather than by calling
// gopmgr/internal/money. Two verdicts are kept apart (ADR-012 DF-A05):
//
//   - Legacy is what the shipped rollup is specified to return, including
//     its float fallback, saturation and zero-as-absent inference.
//   - Correctness reports whether that answer is financially well-defined:
//     inputs that are ambiguous (a zero minor-unit amount beside a nonzero
//     float) or invalid (non-finite or saturated floats) are flagged instead
//     of silently trusted.
package oracle

import (
	"math"
	"math/big"
	"sort"

	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

// Field indexes the five monetary totals.
const (
	Budgeted = iota
	Committed
	Actual
	Earned
	Planned
	numFields
)

var FieldNames = [numFields]string{"budgeted_cost", "committed_cost", "actual_cost", "earned_value", "planned_value"}

// Summary is the expected rollup with exact totals.
type Summary struct {
	ProjectCount               int
	EVMProjectCount            int
	EVMUnavailableProjectCount int
	Totals                     [numFields]*big.Int // minor units, exact
	// SPI and CPI are the float64 nearest the exact ratio ΣEV/ΣPV and
	// ΣEV/ΣAC; 0 when the denominator is zero, as the legacy API reports n/a.
	SPI, CPI               float64
	SPIDefined, CPIDefined bool
}

// Outcome is the legacy result: a summary, or the error class the shipped
// rollup is specified to return.
type Outcome struct {
	Summary *Summary
	// Err is "" or "overflow": a total outside int64 cannot be returned in
	// PortfolioSummary's int64 fields.
	Err string
}

// Correctness lists the inputs that make a legacy total financially
// ambiguous or invalid. An empty value means the totals are exact facts.
type Correctness struct {
	Ambiguous []string // "<project index>.<field>": zero minor units beside a nonzero float
	Invalid   []string // "<project index>.<field>": non-finite or out-of-range float used by the fallback
}

// Exact reports whether the case has no ambiguous or invalid input.
func (c Correctness) Exact() bool { return len(c.Ambiguous) == 0 && len(c.Invalid) == 0 }

// legacyMinor applies the shipped fallback rule as specified: a field's minor
// units are used unless they are zero and the float is nonzero, in which case
// the float is scaled by 100 and rounded half away from zero; NaN and ±Inf
// become 0; a magnitude beyond int64 saturates to the bound with its sign.
func legacyMinor(minorUnits int64, major float64) (value int64, fallback bool, invalid bool) {
	if minorUnits != 0 || major == 0 {
		return minorUnits, false, false
	}
	if math.IsNaN(major) || math.IsInf(major, 0) {
		return 0, true, true
	}
	// The product is taken in float64, as specified, so a binary value such as
	// 1.005*100 = 100.49999999999999 rounds to 100, not 101.
	r := math.Round(major * 100)
	switch {
	case r >= 9223372036854775807.0:
		return math.MaxInt64, true, true
	case r <= -9223372036854775808.0:
		return math.MinInt64, true, true
	}
	return int64(r), true, false
}

// Rollup computes the expected legacy outcome and the correctness verdict.
func Rollup(projects []fixtures.Project) (Outcome, Correctness) {
	var corr Correctness
	totals := [numFields]*big.Int{}
	for i := range totals {
		totals[i] = new(big.Int)
	}
	s := &Summary{ProjectCount: len(projects)}
	for idx, p := range projects {
		minors := [numFields]int64{p.BudgetedCostMinorUnits, p.CommittedCostMinorUnits, p.ActualCostMinorUnits, p.EarnedValueMinorUnits, p.PlannedValueMinorUnits}
		majors := [numFields]float64{p.BudgetedCost, p.CommittedCost, p.ActualCost, p.EarnedValue, p.PlannedValue}
		if p.EVMAvailable {
			s.EVMProjectCount++
		}
		for f := 0; f < numFields; f++ {
			v, fallback, invalid := legacyMinor(minors[f], majors[f])
			evmField := f == Actual || f == Earned || f == Planned
			if evmField && !p.EVMAvailable {
				continue // excluded from the sum, so its input cannot affect the total
			}
			label := itoa(idx) + "." + FieldNames[f]
			if invalid {
				corr.Invalid = append(corr.Invalid, label)
			} else if fallback {
				corr.Ambiguous = append(corr.Ambiguous, label)
			}
			totals[f].Add(totals[f], big.NewInt(v))
		}
	}
	s.EVMUnavailableProjectCount = s.ProjectCount - s.EVMProjectCount
	s.Totals = totals
	sort.Strings(corr.Ambiguous)
	sort.Strings(corr.Invalid)
	for _, t := range totals {
		if !t.IsInt64() {
			return Outcome{Err: "overflow"}, corr
		}
	}
	s.SPI, s.SPIDefined = ratio(totals[Earned], totals[Planned])
	s.CPI, s.CPIDefined = ratio(totals[Earned], totals[Actual])
	return Outcome{Summary: s}, corr
}

// ratio returns the float64 nearest num/den, or 0 and false for den == 0.
func ratio(num, den *big.Int) (float64, bool) {
	if den.Sign() == 0 {
		return 0, false
	}
	f, _ := new(big.Rat).SetFrac(num, den).Float64()
	return f, true
}

func itoa(i int) string { return big.NewInt(int64(i)).String() }

// ULPDistance is the number of float64 steps between a and b (0 when equal,
// MaxUint64 when either is NaN or the signs differ in a way that matters).
func ULPDistance(a, b float64) uint64 {
	if a == b {
		return 0
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.MaxUint64
	}
	ia, ib := orderedBits(a), orderedBits(b)
	if ia > ib {
		return uint64(ia - ib)
	}
	return uint64(ib - ia)
}

func orderedBits(f float64) int64 {
	b := int64(math.Float64bits(f))
	if b < 0 {
		b = math.MinInt64 - b
	}
	return b
}
