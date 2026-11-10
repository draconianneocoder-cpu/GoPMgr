// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Hashes of the vendored engine. third_party/go-sqlcipher/GOPMGR.md records
// where each file comes from and how to rebuild sqlite3.c and sqlite3.h.
const (
	// The amalgamation built, unmodified, from the SQLCipher v4.20.0 tag.
	sqlCipherAmalgamationSHA256 = "8cd2347d18886fee4651153fabeeeffb7cec008700987dab1c00261febc4220f"
	sqlCipherHeaderSHA256       = "e36eb6ed4de77e56a633866a8f84ed339785662d0cf79cf099f420b638bf28ec"
	sqlCipherNoticesSHA256      = "02a4080bdb58c9e23bf40af02619b6e374779cb921b68ca1559e63b2ee52b9ac"
	// Every runtime file as vendored.
	sqlCipherRuntimeTreeSHA256 = "72c4ea6774d9760e2dd65c704217967749294ae54660f4ede10d55829b2d1ba4"
	// The files still taken from go-sqlcipher v4.4.2: everything except the
	// engine source, its build flags and GoPMgr's random-source file, with
	// GoPMgr's transaction-cleanup changes to sqlite3.go reversed.
	publishedWrapperTreeSHA256 = "86c8178235210c18cb451ae981443bae4a7cb611166229eac873d574ed707088"
)

// engineFiles are the vendored files that do not come from go-sqlcipher
// v4.4.2 unchanged.
var engineFiles = map[string]bool{"sqlite3.c": true, "sqlite3.h": true, "flags.go": true, "gopmgr_rng.c": true}

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
	if len(files) != 93 {
		t.Fatalf("vendored SQLCipher runtime file count = %d, want 93", len(files))
	}

	tree := sha256.New()
	wrapper := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		writeInventoryLine(tree, name, data)
		switch name {
		case "sqlite3.c":
			assertSHA256(t, name, data, sqlCipherAmalgamationSHA256)
			for _, version := range []string{`#define SQLITE_VERSION        "3.54.1"`, "#define CIPHER_VERSION_NUMBER 4.20.0"} {
				if !strings.Contains(string(data), version) {
					t.Fatalf("sqlite3.c does not contain %q", version)
				}
			}
		case "sqlite3.h":
			assertSHA256(t, name, data, sqlCipherHeaderSHA256)
		case "sqlite3.go":
			data = []byte(restorePristineCommitCleanup(t, string(data)))
		}
		if !engineFiles[name] {
			writeInventoryLine(wrapper, name, data)
		}
	}
	if got := hex.EncodeToString(tree.Sum(nil)); got != sqlCipherRuntimeTreeSHA256 {
		t.Fatalf("SQLCipher runtime tree hash = %s, want %s", got, sqlCipherRuntimeTreeSHA256)
	}
	if got := hex.EncodeToString(wrapper.Sum(nil)); got != publishedWrapperTreeSHA256 {
		t.Fatalf("go-sqlcipher v4.4.2 wrapper tree hash = %s, want %s", got, publishedWrapperTreeSHA256)
	}
}

// TestEngineReportsTheVendoredVersions checks the versions of the engine
// that is actually linked, not just the source text.
func TestEngineReportsTheVendoredVersions(t *testing.T) {
	// cipher_provider answers only on a keyed database.
	conn, err := sql.Open(Name, compatibilityDSN(filepath.Join(t.TempDir(), "version.db"), compatibilityKey))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	var sqlite, cipher, provider string
	if err := conn.QueryRow("SELECT sqlite_version()").Scan(&sqlite); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("PRAGMA cipher_version").Scan(&cipher); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("PRAGMA cipher_provider").Scan(&provider); err != nil {
		t.Fatal(err)
	}
	if sqlite != "3.54.1" || !strings.HasPrefix(cipher, "4.20.0 ") || provider != "libtomcrypt" {
		t.Fatalf("engine = SQLite %s, SQLCipher %q, provider %q; want 3.54.1, 4.20.0, libtomcrypt", sqlite, cipher, provider)
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

// restorePristineCommitCleanup reverses GoPMgr's changes to sqlite3.go and
// checks the result is byte-identical to go-sqlcipher v4.4.2.
func restorePristineCommitCleanup(t *testing.T, patched string) string {
	t.Helper()
	for _, change := range [][2]string{
		{`func (tx *SQLiteTx) Commit() error {
	_, err := tx.c.exec(context.Background(), "COMMIT", nil)
	if err != nil {
		// Preserve the COMMIT error; Rollback owns cleanup and invalidation.
		_ = tx.Rollback()
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
		{`func (tx *SQLiteTx) Rollback() error {
	_, err := tx.c.exec(context.Background(), "ROLLBACK", nil)
	if err != nil {
		// database/sql finishes Tx even if the native transaction remains open.
		tx.c.mu.Lock()
		if tx.c.db != nil && C.sqlite3_get_autocommit(tx.c.db) == 0 {
			tx.c.txFailed = true
		}
		tx.c.mu.Unlock()
	}
	return err
}

`, `func (tx *SQLiteTx) Rollback() error {
	_, err := tx.c.exec(context.Background(), "ROLLBACK", nil)
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
