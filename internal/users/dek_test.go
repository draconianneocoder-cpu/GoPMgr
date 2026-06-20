// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"path/filepath"
	"testing"
)

func newDEKTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.CreateAccount("alice", "Alice", "p4ssw0rd-original"); err != nil {
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
