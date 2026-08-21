// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"gopmgr/internal/analytics"
	"gopmgr/internal/money"
)

func TestPortfolioSummaryWireUsesExactDecimalStrings(t *testing.T) {
	wire, err := portfolioSummaryWire(analytics.PortfolioSummary{
		ProjectCount:                 2,
		EVMProjectCount:              1,
		EVMUnavailableProjectCount:   1,
		AsOfDate:                     "2026-08-21",
		TotalBudgetedCostMinorUnits:  math.MaxInt64,
		TotalCommittedCostMinorUnits: math.MaxInt64 - 1,
		TotalActualCostMinorUnits:    math.MinInt64,
		TotalEarnedValueMinorUnits:   -1,
		TotalPlannedValueMinorUnits:  1,
		SchedulePerformanceIndex:     1.25,
		CostPerformanceIndex:         0.75,
	})
	if err != nil {
		t.Fatalf("portfolioSummaryWire: %v", err)
	}
	if wire.TotalBudgetedCost != "92233720368547758.07" || wire.TotalCommittedCost != "92233720368547758.06" || wire.Remaining != "0.01" || wire.TotalActualCost != "-92233720368547758.08" || wire.TotalEarnedValue != "-0.01" || wire.TotalPlannedValue != "0.01" {
		t.Fatalf("unexpected exact portfolio wire: %+v", wire)
	}
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("marshal portfolio wire: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"minor_units", `"total_budgeted_cost":9`, `"total_committed_cost":9`, `"remaining":0`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("portfolio wire exposed numeric money: %s", text)
		}
	}
}

func TestPortfolioSummaryWirePreservesNegativeRemaining(t *testing.T) {
	wire, err := portfolioSummaryWire(analytics.PortfolioSummary{
		TotalBudgetedCostMinorUnits:  1,
		TotalCommittedCostMinorUnits: 2,
	})
	if err != nil {
		t.Fatalf("portfolioSummaryWire: %v", err)
	}
	if wire.Remaining != "-0.01" {
		t.Fatalf("remaining = %q, want -0.01", wire.Remaining)
	}
}

func TestPortfolioSummaryWireRejectsRemainingOverflow(t *testing.T) {
	_, err := portfolioSummaryWire(analytics.PortfolioSummary{
		TotalBudgetedCostMinorUnits:  math.MaxInt64,
		TotalCommittedCostMinorUnits: -1,
	})
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("portfolioSummaryWire error = %v, want %v", err, money.ErrOverflow)
	}
}
