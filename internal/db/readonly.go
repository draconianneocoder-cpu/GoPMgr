// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

// Read-only project access.
//
// InitDB and InitEncryptedDB open a project for writing: they create a
// missing file, switch it to WAL, run Migrate() and tighten its permissions.
// Code that only reads a project (the project list, portfolio analytics,
// deletion records, administrator copies and the CLI's read commands) must
// not do any of that. It opens the file through OpenEncryptedReadOnly or
// OpenReadOnly instead.
//
// The read-only connection is a SQLite URI with mode=ro, which SQLite
// enforces for every pooled connection: writes fail even if query_only is
// turned off, and a missing file is not created. The journal mode is left
// as the file has it. A file already in WAL mode gets its -wal and -shm
// files, as any SQLite reader needs; SQLite gives them the main file's
// permissions. The main file is never written.

var (
	// ErrProjectNotFound means there is no project file at the path.
	ErrProjectNotFound = errors.New("project file not found")
	// ErrProjectUnreadable means the file does not open as a SQLite
	// database with this key: either the key is wrong or the file is not a
	// GoPMgr database. SQLCipher cannot tell the two apart.
	ErrProjectUnreadable = errors.New("project file cannot be read with this account's key, or is not a GoPMgr project")
	// ErrProjectDamaged means SQLite reported the file as corrupt.
	ErrProjectDamaged = errors.New("project file is damaged")
	// ErrProjectBusy means another connection held a lock past the busy
	// timeout.
	ErrProjectBusy = errors.New("project file is busy")
	// ErrProjectInaccessible means the file exists but SQLite could not open
	// it or a file it needs beside it, for example for lack of permission or
	// because a WAL file's -shm cannot be created in a read-only folder.
	ErrProjectInaccessible = errors.New("project file exists but cannot be opened (check its permissions and its folder's)")
	// ErrUnsupportedProjectFormat means the file carries a format stamp
	// this build does not know (ADR-011 §5.1: any nonzero user_version).
	ErrUnsupportedProjectFormat = errors.New("project uses a newer format than this version of GoPMgr supports")
	// ErrProjectNeedsUpgrade means the operation needs the current schema
	// and the file has an older one. Opening the project in GoPMgr upgrades
	// it; a read-only operation never does.
	ErrProjectNeedsUpgrade = errors.New("project uses an older format; open it once in GoPMgr to upgrade it")
)

// SchemaState says whether a file has every table and column the current
// Migrate creates. It compares tables and columns only: a Migrate interrupted
// after adding columns but before its data retrofits (timestamp formats,
// money backfills) is not detected, and such a file reads with its values as
// stored.
type SchemaState int

const (
	// SchemaCurrent: every table and column Migrate creates is present.
	SchemaCurrent SchemaState = iota
	// SchemaOlder: at least one is missing, so Migrate would change the
	// file. Readers that need the current schema must refuse such a file.
	SchemaOlder
)

// OpenEncryptedReadOnly opens an existing SQLCipher project for reading
// only. It returns ErrProjectNotFound, ErrProjectUnreadable,
// ErrProjectDamaged, ErrProjectBusy or ErrUnsupportedProjectFormat (wrapped
// with the underlying error) rather than a generic open failure.
func OpenEncryptedReadOnly(path string, dek []byte) (*Database, SchemaState, error) {
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		return nil, 0, err
	}
	return openReadOnly(path, "&_pragma_key=x'"+hexKey+"'")
}

// OpenReadOnly is OpenEncryptedReadOnly for an unencrypted (legacy) file.
func OpenReadOnly(path string) (*Database, SchemaState, error) {
	return openReadOnly(path, "")
}

func openReadOnly(path, keyParam string) (*Database, SchemaState, error) {
	dsn, err := readOnlyDSN(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, fmt.Errorf("%w: %s", ErrProjectNotFound, path)
	}
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("db: %s is not a regular file", path)
	}
	conn, err := sql.Open(sqlitedriver.Name, dsn+keyParam)
	if err != nil {
		return nil, 0, err
	}
	d := &Database{Conn: conn, Path: path}
	state, err := d.inspectFormat()
	if err != nil {
		_ = conn.Close()
		return nil, 0, err
	}
	return d, state, nil
}

// readOnlyDSN builds the SQLite URI for a read-only connection. SQLite
// decodes %HH escapes in a URI path, so '%' is escaped; '?' and '#' would
// end the path and are refused, as encryptedDSN does. Windows paths become
// file:C:/..., SQLite's documented form; a UNC path (//server/...) would be
// read as a URI authority and is refused.
func readOnlyDSN(path string) (string, error) {
	if strings.ContainsAny(path, "?#") {
		return "", fmt.Errorf("db: project path contains an illegal character: %q", path)
	}
	p := filepath.ToSlash(path)
	if strings.HasPrefix(p, "//") {
		return "", fmt.Errorf("db: network paths are not supported: %q", path)
	}
	p = strings.ReplaceAll(p, "%", "%25")
	return "file:" + p + "?mode=ro&_query_only=1&_busy_timeout=5000", nil
}

