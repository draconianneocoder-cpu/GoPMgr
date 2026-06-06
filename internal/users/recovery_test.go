// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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

func newRecoveryTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "PMForge"))
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
	if _, err := store.CreateAccount("alice", "Alice", "old password"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	codes, err := store.IssueRecoveryCodes("alice")
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
