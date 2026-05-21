// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Phase       string    `json:"phase"`
	StartDate   string    `json:"start_date"`
	EndDate     string    `json:"end_date"`
	Budget      float64   `json:"budget"`
	Owner       string    `json:"owner"`
	Industry    string    `json:"industry"`
	SubCategory string    `json:"sub_category"`
	Methodology string    `json:"methodology"`
	CountryCode string    `json:"country_code"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
		        budget, owner, industry, sub_category, methodology, country_code,
		        created_at, updated_at
		 FROM project LIMIT 1`,
	)
	return scanProject(row)
}

// UpsertProject inserts or updates the project row. If p.ID is empty,
// a new project is created and its ID returned via the result.
func (db *Database) UpsertProject(p Project) (Project, error) {
	if p.ID == "" {
		p.ID = newID("prj")
	}
	if p.CountryCode == "" {
		p.CountryCode = "US"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := db.Conn.Exec(`
		INSERT INTO project (id, name, description, status, phase,
			start_date, end_date, budget, owner,
			industry, sub_category, methodology, country_code,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name         = excluded.name,
			description  = excluded.description,
			status       = excluded.status,
			phase        = excluded.phase,
			start_date   = excluded.start_date,
			end_date     = excluded.end_date,
			budget       = excluded.budget,
			owner        = excluded.owner,
			industry     = excluded.industry,
			sub_category = excluded.sub_category,
			methodology  = excluded.methodology,
			country_code = excluded.country_code,
			updated_at   = excluded.updated_at
	`,
		p.ID, p.Name, p.Description, p.Status, p.Phase,
		p.StartDate, p.EndDate, p.Budget, p.Owner,
		p.Industry, p.SubCategory, p.Methodology, p.CountryCode,
		now, now,
	)
	if err != nil {
		return Project{}, err
	}
	return db.GetProject()
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
		&p.StartDate, &p.EndDate, &p.Budget, &p.Owner,
		&p.Industry, &p.SubCategory, &p.Methodology, &p.CountryCode,
		&created, &updated,
	)
	if err == sql.ErrNoRows {
		return Project{}, ErrNoProject
	}
	if err != nil {
		return Project{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		p.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		p.UpdatedAt = t
	}
	if p.CountryCode == "" {
		p.CountryCode = "US"
	}
	return p, nil
}
