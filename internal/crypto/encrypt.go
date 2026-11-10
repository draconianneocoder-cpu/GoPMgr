// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package crypto provides GoPMgr's symmetric-encryption and digital-
// signature primitives, kept intentionally narrow per file:
//
//   - encrypt.go            AES-256-GCM with Argon2id key derivation
//   - keywrap.go            ADR-001 per-user DEK wrapping
//   - pdf_sign.go           X.509 / RSA / SHA-256 signing for archival PDFs
//   - pdf_cms.go            CMS/PKCS#7 ASN.1 structures for PAdES
//   - pdf_cms_timestamp.go  RFC 3161 signature-timestamp embedding
//
// A new cryptographic concern should get its own file in this package
// rather than being folded into one of the existing ones.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"math"

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

// ErrBufferTooLarge is returned by EncryptBuffer for an input longer than
// one AES-GCM message can hold.
var ErrBufferTooLarge = errors.New("crypto: buffer is too large to encrypt in one message")

// maxEncryptBufferLen is the largest input EncryptBuffer accepts: the
// AES-GCM single-message limit of 2^36-32 bytes, or on a 32-bit platform
// the largest int less room for the salt, nonce and tag. A variable so a
// test can lower it instead of allocating 64 GiB.
var maxEncryptBufferLen = min(math.MaxInt-1024, 1<<36-32)

// EncryptBuffer encrypts `data` with AES-256-GCM. The output format is:
//
//	[salt | nonce | ciphertext+tag]
//	  16      12         len(data)+16
//
// The salt is fresh per call (so the same password produces different
// ciphertext each time), and the key is derived from the password with
// Argon2id. Decrypt with DecryptBuffer below. Never use the password bytes
// as the AES key directly: that skips key stretching and makes a short or
// guessable password the key itself. An input longer than one AES-GCM
// message can hold is refused with ErrBufferTooLarge.
func EncryptBuffer(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("crypto: empty password")
	}
	// Bound the input before the output is sized from it: that sum must
	// not overflow int, and Seal panics beyond the AES-GCM limit.
	if len(data) > maxEncryptBufferLen {
		return nil, ErrBufferTooLarge
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// aes.NewCipher only errors on a key whose length isn't 16/24/32
	// bytes. key is always exactly argonKeyLen (32) bytes here, so this
	// can't fail today -- but that's a fact about the argonKeyLen
	// constant's current value, not a type-level guarantee like the
	// dead branches deleted from internal/templates/jdm.go. A future
	// edit to argonKeyLen without updating this call would silently
	// need this check, so it stays; not covered by a test, since
	// forcing it would mean passing a deliberately-wrong-length key,
	// which no real call site does.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// cipher.NewGCM errors only when the process runs under Go's FIPS
	// 140-only enforcement mode (crypto/fips140.Enforced(), a
	// process-global flag). Real, not dead code -- but not practically
	// forceable per-test: it's read once at process/package init, not
	// re-evaluated per call, so no test-local override reaches it.
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
	// Unreachable given the length check above: saltLen+12+16 (44) was
	// chosen assuming nonceSize=12 (standard GCM, fixed by using NewGCM
	// rather than NewGCMWithNonceSize), so len(blob) >= 44 always leaves
	// len(rest) >= 28 >= nonceSize. Kept, not deleted -- like
	// aes.NewCipher above, this guards a value relationship (the "12" in
	// that check) rather than a type-level guarantee, and a future
	// change to saltLen or the nonce size without updating both call
	// sites would need it. Not forceable by a test: blob's length is
	// already constrained past this point by the earlier check.
	if len(rest) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short for nonce")
	}
	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}
