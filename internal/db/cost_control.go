// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CostType classifies a cost along independent accounting dimensions.
type CostType struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Attribution string `json:"attribution"`
	Behavior    string `json:"behavior"`
	Treatment   string `json:"treatment"`
	Active      bool   `json:"active"`
}

// CostEntry is an immutable amount-bearing ledger row once recorded. Amounts
// are canonical signed integer minor units; callers use a wire DTO at Wails.
type CostEntry struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	CostTypeID       string `json:"cost_type_id"`
	Kind             string `json:"kind"`
	CostDate         string `json:"cost_date"`
	Description      string `json:"description"`
	AmountMinorUnits int64  `json:"amount_minor_units"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type CostReserve struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Kind             string `json:"kind"`
	AmountMinorUnits int64  `json:"amount_minor_units"`
	Description      string `json:"description"`
}

var ErrNoCostType = errors.New("db: cost type not found")

func (db *Database) EnsureCostTypes(projectID string) error {
	seeds := []CostType{
		{Code: "labor", Name: "Labor & human resources", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "materials", Name: "Materials & equipment", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "software", Name: "Software & technology", Attribution: "direct", Behavior: "fixed", Treatment: "opex"},
		{Code: "travel", Name: "Travel & on-site", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "facilities", Name: "Facilities & utilities", Attribution: "indirect", Behavior: "fixed", Treatment: "opex"},
		{Code: "shared_support", Name: "Administrative & shared support", Attribution: "indirect", Behavior: "fixed", Treatment: "opex"},
		{Code: "quality", Name: "Quality, compliance & training", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "capital", Name: "Capital equipment & licensing", Attribution: "direct", Behavior: "fixed", Treatment: "capex"},
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var existing int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_types WHERE project_id = ?`, projectID).Scan(&existing); err != nil {
		return err
	}
	for _, seed := range seeds {
		id, idErr := newID("ctype")
		if idErr != nil {
			err = idErr
			return err
		}
		if _, err = tx.Exec(`INSERT INTO cost_types (id, project_id, code, name, attribution, behavior, treatment, active) VALUES (?, ?, ?, ?, ?, ?, ?, 1) ON CONFLICT(project_id, code) DO NOTHING`, id, projectID, seed.Code, seed.Name, seed.Attribution, seed.Behavior, seed.Treatment); err != nil {
			return err
		}
	}
	if existing == 0 {
		payload, marshalErr := json.Marshal(seeds)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: projectID, EventType: "cost_type.seed", EntityType: "cost_type", AfterJSON: string(payload)}); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (db *Database) ListCostTypes(projectID string) ([]CostType, error) {
	if err := db.EnsureCostTypes(projectID); err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(`SELECT id, project_id, code, name, attribution, behavior, treatment, active FROM cost_types WHERE project_id = ? ORDER BY code`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostType
	for rows.Next() {
		var c CostType
		var active int
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Code, &c.Name, &c.Attribution, &c.Behavior, &c.Treatment, &active); err != nil {
			return nil, err
		}
		c.Active = active != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *Database) SaveCostEntry(entry CostEntry) (CostEntry, error) {
	if entry.ProjectID == "" || entry.CostTypeID == "" || entry.Description == "" || entry.CostDate == "" {
		return CostEntry{}, errors.New("cost entry: project, cost type, date, and description are required")
	}
	if entry.AmountMinorUnits == 0 {
		return CostEntry{}, errors.New("cost entry: amount must not be zero")
	}
	if entry.Kind != "planned" && entry.Kind != "commitment" && entry.Kind != "actual" {
		return CostEntry{}, fmt.Errorf("cost entry: invalid kind %q", entry.Kind)
	}
	if _, err := time.Parse("2006-01-02", entry.CostDate); err != nil {
		return CostEntry{}, fmt.Errorf("cost entry: date must be YYYY-MM-DD: %w", err)
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return CostEntry{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var active int
	if err = tx.QueryRow(`SELECT active FROM cost_types WHERE id=? AND project_id=?`, entry.CostTypeID, entry.ProjectID).Scan(&active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CostEntry{}, ErrNoCostType
		}
		return CostEntry{}, err
	}
	if active == 0 {
		return CostEntry{}, errors.New("cost entry: cost type is archived")
	}
	if entry.ID == "" {
		entry.ID, err = newID("cost")
		if err != nil {
			return CostEntry{}, err
		}
	}
	now := captureTimestamp()
	if _, err = tx.Exec(`INSERT INTO cost_entries (id, project_id, cost_type_id, kind, amount_minor_units, cost_date, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, entry.ID, entry.ProjectID, entry.CostTypeID, entry.Kind, entry.AmountMinorUnits, entry.CostDate, entry.Description, now.text, now.text); err != nil {
		return CostEntry{}, err
	}
	entry.CreatedAt, entry.UpdatedAt = now.text, now.text
	after, err := json.Marshal(entry)
	if err != nil {
		return CostEntry{}, err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: entry.ProjectID, EventType: "cost_entry.create", EntityType: "cost_entry", EntityID: entry.ID, AfterJSON: string(after)}); err != nil {
		return CostEntry{}, err
	}
	err = tx.Commit()
	return entry, err
}

func (db *Database) ListCostEntries(projectID string) ([]CostEntry, error) {
	rows, err := db.Conn.Query(`SELECT id,project_id,cost_type_id,kind,amount_minor_units,cost_date,description,created_at,updated_at FROM cost_entries WHERE project_id=? ORDER BY cost_date DESC, created_at DESC`, projectID) // timestamp-order-guard-exempt: Cost Control tables are new in this schema and write created_at only through captureTimestamp's fixed-width UTC timestampLayout; lexicographic order is chronological. See timestamps.go.
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostEntry
	for rows.Next() {
		var e CostEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.CostTypeID, &e.Kind, &e.AmountMinorUnits, &e.CostDate, &e.Description, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *Database) ListCostReserves(projectID string) ([]CostReserve, error) {
	rows, err := db.Conn.Query(`SELECT id,project_id,kind,amount_minor_units,description FROM cost_reserves WHERE project_id=? ORDER BY kind`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CostReserve
	for rows.Next() {
		var r CostReserve
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Kind, &r.AmountMinorUnits, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (db *Database) SaveCostReserve(r CostReserve) (CostReserve, error) {
	if r.ProjectID == "" || (r.Kind != "contingency" && r.Kind != "management") || r.AmountMinorUnits < 0 || strings.TrimSpace(r.Description) == "" {
		return CostReserve{}, errors.New("cost reserve: invalid input")
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return CostReserve{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if r.ID == "" {
		lookupErr := tx.QueryRow(`SELECT id FROM cost_reserves WHERE project_id = ? AND kind = ?`, r.ProjectID, r.Kind).Scan(&r.ID)
		if lookupErr == sql.ErrNoRows {
			r.ID, err = newID("reserve")
			if err != nil {
				return CostReserve{}, err
			}
		} else if lookupErr != nil {
			return CostReserve{}, lookupErr
		}
	}
	now := captureTimestamp()
	_, err = tx.Exec(`INSERT INTO cost_reserves (id,project_id,kind,amount_minor_units,description,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(project_id,kind) DO UPDATE SET amount_minor_units=excluded.amount_minor_units,description=excluded.description,updated_at=excluded.updated_at`, r.ID, r.ProjectID, r.Kind, r.AmountMinorUnits, r.Description, now.text, now.text)
	if err != nil {
		return CostReserve{}, err
	}
	after, _ := json.Marshal(r)
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: r.ProjectID, EventType: "cost_reserve.save", EntityType: "cost_reserve", EntityID: r.ID, AfterJSON: string(after)}); err != nil {
		return CostReserve{}, err
	}
	err = tx.Commit()
	return r, err
}

func validCurrencyCode(code string) bool {
	// Phase 1 is intentionally single-currency per project and does not
	// implement FX. Keep the accepted reporting currencies in a deliberately
	// small, UI-representable set rather than claiming every three-letter string
	// is an ISO 4217 code.
	_, ok := supportedProjectCurrencies[code]
	return ok
}

var supportedProjectCurrencies = map[string]struct{}{
	"AUD": {},
	"CAD": {},
	"CHF": {},
	"EUR": {},
	"GBP": {},
	"JPY": {},
	"USD": {},
}

func normaliseCurrencyCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }
