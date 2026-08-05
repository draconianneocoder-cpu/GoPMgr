// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func newDEKTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.CreateAccount("alice", "Alice", "p4ssw0rd-original", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return s
}

func TestUnlockDEKLazyGenerationAndStability(t *testing.T) {
	s := newDEKTestStore(t)

	// First unlock generates and persists the DEK.
	dek1, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err != nil {
		t.Fatalf("UnlockDEK (first): %v", err)
	}
	if len(dek1) != 32 {
		t.Fatalf("DEK length = %d, want 32", len(dek1))
	}
	// Second unlock returns the SAME DEK (it was persisted, not
	// regenerated).
	dek2, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err != nil {
		t.Fatalf("UnlockDEK (second): %v", err)
	}
	if !bytes.Equal(dek1, dek2) {
		t.Error("DEK changed between unlocks")
	}
}

func TestUnlockDEKWrongPasswordFails(t *testing.T) {
	s := newDEKTestStore(t)
	if _, err := s.UnlockDEK("alice", "p4ssw0rd-original"); err != nil {
		t.Fatalf("priming unlock: %v", err)
	}
	if _, err := s.UnlockDEK("alice", "wrong-password"); err == nil {
		t.Error("wrong password must fail the unwrap")
	}
	if _, err := s.UnlockDEK("nobody", "x"); err == nil {
		t.Error("unknown user must fail")
	}
}

// THE ADR-001 invariant: a password reset via recovery code must
// preserve the DEK, or every encrypted project would be orphaned.
func TestRecoveryResetPreservesDEK(t *testing.T) {
	s := newDEKTestStore(t)

	dek, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}
	codes, err := s.IssueRecoveryCodes("alice", dek)
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}

	// Forget the password; reset with a code.
	if err := s.ResetWithRecoveryCode("alice", codes[3], "brand-new-password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}

	// Old password must no longer unlock; new password must yield the
	// SAME DEK.
	if _, err := s.UnlockDEK("alice", "p4ssw0rd-original"); err == nil {
		t.Error("old password still unlocks after reset")
	}
	got, err := s.UnlockDEK("alice", "brand-new-password")
	if err != nil {
		t.Fatalf("UnlockDEK (new password): %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("DEK changed across recovery reset — encrypted data would be orphaned")
	}

	// And login itself works with the new password.
	if _, err := s.Authenticate("alice", "brand-new-password"); err != nil {
		t.Fatalf("Authenticate after reset: %v", err)
	}
}

// Legacy path: codes issued WITHOUT a DEK wrap (pre-ADR-001) still
// reset the password; the DEK is freshly generated (safe only while
// no encrypted projects exist — enforced by the future
// encryption-enable flow re-issuing codes).
func TestRecoveryResetLegacyCodesFreshDEK(t *testing.T) {
	s := newDEKTestStore(t)

	dek, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}
	codes, err := s.IssueRecoveryCodes("alice", nil) // legacy: no wraps
	if err != nil {
		t.Fatalf("IssueRecoveryCodes: %v", err)
	}
	if err := s.ResetWithRecoveryCode("alice", codes[0], "another-password"); err != nil {
		t.Fatalf("ResetWithRecoveryCode: %v", err)
	}
	got, err := s.UnlockDEK("alice", "another-password")
	if err != nil {
		t.Fatalf("UnlockDEK after legacy reset: %v", err)
	}
	if bytes.Equal(got, dek) {
		t.Error("legacy reset should have generated a FRESH DEK")
	}
}

// TestHasLegacyRecoveryCodeWraps pins the DEK-orphan guard: if any active
// recovery code lacks a wrapped DEK, a future password reset would generate
// a fresh DEK and silently orphan every encrypted project.
func TestHasLegacyRecoveryCodeWraps(t *testing.T) {
	s := newDEKTestStore(t)

	// No codes yet: must not block encryption enablement.
	has, err := s.HasLegacyRecoveryCodeWraps("alice")
	if err != nil {
		t.Fatalf("HasLegacyRecoveryCodeWraps (no codes): %v", err)
	}
	if has {
		t.Error("HasLegacyRecoveryCodeWraps = true before any codes issued")
	}

	// Legacy codes (nil DEK): must signal that codes need re-issuing.
	if _, err := s.IssueRecoveryCodes("alice", nil); err != nil {
		t.Fatalf("IssueRecoveryCodes (nil DEK): %v", err)
	}
	has, err = s.HasLegacyRecoveryCodeWraps("alice")
	if err != nil {
		t.Fatalf("HasLegacyRecoveryCodeWraps (legacy codes): %v", err)
	}
	if !has {
		t.Error("HasLegacyRecoveryCodeWraps = false with nil-DEK codes — DEK-orphan guard broken")
	}

	// Re-issue with DEK: guard must clear.
	dek, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err != nil {
		t.Fatalf("UnlockDEK: %v", err)
	}
	if _, err := s.IssueRecoveryCodes("alice", dek); err != nil {
		t.Fatalf("IssueRecoveryCodes (with DEK): %v", err)
	}
	has, err = s.HasLegacyRecoveryCodeWraps("alice")
	if err != nil {
		t.Fatalf("HasLegacyRecoveryCodeWraps (after re-issue with DEK): %v", err)
	}
	if has {
		t.Error("HasLegacyRecoveryCodeWraps = true after codes re-issued with DEK")
	}
}

func TestDEKMigrationIdempotent(t *testing.T) {
	s := newDEKTestStore(t)
	// Re-running the migration on an already-migrated store must not
	// error (probe-before-ALTER).
	if err := s.migrateDEKColumns(); err != nil {
		t.Fatalf("second migrateDEKColumns: %v", err)
	}
}

