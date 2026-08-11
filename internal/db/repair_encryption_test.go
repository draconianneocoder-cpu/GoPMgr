// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"os"
	"path/filepath"
	"strings"
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
