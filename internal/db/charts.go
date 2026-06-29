// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Chart is one entry in the unified `charts` table. The semantic
// validity of (Kind, Data, Config) is the caller's responsibility —
// the database treats Data and Config as opaque JSON text.
type Chart struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Data       string `json:"data"`   // JSON string
	Config     string `json:"config"` // JSON string
	TemplateID string `json:"template_id"`
	// CreatedAt/UpdatedAt are RFC3339Nano strings (server-managed). They
	// are strings rather than time.Time so the Wails bridge can round-trip
	// records whose timestamps are empty (new records) without failing to
	// unmarshal "" into time.Time. RFC3339 sorts lexicographically, so the
	// "ORDER BY updated_at" / string comparisons stay chronological.
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ErrNoChart is returned when GetChart can't find the requested ID.
var ErrNoChart = errors.New("db: chart not found")

// SaveChart inserts or updates a chart. If c.ID is empty, it's set to
// a new ID; the resulting Chart is returned.
func (db *Database) SaveChart(c Chart) (Chart, error) {
	if c.ID == "" {
		id, err := newID("chart")
		if err != nil {
			return Chart{}, fmt.Errorf("generate chart id: %w", err)
		}
		c.ID = id
	}
	if c.Data == "" {
		c.Data = "{}"
	}
	if c.Config == "" {
		c.Config = "{}"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := db.Conn.Exec(`
		INSERT INTO charts (id, project_id, kind, title, data, config, template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind        = excluded.kind,
			title       = excluded.title,
			data        = excluded.data,
			config      = excluded.config,
			template_id = excluded.template_id,
			updated_at  = excluded.updated_at
	`,
		c.ID, c.ProjectID, c.Kind, c.Title, c.Data, c.Config, c.TemplateID, now, now,
	)
	if err != nil {
		return Chart{}, err
	}
	return db.GetChart(c.ID)
}

// GetChart fetches a chart by ID.
func (db *Database) GetChart(id string) (Chart, error) {
	row := db.Conn.QueryRow(`
		SELECT id, project_id, kind, title, data, config, template_id, created_at, updated_at
		FROM charts WHERE id = ?
	`, id)
	return scanChart(row)
}

// ListCharts returns every chart belonging to a project, ordered by
// updated_at descending. Pass kind == "" to list all kinds.
func (db *Database) ListCharts(projectID, kind string) ([]Chart, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if kind == "" {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, kind, title, data, config, template_id, created_at, updated_at
			FROM charts WHERE project_id = ? ORDER BY updated_at DESC
		`, projectID)
	} else {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, kind, title, data, config, template_id, created_at, updated_at
			FROM charts WHERE project_id = ? AND kind = ? ORDER BY updated_at DESC
		`, projectID, kind)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Chart
	for rows.Next() {
		c, err := scanChart(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteChart removes a chart by ID.
func (db *Database) DeleteChart(id string) error {
	_, err := db.Conn.Exec(`DELETE FROM charts WHERE id = ?`, id)
	return err
}

func scanChart(row interface {
	Scan(dest ...interface{}) error
}) (Chart, error) {
	var (
		c                Chart
		created, updated string
	)
	err := row.Scan(
		&c.ID, &c.ProjectID, &c.Kind, &c.Title, &c.Data, &c.Config, &c.TemplateID, &created, &updated,
	)
	if err == sql.ErrNoRows {
		return Chart{}, ErrNoChart
	}
	if err != nil {
		return Chart{}, err
	}
	c.CreatedAt = created
	c.UpdatedAt = updated
	return c, nil
}
