// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"time"
)

// StakeholderCategory enumerates the role a stakeholder plays in the
// project. Used by the GUI to filter (e.g. "show only vendors when
// composing a procurement plan") and by the budget rollup (vendors
// and contractors contribute to committed cost; team members do too
// but via their hourly rate × work-item points).
type StakeholderCategory string

const (
	StakeholderTeam     StakeholderCategory = "team"     // internal staff
	StakeholderVendor   StakeholderCategory = "vendor"   // supplier / subcontractor
	StakeholderSponsor  StakeholderCategory = "sponsor"  // budget owner
	StakeholderExternal StakeholderCategory = "external" // any other interested party
)

// Stakeholder is one entry in the project-level address book.
// Promoted from per-document strings (Charter, Stakeholder Analysis)
// to a shared project resource in V2.x so RACI rows, document fields,
// and the budget rollup can all reference the same record.
type Stakeholder struct {
	ID            string              `json:"id"`
	ProjectID     string              `json:"project_id"`
	Name          string              `json:"name"`
	Role          string              `json:"role"`
	Organisation  string              `json:"organisation"`
	Email         string              `json:"email"`
	Phone         string              `json:"phone"`
	Category      StakeholderCategory `json:"category"`
	HourlyRate    float64             `json:"hourly_rate"`
	ContractValue float64             `json:"contract_value"`
	Notes         string              `json:"notes"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// ErrNoStakeholder is returned by GetStakeholder for unknown IDs.
var ErrNoStakeholder = errors.New("db: stakeholder not found")

// SaveStakeholder inserts or updates a stakeholder. Empty Category
// defaults to "team"; empty ID gets a fresh one.
func (db *Database) SaveStakeholder(s Stakeholder) (Stakeholder, error) {
	if s.ID == "" {
		s.ID = newID("stk")
	}
	if s.Category == "" {
		s.Category = StakeholderTeam
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := db.Conn.Exec(`
		INSERT INTO stakeholders (id, project_id, name, role, organisation,
			email, phone, category, hourly_rate, contract_value, notes,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name           = excluded.name,
			role           = excluded.role,
			organisation   = excluded.organisation,
			email          = excluded.email,
			phone          = excluded.phone,
			category       = excluded.category,
			hourly_rate    = excluded.hourly_rate,
			contract_value = excluded.contract_value,
			notes          = excluded.notes,
			updated_at     = excluded.updated_at
	`,
		s.ID, s.ProjectID, s.Name, s.Role, s.Organisation,
		s.Email, s.Phone, string(s.Category), s.HourlyRate, s.ContractValue, s.Notes,
		now, now,
	)
	if err != nil {
		return Stakeholder{}, err
	}
	return db.GetStakeholder(s.ID)
}

// GetStakeholder fetches one stakeholder by ID.
func (db *Database) GetStakeholder(id string) (Stakeholder, error) {
	row := db.Conn.QueryRow(`
		SELECT id, project_id, name, role, organisation, email, phone,
		       category, hourly_rate, contract_value, notes, created_at, updated_at
		FROM stakeholders WHERE id = ?
	`, id)
	return scanStakeholder(row)
}

// ListStakeholders returns every stakeholder for the project. Pass
// a non-empty category to filter.
func (db *Database) ListStakeholders(projectID, category string) ([]Stakeholder, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if category == "" {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, name, role, organisation, email, phone,
			       category, hourly_rate, contract_value, notes, created_at, updated_at
			FROM stakeholders WHERE project_id = ? ORDER BY name ASC
		`, projectID)
	} else {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, name, role, organisation, email, phone,
			       category, hourly_rate, contract_value, notes, created_at, updated_at
			FROM stakeholders WHERE project_id = ? AND category = ? ORDER BY name ASC
		`, projectID, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Stakeholder
	for rows.Next() {
		s, err := scanStakeholder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteStakeholder removes one stakeholder. References in other
// documents (string-typed) remain intact; the GUI shows them as
// "(deleted stakeholder)".
func (db *Database) DeleteStakeholder(id string) error {
	_, err := db.Conn.Exec(`DELETE FROM stakeholders WHERE id = ?`, id)
	return err
}

func scanStakeholder(row interface {
	Scan(...interface{}) error
}) (Stakeholder, error) {
	var (
		s                Stakeholder
		category         string
		created, updated string
	)
	err := row.Scan(
		&s.ID, &s.ProjectID, &s.Name, &s.Role, &s.Organisation,
		&s.Email, &s.Phone, &category, &s.HourlyRate, &s.ContractValue, &s.Notes,
		&created, &updated,
	)
	if err == sql.ErrNoRows {
		return Stakeholder{}, ErrNoStakeholder
	}
	if err != nil {
		return Stakeholder{}, err
	}
	s.Category = StakeholderCategory(category)
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		s.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		s.UpdatedAt = t
	}
	return s, nil
}
