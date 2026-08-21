// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"gopmgr/internal/db"
	"gopmgr/internal/money"
)

// ProjectMetaWire is the Wails-safe project metadata boundary. Budget is a
// canonical decimal string so project budget edits never pass through a
// JavaScript number. The database model remains db.Project.
type ProjectMetaWire struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Phase        string `json:"phase"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Budget       string `json:"budget"`
	CurrencyCode string `json:"currency_code"`
	Owner        string `json:"owner"`
	Industry     string `json:"industry"`
	SubCategory  string `json:"sub_category"`
	Methodology  string `json:"methodology"`
	CountryCode  string `json:"country_code"`
	TimeZone     string `json:"time_zone"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func projectMetaWire(project db.Project) ProjectMetaWire {
	return ProjectMetaWire{
		ID: project.ID, Name: project.Name, Description: project.Description,
		Status: project.Status, Phase: project.Phase, StartDate: project.StartDate,
		EndDate: project.EndDate, Budget: money.Amount{MinorUnits: project.BudgetMinorUnits}.Decimal(),
		CurrencyCode: project.CurrencyCode, Owner: project.Owner, Industry: project.Industry,
		SubCategory: project.SubCategory, Methodology: project.Methodology,
		CountryCode: project.CountryCode, TimeZone: project.TimeZone,
		CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
	}
}

func projectFromMetaWire(wire ProjectMetaWire) (db.Project, error) {
	budget, err := money.ParseDecimal(wire.Budget)
	if err != nil {
		return db.Project{}, fmt.Errorf("project budget: %w", err)
	}
	return db.Project{
		ID: wire.ID, Name: wire.Name, Description: wire.Description, Status: wire.Status,
		Phase: wire.Phase, StartDate: wire.StartDate, EndDate: wire.EndDate,
		BudgetMinorUnits: budget.MinorUnits, CurrencyCode: wire.CurrencyCode,
		Owner: wire.Owner, Industry: wire.Industry, SubCategory: wire.SubCategory,
		Methodology: wire.Methodology, CountryCode: wire.CountryCode, TimeZone: wire.TimeZone,
		CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt,
	}, nil
}
