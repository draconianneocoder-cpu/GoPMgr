// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Key wrapping for ADR-001 (database encryption at rest).
//
// A per-user 32-byte Data Encryption Key (DEK) is the SQLCipher raw
// key for every .pmforge the user owns. The DEK itself is never
// stored in plaintext: it is WRAPPED (encrypted) by secrets the user
// proves knowledge of — the login password and each active recovery
// code. Wrapping reuses EncryptBuffer's Argon2id + AES-256-GCM
// construction, so the KDF parameters stay in one place.

// DEKSize is the SQLCipher raw-key length (AES-256).
const DEKSize = 32

// ErrBadDEK is returned when a DEK of the wrong length is supplied.
var ErrBadDEK = errors.New("crypto: DEK must be exactly 32 bytes")

// GenerateDEK returns a fresh 32-byte random data-encryption key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("crypto: generate DEK: %w", err)
	}
	return dek, nil
}

// WrapKey encrypts the DEK under the given secret (login password or
// canonicalised recovery code) and returns a base64 blob suitable for
// a TEXT column. Each call produces different ciphertext (fresh salt
// + nonce).
func WrapKey(dek []byte, secret string) (string, error) {
	if len(dek) != DEKSize {
		return "", ErrBadDEK
	}
	blob, err := EncryptBuffer(dek, secret)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

// UnwrapKey reverses WrapKey. A wrong secret fails GCM
// authentication and returns DecryptBuffer's error.
func UnwrapKey(wrapped, secret string) ([]byte, error) {
	blob, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, fmt.Errorf("crypto: wrapped DEK is not base64: %w", err)
	}
	dek, err := DecryptBuffer(blob, secret)
	if err != nil {
		return nil, err
	}
	if len(dek) != DEKSize {
		return nil, ErrBadDEK
	}
	return dek, nil
}

// KeyspecHex renders a DEK as the 64-char uppercase hex string used
// in SQLCipher raw keyspecs (`PRAGMA key = "x'<hex>'"` or the DSN
// form `_pragma_key=x'<hex>'`). Raw keyspecs bypass SQLCipher's
// internal KDF — correct here because Argon2id already strengthened
// the wrapping secrets and the DEK itself is full-entropy random.
func KeyspecHex(dek []byte) (string, error) {
	if len(dek) != DEKSize {
		return "", ErrBadDEK
	}
	return fmt.Sprintf("%X", dek), nil
}
