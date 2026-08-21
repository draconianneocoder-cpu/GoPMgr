// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"gopmgr/internal/budget"
	"gopmgr/internal/money"
)

// BudgetSummaryWire is the Wails-safe legacy Budget rollup. Every monetary
// value is a canonical decimal string; progress calculations in the frontend
// are display-only and never round-trip into persistence.
type BudgetSummaryWire struct {
	Budget         string            `json:"budget"`
	ContractValue  string            `json:"contract_value"`
	LabourEstimate string            `json:"labour_estimate"`
	Committed      string            `json:"committed"`
	Remaining      string            `json:"remaining"`
	ByCategory     map[string]string `json:"by_category"`
}

func budgetSummaryWire(summary budget.Summary) BudgetSummaryWire {
	byCategory := make(map[string]string, len(summary.ByCategoryMinorUnits))
	for category, minorUnits := range summary.ByCategoryMinorUnits {
		byCategory[category] = money.Amount{MinorUnits: minorUnits}.Decimal()
	}
	return BudgetSummaryWire{
		Budget:         money.Amount{MinorUnits: summary.BudgetMinorUnits}.Decimal(),
		ContractValue:  money.Amount{MinorUnits: summary.ContractValueMinorUnits}.Decimal(),
		LabourEstimate: money.Amount{MinorUnits: summary.LabourEstimateMinorUnits}.Decimal(),
		Committed:      money.Amount{MinorUnits: summary.CommittedMinorUnits}.Decimal(),
		Remaining:      money.Amount{MinorUnits: summary.RemainingMinorUnits}.Decimal(),
		ByCategory:     byCategory,
	}
}
