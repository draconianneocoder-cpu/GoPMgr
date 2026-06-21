// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
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
