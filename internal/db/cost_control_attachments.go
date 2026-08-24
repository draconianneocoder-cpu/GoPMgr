// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// maxAttachmentBytes bounds one attachment. Chosen to comfortably hold
	// a scanned invoice or delivery photo without letting a single upload
	// dominate the aggregate project cap below.
	maxAttachmentBytes = 10 << 20 // 10 MiB
	// maxAttachmentsPerEntry bounds how many files one ledger row can carry.
	maxAttachmentsPerEntry      = 10
	maxAttachmentFilenameLength = 255
)

// maxAttachmentTotalBytesPerProject bounds the sum of every attachment blob
// in one project. Attachments live inside the project's SQLCipher database
// file, so this is a direct budget against internal/db/backup.go's
// maxBackupProjectSize (1 GiB): a project must stay well under that ceiling
// with attachments included, or a resulting .pmba archive would be
// unrestorable (CreateArchivalBundle does not itself cap project size;
// RestoreArchivalBundle does). 200 MiB leaves ample headroom for the rest of
// a project's normal data.
//
// A package-level var, not a const: proving the boundary check itself works
// at the real 200 MiB scale would mean allocating and writing on that order
// in every test run. Tests lower it with t.Cleanup-scoped restoration
// instead; production code never reassigns it.
var maxAttachmentTotalBytesPerProject int64 = 200 << 20

var (
	ErrAttachmentTooLarge        = errors.New("db: attachment exceeds the per-file size limit")
	ErrAttachmentEntryFull       = errors.New("db: cost entry already has the maximum number of attachments")
	ErrAttachmentProjectBudget   = errors.New("db: attachment would exceed the project's total attachment budget")
	ErrAttachmentEmpty           = errors.New("db: attachment must not be empty")
	ErrAttachmentFilenameInvalid = errors.New("db: attachment filename is invalid")
	ErrCostEntryNotFound         = errors.New("db: cost entry not found in this project")
)

// CostEntryAttachment is metadata for one bounded, SQLCipher-encrypted-at-rest
// attachment BLOB. Confidentiality relies entirely on the project database's
// existing encryption -- see cost_entry_attachments' schema comment in
// sqlite.go. Attachments are append-only: there is no delete, matching the
// ledger row they belong to.
type CostEntryAttachment struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	CostEntryID string `json:"cost_entry_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	CreatedAt   string `json:"created_at"`
}

// MaxAttachmentBytes exposes the per-file attachment size bound to callers
// outside this package (the Wails boundary reads a user-selected file
// through a size-limited reader and needs the same number SaveCostEntryAttachment
// itself enforces, so the two limits cannot drift apart).
func MaxAttachmentBytes() int64 { return maxAttachmentBytes }

// sanitizeAttachmentFilename strips any directory component and rejects a
// filename that is empty, is a bare "." or "..", or exceeds the length
// bound. It never accepts the caller's path separators: only the base name
// is ever considered, matching the traversal-resistance discipline every
// other user-supplied path in this codebase applies before use.
func sanitizeAttachmentFilename(name string) (string, error) {
	name = strings.TrimSpace(filepath.Base(strings.TrimSpace(name)))
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return "", ErrAttachmentFilenameInvalid
	}
	if len(name) > maxAttachmentFilenameLength {
		return "", ErrAttachmentFilenameInvalid
	}
	return name, nil
}

// SaveCostEntryAttachment stores one bounded attachment BLOB for an existing
// ledger row. It enforces, in order: the cost entry exists in this project,
// the filename is safe, the blob is non-empty and within maxAttachmentBytes,
// the entry is under maxAttachmentsPerEntry, and the project's running total
// stays within maxAttachmentTotalBytesPerProject. The audit trail records
// only metadata (filename, content type, size, hash) -- never the blob
// bytes.
func (db *Database) SaveCostEntryAttachment(projectID, costEntryID, filename, contentType string, data []byte) (CostEntryAttachment, error) {
	if projectID == "" || costEntryID == "" {
		return CostEntryAttachment{}, errors.New("cost entry attachment: project and cost entry are required")
	}
	filename, err := sanitizeAttachmentFilename(filename)
	if err != nil {
		return CostEntryAttachment{}, err
	}
	if len(data) == 0 {
		return CostEntryAttachment{}, ErrAttachmentEmpty
	}
	if len(data) > maxAttachmentBytes {
		return CostEntryAttachment{}, ErrAttachmentTooLarge
	}
	contentType = strings.TrimSpace(contentType)
	if len(contentType) > maxLedgerFieldLength {
		contentType = contentType[:maxLedgerFieldLength]
	}

	tx, err := db.Conn.Begin()
	if err != nil {
		return CostEntryAttachment{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var entryProjectID string
	if err = tx.QueryRow(`SELECT project_id FROM cost_entries WHERE id=?`, costEntryID).Scan(&entryProjectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CostEntryAttachment{}, ErrCostEntryNotFound
		}
		return CostEntryAttachment{}, err
	}
	if entryProjectID != projectID {
		return CostEntryAttachment{}, ErrCostEntryNotFound
	}

	var entryCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_entry_attachments WHERE cost_entry_id=?`, costEntryID).Scan(&entryCount); err != nil {
		return CostEntryAttachment{}, err
	}
	if entryCount >= maxAttachmentsPerEntry {
		return CostEntryAttachment{}, ErrAttachmentEntryFull
	}

	var projectTotal int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(size_bytes),0) FROM cost_entry_attachments WHERE project_id=?`, projectID).Scan(&projectTotal); err != nil {
		return CostEntryAttachment{}, err
	}
	if projectTotal+int64(len(data)) > maxAttachmentTotalBytesPerProject {
		return CostEntryAttachment{}, ErrAttachmentProjectBudget
	}

	id, err := newID("attachment")
	if err != nil {
		return CostEntryAttachment{}, err
	}
	sum := sha256.Sum256(data)
	a := CostEntryAttachment{
		ID: id, ProjectID: projectID, CostEntryID: costEntryID, Filename: filename,
		ContentType: contentType, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
	}
	now := captureTimestamp()
	a.CreatedAt = now.text
	if _, err = tx.Exec(
		`INSERT INTO cost_entry_attachments (id,project_id,cost_entry_id,filename,content_type,size_bytes,sha256,blob,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ProjectID, a.CostEntryID, a.Filename, a.ContentType, a.SizeBytes, a.SHA256, data, a.CreatedAt,
	); err != nil {
		return CostEntryAttachment{}, err
	}
	after, err := json.Marshal(a) // metadata only -- CostEntryAttachment carries no blob field.
	if err != nil {
		return CostEntryAttachment{}, err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: projectID, EventType: "cost_entry_attachment.create", EntityType: "cost_entry_attachment", EntityID: a.ID, AfterJSON: string(after)}); err != nil {
		return CostEntryAttachment{}, err
	}
	if err = tx.Commit(); err != nil {
		return CostEntryAttachment{}, err
	}
	return a, nil
}

