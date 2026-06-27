// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package money

import "testing"

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

	if got := charge.Add(refund).MinorUnits; got != -1500 {
		t.Fatalf("add refund = %d, want -1500", got)
	}
	if got := charge.Sub(refund).MinorUnits; got != 3500 {
		t.Fatalf("subtract refund = %d, want 3500", got)
	}
}
