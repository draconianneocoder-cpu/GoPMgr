// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"testing"
)

// TestSystemDBForeignKeysOnEveryPooledConnection mirrors the project-DB
// regression for system.db: foreign_keys must be ON for every pooled
// connection, which requires the DSN-parameter form rather than a one-off
// Exec at Open.
func TestSystemDBForeignKeysOnEveryPooledConnection(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	c1, err := s.conn.Conn(ctx)
	if err != nil {
		t.Fatalf("first conn: %v", err)
	}
	defer func() { _ = c1.Close() }()
	c2, err := s.conn.Conn(ctx)
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
		t.Fatalf("system.db foreign_keys not enforced on all pooled connections: conn1=%d conn2=%d", fk1, fk2)
	}
}
