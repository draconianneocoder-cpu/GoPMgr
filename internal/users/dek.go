// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"fmt"
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
				dflt        any
			)
			// Real error-handling code, not tested: PRAGMA table_info's six
			// output columns (int, text, text, int, any, int) always match
			// these Scan targets' types under normal SQLite operation, so
			// this can't be forced without a corrupted SQLite build --
			// unlike the ALTER failure below, which a dropped table forces
			// cleanly.
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				_ = rows.Close()
				return err
			}
			if name == m.column {
				present = true
			}
		}
		// Without this, a mid-iteration error could truncate the probe
		// and falsely conclude the column is missing. Not tested: this
		// fires only on a mid-iteration I/O error against the PRAGMA
		// cursor, which has no portable forcing method (unlike the
		// ALTER failure below).
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

// HasLegacyRecoveryCodeWraps reports whether any active recovery code
// lacks a wrapped DEK. Such codes must be reissued before encrypting
// project databases: they cannot unlock the DEK, so once encrypted data
// exists a password reset with one is refused (ErrLegacyRecoveryCode).
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
