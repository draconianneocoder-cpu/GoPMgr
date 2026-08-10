// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package money

import (
	"math"
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

// TestFromMajorFloat_SaturatesRatherThanWrappingOnOverflow pins
// observed (not spec-guaranteed) toolchain behavior, not a property
// FromMajorFloat itself enforces: Go's float64->int64 conversion is
// documented as implementation-defined for out-of-range values, and
// the exact saturated value differs by architecture (arm64's FCVTZS
// saturates toward the input's sign; amd64 has historically produced
// a single "integer indefinite" value for either direction) -- this
// repo's CI runs Go tests on ubuntu-24.04 (amd64), so this assertion
// deliberately checks only that an absurdly large input (e.g. a
// legacy import with transposed major/minor units) saturates to one
// of the two int64 extremes rather than silently wrapping to a small,
// plausible-looking wrong dollar figure; it does not pin which
// extreme, since that's arch-dependent and not something this test
// should fail over. FromMajorFloat performs no bounds validation of
// its own on this input -- it silently "succeeds" on a nonsensical
// multi-quadrillion-dollar value -- disclosed here, not fixed (out of
// this increment's authorized scope).
func TestFromMajorFloat_SaturatesRatherThanWrappingOnOverflow(t *testing.T) {
	for _, in := range []float64{1e18, -1e18} {
		got := FromMajorFloat(in).MinorUnits
		if got != math.MaxInt64 && got != math.MinInt64 {
			t.Errorf("FromMajorFloat(%v).MinorUnits = %d, want math.MaxInt64 or math.MinInt64 (saturation behavior changed)", in, got)
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
