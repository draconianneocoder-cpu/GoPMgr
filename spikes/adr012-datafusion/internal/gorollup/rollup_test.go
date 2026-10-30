// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package gorollup

import (
	"errors"
	"math"
	"testing"

	"gopmgr/internal/analytics"
)

func TestAcc128(t *testing.T) {
	tests := []struct {
		name string
		in   []int64
		want int64
		ok   bool
	}{
		{"small", []int64{1, 2, -5}, -2, true},
		{"max", []int64{math.MaxInt64, 0}, math.MaxInt64, true},
		{"above max", []int64{math.MaxInt64, 1}, 0, false},
		{"below min", []int64{math.MinInt64, -1}, 0, false},
		{"min plus max", []int64{math.MinInt64, math.MaxInt64}, -1, true},
		{"cancels after overflow", []int64{math.MaxInt64, 1, -1}, math.MaxInt64, true},
		{"cancels after underflow", []int64{math.MinInt64, -1, 1}, math.MinInt64, true},
		{"far above max then back", []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MinInt64, math.MinInt64, math.MinInt64}, -3, true},
		{"min exactly", []int64{math.MinInt64 + 1, -1}, math.MinInt64, true},
		{"two mins", []int64{math.MinInt64, math.MinInt64}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a acc128
			for _, v := range tt.in {
				a.add(v)
			}
			got, ok := a.int64()
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("sum %v = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestRollupFinalTotalOnly pins the Codex review case on #153: an
// intermediate total outside int64 is fine when the final total fits, in
// every order, as in the oracle.
func TestRollupFinalTotalOnly(t *testing.T) {
	orders := [][]int64{
		{math.MaxInt64, 1, -1},
		{1, math.MaxInt64, -1},
		{-1, 1, math.MaxInt64},
	}
	for _, order := range orders {
		ps := make([]analytics.ProjectMetrics, len(order))
		for i, v := range order {
			ps[i] = analytics.ProjectMetrics{BudgetedCostMinorUnits: v}
		}
		got, err := Rollup(ps)
		if err != nil {
			t.Fatalf("%v: %v", order, err)
		}
		if got.TotalBudgetedCostMinorUnits != math.MaxInt64 {
			t.Errorf("%v: total = %d, want MaxInt64", order, got.TotalBudgetedCostMinorUnits)
		}
	}
}

func TestRollupOverflowIsTyped(t *testing.T) {
	_, err := Rollup([]analytics.ProjectMetrics{{BudgetedCostMinorUnits: math.MaxInt64}, {BudgetedCostMinorUnits: 1}})
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
	// An overflowing AC on a project without EVM is excluded, as shipped.
	if _, err := Rollup([]analytics.ProjectMetrics{{ActualCostMinorUnits: math.MaxInt64}, {ActualCostMinorUnits: math.MaxInt64}}); err != nil {
		t.Fatalf("excluded AC overflowed: %v", err)
	}
}
