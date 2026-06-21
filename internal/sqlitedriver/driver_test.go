// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"database/sql"
	"testing"
)

func TestDriverProvidesSQLCipher(t *testing.T) {
	conn, err := sql.Open(Name, ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer conn.Close()

	var version string
	if err := conn.QueryRow("PRAGMA cipher_version").Scan(&version); err != nil {
		t.Fatalf("PRAGMA cipher_version: %v", err)
	}
	if version == "" {
		t.Fatal("PRAGMA cipher_version returned empty version")
	}
}
