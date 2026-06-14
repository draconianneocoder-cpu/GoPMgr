// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"pmforge/internal/crypto"
)

func testDEK(t *testing.T, fill byte) []byte {
	t.Helper()
	return bytes.Repeat([]byte{fill}, crypto.DEKSize)
}

func requireEncryptedHeader(t *testing.T, path string) {
	t.Helper()
	encrypted, err := IsEncryptedFile(path)
	if err != nil {
		t.Fatalf("IsEncryptedFile: %v", err)
	}
	if !encrypted {
		t.Fatalf("%s has a plaintext SQLite header", path)
	}
}

func requireCipherIntegrity(t *testing.T, d *Database) {
	t.Helper()
	var version string
	if err := d.Conn.QueryRow("PRAGMA cipher_version").Scan(&version); err != nil {
		t.Fatalf("cipher_version: %v", err)
	}
	if version == "" {
		t.Fatal("cipher_version is empty")
	}
	ok, err := d.CheckIntegrity()
	if err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
	if !ok {
		t.Fatal("integrity_check returned non-ok")
	}
	rows, err := d.Conn.Query("PRAGMA cipher_integrity_check")
	if err != nil {
		t.Fatalf("cipher_integrity_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("cipher_integrity_check reported at least one failure")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("cipher_integrity_check rows: %v", err)
	}
}

func TestInitEncryptedDBCreatesEncryptedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "encrypted.pmforge")
	d, err := InitEncryptedDB(path, testDEK(t, 0x42))
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	requireEncryptedHeader(t, path)
	requireCipherIntegrity(t, d)
}

func TestInitEncryptedDBRejectsWrongKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "encrypted.pmforge")
	d, err := InitEncryptedDB(path, testDEK(t, 0x11))
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if _, err := InitEncryptedDB(path, testDEK(t, 0x22)); err == nil {
		t.Fatal("InitEncryptedDB accepted a wrong key")
	}
}

func TestMigratePlaintextToEncryptedPreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.pmforge")
	plain, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	project, err := plain.UpsertProject(Project{
		Name:        "Encryption Migration",
		Description: "plaintext source",
		Status:      "active",
		Phase:       "execution",
		Owner:       "alice",
	})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := plain.SaveChart(Chart{
		ProjectID: project.ID,
		Kind:      "gantt",
		Title:     "Source Chart",
		Data:      `{"tasks":[{"id":"a"}]}`,
	}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if err := plain.Close(); err != nil {
		t.Fatalf("close plaintext: %v", err)
	}

	backupPath, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x33))
	if err != nil {
		t.Fatalf("MigratePlaintextToEncrypted: %v", err)
	}
	if backupPath != path+".pre-encryption.bak" {
		t.Fatalf("backupPath = %q, want %q", backupPath, path+".pre-encryption.bak")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	requireEncryptedHeader(t, path)

	encrypted, err := InitEncryptedDB(path, testDEK(t, 0x33))
	if err != nil {
		t.Fatalf("InitEncryptedDB migrated: %v", err)
	}
	defer encrypted.Close()
	gotProject, err := encrypted.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if gotProject.Name != project.Name || gotProject.Owner != project.Owner {
		t.Fatalf("project after migration = %#v, want name %q owner %q", gotProject, project.Name, project.Owner)
	}
	charts, err := encrypted.ListCharts(project.ID, "")
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(charts) != 1 || charts[0].Title != "Source Chart" {
		t.Fatalf("charts after migration = %#v", charts)
	}
	requireCipherIntegrity(t, encrypted)
}

func TestOpenEncryptedDBRejectsBadDEKLength(t *testing.T) {
	if _, err := InitEncryptedDB(filepath.Join(t.TempDir(), "bad.pmforge"), []byte("short")); err != crypto.ErrBadDEK {
		t.Fatalf("InitEncryptedDB(short) err = %v, want ErrBadDEK", err)
	}
	if _, err := MigratePlaintextToEncrypted(filepath.Join(t.TempDir(), "missing.pmforge"), []byte("short")); err != crypto.ErrBadDEK {
		t.Fatalf("MigratePlaintextToEncrypted(short) err = %v, want ErrBadDEK", err)
	}
}
