// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"fmt"

	"pmforge/internal/crypto"
)

// Per-user Data Encryption Key plumbing (ADR-001). The DEK is the
// SQLCipher raw key for every .pmforge the user owns. It is stored
// only in wrapped (encrypted) form:
//
//   - users.wrapped_dek_pw       — wrapped by the login password
//   - recovery_codes.wrapped_dek — wrapped by that code's plaintext
//
// so a recovery-code password reset can re-wrap the SAME DEK instead
// of orphaning the user's encrypted projects.

// migrateDEKColumns adds the wrapped-DEK columns to pre-ADR-001
// system databases. Idempotent (probe before ALTER, mirroring
// internal/db's migrateLegacyColumns pattern).
func (s *Store) migrateDEKColumns() error {
	for _, m := range []struct{ table, column, ddl string }{
		{"users", "wrapped_dek_pw",
			"ALTER TABLE users ADD COLUMN wrapped_dek_pw TEXT NOT NULL DEFAULT ''"},
		{"recovery_codes", "wrapped_dek",
			"ALTER TABLE recovery_codes ADD COLUMN wrapped_dek TEXT NOT NULL DEFAULT ''"},
	} {
		rows, err := s.conn.Query("PRAGMA table_info(" + m.table + ")")
		if err != nil {
			return err
		}
		present := false
		for rows.Next() {
			var (
				cid         int
				name, typ   string
				notnull, pk int
				dflt        interface{}
			)
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				_ = rows.Close()
				return err
			}
			if name == m.column {
				present = true
			}
		}
		// Without this, a mid-iteration error could truncate the probe
		// and falsely conclude the column is missing.
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		if !present {
			if _, err := s.conn.Exec(m.ddl); err != nil {
				return fmt.Errorf("add column %s.%s: %w", m.table, m.column, err)
			}
		}
	}
	return nil
}

// UnlockDEK returns the user's DEK, unwrapped with the login
// password. The caller MUST have authenticated first — this function
// trusts the password only as far as GCM authentication does (a
// wrong password fails the unwrap).
//
// Accounts created before ADR-001 have no wrapped DEK yet; the first
// unlock after this code ships generates one lazily and persists the
// password wrap (we hold the verified password right now, which is
// the only moment that is possible).
func (s *Store) UnlockDEK(username, password string) ([]byte, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	var wrapped string
	if err := s.conn.QueryRow(
		`SELECT wrapped_dek_pw FROM users WHERE username = ?`, username,
	).Scan(&wrapped); err != nil {
		return nil, ErrNoSuchUser
	}

	if wrapped == "" {
		dek, err := crypto.GenerateDEK()
		if err != nil {
			return nil, err
		}
		blob, err := crypto.WrapKey(dek, password)
		if err != nil {
			return nil, err
		}
		if _, err := s.conn.Exec(
			`UPDATE users SET wrapped_dek_pw = ? WHERE username = ?`,
			blob, username,
		); err != nil {
			return nil, err
		}
		return dek, nil
	}

	return crypto.UnwrapKey(wrapped, password)
}

// HasLegacyRecoveryCodeWraps reports whether any active recovery code
// lacks a wrapped DEK. Such codes must be reissued before encrypting
// project databases, otherwise a future password reset would generate
// a fresh DEK and orphan encrypted projects.
func (s *Store) HasLegacyRecoveryCodeWraps(username string) (bool, error) {
	if err := ValidateUsername(username); err != nil {
		return false, err
	}
	var count int
	err := s.conn.QueryRow(
		`SELECT COUNT(*) FROM recovery_codes WHERE username = ? AND used = 0 AND wrapped_dek = ''`,
		username,
	).Scan(&count)
	return count > 0, err
}
