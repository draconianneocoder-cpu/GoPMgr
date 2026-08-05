// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestOpenCreatesPrivateRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("root mode = %o, want 700", mode)
	}
}

func TestOpenTightensExistingRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("root mode = %o, want 700", mode)
	}
}

func TestOpenCreatesPrivateSystemDatabaseFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(filepath.Join(root, "system.db"))
	if err != nil {
		t.Fatalf("stat system.db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("system.db mode = %o, want 600", mode)
	}
}

func TestOpenTightensExistingSystemDatabaseFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	dbPath := filepath.Join(root, "system.db")
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		t.Fatalf("write system.db: %v", err)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat system.db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("system.db mode = %o, want 600", mode)
	}
}

func TestCreateAccountTightensExistingUserDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	for _, sub := range []string{
		"alice",
		filepath.Join("alice", "projects"),
		filepath.Join("alice", "certs"),
		filepath.Join("alice", "exports"),
	} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	for _, sub := range []string{"alice", filepath.Join("alice", "projects"), filepath.Join("alice", "certs"), filepath.Join("alice", "exports")} {
		info, err := os.Stat(filepath.Join(root, sub))
		if err != nil {
			t.Fatalf("stat %s: %v", sub, err)
		}
		if mode := info.Mode().Perm(); mode != 0o700 {
			t.Fatalf("%s mode = %o, want 700", sub, mode)
		}
	}
}

func TestAuthenticateReturnsLastLoginUpdateError(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "GoPMgr"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_last_login
		BEFORE UPDATE OF last_login ON users
		BEGIN
			SELECT RAISE(ABORT, 'last login blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err = store.Authenticate("alice", "correct horse battery staple")
	if err == nil || !strings.Contains(err.Error(), "update last_login") {
		t.Fatalf("Authenticate error = %v, want update last_login error", err)
	}
}

func TestAuthenticateReturnsPasswordRehashUpdateError(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "GoPMgr"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	const password = "correct horse battery staple"
	if _, err := store.CreateAccount("alice", "Alice", password, false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(
		`UPDATE users SET password_hash = ? WHERE username = ?`,
		weakPasswordHash(password), "alice",
	); err != nil {
		t.Fatalf("seed weak hash: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_password_rehash
		BEFORE UPDATE OF password_hash ON users
		BEGIN
			SELECT RAISE(ABORT, 'password rehash blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err = store.Authenticate("alice", password)
	if err == nil || !strings.Contains(err.Error(), "persist password rehash") {
		t.Fatalf("Authenticate error = %v, want persist password rehash error", err)
	}
}

// TestCreateAccount_RejectsCaseVariantUsername is a regression test for the
// APFS case-insensitive filesystem collision: "alice" and "Alice" resolve to
// the same directory on macOS, leaking project names across accounts.
// The fix uses lower(username) = lower(?) in the duplicate check.
func TestCreateAccount_RejectsCaseVariantUsername(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "GoPMgr"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount (original): %v", err)
	}
	for _, variant := range []string{"Alice", "ALICE", "aLiCe"} {
		_, got := store.CreateAccount(variant, "Alice", "another-password", false)
		if got != ErrUserExists {
			t.Errorf("CreateAccount(%q) error = %v, want ErrUserExists", variant, got)
		}
	}
}

// TestOpen_RootDirIsFile forces ensurePrivateDir's os.MkdirAll(rootDir, ...)
// to fail by pre-occupying rootDir with a plain file. It asserts the
// specific "mkdir root" wrapper text rather than a bare err != nil check:
// deleting Open's ensurePrivateDir guard doesn't make Open succeed, since
// sql.Open never touches the filesystem (the driver connects lazily) and
// execution falls through to s.migrate()'s first Exec, which fails against
// "<rootDir>/system.db" with an unrelated "unable to open database file"
// error — the same cascading-fallible-path shape as this session's other
// masked-mutation findings, just surfaced by probing before writing the
// assertion instead of by a failed break-verify run.
func TestOpen_RootDirIsFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	if err := os.WriteFile(root, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write root as file: %v", err)
	}

	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "mkdir root") {
		t.Fatalf("Open with rootDir-as-file error = %v, want \"mkdir root\"", err)
	}
	if strings.Contains(err.Error(), "migrate") {
		t.Fatalf("Open error = %v, want the ensurePrivateDir failure, not the migrate cascade", err)
	}
}

// TestOpen_DBPathIsDirectory forces s.migrate()'s first Exec to fail by
// pre-occupying <rootDir>/system.db with a directory, so sqlite's "unable to
// open database file" surfaces through Open's migrate error wrapper.
func TestOpen_DBPathIsDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	if err := os.MkdirAll(filepath.Join(root, "system.db"), 0o700); err != nil {
		t.Fatalf("mkdir system.db: %v", err)
	}

	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "migrate") {
		t.Fatalf("Open with system.db-as-directory error = %v, want a migrate error", err)
	}
}