// inspectFormat proves the key, refuses a future format stamp and
// classifies the schema. It only reads.
func (db *Database) inspectFormat() (SchemaState, error) {
	var userVersion int64
	if err := db.Conn.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		return 0, classifyOpenError(err)
	}
	if _, err := db.Conn.Exec("SELECT count(*) FROM sqlite_master"); err != nil {
		return 0, classifyOpenError(err)
	}
	if userVersion != 0 {
		return 0, fmt.Errorf("%w (format stamp %d)", ErrUnsupportedProjectFormat, userVersion)
	}
	expected, err := currentSchema()
	if err != nil {
		return 0, err
	}
	have, err := db.tableColumns()
	if err != nil {
		return 0, classifyOpenError(err)
	}
	for table, columns := range expected {
		got, ok := have[table]
		if !ok {
			return SchemaOlder, nil
		}
		for column := range columns {
			if _, ok := got[column]; !ok {
				return SchemaOlder, nil
			}
		}
	}
	return SchemaCurrent, nil
}

// classifyOpenError maps SQLite result codes to the typed errors above,
// keeping the original error wrapped.
func classifyOpenError(err error) error {
	switch sqlitedriver.Classify(err) {
	case sqlitedriver.FailureNotADatabase:
		return fmt.Errorf("%w: %w", ErrProjectUnreadable, err)
	case sqlitedriver.FailureCorrupt:
		return fmt.Errorf("%w: %w", ErrProjectDamaged, err)
	case sqlitedriver.FailureBusy:
		return fmt.Errorf("%w: %w", ErrProjectBusy, err)
	case sqlitedriver.FailureCantOpen, sqlitedriver.FailureReadOnly:
		// Lstat found the file, so this is not "missing". A read-only
		// connection that only reads gets SQLITE_READONLY only when SQLite
		// cannot create a WAL's -shm beside the file.
		return fmt.Errorf("%w: %w", ErrProjectInaccessible, err)
	}
	return err
}

// tableColumns lists every table's columns, ignoring SQLite's own tables.
func (db *Database) tableColumns() (map[string]map[string]struct{}, error) {
	rows, err := db.Conn.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make(map[string]map[string]struct{}, len(tables))
	for _, table := range tables {
		cols, err := db.columnSet(table)
		if err != nil {
			return nil, err
		}
		out[table] = cols
	}
	return out, nil
}

var current struct {
	once   sync.Once
	schema map[string]map[string]struct{}
	err    error
}

// currentSchema is the set of tables and columns Migrate creates, taken by
// running Migrate on an empty in-memory database, so it cannot drift from
// Migrate itself.
func currentSchema() (map[string]map[string]struct{}, error) {
	current.once.Do(func() {
		conn, err := sql.Open(sqlitedriver.Name, ":memory:")
		if err != nil {
			current.err = err
			return
		}
		defer func() { _ = conn.Close() }()
		// Each connection to :memory: is its own database.
		conn.SetMaxOpenConns(1)
		d := &Database{Conn: conn}
		if err := d.Migrate(); err != nil {
			current.err = fmt.Errorf("db: build the current schema: %w", err)
			return
		}
		current.schema, current.err = d.tableColumns()
	})
	return current.schema, current.err
}

// hasTable reports whether the database has the named table.
func (db *Database) hasTable(name string) (bool, error) {
	var n int
	err := db.Conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return n > 0, err
}

// ProjectOverview is what the project list shows for a project.
type ProjectOverview struct {
	ID        string
	Name      string
	Status    string
	Phase     string
	StartDate string
	EndDate   string
	Charts    int
	Documents int
}

// ReadProjectOverview reads the project row's original columns and counts
// its charts and documents without loading them. It needs only the tables
// and columns every supported schema has, so it works on an Older file
// opened read-only. It returns ErrNoProject when there is no project row,
// as GetProject does.
func (db *Database) ReadProjectOverview() (ProjectOverview, error) {
	var ov ProjectOverview
	ok, err := db.hasTable("project")
	if err != nil {
		return ov, err
	}
	if !ok {
		return ov, ErrNoProject
	}
	err = db.Conn.QueryRow(`SELECT id, name, status, phase, start_date, end_date FROM project LIMIT 1`).
		Scan(&ov.ID, &ov.Name, &ov.Status, &ov.Phase, &ov.StartDate, &ov.EndDate)
	if errors.Is(err, sql.ErrNoRows) {
		return ov, ErrNoProject
	}
	if err != nil {
		return ov, err
	}
	if ov.Charts, err = db.countRows("charts", ov.ID); err != nil {
		return ov, err
	}
	if ov.Documents, err = db.countRows("documents", ov.ID); err != nil {
		return ov, err
	}
	return ov, nil
}

// countRows counts a table's rows for a project; a missing table counts as
// empty, which is what Migrate would create.
func (db *Database) countRows(table, projectID string) (int, error) {
	ok, err := db.hasTable(table)
	if err != nil || !ok {
		return 0, err
	}
	var n int
	// #nosec G202 -- table is one of the two literal names passed by ReadProjectOverview.
	err = db.Conn.QueryRow(`SELECT count(*) FROM `+table+` WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

// VerifyAuditChainIfPresent is VerifyAuditChain for a file that may predate
// the audit_events table. Without the table the chain is empty and valid,
// which is what Migrate would create.
func (db *Database) VerifyAuditChainIfPresent(projectID string) (AuditVerification, error) {
	ok, err := db.hasTable("audit_events")
	if err != nil {
		return AuditVerification{}, err
	}
	if !ok {
		return AuditVerification{ProjectID: projectID, Valid: true}, nil
	}
	return db.VerifyAuditChain(projectID)
}
