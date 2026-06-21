// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package crypto provides PMForge's symmetric-encryption and digital-
// signature primitives. The file is intentionally narrow:
//
//   - encrypt.go    AES-256-GCM with Argon2id key derivation
//   - pdf_sign.go   X.509 / RSA / SHA-256 signing for archival PDFs
//
// Anything cryptographic that isn't one of those two things should be
// added as a new file in this package rather than dropped into either
// of the existing ones.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// Parameters for Argon2id. These match the OWASP 2023 cheat-sheet
// recommendation for an interactive desktop application.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32 // AES-256
	saltLen      = 16
)

// EncryptBuffer encrypts `data` with AES-256-GCM. The output format is:
//
//	[salt | nonce | ciphertext+tag]
//	  16      12         len(data)+16
//
// The salt is fresh per call (so the same password produces different
// ciphertext each time), and the key is derived from the password with
// Argon2id. Decrypt with DecryptBuffer below.
//
// This replaces the placeholder from the Gemini transcript, which
// `copy(key, password)`'d the password directly into the AES key — a
// flaw that defeats the entire point of using AES in the first place.
func EncryptBuffer(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("crypto: empty password")
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	out := make([]byte, 0, saltLen+len(nonce)+len(data)+gcm.Overhead())
	out = append(out, salt...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, data, nil)
	return out, nil
}

// DecryptBuffer reverses EncryptBuffer.
func DecryptBuffer(blob []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("crypto: empty password")
	}
	if len(blob) < saltLen+12+16 {
		return nil, errors.New("crypto: ciphertext too short")
	}

	salt := blob[:saltLen]
	rest := blob[saltLen:]

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short for nonce")
	}
	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