// TestClose_NilStoreAndNilConnAreNoops covers Close's s == nil || s.conn ==
// nil guard, both ways it can be true: a nil *Store (e.g. a caller that
// never checked Open's error) and a zero-value &Store{} (conn never set).
func TestClose_NilStoreAndNilConnAreNoops(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("(*Store)(nil).Close() = %v, want nil", err)
	}
	if err := (&Store{}).Close(); err != nil {
		t.Fatalf("(&Store{}).Close() = %v, want nil", err)
	}
}

// TestStore_RootDirReturnsConfiguredRoot covers RootDir, a trivial getter
// that no other test happens to call.
func TestStore_RootDirReturnsConfiguredRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if got := store.RootDir(); got != root {
		t.Fatalf("RootDir() = %q, want %q", got, root)
	}
}

// TestMigrate_FailsOnClosedStore forces migrate's own first Exec (the users
// table CREATE) to fail. migrate's two downstream propagation checks
// (migrateRecoveryTable's and migrateDEKColumns's own "if err != nil"
// wrappers) are NOT independently forceable: both run as later statements
// on the same already-open connection with no intervening hook a test can
// break, and SQLite triggers only fire on DML, not the CREATE TABLE DDL
// those functions issue — closing the store trips this first Exec instead,
// every time. Kept as documented, uncovered lines rather than deleted.
func TestMigrate_FailsOnClosedStore(t *testing.T) {
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.migrate(); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("migrate on closed store = %v, want \"database is closed\"", err)
	}
}

// TestMigrateAdminColumn_QueryFailsOnClosedStore forces the PRAGMA
// table_info query itself to fail. The Scan and rows.Err() checks that
// follow it read PRAGMA table_info's fixed six-column shape, which cannot
// be forced to fail without a corrupted SQLite build — the same reasoning
// already applied to dek.go's migrateDEKColumns; kept as documented,
// uncovered lines.
func TestMigrateAdminColumn_QueryFailsOnClosedStore(t *testing.T) {
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.migrateAdminColumn(); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("migrateAdminColumn on closed store = %v, want \"database is closed\"", err)
	}
}

