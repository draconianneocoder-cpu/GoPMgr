// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"os"
	"path/filepath"
	"runtime"
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
	legacy := filepath.Join(t.TempDir(), "Documents", "GoPMgr")
	newRoot := filepath.Join(t.TempDir(), "Application Support", "GoPMgr")
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
	legacy := filepath.Join(t.TempDir(), "Documents", "GoPMgr")
	newRoot := filepath.Join(t.TempDir(), "Application Support", "GoPMgr")
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
		legacy := filepath.Join(t.TempDir(), "Documents", "GoPMgr") // never created
		newRoot := filepath.Join(t.TempDir(), "Application Support", "GoPMgr")
		if migrated, err := migrateLegacyRoot(legacy, newRoot); err != nil || migrated {
			t.Fatalf("expected skip: migrated=%v err=%v", migrated, err)
		}
		if _, err := os.Stat(newRoot); !os.IsNotExist(err) {
			t.Fatalf("new root should not be created when there is nothing to migrate: %v", err)
		}
	})

	t.Run("new root already initialised", func(t *testing.T) {
		legacy := filepath.Join(t.TempDir(), "Documents", "GoPMgr")
		newRoot := filepath.Join(t.TempDir(), "Application Support", "GoPMgr")
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
		newRoot := filepath.Join(t.TempDir(), "Application Support", "GoPMgr")
		if migrated, err := migrateLegacyRoot("", newRoot); err != nil || migrated {
			t.Fatalf("empty legacy path should skip: migrated=%v err=%v", migrated, err)
		}
	})
}

// The tests above exercise migrateLegacyRoot directly with hand-picked
// paths; they never call the public MigrateLegacyRoot / DefaultRootDir /
// legacyRootCandidates path a real 2026-08-04-rename upgrade actually goes
// through, so a bug in how those wire together (wrong candidate order,
// wrong newRoot) would slip past them. These two lay out a real "PMForge"
// install under a fake $HOME the way an actual pre-rename user's disk would
// look, then call the public entry points exactly as main.go's NewApp does.

// TestMigrateLegacyRoot_FindsCurrentPMForgeInstall covers the common
// upgrade case: a user who was already on the most recent pre-rename
// default location (Application Support/PMForge on macOS, Documents/
// PMForge elsewhere — legacyRootCandidates()[0] on every platform).
func TestMigrateLegacyRoot_FindsCurrentPMForgeInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	var legacy string
	if runtime.GOOS == "darwin" {
		legacy = filepath.Join(home, "Library", "Application Support", "PMForge")
	} else {
		legacy = filepath.Join(home, "Documents", "PMForge")
	}
	seedLegacyRoot(t, legacy)

	newRoot, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	migrated, err := MigrateLegacyRoot(newRoot)
	if err != nil {
		t.Fatalf("MigrateLegacyRoot: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration from the current pre-rename PMForge install")
	}
	if got, err := os.ReadFile(filepath.Join(newRoot, "system.db")); err != nil || string(got) != "SYSTEM" {
		t.Fatalf("system.db not migrated into %s: got %q err %v", newRoot, got, err)
	}
}

// TestMigrateLegacyRoot_FallsBackToPreRelocationInstall covers a macOS user
// who upgrades straight from a very old install that predates the 2026-06
// Application Support relocation and never had a Application Support/
// PMForge directory at all — only the original ~/Documents/PMForge.
func TestMigrateLegacyRoot_FallsBackToPreRelocationInstall(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS has a second, older legacy candidate to fall back to")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	legacy := filepath.Join(home, "Documents", "PMForge")
	seedLegacyRoot(t, legacy)
	// Deliberately do NOT create Application Support/PMForge, so the first
	// candidate is absent and MigrateLegacyRoot must fall through to it.

	newRoot, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	migrated, err := MigrateLegacyRoot(newRoot)
	if err != nil {
		t.Fatalf("MigrateLegacyRoot: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to fall back to the pre-relocation Documents/PMForge install")
	}
	if got, err := os.ReadFile(filepath.Join(newRoot, "system.db")); err != nil || string(got) != "SYSTEM" {
		t.Fatalf("system.db not migrated into %s: got %q err %v", newRoot, got, err)
	}
}

