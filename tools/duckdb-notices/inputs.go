// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// reviewedInputs freezes the complete file inventory of the seven modules
// reviewed for this audit, including C preambles, headers and native archives.
// Reproduce with the procedure in docs/testing/duckdb-notices-qualification.md.
// Never update these pins without reviewing and verifying the new inputs.
var reviewedInputs = map[string]string{
	"github.com/duckdb/duckdb-go-bindings":                   "7ceffcd292ade730e7a7389c34e8385c8e84df2d1c684112a1f80f529747214e",
	"github.com/duckdb/duckdb-go-bindings/lib/darwin-amd64":  "a4ba2df836f09a300b39d4cd79b5b8e0cb16dd4b5d87c03be10eeb510408bd95",
	"github.com/duckdb/duckdb-go-bindings/lib/darwin-arm64":  "3f169d13eb0d916b405f8f4d8229c572b9914b47dd534fb4f690099c2fef2aa5",
	"github.com/duckdb/duckdb-go-bindings/lib/linux-amd64":   "b463d35137eb1e84e2bacae2bee28c28765a6393fd40c46825ddcfe56c90988d",
	"github.com/duckdb/duckdb-go-bindings/lib/linux-arm64":   "b479d53047cf3c16d9d94964b7c9e5c076a1c59d790a4a9cbb5a5bb2536db657",
	"github.com/duckdb/duckdb-go-bindings/lib/windows-amd64": "15c0dff8ffb7c7bd954a7213ed918ef9b5f855c642e66beb6ffcac9ddd66e970",
	"github.com/duckdb/duckdb-go/v2":                         "1acca3838dd7ed4d019b7e8acdd1119cf374dd5983620ff5d2092e680d48ec35",
}

// moduleInputDigest hashes lexically sorted slash-relative filenames. Each
// record is an eight-byte big-endian filename length, filename bytes, and the
// raw 32-byte SHA-256 digest of the file. Permissions and timestamps are not
// content inputs. Concurrent cache replacement is outside this check's scope.
func moduleInputDigest(dir string) (string, error) {
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nonregular audit input: %s", path)
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(name))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("list audit inputs: %w", err)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		file, err := os.Open(filepath.Join(dir, filepath.FromSlash(name))) // #nosec G304 -- a file in the module being verified.
		if err != nil {
			return "", fmt.Errorf("open audit input %s: %w", name, err)
		}
		fileHash := sha256.New()
		_, readErr := io.Copy(fileHash, file)
		closeErr := file.Close()
		if readErr != nil {
			return "", fmt.Errorf("read audit input %s: %w", name, readErr)
		}
		if closeErr != nil {
			return "", closeErr
		}
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(name)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(name))
		_, _ = h.Write(fileHash.Sum(nil))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func verifyModuleInputs(module, dir string) error {
	want, ok := reviewedInputs[module]
	if !ok {
		return fmt.Errorf("unreviewed native module %s", module)
	}
	got, err := moduleInputDigest(dir)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s audit inputs changed: got %s, reviewed %s; re-review before updating pins", module, got, want)
	}
	return nil
}
