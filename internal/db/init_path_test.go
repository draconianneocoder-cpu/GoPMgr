// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SEC-04 hardening: the driver reads everything after the first '?' of a
// plaintext path as connection options, so "proj?_journal_mode=OFF" opened
// a file named "proj" with weakened settings. InitDB now refuses such a
// path and creates nothing.
func TestInitDBRefusesAQuestionMarkInThePath(t *testing.T) {
	dir := t.TempDir()
	d, err := InitDB(filepath.Join(dir, "proj?_journal_mode=OFF&x="))
	if err == nil {
		_ = d.Close()
		t.Fatal("InitDB accepted a path containing '?'")
	}
	if !strings.Contains(err.Error(), "illegal character") {
		t.Fatalf("InitDB error = %v, want an illegal-character error", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("InitDB created %q although it refused the path", entries[0].Name())
	}
}

// A '#' is not special in a plaintext path: the driver splits only at '?'.
// Existing plaintext projects whose names contain '#' must still open, so
// that they can be migrated to encryption.
func TestInitDBStillOpensAHashInThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Phase #2.gopmgr")
	d, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB(%q): %v", path, err)
	}
	_ = d.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("InitDB did not create the named file: %v", err)
	}
}
