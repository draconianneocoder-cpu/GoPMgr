// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package budget

import (
	"testing"

	"pmforge/internal/agile"
	"pmforge/internal/db"
)

// TestComputeEmpty: with no stakeholders and no work items, Committed
// should be 0 and Remaining should equal Budget.
func TestComputeEmpty(t *testing.T) {
	got := Compute(db.Project{Budget: 100_000}, nil, nil)
	if got.Budget != 100_000 {
		t.Errorf("Budget: want 100000, got %v", got.Budget)
	}
	if got.Committed != 0 {
		t.Errorf("Committed: want 0, got %v", got.Committed)
	}
	if got.Remaining != 100_000 {
		t.Errorf("Remaining: want 100000, got %v", got.Remaining)
	}
}

// TestVendorContractsRollUp: vendor stakeholders with contract_value
// contribute to Committed regardless of work-item assignments.
func TestVendorContractsRollUp(t *testing.T) {
	stake := []db.Stakeholder{
		{Name: "Acme Corp", Category: db.StakeholderVendor, ContractValue: 40_000},
		{Name: "Beta LLC", Category: db.StakeholderVendor, ContractValue: 25_000},
		{Name: "Sponsor", Category: db.StakeholderSponsor, ContractValue: 0},
	}
	got := Compute(db.Project{Budget: 100_000}, stake, nil)
	if got.ContractValue != 65_000 {
		t.Errorf("ContractValue: want 65000, got %v", got.ContractValue)
	}
	if got.Committed != 65_000 {
		t.Errorf("Committed: want 65000, got %v", got.Committed)
	}
	if got.Remaining != 35_000 {
		t.Errorf("Remaining: want 35000, got %v", got.Remaining)
	}
	if got.ByCategory["vendor"] != 65_000 {
		t.Errorf("ByCategory[vendor]: want 65000, got %v", got.ByCategory["vendor"])
	}
}

// TestLabourEstimateNameMatch: work items with an Assignee that
// case-insensitively matches a stakeholder's Name pick up that
// stakeholder's hourly rate.
func TestLabourEstimateNameMatch(t *testing.T) {
	stake := []db.Stakeholder{
		{Name: "Alice", Category: db.StakeholderTeam, HourlyRate: 120},
		{Name: "Bob", Category: db.StakeholderTeam, HourlyRate: 90},
	}
	items := []agile.WorkItem{
		{Assignee: "alice", Points: 4}, // case-insensitive match
		{Assignee: "Bob", Points: 3},
		{Assignee: "Carol", Points: 2}, // no stakeholder → ignored
	}
	got := Compute(db.Project{Budget: 5000}, stake, items)
	want := 4*120 + 3*90 // 750
	if int(got.LabourEstimate) != want {
		t.Errorf("LabourEstimate: want %d, got %v", want, got.LabourEstimate)
	}
	if int(got.Committed) != want {
		t.Errorf("Committed: want %d, got %v", want, got.Committed)
	}
}

// TestOverBudgetNegativeRemaining: Remaining can go negative when
// commitments exceed the budget.
func TestOverBudgetNegativeRemaining(t *testing.T) {
	stake := []db.Stakeholder{
		{Category: db.StakeholderVendor, ContractValue: 200},
	}
	got := Compute(db.Project{Budget: 100}, stake, nil)
	if got.Remaining != -100 {
		t.Errorf("Remaining: want -100 (over budget), got %v", got.Remaining)
	}
}

// TestZeroPointsAndZeroRates: items with no points OR stakeholders
// with no rate should not contribute to the labour estimate.
func TestZeroPointsAndZeroRates(t *testing.T) {
	stake := []db.Stakeholder{
		{Name: "Alice", Category: db.StakeholderTeam, HourlyRate: 0},
		{Name: "Bob", Category: db.StakeholderTeam, HourlyRate: 50},
	}
	items := []agile.WorkItem{
		{Assignee: "Alice", Points: 8}, // rate=0 → 0
		{Assignee: "Bob", Points: 0},   // points=0 → 0
		{Assignee: "Bob", Points: 2},   // 100
	}
	got := Compute(db.Project{}, stake, items)
	if got.LabourEstimate != 100 {
		t.Errorf("LabourEstimate: want 100, got %v", got.LabourEstimate)
	}
}
