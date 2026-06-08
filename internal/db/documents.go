// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Document is one entry in the unified `documents` table.
//
// Version monotonically increases on every save. Status moves through
// "draft" → "review" → "approved" → "archived" but is enforced at the
// application layer; the database accepts any string so callers can
// add custom states without a migration.
type Document struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Content    string    `json:"content"` // JSON string
	TemplateID string    `json:"template_id"`
	Version    int       `json:"version"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ErrNoDocument is returned when GetDocument can't find the ID.
var ErrNoDocument = errors.New("db: document not found")

// ValidDocumentStatuses lists the canonical lifecycle values.
var ValidDocumentStatuses = []string{"draft", "review", "approved", "archived"}

// SaveDocument inserts or updates a document. On update, version is
// bumped automatically.
func (db *Database) SaveDocument(d Document) (Document, error) {
	if d.ID == "" {
		id, err := newID("doc")
		if err != nil {
			return Document{}, fmt.Errorf("generate document id: %w", err)
		}
		d.ID = id
	}
	if d.Content == "" {
		d.Content = "{}"
	}
	if d.Status == "" {
		d.Status = "draft"
	}
	if d.Version == 0 {
		d.Version = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := db.Conn.Exec(`
		INSERT INTO documents (id, project_id, kind, title, content, template_id, version, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind        = excluded.kind,
			title       = excluded.title,
			content     = excluded.content,
			template_id = excluded.template_id,
			version     = documents.version + 1,
			status      = excluded.status,
			updated_at  = excluded.updated_at
	`,
		d.ID, d.ProjectID, d.Kind, d.Title, d.Content, d.TemplateID,
		d.Version, d.Status, now, now,
	)
	if err != nil {
		return Document{}, err
	}
	return db.GetDocument(d.ID)
}

// GetDocument fetches one document by ID.
func (db *Database) GetDocument(id string) (Document, error) {
	row := db.Conn.QueryRow(`
		SELECT id, project_id, kind, title, content, template_id, version, status, created_at, updated_at
		FROM documents WHERE id = ?
	`, id)
	return scanDocument(row)
}

// ListDocuments returns every document in a project. Pass kind == ""
// to list all kinds.
func (db *Database) ListDocuments(projectID, kind string) ([]Document, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if kind == "" {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, kind, title, content, template_id, version, status, created_at, updated_at
			FROM documents WHERE project_id = ? ORDER BY updated_at DESC
		`, projectID)
	} else {
		rows, err = db.Conn.Query(`
			SELECT id, project_id, kind, title, content, template_id, version, status, created_at, updated_at
			FROM documents WHERE project_id = ? AND kind = ? ORDER BY updated_at DESC
		`, projectID, kind)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Document
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDocument removes a document by ID.
func (db *Database) DeleteDocument(id string) error {
	_, err := db.Conn.Exec(`DELETE FROM documents WHERE id = ?`, id)
	return err
}

func scanDocument(row interface {
	Scan(dest ...interface{}) error
}) (Document, error) {
	var (
		d                Document
		created, updated string
	)
	err := row.Scan(
		&d.ID, &d.ProjectID, &d.Kind, &d.Title, &d.Content, &d.TemplateID,
		&d.Version, &d.Status, &created, &updated,
	)
	if err == sql.ErrNoRows {
		return Document{}, ErrNoDocument
	}
	if err != nil {
		return Document{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
		d.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updated); err == nil {
		d.UpdatedAt = t
	}
	return d, nil
}
