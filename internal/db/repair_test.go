// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwapInSnapshotRejectsStaleCorruptDirectoryBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
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

	if _, err := d.SwapInSnapshot(livePath); err == nil || !strings.Contains(err.Error(), "clear stale corrupt") {
		t.Fatalf("SwapInSnapshot error = %v, want clear stale corrupt error", err)
	}

	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("live handle should remain usable after preflight failure: %v", err)
	}
	if !ok {
		t.Fatal("live database failed integrity check after preflight failure")
	}
}

func TestSwapInSnapshotRejectsDirectorySnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := os.Mkdir(livePath+".bak", 0o700); err != nil {
		t.Fatalf("make snapshot directory: %v", err)
	}

	if _, err := d.SwapInSnapshot(livePath); err == nil || !strings.Contains(err.Error(), "snapshot is not a regular file") {
		t.Fatalf("SwapInSnapshot error = %v, want non-regular snapshot error", err)
	}

	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("live handle should remain usable after snapshot preflight failure: %v", err)
	}
	if !ok {
		t.Fatal("live database failed integrity check after snapshot preflight failure")
	}
}

func TestSwapInSnapshotRejectsInvalidSnapshotBeforeClosingLive(t *testing.T) {
	livePath := filepath.Join(t.TempDir(), "project.pmforge")
	d, err := InitDB(livePath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := os.WriteFile(livePath+".bak", []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("write invalid snapshot: %v", err)
	}

	if _, err := d.SwapInSnapshot(livePath); err == nil || !strings.Contains(err.Error(), "snapshot integrity") {
		t.Fatalf("SwapInSnapshot error = %v, want snapshot integrity error", err)
	}

	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("live handle should remain usable after invalid snapshot preflight: %v", err)
	}
	if !ok {
		t.Fatal("live database failed integrity check after invalid snapshot preflight")
	}
}
