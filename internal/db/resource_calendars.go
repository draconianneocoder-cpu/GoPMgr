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

// ResourceCalendar is a named project-level capacity calendar for one
// schedulable resource. Capacity values are units, where 1.0 is a
// full-time resource; overrides use integer working-day offsets.
type ResourceCalendar struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"project_id"`
	Resource        string          `json:"resource"`
	Name            string          `json:"name"`
	DefaultCapacity float64         `json:"default_capacity"`
	WeeklyCapacity  map[int]float64 `json:"weekly_capacity"`
	Overrides       map[int]float64 `json:"overrides"`
	SkillTags       []string        `json:"skill_tags"`
	Notes           map[int]string  `json:"notes"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ErrNoResourceCalendar is returned by GetResourceCalendar for unknown IDs.
var ErrNoResourceCalendar = errors.New("db: resource calendar not found")

// SaveResourceCalendar inserts or updates a named resource calendar.
func (db *Database) SaveResourceCalendar(c ResourceCalendar) (ResourceCalendar, error) {
	if c.ID == "" {
		id, err := newID("rcal")
		if err != nil {
			return ResourceCalendar{}, fmt.Errorf("generate resource calendar id: %w", err)
		}
		c.ID = id
	}
	if c.DefaultCapacity <= 0 {
		c.DefaultCapacity = 1
	}
	if c.Name == "" {
		c.Name = c.Resource
	}

	weekly, err := marshalJSONMap(c.WeeklyCapacity)
	if err != nil {
		return ResourceCalendar{}, fmt.Errorf("weekly capacity: %w", err)
	}
	overrides, err := marshalJSONMap(c.Overrides)
	if err != nil {
		return ResourceCalendar{}, fmt.Errorf("overrides: %w", err)
	}
	tags, err := marshalJSONSlice(c.SkillTags)
	if err != nil {
		return ResourceCalendar{}, fmt.Errorf("skill tags: %w", err)
	}
	notes, err := marshalJSONMap(c.Notes)
	if err != nil {
		return ResourceCalendar{}, fmt.Errorf("notes: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Conn.Exec(`
		INSERT INTO resource_calendars (
			id, project_id, resource, name, default_capacity,
			weekly_capacity, overrides, skill_tags, notes, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id        = excluded.project_id,
			resource          = excluded.resource,
			name              = excluded.name,
			default_capacity  = excluded.default_capacity,
			weekly_capacity   = excluded.weekly_capacity,
			overrides         = excluded.overrides,
			skill_tags        = excluded.skill_tags,
			notes             = excluded.notes,
			updated_at        = excluded.updated_at
	`, c.ID, c.ProjectID, c.Resource, c.Name, c.DefaultCapacity,
		weekly, overrides, tags, notes, now, now)
	if err != nil {
		return ResourceCalendar{}, err
	}
	return db.GetResourceCalendar(c.ID)
}

// GetResourceCalendar fetches one resource calendar by ID.
func (db *Database) GetResourceCalendar(id string) (ResourceCalendar, error) {
	row := db.Conn.QueryRow(`
		SELECT id, project_id, resource, name, default_capacity,
		       weekly_capacity, overrides, skill_tags, notes, created_at, updated_at
		FROM resource_calendars WHERE id = ?
	`, id)
	return scanResourceCalendar(row)
}

// ListResourceCalendars returns every named capacity calendar for a project.
func (db *Database) ListResourceCalendars(projectID string) ([]ResourceCalendar, error) {
	rows, err := db.Conn.Query(`
		SELECT id, project_id, resource, name, default_capacity,
		       weekly_capacity, overrides, skill_tags, notes, created_at, updated_at
		FROM resource_calendars
		WHERE project_id = ?
		ORDER BY resource ASC, name ASC, created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ResourceCalendar
	for rows.Next() {
		c, err := scanResourceCalendar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteResourceCalendar removes one named resource calendar.
func (db *Database) DeleteResourceCalendar(id string) error {
	_, err := db.Conn.Exec(`DELETE FROM resource_calendars WHERE id = ?`, id)
	return err
}

func scanResourceCalendar(row interface {
	Scan(...interface{}) error
}) (ResourceCalendar, error) {
	var (
		c                ResourceCalendar
		weekly, override string
		tags, notes      string
		created, updated string
	)
	err := row.Scan(
		&c.ID, &c.ProjectID, &c.Resource, &c.Name, &c.DefaultCapacity,
		&weekly, &override, &tags, &notes, &created, &updated,
	)
	if err == sql.ErrNoRows {
		return ResourceCalendar{}, ErrNoResourceCalendar
	}
	if err != nil {
		return ResourceCalendar{}, err
	}
	if err := unmarshalJSONMap(weekly, &c.WeeklyCapacity); err != nil {
		return ResourceCalendar{}, fmt.Errorf("weekly capacity: %w", err)
	}
	if err := unmarshalJSONMap(override, &c.Overrides); err != nil {
		return ResourceCalendar{}, fmt.Errorf("overrides: %w", err)
	}
	if err := unmarshalJSONSlice(tags, &c.SkillTags); err != nil {
		return ResourceCalendar{}, fmt.Errorf("skill tags: %w", err)
	}
	if err := unmarshalJSONMap(notes, &c.Notes); err != nil {
		return ResourceCalendar{}, fmt.Errorf("notes: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return c, nil
}

func marshalJSONMap[T any](m map[int]T) (string, error) {
	if m == nil {
		m = map[int]T{}
	}
	blob, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func unmarshalJSONMap[T any](raw string, target *map[int]T) error {
	if raw == "" {
		*target = map[int]T{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return err
	}
	if *target == nil {
		*target = map[int]T{}
	}
	return nil
}

func marshalJSONSlice[T any](s []T) (string, error) {
	if s == nil {
		s = []T{}
	}
	blob, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func unmarshalJSONSlice[T any](raw string, target *[]T) error {
	if raw == "" {
		*target = []T{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return err
	}
	if *target == nil {
		*target = []T{}
	}
	return nil
}
