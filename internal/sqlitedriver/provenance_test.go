// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	patchedSQLCipherAmalgamationSHA256 = "054c25f4d2637247452338f8277884eeaf67731697a18c7a46b5581c7fae8b78"
	sqlCipherNoticesSHA256             = "afdc16e2d241db3cbada324fbd5b9ed5a06adb51b52928d7cc66b4b75372408b"
	pristineSQLCipherRuntimeTreeSHA256 = "53b55af239a633b04a6fee66f0e5a9a76fdf2f7dc24c57b78e378d4beda6a73b"
	patchedSQLCipherRuntimeTreeSHA256  = "53faf187f7ff6d4306280b06f48f8e9801e7aaaddce345eeb41769bd3675c430"
)

func TestVendoredSQLCipherProvenance(t *testing.T) {
	root := filepath.Join("..", "..", "third_party", "go-sqlcipher")
	notices, err := os.ReadFile(filepath.Join(root, "THIRD_PARTY_NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	assertSHA256(t, "SQLCipher notices", notices, sqlCipherNoticesSHA256)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "GOPMGR.md" || entry.Name() == "THIRD_PARTY_NOTICES.txt" {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) != 92 {
		t.Fatalf("vendored SQLCipher runtime file count = %d, want 92", len(files))
	}

	patched := sha256.New()
	pristine := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		pristineData := data
		if name == "sqlite3.c" {
			pristineData = []byte(restorePristineWALCheckpoint(t, string(data)))
			assertSHA256(t, name+" (patched)", data, patchedSQLCipherAmalgamationSHA256)
		}
		if name == "sqlite3.go" {
			pristineData = []byte(restorePristineCommitCleanup(t, string(data)))
		}
		writeInventoryLine(patched, name, data)
		writeInventoryLine(pristine, name, pristineData)
	}
	if got := hex.EncodeToString(patched.Sum(nil)); got != patchedSQLCipherRuntimeTreeSHA256 {
		t.Fatalf("patched SQLCipher runtime tree hash = %s, want %s", got, patchedSQLCipherRuntimeTreeSHA256)
	}
	if got := hex.EncodeToString(pristine.Sum(nil)); got != pristineSQLCipherRuntimeTreeSHA256 {
		t.Fatalf("pristine SQLCipher runtime tree hash = %s, want %s", got, pristineSQLCipherRuntimeTreeSHA256)
	}
}

func writeInventoryLine(dst hash.Hash, name string, data []byte) {
	sum := sha256.Sum256(data)
	_, _ = dst.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
}

func assertSHA256(t *testing.T, name string, data []byte, want string) {
	t.Helper()
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("%s hash = %s, want %s", name, got, want)
	}
}

// restorePristineCommitCleanup keeps the compatibility helper byte-identical to v4.4.2.
func restorePristineCommitCleanup(t *testing.T, patched string) string {
	t.Helper()
	for _, change := range [][2]string{
		{`func (tx *SQLiteTx) Commit() error {
	_, err := tx.c.exec(context.Background(), "COMMIT", nil)
	if err != nil {
		// database/sql finishes Tx even when SQLite leaves it open. Preserve
		// the COMMIT error, but never reuse a transaction whose cleanup failed.
		if _, rollbackErr := tx.c.exec(context.Background(), "ROLLBACK", nil); rollbackErr != nil {
			tx.c.mu.Lock()
			if tx.c.db != nil && C.sqlite3_get_autocommit(tx.c.db) == 0 {
				tx.c.txFailed = true
			}
			tx.c.mu.Unlock()
		}
	}
	return err
}

`, `func (tx *SQLiteTx) Commit() error {
	_, err := tx.c.exec(context.Background(), "COMMIT", nil)
	if err != nil && err.(Error).Code == C.SQLITE_BUSY {
		// sqlite3 will leave the transaction open in this scenario.
		// However, database/sql considers the transaction complete once we
		// return from Commit() - we must clean up to honour its semantics.
		tx.c.exec(context.Background(), "ROLLBACK", nil)
	}
	return err
}

`},
		{`func (c *SQLiteConn) Close() error {
	rv := C.sqlite3_close_v2(c.db)
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	c.mu.Lock()
	c.db = nil
	last := c.stmts == 0
	c.mu.Unlock()
	if last {
		deleteHandles(c)
	}
	runtime.SetFinalizer(c, nil)
	return nil
}

`, `func (c *SQLiteConn) Close() error {
	rv := C.sqlite3_close_v2(c.db)
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	deleteHandles(c)
	c.mu.Lock()
	c.db = nil
	c.mu.Unlock()
	runtime.SetFinalizer(c, nil)
	return nil
}

`},
		{`func (s *SQLiteStmt) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	// close_v2 keeps a zombie connection alive until its statements finalize.
	// Callback handles must survive the final native rollback during finalize.
	rv := C.sqlite3_finalize(s.s)
	s.s = nil
	runtime.SetFinalizer(s, nil)
	s.c.mu.Lock()
	s.c.stmts--
	closed := s.c.db == nil
	last := closed && s.c.stmts == 0
	s.c.mu.Unlock()
	if last {
		deleteHandles(s.c)
	}
	if rv != C.SQLITE_OK {
		if closed {
			return Error{Code: ErrNo(rv & 0xff), ExtendedCode: ErrNoExtended(rv)}
		}
		return s.c.lastError()
	}
	return nil
}

`, `func (s *SQLiteStmt) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if !s.c.dbConnOpen() {
		return errors.New("sqlite statement with already closed database connection")
	}
	rv := C.sqlite3_finalize(s.s)
	s.s = nil
	if rv != C.SQLITE_OK {
		return s.c.lastError()
	}
	runtime.SetFinalizer(s, nil)
	return nil
}

`},
		{`	txFailed    bool
	stmts       int
`, ``},
		{`// IsValid prevents database/sql from pooling a failed transaction cleanup.
func (c *SQLiteConn) IsValid() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db != nil && !c.txFailed
}

`, ``},
		{`	c.mu.Lock()
	c.stmts++
	c.mu.Unlock()
`, ``},
		{`func (c *SQLiteConn) prepare(ctx context.Context, query string) (driver.Stmt, error) {
	if !c.IsValid() {
		return nil, driver.ErrBadConn
	}
`, `func (c *SQLiteConn) prepare(ctx context.Context, query string) (driver.Stmt, error) {
`},
		{`func (s *SQLiteStmt) query(ctx context.Context, args []namedValue) (driver.Rows, error) {
	if !s.c.IsValid() {
		return nil, driver.ErrBadConn
	}
`, `func (s *SQLiteStmt) query(ctx context.Context, args []namedValue) (driver.Rows, error) {
`},
		{`func (s *SQLiteStmt) exec(ctx context.Context, args []namedValue) (driver.Result, error) {
	if !s.c.IsValid() {
		return nil, driver.ErrBadConn
	}
`, `func (s *SQLiteStmt) exec(ctx context.Context, args []namedValue) (driver.Result, error) {
`},
		{`func (rc *SQLiteRows) Next(dest []driver.Value) error {
	if !rc.s.c.IsValid() {
		return driver.ErrBadConn
	}
`, `func (rc *SQLiteRows) Next(dest []driver.Value) error {
`},
	} {
		if strings.Count(patched, change[0]) != 1 {
			t.Fatal("failed-COMMIT patch markers changed")
		}
		patched = strings.Replace(patched, change[0], change[1], 1)
	}
	assertSHA256(t, "pristine Go driver", []byte(patched), "8e1c03179bf1259c8fbbb983e6932aa9e847692c509c3cfd4cc3fcb6f6bb4071")
	return patched
}
