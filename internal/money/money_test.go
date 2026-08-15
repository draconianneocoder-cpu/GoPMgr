// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package money

import (
	"errors"
	"math"
	"testing"
)

func TestAmountFromMajorFloatRoundsToMinorUnits(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want int64
	}{
		{name: "whole dollars", in: 42, want: 4200},
		{name: "fractional cents round up", in: 10.235, want: 1024},
		{name: "negative refunds", in: -7.255, want: -726},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromMajorFloat(tc.in).MinorUnits; got != tc.want {
				t.Fatalf("minor units = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRateTimesQuantityUsesExactRationalRounding(t *testing.T) {
	rate := Amount{MinorUnits: 3333} // 33.33/hour

	got := RateTimesQuantity(rate, 1.5)
	if got.MinorUnits != 5000 {
		t.Fatalf("1.5h at 33.33 = %d cents, want 5000", got.MinorUnits)
	}

	thirdHour := RateTimesQuantity(Amount{MinorUnits: 1000}, 1.0/3.0)
	if thirdHour.MinorUnits != 333 {
		t.Fatalf("1/3h at 10.00 = %d cents, want 333", thirdHour.MinorUnits)
	}
}

func TestAmountHandlesNegativeValues(t *testing.T) {
	refund := Amount{MinorUnits: -2500}
	charge := Amount{MinorUnits: 1000}

	got, err := charge.Add(refund)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinorUnits != -1500 {
		t.Fatalf("add refund = %d, want -1500", got)
	}
	got, err = charge.Sub(refund)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinorUnits != 3500 {
		t.Fatalf("subtract refund = %d, want 3500", got)
	}
}

func TestAmountAddSubOverflow(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (Amount, error)
	}{
		{"add positive", func() (Amount, error) { return Amount{MinorUnits: math.MaxInt64}.Add(Amount{MinorUnits: 1}) }},
		{"add negative", func() (Amount, error) { return Amount{MinorUnits: math.MinInt64}.Add(Amount{MinorUnits: -1}) }},
		{"subtract positive", func() (Amount, error) { return Amount{MinorUnits: math.MaxInt64}.Sub(Amount{MinorUnits: -1}) }},
		{"subtract minimum", func() (Amount, error) { return Amount{}.Sub(Amount{MinorUnits: math.MinInt64}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn()
			if !errors.Is(err, ErrOverflow) {
				t.Fatalf("error = %v, want ErrOverflow", err)
			}
		})
	}
}

func TestAccumulatorIsOrderIndependent(t *testing.T) {
	for _, amounts := range [][]Amount{
		{{MinorUnits: math.MaxInt64}, {MinorUnits: 1}, {MinorUnits: -1}},
		{{MinorUnits: 1}, {MinorUnits: -1}, {MinorUnits: math.MaxInt64}},
		{{MinorUnits: -1}, {MinorUnits: math.MaxInt64}, {MinorUnits: 1}},
	} {
		var total Accumulator
		for _, amount := range amounts {
			total.Add(amount)
		}
		got, err := total.Amount()
		if err != nil {
			t.Fatal(err)
		}
		if got.MinorUnits != math.MaxInt64 {
			t.Fatalf("total = %d, want %d", got.MinorUnits, int64(math.MaxInt64))
		}
	}
}
