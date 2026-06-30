// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package money provides exact monetary arithmetic for PMForge.
//
// Money is stored as integer minor units (cents for USD-style
// currencies). Calculations that combine rates and fractional effort use
// math/big.Rat, then round once at the boundary back to minor units.
package money

import (
	"math"
	"math/big"
)

const MinorUnitsPerMajor int64 = 100

// Amount is a signed monetary value in minor units.
type Amount struct {
	MinorUnits int64
}

// FromMajorFloat converts a UI/database compatibility number such as
// 12.34 into exact minor units. The rest of the application should use
// Amount for arithmetic; this adapter exists only at legacy boundaries.
func FromMajorFloat(v float64) Amount {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Amount{}
	}
	return Amount{MinorUnits: int64(math.Round(v * float64(MinorUnitsPerMajor)))}
}

// MajorFloat returns a display/compatibility number. Do not use the
// returned value for monetary arithmetic.
func (a Amount) MajorFloat() float64 {
	return float64(a.MinorUnits) / float64(MinorUnitsPerMajor)
}

func (a Amount) Add(b Amount) Amount {
	return Amount{MinorUnits: a.MinorUnits + b.MinorUnits}
}

func (a Amount) Sub(b Amount) Amount {
	return Amount{MinorUnits: a.MinorUnits - b.MinorUnits}
}

func (a Amount) Positive() bool {
	return a.MinorUnits > 0
}

// RateTimesQuantity multiplies a monetary rate by a fractional
// quantity exactly and rounds half away from zero to minor units.
func RateTimesQuantity(rate Amount, quantity float64) Amount {
	if rate.MinorUnits == 0 || quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
		return Amount{}
	}
	q := new(big.Rat).SetFloat64(quantity)
	if q == nil {
		return Amount{}
	}
	value := new(big.Rat).Mul(big.NewRat(rate.MinorUnits, 1), q)
	return Amount{MinorUnits: roundRat(value)}
}

// ScaleByRatio multiplies amount by numerator/denominator exactly and
// rounds half away from zero to minor units. A zero denominator returns
// zero because the caller has no valid ratio.
func ScaleByRatio(amount Amount, numerator, denominator int64) Amount {
	if amount.MinorUnits == 0 || numerator == 0 || denominator == 0 {
		return Amount{}
	}
	value := new(big.Rat).Mul(
		big.NewRat(amount.MinorUnits, 1),
		big.NewRat(numerator, denominator),
	)
	return Amount{MinorUnits: roundRat(value)}
}

func roundRat(r *big.Rat) int64 {
	n := new(big.Int).Set(r.Num())
	d := new(big.Int).Set(r.Denom())
	q, rem := new(big.Int).QuoRem(n, d, new(big.Int))
	if rem.Sign() == 0 {
		return q.Int64()
	}

	absRem := new(big.Int).Abs(rem)
	absRem.Mul(absRem, big.NewInt(2))
	if absRem.Cmp(d) >= 0 {
		if r.Sign() >= 0 {
			q.Add(q, big.NewInt(1))
		} else {
			q.Sub(q, big.NewInt(1))
		}
	}
	return q.Int64()
}
