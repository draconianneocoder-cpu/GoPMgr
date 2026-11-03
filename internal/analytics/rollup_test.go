// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"errors"
	"math"
	"testing"
)

func TestPortfolioRollupAggregates(t *testing.T) {

	in := []ProjectMetrics{
		{
			ProjectID: "a", Name: "Alpha", BudgetedCost: 100,
			CommittedCost: 70, ActualCost: 80, EarnedValue: 90, PlannedValue: 100,
			PercentComplete: 90, EVMAvailable: true,
		},
		{
			ProjectID: "b", Name: "Beta", BudgetedCost: 200,
			CommittedCost: 140, ActualCost: 150, EarnedValue: 120, PlannedValue: 100,
			PercentComplete: 60, EVMAvailable: true,
		},
		{
			ProjectID: "c", Name: "No schedule", BudgetedCost: 50,
			CommittedCost: 30, ActualCost: 999, EarnedValue: 999, PlannedValue: 999,
		},
	}

	got, err := PortfolioRollup(in)
	if err != nil {
		t.Fatalf("PortfolioRollup: %v", err)
	}

	if got.ProjectCount != 3 {
		t.Errorf("ProjectCount = %d, want 3", got.ProjectCount)
	}
	if got.EVMProjectCount != 2 || got.EVMUnavailableProjectCount != 1 {
		t.Errorf(
			"EVM coverage = %d available / %d unavailable, want 2 / 1",
			got.EVMProjectCount,
			got.EVMUnavailableProjectCount,
		)
	}
	if got.TotalBudgetedCost != 350 {
		t.Errorf("TotalBudgetedCost = %v, want 350", got.TotalBudgetedCost)
	}
	if got.TotalBudgetedCostMinorUnits != 35000 {
		t.Errorf("TotalBudgetedCostMinorUnits = %d, want 35000", got.TotalBudgetedCostMinorUnits)
	}
	if got.TotalCommittedCost != 240 || got.TotalCommittedCostMinorUnits != 24000 {
		t.Errorf(
			"TotalCommittedCost = %v / %d, want 240 / 24000",
			got.TotalCommittedCost,
			got.TotalCommittedCostMinorUnits,
		)
	}
	if got.TotalActualCost != 230 {
		t.Errorf("TotalActualCost = %v, want 230", got.TotalActualCost)
	}
	if got.TotalEarnedValue != 210 {
		t.Errorf("TotalEarnedValue = %v, want 210", got.TotalEarnedValue)
	}
	if got.TotalPlannedValue != 200 {
		t.Errorf("TotalPlannedValue = %v, want 200", got.TotalPlannedValue)
	}
	if got.TotalPlannedValueMinorUnits != 20000 {
		t.Errorf("TotalPlannedValueMinorUnits = %d, want 20000", got.TotalPlannedValueMinorUnits)
	}
	// SPI = EV/PV = 210/200 = 1.05
	if math.Abs(got.SchedulePerformanceIndex-1.05) > 1e-9 {
		t.Errorf("SPI = %v, want ~1.05", got.SchedulePerformanceIndex)
	}
	// CPI = EV/AC = 210/230 ≈ 0.913043
	if math.Abs(got.CostPerformanceIndex-(210.0/230.0)) > 1e-9 {
		t.Errorf("CPI = %v, want ~0.913", got.CostPerformanceIndex)
	}
}

