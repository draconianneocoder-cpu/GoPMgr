// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"testing"
)

// failingReaderAtCall swaps crypto/rand.Reader (the stdlib's own
// documented indirection point for randomness, not a seam this package
// adds) to force EncryptBuffer's salt/nonce-generation error branches
// one at a time. Only the failAtCall-th Read call fails; every other
// call delegates to the real rand.Reader, so a test can isolate exactly
// one of the two io.ReadFull calls without the other one's error
// masking it -- a reader that failed permanently after the first
// failure would make "remove the salt check" and "remove the nonce
// check" mutations indistinguishable, since the ignored salt error
// would just cascade into the nonce read failing too.
//
// These tests must NOT call t.Parallel(): crypto/rand.Reader is a
// process-global var, and any test running concurrently while it's
// swapped (e.g. certificate/key generation elsewhere in this package)
// would silently get broken randomness. Restoring via t.Cleanup keeps
// this scoped to a single sequential test.
type failingReaderAtCall struct {
	orig       io.Reader // the real rand.Reader, captured before the swap
	failAtCall int       // 1-indexed
	call       int
}

func (f *failingReaderAtCall) Read(p []byte) (int, error) {
	f.call++
	if f.call == f.failAtCall {
		return 0, errors.New("injected rand.Reader failure")
	}
	// Delegate to the captured original, NOT the package-level rand.Reader
	// var -- that var is still pointed at this same fake reader for the
	// rest of the test, so calling io.ReadFull(rand.Reader, p) here would
	// recurse into Read again indefinitely once past the failing call.
	return io.ReadFull(f.orig, p)
}

func withRandReaderFailingAtCall(t *testing.T, failAtCall int) {
	t.Helper()
	orig := rand.Reader
	rand.Reader = &failingReaderAtCall{orig: orig, failAtCall: failAtCall}
	t.Cleanup(func() { rand.Reader = orig })
}

// TestEncryptBuffer_SaltGenerationFails forces the first io.ReadFull
// (salt) to fail; the second (nonce), if reached, would succeed --
// isolating this from the nonce-generation branch below.
func TestEncryptBuffer_SaltGenerationFails(t *testing.T) {
	withRandReaderFailingAtCall(t, 1)
	if _, err := EncryptBuffer([]byte("data"), "password"); err == nil {
		t.Fatal("expected an error when rand.Reader fails generating the salt")
	}
}

// TestEncryptBuffer_NonceGenerationFails lets the first read (salt)
// succeed and fails only the second (nonce).
func TestEncryptBuffer_NonceGenerationFails(t *testing.T) {
	withRandReaderFailingAtCall(t, 2)
	if _, err := EncryptBuffer([]byte("data"), "password"); err == nil {
		t.Fatal("expected an error when rand.Reader fails generating the nonce")
	}
}

// These tests call Argon2id (64 MiB per invocation) and take roughly
// 0.5–2 s each on modern hardware. Run with -short to skip them.

// The limit is lowered for the test: the real one is 64 GiB.
func TestEncryptBuffer_RefusesInputBeyondTheMessageLimit(t *testing.T) {
	old := maxEncryptBufferLen
	maxEncryptBufferLen = 8
	t.Cleanup(func() { maxEncryptBufferLen = old })

	if _, err := EncryptBuffer(make([]byte, 9), "pw"); !errors.Is(err, ErrBufferTooLarge) {
		t.Fatalf("EncryptBuffer(9 bytes) = %v, want ErrBufferTooLarge", err)
	}
	blob, err := EncryptBuffer(make([]byte, 8), "pw")
	if err != nil {
		t.Fatalf("EncryptBuffer(8 bytes) at the limit: %v", err)
	}
	if plain, err := DecryptBuffer(blob, "pw"); err != nil || len(plain) != 8 {
		t.Fatalf("round trip at the limit = %d bytes, %v", len(plain), err)
	}
	if want := min(math.MaxInt-1024, 1<<36-32); old != want {
		t.Errorf("maxEncryptBufferLen = %d, want %d", old, want)
	}
}

func TestEncryptBuffer_EmptyPassword(t *testing.T) {
	_, err := EncryptBuffer([]byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestDecryptBuffer_EmptyPassword(t *testing.T) {
	_, err := DecryptBuffer([]byte("anything"), "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestDecryptBuffer_TooShort(t *testing.T) {
	// saltLen(16) + nonceSize(12) + GCM overhead(16) = 44 bytes minimum.
	// A 20-byte blob is too short.
	_, err := DecryptBuffer(make([]byte, 20), "password")
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Argon2id-heavy crypto roundtrip in short mode")
	}
	plaintext := []byte("GoPMgr confidential project data — encrypt me.")
	password := "correct-horse-battery-staple"

	blob, err := EncryptBuffer(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptBuffer: %v", err)
	}

	got, err := DecryptBuffer(blob, password)
	if err != nil {
		t.Fatalf("DecryptBuffer: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("roundtrip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestDecryptBuffer_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Argon2id-heavy crypto test in short mode")
	}
	blob, err := EncryptBuffer([]byte("secret"), "correct-password")
	if err != nil {
		t.Fatalf("EncryptBuffer: %v", err)
	}

	_, err = DecryptBuffer(blob, "wrong-password")
	if err == nil {
		t.Fatal("expected error when decrypting with wrong password")
	}
}

func TestEncryptBuffer_FreshNoncePerCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Argon2id-heavy crypto test in short mode")
	}
	plaintext := []byte("same message")
	password := "same-password"

	blob1, err := EncryptBuffer(plaintext, password)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	blob2, err := EncryptBuffer(plaintext, password)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}

	if bytes.Equal(blob1, blob2) {
		t.Error("two encryptions of the same plaintext must differ (fresh salt+nonce)")
	}
}

// Captured using x/crypto v0.58.0 before the dependency change.
func TestDecryptBufferAcceptsPersistedV058Ciphertext(t *testing.T) {
	blob, err := hex.DecodeString("21cf848717c4f8bdbfdea25ea97bcdc74ba008645724b50e62d167b32b59e2ac9cc8cc8961a20b8300e626702fcbf1c5cadf657de99dc9eef9c863613964e25798a00a8a52f00751a36469a2af042fdb81a314")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptBuffer(blob, "gopmgr-compatibility-test-only")
	if err != nil || string(got) != "persisted project compatibility fixture" {
		t.Fatalf("decrypt persisted buffer: got %q, err %v", got, err)
	}
	if _, err := DecryptBuffer(blob, "incorrect-password"); err == nil {
		t.Fatal("persisted buffer accepted wrong password")
	}
	blob[len(blob)-1] ^= 1
	if _, err := DecryptBuffer(blob, "gopmgr-compatibility-test-only"); err == nil {
		t.Fatal("persisted buffer accepted a modified authentication tag")
	}
}
