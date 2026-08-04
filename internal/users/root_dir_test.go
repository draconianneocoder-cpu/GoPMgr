// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The data-root directory name "PMForge" is a persistence boundary, not
// branding: it is where every existing install already has its accounts,
// projects, and settings, and DefaultRootDir must keep resolving to it after
// the PMForge -> GoPMgr rename or every existing user's data goes missing on
// upgrade. These tests pin that literal so a future rebrand or find/replace
// pass fails loudly here instead of silently orphaning installed data.

func TestDefaultRootDirXDGOverride_UsesPMForgeLeaf(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	root, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if got := filepath.Base(root); got != "PMForge" {
		t.Fatalf("DefaultRootDir leaf directory = %q, want %q (renaming this orphans every existing install)", got, "PMForge")
	}
}

func TestDefaultRootDirPlatformDefault_UsesPMForgeLeaf(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "") // exercise the per-platform branch, not the override

	root, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if got := filepath.Base(root); got != "PMForge" {
		t.Fatalf("DefaultRootDir leaf directory = %q, want %q (renaming this orphans every existing install)", got, "PMForge")
	}

	// The leaf name alone isn't enough: swapping macOS's parent from
	// "Library/Application Support" to "Documents" (or back), or swapping
	// Linux/Windows's "Documents" for something else, would keep the leaf
	// identical while pointing at a location no existing install has ever
	// written to, AND silently defeat MigrateLegacyRoot (it treats
	// legacy == newRoot as "nothing to migrate" and no-ops). Pin the full
	// resolved path on both branches so CI (which runs on Linux) actually
	// exercises the non-darwin case instead of only the leaf check above.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	var want string
	if runtime.GOOS == "darwin" {
		want = filepath.Join(home, "Library", "Application Support", "PMForge")
	} else {
		want = filepath.Join(home, "Documents", "PMForge")
	}
	if root != want {
		t.Fatalf("DefaultRootDir on %s = %q, want %q (existing installs are at this exact path)", runtime.GOOS, root, want)
	}
}

func TestLegacyMacRootDir_UsesPMForgeLeaf(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	legacy := legacyMacRootDir()
	if runtime.GOOS != "darwin" {
		if legacy != "" {
			t.Fatalf("legacyMacRootDir on %s = %q, want empty (nothing to migrate off this platform)", runtime.GOOS, legacy)
		}
		return
	}
	if got := filepath.Base(legacy); got != "PMForge" {
		t.Fatalf("legacyMacRootDir leaf directory = %q, want %q (MigrateLegacyRoot reads real ~/Documents/PMForge installs; renaming this stops migration from finding them)", got, "PMForge")
	}
}
