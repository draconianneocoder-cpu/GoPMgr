// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package auth handles PMForge's local authentication: password
// hashing with Argon2id and constant-time verification.
//
// Password hashes are stored in the PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<base64-salt>$<base64-hash>
//
// This format embeds the algorithm, parameters, salt, and hash in one
// string, so rotating parameters in the future does not require a
// schema migration: we just verify against whatever params each hash
// was generated with and re-hash on the next successful login.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. These match OWASP 2023 recommendations for
// interactive authentication on commodity hardware.
const (
	pwTime    = 3
	pwMemory  = 64 * 1024 // 64 MiB
	pwThreads = 4
	pwKeyLen  = 32
	pwSaltLen = 16

	maxArgon2Threads = 1<<8 - 1
	maxArgon2KeyLen  = 1<<32 - 1
)

// ErrInvalidHash is returned when the stored hash string is malformed.
var ErrInvalidHash = errors.New("auth: invalid password hash format")

// ErrMismatch is returned when the password does not match the hash.
// Callers SHOULD return a generic "invalid credentials" message to the
// user; do not leak which of (username, password) was wrong.
var ErrMismatch = errors.New("auth: password mismatch")

// HashPassword derives an Argon2id hash from password and returns the
// PHC-formatted string. A fresh random salt is generated per call.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("auth: empty password")
	}

	salt := make([]byte, pwSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, pwTime, pwMemory, pwThreads, pwKeyLen)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		pwMemory, pwTime, pwThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

// VerifyPassword checks `password` against a stored PHC hash string.
// Returns nil on success, ErrMismatch on a bad password, or
// ErrInvalidHash if the stored value is malformed.
//
// The hash and the computed key are compared with crypto/subtle to
// avoid timing-based username enumeration.
func VerifyPassword(password, encoded string) error {
	parts := strings.Split(encoded, "$")
	// Expect ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return ErrInvalidHash
	}
	if version != argon2.Version {
		return ErrInvalidHash
	}

	var memory uint32
	var time, threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrInvalidHash
	}
	if memory == 0 || time == 0 || threads == 0 || threads > maxArgon2Threads {
		return ErrInvalidHash
	}
	if len(salt) == 0 || len(want) == 0 || uint64(len(want)) > maxArgon2KeyLen {
		return ErrInvalidHash
	}
	threads8 := uint8(threads)  // #nosec G115 -- bounded by maxArgon2Threads above.
	keyLen := uint32(len(want)) // #nosec G115 -- bounded by maxArgon2KeyLen above.

	got := argon2.IDKey([]byte(password), salt, time, memory, threads8, keyLen)
	if subtle.ConstantTimeCompare(got, want) == 1 {
		return nil
	}
	return ErrMismatch
}

// NeedsRehash reports whether `encoded` was produced with weaker
// parameters than the current defaults. Callers should invoke this on
// every successful login and silently re-hash if it returns true, so
// existing accounts strengthen over time as defaults are increased.
func NeedsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return true
	}
	var memory, time, threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return true
	}
	return memory < pwMemory || time < pwTime || threads < pwThreads
}
