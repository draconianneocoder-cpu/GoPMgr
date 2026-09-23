// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package deletionlog keeps a signed-in user's durable record of deleted
// projects. A project's own audit chain is deleted with the project, so the
// record has to live outside it. It is encrypted like the reusable catalog,
// because system.db is readable before login and must not hold project
// metadata. The log is append-only: triggers refuse UPDATE and DELETE.
package deletionlog

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	appcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

// Outcomes a deletion can reach. A deletion with no outcome was requested
// but the app stopped before it could record whether removal finished.
const (
	OutcomeDeleted = "deleted"
	OutcomeFailed  = "failed"
)

// Request describes a project about to be deleted. ProjectID is empty for a
// file that held no project (an empty, uninitialised database).
type Request struct {
	Actor             string
	ProjectID         string
	ProjectName       string
	Location          string // path relative to the user's projects folder
	AuditEvents       int
	AuditValid        bool
	AuditTerminalHash string
}

// Deletion is one recorded deletion with its outcome, if one was recorded.
type Deletion struct {
	ID                string
	Actor             string
	ProjectID         string
	ProjectName       string
	Location          string
	AuditEvents       int
	AuditValid        bool
	AuditTerminalHash string
	RequestedAt       string
	Outcome           string // OutcomeDeleted, OutcomeFailed, or "" if unknown
	OutcomeAt         string
	Detail            string
}

type Log struct{ db *sql.DB }

// Open opens or creates the encrypted deletion log. Commits are fully synced
// (synchronous=FULL): the "requested" entry must be on disk before a project
// is removed, and the log is written rarely enough that the cost does not
// matter.
func Open(path string, sessionDEK []byte) (*Log, error) {
	if strings.ContainsAny(path, "?#") {
		return nil, errors.New("deletion log: path contains an illegal character")
	}
	key, err := appcrypto.DeriveSubkey(sessionDEK, "gopmgr/deletionlog/sqlcipher/v1")
	if err != nil {
		return nil, err
	}
	defer zero(key)
	hexKey, err := appcrypto.KeyspecHex(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("deletion log: create directory: %w", err)
	}
	dsn := path + "?_pragma_key=x'" + hexKey + "'&_foreign_keys=on&_journal_mode=WAL&_synchronous=FULL&_busy_timeout=5000"
	db, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		return nil, fmt.Errorf("deletion log: open: %w", err)
	}
	if _, err = db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("deletion log: connect: %w", err)
	}
	l := &Log{db: db}
	if err = l.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = privateFiles(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return l, nil
}

func (l *Log) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *Log) migrate() error {
	_, err := l.db.Exec(`
		CREATE TABLE IF NOT EXISTS deletion_events (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			deletion_id TEXT NOT NULL,
			kind TEXT NOT NULL CHECK(kind IN ('requested','deleted','failed')),
			at TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			project_name TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			audit_events INTEGER NOT NULL DEFAULT 0,
			audit_valid INTEGER NOT NULL DEFAULT 0 CHECK(audit_valid IN (0,1)),
			audit_terminal_hash TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT ''
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_deletion_events_one_per_kind
			ON deletion_events(deletion_id, kind);
		CREATE TRIGGER IF NOT EXISTS deletion_events_no_update
			BEFORE UPDATE ON deletion_events
			BEGIN SELECT RAISE(ABORT, 'deletion log is append-only'); END;
		CREATE TRIGGER IF NOT EXISTS deletion_events_no_delete
			BEFORE DELETE ON deletion_events
			BEGIN SELECT RAISE(ABORT, 'deletion log is append-only'); END;`)
	if err != nil {
		return fmt.Errorf("deletion log: migrate: %w", err)
	}
	return nil
}

// Requested records that a deletion is about to happen and returns its ID.
// Callers must not remove anything unless this returns without error.
func (l *Log) Requested(r Request) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	_, err = l.db.Exec(`INSERT INTO deletion_events
		(deletion_id, kind, at, actor, project_id, project_name, location, audit_events, audit_valid, audit_terminal_hash)
		VALUES (?, 'requested', ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, now(), r.Actor, r.ProjectID, r.ProjectName, r.Location, r.AuditEvents, boolInt(r.AuditValid), r.AuditTerminalHash)
	if err != nil {
		return "", fmt.Errorf("deletion log: record request: %w", err)
	}
	return id, nil
}

// Deleted records that the requested deletion completed.
func (l *Log) Deleted(deletionID string) error {
	return l.outcome(deletionID, OutcomeDeleted, "")
}

// Failed records that the requested deletion did not complete.
func (l *Log) Failed(deletionID string, cause error) error {
	detail := ""
	if cause != nil {
		detail = cause.Error()
	}
	return l.outcome(deletionID, OutcomeFailed, detail)
}

func (l *Log) outcome(deletionID, kind, detail string) error {
	res, err := l.db.Exec(`INSERT INTO deletion_events (deletion_id, kind, at, detail)
		SELECT ?, ?, ?, ?
		WHERE EXISTS (SELECT 1 FROM deletion_events WHERE deletion_id = ? AND kind = 'requested')
		  AND NOT EXISTS (SELECT 1 FROM deletion_events WHERE deletion_id = ? AND kind IN ('deleted','failed'))`,
		deletionID, kind, now(), detail, deletionID, deletionID)
	if err != nil {
		return fmt.Errorf("deletion log: record %s: %w", kind, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return fmt.Errorf("deletion log: record %s: no open request %q", kind, deletionID)
	}
	return nil
}

// List returns every recorded deletion, newest first.
func (l *Log) List() ([]Deletion, error) {
	rows, err := l.db.Query(`
		SELECT r.deletion_id, r.actor, r.project_id, r.project_name, r.location,
		       r.audit_events, r.audit_valid, r.audit_terminal_hash, r.at,
		       COALESCE(o.kind, ''), COALESCE(o.at, ''), COALESCE(o.detail, '')
		FROM deletion_events r
		LEFT JOIN deletion_events o
		  ON o.deletion_id = r.deletion_id AND o.kind IN ('deleted','failed')
		WHERE r.kind = 'requested'
		ORDER BY r.seq DESC`)
	if err != nil {
		return nil, fmt.Errorf("deletion log: list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Deletion
	for rows.Next() {
		var d Deletion
		var valid int
		if err := rows.Scan(&d.ID, &d.Actor, &d.ProjectID, &d.ProjectName, &d.Location,
			&d.AuditEvents, &valid, &d.AuditTerminalHash, &d.RequestedAt,
			&d.Outcome, &d.OutcomeAt, &d.Detail); err != nil {
			return nil, fmt.Errorf("deletion log: list: %w", err)
		}
		d.AuditValid = valid == 1
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deletion log: list: %w", err)
	}
	return out, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return "deletion_" + hex.EncodeToString(b), nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func privateFiles(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	for _, p := range []string{path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
