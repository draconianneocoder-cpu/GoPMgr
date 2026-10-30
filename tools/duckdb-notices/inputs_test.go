// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestModuleInputDigest(t *testing.T) {
	const want = "5263587c86c66e542e98dff5b09088a59b5bd9f7a8c25cfa2fb0f878975a363c"
	for _, mutation := range []string{"none", "edit", "add native source", "delete", "rename", "symlink"} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "include"), 0o700); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{"wrapper.go": "package p\n", "include.h": "// root header\n", "include/bridge.h": "// bridge\n"} {
				if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			switch mutation {
			case "edit":
				err = os.WriteFile(filepath.Join(dir, "include", "bridge.h"), []byte("void wrapper(){ tpch_generate(); }\n"), 0o600)
			case "add native source":
				err = os.WriteFile(filepath.Join(dir, "extra.c"), []byte("void extra(){}\n"), 0o600)
			case "delete":
				err = os.Remove(filepath.Join(dir, "wrapper.go"))
			case "rename":
				err = os.Rename(filepath.Join(dir, "wrapper.go"), filepath.Join(dir, "renamed.go"))
			case "symlink":
				err = os.Symlink(filepath.Join(dir, "wrapper.go"), filepath.Join(dir, "linked.go"))
				if err != nil && runtime.GOOS == "windows" {
					t.Skipf("symlink privileges unavailable: %v", err)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := moduleInputDigest(dir)
			if mutation == "symlink" {
				if err == nil || !strings.Contains(err.Error(), "nonregular") {
					t.Fatalf("linked input = %s, %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if (got == want) != (mutation == "none") {
				t.Fatalf("digest %s, mutation %s", got, mutation)
			}
		})
	}
	if _, err := moduleInputDigest(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing inventory accepted")
	}
}

func TestCheckExclusionsRejectsChangedInputsBeforeReadingThem(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wrapper.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := checkExclusions(func(module string) (string, error) { calls++; return dir, nil })
	if err == nil || !strings.Contains(err.Error(), bindingsModule+" audit inputs changed") || calls != 1 {
		t.Fatalf("unreviewed inputs: calls=%d, err=%v", calls, err)
	}
	if err := verifyModuleInputs("unreviewed/module", dir); err == nil {
		t.Fatal("unknown module accepted")
	}
}