// TestMigrateLegacyRoot_FindsXDGInstall covers a Linux user with
// $XDG_DATA_HOME set — a routine desktop-environment configuration, not
// just a test knob (see the comment on DefaultRootDir). Before this rename,
// DefaultRootDir under an XDG override resolved to $XDG_DATA_HOME/PMForge
// both "before" and "after" (there was only ever one XDG-scoped location),
// so legacyRootCandidates correctly returned nil: nothing to migrate from.
// The rename gave XDG installs a real move too — DefaultRootDir now
// resolves to $XDG_DATA_HOME/GoPMgr under the same override — and an
// earlier version of legacyRootCandidates still returned nil here, which
// would have silently orphaned every XDG-configured install's accounts and
// projects on upgrade: MigrateLegacyRoot loops over an empty candidate list
// and reports (false, nil), so NewApp proceeds with a brand-new empty root
// and never surfaces an error. This test calls the exact same public
// entry points (DefaultRootDir, MigrateLegacyRoot) main.go's NewApp does,
// under a real XDG_DATA_HOME override, so a regression of that bug fails
// here loudly instead of shipping silently.
// TestMigrateLegacyRoot_NewRootAlreadyInitialised covers MigrateLegacyRoot's
// own early-return statement (as opposed to migrateLegacyRoot's identical
// check, already covered by TestMigrateLegacyRootSkips's "new root already
// initialised" subtest above). This is a coverage test, not a
// guard-presence one: deleting MigrateLegacyRoot's own check produces the
// exact same (false, nil) result, confirmed by direct mutation, because
// migrateLegacyRoot repeats the identical os.Stat(newRoot/system.db) check
// one call deeper (it needs its own copy regardless, since it's also
// called per-candidate) and catches it there instead — the same
// redundant-downstream-guard shape as internal/admin's
// LogDocumentSignatureOutcome finding. The outer check is a pure
// optimization (skips the legacyRootCandidates() call and one os.Stat per
// candidate), not a second layer of correctness.
func TestMigrateLegacyRoot_NewRootAlreadyInitialised(t *testing.T) {
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "system.db"), []byte("EXISTING"), 0o600); err != nil {
		t.Fatal(err)
	}
	migrated, err := MigrateLegacyRoot(newRoot)
	if err != nil || migrated {
		t.Fatalf("MigrateLegacyRoot with an already-initialised newRoot: migrated=%v err=%v, want (false, nil)", migrated, err)
	}
}

// TestMigrateLegacyRoot_NoLegacyInstallFound covers MigrateLegacyRoot's
// final "no candidate matched" fallback after the loop over
// legacyRootCandidates() exhausts without finding an installable legacy
// root. This is a coverage test only, not break-verifiable: the statement
// itself can't be deleted without a compile error (the function needs some
// final return), so there is no mutation to run against it.
func TestMigrateLegacyRoot_NoLegacyInstallFound(t *testing.T) {
	xdg := t.TempDir() // no "PMForge" subdirectory created under it
	t.Setenv("XDG_DATA_HOME", xdg)
	newRoot := filepath.Join(xdg, "GoPMgr")

	migrated, err := MigrateLegacyRoot(newRoot)
	if err != nil || migrated {
		t.Fatalf("MigrateLegacyRoot with no legacy install anywhere: migrated=%v err=%v, want (false, nil)", migrated, err)
	}
}

// TestMigrateLegacyRoot_PropagatesCopyTreeError forces migrateLegacyRoot's
// copyTree call to fail (a subdirectory the copy needs to create already
// exists as a plain file under newRoot) and confirms MigrateLegacyRoot
// propagates that error out through its own loop rather than swallowing it.
// Uses $XDG_DATA_HOME to make legacyRootCandidates' real candidate-resolution
// path test-controllable, since it re-reads the env var on every call.
func TestMigrateLegacyRoot_PropagatesCopyTreeError(t *testing.T) {
	xdgBase := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgBase)

	legacy := filepath.Join(xdgBase, "PMForge")
	seedLegacyRoot(t, legacy) // writes legacy/system.db and legacy/alice/projects/p.pmforge

	newRoot := filepath.Join(xdgBase, "GoPMgr")
	if err := os.MkdirAll(newRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	// Collide: newRoot/alice exists as a plain file, so copyTree's
	// os.MkdirAll for that directory fails.
	if err := os.WriteFile(filepath.Join(newRoot, "alice"), []byte("collide"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateLegacyRoot(newRoot)
	if err == nil {
		t.Fatal("MigrateLegacyRoot with a blocked copyTree destination = nil error, want an error")
	}
	if migrated {
		t.Fatal("MigrateLegacyRoot reported migrated=true despite a copyTree error")
	}
}

func TestMigrateLegacyRoot_FindsXDGInstall(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	legacy := filepath.Join(xdg, "PMForge")
	seedLegacyRoot(t, legacy)

	newRoot, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if newRoot != filepath.Join(xdg, "GoPMgr") {
		t.Fatalf("DefaultRootDir under XDG_DATA_HOME = %q, want %q", newRoot, filepath.Join(xdg, "GoPMgr"))
	}
	migrated, err := MigrateLegacyRoot(newRoot)
	if err != nil {
		t.Fatalf("MigrateLegacyRoot: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration from the XDG_DATA_HOME/PMForge install")
	}
	if got, err := os.ReadFile(filepath.Join(newRoot, "system.db")); err != nil || string(got) != "SYSTEM" {
		t.Fatalf("system.db not migrated into %s: got %q err %v", newRoot, got, err)
	}
}