// TestMigrateAdminColumn_AlreadyPresentIsNoop covers the "is_admin already
// exists" short-circuit by opening the same rootDir a second time — the
// first Open's migration already added the column, so the second Open's
// migrateAdminColumn call must take the already-present branch instead of
// re-running the ALTER TABLE (which would error on a duplicate column).
func TestMigrateAdminColumn_AlreadyPresentIsNoop(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	s1, err := Open(root)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	s2, err := Open(root)
	if err != nil {
		t.Fatalf("second Open (should hit the already-present branch, not error): %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
}

// TestSetAdmin_NoSuchUserReturnsError covers SetAdmin's first QueryRow
// (reading the target's current admin status) failing with sql.ErrNoRows
// for a username that was never created. The second QueryRow (the admin
// COUNT) is only reached when the first found an existing admin row, so it
// has no independent failure trigger on the same live connection — same
// reasoning as DeleteAccount's equivalent COUNT query below; kept as a
// documented, uncovered line.
func TestSetAdmin_NoSuchUserReturnsError(t *testing.T) {
	store := openTestStore(t)
	if err := store.SetAdmin("nobody", false); err == nil {
		t.Fatal("SetAdmin(nonexistent user) = nil, want an error")
	}
}

// TestDeleteAccount_NoSuchUserReturnsError is DeleteAccount's analogue of
// TestSetAdmin_NoSuchUserReturnsError above.
func TestDeleteAccount_NoSuchUserReturnsError(t *testing.T) {
	store := openTestStore(t)
	if err := store.DeleteAccount("nobody"); err == nil {
		t.Fatal("DeleteAccount(nonexistent user) = nil, want an error")
	}
}

// TestCreateAccount_RejectsInvalidUsername covers CreateAccount's
// ValidateUsername guard. Unlike Authenticate's same guard (see
// TestAuthenticate_ValidateUsernameShortCircuitsBeforeDBAccess below),
// CreateAccount returns the distinct ErrInvalidUsername sentinel with no
// downstream code path that could produce the same value, so a plain
// equality check already break-verifies it — no closed-store trick needed.
func TestCreateAccount_RejectsInvalidUsername(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("!!", "Bad Name", "passphrase-long", false); err != ErrInvalidUsername {
		t.Fatalf("CreateAccount with invalid username error = %v, want ErrInvalidUsername", err)
	}
}

// TestCreateAccount_DuplicateCheckFailsOnClosedStore forces the
// case-insensitive duplicate-username SELECT to fail.
func TestCreateAccount_DuplicateCheckFailsOnClosedStore(t *testing.T) {
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err := store.CreateAccount("alice", "Alice", "passphrase-long", false)
	if err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("CreateAccount on closed store = %v, want \"database is closed\"", err)
	}
}

// TestCreateAccount_HashPasswordEntropyFailure forces auth.HashPassword's
// salt read to fail by swapping crypto/rand.Reader for an always-failing
// reader (the same package-level swap recovery_test.go already validates).
func TestCreateAccount_HashPasswordEntropyFailure(t *testing.T) {
	store := openTestStore(t)
	restore := replaceRecoveryRandReader(t, failingRecoveryReader{})
	defer restore()

	_, err := store.CreateAccount("alice", "Alice", "passphrase-long", false)
	if err == nil || !strings.Contains(err.Error(), "read salt") {
		t.Fatalf("CreateAccount with failing entropy source = %v, want a \"read salt\" error", err)
	}
}

// TestCreateAccount_ProvisioningFailsWhenDataDirIsFile forces
// ensurePrivateDir's os.MkdirAll to fail while provisioning the new
// account's data directory, by pre-occupying that path with a plain file.
func TestCreateAccount_ProvisioningFailsWhenDataDirIsFile(t *testing.T) {
	store := openTestStore(t)
	dataDir := filepath.Join(store.RootDir(), "alice")
	if err := os.WriteFile(dataDir, []byte("collide"), 0o644); err != nil {
		t.Fatalf("write colliding file: %v", err)
	}

	_, err := store.CreateAccount("alice", "Alice", "passphrase-long", false)
	if err == nil || !strings.Contains(err.Error(), "provision") {
		t.Fatalf("CreateAccount with blocked data dir = %v, want a \"provision\" error", err)
	}
}

// TestCreateAccount_InsertFailsOnBlockedTrigger forces the final INSERT
// with a SQLite trigger, distinct message from every other trigger in this
// package's test files.
func TestCreateAccount_InsertFailsOnBlockedTrigger(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_account_insert
		BEFORE INSERT ON users
		BEGIN
			SELECT RAISE(ABORT, 'account insert blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := store.CreateAccount("alice", "Alice", "passphrase-long", false)
	if err == nil || !strings.Contains(err.Error(), "account insert blocked") {
		t.Fatalf("CreateAccount with blocked insert = %v, want the trigger error", err)
	}
}

// TestAuthenticate_ValidateUsernameShortCircuitsBeforeDBAccess distinguishes
// Authenticate's ValidateUsername guard from a deleted one, even though
// both a present and an absent guard can return the identical ErrNoSuchUser
// value for an invalid-format username (present: the guard itself; absent:
// zero rows from the following SELECT, per ErrNoSuchUser's own merge-in-UI
// doc comment) — so an open-store equality check alone cannot break-verify
// this guard, the same shape as recovery.go's anti-enumeration finding.
// The discriminator is instead WHEN the DB gets touched: closing the store
// first means a present guard still returns ErrNoSuchUser untouched, while
// a deleted guard would reach the closed connection and surface
// "database is closed" instead — a value ErrNoSuchUser can never equal.
func TestAuthenticate_ValidateUsernameShortCircuitsBeforeDBAccess(t *testing.T) {
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := store.Authenticate("!!", "whatever"); err != ErrNoSuchUser {
		t.Fatalf("Authenticate(invalid username, closed store) = %v, want ErrNoSuchUser (guard must run before any DB access)", err)
	}
}

// TestAuthenticate_NoSuchUser covers the real sql.ErrNoRows branch: a
// well-formed username that simply has no matching row, as opposed to the
// ValidateUsername short-circuit above.
func TestAuthenticate_NoSuchUser(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Authenticate("nosuchuser", "whatever"); err != ErrNoSuchUser {
		t.Fatalf("Authenticate(unknown username) = %v, want ErrNoSuchUser", err)
	}
}

// TestAuthenticate_ScanFailsOnCorruptedIsAdminColumn forces the query's
// Scan to fail by writing a non-integer value into is_admin directly —
// SQLite's type affinity stores it as TEXT rather than rejecting the
// UPDATE, and Scan into *int then fails to convert it.
func TestAuthenticate_ScanFailsOnCorruptedIsAdminColumn(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(`UPDATE users SET is_admin = 'not-an-int' WHERE username = 'alice'`); err != nil {
		t.Fatalf("corrupt is_admin: %v", err)
	}

	if _, err := store.Authenticate("alice", "passphrase-long"); err == nil {
		t.Fatal("Authenticate with corrupted is_admin column = nil, want a Scan error")
	}
}

// TestAuthenticate_WrongPasswordReturnsMismatch covers the
// auth.VerifyPassword failure branch.
func TestAuthenticate_WrongPasswordReturnsMismatch(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.Authenticate("alice", "wrong password entirely"); err == nil {
		t.Fatal("Authenticate with wrong password = nil, want an error")
	}
}

// TestAuthenticate_ParsesExistingLastLogin covers the lastLogin != ""
// branch and its successful time.Parse: the first Authenticate call always
// finds last_login empty (fresh account), so a second call is needed to
// exercise the non-empty, successfully-parsed path.
func TestAuthenticate_ParsesExistingLastLogin(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.Authenticate("alice", "passphrase-long"); err != nil {
		t.Fatalf("first Authenticate: %v", err)
	}

	acc, err := store.Authenticate("alice", "passphrase-long")
	if err != nil {
		t.Fatalf("second Authenticate: %v", err)
	}
	if acc.LastLogin.IsZero() {
		t.Fatal("LastLogin is zero after a prior successful Authenticate, want a parsed timestamp")
	}
}

// TestAuthenticate_RehashEntropyFailure forces auth.HashPassword's salt
// read to fail during the transparent-rehash path: seed a weak hash (so
// NeedsRehash reports true after VerifyPassword succeeds), then fail
// crypto/rand.Reader before the rehash's own HashPassword call.
func TestAuthenticate_RehashEntropyFailure(t *testing.T) {
	store := openTestStore(t)
	const password = "correct horse battery staple"
	if _, err := store.CreateAccount("alice", "Alice", password, false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(
		`UPDATE users SET password_hash = ? WHERE username = ?`,
		weakPasswordHash(password), "alice",
	); err != nil {
		t.Fatalf("seed weak hash: %v", err)
	}

	restore := replaceRecoveryRandReader(t, failingRecoveryReader{})
	defer restore()

	_, err := store.Authenticate("alice", password)
	if err == nil || !strings.Contains(err.Error(), "rehash password") {
		t.Fatalf("Authenticate with failing rehash entropy = %v, want a \"rehash password\" error", err)
	}
}

// TestList_QueryFailsOnClosedStore forces List's top-level Query to fail.
func TestList_QueryFailsOnClosedStore(t *testing.T) {
	store := openTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := store.List(); err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("List on closed store = %v, want \"database is closed\"", err)
	}
}

// TestList_ScanFailsOnCorruptedIsAdminColumn is List's analogue of
// TestAuthenticate_ScanFailsOnCorruptedIsAdminColumn above.
func TestList_ScanFailsOnCorruptedIsAdminColumn(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(`UPDATE users SET is_admin = 'not-an-int' WHERE username = 'alice'`); err != nil {
		t.Fatalf("corrupt is_admin: %v", err)
	}

	if _, err := store.List(); err == nil {
		t.Fatal("List with corrupted is_admin column = nil, want a Scan error")
	}
}

// TestList_ParsesExistingLastLogin is List's analogue of
// TestAuthenticate_ParsesExistingLastLogin above.
func TestList_ParsesExistingLastLogin(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.Authenticate("alice", "passphrase-long"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	accs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, a := range accs {
		if a.Username == "alice" {
			found = true
			if a.LastLogin.IsZero() {
				t.Error("alice's LastLogin is zero after a prior successful Authenticate, want a parsed timestamp")
			}
		}
	}
	if !found {
		t.Fatal("alice missing from List() results")
	}
}

// TestEnsurePrivateSQLiteFiles_MainPathMissing forces the main os.Chmod call
// to fail directly, by pointing it at a path that was never created. Not
// reachable through Open (see the disclosure comment on Open's own
// ensurePrivateSQLiteFiles call): by the time Open reaches that call,
// s.migrate() has already written to dbPath, so it necessarily exists.
func TestEnsurePrivateSQLiteFiles_MainPathMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.db")
	if err := ensurePrivateSQLiteFiles(path); err == nil {
		t.Fatal("ensurePrivateSQLiteFiles on a missing path = nil, want an error")
	}
}

func weakPasswordHash(password string) string {
	const (
		memory  = 8 * 1024
		time    = 1
		threads = 1
		keyLen  = 32
	)
	salt := []byte("weak-test-salt!!")
	key := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory, time, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// --- admin role tests -------------------------------------------------------

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "GoPMgr"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return store
}

func TestHasAnyAdmin_FalseOnFreshStore(t *testing.T) {
	store := openTestStore(t)
	got, err := store.HasAnyAdmin()
	if err != nil {
		t.Fatalf("HasAnyAdmin: %v", err)
	}
	if got {
		t.Fatal("HasAnyAdmin = true on empty store, want false")
	}
}

func TestHasAnyAdmin_TrueAfterAdminCreated(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	got, err := store.HasAnyAdmin()
	if err != nil {
		t.Fatalf("HasAnyAdmin: %v", err)
	}
	if !got {
		t.Fatal("HasAnyAdmin = false after admin created, want true")
	}
}

func TestSetAdmin_DemoteSoleAdminReturnsErrLastAdmin(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := store.SetAdmin("alice", false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("SetAdmin sole admin to false: got %v, want ErrLastAdmin", err)
	}
}

func TestSetAdmin_DemoteNonAdminSucceeds(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := store.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	// Demoting a non-admin with exactly one real admin should NOT return ErrLastAdmin.
	if err := store.SetAdmin("bob", false); err != nil {
		t.Fatalf("SetAdmin non-admin to false: got %v, want nil", err)
	}
}

func TestSetAdmin_DemoteSucceedsWhenMultipleAdmins(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount alice: %v", err)
	}
	if _, err := store.CreateAccount("bob", "Bob", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount bob: %v", err)
	}
	if err := store.SetAdmin("alice", false); err != nil {
		t.Fatalf("SetAdmin alice to false: %v", err)
	}
	// Verify bob is still admin.
	accs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, a := range accs {
		if a.Username == "bob" && !a.IsAdmin {
			t.Error("bob should still be admin after alice demoted")
		}
		if a.Username == "alice" && a.IsAdmin {
			t.Error("alice should no longer be admin")
		}
	}
}

func TestDeleteAccount_SoleAdminReturnsErrLastAdmin(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := store.DeleteAccount("alice"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("DeleteAccount sole admin: got %v, want ErrLastAdmin", err)
	}
}

func TestDeleteAccount_StandardUserSucceeds(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := store.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	if err := store.DeleteAccount("bob"); err != nil {
		t.Fatalf("DeleteAccount standard user: %v", err)
	}
	accs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, a := range accs {
		if strings.EqualFold(a.Username, "bob") {
			t.Error("bob still present after DeleteAccount")
		}
	}
}

func TestDeleteAccount_CascadesRecoveryCodes(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("CreateAccount admin: %v", err)
	}
	if _, err := store.CreateAccount("bob", "Bob", "passphrase-long", false); err != nil {
		t.Fatalf("CreateAccount standard: %v", err)
	}
	// Insert a fake recovery code for bob directly via the connection.
	if _, err := store.conn.Exec(
		`INSERT INTO recovery_codes (username, code_hash, used, created_at) VALUES ('bob', 'fakehash', 0, '2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert recovery code: %v", err)
	}
	if err := store.DeleteAccount("bob"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	var n int
	if err := store.conn.QueryRow(
		`SELECT COUNT(*) FROM recovery_codes WHERE username = 'bob'`,
	).Scan(&n); err != nil {
		t.Fatalf("count recovery_codes: %v", err)
	}
	if n != 0 {
		t.Errorf("recovery_codes: got %d rows for deleted user, want 0", n)
	}
}
