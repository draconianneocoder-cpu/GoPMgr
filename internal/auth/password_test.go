// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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

// ----- HashPassword -----

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	_, err := HashPassword("")
	if err == nil || !strings.Contains(err.Error(), "auth: empty password") {
		t.Fatalf("HashPassword(\"\") err = %v, want empty password error", err)
	}
}

// ----- VerifyPassword error branches -----

func TestVerifyPassword_WrongPartCount(t *testing.T) {
	if err := VerifyPassword("pw", "$a$b$c$d"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_WrongAlgorithm(t *testing.T) {
	if err := VerifyPassword("pw", "$notargon$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_BadVersionScan(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=bad$m=65536,t=3,p=4$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_WrongVersion(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=18$m=65536,t=3,p=4$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_BadParamScan(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=19$badparams$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_BadSaltBase64(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=19$m=65536,t=3,p=4$!!!$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_ZeroMemory(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=19$m=0,t=3,p=4$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

func TestVerifyPassword_ZeroTime(t *testing.T) {
	if err := VerifyPassword("pw", "$argon2id$v=19$m=65536,t=0,p=4$c2FsdA$aGFzaA"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("err = %v, want ErrInvalidHash", err)
	}
}

// ----- NeedsRehash -----

func TestNeedsRehash_Malformed(t *testing.T) {
	if !NeedsRehash("notahash") {
		t.Error("malformed string should need rehash")
	}
}

func TestNeedsRehash_WrongAlgorithm(t *testing.T) {
	if !NeedsRehash("$notargon$v=19$m=65536,t=3,p=4$s$h") {
		t.Error("wrong algorithm should need rehash")
	}
}

func TestNeedsRehash_BadParamFormat(t *testing.T) {
	if !NeedsRehash("$argon2id$v=19$badparams$s$h") {
		t.Error("bad param format should need rehash")
	}
}

func TestNeedsRehash_WeakerMemory(t *testing.T) {
	if !NeedsRehash("$argon2id$v=19$m=32768,t=3,p=4$s$h") {
		t.Error("weaker memory should need rehash")
	}
}

func TestNeedsRehash_WeakerTime(t *testing.T) {
	if !NeedsRehash("$argon2id$v=19$m=65536,t=1,p=4$s$h") {
		t.Error("weaker time should need rehash")
	}
}

func TestNeedsRehash_WeakerThreads(t *testing.T) {
	if !NeedsRehash("$argon2id$v=19$m=65536,t=3,p=2$s$h") {
		t.Error("weaker threads should need rehash")
	}
}

func TestNeedsRehash_CurrentParams(t *testing.T) {
	if NeedsRehash("$argon2id$v=19$m=65536,t=3,p=4$s$h") {
		t.Error("current params should not need rehash")
	}
}

func TestNeedsRehash_StrongerParams(t *testing.T) {
	if NeedsRehash("$argon2id$v=19$m=131072,t=4,p=8$s$h") {
		t.Error("stronger params should not need rehash")
	}
}

// ----- HashPassword + VerifyPassword round-trip -----

func TestHashVerifyPassword_RoundTrip(t *testing.T) {
	const password = "correct-horse-battery-staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := VerifyPassword(password, hash); err != nil {
		t.Fatalf("VerifyPassword correct: %v", err)
	}
	if err := VerifyPassword("wrong-password", hash); !errors.Is(err, ErrMismatch) {
		t.Fatalf("VerifyPassword wrong: err = %v, want ErrMismatch", err)
	}
}
