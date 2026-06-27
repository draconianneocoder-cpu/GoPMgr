// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package budget is the cost-rollup engine. It folds two sources
// into one Summary:
//
//   - vendor contract values from the stakeholders table
//   - work-item points × assignee hourly rate from agile (work-item
//     points are interpreted as hours for cost purposes; teams that
//     use a different convention can override the rate per
//     stakeholder)
//
// Output is a snapshot for the Dashboard Budget panel. The package
// is read-only and does no I/O — pass it the pre-fetched records.
package budget

import (
	"pmforge/internal/agile"
	"pmforge/internal/db"
	"pmforge/internal/money"
)

// Summary is the panel-ready cost rollup.
type Summary struct {
	Budget         float64 `json:"budget"`          // project.budget — the cap
	ContractValue  float64 `json:"contract_value"`  // Σ stakeholder.contract_value for vendors
	LabourEstimate float64 `json:"labour_estimate"` // Σ work-item-points × assignee.hourly_rate
	Committed      float64 `json:"committed"`       // contract_value + labour_estimate
	Remaining      float64 `json:"remaining"`       // budget - committed (negative if over)

	BudgetMinorUnits         int64              `json:"budget_minor_units"`
	ContractValueMinorUnits  int64              `json:"contract_value_minor_units"`
	LabourEstimateMinorUnits int64              `json:"labour_estimate_minor_units"`
	CommittedMinorUnits      int64              `json:"committed_minor_units"`
	RemainingMinorUnits      int64              `json:"remaining_minor_units"`
	ByCategoryMinorUnits     map[string]int64   `json:"by_category_minor_units"`
	ByCategory               map[string]float64 `json:"by_category"` // breakdown by stakeholder category
}

// Compute walks the inputs and produces a Summary. Stakeholder is
// the lookup table; we index it by ID before scanning work items so
// the rollup is O(workItems + stakeholders).
func Compute(project db.Project, stakeholders []db.Stakeholder, workItems []agile.WorkItem) Summary {
	budget := amountFromProject(project)
	sum := Summary{
		BudgetMinorUnits:     budget.MinorUnits,
		ByCategory:           map[string]float64{},
		ByCategoryMinorUnits: map[string]int64{},
	}

	for _, s := range stakeholders {
		// Vendor / contract values roll up directly.
		contract := amountFromContractValue(s)
		if contract.Positive() {
			sum.ContractValueMinorUnits += contract.MinorUnits
			addCategory(&sum, string(s.Category), contract)
		}
	}

	// Labour estimate: points × rate. Work items are matched to
	// stakeholders by Assignee field; if the assignee string equals
	// a stakeholder's name (case-insensitive), apply that rate.
	rateByName := make(map[string]money.Amount, len(stakeholders))
	catByName := make(map[string]string, len(stakeholders))
	for _, s := range stakeholders {
		rate := amountFromHourlyRate(s)
		if rate.Positive() {
			rateByName[lower(s.Name)] = rate
			catByName[lower(s.Name)] = string(s.Category)
		}
	}
	for _, wi := range workItems {
		if wi.Assignee == "" || wi.Points <= 0 {
			continue
		}
		rate, ok := rateByName[lower(wi.Assignee)]
		if !ok {
			continue
		}
		cost := money.RateTimesQuantity(rate, wi.Points)
		sum.LabourEstimateMinorUnits += cost.MinorUnits
		if cat := catByName[lower(wi.Assignee)]; cat != "" {
			addCategory(&sum, cat, cost)
		}
	}

	sum.CommittedMinorUnits = sum.ContractValueMinorUnits + sum.LabourEstimateMinorUnits
	sum.RemainingMinorUnits = sum.BudgetMinorUnits - sum.CommittedMinorUnits
	sum.Budget = money.Amount{MinorUnits: sum.BudgetMinorUnits}.MajorFloat()
	sum.ContractValue = money.Amount{MinorUnits: sum.ContractValueMinorUnits}.MajorFloat()
	sum.LabourEstimate = money.Amount{MinorUnits: sum.LabourEstimateMinorUnits}.MajorFloat()
	sum.Committed = money.Amount{MinorUnits: sum.CommittedMinorUnits}.MajorFloat()
	sum.Remaining = money.Amount{MinorUnits: sum.RemainingMinorUnits}.MajorFloat()
	for cat, minor := range sum.ByCategoryMinorUnits {
		sum.ByCategory[cat] = money.Amount{MinorUnits: minor}.MajorFloat()
	}
	return sum
}

func amountFromProject(p db.Project) money.Amount {
	if p.BudgetMinorUnits != 0 || p.Budget == 0 {
		return money.Amount{MinorUnits: p.BudgetMinorUnits}
	}
	return money.FromMajorFloat(p.Budget)
}

func amountFromHourlyRate(s db.Stakeholder) money.Amount {
	if s.HourlyRateMinorUnits != 0 || s.HourlyRate == 0 {
		return money.Amount{MinorUnits: s.HourlyRateMinorUnits}
	}
	return money.FromMajorFloat(s.HourlyRate)
}

func amountFromContractValue(s db.Stakeholder) money.Amount {
	if s.ContractValueMinorUnits != 0 || s.ContractValue == 0 {
		return money.Amount{MinorUnits: s.ContractValueMinorUnits}
	}
	return money.FromMajorFloat(s.ContractValue)
}

func addCategory(sum *Summary, cat string, amount money.Amount) {
	sum.ByCategoryMinorUnits[cat] += amount.MinorUnits
}

// lower is a cheap ASCII-lowercase fold used for case-insensitive
// name matching. Full Unicode case folding (strings.ToLower) is fine
// too; we keep this local to avoid pulling in unicode tables on the
// hot path.
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
