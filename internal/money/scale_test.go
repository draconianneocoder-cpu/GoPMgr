// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package money

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

// TestScaleByRatio_ComputesExactRatio exercises ScaleByRatio's one
// real production call site: internal/kernel/evm.go computes EAC
// (Estimate At Completion) as BAC*(AC/EV), the standard EVM formula.
// A rounding defect here silently misstates the dollar figure GoPMgr
// shows a project manager as the forecast final cost.
func TestScaleByRatio_ComputesExactRatio(t *testing.T) {
	cases := []struct {
		name                   string
		amount                 Amount
		numerator, denominator int64
		wantMinorUnits         int64
	}{
		{"EAC-style whole-number ratio", Amount{MinorUnits: 10000}, 6000, 3000, 20000},
		{"repeating-decimal ratio rounds to nearest cent", Amount{MinorUnits: 1000}, 1, 3, 333},
		{"ratio greater than one", Amount{MinorUnits: 500}, 7, 2, 1750},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScaleByRatio(tc.amount, tc.numerator, tc.denominator)
			if got.MinorUnits != tc.wantMinorUnits {
				t.Fatalf("ScaleByRatio(%v, %d, %d).MinorUnits = %d, want %d",
					tc.amount, tc.numerator, tc.denominator, got.MinorUnits, tc.wantMinorUnits)
			}
		})
	}
}

// TestScaleByRatio_ZeroInputsReturnZeroWithoutPanic covers all three
// of ScaleByRatio's explicit zero guards. denominator==0 is the
// highest-stakes case: without the guard, big.NewRat(n, 0) panics
// ("division by zero"). Note this is a general-purpose-helper
// contract, not a guard the live EAC call site currently depends on:
// internal/kernel/evm.go pre-guards EV==0 itself before calling
// ScaleByRatio at all (`if m.EVMinorUnits > 0 && m.ACMinorUnits > 0`),
// assigning EAC = BAC directly in the else branch. ScaleByRatio must
// still be safe against a zero denominator on its own terms, the same
// way internal/crypto's pdf_cms_test.go tests helpers no current
// caller happens to be able to break.
func TestScaleByRatio_ZeroInputsReturnZeroWithoutPanic(t *testing.T) {
	cases := []struct {
		name                   string
		amount                 Amount
		numerator, denominator int64
	}{
		{"zero amount", Amount{MinorUnits: 0}, 5, 10},
		{"zero numerator", Amount{MinorUnits: 1000}, 0, 10},
		{"zero denominator", Amount{MinorUnits: 1000}, 5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScaleByRatio(tc.amount, tc.numerator, tc.denominator)
			if got.MinorUnits != 0 {
				t.Errorf("ScaleByRatio(%v, %d, %d).MinorUnits = %d, want 0",
					tc.amount, tc.numerator, tc.denominator, got.MinorUnits)
			}
		})
	}
}

func TestScaleByRatioChecked(t *testing.T) {
	got, err := ScaleByRatioChecked(Amount{MinorUnits: 1000}, 1, 3)
	if err != nil || got.MinorUnits != 333 {
		t.Fatalf("ScaleByRatioChecked() = %v, %v; want 333, nil", got, err)
	}
	got, err = ScaleByRatioChecked(Amount{MinorUnits: 1000}, 0, 3)
	if err != nil || got.MinorUnits != 0 {
		t.Fatalf("ScaleByRatioChecked zero numerator = %v, %v; want 0, nil", got, err)
	}
	_, err = ScaleByRatioChecked(Amount{MinorUnits: 2}, math.MaxInt64, 1)
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("ScaleByRatioChecked overflow error = %v, want ErrOverflow", err)
	}
}

// TestFromMajorFloat_RejectsNaNAndInf: a NaN or infinite float (e.g.
// from an upstream 0/0 or overflowing computation feeding into a
// legacy float boundary) must become a defined zero Amount, not
// propagate NaN into int64 minor units via an undefined conversion.
func TestFromMajorFloat_RejectsNaNAndInf(t *testing.T) {
	cases := []struct {
		name string
		in   float64
	}{
		{"NaN", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FromMajorFloat(tc.in)
			if got.MinorUnits != 0 {
				t.Errorf("FromMajorFloat(%v).MinorUnits = %d, want 0", tc.in, got.MinorUnits)
			}
		})
	}
}

