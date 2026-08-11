// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package db

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestSwapInSnapshot_RenameLiveToCorruptFailureIsReported is the plain
// counterpart to TestSwapInEncryptedSnapshot_RenameLiveToCorruptFailureIsReported
// (internal/db/repair_encryption_darwin_test.go): step 2 (moving the live
// file aside to .corrupt) fails AFTER db.Close() has already succeeded, so
// the original *Database handle d is no longer usable -- the assertion
// must confirm the live FILE survived untouched and is reopenable, not
// that the closed handle still works. Same chflags(UF_IMMUTABLE) technique
// as the rollback test below.
//
// This file is darwin-only via a build constraint, not a runtime
// t.Skip(runtime.GOOS != "darwin"): syscall.Chflags does not exist in the
// syscall package on linux at all, so referencing it unconditionally
// broke compilation (not just execution) on this repo's ubuntu-24.04 CI
// the moment these tests were added -- a runtime skip guard doesn't
// prevent a compile error. Discovered when CI actually ran against a
// commit including this file for the first time; confirmed via
// `git merge-base --is-ancestor` that the break predated that commit,
// unrelated to whatever change happened to be the first to trigger CI.
func TestSwapInSnapshot_RenameLiveToCorruptFailureIsReported(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "OriginalLiveState", Status: "active", Phase: "execution"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	const ufImmutable = 0x2
	if err := syscall.Chflags(livePath, ufImmutable); err != nil {
		t.Fatalf("chflags(UF_IMMUTABLE) on live file: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Chflags(livePath, 0) })

	_, err = d.SwapInSnapshot(livePath)
	if err == nil {
		t.Fatal("SwapInSnapshot succeeded despite an immutable live file blocking the rename to .corrupt")
	}
	// Assert the specific branch, not just any error -- db.Close() could
	// plausibly fail first and land in the "swap: close live" branch
	// instead of the rename branch this test targets.
	if !strings.Contains(err.Error(), "rename live → corrupt") {
		t.Fatalf("SwapInSnapshot error = %v, want a rename-live-to-corrupt error", err)
	}

	if err := syscall.Chflags(livePath, 0); err != nil {
		t.Fatalf("clear chflags before reopen: %v", err)
	}
	restored, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("live path unusable after failed rename-to-corrupt: %v", err)
	}
	defer restored.Close()
	p, err := restored.GetProject()
	if err != nil {
		t.Fatalf("GetProject on surviving live file: %v", err)
	}
	if p.Name != "OriginalLiveState" {
		t.Errorf("surviving live project = %q, want %q", p.Name, "OriginalLiveState")
	}
	if _, err := os.Stat(livePath + ".corrupt"); !os.IsNotExist(err) {
		t.Errorf(".corrupt was created despite the rename failing: stat err = %v", err)
	}
}

// TestSwapInSnapshot_RollsBackLiveFileWhenFinalRenameFails is the
// highest-consequence untested branch in this package: if the final
// rename (snapshot -> live) fails AFTER the live file was already
// moved aside to .corrupt, the code must roll the .corrupt file back
// to the live path so the user is never left with zero database
// files -- worse than the corruption they started with. This
// specific interleaving (fail the second of two same-directory
// renames without touching the first) has no portable POSIX
// permission-based trigger: both renames share one parent directory's
// write permission, so blocking the directory blocks step 1 too, and
// no hook exists to intercept between the two calls. Confirmed
// forceable only via macOS's non-POSIX chflags(UF_IMMUTABLE), which
// blocks renaming a specific file regardless of directory
// permissions; syscall.Chflags does not exist on Linux, where this
// repository's CI runs (ubuntu-24.04) -- hence this whole file's
// //go:build darwin constraint, not a per-test runtime skip.
func TestSwapInSnapshot_RollsBackLiveFileWhenFinalRenameFails(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "OriginalLiveState", Status: "active", Phase: "execution"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	const ufImmutable = 0x2
	if err := syscall.Chflags(livePath+".bak", ufImmutable); err != nil {
		t.Fatalf("chflags(UF_IMMUTABLE) on snapshot: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Chflags(livePath+".bak", 0) })

	_, err = d.SwapInSnapshot(livePath)
	if err == nil {
		t.Fatal("SwapInSnapshot succeeded despite an immutable snapshot file blocking the final rename")
	}
	if strings.Contains(err.Error(), "rollback live:") {
		t.Fatalf("rollback itself failed: %v", err)
	}

	// The rollback must have restored the ORIGINAL live file, not left
	// the user with no database at all.
	restored, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("live path unusable after rollback: %v", err)
	}
	defer restored.Close()
	p, err := restored.GetProject()
	if err != nil {
		t.Fatalf("GetProject on rolled-back live file: %v", err)
	}
	if p.Name != "OriginalLiveState" {
		t.Errorf("rolled-back live project = %q, want %q", p.Name, "OriginalLiveState")
	}
	if _, err := os.Stat(livePath + ".corrupt"); !os.IsNotExist(err) {
		t.Errorf(".corrupt still exists after a successful rollback (should have been renamed back to live): stat err = %v", err)
	}
}
