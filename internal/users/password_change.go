// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"database/sql"
	"errors"
	"fmt"

	"gopmgr/internal/auth"
	"gopmgr/internal/crypto"
)

// MinPasswordLength is the shortest password GoPMgr accepts.
const MinPasswordLength = 8

// ErrPasswordTooShort is returned for a password shorter than
// MinPasswordLength.
var ErrPasswordTooShort = fmt.Errorf("users: password must be at least %d characters", MinPasswordLength)

// ErrPasswordChangedElsewhere is returned by ChangePassword when the
// account's password changed between reading it and writing the new one,
// for example through a recovery reset in another GoPMgr process.
var ErrPasswordChangedElsewhere = errors.New("users: password was changed elsewhere")

// ErrPasswordWrapCorrupt is returned by ChangePassword when the current
// password verifies but does not unwrap the stored DEK.
var ErrPasswordWrapCorrupt = errors.New("users: password wrap of the encryption key is corrupt")

// ValidatePassword returns ErrPasswordTooShort for a password shorter than
// MinPasswordLength bytes. The frontend counts JavaScript string length
// (UTF-16 code units); a string's UTF-8 byte length is never smaller, so
// this never refuses a password the frontend accepted.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// beforePasswordSwap runs in ChangePassword after the new hash and wrap
// are prepared and before they are written. Tests replace it to change
// the password in between.
var beforePasswordSwap = func() {}

// ChangePassword replaces username's password, re-wrapping the same DEK
// (ADR-001), so encrypted projects and recovery codes are unaffected.
// currentPassword must verify; a wrong one returns auth.ErrMismatch.
//
// The Argon2id work (verify, unwrap, wrap, hash) happens before any lock
// is taken. The write is a single compare-and-swap on the hash and wrap
// that were read, so a password changed in the meantime is not
// overwritten: ErrPasswordChangedElsewhere is returned instead. It does
// not go through Authenticate, which would stamp last_login and could
// re-hash the password, changing the value the swap compares against.
func (s *Store) ChangePassword(username, currentPassword, newPassword string) error {
	if err := ValidateUsername(username); err != nil {
		return ErrNoSuchUser
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	var hash, wrapped string
	err := s.conn.QueryRow(
		`SELECT password_hash, wrapped_dek_pw FROM users WHERE username = ?`, username,
	).Scan(&hash, &wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchUser
	}
	if err != nil {
		return err
	}
	if err := auth.VerifyPassword(currentPassword, hash); err != nil {
		return err
	}
	if wrapped == "" {
		// Signing in creates the wrap (UnlockDEK), so a signed-in account
		// always has one.
		return ErrPasswordWrapCorrupt
	}
	dek, err := crypto.UnwrapKey(wrapped, currentPassword)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordWrapCorrupt, err)
	}
	defer zero(dek)

	newWrap, err := crypto.WrapKey(dek, newPassword)
	if err != nil {
		return err
	}
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	beforePasswordSwap()
	res, err := s.conn.Exec(
		`UPDATE users SET password_hash = ?, wrapped_dek_pw = ?
		 WHERE username = ? AND password_hash = ? AND wrapped_dek_pw = ?`,
		newHash, newWrap, username, hash, wrapped,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrPasswordChangedElsewhere
	}
	return nil
}

// zero overwrites b in place, so an unwrapped key does not linger in the
// heap (ADR-001).
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
