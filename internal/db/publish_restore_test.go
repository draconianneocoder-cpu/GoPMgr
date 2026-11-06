// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublishRestoredFileDoesNotReplaceExistingEntries(t *testing.T) {
	for _, kind := range []string{"absent", "file", "directory", "dangling symlink", "missing parent"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "staging")
			destination := filepath.Join(dir, "destination")
			if err := os.WriteFile(source, []byte("complete project"), 0o600); err != nil {
				t.Fatal(err)
			}
			sourceInfo, err := os.Stat(source)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && sourceInfo.Mode().Perm() != 0o600 {
				t.Fatal("staging permissions are not private")
			}
			switch kind {
			case "file":
				if err := os.WriteFile(destination, []byte("keep"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink("missing target", destination); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("symlink privileges unavailable: %v", err)
					}
					t.Fatal(err)
				}
			case "missing parent":
				destination = filepath.Join(dir, "missing", "destination")
			}
			before, _ := os.Lstat(destination)
			err = publishRestoredFile(source, destination)
			if kind == "absent" {
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(destination)
				if err != nil || string(got) != "complete project" {
					t.Fatalf("published content = %q, %v", got, err)
				}
				if _, err := os.Lstat(source); !os.IsNotExist(err) {
					t.Fatalf("publication did not consume staging: %v", err)
				}
				info, err := os.Stat(destination)
				if err != nil || info.Mode() != sourceInfo.Mode() {
					t.Fatalf("published mode = %v, %v", info, err)
				}
				return
			}
			if err == nil {
				t.Fatal("publication unexpectedly succeeded")
			}
			got, err := os.ReadFile(source)
			if err != nil || string(got) != "complete project" {
				t.Fatalf("failure changed staging: %q, %v", got, err)
			}
			if kind == "missing parent" {
				if _, err := os.Lstat(destination); !os.IsNotExist(err) {
					t.Fatalf("failure created a destination: %v", err)
				}
				return
			}
			after, err := os.Lstat(destination)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatalf("existing entry changed: %v, %v", after, err)
			}
			if kind == "file" {
				got, err := os.ReadFile(destination)
				if err != nil || string(got) != "keep" {
					t.Fatalf("existing content changed: %q, %v", got, err)
				}
			}
			if kind == "dangling symlink" {
				target, err := os.Readlink(destination)
				if err != nil || target != "missing target" {
					t.Fatalf("symlink changed: %q, %v", target, err)
				}
			}
		})
	}
}
