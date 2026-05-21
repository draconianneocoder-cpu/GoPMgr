// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"pmforge/internal/auth"
)

// RecoveryCodeCount is the number of one-time codes generated at
// account creation. Eight is the conventional count (matches GitHub /
// 1Password). Each code carries 80 bits of entropy.
const RecoveryCodeCount = 8

// rawCodeBytes is the entropy per code. 10 bytes → 16 base32 chars
// after stripping padding, displayed as two groups of 8 with a dash.
const rawCodeBytes = 10

// ErrInvalidRecoveryCode is returned when the supplied code does not
// match any unused hash for the user. Indistinguishable from "no
// such user" at the GUI level to avoid enumeration.
var ErrInvalidRecoveryCode = errors.New("users: invalid or used recovery code")

// migrateRecoveryTable is called by Store.migrate() (system.db
// migration step). Idempotent; safe to re-run.
func (s *Store) migrateRecoveryTable() error {
	_, err := s.conn.Exec(`
		CREATE TABLE IF NOT EXISTS recovery_codes (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			username    TEXT NOT NULL,
			code_hash   TEXT NOT NULL,         -- Argon2id PHC hash
			used        INTEGER NOT NULL DEFAULT 0,
			used_at     TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_recovery_user ON recovery_codes(username, used);
	`)
	return err
}

// IssueRecoveryCodes generates RecoveryCodeCount fresh codes for the
// given username, hashes them with Argon2id, and stores the hashes.
// The plaintext codes are returned to the caller exactly ONCE — they
// MUST be shown to the user and never persisted in plaintext.
//
// Calling IssueRecoveryCodes a second time invalidates any unused
// previous codes (delete-then-insert in one transaction), matching
// the "rotate codes" UX users expect.
func (s *Store) IssueRecoveryCodes(username string) ([]string, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	// Verify the user exists.
	var count int
	if err := s.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrNoSuchUser
	}

	plain := make([]string, RecoveryCodeCount)
	hashes := make([]string, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}
		hash, err := auth.HashPassword(canonicalise(code))
		if err != nil {
			return nil, err
		}
		plain[i] = code
		hashes[i] = hash
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE username = ?`, username); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, h := range hashes {
		if _, err := tx.Exec(
			`INSERT INTO recovery_codes (username, code_hash, created_at) VALUES (?, ?, ?)`,
			username, h, now,
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return plain, nil
}

// ResetWithRecoveryCode verifies the recovery code, marks it used,
// and updates the user's password hash to a fresh Argon2id hash of
// newPassword. Returns ErrInvalidRecoveryCode on no match.
//
// Verification scans every unused hash for the user and tries
// auth.VerifyPassword on each. The fixed-time Argon2 cost makes this
// O(n) but n ≤ 8, which is fine.
func (s *Store) ResetWithRecoveryCode(username, code, newPassword string) error {
	if err := ValidateUsername(username); err != nil {
		return ErrInvalidRecoveryCode
	}
	if len(newPassword) < 8 {
		return errors.New("users: new password too short")
	}

	rows, err := s.conn.Query(
		`SELECT id, code_hash FROM recovery_codes WHERE username = ? AND used = 0`,
		username,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	canon := canonicalise(code)
	var matchID int64 = -1
	for rows.Next() {
		var id int64
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return err
		}
		if auth.VerifyPassword(canon, hash) == nil {
			matchID = id
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if matchID < 0 {
		return ErrInvalidRecoveryCode
	}

	// Atomically: mark code used + rotate password hash.
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(
		`UPDATE recovery_codes SET used = 1, used_at = ? WHERE id = ?`,
		now, matchID,
	); err != nil {
		return err
	}
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE users SET password_hash = ? WHERE username = ?`,
		newHash, username,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// RemainingRecoveryCodes returns how many unused codes the user has.
// Used by the Settings GUI to nag at 0 or 1.
func (s *Store) RemainingRecoveryCodes(username string) (int, error) {
	if err := ValidateUsername(username); err != nil {
		return 0, err
	}
	var n int
	err := s.conn.QueryRow(
		`SELECT COUNT(*) FROM recovery_codes WHERE username = ? AND used = 0`,
		username,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

// generateCode returns a 16-char base32 token, dashed in the middle
// for legibility. Example: "JBSWY3DP-EHPK3PXP".
func generateCode() (string, error) {
	var buf [rawCodeBytes]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("recovery: read entropy: %w", err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:])
	if len(enc) < 16 {
		return enc, nil
	}
	return enc[:8] + "-" + enc[8:16], nil
}

// canonicalise strips whitespace and dashes from a user-typed code
// and uppercases it. The user might paste "abcd-1234" or "ABCD 1234"
// or "abcd1234" — all three should match the same stored hash.
func canonicalise(s string) string {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}
