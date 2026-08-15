// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package money provides exact monetary arithmetic for GoPMgr.
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
//
// A magnitude too large to fit in int64 once scaled to minor units
// (e.g. a legacy import with transposed major/minor units) saturates
// to math.MaxInt64/math.MinInt64 by sign rather than relying on Go's
// float64->int64 conversion, which is documented as
// implementation-defined once the value no longer fits -- this makes
// the saturating behavior a guarantee of this function rather than an
// artifact of the current toolchain/architecture. This is a real
// behavior change on amd64 (this repo's CI target): amd64's prior
// implementation-defined conversion has been observed to produce a
// single sign-losing "integer indefinite" value for either direction,
// so the sign-preserving clamp here is strictly more correct there,
// not merely a portability no-op.
func FromMajorFloat(v float64) Amount {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Amount{}
	}
	scaled := math.Round(v * float64(MinorUnitsPerMajor))
	switch {
	case scaled >= float64(math.MaxInt64):
		return Amount{MinorUnits: math.MaxInt64}
	case scaled <= float64(math.MinInt64):
		return Amount{MinorUnits: math.MinInt64}
	default:
		return Amount{MinorUnits: int64(scaled)}
	}
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
// quantity exactly and rounds half away from zero to minor units. A
// rounded result outside int64 saturates by sign.
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
// zero because the caller has no valid ratio. A rounded result outside
// int64 saturates by sign.
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

// roundRat rounds half away from zero and saturates a result that cannot fit
// in Amount's int64 minor-unit representation. The range check happens after
// rounding so a half-unit at either endpoint has the same result as any other
// exact monetary calculation.
func roundRat(r *big.Rat) int64 {
	n := new(big.Int).Set(r.Num())
	d := new(big.Int).Set(r.Denom())
	q, rem := new(big.Int).QuoRem(n, d, new(big.Int))
	if rem.Sign() == 0 {
		return saturatingInt64(q)
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
	return saturatingInt64(q)
}

func saturatingInt64(q *big.Int) int64 {
	if !q.IsInt64() {
		if q.Sign() < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return q.Int64()
}
