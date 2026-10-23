// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package money provides exact monetary arithmetic for GoPMgr.
//
// Money is stored as integer minor units of the project's reporting currency.
// Most currencies use 100 minor units per major unit (cents). Japanese Yen
// (JPY) uses 1 minor unit per major unit (whole yen) per ISO 4217.
// Calculations that combine rates and fractional effort use math/big.Rat,
// then round once at the boundary back to minor units.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// MinorUnitsPerMajor is the legacy fixed scale used by ParseDecimal and
// Decimal. New code should prefer the scale-aware variants when the
// project's currency is known.
const MinorUnitsPerMajor int64 = 100

// ErrOverflow reports that an exact monetary result cannot be represented in
// Amount's signed int64 minor-unit storage.
var ErrOverflow = errors.New("monetary amount exceeds int64 minor-unit range")

// Amount is a signed monetary value in minor units of its currency.
type Amount struct {
	MinorUnits int64
}

// ExponentForCurrency returns the ISO 4217 minor-unit exponent for a
// supported project reporting currency. JPY is 0 (whole yen). All other
// currently supported currencies are 2.
func ExponentForCurrency(code string) int {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "JPY":
		return 0
	default:
		return 2
	}
}

// ParseDecimal parses an unsigned-or-signed major-unit decimal at the public
// boundary using the legacy two-decimal scale. Prefer ParseDecimalWithScale
// when the project's currency exponent is known.
func ParseDecimal(v string) (Amount, error) {
	return ParseDecimalWithScale(v, 2)
}

// ParseDecimalWithScale parses a major-unit decimal using the given number of
// fractional digits (the currency's exponent). It accepts a whole number or
// up to `scale` fractional digits, rejects alternate spellings that could
// hide a value (whitespace, exponent notation, grouping separators, a leading
// plus, leading zeroes, or negative zero), and returns the exact signed int64
// minor-unit representation.
func ParseDecimalWithScale(v string, scale int) (Amount, error) {
	if scale < 0 || scale > 6 {
		return Amount{}, fmt.Errorf("invalid currency scale %d", scale)
	}
	if v == "" {
		return Amount{}, errors.New("amount is required")
	}
	if strings.TrimSpace(v) != v {
		return Amount{}, errors.New("amount must not contain surrounding whitespace")
	}
	negative := strings.HasPrefix(v, "-")
	if negative {
		v = v[1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && (parts[1] == "" || len(parts[1]) > scale)) {
		return Amount{}, fmt.Errorf("amount must be a decimal with at most %d fractional digits", scale)
	}
	if len(parts[0]) > 1 && parts[0][0] == '0' {
		return Amount{}, errors.New("amount must not contain leading zeroes")
	}
	whole, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return Amount{}, fmt.Errorf("amount: %w", err)
	}
	frac := uint64(0)
	if len(parts) == 2 && scale > 0 {
		fraction := parts[1]
		for len(fraction) < scale {
			fraction += "0"
		}
		frac, err = strconv.ParseUint(fraction, 10, 64)
		if err != nil {
			return Amount{}, fmt.Errorf("amount: %w", err)
		}
	}
	multiplier := uint64(1)
	for i := 0; i < scale; i++ {
		multiplier *= 10
	}
	limit := uint64(math.MaxInt64)
	if negative {
		limit++ // absolute value of math.MinInt64
	}
	if whole > (limit-frac)/multiplier {
		return Amount{}, ErrOverflow
	}
	minor := whole*multiplier + frac
	if minor == 0 && negative {
		return Amount{}, errors.New("amount must not be negative zero")
	}
	if negative {
		if minor == uint64(math.MaxInt64)+1 {
			return Amount{MinorUnits: math.MinInt64}, nil
		}
		return Amount{MinorUnits: -int64(minor)}, nil
	}
	return Amount{MinorUnits: int64(minor)}, nil
}

// Decimal returns a canonical, fixed-two-decimal major-unit representation
// suitable for Wails transport. Prefer DecimalWithScale when the project's
// currency exponent is known. It is never a JavaScript number.
func (a Amount) Decimal() string {
	return a.DecimalWithScale(2)
}

