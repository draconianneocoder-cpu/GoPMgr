// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

type failingPasswordReader struct{}

func (failingPasswordReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestHashPasswordReturnsEntropyFailure(t *testing.T) {
	restoreRand := replacePasswordRandReader(t, failingPasswordReader{})
	defer restoreRand()

	_, err := HashPassword("valid password")
	if err == nil || !strings.Contains(err.Error(), "auth: read salt") {
		t.Fatalf("HashPassword error = %v, want read salt error", err)
	}
}

func TestVerifyPasswordRejectsOutOfRangeThreadCount(t *testing.T) {
	hash := "$argon2id$v=19$m=65536,t=3,p=256$c2FsdHNhbHRzYWx0MTIzNA$YWJjZA"

	if err := VerifyPassword("password", hash); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyPassword err = %v, want ErrInvalidHash", err)
	}
}

func replacePasswordRandReader(t *testing.T, r io.Reader) func() {
	t.Helper()
	original := rand.Reader
	rand.Reader = r
	return func() {
		rand.Reader = original
	}
}

func TestVerifyPasswordRejectsEmptyDerivedKey(t *testing.T) {
	hash := "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0MTIzNA$"

	if err := VerifyPassword("password", hash); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyPassword err = %v, want ErrInvalidHash", err)
	}
}
