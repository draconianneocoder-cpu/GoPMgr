// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"pmforge/internal/money"
	"time"
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
	Owner            string  `json:"owner"`
	Industry         string  `json:"industry"`
	SubCategory      string  `json:"sub_category"`
	Methodology      string  `json:"methodology"`
	CountryCode      string  `json:"country_code"`
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
// (the model is one-project-per-file for now).
func (db *Database) GetProject() (Project, error) {
	row := db.Conn.QueryRow(
		`SELECT id, name, description, status, phase, start_date, end_date,
		        budget, budget_minor_units, owner, industry, sub_category, methodology, country_code,
		        created_at, updated_at
		 FROM project LIMIT 1`,
	)
	return scanProject(row)
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
	if p.BudgetMinorUnits == 0 && p.Budget != 0 {
		p.BudgetMinorUnits = money.FromMajorFloat(p.Budget).MinorUnits
	}
	p.Budget = money.Amount{MinorUnits: p.BudgetMinorUnits}.MajorFloat()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	tx, err := db.Conn.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	before, err := getProjectByIDTx(tx, p.ID)
	isCreate := false
	if err == ErrNoProject {
		isCreate = true
		err = nil
	} else if err != nil {
		return Project{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO project (id, name, description, status, phase,
			start_date, end_date, budget, budget_minor_units, owner,
			industry, sub_category, methodology, country_code,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name         = excluded.name,
			description  = excluded.description,
			status       = excluded.status,
			phase        = excluded.phase,
			start_date   = excluded.start_date,
			end_date     = excluded.end_date,
			budget       = excluded.budget,
			budget_minor_units = excluded.budget_minor_units,
			owner        = excluded.owner,
			industry     = excluded.industry,
			sub_category = excluded.sub_category,
			methodology  = excluded.methodology,
			country_code = excluded.country_code,
			updated_at   = excluded.updated_at
	`,
		p.ID, p.Name, p.Description, p.Status, p.Phase,
		p.StartDate, p.EndDate, p.Budget, p.BudgetMinorUnits, p.Owner,
		p.Industry, p.SubCategory, p.Methodology, p.CountryCode,
		now, now,
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
		        budget, budget_minor_units, owner, industry, sub_category, methodology, country_code,
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
		&p.StartDate, &p.EndDate, &p.Budget, &p.BudgetMinorUnits, &p.Owner,
		&p.Industry, &p.SubCategory, &p.Methodology, &p.CountryCode,
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
	if p.CountryCode == "" {
		p.CountryCode = "US"
	}
	if p.BudgetMinorUnits == 0 && p.Budget != 0 {
		p.BudgetMinorUnits = money.FromMajorFloat(p.Budget).MinorUnits
	} else {
		p.Budget = money.Amount{MinorUnits: p.BudgetMinorUnits}.MajorFloat()
	}
	return p, nil
}
