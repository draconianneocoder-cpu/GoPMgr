// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const (
	patchedSQLCipherAmalgamationSHA256 = "054c25f4d2637247452338f8277884eeaf67731697a18c7a46b5581c7fae8b78"
	sqlCipherNoticesSHA256             = "efd6c5893bde84926481241ef28ae7af8ad1cf520adc9c89517fa21f83d768e9"
	pristineSQLCipherRuntimeTreeSHA256 = "53b55af239a633b04a6fee66f0e5a9a76fdf2f7dc24c57b78e378d4beda6a73b"
	patchedSQLCipherRuntimeTreeSHA256  = "6a77e9228f6395d60122a334a6537e7a19cc32f6d521849730b6942bd997636f"
)

func TestVendoredSQLCipherProvenance(t *testing.T) {
	root := filepath.Join("..", "..", "third_party", "go-sqlcipher")
	notices, err := os.ReadFile(filepath.Join(root, "THIRD_PARTY_NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	assertSHA256(t, "SQLCipher notices", notices, sqlCipherNoticesSHA256)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "GOPMGR.md" || entry.Name() == "THIRD_PARTY_NOTICES.txt" {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) != 92 {
		t.Fatalf("vendored SQLCipher runtime file count = %d, want 92", len(files))
	}

	patched := sha256.New()
	pristine := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		pristineData := data
		if name == "sqlite3.c" {
			pristineData = []byte(restorePristineWALCheckpoint(t, string(data)))
			assertSHA256(t, name+" (patched)", data, patchedSQLCipherAmalgamationSHA256)
		}
		writeInventoryLine(patched, name, data)
		writeInventoryLine(pristine, name, pristineData)
	}
	if got := hex.EncodeToString(patched.Sum(nil)); got != patchedSQLCipherRuntimeTreeSHA256 {
		t.Fatalf("patched SQLCipher runtime tree hash = %s, want %s", got, patchedSQLCipherRuntimeTreeSHA256)
	}
	if got := hex.EncodeToString(pristine.Sum(nil)); got != pristineSQLCipherRuntimeTreeSHA256 {
		t.Fatalf("pristine SQLCipher runtime tree hash = %s, want %s", got, pristineSQLCipherRuntimeTreeSHA256)
	}
}

func writeInventoryLine(dst hash.Hash, name string, data []byte) {
	sum := sha256.Sum256(data)
	_, _ = dst.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
}

func assertSHA256(t *testing.T, name string, data []byte, want string) {
	t.Helper()
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("%s hash = %s, want %s", name, got, want)
	}
}
