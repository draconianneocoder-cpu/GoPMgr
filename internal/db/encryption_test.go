// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/crypto"
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
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup permissions: %v", err)
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %o, want 600", backupInfo.Mode().Perm())
	}
	backupEncrypted, err := IsEncryptedFile(backupPath)
	if err != nil {
		t.Fatalf("IsEncryptedFile(backup): %v", err)
	}
	if backupEncrypted {
		t.Fatal("plaintext backup has an encrypted header")
	}
	requireEncryptedHeader(t, path)
	liveInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat encrypted project: %v", err)
	}
	if os.SameFile(liveInfo, backupInfo) {
		t.Fatal("encrypted project and plaintext backup alias the same file")
	}
	if matches, err := filepath.Glob(backupTempPattern(backupPath)); err != nil || len(matches) != 0 {
		t.Fatalf("backup temp files = %v, err = %v; want none", matches, err)
	}
	if _, err := os.Stat(path + ".encrypted.tmp"); !os.IsNotExist(err) {
		t.Fatalf("encrypted temp stat = %v, want not exist", err)
	}

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

func TestMigratePlaintextToEncryptedKeepsLiveSourceOnBackupPreparationFailure(t *testing.T) {
	path := newPlaintextMigrationFixture(t)
	originalBackup := createPlaintextBackupForMigration
	createPlaintextBackupForMigration = func(_, backupPath string) (string, error) {
		temp, err := os.CreateTemp(filepath.Dir(backupPath), "."+filepath.Base(backupPath)+".tmp-*")
		if err != nil {
			t.Fatalf("create injected backup temp: %v", err)
		}
		if err := temp.Close(); err != nil {
			t.Fatalf("close injected backup temp: %v", err)
		}
		return temp.Name(), errors.New("injected backup copy failure")
	}
	t.Cleanup(func() { createPlaintextBackupForMigration = originalBackup })

	backup, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x51))
	if err == nil || !strings.Contains(err.Error(), "injected backup copy failure") {
		t.Fatalf("MigratePlaintextToEncrypted error = %v, want injected backup failure", err)
	}
	if backup != "" {
		t.Fatalf("backup = %q, want empty on failed migration", backup)
	}
	requireReadablePlaintextMigrationSource(t, path)
	if _, err := os.Stat(path + ".pre-encryption.bak"); !os.IsNotExist(err) {
		t.Fatalf("backup stat = %v, want not exist", err)
	}
	if _, err := os.Stat(path + ".encrypted.tmp"); !os.IsNotExist(err) {
		t.Fatalf("encrypted temp stat = %v, want not exist", err)
	}
	if matches, err := filepath.Glob(backupTempPattern(path + ".pre-encryption.bak")); err != nil || len(matches) != 0 {
		t.Fatalf("backup temp files = %v, err = %v; want none", matches, err)
	}
}

func TestMigratePlaintextToEncryptedKeepsLiveSourceOnPublicationFailure(t *testing.T) {
	path := newPlaintextMigrationFixture(t)
	originalReplace := replaceCanonicalFileForMigration
	var replacementCalls int
	replaceCanonicalFileForMigration = func(source, destination string) error {
		replacementCalls++
		if source != path+".encrypted.tmp" || destination != path {
			t.Fatalf("replace source,destination = %q,%q", source, destination)
		}
		// The canonical path must still be the source immediately before the
		// one replacement operation. This rejects a regression to the old
		// live→backup then temp→live publication sequence.
		encrypted, checkErr := IsEncryptedFile(path)
		if checkErr != nil {
			t.Fatalf("IsEncryptedFile before replacement: %v", checkErr)
		}
		if encrypted {
			t.Fatal("canonical path was replaced before the publication seam")
		}
		return errors.New("injected replacement failure")
	}
	t.Cleanup(func() { replaceCanonicalFileForMigration = originalReplace })

	backup, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x52))
	if err == nil || !strings.Contains(err.Error(), "injected replacement failure") {
		t.Fatalf("MigratePlaintextToEncrypted error = %v, want injected replacement failure", err)
	}
	if backup != "" {
		t.Fatalf("backup = %q, want empty on failed migration", backup)
	}
	if replacementCalls != 1 {
		t.Fatalf("replacement calls = %d, want 1", replacementCalls)
	}
	requireReadablePlaintextMigrationSource(t, path)
	if _, err := os.Stat(path + ".pre-encryption.bak"); !os.IsNotExist(err) {
		t.Fatalf("backup stat = %v, want not exist after failed publication", err)
	}
	if _, err := os.Stat(path + ".encrypted.tmp"); !os.IsNotExist(err) {
		t.Fatalf("encrypted temp stat = %v, want not exist", err)
	}

	replaceCanonicalFileForMigration = originalReplace
	if _, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x52)); err != nil {
		t.Fatalf("MigratePlaintextToEncrypted retry: %v", err)
	}
	requireEncryptedHeader(t, path)
}