const attachmentMetaColumns = `id,project_id,cost_entry_id,filename,content_type,size_bytes,sha256,created_at`

func scanAttachmentMeta(row interface{ Scan(...any) error }) (CostEntryAttachment, error) {
	var a CostEntryAttachment
	err := row.Scan(&a.ID, &a.ProjectID, &a.CostEntryID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.SHA256, &a.CreatedAt)
	return a, err
}

// ListCostEntryAttachments returns metadata (never blob bytes) for every
// attachment on one ledger row, oldest first.
func (db *Database) ListCostEntryAttachments(projectID, costEntryID string) ([]CostEntryAttachment, error) {
	rows, err := db.Conn.Query(`SELECT `+attachmentMetaColumns+` FROM cost_entry_attachments WHERE project_id=? AND cost_entry_id=? ORDER BY created_at ASC`, projectID, costEntryID) // timestamp-order-guard-exempt: cost_entry_attachments.created_at is written only through captureTimestamp's fixed-width UTC timestampLayout, so lexicographic order is chronological. See internal/db/cost_control.go's ListCostEntries.
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostEntryAttachment
	for rows.Next() {
		a, err := scanAttachmentMeta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListCostEntryAttachmentsForProject returns metadata (never blob bytes) for
// every attachment across the whole project, oldest first. Used to enumerate
// what an attachments ZIP export needs to stream.
func (db *Database) ListCostEntryAttachmentsForProject(projectID string) ([]CostEntryAttachment, error) {
	rows, err := db.Conn.Query(`SELECT `+attachmentMetaColumns+` FROM cost_entry_attachments WHERE project_id=? ORDER BY created_at ASC`, projectID) // timestamp-order-guard-exempt: same as ListCostEntryAttachments above.
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostEntryAttachment
	for rows.Next() {
		a, err := scanAttachmentMeta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetCostEntryAttachmentBlob fetches one attachment's bytes, scoped to
// projectID so a caller can never read another project's blob by ID alone.
// Callers that need many attachments (e.g. a ZIP export) should call this
// once per ID rather than loading every blob into memory at once --
// maxAttachmentBytes bounds the memory cost of a single call.
func (db *Database) GetCostEntryAttachmentBlob(projectID, attachmentID string) (CostEntryAttachment, []byte, error) {
	var a CostEntryAttachment
	var data []byte
	err := db.Conn.QueryRow(
		`SELECT `+attachmentMetaColumns+`,blob FROM cost_entry_attachments WHERE project_id=? AND id=?`,
		projectID, attachmentID,
	).Scan(&a.ID, &a.ProjectID, &a.CostEntryID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.SHA256, &a.CreatedAt, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return CostEntryAttachment{}, nil, fmt.Errorf("db: attachment not found in this project")
	}
	if err != nil {
		return CostEntryAttachment{}, nil, err
	}
	return a, data, nil
}
