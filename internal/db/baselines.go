// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Baseline is one snapshot of a CPM chart's scheduled task map.
// Data is opaque JSON (kernel.Task keyed by task ID) — the database
// does not interpret it.
type Baseline struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	ChartID   string    `json:"chart_id"`
	Name      string    `json:"name"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrNoBaseline is returned when GetBaseline can't find the ID.
var ErrNoBaseline = errors.New("db: baseline not found")

// SaveBaseline inserts a new baseline snapshot. Baselines are
// immutable: there is no update path, only insert and delete.
func (db *Database) SaveBaseline(b Baseline) (Baseline, error) {
	tx, err := db.Conn.Begin()
	if err != nil {
		return Baseline{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	saved, _, err := saveBaselineTx(tx, b)
	if err != nil {
		return Baseline{}, err
	}
	if err = tx.Commit(); err != nil {
		return Baseline{}, err
	}
	return saved, nil
}

// GetBaseline fetches a baseline by ID.
func (db *Database) GetBaseline(id string) (Baseline, error) {
	return scanBaseline(db.Conn.QueryRow(`
		SELECT id, project_id, chart_id, name, data, created_at
		FROM baselines WHERE id = ?
	`, id))
}

// ListBaselines returns every baseline for a chart, newest first.
func (db *Database) ListBaselines(chartID string) ([]Baseline, error) {
	rows, err := db.Conn.Query(`
		SELECT id, project_id, chart_id, name, data, created_at
		FROM baselines WHERE chart_id = ? ORDER BY created_at DESC
	`, chartID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Baseline
	for rows.Next() {
		var b Baseline
		var created string
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.ChartID, &b.Name, &b.Data, &created); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, b)
	}
	return out, rows.Err()
}

// DeleteBaseline removes a baseline snapshot.
func (db *Database) DeleteBaseline(id string) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	before, err := getBaselineTx(tx, id)
	if err == ErrNoBaseline {
		err = nil
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM baselines WHERE id = ?`, id); err != nil {
		return err
	}
	beforeJSON, err := baselineAuditJSON(before)
	if err != nil {
		return err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{
		ProjectID:  before.ProjectID,
		EventType:  "baseline.delete",
		EntityType: "baseline",
		EntityID:   before.ID,
		BeforeJSON: beforeJSON,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func saveBaselineTx(tx *sql.Tx, b Baseline) (Baseline, string, error) {
	if b.ID == "" {
		id, err := newID("baseline")
		if err != nil {
			return Baseline{}, "", fmt.Errorf("generate baseline id: %w", err)
		}
		b.ID = id
	}
	if b.Data == "" {
		b.Data = "{}"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if _, err := tx.Exec(`
		INSERT INTO baselines (id, project_id, chart_id, name, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, b.ID, b.ProjectID, b.ChartID, b.Name, b.Data, now); err != nil {
		return Baseline{}, "", err
	}
	saved, err := getBaselineTx(tx, b.ID)
	if err != nil {
		return Baseline{}, "", err
	}
	afterJSON, err := baselineAuditJSON(saved)
	if err != nil {
		return Baseline{}, "", err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{
		ProjectID:  saved.ProjectID,
		EventType:  "baseline.create",
		EntityType: "baseline",
		EntityID:   saved.ID,
		AfterJSON:  afterJSON,
	}); err != nil {
		return Baseline{}, "", err
	}
	return saved, afterJSON, nil
}

func getBaselineTx(tx *sql.Tx, id string) (Baseline, error) {
	return scanBaseline(tx.QueryRow(`
		SELECT id, project_id, chart_id, name, data, created_at
		FROM baselines WHERE id = ?
	`, id))
}

func scanBaseline(row interface {
	Scan(dest ...interface{}) error
}) (Baseline, error) {
	var b Baseline
	var created string
	if err := row.Scan(&b.ID, &b.ProjectID, &b.ChartID, &b.Name, &b.Data, &created); err != nil {
		return Baseline{}, ErrNoBaseline
	}
	b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return b, nil
}

func baselineAuditJSON(b Baseline) (string, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