// TestRateTimesQuantity_GuardsReturnZero covers every explicit guard
// on RateTimesQuantity's input: a zero rate, a non-positive quantity,
// and a NaN/infinite quantity (the same malformed-float class as
// FromMajorFloat's guard, reachable here via a corrupted or
// division-derived fractional quantity such as percent-complete).
func TestRateTimesQuantity_GuardsReturnZero(t *testing.T) {
	rate := Amount{MinorUnits: 3333}
	cases := []struct {
		name     string
		rate     Amount
		quantity float64
	}{
		{"zero rate", Amount{MinorUnits: 0}, 1.5},
		{"zero quantity", rate, 0},
		{"negative quantity", rate, -1},
		{"NaN quantity", rate, math.NaN()},
		{"infinite quantity", rate, math.Inf(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RateTimesQuantity(tc.rate, tc.quantity)
			if got.MinorUnits != 0 {
				t.Errorf("RateTimesQuantity(%v, %v).MinorUnits = %d, want 0", tc.rate, tc.quantity, got.MinorUnits)
			}
		})
	}
}

// TestMajorFloat_RoundTripsWithFromMajorFloat pins the invariant the
// ~25 call sites across budget/db/analytics/kernel actually depend
// on: converting a two-decimal display value to minor units and back
// must reproduce the same minor-units value. Compares minor units
// after a double round-trip rather than raw floats, since float
// equality on a division result is not a meaningful assertion.
func TestMajorFloat_RoundTripsWithFromMajorFloat(t *testing.T) {
	for _, major := range []float64{0, 12.34, -7.25, 1000000.01, 0.01, -0.01} {
		minor := FromMajorFloat(major).MinorUnits
		roundTripped := FromMajorFloat(Amount{MinorUnits: minor}.MajorFloat()).MinorUnits
		if roundTripped != minor {
			t.Errorf("FromMajorFloat(%v).MajorFloat() round-trip = %d minor units, want %d", major, roundTripped, minor)
		}
	}
}

func TestDecimalRoundTripsFullInt64Range(t *testing.T) {
	for _, minorUnits := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		amount := Amount{MinorUnits: minorUnits}
		parsed, err := ParseDecimal(amount.Decimal())
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", amount.Decimal(), err)
		}
		if parsed != amount {
			t.Fatalf("ParseDecimal(%q) = %+v, want %+v", amount.Decimal(), parsed, amount)
		}
	}
}

func TestParseDecimalRejectsAmbiguousOrUnsafeInput(t *testing.T) {
	for _, input := range []string{
		"", " 1", "1 ", "+1", "01", "01.00", "1.", ".1", "1.234",
		"1e2", "1,000", "-0", "-0.00", "92233720368547758.08",
	} {
		if _, err := ParseDecimal(input); err == nil {
			t.Errorf("ParseDecimal(%q) accepted invalid input", input)
		}
	}
}

