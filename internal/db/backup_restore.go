// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const (
	maxBackupEntries     = 34
	maxBackupProjectSize = 1 << 30
	maxBackupManifest    = 1 << 20
	maxBackupCertSize    = 16 << 20
	maxBackupTotalSize   = maxBackupProjectSize + 32*maxBackupCertSize + maxBackupManifest
)

// RestoreArchivalBundle validates a PMForge archive and publishes only its
// project database at destPath. Certificates are integrity-checked but never
// imported automatically because an archive may contain private key material.
func RestoreArchivalBundle(archivePath, destPath string) (manifest BackupManifest, err error) {
	zr, err := zip.OpenReader(archivePath) // #nosec G304 -- selected by the signed-in user and parsed as an untrusted archive below.
	if err != nil {
		return manifest, fmt.Errorf("open backup archive: %w", err)
	}
	defer func() {
		if closeErr := zr.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if len(zr.File) == 0 || len(zr.File) > maxBackupEntries {
		return manifest, fmt.Errorf("backup archive has an invalid entry count")
	}

	entries := make(map[string]*zip.File, len(zr.File))
	var total uint64
	for _, f := range zr.File {
		if !validBackupEntryName(f.Name) || !f.Mode().IsRegular() {
			return manifest, fmt.Errorf("backup archive contains unsafe entry %q", f.Name)
		}
		if _, exists := entries[f.Name]; exists {
			return manifest, fmt.Errorf("backup archive contains duplicate entry %q", f.Name)
		}
		limit := uint64(maxBackupCertSize)
		switch f.Name {
		case "project.pmforge":
			limit = maxBackupProjectSize
		case "manifest.json":
			limit = maxBackupManifest
		}
		if f.UncompressedSize64 > limit {
			return manifest, fmt.Errorf("backup entry %q exceeds its size limit", f.Name)
		}
		total += f.UncompressedSize64
		if total > maxBackupTotalSize {
			return manifest, fmt.Errorf("backup archive exceeds its total size limit")
		}
		entries[f.Name] = f
	}

	manifestFile := entries["manifest.json"]
	projectFile := entries["project.pmforge"]
	if manifestFile == nil || projectFile == nil {
		return manifest, errors.New("backup archive is missing manifest.json or project.pmforge")
	}
	manifestBytes, err := readZipEntry(manifestFile, maxBackupManifest)
	if err != nil {
		return manifest, fmt.Errorf("read backup manifest: %w", err)
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return manifest, fmt.Errorf("parse backup manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return manifest, fmt.Errorf("unsupported backup schema version %d", manifest.SchemaVersion)
	}
	if manifest.EntrySHA256["project.pmforge"] == "" {
		return manifest, errors.New("backup manifest is missing the project digest")
	}
	for name, f := range entries {
		if name == "manifest.json" {
			continue
		}
		want := strings.ToLower(manifest.EntrySHA256[name])
		if len(want) != 64 {
			return manifest, fmt.Errorf("backup manifest has no valid digest for %q", name)
		}
		got, hashErr := hashZipEntry(f)
		if hashErr != nil {
			return manifest, fmt.Errorf("hash backup entry %q: %w", name, hashErr)
		}
		if got != want {
			return manifest, fmt.Errorf("backup entry %q failed its SHA-256 check", name)
		}
	}

	if _, err := os.Lstat(destPath); err == nil {
		return manifest, errors.New("restore destination already exists")
	} else if !os.IsNotExist(err) {
		return manifest, err
	}
	tempPath := destPath + ".tmp.restore"
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return manifest, fmt.Errorf("remove stale restore file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err := extractZipEntry(projectFile, tempPath); err != nil {
		return manifest, err
	}
	if err := os.Rename(tempPath, destPath); err != nil {
		return manifest, fmt.Errorf("publish restored project: %w", err)
	}
	return manifest, nil
}

func validBackupEntryName(name string) bool {
	if name == "manifest.json" || name == "project.pmforge" {
		return true
	}
	return strings.HasPrefix(name, "certs/") && path.Clean(name) == name &&
		!strings.Contains(name, "\\") && path.Base(name) != "." && path.Base(name) != ".." &&
		path.Dir(name) == "certs"
}

func readZipEntry(f *zip.File, limit int64) (data []byte, err error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := r.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("entry exceeds size limit")
	}
	return b, nil
}

func hashZipEntry(f *zip.File) (digest string, err error) {
	r, err := f.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := r.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractZipEntry(f *zip.File, destPath string) (err error) {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := r.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("extract project database: %w", err)
	}
	return nil
}
