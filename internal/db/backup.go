// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"pmforge/internal/cli"
	"pmforge/internal/debug"
)

// BackupManifest is the JSON document placed inside every .pmba archive
// at /manifest.json. It records exactly when and how the bundle was
// produced so auditors can verify provenance years later.
type BackupManifest struct {
	CreatedAt     time.Time `json:"created_at"`
	AppVersion    string    `json:"app_version"`
	DatabaseID    string    `json:"database_id"`
	IncludedCerts []string  `json:"included_certificates"`
}

// CreateArchivalBundle produces a single .pmba file containing:
//   - project.pmforge  (a fresh snapshot of the live database)
//   - certs/*          (every certificate file in certPaths that exists)
//   - manifest.json    (BackupManifest with nanosecond-precision UTC ts)
//
// It refuses to back up a corrupt database — there is no point in
// preserving bad state. Run InformativeSelfHeal first if integrity
// fails.
func (db *Database) CreateArchivalBundle(destPath string, certPaths []string) error {
	// 1. Integrity gate.
	ok, err := db.CheckIntegrity()
	if err != nil {
		return debug.Wrap(err, "BACKUP_INTEGRITY_CHECK_ERROR").ToError()
	}
	if !ok {
		return debug.Wrap(
			fmt.Errorf("integrity_check did not return ok"),
			"REFUSED_BACKUP_OF_CORRUPT_DATABASE",
		).ToError()
	}

	// 2. Create the archive file.
	archiveFile, err := os.Create(destPath)
	if err != nil {
		return debug.Wrap(err, "BACKUP_FILE_CREATION_FAILED").ToError()
	}
	defer archiveFile.Close()

	zipWriter := zip.NewWriter(archiveFile)
	defer zipWriter.Close()

	// 3. Snapshot the database into a temp file, then bundle it.
	tempDB := destPath + ".tmp.snapshot"
	// VACUUM INTO refuses to overwrite, so clear any stale temp first.
	_ = os.Remove(tempDB)
	if err := db.CreateSnapshot(tempDB); err != nil {
		return debug.Wrap(err, "BACKUP_SNAPSHOT_FAILED").ToError()
	}
	defer os.Remove(tempDB)

	if err := addFileToZip(zipWriter, tempDB, "project.pmforge"); err != nil {
		return debug.Wrap(err, "BACKUP_SNAPSHOT_BUNDLE_FAILED").ToError()
	}

	// 4. Bundle certificates that actually exist on disk.
	var backedUpCerts []string
	for _, certPath := range certPaths {
		if certPath == "" {
			continue
		}
		if _, err := os.Stat(certPath); err != nil {
			// Silently skip missing certs — they may have been moved.
			continue
		}
		fileName := filepath.Base(certPath)
		if err := addFileToZip(zipWriter, certPath, "certs/"+fileName); err != nil {
			return debug.Wrap(err, "CERT_BUNDLING_FAILED").ToError()
		}
		backedUpCerts = append(backedUpCerts, fileName)
	}

	// 5. Manifest.
	manifest := BackupManifest{
		CreatedAt:     time.Now().UTC(),
		AppVersion:    cli.Version,
		IncludedCerts: backedUpCerts,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return debug.Wrap(err, "MANIFEST_MARSHAL_FAILED").ToError()
	}
	mFile, err := zipWriter.Create("manifest.json")
	if err != nil {
		return debug.Wrap(err, "MANIFEST_CREATE_FAILED").ToError()
	}
	if _, err := mFile.Write(manifestData); err != nil {
		return debug.Wrap(err, "MANIFEST_WRITE_FAILED").ToError()
	}

	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, destName string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := zw.Create(destName)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}
