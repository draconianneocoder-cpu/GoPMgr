// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"errors"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"
)

// Failure is the kind of SQLite failure behind an error, so callers can
// classify errors without importing the driver.
type Failure int

const (
	// FailureOther is any other error, or one that is not from SQLite.
	FailureOther Failure = iota
	// FailureNotADatabase is SQLITE_NOTADB: a wrong SQLCipher key or a
	// file that is not a database. SQLCipher cannot tell the two apart.
	FailureNotADatabase
	// FailureCorrupt is SQLITE_CORRUPT.
	FailureCorrupt
	// FailureBusy is SQLITE_BUSY or SQLITE_LOCKED.
	FailureBusy
	// FailureCantOpen is SQLITE_CANTOPEN: the file or a file SQLite needs
	// beside it (such as a WAL's -shm) could not be opened.
	FailureCantOpen
	// FailureReadOnly is SQLITE_READONLY: SQLite needed to write and could
	// not. On a read-only connection that only reads, this means a file it
	// needs beside the database, such as a WAL's -shm, cannot be created.
	FailureReadOnly
)

// Classify returns the kind of SQLite failure behind err.
func Classify(err error) Failure {
	var se sqlite3.Error
	if !errors.As(err, &se) {
		return FailureOther
	}
	switch se.Code {
	case sqlite3.ErrNotADB:
		return FailureNotADatabase
	case sqlite3.ErrCorrupt:
		return FailureCorrupt
	case sqlite3.ErrBusy, sqlite3.ErrLocked:
		return FailureBusy
	case sqlite3.ErrCantOpen:
		return FailureCantOpen
	case sqlite3.ErrReadonly:
		return FailureReadOnly
	}
	return FailureOther
}