func TestPortfolioRollupPrefersMinorUnits(t *testing.T) {

	got, err := PortfolioRollup([]ProjectMetrics{{
		ProjectID:               "exact",
		BudgetedCost:            999,
		BudgetedCostMinorUnits:  12345,
		CommittedCost:           999,
		CommittedCostMinorUnits: 1500,
		ActualCost:              999,
		ActualCostMinorUnits:    2345,
		EarnedValueMinorUnits:   3456,
		PlannedValueMinorUnits:  4567,
		EVMAvailable:            true,
	}})
	if err != nil {
		t.Fatalf("PortfolioRollup: %v", err)
	}
	if got.TotalBudgetedCostMinorUnits != 12345 || got.TotalBudgetedCost != 123.45 {
		t.Fatalf("budgeted total = %d / %v, want 12345 / 123.45", got.TotalBudgetedCostMinorUnits, got.TotalBudgetedCost)
	}
	if got.TotalCommittedCostMinorUnits != 1500 || got.TotalCommittedCost != 15 {
		t.Fatalf(
			"committed total = %d / %v, want 1500 / 15",
			got.TotalCommittedCostMinorUnits,
			got.TotalCommittedCost,
		)
	}
	if got.TotalActualCostMinorUnits != 2345 || got.TotalActualCost != 23.45 {
		t.Fatalf("actual total = %d / %v, want 2345 / 23.45", got.TotalActualCostMinorUnits, got.TotalActualCost)
	}
}

func TestPortfolioRollupEmptyIsNeutral(t *testing.T) {

	got, err := PortfolioRollup(nil)
	if err != nil {
		t.Fatalf("empty rollup: %v", err)
	}
	if got.ProjectCount != 0 {
		t.Errorf("ProjectCount = %d, want 0", got.ProjectCount)
	}
	// Indices must be 0 ("n/a"), never NaN/Inf from divide-by-zero.
	if got.SchedulePerformanceIndex != 0 || got.CostPerformanceIndex != 0 {
		t.Errorf("empty indices = (%v, %v), want (0, 0)", got.SchedulePerformanceIndex, got.CostPerformanceIndex)
	}
}

// Re-running on the same engine must start fresh (CREATE OR REPLACE), not
// accumulate rows from the previous call.
func TestPortfolioRollupIsIdempotentPerCall(t *testing.T) {

	one := []ProjectMetrics{{ProjectID: "a", BudgetedCost: 100}}
	if _, err := PortfolioRollup(one); err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	got, err := PortfolioRollup(one)
	if err != nil {
		t.Fatalf("second rollup: %v", err)
	}
	if got.ProjectCount != 1 || got.TotalBudgetedCost != 100 {
		t.Errorf("second rollup not fresh: %+v", got)
	}
}

// TestPortfolioRollupOverflow: a total beyond int64 minor units is an error,
// not a wrapped number, but sums that pass beyond int64 and come back are
// exact, because accumulation is 128-bit.
func TestPortfolioRollupOverflow(t *testing.T) {
	big := ProjectMetrics{BudgetedCostMinorUnits: math.MaxInt64}
	if _, err := PortfolioRollup([]ProjectMetrics{big, {BudgetedCostMinorUnits: 1}}); !errors.Is(err, ErrRollupOverflow) {
		t.Fatalf("overflowing budget total: err = %v, want ErrRollupOverflow", err)
	}
	if _, err := PortfolioRollup([]ProjectMetrics{{CommittedCostMinorUnits: math.MinInt64}, {CommittedCostMinorUnits: -1}}); !errors.Is(err, ErrRollupOverflow) {
		t.Fatalf("underflowing committed total: err = %v, want ErrRollupOverflow", err)
	}
	got, err := PortfolioRollup([]ProjectMetrics{big, big, {BudgetedCostMinorUnits: -math.MaxInt64}, {BudgetedCostMinorUnits: -math.MaxInt64 + 5}})
	if err != nil || got.TotalBudgetedCostMinorUnits != 5 {
		t.Fatalf("intermediate overflow: total = %d, err = %v; want 5", got.TotalBudgetedCostMinorUnits, err)
	}
	// AC/EV/PV of a project without EVM are ignored, even when huge.
	got, err = PortfolioRollup([]ProjectMetrics{{ActualCostMinorUnits: math.MaxInt64}, {ActualCostMinorUnits: math.MaxInt64}})
	if err != nil || got.TotalActualCostMinorUnits != 0 || got.EVMUnavailableProjectCount != 2 {
		t.Fatalf("non-EVM actual cost counted: %+v, %v", got, err)
	}
}