func TestMigratePlaintextToEncryptedRejectsExistingBackupWithoutTouchingSource(t *testing.T) {
	path := newPlaintextMigrationFixture(t)
	backupPath := path + ".pre-encryption.bak"
	if err := os.WriteFile(backupPath, []byte("existing backup"), 0o600); err != nil {
		t.Fatalf("write existing backup: %v", err)
	}

	if _, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x53)); err == nil || !strings.Contains(err.Error(), "backup already exists") {
		t.Fatalf("MigratePlaintextToEncrypted error = %v, want existing backup rejection", err)
	}
	requireReadablePlaintextMigrationSource(t, path)
	contents, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read existing backup: %v", err)
	}
	if string(contents) != "existing backup" {
		t.Fatalf("existing backup contents = %q", contents)
	}
}

func TestMigratePlaintextToEncryptedRejectsEncryptedSourceWithoutTouchingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "encrypted.pmforge")
	dek := testDEK(t, 0x54)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close encrypted source: %v", err)
	}

	if _, err := MigratePlaintextToEncrypted(path, dek); err == nil || !strings.Contains(err.Error(), "already encrypted") {
		t.Fatalf("MigratePlaintextToEncrypted error = %v, want already-encrypted rejection", err)
	}
	requireEncryptedHeader(t, path)
	if _, err := os.Stat(path + ".pre-encryption.bak"); !os.IsNotExist(err) {
		t.Fatalf("backup stat = %v, want not exist", err)
	}
}

func TestMigratePlaintextToEncryptedRejectsNonRegularSource(t *testing.T) {
	path := t.TempDir()
	if _, err := MigratePlaintextToEncrypted(path, testDEK(t, 0x55)); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("MigratePlaintextToEncrypted error = %v, want non-regular rejection", err)
	}
}

func newPlaintextMigrationFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plain.pmforge")
	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if _, err := d.UpsertProject(Project{Name: "Encryption publication fixture", Owner: "alice"}); err != nil {
		_ = d.Close()
		t.Fatalf("UpsertProject: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close plaintext fixture: %v", err)
	}
	return path
}

func backupTempPattern(backupPath string) string {
	return filepath.Join(filepath.Dir(backupPath), "."+filepath.Base(backupPath)+".tmp-*")
}

func requireReadablePlaintextMigrationSource(t *testing.T, path string) {
	t.Helper()
	encrypted, err := IsEncryptedFile(path)
	if err != nil {
		t.Fatalf("IsEncryptedFile source: %v", err)
	}
	if encrypted {
		t.Fatal("failed migration encrypted the canonical source")
	}
	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB source after failed migration: %v", err)
	}
	defer d.Close()
	project, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject source after failed migration: %v", err)
	}
	if project.Name != "Encryption publication fixture" {
		t.Fatalf("project after failed migration = %#v", project)
	}
}

func TestOpenEncryptedDBRejectsBadDEKLength(t *testing.T) {
	if _, err := InitEncryptedDB(filepath.Join(t.TempDir(), "bad.pmforge"), []byte("short")); err != crypto.ErrBadDEK {
		t.Fatalf("InitEncryptedDB(short) err = %v, want ErrBadDEK", err)
	}
	if _, err := MigratePlaintextToEncrypted(filepath.Join(t.TempDir(), "missing.pmforge"), []byte("short")); err != crypto.ErrBadDEK {
		t.Fatalf("MigratePlaintextToEncrypted(short) err = %v, want ErrBadDEK", err)
	}
}
