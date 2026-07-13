// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// requireForeignKeysOnTwoConns holds two simultaneous connections (forcing
// the pool to open a second physical connection) and asserts foreign_keys
// is ON for both.
func requireForeignKeysOnTwoConns(t *testing.T, pool *sql.DB) {
	t.Helper()
	ctx := context.Background()

	c1, err := pool.Conn(ctx)
	if err != nil {
		t.Fatalf("first conn: %v", err)
	}
	defer func() { _ = c1.Close() }()
	c2, err := pool.Conn(ctx) // second physical connection while c1 is held
	if err != nil {
		t.Fatalf("second conn: %v", err)
	}
	defer func() { _ = c2.Close() }()

	var fk1, fk2 int
	if err := c1.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk1); err != nil {
		t.Fatalf("pragma on conn1: %v", err)
	}
	if err := c2.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk2); err != nil {
		t.Fatalf("pragma on conn2: %v", err)
	}
	if fk1 != 1 || fk2 != 1 {
		t.Fatalf("foreign_keys not enforced on all pooled connections: conn1=%d conn2=%d", fk1, fk2)
	}
}

// TestForeignKeysOnEveryPooledConnection locks in the fix for the audit's
// C-1 finding: foreign_keys (a per-connection pragma) must be ON for every
// connection the *sql.DB pool opens, not just the one that served a one-off
// Exec at init.
func TestForeignKeysOnEveryPooledConnection(t *testing.T) {
	d, err := InitDB(filepath.Join(t.TempDir(), "pragmas.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = d.Close() }()
	requireForeignKeysOnTwoConns(t, d.Conn)
}

// TestEncryptedDSNKeepsConnPragmas confirms the pragma options survive on
// the encrypted DSN path (which already carries _pragma_key options).
func TestEncryptedDSNKeepsConnPragmas(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	d, err := InitEncryptedDB(filepath.Join(t.TempDir(), "enc.pmforge"), dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	defer func() { _ = d.Close() }()
	requireForeignKeysOnTwoConns(t, d.Conn)
}
