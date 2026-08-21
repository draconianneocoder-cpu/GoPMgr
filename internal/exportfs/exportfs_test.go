// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package exportfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNewPrivateCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := WriteNewPrivate(path, []byte("complete report")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "complete report" {
		t.Fatalf("contents = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %#o, want 0600", info.Mode().Perm())
	}
}

func TestWriteNewPrivateRefusesExistingArtifactWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteNewPrivate(path, []byte("replace")); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("error = %v, want destination exists", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("existing contents = %q, want keep", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("existing mode = %#o, want unchanged 0644", info.Mode().Perm())
	}
}
