// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

const restoreTestMethod = 0xfd12

// ZIP registration is process-wide. Serialize only tests using this method;
// the hook lives in the test decompressor, not in production restore code.
var restoreTestDecoder struct {
	once   sync.Once
	mu     sync.Mutex
	onRead func()
}

type restoreTestReader struct {
	io.Reader
	onRead func()
}

func (r *restoreTestReader) Read(p []byte) (int, error) {
	if r.onRead != nil {
		hook := r.onRead
		r.onRead = nil
		hook()
	}
	return r.Reader.Read(p)
}

func (*restoreTestReader) Close() error { return nil }

type restoreTestWriter struct{ io.Writer }

func (restoreTestWriter) Close() error { return nil }

func restoreFixture(t *testing.T, body []byte, hook func()) (string, []byte) {
	t.Helper()
	restoreTestDecoder.mu.Lock()
	t.Cleanup(func() {
		restoreTestDecoder.onRead = nil
		restoreTestDecoder.mu.Unlock()
	})
	restoreTestDecoder.once.Do(func() {
		zip.RegisterDecompressor(restoreTestMethod, func(r io.Reader) io.ReadCloser {
			return &restoreTestReader{Reader: r, onRead: restoreTestDecoder.onRead}
		})
	})
	restoreTestDecoder.onRead = hook
	digest := sha256.Sum256(body)
	manifest, err := json.Marshal(BackupManifest{
		SchemaVersion: 2,
		EntrySHA256:   map[string]string{"project.gopmgr": hex.EncodeToString(digest[:])},
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(restoreTestMethod, func(w io.Writer) (io.WriteCloser, error) {
		return restoreTestWriter{w}, nil
	})
	for _, entry := range []struct {
		name string
		body []byte
	}{{"manifest.json", manifest}, {"project.gopmgr", body}} {
		method := uint16(zip.Store)
		if entry.name == "project.gopmgr" {
			method = restoreTestMethod
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: entry.name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.pmba")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, body
}

func TestRestoreArchivalBundlePreservesDestinationCreatedDuringExtraction(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "restored.gopmgr")
	reads := 0
	var sentinelMode os.FileMode
	archive, _ := restoreFixture(t, []byte("complete restored project bytes"), func() {
		reads++
		if reads == 2 {
			if err := os.WriteFile(dest, []byte("other operation"), 0o644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(dest)
			if err != nil {
				t.Fatal(err)
			}
			sentinelMode = info.Mode()
		}
	})
	_, err := RestoreArchivalBundle(archive, dest)
	if reads != 2 {
		t.Fatalf("project reads = %d, want hash then extraction", reads)
	}
	got, readErr := os.ReadFile(dest)
	if readErr != nil || string(got) != "other operation" {
		t.Fatalf("late destination changed: %q, %v", got, readErr)
	}
	if err == nil {
		t.Fatal("restore succeeded after another operation created its destination")
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode() != sentinelMode {
		t.Fatalf("late destination mode changed: %v, %v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil || len(entries) != 1 {
		t.Fatalf("restore left staging files: %v, %v", entries, err)
	}
}

func TestRestoreArchivalBundleDoesNotRemoveAnotherRestoresScratch(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "restored.gopmgr")
	scratch := dest + ".tmp.restore"
	if err := os.WriteFile(scratch, []byte("another restore owns this"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, body := restoreFixture(t, []byte("complete restored project bytes"), nil)
	if _, err := RestoreArchivalBundle(archive, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(scratch)
	if err != nil || string(got) != "another restore owns this" {
		t.Fatalf("another restore's scratch changed: %q, %v", got, err)
	}
	got, err = os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("restored body = %q, %v", got, err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("restored file is not private: %v", info.Mode())
	}
}

func TestRestoreArchivalBundleRejectsUnderstatedDecodedSize(t *testing.T) {
	body := []byte("this decoded body exceeds its advertised one-byte size")
	sum := sha256.Sum256(body)
	manifest, err := json.Marshal(BackupManifest{
		SchemaVersion: 2,
		EntrySHA256:   map[string]string{"project.gopmgr": hex.EncodeToString(sum[:])},
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifest); err != nil {
		t.Fatal(err)
	}
	w, err = zw.CreateRaw(&zip.FileHeader{
		Name: "project.gopmgr", Method: zip.Store,
		CompressedSize64: uint64(len(body)), UncompressedSize64: 1,
		CRC32: crc32.ChecksumIEEE(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "understated.pmba")
	if err := os.WriteFile(archive, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "restored.gopmgr")
	if _, err := RestoreArchivalBundle(archive, dest); !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("understated size: %v, want ZIP format error", err)
	}
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid archive created output: %v, %v", entries, err)
	}
}

func TestRestoreArchivalBundleRejectsChangedExtractionBytes(t *testing.T) {
	before := []byte{0x42, 0x5a, 0xb9, 0xe9, 0xe0, 0xaa, 0x22, 0x13, 0xb4, 0xc3, 0xe1, 0x92}
	collision := []byte{0xbe, 0x55, 0x21, 0x4e, 0xb3, 0x9d, 0xda, 0x07, 0x61, 0x89, 0x7c, 0xc1}
	if len(before) != len(collision) || crc32.ChecksumIEEE(before) != crc32.ChecksumIEEE(collision) ||
		sha256.Sum256(before) == sha256.Sum256(collision) {
		t.Fatal("fixture must contain distinct equal-length CRC32-colliding payloads")
	}
	for _, tc := range []struct {
		name  string
		after []byte
	}{
		{"CRC mismatch", []byte("other data!!")},
		{"SHA256 mismatch despite valid CRC", collision},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "restored.gopmgr")
			reads := 0
			var archive string
			var offset int64
			archive, _ = restoreFixture(t, before, func() {
				reads++
				if reads == 2 {
					f, err := os.OpenFile(archive, os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					n, writeErr := f.WriteAt(tc.after, offset)
					closeErr := f.Close()
					if writeErr != nil || closeErr != nil || n != len(tc.after) {
						t.Fatalf("change archive: wrote %d, %v, %v", n, writeErr, closeErr)
					}
				}
			})
			zr, err := zip.OpenReader(archive)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range zr.File {
				if f.Name == "project.gopmgr" {
					offset, err = f.DataOffset()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := zr.Close(); err != nil {
				t.Fatal(err)
			}
			if offset == 0 {
				t.Fatal("fixture has no project entry")
			}
			_, restoreErr := RestoreArchivalBundle(archive, dest)
			if reads != 2 {
				t.Fatalf("project reads = %d, want hash then extraction", reads)
			}
			if restoreErr == nil {
				got, _ := os.ReadFile(dest)
				t.Fatalf("changed extraction published: %x", got)
			}
			entries, err := os.ReadDir(filepath.Dir(dest))
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed extraction left output: %v, %v", entries, err)
			}
		})
	}
}
