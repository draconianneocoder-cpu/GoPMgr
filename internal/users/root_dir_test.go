// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The data-root directory name is a persistence boundary, not branding: it
// is where every existing install already has its accounts, projects, and
// settings. Renamed 2026-08-04 from "PMForge" to "GoPMgr" as part of the
// PMForge -> GoPMgr rebrand; DefaultRootDir now resolves to "GoPMgr" and
// MigrateLegacyRoot (migrate_root_test.go) copies any existing "PMForge"
// install forward so the rename doesn't orphan it. These tests pin the
// *current* literal so a future rebrand or find/replace pass fails loudly
// here instead of silently orphaning installed data, exactly as the
// "PMForge" version of these tests did before this rename.

func TestDefaultRootDirXDGOverride_UsesGoPMgrLeaf(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	root, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if got := filepath.Base(root); got != "GoPMgr" {
		t.Fatalf("DefaultRootDir leaf directory = %q, want %q (renaming this orphans every existing install)", got, "GoPMgr")
	}
}

func TestDefaultRootDirPlatformDefault_UsesGoPMgrLeaf(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "") // exercise the per-platform branch, not the override

	root, err := DefaultRootDir()
	if err != nil {
		t.Fatalf("DefaultRootDir: %v", err)
	}
	if got := filepath.Base(root); got != "GoPMgr" {
		t.Fatalf("DefaultRootDir leaf directory = %q, want %q (renaming this orphans every existing install)", got, "GoPMgr")
	}

	// The leaf name alone isn't enough: swapping macOS's parent from
	// "Library/Application Support" to "Documents" (or back), or swapping
	// Linux/Windows's "Documents" for something else, would keep the leaf
	// identical while pointing at a location no existing install has ever
	// written to. Pin the full resolved path on both branches so CI (which
	// runs on Linux) actually exercises the non-darwin case instead of only
	// the leaf check above.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	var want string
	if runtime.GOOS == "darwin" {
		want = filepath.Join(home, "Library", "Application Support", "GoPMgr")
	} else {
		want = filepath.Join(home, "Documents", "GoPMgr")
	}
	if root != want {
		t.Fatalf("DefaultRootDir on %s = %q, want %q (existing installs are at this exact path)", runtime.GOOS, root, want)
	}
}

func TestLegacyRootCandidates_PrefersApplicationSupportOverDocuments(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	candidates := legacyRootCandidates()
	if runtime.GOOS != "darwin" {
		if len(candidates) != 1 {
			t.Fatalf("legacyRootCandidates on %s = %v, want exactly one candidate (Documents/PMForge)", runtime.GOOS, candidates)
		}
		if got := filepath.Base(candidates[0]); got != "PMForge" {
			t.Fatalf("legacyRootCandidates[0] leaf = %q, want %q (MigrateLegacyRoot reads real ~/Documents/PMForge installs; renaming this stops migration from finding them)", got, "PMForge")
		}
		return
	}

	// macOS has two legacy sources because the app's data root moved once
	// already (2026-06, TCC/iCloud fix) before this rename: the most recent
	// pre-rename default (Application Support/PMForge) must be checked
	// before the original pre-relocation location (Documents/PMForge), or a
	// user who has data in both would be migrated from the stale one.
	if len(candidates) != 2 {
		t.Fatalf("legacyRootCandidates on darwin = %v, want exactly two candidates", candidates)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	wantFirst := filepath.Join(home, "Library", "Application Support", "PMForge")
	wantSecond := filepath.Join(home, "Documents", "PMForge")
	if candidates[0] != wantFirst {
		t.Fatalf("legacyRootCandidates[0] = %q, want %q (must be checked before the older Documents location)", candidates[0], wantFirst)
	}
	if candidates[1] != wantSecond {
		t.Fatalf("legacyRootCandidates[1] = %q, want %q", candidates[1], wantSecond)
	}
}

// TestLegacyRootCandidates_UnderXDGOverride_ReturnsPMForgeCandidate replaces
// an earlier version of this test that asserted candidates == nil under an
// XDG_DATA_HOME override, on the reasoning that "only one location has ever
// existed" there. That was true before this rename — DefaultRootDir under
// an override resolved to $XDG_DATA_HOME/PMForge both before and after, so
// there was nothing to migrate from. The rename changed DefaultRootDir's
// override behavior to $XDG_DATA_HOME/GoPMgr without this function being
// revisited, so a real $XDG_DATA_HOME/PMForge install (a routine Linux
// desktop-environment setup, not just a test knob) would silently get a
// fresh empty root instead of being migrated. Inverted here the same way
// TestProjectPathForRejectsWrongExtension was inverted elsewhere in this
// rename, with this comment explaining why.
func TestLegacyRootCandidates_UnderXDGOverride_ReturnsPMForgeCandidate(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	candidates := legacyRootCandidates()
	want := []string{filepath.Join(xdg, "PMForge")}
	if len(candidates) != 1 || candidates[0] != want[0] {
		t.Fatalf("legacyRootCandidates under XDG_DATA_HOME override = %v, want %v (an existing $XDG_DATA_HOME/PMForge install must still be found)", candidates, want)
	}
}
