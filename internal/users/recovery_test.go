// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"crypto/rand"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

type failingRecoveryReader struct{}

func (failingRecoveryReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

// failingReaderAtCall fails exactly the Nth Read call (1-indexed) and
// succeeds on every other call, delegating to the real rand.Reader
// captured at construction time. Same design as
// internal/crypto/encrypt_test.go's fake of the same shape, and for
// the same two reasons: delegating through the captured `orig` (not
// through the swapped package var) avoids self-referential recursion,
// and delegating via io.ReadFull(f.orig, p) rather than a raw Read
// guarantees each logical caller's read consumes exactly one count --
// a short read would make io.ReadFull call back in, double-counting
// one logical read as two and silently targeting the wrong call site.
type failingReaderAtCall struct {
	orig       io.Reader
	failAtCall int
	call       int
}

func (f *failingReaderAtCall) Read(p []byte) (int, error) {
	f.call++
	if f.call == f.failAtCall {
		return 0, errors.New("injected rand.Reader failure")
	}
	return io.ReadFull(f.orig, p)
}

func withRecoveryRandReaderFailingAtCall(t *testing.T, failAtCall int) {
	t.Helper()
	orig := rand.Reader
	rand.Reader = &failingReaderAtCall{orig: orig, failAtCall: failAtCall}
	t.Cleanup(func() { rand.Reader = orig })
}

func newRecoveryTestStore(t *testing.T) *Store {
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

func TestResetWithRecoveryCodeCanonicalisesPastedWhitespace(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "old password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	codes, err := store.IssueRecoveryCodes("alice", nil)
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	code := strings.ToLower(codes[0])
	pasted := "\t" + strings.ReplaceAll(code, "-", " \n-\t ") + "\n"

	if err := store.ResetWithRecoveryCode("alice", pasted, "new password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}
	if _, err := store.Authenticate("alice", "new password"); err != nil {
		t.Fatalf("Authenticate with reset password: %v", err)
	}
}

func TestGenerateCodeReturnsEntropyFailure(t *testing.T) {
	restoreRand := replaceRecoveryRandReader(t, failingRecoveryReader{})
	defer restoreRand()

	_, err := generateCode()
	if err == nil || !strings.Contains(err.Error(), "recovery: read entropy") {
		t.Fatalf("generateCode error = %v, want read entropy error", err)
	}
}

func replaceRecoveryRandReader(t *testing.T, r io.Reader) func() {
	t.Helper()
	original := rand.Reader
	rand.Reader = r
	return func() {
		rand.Reader = original
	}
}

// TestMigrateRecoveryTable_FailsOnClosedStore forces the CREATE TABLE
// exec itself to fail, by calling migrateRecoveryTable directly on a
// Store whose connection is already closed (Close is idempotent, so
// this doesn't conflict with newRecoveryTestStore's own
// t.Cleanup(store.Close)). This is the migrateRecoveryTable
// equivalent of dek_test.go's TestMigrateDEKColumns_QueryFailsOnClosedStore.
func TestMigrateRecoveryTable_FailsOnClosedStore(t *testing.T) {
	store := newRecoveryTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	err := store.migrateRecoveryTable()
	if err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("migrateRecoveryTable() on closed store = %v, want a closed-database error", err)
	}
}

func TestRemainingRecoveryCodes_RejectsInvalidUsername(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.RemainingRecoveryCodes("a"); err != ErrInvalidUsername {
		t.Fatalf("RemainingRecoveryCodes(invalid username) error = %v, want ErrInvalidUsername", err)
	}
}

func TestRemainingRecoveryCodes_CountsOnlyUnused(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// No codes issued yet.
	n, err := store.RemainingRecoveryCodes("alice")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes (no codes): %v", err)
	}
	if n != 0 {
		t.Fatalf("RemainingRecoveryCodes (no codes) = %d, want 0", n)
	}

	codes, err := store.IssueRecoveryCodes("alice", nil)
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	n, err = store.RemainingRecoveryCodes("alice")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes (fresh codes): %v", err)
	}
	if n != RecoveryCodeCount {
		t.Fatalf("RemainingRecoveryCodes (fresh codes) = %d, want %d", n, RecoveryCodeCount)
	}

	// Using one code (via a reset) must decrement the remaining count.
	if err := store.ResetWithRecoveryCode("alice", codes[0], "brand-new-password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}
	n, err = store.RemainingRecoveryCodes("alice")
	if err != nil {
		t.Fatalf("RemainingRecoveryCodes (one used): %v", err)
	}
	if n != RecoveryCodeCount-1 {
		t.Fatalf("RemainingRecoveryCodes (one used) = %d, want %d", n, RecoveryCodeCount-1)
	}
}

func TestIssueRecoveryCodes_RejectsInvalidUsername(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.IssueRecoveryCodes("a", nil); err != ErrInvalidUsername {
		t.Fatalf("IssueRecoveryCodes(invalid username) error = %v, want ErrInvalidUsername", err)
	}
}

// TestIssueRecoveryCodes_UserExistenceCheckFailsOnClosedStore forces
// the `SELECT COUNT(*) FROM users` existence check to fail, by
// closing the store first. This is the first DB call IssueRecoveryCodes
// makes (ValidateUsername touches no DB), so closed-DB isolates it
// cleanly from the function's later DB calls.
func TestIssueRecoveryCodes_UserExistenceCheckFailsOnClosedStore(t *testing.T) {
	store := newRecoveryTestStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	_, err := store.IssueRecoveryCodes("alice", nil)
	if err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("IssueRecoveryCodes() on closed store = %v, want a closed-database error", err)
	}
}

