// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"os"
	"path/filepath"
	"testing"
)

// seedLegacyRoot writes a minimal legacy data tree: a system.db plus a nested
// per-user file, so a migration has something recognisable to copy.
func seedLegacyRoot(t *testing.T, legacy string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(legacy, "alice", "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "system.db"), []byte("SYSTEM"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "alice", "projects", "p.pmforge"), []byte("PROJECT"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyRootCopiesTree(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), "Documents", "PMForge")
	newRoot := filepath.Join(t.TempDir(), "Application Support", "PMForge")
	seedLegacyRoot(t, legacy)

	migrated, err := migrateLegacyRoot(legacy, newRoot)
	if err != nil {
		t.Fatalf("migrateLegacyRoot: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to run")
	}

	// system.db and the nested project file must land in the new root.
	if got, err := os.ReadFile(filepath.Join(newRoot, "system.db")); err != nil || string(got) != "SYSTEM" {
		t.Fatalf("system.db not migrated: got %q err %v", got, err)
	}
	proj := filepath.Join(newRoot, "alice", "projects", "p.pmforge")
	if got, err := os.ReadFile(proj); err != nil || string(got) != "PROJECT" {
		t.Fatalf("nested project not migrated: got %q err %v", got, err)
	}
	// Owner-only permissions must be preserved on the copied database.
	info, err := os.Stat(filepath.Join(newRoot, "system.db"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("migrated system.db perm = %o, want 600", perm)
	}
	// The original must be left intact (copy, not move).
	if _, err := os.Stat(filepath.Join(legacy, "system.db")); err != nil {
		t.Fatalf("legacy system.db should remain: %v", err)
	}
}

func TestMigrateLegacyRootIdempotent(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), "Documents", "PMForge")
	newRoot := filepath.Join(t.TempDir(), "Application Support", "PMForge")
	seedLegacyRoot(t, legacy)

	if migrated, err := migrateLegacyRoot(legacy, newRoot); err != nil || !migrated {
		t.Fatalf("first migration: migrated=%v err=%v", migrated, err)
	}
	// A second call is a no-op because the new root now has a system.db.
	if migrated, err := migrateLegacyRoot(legacy, newRoot); err != nil || migrated {
		t.Fatalf("second migration should be a no-op: migrated=%v err=%v", migrated, err)
	}
}

func TestMigrateLegacyRootSkips(t *testing.T) {
	t.Run("no legacy install", func(t *testing.T) {
		legacy := filepath.Join(t.TempDir(), "Documents", "PMForge") // never created
		newRoot := filepath.Join(t.TempDir(), "Application Support", "PMForge")
		if migrated, err := migrateLegacyRoot(legacy, newRoot); err != nil || migrated {
			t.Fatalf("expected skip: migrated=%v err=%v", migrated, err)
		}
		if _, err := os.Stat(newRoot); !os.IsNotExist(err) {
			t.Fatalf("new root should not be created when there is nothing to migrate: %v", err)
		}
	})

	t.Run("new root already initialised", func(t *testing.T) {
		legacy := filepath.Join(t.TempDir(), "Documents", "PMForge")
		newRoot := filepath.Join(t.TempDir(), "Application Support", "PMForge")
		seedLegacyRoot(t, legacy)
		if err := os.MkdirAll(newRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(newRoot, "system.db"), []byte("EXISTING"), 0o600); err != nil {
			t.Fatal(err)
		}
		if migrated, err := migrateLegacyRoot(legacy, newRoot); err != nil || migrated {
			t.Fatalf("expected skip when new root has a system.db: migrated=%v err=%v", migrated, err)
		}
		// The existing new-root database must not be overwritten by the legacy one.
		if got, err := os.ReadFile(filepath.Join(newRoot, "system.db")); err != nil || string(got) != "EXISTING" {
			t.Fatalf("existing system.db was clobbered: got %q err %v", got, err)
		}
	})

	t.Run("empty legacy path", func(t *testing.T) {
		newRoot := filepath.Join(t.TempDir(), "Application Support", "PMForge")
		if migrated, err := migrateLegacyRoot("", newRoot); err != nil || migrated {
			t.Fatalf("empty legacy path should skip: migrated=%v err=%v", migrated, err)
		}
	})
}
