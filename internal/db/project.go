// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopmgr/internal/money"
)

// Project is the canonical metadata for an in-progress effort.
//
// Phase values track PMI's process groups: initiation, planning,
// execution, monitoring, closing.
//
// Status values track operational state: planning, active, on_hold,
// complete, cancelled.
//
// Industry, SubCategory, Methodology, and CountryCode were added in
// V2.x to support the Project Launchpad and country-aware calendar
// features. All four are optional; legacy .pmforge files default to
// empty strings (and "US" for CountryCode).
type Project struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Status           string  `json:"status"`
	Phase            string  `json:"phase"`
	StartDate        string  `json:"start_date"`
	EndDate          string  `json:"end_date"`
	Budget           float64 `json:"budget"`
	BudgetMinorUnits int64   `json:"budget_minor_units,omitempty"`
	CurrencyCode     string  `json:"currency_code"`
	Owner            string  `json:"owner"`
	Industry         string  `json:"industry"`
	SubCategory      string  `json:"sub_category"`
	Methodology      string  `json:"methodology"`
	CountryCode      string  `json:"country_code"`
	TimeZone         string  `json:"time_zone"`
	// RFC3339Nano strings (server-managed); see the note on db.Chart for why
	// these are strings rather than time.Time (Wails empty-string round-trip).
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ValidPhases is the canonical set of project phases.
var ValidPhases = []string{"initiation", "planning", "execution", "monitoring", "closing"}

// ValidStatuses is the canonical set of project statuses.
var ValidStatuses = []string{"planning", "active", "on_hold", "complete", "cancelled"}

// ErrNoProject indicates the .pmforge file has no project row yet.
var ErrNoProject = errors.New("db: no project initialised in this file")

// GetProject returns the single project stored in this .pmforge file
// (the model is one-project-per-file for now). Known additive columns an older
// file does not have yet use the same in-memory defaults a writable migration
// would store. Missing original columns return an error. The file is not
// altered. Names in the SELECT are a fixed allowlist intersected with
// PRAGMA table_info, never names taken from the file as SQL text.
func (db *Database) GetProject() (Project, error) {
	present, err := db.columnSet("project")
	if err != nil {
		return Project{}, err
	}
	if len(present) == 0 {
		return Project{}, ErrNoProject
	}
	var p Project
	fields := []struct {
		name string
		dest any
	}{
		{"id", &p.ID},
		{"name", &p.Name},
		{"description", &p.Description},
		{"status", &p.Status},
		{"phase", &p.Phase},
		{"start_date", &p.StartDate},
		{"end_date", &p.EndDate},
		{"budget", &p.Budget},
		{"budget_minor_units", &p.BudgetMinorUnits},
		{"currency_code", &p.CurrencyCode},
		{"owner", &p.Owner},
		{"industry", &p.Industry},
		{"sub_category", &p.SubCategory},
		{"methodology", &p.Methodology},
		{"country_code", &p.CountryCode},
		{"time_zone", &p.TimeZone},
		{"created_at", &p.CreatedAt},
		{"updated_at", &p.UpdatedAt},
	}
	names := make([]string, 0, len(fields))
	dest := make([]any, 0, len(fields))
	for _, field := range fields {
		if _, ok := present[field.name]; !ok {
			switch field.name {
			case "industry", "sub_category", "methodology", "country_code", "time_zone", "budget_minor_units", "currency_code":
				continue // Only migrateLegacyColumns additions may be absent.
			default:
				return Project{}, fmt.Errorf("db: project is missing required column %s", field.name)
			}
		}
		names = append(names, field.name)
		dest = append(dest, field.dest)
	}
	query := "SELECT " + strings.Join(names, ", ") + " FROM project LIMIT 1" // #nosec G201 -- allowlisted column names only.
	err = db.Conn.QueryRow(query).Scan(dest...)
	if err == sql.ErrNoRows {
		return Project{}, ErrNoProject
	}
	if err != nil {
		return Project{}, err
	}
	return normalizeProject(p), nil
}

