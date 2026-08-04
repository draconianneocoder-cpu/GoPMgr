// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

// Tests for the DuckDB-backed engine. Run with: go test -tags duckdb ./internal/analytics/...
package analytics

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDuckEngineAvailable(t *testing.T) {
	e := New()
	t.Cleanup(func() { _ = e.Close() })
	if !e.Available() {
		t.Fatal("duckdb build: Available() should be true")
	}
}

func TestPortfolioRollupAggregates(t *testing.T) {
	e := New()
	t.Cleanup(func() { _ = e.Close() })

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

	got, err := e.PortfolioRollup(context.Background(), in)
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
	e := New()
	t.Cleanup(func() { _ = e.Close() })

	got, err := e.PortfolioRollup(context.Background(), []ProjectMetrics{{
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
	e := New()
	t.Cleanup(func() { _ = e.Close() })

	got, err := e.PortfolioRollup(context.Background(), nil)
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
	e := New()
	t.Cleanup(func() { _ = e.Close() })

	one := []ProjectMetrics{{ProjectID: "a", BudgetedCost: 100}}
	if _, err := e.PortfolioRollup(context.Background(), one); err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	got, err := e.PortfolioRollup(context.Background(), one)
	if err != nil {
		t.Fatalf("second rollup: %v", err)
	}
	if got.ProjectCount != 1 || got.TotalBudgetedCost != 100 {
		t.Errorf("second rollup not fresh: %+v", got)
	}
}

func TestImportTabularCSV(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(p, []byte("a,b\n1,x\n2,y\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	e := New()
	t.Cleanup(func() { _ = e.Close() })

	ds, err := e.ImportTabular(context.Background(), p)
	if err != nil {
		t.Fatalf("ImportTabular: %v", err)
	}
	if len(ds.Columns) != 2 || ds.Columns[0] != "a" || ds.Columns[1] != "b" {
		t.Errorf("Columns = %v, want [a b]", ds.Columns)
	}
	if len(ds.Rows) != 2 {
		t.Errorf("Rows = %d, want 2", len(ds.Rows))
	}
}

func TestImportTabularRejectsUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.xlsx")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := New()
	t.Cleanup(func() { _ = e.Close() })
	if _, err := e.ImportTabular(context.Background(), p); err == nil {
		t.Fatal("expected an error for unsupported .xlsx, got nil")
	}
}

func TestImportTabularMissingFile(t *testing.T) {
	e := New()
	t.Cleanup(func() { _ = e.Close() })
	if _, err := e.ImportTabular(context.Background(), filepath.Join(t.TempDir(), "nope.csv")); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}
