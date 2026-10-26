// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package gorollup

import (
	"errors"
	"math"
	"testing"

	"gopmgr/internal/analytics"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		a, b int64
		ok   bool
	}{
		{1, 2, true}, {math.MaxInt64, 0, true}, {math.MaxInt64, 1, false}, {math.MinInt64, -1, false},
		{math.MinInt64, math.MaxInt64, true}, {-5, 3, true}, {math.MaxInt64, math.MinInt64, true},
	}
	for _, tt := range tests {
		if got, ok := add(tt.a, tt.b); ok != tt.ok || (ok && got != tt.a+tt.b) {
			t.Errorf("add(%d, %d) = %d, %v", tt.a, tt.b, got, ok)
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