// TestIssueRecoveryCodes_RejectsNonexistentUser pins the count == 0
// early-return, which is a cost guard as much as a correctness one:
// break-verifying it (deleting the guard) doesn't just change the
// error type, it lets execution run all the way through 8 real
// Argon2id hashes and a full transaction before the recovery_codes ->
// users foreign key finally rejects the insert with a raw driver
// string ("FOREIGN KEY constraint failed") instead of this function's
// own typed ErrNoSuchUser. The FK constraint is a real backstop, not
// this check's replacement.
func TestIssueRecoveryCodes_RejectsNonexistentUser(t *testing.T) {
	store := newRecoveryTestStore(t)
	// "nobody" passes ValidateUsername's format check but was never created.
	if _, err := store.IssueRecoveryCodes("nobody", nil); err != ErrNoSuchUser {
		t.Fatalf("IssueRecoveryCodes(nonexistent user) error = %v, want ErrNoSuchUser", err)
	}
}

// TestIssueRecoveryCodes_GenerateCodeEntropyFailure forces
// generateCode's own entropy read to fail (the first of two rand.Reader
// calls per loop iteration, confirmed by direct measurement: call 1 is
// generateCode's 10-byte read, call 2 is HashPassword's 16-byte salt
// read). Asserts generateCode's own wrapped text, not just err != nil:
// see TestIssueRecoveryCodes_HashPasswordEntropyFailure below, which
// forces the immediately-following call and would otherwise be
// indistinguishable from this one under a deleted guard.
func TestIssueRecoveryCodes_GenerateCodeEntropyFailure(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	restore := replaceRecoveryRandReader(t, failingRecoveryReader{})
	defer restore()

	_, err := store.IssueRecoveryCodes("alice", nil)
	if err == nil || !strings.Contains(err.Error(), "recovery: read entropy") {
		t.Fatalf("IssueRecoveryCodes with failing entropy source = %v, want a read-entropy error", err)
	}
}

// TestIssueRecoveryCodes_HashPasswordEntropyFailure forces
// auth.HashPassword's salt read (rand call #2: generateCode's call #1
// succeeds, HashPassword's call #2 fails) using a call-indexed fake
// rather than always-fail, so generateCode's own guard is proven to
// have run and succeeded before this one fires. Asserts HashPassword's
// own wrapped text ("read salt"), distinct from generateCode's ("read
// entropy") above, so the two tests cannot be satisfied by each
// other's failure.
func TestIssueRecoveryCodes_HashPasswordEntropyFailure(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	withRecoveryRandReaderFailingAtCall(t, 2)

	_, err := store.IssueRecoveryCodes("alice", nil)
	if err == nil || !strings.Contains(err.Error(), "read salt") {
		t.Fatalf("IssueRecoveryCodes with HashPassword entropy failure = %v, want a read-salt error", err)
	}
}

// TestIssueRecoveryCodes_WrapKeyRejectsWrongLengthDEK forces
// crypto.WrapKey's error-propagation line inside the loop (only taken
// when dek != nil) deterministically: WrapKey rejects any dek whose
// length isn't exactly crypto.DEKSize before ever touching
// rand.Reader, so a wrong-length dek fails on the very first loop
// iteration with no entropy tricks needed.
func TestIssueRecoveryCodes_WrapKeyRejectsWrongLengthDEK(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	wrongLengthDEK := make([]byte, 16) // crypto.DEKSize is 32
	_, err := store.IssueRecoveryCodes("alice", wrongLengthDEK)
	if err == nil || !strings.Contains(err.Error(), "DEK") {
		t.Fatalf("IssueRecoveryCodes with wrong-length DEK = %v, want a DEK-length error", err)
	}
}

// TestIssueRecoveryCodes_DeleteFailsOnBlockedTrigger forces the
// `DELETE FROM recovery_codes` step to fail via a SQLite trigger with
// a message distinct from the INSERT-blocking trigger below, so the
// two tests cannot pass on each other's failure. Requires an account
// that already has recovery codes before the trigger is installed:
// SQLite's BEFORE DELETE trigger only fires for rows actually matched
// by the DELETE, and a fresh account has none, so a delete against an
// empty table is a silent no-op that would never reach this trigger.
func TestIssueRecoveryCodes_DeleteFailsOnBlockedTrigger(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.IssueRecoveryCodes("alice", nil); err != nil {
		t.Fatalf("prime existing codes: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_recovery_codes_delete
		BEFORE DELETE ON recovery_codes
		BEGIN
			SELECT RAISE(ABORT, 'recovery codes delete blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := store.IssueRecoveryCodes("alice", nil)
	if err == nil || !strings.Contains(err.Error(), "recovery codes delete blocked") {
		t.Fatalf("IssueRecoveryCodes() error = %v, want the delete trigger's abort message", err)
	}
}

// TestIssueRecoveryCodes_InsertFailsOnBlockedTrigger forces the
// per-code `INSERT INTO recovery_codes` step to fail via a SQLite
// trigger. The DELETE above must be allowed to succeed (there's
// nothing to delete on a fresh account anyway) so only the INSERT is
// blocked -- otherwise this test could pass for the same reason as
// the DELETE test above rather than proving the INSERT branch.
func TestIssueRecoveryCodes_InsertFailsOnBlockedTrigger(t *testing.T) {
	store := newRecoveryTestStore(t)
	if _, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_recovery_codes_insert
		BEFORE INSERT ON recovery_codes
		BEGIN
			SELECT RAISE(ABORT, 'recovery codes insert blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := store.IssueRecoveryCodes("alice", nil)
	if err == nil || !strings.Contains(err.Error(), "recovery codes insert blocked") {
		t.Fatalf("IssueRecoveryCodes() error = %v, want the insert trigger's abort message", err)
	}
}