// TestMigrateDEKColumns_QueryFailsOnClosedStore forces the PRAGMA
// table_info query itself to fail, by calling migrateDEKColumns
// directly on a Store whose connection is already closed (Close is
// idempotent, so this doesn't conflict with newDEKTestStore's own
// t.Cleanup(s.Close)). Break-verified: under a deleted `if err !=
// nil` guard here, `rows` is nil and the very next line
// (`rows.Next()`) panics rather than continuing past a clean
// assertion -- the test still fails as intended (a panic fails the
// test), just not via the strings.Contains check below.
func TestMigrateDEKColumns_QueryFailsOnClosedStore(t *testing.T) {
	s := newDEKTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	err := s.migrateDEKColumns()
	if err == nil || !strings.Contains(err.Error(), "database is closed") {
		t.Fatalf("migrateDEKColumns() on closed store = %v, want a closed-database error", err)
	}
}

// TestMigrateDEKColumns_AlterFailsWhenTableMissing forces the ALTER
// TABLE step specifically (not the preceding PRAGMA probe): dropping
// "users" makes the probe correctly report the column absent
// (present=false, since PRAGMA table_info of a dropped table returns
// zero rows), so migrateDEKColumns proceeds to ALTER TABLE users ...,
// which then fails with "no such table" -- confirmed directly before
// writing this test that DROP TABLE succeeds cleanly on a store with
// no dependent rows.
func TestMigrateDEKColumns_AlterFailsWhenTableMissing(t *testing.T) {
	s := newDEKTestStore(t)
	if _, err := s.conn.Exec(`DROP TABLE users`); err != nil {
		t.Fatalf("drop users table: %v", err)
	}
	err := s.migrateDEKColumns()
	if err == nil || !strings.Contains(err.Error(), "add column users.wrapped_dek_pw") {
		t.Fatalf("migrateDEKColumns() with users dropped = %v, want an add-column error naming users.wrapped_dek_pw", err)
	}
}

func TestUnlockDEK_RejectsInvalidUsername(t *testing.T) {
	s := newDEKTestStore(t)
	if _, err := s.UnlockDEK("a", "whatever"); err != ErrInvalidUsername {
		t.Fatalf("UnlockDEK(invalid username) error = %v, want ErrInvalidUsername", err)
	}
}

// TestUnlockDEK_GenerateDEKEntropyFailure forces crypto.GenerateDEK's
// rand.Reader read to fail during the lazy-generation path (a fresh
// account has no wrapped_dek_pw yet, so UnlockDEK's first unlock
// always takes this branch). Asserts the specific "generate DEK"
// error text, not just err != nil: on a deleted `if err != nil`
// guard here, execution falls through to WrapKey(nil-or-empty dek,
// password), which independently fails with ErrBadDEK -- a plain
// err != nil check could not tell that cascade apart from this
// branch actually running (break-verified: the weaker assertion
// stayed green under that exact mutation).
func TestUnlockDEK_GenerateDEKEntropyFailure(t *testing.T) {
	s := newDEKTestStore(t)
	restore := replaceRecoveryRandReader(t, failingRecoveryReader{})
	defer restore()

	_, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err == nil || !strings.Contains(err.Error(), "generate DEK") {
		t.Fatalf("UnlockDEK with failing entropy source = %v, want a generate-DEK error", err)
	}
}

// TestUnlockDEK_WrapKeyFailsOnEmptyPassword forces crypto.WrapKey's
// error-propagation line (distinct from GenerateDEK's, just above)
// deterministically: EncryptBuffer rejects an empty password before
// touching rand.Reader at all, so no entropy-source tricks are
// needed. Requires a FRESH account (wrapped_dek_pw == "") so
// UnlockDEK's lazy-generation path is taken and reaches WrapKey --
// any account that already has a real DEK would instead route to
// UnwrapKey, which never calls WrapKey.
func TestUnlockDEK_WrapKeyFailsOnEmptyPassword(t *testing.T) {
	s := newDEKTestStore(t)
	_, err := s.UnlockDEK("alice", "")
	if err == nil || !strings.Contains(err.Error(), "empty password") {
		t.Fatalf("UnlockDEK(empty password) error = %v, want an empty-password error", err)
	}
}

// TestUnlockDEK_PersistFailsOnBlockedUpdate forces the final
// `UPDATE users SET wrapped_dek_pw = ...` to fail via a SQLite
// trigger (matching store_test.go's block_last_login /
// block_password_rehash pattern), and asserts the trigger's own
// abort message rather than just err != nil -- otherwise this test
// could not be told apart from the WrapKey-failure test above under
// a deleted-guard mutation, since both currently just propagate
// "some error".
func TestUnlockDEK_PersistFailsOnBlockedUpdate(t *testing.T) {
	s := newDEKTestStore(t)
	if _, err := s.conn.Exec(`
		CREATE TRIGGER block_wrapped_dek_pw
		BEFORE UPDATE OF wrapped_dek_pw ON users
		BEGIN
			SELECT RAISE(ABORT, 'wrapped dek update blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := s.UnlockDEK("alice", "p4ssw0rd-original")
	if err == nil || !strings.Contains(err.Error(), "wrapped dek update blocked") {
		t.Fatalf("UnlockDEK() error = %v, want the trigger's abort message", err)
	}
}

func TestHasLegacyRecoveryCodeWraps_RejectsInvalidUsername(t *testing.T) {
	s := newDEKTestStore(t)
	if _, err := s.HasLegacyRecoveryCodeWraps("a"); err != ErrInvalidUsername {
		t.Fatalf("HasLegacyRecoveryCodeWraps(invalid username) error = %v, want ErrInvalidUsername", err)
	}
}
