// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopmgr/internal/crypto"
	"gopmgr/internal/sqlitedriver"
)

// OpenEncryptedReadOnly opens an existing SQLCipher project for reading.
// It does not create the main database, run Migrate, change journal mode, or
// chmod the file. mode=ro is set on the URI so the driver's READWRITE|CREATE
// open flags do not apply. _query_only is also set, and is not the only control.
// SQLite may create WAL/SHM coordination files; existing WAL data is not changed.
func OpenEncryptedReadOnly(path string, dek []byte) (*Database, error) {
	path, err := canonicalReadSource(path)
	if err != nil {
		return nil, err
	}
	dsn, err := readOnlyEncryptedDSN(path, dek)
	if err != nil {
		return nil, err
	}
	return openReadOnly(path, dsn)
}

// OpenReadOnly opens an existing plaintext project without migrating it.
func OpenReadOnly(path string) (*Database, error) {
	path, err := canonicalReadSource(path)
	if err != nil {
		return nil, err
	}
	dsn, err := readOnlyFileURI(path, "")
	if err != nil {
		return nil, err
	}
	return openReadOnly(path, dsn)
}

// Resolve symlinks before cleaning .. so SQLite and file guards share the
// same source identity. This does not defend against concurrent link changes.
func canonicalReadSource(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

func openReadOnly(path, dsn string) (*Database, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("db: read-only project is not a regular file: %s", path)
	}
	conn, err := sql.Open(sqlitedriver.Name, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := applyStandardPragmas(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Database{Conn: conn, Path: path}, nil
}

func readOnlyEncryptedDSN(path string, dek []byte) (string, error) {
	if _, err := encryptedDSN(path, dek); err != nil {
		return "", err
	}
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		return "", err
	}
	return readOnlyFileURI(path, hexKey)
}

// readOnlyFileURI builds the DSN sqlite3_open_v2 receives unchanged because
// it starts with file:. The path is percent-encoded so a space cannot split
// the URI. Reject '?' and '#' to match the encrypted-project path policy.
func readOnlyFileURI(abs, hexKey string) (string, error) {
	if strings.ContainsAny(abs, "?#") {
		return "", fmt.Errorf("db: project path contains an illegal character: %q", abs)
	}
	slash := filepath.ToSlash(abs)
	var b strings.Builder
	b.WriteString("file://")
	if !strings.HasPrefix(slash, "/") {
		b.WriteByte('/')
	}
	for i := 0; i < len(slash); i++ {
		c := slash[i]
		if isReadOnlyURIChar(c) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	b.WriteString("?mode=ro&_query_only=1&_foreign_keys=on&_busy_timeout=5000")
	if hexKey != "" {
		b.WriteString("&_pragma_key=x'")
		b.WriteString(hexKey)
		b.WriteByte('\'')
	}
	return b.String(), nil
}

func isReadOnlyURIChar(c byte) bool {
	return c == '/' || c == ':' || c == '.' || c == '-' || c == '_' ||
		(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// CheckExportDestination prevents report output from replacing this project's
// database or SQLite journals. Existing file aliases and symlinked parent
// directories are resolved before the caller opens the output for writing.
func (db *Database) CheckExportDestination(path string) error {
	if db.Path == "" {
		return nil // An in-memory handle has no project file to protect.
	}
	source, err := filepath.EvalSymlinks(db.Path)
	if err != nil {
		return fmt.Errorf("resolve export source: %w", err)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	destination, err := resolveExportDestination(path)
	if err != nil {
		return fmt.Errorf("resolve export destination: %w", err)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	destinationInfo, statErr := os.Stat(destination)
	if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		protected := source + suffix
		if destination == protected {
			return fmt.Errorf("db: export destination is project storage")
		}
		if destinationInfo != nil {
			info, err := os.Stat(protected)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if info != nil && os.SameFile(info, destinationInfo) {
				return fmt.Errorf("db: export destination aliases project storage")
			}
		}
		if suffix != "" && strings.HasSuffix(strings.ToLower(destination), suffix) {
			prefix := destination[:len(destination)-len(suffix)]
			info, err := os.Stat(prefix)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if info != nil && os.SameFile(info, sourceInfo) {
				if strings.HasSuffix(destination, suffix) {
					return fmt.Errorf("db: export destination aliases a project journal")
				}
				foldsCase, err := exportDirectoryFoldsCase(source, sourceInfo)
				if err != nil {
					return err
				}
				if foldsCase {
					return fmt.Errorf("db: export destination aliases a project journal")
				}
			}
		}
	}
	return nil
}

// Follow dangling output links as OpenFile would, without creating their
// targets. Existing parents still pass through EvalSymlinks.
func resolveExportDestination(path string) (string, error) {
	for depth := 0; depth <= 255; depth++ {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Abs(resolved)
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		// Split preserves symlink/.. traversal; Dir and Join clean it
		// before the filesystem follows the symlink.
		dir, leaf := filepath.Split(path)
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = dir + target
			}
			path = target
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if dir == "" {
			dir = "."
		}
		parent, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(parent, leaf))
	}
	return "", fmt.Errorf("too many export symlinks")
}

// Probe an existing filename rather than guessing from GOOS: macOS and
// Windows can use case-sensitive directories, and Unix can use case folding.
func exportDirectoryFoldsCase(source string, info os.FileInfo) (bool, error) {
	name := filepath.Base(source)
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		} else if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		} else {
			continue
		}
		probe := filepath.Join(filepath.Dir(source), name[:i]+string(c)+name[i+1:])
		other, err := os.Stat(probe)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return os.SameFile(info, other), nil
	}
	return false, fmt.Errorf("db: cannot establish project journal name case semantics")
}

