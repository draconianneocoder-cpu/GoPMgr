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
)

// Summary is the panel-ready cost rollup.
type Summary struct {
	Budget         float64            `json:"budget"`          // project.budget — the cap
	ContractValue  float64            `json:"contract_value"`  // Σ stakeholder.contract_value for vendors
	LabourEstimate float64            `json:"labour_estimate"` // Σ work-item-points × assignee.hourly_rate
	Committed      float64            `json:"committed"`       // contract_value + labour_estimate
	Remaining      float64            `json:"remaining"`       // budget - committed (negative if over)
	ByCategory     map[string]float64 `json:"by_category"`     // breakdown by stakeholder category
}

// Compute walks the inputs and produces a Summary. Stakeholder is
// the lookup table; we index it by ID before scanning work items so
// the rollup is O(workItems + stakeholders).
func Compute(project db.Project, stakeholders []db.Stakeholder, workItems []agile.WorkItem) Summary {
	sum := Summary{
		Budget:     project.Budget,
		ByCategory: map[string]float64{},
	}

	byID := make(map[string]db.Stakeholder, len(stakeholders))
	for _, s := range stakeholders {
		byID[s.ID] = s
		// Vendor / contract values roll up directly.
		if s.ContractValue > 0 {
			sum.ContractValue += s.ContractValue
			sum.ByCategory[string(s.Category)] += s.ContractValue
		}
	}

	// Labour estimate: points × rate. Work items are matched to
	// stakeholders by Assignee field; if the assignee string equals
	// a stakeholder's name (case-insensitive), apply that rate.
	rateByName := make(map[string]float64, len(stakeholders))
	catByName := make(map[string]string, len(stakeholders))
	for _, s := range stakeholders {
		if s.HourlyRate > 0 {
			rateByName[lower(s.Name)] = s.HourlyRate
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
		cost := wi.Points * rate
		sum.LabourEstimate += cost
		if cat := catByName[lower(wi.Assignee)]; cat != "" {
			sum.ByCategory[cat] += cost
		}
	}

	sum.Committed = sum.ContractValue + sum.LabourEstimate
	sum.Remaining = sum.Budget - sum.Committed
	return sum
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
