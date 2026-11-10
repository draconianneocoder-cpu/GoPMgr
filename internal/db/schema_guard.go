// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ErrForeignSchemaObject reports a project file containing a trigger, view
// or virtual table. GoPMgr's project schema has never created any of them,
// so such a file was made or altered outside GoPMgr. Opening it normally
// would let its SQL run inside migrations and every later write.
var ErrForeignSchemaObject = errors.New("db: project file contains database objects GoPMgr does not create")

// virtualTableSQL matches a CREATE VIRTUAL TABLE statement however it is
// cased or spaced. It backs up the root-page test in ForeignSchemaObjects.
var virtualTableSQL = regexp.MustCompile(`(?is)^\s*create\s+virtual\s+table\b`)

// ForeignSchemaObjects lists the triggers, views and virtual tables in the
// database as "type name". A crafted file controls the text in
// sqlite_master, so virtual tables are found by structure (a table with no
// root page, as views and triggers also have) as well as by their SQL.
func (db *Database) ForeignSchemaObjects() ([]string, error) {
	rows, err := db.Conn.Query(`SELECT type, name, rootpage, COALESCE(sql, '') FROM sqlite_master`)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var found []string
	for rows.Next() {
		var typ, name, sql string
		var rootpage int64
		if err := rows.Scan(&typ, &name, &rootpage, &sql); err != nil {
			return nil, fmt.Errorf("read schema: %w", err)
		}
		switch kind := strings.ToLower(typ); {
		case kind == "trigger" || kind == "view":
			found = append(found, kind+" "+name)
		case kind == "table" && (rootpage == 0 || virtualTableSQL.MatchString(sql)):
			found = append(found, "virtual table "+name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	sort.Strings(found)
	return found, nil
}

// CheckForeignSchemaObjects opens the project file at path read-only,
// without migrating it, and returns ErrForeignSchemaObject if it contains a
// trigger, view or virtual table. dek is nil for a plaintext file. Run it
// before opening a file from outside this account for writing.
func CheckForeignSchemaObjects(path string, dek []byte) (err error) {
	var check *Database
	if dek == nil {
		check, err = OpenReadOnly(path)
	} else {
		check, err = OpenEncryptedReadOnly(path, dek)
	}
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := check.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	found, err := check.ForeignSchemaObjects()
	if err != nil {
		return err
	}
	if len(found) > 0 {
		return &ForeignSchemaError{Objects: found}
	}
	return nil
}

// ForeignSchemaError lists the objects that made CheckForeignSchemaObjects
// refuse a file. The names come from the file, so they belong in logs, not
// in messages shown to the user; Error reports only how many there are.
type ForeignSchemaError struct {
	Objects []string // "type name", sorted
}

func (e *ForeignSchemaError) Error() string {
	return fmt.Sprintf("%v (%d found)", ErrForeignSchemaObject, len(e.Objects))
}

func (e *ForeignSchemaError) Unwrap() error { return ErrForeignSchemaObject }
