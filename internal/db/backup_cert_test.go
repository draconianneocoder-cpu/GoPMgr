// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readBundle returns every entry of the archive at path and its manifest.
func readBundle(t *testing.T, path string) (map[string][]byte, BackupManifest) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer zr.Close()
	entries := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		entries[f.Name] = data
	}
	var manifest BackupManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return entries, manifest
}

// requireCertRefused checks that bundling certPath fails for wantReason and
// publishes no archive. The error crosses debug.Wrap, which keeps only its
// text, so the reason is matched as text.
func requireCertRefused(t *testing.T, certPath, wantReason string) {
	t.Helper()
	d := newBackupTestDB(t)
	destPath := filepath.Join(t.TempDir(), "refused.pmba")
	err := d.CreateArchivalBundle(destPath, []string{certPath})
	requireCertRefusal(t, err, wantReason)
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("archive published after a refused certificate: stat err=%v", statErr)
	}
}

func requireCertRefusal(t *testing.T, err error, wantReason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("CreateArchivalBundle succeeded, want a refusal: %s", wantReason)
	}
	for _, want := range []string{"CERT_BUNDLING_FAILED", errBackupCertRejected.Error(), wantReason} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("CreateArchivalBundle error = %v, want it to contain %q", err, want)
		}
	}
}

func TestCreateArchivalBundleIncludesACertificateFile(t *testing.T) {
	for _, name := range []string{"signer.p12", "SIGNER.PFX"} {
		t.Run(name, func(t *testing.T) {
			d := newBackupTestDB(t)
			dir := t.TempDir()
			certPath := filepath.Join(dir, name)
			want := []byte("pkcs12 bytes for " + name)
			if err := os.WriteFile(certPath, want, 0o600); err != nil {
				t.Fatalf("write cert: %v", err)
			}
			destPath := filepath.Join(dir, "with-cert.pmba")
			if err := d.CreateArchivalBundle(destPath, []string{certPath}); err != nil {
				t.Fatalf("CreateArchivalBundle: %v", err)
			}

			entries, manifest := readBundle(t, destPath)
			entry := "certs/" + name
			if got := entries[entry]; !bytes.Equal(got, want) {
				t.Fatalf("%s = %q, want %q", entry, got, want)
			}
			sum := sha256.Sum256(want)
			if got := manifest.EntrySHA256[entry]; got != hex.EncodeToString(sum[:]) {
				t.Fatalf("manifest digest for %s = %q, want the digest of the bundled bytes", entry, got)
			}
			if len(manifest.IncludedCerts) != 1 || manifest.IncludedCerts[0] != name {
				t.Fatalf("included certificates = %v, want [%s]", manifest.IncludedCerts, name)
			}
		})
	}
}

// A project's cert_path setting comes from the project database, so a
// restored project can name any file. Only a certificate bundle may be
// copied into an archive.
func TestCreateArchivalBundleRefusesAFileThatIsNotACertificateBundle(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"id_ed25519", "notes.txt", "signer.p12.bak", "p12"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			requireCertRefused(t, path, "is not a .p12 or .pfx file")
		})
	}
}

// The extension check must not depend on the file existing: a path with
// another extension is refused before the file system is touched.
func TestCreateArchivalBundleRefusesAMissingFileThatIsNotACertificateBundle(t *testing.T) {
	requireCertRefused(t, filepath.Join(t.TempDir(), "absent", "id_ed25519"), "is not a .p12 or .pfx file")
}

func TestCreateArchivalBundleRefusesADirectoryNamedLikeACertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signer.p12")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	requireCertRefused(t, path, "is not a regular file")
}

func TestCreateArchivalBundleRefusesAnOversizedCertificate(t *testing.T) {
	dir := t.TempDir()

	atLimit := filepath.Join(dir, "limit.p12")
	if err := os.WriteFile(atLimit, nil, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.Truncate(atLimit, maxBackupCertSize); err != nil {
		t.Fatalf("grow cert: %v", err)
	}
	d := newBackupTestDB(t)
	destPath := filepath.Join(dir, "limit.pmba")
	if err := d.CreateArchivalBundle(destPath, []string{atLimit}); err != nil {
		t.Fatalf("a certificate of exactly the limit must be bundled: %v", err)
	}
	// The archive must restore: the writer's limit is the reader's limit.
	restored := filepath.Join(dir, "restored.gopmgr")
	if _, err := RestoreArchivalBundle(destPath, restored); err != nil {
		t.Fatalf("restore an archive holding a certificate at the limit: %v", err)
	}

	overLimit := filepath.Join(dir, "over.p12")
	if err := os.WriteFile(overLimit, nil, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.Truncate(overLimit, maxBackupCertSize+1); err != nil {
		t.Fatalf("grow cert: %v", err)
	}
	requireCertRefused(t, overLimit, "is larger than 16 MiB")
}

// A certificate that has been moved or deleted is left out, as before.
func TestCreateArchivalBundleSkipsAMissingCertificate(t *testing.T) {
	d := newBackupTestDB(t)
	dir := t.TempDir()
	destPath := filepath.Join(dir, "no-cert.pmba")
	if err := d.CreateArchivalBundle(destPath, []string{"", filepath.Join(dir, "moved.p12")}); err != nil {
		t.Fatalf("CreateArchivalBundle with a missing certificate: %v", err)
	}
	entries, manifest := readBundle(t, destPath)
	for name := range entries {
		if name != "project.gopmgr" && name != "manifest.json" {
			t.Fatalf("unexpected entry %s", name)
		}
	}
	if len(manifest.IncludedCerts) != 0 || len(manifest.EntrySHA256) != 1 {
		t.Fatalf("manifest lists a certificate that was not bundled: %v / %v", manifest.IncludedCerts, manifest.EntrySHA256)
	}
}