// UpsertProject inserts or updates the project row. If p.ID is empty,
// a new project is created and its ID returned via the result.
func (db *Database) UpsertProject(p Project) (Project, error) {
	if p.ID == "" {
		id, err := newID("prj")
		if err != nil {
			return Project{}, fmt.Errorf("generate project id: %w", err)
		}
		p.ID = id
	}
	if p.CountryCode == "" {
		p.CountryCode = "US"
	}
	currencyCode, err := CanonicalProjectCurrency(p.CurrencyCode)
	if err != nil {
		return Project{}, err
	}
	p.CurrencyCode = currencyCode
	if p.TimeZone == "" {
		p.TimeZone = "America/New_York"
	}
	if p.BudgetMinorUnits == 0 && p.Budget != 0 {
		p.BudgetMinorUnits = money.FromMajorFloat(p.Budget).MinorUnits
	}
	p.Budget = money.Amount{MinorUnits: p.BudgetMinorUnits}.MajorFloat()
	now := captureTimestamp()

	tx, err := db.Conn.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback() }() // harmless after Commit; releases every early-return path.

	before, err := getProjectByIDTx(tx, p.ID)
	isCreate := false
	if err == ErrNoProject {
		isCreate = true
	} else if err != nil {
		return Project{}, err
	}
	if !isCreate && before.CurrencyCode != p.CurrencyCode {
		var count, reserveCount, baselineCount int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_entries WHERE project_id = ?`, p.ID).Scan(&count); err != nil {
			return Project{}, err
		}
		if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_reserves WHERE project_id = ? AND amount_minor_units != 0`, p.ID).Scan(&reserveCount); err != nil {
			return Project{}, err
		}
		if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_baseline_snapshots WHERE project_id = ?`, p.ID).Scan(&baselineCount); err != nil {
			return Project{}, err
		}
		if count > 0 || reserveCount > 0 || baselineCount > 0 || before.BudgetMinorUnits != 0 {
			return Project{}, errors.New("project reporting currency cannot change while a Budget value, Cost Control entry, non-zero reserve balance, or approved Cost Control baseline exists")
		}
	}

	_, err = tx.Exec(`
		INSERT INTO project (id, name, description, status, phase,
			start_date, end_date, budget, budget_minor_units, currency_code, owner,
			industry, sub_category, methodology, country_code, time_zone,
			created_at, updated_at, created_at_unixnano, updated_at_unixnano)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name         = excluded.name,
			description  = excluded.description,
			status       = excluded.status,
			phase        = excluded.phase,
			start_date   = excluded.start_date,
			end_date     = excluded.end_date,
			budget       = excluded.budget,
			budget_minor_units = excluded.budget_minor_units,
			currency_code = excluded.currency_code,
			owner        = excluded.owner,
			industry     = excluded.industry,
			sub_category = excluded.sub_category,
			methodology  = excluded.methodology,
			country_code = excluded.country_code,
			time_zone    = excluded.time_zone,
			updated_at   = excluded.updated_at,
			updated_at_unixnano = excluded.updated_at_unixnano
	`,
		p.ID, p.Name, p.Description, p.Status, p.Phase,
		p.StartDate, p.EndDate, p.Budget, p.BudgetMinorUnits, p.CurrencyCode, p.Owner,
		p.Industry, p.SubCategory, p.Methodology, p.CountryCode, p.TimeZone,
		now.text, now.text, now.unixNano, now.unixNano,
	)
	if err != nil {
		return Project{}, err
	}
	after, err := getProjectByIDTx(tx, p.ID)
	if err != nil {
		return Project{}, err
	}
	afterJSON, err := projectAuditJSON(after)
	if err != nil {
		return Project{}, err
	}
	beforeJSON := ""
	eventType := "project.create"
	if !isCreate {
		beforeJSON, err = projectAuditJSON(before)
		if err != nil {
			return Project{}, err
		}
		eventType = "project.update"
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{
		ProjectID:  after.ID,
		EventType:  eventType,
		EntityType: "project",
		EntityID:   after.ID,
		BeforeJSON: beforeJSON,
		AfterJSON:  afterJSON,
	}); err != nil {
		return Project{}, err
	}
	if err = tx.Commit(); err != nil {
		return Project{}, err
	}
	return after, nil
}

func getProjectByIDTx(tx *sql.Tx, id string) (Project, error) {
	row := tx.QueryRow(
		`SELECT id, name, description, status, phase, start_date, end_date,
		        budget, budget_minor_units, currency_code, owner, industry, sub_category, methodology, country_code, time_zone,
		        created_at, updated_at
		 FROM project WHERE id = ? LIMIT 1`,
		id,
	)
	return scanProject(row)
}

func projectAuditJSON(p Project) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func scanProject(row interface {
	Scan(dest ...interface{}) error
}) (Project, error) {
	var (
		p                Project
		created, updated string
	)
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Status, &p.Phase,
		&p.StartDate, &p.EndDate, &p.Budget, &p.BudgetMinorUnits, &p.CurrencyCode, &p.Owner,
		&p.Industry, &p.SubCategory, &p.Methodology, &p.CountryCode, &p.TimeZone,
		&created, &updated,
	)
	if err == sql.ErrNoRows {
		return Project{}, ErrNoProject
	}
	if err != nil {
		return Project{}, err
	}
	p.CreatedAt = created
	p.UpdatedAt = updated
	return normalizeProject(p), nil
}

func normalizeProject(p Project) Project {
	if p.CountryCode == "" {
		p.CountryCode = "US"
	}
	if p.CurrencyCode == "" {
		p.CurrencyCode = "USD"
	}
	if p.TimeZone == "" {
		p.TimeZone = "America/New_York"
	}
	if p.BudgetMinorUnits == 0 && p.Budget != 0 {
		p.BudgetMinorUnits = money.FromMajorFloat(p.Budget).MinorUnits
	} else {
		p.Budget = money.Amount{MinorUnits: p.BudgetMinorUnits}.MajorFloat()
	}
	return p
}
