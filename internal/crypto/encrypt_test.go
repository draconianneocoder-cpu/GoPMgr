// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"testing"
)

// These tests call Argon2id (64 MiB per invocation) and take roughly
// 0.5–2 s each on modern hardware. Run with -short to skip them.

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
	plaintext := []byte("PMForge confidential project data — encrypt me.")
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
