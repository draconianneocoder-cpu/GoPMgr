// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sqlite3 "github.com/mutecomm/go-sqlcipher/v4"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Failure
	}{
		{"nil", nil, FailureOther},
		{"not from SQLite", errors.New("x"), FailureOther},
		{"not a database", sqlite3.Error{Code: sqlite3.ErrNotADB}, FailureNotADatabase},
		{"corrupt", sqlite3.Error{Code: sqlite3.ErrCorrupt}, FailureCorrupt},
		{"busy", sqlite3.Error{Code: sqlite3.ErrBusy}, FailureBusy},
		{"locked", sqlite3.Error{Code: sqlite3.ErrLocked}, FailureBusy},
		{"cannot open", sqlite3.Error{Code: sqlite3.ErrCantOpen}, FailureCantOpen},
		{"read-only", sqlite3.Error{Code: sqlite3.ErrReadonly}, FailureReadOnly},
		{"hot rollback journal", sqlite3.Error{Code: sqlite3.ErrReadonly, ExtendedCode: sqlite3.ErrReadonlyRollback}, FailureNeedsRecovery},
		{"WAL needs recovery", sqlite3.Error{Code: sqlite3.ErrReadonly, ExtendedCode: sqlite3.ErrReadonlyRecovery}, FailureNeedsRecovery},
		{"wrapped", fmt.Errorf("open: %w", sqlite3.Error{Code: sqlite3.ErrBusy}), FailureBusy},
		{"constraint", sqlite3.Error{Code: sqlite3.ErrConstraint}, FailureOther},
	}
	for _, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("%s: Classify = %v, want %v", c.name, got, c.want)
		}
	}
}

// A real wrong-key read is SQLITE_NOTADB.
func TestClassifyWrongKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.db")
	key := func(b byte) string {
		return fmt.Sprintf("?_pragma_key=x'%064x'", []byte{b})
	}
	conn, err := sql.Open(Name, path+key(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TABLE t(x)"); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	other, err := sql.Open(Name, path+key(2))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Close() }()
	_, err = other.Exec("SELECT count(*) FROM t")
	if Classify(err) != FailureNotADatabase {
		t.Fatalf("wrong key: Classify(%v) = %v, want FailureNotADatabase", err, Classify(err))
	}
}
