// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestOpenCreatesPrivateRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PMForge")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("root mode = %o, want 700", mode)
	}
}

func TestOpenTightensExistingRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PMForge")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("root mode = %o, want 700", mode)
	}
}

func TestOpenCreatesPrivateSystemDatabaseFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PMForge")
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(filepath.Join(root, "system.db"))
	if err != nil {
		t.Fatalf("stat system.db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("system.db mode = %o, want 600", mode)
	}
}

func TestOpenTightensExistingSystemDatabaseFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PMForge")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	dbPath := filepath.Join(root, "system.db")
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		t.Fatalf("write system.db: %v", err)
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat system.db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("system.db mode = %o, want 600", mode)
	}
}

func TestCreateAccountTightensExistingUserDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "PMForge")
	for _, sub := range []string{
		"alice",
		filepath.Join("alice", "projects"),
		filepath.Join("alice", "certs"),
		filepath.Join("alice", "exports"),
	} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	for _, sub := range []string{"alice", filepath.Join("alice", "projects"), filepath.Join("alice", "certs"), filepath.Join("alice", "exports")} {
		info, err := os.Stat(filepath.Join(root, sub))
		if err != nil {
			t.Fatalf("stat %s: %v", sub, err)
		}
		if mode := info.Mode().Perm(); mode != 0o700 {
			t.Fatalf("%s mode = %o, want 700", sub, mode)
		}
	}
}

func TestAuthenticateReturnsLastLoginUpdateError(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "PMForge"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_last_login
		BEFORE UPDATE OF last_login ON users
		BEGIN
			SELECT RAISE(ABORT, 'last login blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err = store.Authenticate("alice", "correct horse battery staple")
	if err == nil || !strings.Contains(err.Error(), "update last_login") {
		t.Fatalf("Authenticate error = %v, want update last_login error", err)
	}
}

func TestAuthenticateReturnsPasswordRehashUpdateError(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "PMForge"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	const password = "correct horse battery staple"
	if _, err := store.CreateAccount("alice", "Alice", password); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := store.conn.Exec(
		`UPDATE users SET password_hash = ? WHERE username = ?`,
		weakPasswordHash(password), "alice",
	); err != nil {
		t.Fatalf("seed weak hash: %v", err)
	}
	if _, err := store.conn.Exec(`
		CREATE TRIGGER block_password_rehash
		BEFORE UPDATE OF password_hash ON users
		BEGIN
			SELECT RAISE(ABORT, 'password rehash blocked');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err = store.Authenticate("alice", password)
	if err == nil || !strings.Contains(err.Error(), "persist password rehash") {
		t.Fatalf("Authenticate error = %v, want persist password rehash error", err)
	}
}

// TestCreateAccount_RejectsCaseVariantUsername is a regression test for the
// APFS case-insensitive filesystem collision: "alice" and "Alice" resolve to
// the same directory on macOS, leaking project names across accounts.
// The fix uses lower(username) = lower(?) in the duplicate check.
func TestCreateAccount_RejectsCaseVariantUsername(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "PMForge"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	if _, err := store.CreateAccount("alice", "Alice", "correct horse battery staple"); err != nil {
		t.Fatalf("CreateAccount (original): %v", err)
	}
	for _, variant := range []string{"Alice", "ALICE", "aLiCe"} {
		_, got := store.CreateAccount(variant, "Alice", "another-password")
		if got != ErrUserExists {
			t.Errorf("CreateAccount(%q) error = %v, want ErrUserExists", variant, got)
		}
	}
}

func weakPasswordHash(password string) string {
	const (
		memory  = 8 * 1024
		time    = 1
		threads = 1
		keyLen  = 32
	)
	salt := []byte("weak-test-salt!!")
	key := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory, time, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}
