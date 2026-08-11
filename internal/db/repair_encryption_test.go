// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestSwapInEncryptedSnapshotPreservesEncryptionAndReopensWithDEK(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x44)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}

	project, err := d.UpsertProject(Project{
		Name:        "Snapshot State",
		Description: "before live mutation",
		Status:      "active",
		Phase:       "execution",
		Owner:       "alice",
	})
	if err != nil {
		t.Fatalf("UpsertProject initial: %v", err)
	}

	snapshotPath := livePath + ".bak"
	if err := d.CreateSnapshot(snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	requireEncryptedHeader(t, snapshotPath)
	if err := CheckEncryptedSnapshotIntegrity(snapshotPath, dek); err != nil {
		t.Fatalf("CheckEncryptedSnapshotIntegrity: %v", err)
	}
	if err := CheckEncryptedSnapshotIntegrity(snapshotPath, testDEK(t, 0x45)); err == nil {
		t.Fatal("CheckEncryptedSnapshotIntegrity accepted the wrong DEK")
	}

	project.Name = "Mutated Live State"
	if _, err := d.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject mutated: %v", err)
	}

	fresh, err := d.SwapInEncryptedSnapshot(livePath, dek)
	if err != nil {
		t.Fatalf("SwapInEncryptedSnapshot: %v", err)
	}
	defer fresh.Close()

	requireEncryptedHeader(t, livePath)
	got, err := fresh.GetProject()
	if err != nil {
		t.Fatalf("GetProject after swap: %v", err)
	}
	if got.Name != "Snapshot State" {
		t.Fatalf("project name after encrypted swap = %q, want Snapshot State", got.Name)
	}
}

// requireLiveEncryptedHandleUsable asserts that a failed preflight check
// left db's connection open and functional, mirroring the plain-variant
// assertions in repair_test.go.
func requireLiveEncryptedHandleUsable(t *testing.T, d *Database) {
	t.Helper()
	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("live handle should remain usable after preflight failure: %v", err)
	}
	if !ok {
		t.Fatal("live database failed integrity check after preflight failure")
	}
}

func TestSwapInEncryptedSnapshotRejectsStaleCorruptDirectoryBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x46)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	corruptPath := livePath + ".corrupt"
	if err := os.Mkdir(corruptPath, 0o700); err != nil {
		t.Fatalf("make stale corrupt directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corruptPath, "marker"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale corrupt marker: %v", err)
	}

	if _, err := d.SwapInEncryptedSnapshot(livePath, dek); err == nil || !strings.Contains(err.Error(), "clear stale corrupt") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want clear stale corrupt error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

func TestSwapInEncryptedSnapshotRejectsDirectorySnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x47)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := os.Mkdir(livePath+".bak", 0o700); err != nil {
		t.Fatalf("make snapshot directory: %v", err)
	}

	if _, err := d.SwapInEncryptedSnapshot(livePath, dek); err == nil || !strings.Contains(err.Error(), "snapshot is not a regular file") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want non-regular snapshot error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

func TestSwapInEncryptedSnapshotRejectsInvalidSnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x48)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := os.WriteFile(livePath+".bak", []byte("not a sqlite database, encrypted or otherwise"), 0o600); err != nil {
		t.Fatalf("write invalid snapshot: %v", err)
	}

	if _, err := d.SwapInEncryptedSnapshot(livePath, dek); err == nil || !strings.Contains(err.Error(), "encrypted snapshot integrity") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want encrypted snapshot integrity error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

// TestSwapInEncryptedSnapshotRejectsWrongDEKBeforeClosingLive covers the
// preflight branch with no analogue in the plain SwapInSnapshot: a
// structurally valid, correctly-encrypted snapshot that the caller can't
// actually read because they supplied the wrong key. The live connection
// (opened with the correct DEK) must survive the rejection untouched.
func TestSwapInEncryptedSnapshotRejectsWrongDEKBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x49)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := d.CreateSnapshot(livePath + ".bak"); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	wrongDEK := testDEK(t, 0x4a)
	if _, err := d.SwapInEncryptedSnapshot(livePath, wrongDEK); err == nil || !strings.Contains(err.Error(), "encrypted snapshot integrity") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want encrypted snapshot integrity error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

func TestSwapInEncryptedSnapshotRejectsMissingSnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x4b)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	// No .bak file exists.
	if _, err := d.SwapInEncryptedSnapshot(livePath, dek); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want missing-snapshot error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

// TestSwapInEncryptedSnapshotRejectsPlaintextSnapshotBeforeClosingLive covers
// the downgrade-to-plaintext guard inside CheckEncryptedSnapshotIntegrity: a
// structurally valid, uncorrupted SQLite file placed at the snapshot path
// must still be refused if it isn't SQLCipher-encrypted, even though it
// would pass an ordinary (non-encrypted) integrity check.
func TestSwapInEncryptedSnapshotRejectsPlaintextSnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	dek := testDEK(t, 0x4c)
	d, err := InitEncryptedDB(livePath, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	plainPath := filepath.Join(t.TempDir(), "plain.pmforge")
	plainDB, err := InitDB(plainPath)
	if err != nil {
		t.Fatalf("InitDB (plaintext source): %v", err)
	}
	t.Cleanup(func() {
		if err := plainDB.Close(); err != nil {
			t.Fatalf("close plaintext db: %v", err)
		}
	})

	plainSnapshotPath := livePath + ".bak"
	if err := plainDB.CreateSnapshot(plainSnapshotPath); err != nil {
		t.Fatalf("CreateSnapshot (plaintext): %v", err)
	}

	if _, err := d.SwapInEncryptedSnapshot(livePath, dek); err == nil || !strings.Contains(err.Error(), "not encrypted") {
		t.Fatalf("SwapInEncryptedSnapshot error = %v, want not-encrypted error", err)
	}

	requireLiveEncryptedHandleUsable(t, d)
}

// TestSwapInEncryptedSnapshot_RenameLiveToCorruptFailureIsReported covers
// the post-close branch where step 2 (moving the live file aside to
// .corrupt) fails: unlike every preflight-rejection test above, this
// failure happens AFTER db.Close() has already succeeded, so the
// original *Database handle d is no longer usable -- the assertion must
// instead confirm the live FILE survived untouched and is reopenable,
// not that the closed handle still works. Forced via macOS's
// chflags(UF_IMMUTABLE) on livePath itself, the same non-portable
// technique repair_selfheal_test.go already uses for the final-rename
// rollback case; darwin-only, skips elsewhere.
func TestSwapInEncryptedSnapshot_RenameLiveToCorruptFailureIsReported(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("rename-live-to-corrupt failure is only forceable via macOS's chflags(UF_IMMUTABLE); unverified on this GOOS")
	}

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
// chflags(UF_IMMUTABLE)-on-the-snapshot technique, same darwin-only
// scope; the rollback branch itself is unverified on non-darwin CI,
// disclosed rather than silently skipped.
func TestSwapInEncryptedSnapshot_RollsBackLiveFileWhenFinalRenameFails(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("rollback-on-final-rename-failure is only forceable via macOS's chflags(UF_IMMUTABLE); unverified on this GOOS")
	}

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
