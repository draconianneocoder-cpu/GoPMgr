// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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
	"fmt"

	"gopmgr/internal/agile"
	"gopmgr/internal/db"
	"gopmgr/internal/money"
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

// Compute walks the inputs and produces a Summary. Stakeholders are
// indexed by lower-cased Name before scanning work items (matched
// against each work item's Assignee) so the rollup is
// O(workItems + stakeholders). An unrepresentable aggregate or
// derived difference returns an error wrapping money.ErrOverflow.
func Compute(project db.Project, stakeholders []db.Stakeholder, workItems []agile.WorkItem) (Summary, error) {
	budget := amountFromProject(project)
	var contracts, labour money.Accumulator
	byCategory := make(map[string]*money.Accumulator)

	for _, s := range stakeholders {
		// Vendor / contract values roll up directly.
		contract := amountFromContractValue(s)
		if contract.Positive() {
			contracts.Add(contract)
			addCategory(byCategory, string(s.Category), contract)
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
		labour.Add(cost)
		if cat := catByName[lower(wi.Assignee)]; cat != "" {
			addCategory(byCategory, cat, cost)
		}
	}

	contractValue, err := accumulatedAmount("contract value", &contracts)
	if err != nil {
		return Summary{}, err
	}
	labourEstimate, err := accumulatedAmount("labour estimate", &labour)
	if err != nil {
		return Summary{}, err
	}
	committed, err := contractValue.Add(labourEstimate)
	if err != nil {
		return Summary{}, fmt.Errorf("compute committed cost: %w", err)
	}
	remaining, err := budget.Sub(committed)
	if err != nil {
		return Summary{}, fmt.Errorf("compute budget remaining: %w", err)
	}

	sum := Summary{
		BudgetMinorUnits:         budget.MinorUnits,
		ContractValueMinorUnits:  contractValue.MinorUnits,
		LabourEstimateMinorUnits: labourEstimate.MinorUnits,
		CommittedMinorUnits:      committed.MinorUnits,
		RemainingMinorUnits:      remaining.MinorUnits,
		ByCategory:               make(map[string]float64, len(byCategory)),
		ByCategoryMinorUnits:     make(map[string]int64, len(byCategory)),
	}
	sum.Budget = money.Amount{MinorUnits: sum.BudgetMinorUnits}.MajorFloat()
	sum.ContractValue = money.Amount{MinorUnits: sum.ContractValueMinorUnits}.MajorFloat()
	sum.LabourEstimate = money.Amount{MinorUnits: sum.LabourEstimateMinorUnits}.MajorFloat()
	sum.Committed = money.Amount{MinorUnits: sum.CommittedMinorUnits}.MajorFloat()
	sum.Remaining = money.Amount{MinorUnits: sum.RemainingMinorUnits}.MajorFloat()
	for cat, total := range byCategory {
		amount, err := accumulatedAmount("category "+cat, total)
		if err != nil {
			// Defensive, not reachable through this function today: every
			// category's accumulator only receives non-negative contract
			// values (guarded by contract.Positive() above) and
			// non-negative labour costs (rate/points both gated positive
			// before RateTimesQuantity), and each category is a subset of
			// the exact same non-negative contributions already summed
			// into contractValue/labourEstimate/committed above. Since
			// committed didn't overflow (checked before this loop), no
			// category subtotal — being a non-negative subset of a
			// representable sum — can overflow either. Kept as a guard in
			// case a future change (e.g. allowing negative adjustments)
			// breaks that invariant; no portable test reaches it under the
			// current one.
			return Summary{}, err
		}
		sum.ByCategoryMinorUnits[cat] = amount.MinorUnits
		sum.ByCategory[cat] = amount.MajorFloat()
	}
	return sum, nil
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

func addCategory(totals map[string]*money.Accumulator, cat string, amount money.Amount) {
	total := totals[cat]
	if total == nil {
		total = &money.Accumulator{}
		totals[cat] = total
	}
	total.Add(amount)
}

func accumulatedAmount(label string, total *money.Accumulator) (money.Amount, error) {
	amount, err := total.Amount()
	if err != nil {
		return money.Amount{}, fmt.Errorf("compute %s: %w", label, err)
	}
	return amount, nil
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
