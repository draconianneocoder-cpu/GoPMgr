// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newBackupTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := InitDB(filepath.Join(t.TempDir(), "backup.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	return d
}

func TestCreateSnapshotAcceptsQuotedTargetPath(t *testing.T) {
	d := newBackupTestDB(t)
	snapshotPath := filepath.Join(t.TempDir(), "audit's snapshot.pmforge")

	if err := d.CreateSnapshot(snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot with quoted path: %v", err)
	}

	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
}

func TestInitDBCreatesPrivateDatabaseFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "private.pmforge")
	d, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("database mode = %o, want 600", mode)
	}
}

func TestInitDBTightensExistingDatabaseFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "existing.pmforge")
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		t.Fatalf("write existing db file: %v", err)
	}

	d, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("database mode = %o, want 600", mode)
	}
}

func TestCreateArchivalBundleAcceptsQuotedDestination(t *testing.T) {
	d := newBackupTestDB(t)
	destPath := filepath.Join(t.TempDir(), "owner's archive.pmba")

	if err := d.CreateArchivalBundle(destPath, nil); err != nil {
		t.Fatalf("CreateArchivalBundle with quoted path: %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("archive mode = %o, want 600", mode)
	}

	zr, err := zip.OpenReader(destPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer zr.Close()

	wantEntries := map[string]bool{
		"project.pmforge": false,
		"manifest.json":   false,
	}
	for _, f := range zr.File {
		if _, ok := wantEntries[f.Name]; ok {
			wantEntries[f.Name] = true
		}
	}
	for name, found := range wantEntries {
		if !found {
			t.Fatalf("archive missing %s", name)
		}
	}
}

func TestCreateArchivalBundlePreservesEncryptedProjectBytes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "encrypted.pmforge")
	d, err := InitEncryptedDB(dbPath, testDEK(t, 0x55))
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	if _, err := d.UpsertProject(Project{Name: "Encrypted Backup"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	requireEncryptedHeader(t, dbPath)

	destPath := filepath.Join(t.TempDir(), "encrypted.pmba")
	if err := d.CreateArchivalBundle(destPath, nil); err != nil {
		t.Fatalf("CreateArchivalBundle: %v", err)
	}

	zr, err := zip.OpenReader(destPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != "project.pmforge" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open project.pmforge entry: %v", err)
		}
		defer rc.Close()
		header := make([]byte, len(sqliteHeader))
		if _, err := io.ReadFull(rc, header); err != nil {
			t.Fatalf("read archived project header: %v", err)
		}
		if string(header) == sqliteHeader {
			t.Fatal("archived project.pmforge exposes a plaintext SQLite header")
		}
		return
	}
	t.Fatal("archive missing project.pmforge")
}

func TestCreateArchivalBundleRejectsBlockedStaleTempBeforeCreatingArchive(t *testing.T) {
	d := newBackupTestDB(t)
	destPath := filepath.Join(t.TempDir(), "blocked.pmba")
	tempSnapshot := destPath + ".tmp.snapshot"
	if err := os.MkdirAll(tempSnapshot, 0o700); err != nil {
		t.Fatalf("mkdir stale temp snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempSnapshot, "marker"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale temp marker: %v", err)
	}

	err := d.CreateArchivalBundle(destPath, nil)
	if err == nil || !strings.Contains(err.Error(), "BACKUP_STALE_SNAPSHOT_REMOVE_FAILED") {
		t.Fatalf("CreateArchivalBundle error = %v, want stale snapshot remove failure", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("archive path exists after snapshot preflight failure: stat err=%v", statErr)
	}
}

func TestCreateArchivalBundleDoesNotPublishPartialArchiveOnBundleFailure(t *testing.T) {
	d := newBackupTestDB(t)
	dir := t.TempDir()
	destPath := filepath.Join(dir, "partial.pmba")
	unreadableCert := filepath.Join(dir, "certdir.pem")
	if err := os.Mkdir(unreadableCert, 0o700); err != nil {
		t.Fatalf("mkdir cert path: %v", err)
	}

	err := d.CreateArchivalBundle(destPath, []string{unreadableCert})
	if err == nil || !strings.Contains(err.Error(), "CERT_BUNDLING_FAILED") {
		t.Fatalf("CreateArchivalBundle error = %v, want cert bundling failure", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("archive path exists after bundle failure: stat err=%v", statErr)
	}
}
