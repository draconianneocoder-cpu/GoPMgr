// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"path/filepath"
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
