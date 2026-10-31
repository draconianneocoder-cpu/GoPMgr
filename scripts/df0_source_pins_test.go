// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestDF0SourcePins(t *testing.T) {
	root := filepath.Join("..", "spikes", "adr012-datafusion")
	pins := map[string]string{
		"arrow-ipc-59.3.2":             "bada04672a54fa0d05499aa5218bf2b83be30adf702f58087bc215217864d9aa",
		"parquet-59.3.2":               "3cef8fa68a8ddb45fb4dfe15e96f2f049032eb101153f96e4f1a4ecd76ec83c5",
		"datafusion-55.2.0":            "d026fb0c7240f653d6f88b83bc2bcf9e00a6ece9d790d559b31221c4a92ff924",
		"datafusion-datasource-55.2.0": "727b490b123f5af223e639dab4b7944d85c1e3774643c884e1bd50f2bea3029e",
	}
	var manifest struct {
		Patch struct {
			Crates map[string]struct{ Path string } `toml:"crates-io"`
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "rust", "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Patch.Crates) != len(pins) {
		t.Fatalf("expected exactly four DF0 source overrides, got %v", manifest.Patch.Crates)
	}
	for dir, want := range pins {
		t.Run(dir, func(t *testing.T) {
			name := dir[:strings.LastIndex(dir, "-")]
			if path := manifest.Patch.Crates[name].Path; path != "../third_party/"+dir {
				t.Fatalf("%s override path = %q", name, path)
			}
			got, err := df0SourceDigest(filepath.Join(root, "third_party", dir))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("source pin mismatch: got %s, want %s; review upstream provenance before changing the pin", got, want)
			}
		})
	}
}

func TestDF0ZstdLockPins(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spikes", "adr012-datafusion", "rust", "Cargo.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Package []struct{ Name, Version string }
	}
	if err := toml.Unmarshal(b, &lock); err != nil {
		t.Fatal(err)
	}
	for name, version := range map[string]string{"zstd": "0.14.3", "zstd-safe": "8.2.0"} {
		count := 0
		for _, p := range lock.Package {
			if p.Name != name {
				continue
			}
			count++
			if p.Version != version {
				t.Errorf("%s: unexpected locked version %s, want %s", name, p.Version, version)
			}
		}
		if count != 1 {
			t.Errorf("%s: expected one locked package, got %d", name, count)
		}
	}
}

// Match GOPMGR.md: sorted slash paths, length-prefixed names and raw file hashes.
func df0SourceDigest(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("nonregular source input: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, name := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(name)))
		h.Write(size[:])
		h.Write([]byte(name))
		sum := sha256.Sum256(body)
		h.Write(sum[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func TestDF0SourceDigestDetectsDrift(t *testing.T) {
	tests := []struct {
		name string
		edit func(string) error
	}{
		{"changed bytes", func(root string) error { return os.WriteFile(filepath.Join(root, "a"), []byte("changed"), 0o600) }},
		{"added file", func(root string) error { return os.WriteFile(filepath.Join(root, "extra"), []byte("extra"), 0o600) }},
		{"removed file", func(root string) error { return os.Remove(filepath.Join(root, "a")) }},
		{"renamed file", func(root string) error { return os.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "a", "original")
			before, err := df0SourceDigest(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.edit(root); err != nil {
				t.Fatal(err)
			}
			after, err := df0SourceDigest(root)
			if err != nil {
				t.Fatal(err)
			}
			if before == after {
				t.Fatal("source drift was not detected")
			}
		})
	}
}

func TestDF0SourceDigestRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "source", "source")
	if err := os.Symlink("source", filepath.Join(root, "alias")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := df0SourceDigest(root); err == nil || !strings.Contains(err.Error(), "nonregular source input") {
		t.Fatalf("symlink accepted: %v", err)
	}
}
