// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

func TestMoneyMinorUnitColumnsExist(t *testing.T) {
	d := newBackupTestDB(t)

	projectCols, err := d.columnSet("project")
	if err != nil {
		t.Fatalf("project columnSet: %v", err)
	}
	if _, ok := projectCols["budget_minor_units"]; !ok {
		t.Fatal("project.budget_minor_units column missing")
	}

	stakeholderCols, err := d.columnSet("stakeholders")
	if err != nil {
		t.Fatalf("stakeholders columnSet: %v", err)
	}
	for _, name := range []string{"hourly_rate_minor_units", "contract_value_minor_units"} {
		if _, ok := stakeholderCols[name]; !ok {
			t.Fatalf("stakeholders.%s column missing", name)
		}
	}
}

func TestProjectBudgetMinorUnitsRoundTrip(t *testing.T) {
	d := newBackupTestDB(t)

	saved, err := d.UpsertProject(Project{Name: "Money", Budget: 1234.56})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if saved.BudgetMinorUnits != 123456 {
		t.Fatalf("saved budget minor units = %d, want 123456", saved.BudgetMinorUnits)
	}

	var stored int64
	if err := d.Conn.QueryRow(`SELECT budget_minor_units FROM project WHERE id = ?`, saved.ID).Scan(&stored); err != nil {
		t.Fatalf("select budget_minor_units: %v", err)
	}
	if stored != 123456 {
		t.Fatalf("stored budget_minor_units = %d, want 123456", stored)
	}

	saved.Budget = 9999
	saved.BudgetMinorUnits = 78901
	updated, err := d.UpsertProject(saved)
	if err != nil {
		t.Fatalf("UpsertProject update: %v", err)
	}
	if updated.BudgetMinorUnits != 78901 || updated.Budget != 789.01 {
		t.Fatalf("updated budget = %v / %d, want 789.01 / 78901", updated.Budget, updated.BudgetMinorUnits)
	}
}

func TestProjectTimeZoneRoundTrip(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Tokyo", CountryCode: "JP", TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	got, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.TimeZone != "Asia/Tokyo" || got.TimeZone != "Asia/Tokyo" {
		t.Fatalf("time zone round trip = %q / %q", p.TimeZone, got.TimeZone)
	}
}

func TestStakeholderMoneyMinorUnitsRoundTrip(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Stakeholder Money"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	saved, err := d.SaveStakeholder(Stakeholder{
		ProjectID:     p.ID,
		Name:          "Alice",
		HourlyRate:    88.25,
		ContractValue: 1200.99,
	})
	if err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	if saved.HourlyRateMinorUnits != 8825 {
		t.Fatalf("HourlyRateMinorUnits = %d, want 8825", saved.HourlyRateMinorUnits)
	}
	if saved.ContractValueMinorUnits != 120099 {
		t.Fatalf("ContractValueMinorUnits = %d, want 120099", saved.ContractValueMinorUnits)
	}

	var rate, contract int64
	if err := d.Conn.QueryRow(
		`SELECT hourly_rate_minor_units, contract_value_minor_units FROM stakeholders WHERE id = ?`,
		saved.ID,
	).Scan(&rate, &contract); err != nil {
		t.Fatalf("select stakeholder money columns: %v", err)
	}
	if rate != 8825 || contract != 120099 {
		t.Fatalf("stored rate/contract = %d/%d, want 8825/120099", rate, contract)
	}
}