// TestFromMajorFloat_SaturatesRatherThanWrappingOnOverflow previously
// pinned only that an overflowing input saturates to ONE OF the two
// int64 extremes, not which one -- because that behavior was inherited
// from Go's float64->int64 conversion, documented as
// implementation-defined once the value no longer fits, and observed
// to differ by architecture (arm64's FCVTZS saturates toward the
// input's sign; amd64 has historically produced a single "integer
// indefinite" value for either direction). FromMajorFloat now clamps
// explicitly in Go code instead of relying on that conversion, so the
// direction is a guarantee of this function, not an artifact of the
// toolchain -- this test was strengthened to pin it, rather than left
// unpinned or deleted. A legacy import with transposed major/minor
// units (or any other absurdly large input) now deterministically
// saturates to math.MaxInt64/math.MinInt64 by sign, on every
// architecture this package runs on.
func TestFromMajorFloat_SaturatesRatherThanWrappingOnOverflow(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{1e18, math.MaxInt64},
		{-1e18, math.MinInt64},
	}
	for _, tc := range cases {
		got := FromMajorFloat(tc.in).MinorUnits
		if got != tc.want {
			t.Errorf("FromMajorFloat(%v).MinorUnits = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestPositive_ZeroIsNotPositive pins the boundary internal/budget's
// cost rollup relies on (money.Amount.Positive gates whether a
// contract/rate is included in ByCategory totals): exactly zero must
// be excluded, not included as a no-op positive value.
func TestPositive_ZeroIsNotPositive(t *testing.T) {
	cases := []struct {
		name  string
		minor int64
		want  bool
	}{
		{"negative", -100, false},
		{"zero", 0, false},
		{"positive", 100, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Amount{MinorUnits: tc.minor}.Positive()
			if got != tc.want {
				t.Errorf("Amount{%d}.Positive() = %v, want %v", tc.minor, got, tc.want)
			}
		})
	}
}

// TestRateTimesQuantity_NegativeRateRoundsAwayFromZeroAtTie exercises
// roundRat's negative-quotient branch, which every existing test in
// this package (including RateTimesQuantity's own) only reaches via
// positive values. A negative rate is not validated against at this
// layer -- internal/db/stakeholders.go's HourlyRate is a
// user/import-supplied float with no non-negative constraint visible
// at the money-package boundary, so this is a general-purpose
// contract this package must uphold on any input it's given, not a
// scenario the UI is known to expose today.
func TestRateTimesQuantity_NegativeRateRoundsAwayFromZeroAtTie(t *testing.T) {
	rate := Amount{MinorUnits: -3333} // -$33.33/hour
	got := RateTimesQuantity(rate, 1.5)
	// -$33.33 * 1.5h = -$49.995 = -4999.5 minor units, a half-cent
	// tie; away-from-zero means the more-negative neighbor, -$50.00
	// (-5000 minor units) -- symmetric with the existing positive-tie
	// case (33.33 * 1.5 = 5000, not 4999).
	const want = -5000
	if got.MinorUnits != want {
		t.Fatalf("RateTimesQuantity(-33.33/hr, 1.5h).MinorUnits = %d, want %d (half-cent tie must round away from zero)", got.MinorUnits, want)
	}
}

func TestRoundRat_ClampsAfterRoundingOutsideInt64(t *testing.T) {
	two := big.NewInt(2)
	cases := []struct {
		name string
		num  *big.Int
		want int64
	}{
		{
			name: "positive half unit at maximum still rounds to maximum",
			num:  new(big.Int).Sub(new(big.Int).Mul(big.NewInt(math.MaxInt64), two), big.NewInt(1)),
			want: math.MaxInt64,
		},
		{
			name: "positive half unit past maximum clamps",
			num:  new(big.Int).Add(new(big.Int).Mul(big.NewInt(math.MaxInt64), two), big.NewInt(1)),
			want: math.MaxInt64,
		},
		{
			name: "negative half unit at minimum still rounds to minimum",
			num:  new(big.Int).Add(new(big.Int).Mul(big.NewInt(math.MinInt64), two), big.NewInt(1)),
			want: math.MinInt64,
		},
		{
			name: "negative half unit past minimum clamps",
			num:  new(big.Int).Sub(new(big.Int).Mul(big.NewInt(math.MinInt64), two), big.NewInt(1)),
			want: math.MinInt64,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roundRat(new(big.Rat).SetFrac(tc.num, two)); got != tc.want {
				t.Fatalf("roundRat(%s/2) = %d, want %d", tc.num, got, tc.want)
			}
		})
	}
}

func TestRationalArithmetic_ClampsOverflowingResults(t *testing.T) {
	cases := []struct {
		name      string
		calculate func() Amount
		want      int64
	}{
		{
			name: "rate times quantity positive overflow",
			calculate: func() Amount {
				return RateTimesQuantity(Amount{MinorUnits: math.MaxInt64}, 2)
			},
			want: math.MaxInt64,
		},
		{
			name: "rate times quantity negative overflow",
			calculate: func() Amount {
				return RateTimesQuantity(Amount{MinorUnits: math.MinInt64}, 2)
			},
			want: math.MinInt64,
		},
		{
			name: "rate times quantity exact maximum",
			calculate: func() Amount {
				return RateTimesQuantity(Amount{MinorUnits: math.MaxInt64}, 1)
			},
			want: math.MaxInt64,
		},
		{
			name: "rate times quantity exact minimum",
			calculate: func() Amount {
				return RateTimesQuantity(Amount{MinorUnits: math.MinInt64}, 1)
			},
			want: math.MinInt64,
		},
		{
			name: "scale ratio exact maximum",
			calculate: func() Amount {
				return ScaleByRatio(Amount{MinorUnits: math.MaxInt64}, 1, 1)
			},
			want: math.MaxInt64,
		},
		{
			name: "scale ratio exact minimum",
			calculate: func() Amount {
				return ScaleByRatio(Amount{MinorUnits: math.MinInt64}, 1, 1)
			},
			want: math.MinInt64,
		},
		{
			name: "scale ratio positive overflow",
			calculate: func() Amount {
				return ScaleByRatio(Amount{MinorUnits: math.MaxInt64}, 2, 1)
			},
			want: math.MaxInt64,
		},
		{
			name: "scale ratio negative overflow",
			calculate: func() Amount {
				return ScaleByRatio(Amount{MinorUnits: math.MinInt64}, 2, 1)
			},
			want: math.MinInt64,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.calculate().MinorUnits; got != tc.want {
				t.Fatalf("minor units = %d, want %d", got, tc.want)
			}
		})
	}
}
