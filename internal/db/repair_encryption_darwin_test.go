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

// TestSwapInEncryptedSnapshot_RenameLiveToCorruptFailureIsReported covers
// the post-close branch where step 2 (moving the live file aside to
// .corrupt) fails: unlike every preflight-rejection test in
// repair_encryption_test.go, this failure happens AFTER db.Close() has
// already succeeded, so the original *Database handle d is no longer
// usable -- the assertion must instead confirm the live FILE survived
// untouched and is reopenable, not that the closed handle still works.
// Forced via macOS's chflags(UF_IMMUTABLE) on livePath itself, the same
// non-portable technique repair_selfheal_darwin_test.go uses for the
// final-rename rollback case.
//
// This file is darwin-only via a build constraint, not a runtime
// t.Skip(runtime.GOOS != "darwin"): syscall.Chflags does not exist in the
// syscall package on linux at all, so referencing it unconditionally
// broke compilation (not just execution) on this repo's ubuntu-24.04 CI
// the moment these tests were added -- a runtime skip guard doesn't
// prevent a compile error.
func TestSwapInEncryptedSnapshot_RenameLiveToCorruptFailureIsReported(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x4d)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
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

	_, err = d.SwapInEncryptedSnapshot(livePath, dek)
	if err == nil {
		t.Fatal("SwapInEncryptedSnapshot succeeded despite an immutable live file blocking the rename to .corrupt")
	}
	// Assert the specific branch, not just any error -- db.Close() itself
	// touching WAL/SHM sidecars could plausibly fail first and land in
	// the "swap: close live" branch instead of the rename branch this
	// test targets.
	if !strings.Contains(err.Error(), "rename live → corrupt") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want a rename-live-to-corrupt error", err)
	}

	if err := syscall.Chflags(livePath, 0); err != nil {
		t.Fatalf("clear chflags before reopen: %v", err)
	}
	restored, err := InitEncryptedDB(livePath, dek)
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

// TestSwapInEncryptedSnapshot_RollsBackLiveFileWhenFinalRenameFails is the
// encrypted counterpart to TestSwapInSnapshot_RollsBackLiveFileWhenFinalRenameFails:
// if the final rename (snapshot -> live) fails AFTER the live file was
// already moved aside to .corrupt, the code must roll the .corrupt file
// back so the user is never left with zero database files. Same
// chflags(UF_IMMUTABLE)-on-the-snapshot technique, same darwin-only scope.
func TestSwapInEncryptedSnapshot_RollsBackLiveFileWhenFinalRenameFails(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x4e)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
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

	_, err = d.SwapInEncryptedSnapshot(livePath, dek)
	if err == nil {
		t.Fatal("SwapInEncryptedSnapshot succeeded despite an immutable snapshot file blocking the final rename")
	}
	if strings.Contains(err.Error(), "rollback live:") {
		t.Fatalf("rollback itself failed: %v", err)
	}

	restored, err := InitEncryptedDB(livePath, dek)
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
