// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestKeyWrapRoundTrip(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	if len(dek) != DEKSize {
		t.Fatalf("DEK length = %d, want %d", len(dek), DEKSize)
	}

	wrapped, err := WrapKey(dek, "correct horse battery staple")
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}
	got, err := UnwrapKey(wrapped, "correct horse battery staple")
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Error("unwrapped DEK differs from original")
	}
}

func TestKeyWrapWrongSecretFails(t *testing.T) {
	dek, _ := GenerateDEK()
	wrapped, _ := WrapKey(dek, "right")
	if _, err := UnwrapKey(wrapped, "wrong"); err == nil {
		t.Error("wrong secret must fail GCM authentication")
	}
}

func TestKeyWrapFreshCiphertextPerCall(t *testing.T) {
	dek, _ := GenerateDEK()
	w1, _ := WrapKey(dek, "s")
	w2, _ := WrapKey(dek, "s")
	if w1 == w2 {
		t.Error("two wraps of the same DEK must not be identical (fresh salt+nonce)")
	}
}

func TestKeyWrapRejectsBadDEK(t *testing.T) {
	if _, err := WrapKey([]byte("short"), "s"); err != ErrBadDEK {
		t.Errorf("WrapKey(short) err = %v, want ErrBadDEK", err)
	}
	if _, err := KeyspecHex([]byte("short")); err != ErrBadDEK {
		t.Errorf("KeyspecHex(short) err = %v, want ErrBadDEK", err)
	}
}

// TestGenerateDEK_RandReaderFails uses the same call-indexed fake
// rand.Reader as encrypt_test.go (shared within this package). Only one
// io.ReadFull call happens here, so there's no cascading-failure risk
// to isolate against, unlike EncryptBuffer's two-call case.
func TestGenerateDEK_RandReaderFails(t *testing.T) {
	withRandReaderFailingAtCall(t, 1)
	if _, err := GenerateDEK(); err == nil {
		t.Fatal("expected an error when rand.Reader fails generating the DEK")
	}
}

func TestWrapKey_PropagatesEncryptBufferError(t *testing.T) {
	dek := bytes.Repeat([]byte{0xCD}, DEKSize)
	if _, err := WrapKey(dek, ""); err == nil {
		t.Fatal("expected an error when the wrapping secret is empty")
	}
}

func TestUnwrapKey_RejectsNonBase64(t *testing.T) {
	if _, err := UnwrapKey("not valid base64!!!", "secret"); err == nil {
		t.Fatal("expected an error for a non-base64 wrapped value")
	}
}

// TestUnwrapKey_RejectsWrongLengthAfterDecrypt exercises UnwrapKey's own
// post-decrypt length check. Deliberately does NOT go through WrapKey:
// WrapKey rejects a wrong-length DEK before ever calling EncryptBuffer,
// so a blob built via WrapKey could never reach this branch. Calling
// EncryptBuffer directly on a wrong-length payload bypasses that guard,
// producing a blob that decrypts successfully but to the wrong length --
// the only way to reach UnwrapKey's own check rather than WrapKey's.
func TestUnwrapKey_RejectsWrongLengthAfterDecrypt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Argon2id-heavy crypto test in short mode")
	}
	blob, err := EncryptBuffer([]byte("not 32 bytes"), "secret")
	if err != nil {
		t.Fatalf("EncryptBuffer: %v", err)
	}
	wrapped := base64.StdEncoding.EncodeToString(blob)

	if _, err := UnwrapKey(wrapped, "secret"); err != ErrBadDEK {
		t.Fatalf("UnwrapKey err = %v, want ErrBadDEK", err)
	}
}

func TestKeyspecHex(t *testing.T) {
	dek := bytes.Repeat([]byte{0xAB}, DEKSize)
	hexspec, err := KeyspecHex(dek)
	if err != nil {
		t.Fatalf("KeyspecHex: %v", err)
	}
	if len(hexspec) != 64 || hexspec != strings.Repeat("AB", 32) {
		t.Errorf("KeyspecHex = %q, want 64 uppercase hex chars", hexspec)
	}
}