// DecimalWithScale returns a canonical major-unit representation with exactly
// `scale` fractional digits (or no decimal point when scale is 0).
func (a Amount) DecimalWithScale(scale int) string {
	if scale < 0 || scale > 6 {
		scale = 2
	}
	multiplier := int64(1)
	for i := 0; i < scale; i++ {
		multiplier *= 10
	}
	n := a.MinorUnits
	if scale == 0 {
		if n < 0 {
			return fmt.Sprintf("-%d", -n)
		}
		return fmt.Sprintf("%d", n)
	}
	if n < 0 {
		// Dividing first keeps MinInt64 representable; negating it does not.
		return fmt.Sprintf("-%d.%0*d", -(n / multiplier), scale, -(n % multiplier))
	}
	return fmt.Sprintf("%d.%0*d", n/multiplier, scale, n%multiplier)
}

// FromMajorFloat converts a UI/database compatibility number such as
// 12.34 into exact minor units using the legacy two-decimal scale.
// Prefer scale-aware conversion when the currency is known.
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

// MajorFloat returns a display/compatibility number using the legacy
// two-decimal scale. Do not use the returned value for monetary arithmetic.
func (a Amount) MajorFloat() float64 {
	return float64(a.MinorUnits) / float64(MinorUnitsPerMajor)
}

// Add returns the exact sum when it fits in Amount.
func (a Amount) Add(b Amount) (Amount, error) {
	var total Accumulator
	total.Add(a)
	total.Add(b)
	return total.Amount()
}

// Sub returns the exact difference when it fits in Amount.
func (a Amount) Sub(b Amount) (Amount, error) {
	var total Accumulator
	total.Add(a)
	total.Sub(b)
	return total.Amount()
}

func (a Amount) Positive() bool {
	return a.MinorUnits > 0
}

// Accumulator aggregates monetary amounts exactly. It defers the int64 range
// check until Amount is called, so a representable final total does not depend
// on the order in which its terms were added.
//
// Do not copy an Accumulator after first use.
type Accumulator struct {
	total big.Int
}

// Add includes amount in the exact total.
func (a *Accumulator) Add(amount Amount) {
	a.total.Add(&a.total, big.NewInt(amount.MinorUnits))
}

// Sub removes amount from the exact total.
func (a *Accumulator) Sub(amount Amount) {
	a.total.Sub(&a.total, big.NewInt(amount.MinorUnits))
}

// Amount returns the accumulated total when it fits in Amount.
func (a *Accumulator) Amount() (Amount, error) {
	return amountFromBigInt(&a.total)
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

// ScaleByRatioChecked multiplies amount by numerator/denominator exactly and
// rounds half away from zero to minor units. A zero amount or numerator
// returns zero, as does a zero denominator (there is no valid ratio); an
// unrepresentable rounded result returns ErrOverflow instead of saturating.
func ScaleByRatioChecked(amount Amount, numerator, denominator int64) (Amount, error) {
	if amount.MinorUnits == 0 || numerator == 0 || denominator == 0 {
		return Amount{}, nil
	}
	value := new(big.Rat).Mul(
		big.NewRat(amount.MinorUnits, 1),
		big.NewRat(numerator, denominator),
	)
	return amountFromBigInt(roundedRat(value))
}

// roundRat rounds half away from zero and saturates a result that cannot fit
// in Amount's int64 minor-unit representation. The range check happens after
// rounding so a half-unit at either endpoint has the same result as any other
// exact monetary calculation.
func roundRat(r *big.Rat) int64 {
	return saturatingInt64(roundedRat(r))
}

func roundedRat(r *big.Rat) *big.Int {
	n := new(big.Int).Set(r.Num())
	d := new(big.Int).Set(r.Denom())
	q, rem := new(big.Int).QuoRem(n, d, new(big.Int))
	if rem.Sign() == 0 {
		return q
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
	return q
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

func amountFromBigInt(value *big.Int) (Amount, error) {
	if !value.IsInt64() {
		return Amount{}, ErrOverflow
	}
	return Amount{MinorUnits: value.Int64()}, nil
}
