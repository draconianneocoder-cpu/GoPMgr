// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"path/filepath"
	"testing"
)

var escrowTables = []string{"escrow_keys", "personal_keys", "sealed_deks", "escrow_pins", "escrow_grants"}

func countRows(t *testing.T, store *Store, table, where string, args ...any) int {
	t.Helper()
	var n int
	if err := store.conn.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func insertEscrowKey(t *testing.T, store *Store, id, retiredAt string) error {
	t.Helper()
	_, err := store.conn.Exec(
		`INSERT INTO escrow_keys (id, kem_id, public_key, created_at, retired_at) VALUES (?, 32, x'01', '2026-10-02T00:00:00Z', ?)`,
		id, retiredAt,
	)
	return err
}

// Opening an existing system.db adds any missing escrow table and leaves
// existing ones alone, however many times it runs.
func TestEscrowTablesMigrateIdempotently(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoPMgr")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := insertEscrowKey(t, store, "key-1", ""); err != nil {
		t.Fatalf("insert escrow key: %v", err)
	}
	if _, err := store.CreateAccount("alice", "Alice", "passphrase-long", true); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.conn.Exec(`INSERT INTO personal_keys (username, public_key, wrapped_private_key, created_at) VALUES ('alice', x'01', x'02', '2026-10-02T00:00:00Z')`); err != nil {
		t.Fatalf("seed alice's personal key: %v", err)
	}
	if _, err := store.conn.Exec(`DROP TABLE escrow_grants`); err != nil {
		t.Fatalf("drop escrow_grants: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for i := range 2 {
		store, err = Open(root)
		if err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
		for _, table := range escrowTables {
			if n := countRows(t, store, "sqlite_master", "type = 'table' AND name = ?", table); n != 1 {
				t.Fatalf("reopen %d: table %s exists %d times, want 1", i, table, n)
			}
		}
		if n := countRows(t, store, "escrow_keys", "id = 'key-1'"); n != 1 {
			t.Fatalf("reopen %d: the existing escrow key is gone", i)
		}
		if n := countRows(t, store, "personal_keys", "username = 'alice'"); n != 1 {
			t.Fatalf("reopen %d: alice's personal key is gone", i)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
}

// At most one escrow key is active; any number may be retired.
func TestOnlyOneEscrowKeyIsActive(t *testing.T) {
	store := openTestStore(t)
	if err := insertEscrowKey(t, store, "key-1", ""); err != nil {
		t.Fatalf("first active key: %v", err)
	}
	if err := insertEscrowKey(t, store, "key-2", ""); err == nil {
		t.Fatal("a second active escrow key was stored")
	}
	if _, err := store.conn.Exec(`UPDATE escrow_keys SET retired_at = '2026-10-02T01:00:00Z' WHERE id = 'key-1'`); err != nil {
		t.Fatalf("retire key-1: %v", err)
	}
	if err := insertEscrowKey(t, store, "key-2", ""); err != nil {
		t.Fatalf("active key after retiring the first: %v", err)
	}
	if err := insertEscrowKey(t, store, "key-3", "2026-10-02T02:00:00Z"); err != nil {
		t.Fatalf("a retired key beside the active one: %v", err)
	}
}

// Deleting an account removes its personal key, sealed DEK, pin, and grant,
// and leaves other accounts' rows.
func TestDeletingAnAccountRemovesItsEscrowRows(t *testing.T) {
	store := newStatusStore(t) // alice (administrator) and bob
	if err := insertEscrowKey(t, store, "key-1", ""); err != nil {
		t.Fatalf("insert escrow key: %v", err)
	}
	for _, user := range []string{"alice", "bob"} {
		for _, stmt := range []string{
			`INSERT INTO personal_keys (username, public_key, wrapped_private_key, created_at) VALUES (?, x'01', x'02', '2026-10-02T00:00:00Z')`,
			`INSERT INTO sealed_deks (username, escrow_key_id, sealed, sealed_at) VALUES (?, 'key-1', x'03', '2026-10-02T00:00:00Z')`,
			`INSERT INTO escrow_pins (username, escrow_key_id, pin) VALUES (?, 'key-1', x'04')`,
			`INSERT INTO escrow_grants (admin_username, escrow_key_id, sealed, granted_at) VALUES (?, 'key-1', x'05', '2026-10-02T00:00:00Z')`,
		} {
			if _, err := store.conn.Exec(stmt, user); err != nil {
				t.Fatalf("seed %s: %v", user, err)
			}
		}
	}

	if err := store.PurgeAccount("alice", "bob"); err != nil {
		t.Fatalf("PurgeAccount(bob): %v", err)
	}
	for _, table := range []string{"personal_keys", "sealed_deks", "escrow_pins"} {
		if n := countRows(t, store, table, "username = 'bob'"); n != 0 {
			t.Errorf("%s keeps %d row(s) for the deleted account", table, n)
		}
		if n := countRows(t, store, table, "username = 'alice'"); n != 1 {
			t.Errorf("%s lost alice's row", table)
		}
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'bob'"); n != 0 {
		t.Errorf("escrow_grants keeps the deleted account's grant")
	}
	if n := countRows(t, store, "escrow_grants", "admin_username = 'alice'"); n != 1 {
		t.Errorf("escrow_grants lost alice's grant")
	}
}

// A sealed row cannot name an escrow key that does not exist.
func TestEscrowRowsMustNameAnExistingEscrowKey(t *testing.T) {
	store := newStatusStore(t)
	if _, err := store.conn.Exec(
		`INSERT INTO sealed_deks (username, escrow_key_id, sealed, sealed_at) VALUES ('alice', 'missing', x'01', '2026-10-02T00:00:00Z')`,
	); err == nil {
		t.Fatal("a sealed DEK naming a missing escrow key was stored")
	}
}

func TestEscrowAccountEventsAreAccepted(t *testing.T) {
	store := newStatusStore(t)
	for _, action := range []string{
		AccountAdminAccess, AccountEscrowKeyMismatch, AccountEscrowReenrolled,
		AccountPersonalKeyRepaired, AccountEscrowRotated,
	} {
		if err := recordAccountEvent(context.Background(), store.conn, "alice", "bob", action, ""); err != nil {
			t.Errorf("record %s: %v", action, err)
		}
	}
}