// CountProjectRecords counts rows for one project. A table this build's
// older files do not have counts as zero. The table name is a fixed
// switch so it is never pasted in from outside the program.
func (db *Database) CountProjectRecords(table, projectID string) (int, error) {
	var query string
	var required string
	switch table {
	case "charts":
		query = `SELECT COUNT(*) FROM charts WHERE project_id = ?`
		required = "id,project_id,kind,title,data,config,template_id,created_at,updated_at"
	case "documents":
		query = `SELECT COUNT(*) FROM documents WHERE project_id = ?`
		required = "id,project_id,kind,title,content,template_id,version,status,created_at,updated_at"
	case "stakeholders":
		query = `SELECT COUNT(*) FROM stakeholders WHERE project_id = ?`
		required = "id,project_id,name,role,organisation,email,phone,category,hourly_rate,contract_value,notes,created_at,updated_at"
	case "audit_events":
		query = `SELECT COUNT(*) FROM audit_events WHERE project_id = ?`
		required = "id,project_id,sequence_number,previous_event_hash,event_hash,event_type,entity_type,entity_id,before_canonical_json,after_canonical_json,user_id,session_id,timestamp_utc,signature_status,signature_blob_optional"
	default:
		return 0, fmt.Errorf("db: unsupported project count %q", table)
	}
	present, err := db.columnSet(table)
	if err != nil || len(present) == 0 {
		return 0, err
	}
	for _, column := range strings.Split(required, ",") {
		if _, ok := present[column]; !ok {
			return 0, fmt.Errorf("db: %s is missing required column %s", table, column)
		}
	}
	var n int
	if err := db.Conn.QueryRow(query, projectID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// legacyReadColumns supplies migration defaults only for known additive
// columns. Both columns and defaults are hardcoded by the domain reader;
// names from the file are used only for membership checks, never as SQL.
func legacyReadColumns(present map[string]struct{}, columns string, defaults map[string]string) string {
	fields := strings.Split(columns, ",")
	for i, name := range fields {
		if _, ok := present[name]; !ok {
			if value, optional := defaults[name]; optional {
				fields[i] = value + " AS " + name
			}
		}
	}
	return strings.Join(fields, ",")
}

func (db *Database) tableExists(name string) (bool, error) {
	var n int
	err := db.Conn.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// HasTableColumns reports whether table exists and has every named column.
// Callers use a false result to skip a query that would fail on an older
// file. They still run the query, and return its error, when the result is
// true. This probe does not validate the rest of the schema: readers must
// not treat a missing required column as an empty dataset.
func (db *Database) HasTableColumns(table string, columns ...string) (bool, error) {
	ok, err := db.tableExists(table)
	if err != nil || !ok {
		return false, err
	}
	have, err := db.columnSet(table)
	if err != nil {
		return false, err
	}
	for _, column := range columns {
		if _, found := have[column]; !found {
			return false, nil
		}
	}
	return true, nil
}
