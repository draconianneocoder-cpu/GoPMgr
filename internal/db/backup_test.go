// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	// "project.gopmgr" is asserted as a literal, not derived from a
	// constant, because it is a persistence boundary: every .pmba archive
	// this build produces has an entry with exactly this name, and
	// RestoreArchivalBundle (backup_restore.go) looks it up by this same
	// literal for schema_version 2. If a future rebrand or refactor renamed
	// it in lockstep on both sides, a round-trip create+restore test would
	// still pass — only pinning the on-disk name here catches that. Renamed
	// 2026-08-04 from "project.pmforge" (schema_version 1); see
	// TestRestoreArchivalBundleReadsSchemaVersion1Archive below for proof
	// that old-format archives still restore.
	wantEntries := map[string]bool{
		"project.gopmgr": false,
		"manifest.json":  false,
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

func TestRestoreArchivalBundleValidatesAndPublishesProjectOnly(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Restore Source"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "source.pmba")
	if err := d.CreateArchivalBundle(archivePath, nil); err != nil {
		t.Fatalf("CreateArchivalBundle: %v", err)
	}
	destPath := filepath.Join(t.TempDir(), "restored.pmforge")
	manifest, err := RestoreArchivalBundle(archivePath, destPath)
	if err != nil {
		t.Fatalf("RestoreArchivalBundle: %v", err)
	}
	if manifest.SchemaVersion != 2 || manifest.DatabaseID != project.ID {
		t.Fatalf("manifest = %#v, want schema 2 and database %q", manifest, project.ID)
	}
	restored, err := InitDB(destPath)
	if err != nil {
		t.Fatalf("InitDB restored project: %v", err)
	}
	defer restored.Close()
	got, err := restored.GetProject()
	if err != nil || got.ID != project.ID {
		t.Fatalf("restored project = %#v, err %v", got, err)
	}
}

func TestRestoreArchivalBundleRejectsTraversalEntry(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.pmba")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"../outside":      "bad",
		"project.pmforge": "db",
		"manifest.json":   string(mustJSON(t, BackupManifest{SchemaVersion: 1})),
	} {
		w, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := w.Write([]byte(body)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = RestoreArchivalBundle(archivePath, filepath.Join(t.TempDir(), "restored.pmforge"))
	if err == nil || !strings.Contains(err.Error(), "unsafe entry") {
		t.Fatalf("RestoreArchivalBundle error = %v, want unsafe entry", err)
	}
}

// TestRestoreArchivalBundleReadsSchemaVersion1Archive proves the dual-read
// compatibility RestoreArchivalBundle claims for pre-2026-08-04 archives.
// CreateArchivalBundle only ever writes the current (schema_version 2,
// "project.gopmgr") format now, so a round-trip create+restore test can
// never exercise the schema_version 1 ("project.pmforge") path — this hand-
// builds a v1 archive byte-for-byte the way a pre-rename release did, using
// the same zip-plus-manifest shape TestRestoreArchivalBundleRejectsTraversalEntry
// uses, but with a correct SHA-256 digest so it passes the integrity check
// and actually restores. If the v1 compatibility branch in
// RestoreArchivalBundle/schemaProjectEntry were ever deleted, this test
// fails — verified by temporarily deleting it and confirming the failure.
func TestRestoreArchivalBundleReadsSchemaVersion1Archive(t *testing.T) {
	const projectBytes = "legacy pre-rename project bytes"
	sum := sha256.Sum256([]byte(projectBytes))
	digest := hex.EncodeToString(sum[:])

	archivePath := filepath.Join(t.TempDir(), "legacy.pmba")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	entries := map[string]string{
		"project.pmforge": projectBytes,
		"manifest.json": string(mustJSON(t, BackupManifest{
			SchemaVersion: 1,
			DatabaseID:    "prj_legacy",
			EntrySHA256:   map[string]string{"project.pmforge": digest},
		})),
	}
	for name, body := range entries {
		w, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := w.Write([]byte(body)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(t.TempDir(), "restored-legacy.pmforge")
	manifest, err := RestoreArchivalBundle(archivePath, destPath)
	if err != nil {
		t.Fatalf("RestoreArchivalBundle on a v1 archive: %v", err)
	}
	if manifest.SchemaVersion != 1 || manifest.DatabaseID != "prj_legacy" {
		t.Fatalf("manifest = %#v, want schema 1 and database %q", manifest, "prj_legacy")
	}
	got, err := os.ReadFile(destPath)
	if err != nil || string(got) != projectBytes {
		t.Fatalf("restored project bytes = %q err %v, want %q", got, err, projectBytes)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
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
		if f.Name != "project.gopmgr" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open project.gopmgr entry: %v", err)
		}
		defer rc.Close()
		header := make([]byte, len(sqliteHeader))
		if _, err := io.ReadFull(rc, header); err != nil {
			t.Fatalf("read archived project header: %v", err)
		}
		if string(header) == sqliteHeader {
			t.Fatal("archived project.gopmgr exposes a plaintext SQLite header")
		}
		return
	}
	t.Fatal("archive missing project.gopmgr")
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
