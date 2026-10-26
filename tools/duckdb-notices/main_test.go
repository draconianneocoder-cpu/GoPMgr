// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestComponentTableIsWellFormed(t *testing.T) {
	known := map[string]bool{}
	for _, p := range platforms {
		known[p.Name] = true
	}
	seen := map[string]bool{}
	for _, c := range components {
		if c.Name == "" || seen[c.Name] {
			t.Errorf("component name %q is empty or repeated", c.Name)
		}
		seen[c.Name] = true
		if len(c.Libraries) == 0 || len(c.Parts) == 0 {
			t.Errorf("%s: needs at least one library and one part", c.Name)
		}
		for _, p := range c.OnlyOn {
			if !known[p] {
				t.Errorf("%s: unknown platform %q", c.Name, p)
			}
		}
		for _, p := range c.Parts {
			if _, ok := sources[p.Source]; !ok || p.Path == "" {
				t.Errorf("%s: part %+v has an unknown source or no path", c.Name, p)
			}
			if p.From < 0 || (p.From > 0 && p.To < p.From) {
				t.Errorf("%s: part %+v has a bad line range", c.Name, p)
			}
		}
	}
	for name, s := range sources {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(s.Commit) || s.Tag == "" || s.URL == "" {
			t.Errorf("source %s: pin %+v is incomplete", name, s)
		}
	}
}

// TestComponentsCoverTheLinkedLibraries reads each platform module's cgo
// link flags and checks that the component table only names libraries that
// are linked, and that every linked third-party library is covered.
func TestComponentsCoverTheLinkedLibraries(t *testing.T) {
	// Libraries that hold only DuckDB's own code, under its MIT license.
	duckdbOwn := map[string]bool{
		"libduckdb_generated_extension_loader": true,
		"libautocomplete_extension":            true,
	}
	named := map[string]bool{}
	for _, c := range components {
		for _, l := range c.Libraries {
			named[l] = true
		}
	}
	flag := regexp.MustCompile(`-l(\w+)`)
	for _, p := range platforms {
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", p.Module).Output()
		dir := strings.TrimSpace(string(out))
		if err != nil || dir == "" {
			t.Skipf("%s is not in the module cache; run `go mod download`", p.Module)
		}
		src, err := os.ReadFile(filepath.Join(dir, "prebuilt.go"))
		if err != nil {
			t.Fatalf("read %s link flags: %v", p.Module, err)
		}
		linked := map[string]bool{}
		for _, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "#cgo LDFLAGS:") {
				continue
			}
			for _, m := range flag.FindAllStringSubmatch(line, -1) {
				if strings.HasPrefix(m[1], "duckdb") || strings.HasSuffix(m[1], "_extension") {
					linked["lib"+m[1]] = true
				}
			}
		}
		if len(linked) == 0 {
			t.Fatalf("%s: no DuckDB libraries in its link flags", p.Module)
		}
		for l := range named {
			if !linked[l] {
				t.Errorf("%s: the component table names %s, which is not linked", p.Name, l)
			}
		}
		for l := range linked {
			if !named[l] && !duckdbOwn[l] {
				t.Errorf("%s: %s is linked but no component covers it; review its contents", p.Name, l)
			}
		}
	}
}

func TestCheckModuleVersions(t *testing.T) {
	tests := []struct {
		name    string
		version func(string) (string, error)
		wantErr string
	}{
		{name: "all at the reviewed version", version: func(string) (string, error) { return bindingsVersion, nil }},
		{
			name: "one platform module moved",
			version: func(m string) (string, error) {
				if strings.HasSuffix(m, "linux-amd64") {
					return "v0.10507.0", nil
				}
				return bindingsVersion, nil
			},
			wantErr: "lib/linux-amd64 v0.10507.0",
		},
		{
			name:    "the bindings module moved",
			version: func(string) (string, error) { return "v0.10600.0", nil },
			wantErr: "were reviewed for " + bindingsVersion,
		},
		{
			name:    "go list fails",
			version: func(string) (string, error) { return "", errors.New("boom") },
			wantErr: "boom",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkModuleVersions(tc.version)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkModuleVersions: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("checkModuleVersions error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

// fakeSources writes every part the table names, so generate can run
// without the real checkouts.
func fakeSources(t *testing.T) map[string]string {
	t.Helper()
	dirs := map[string]string{}
	for name := range sources {
		dirs[name] = t.TempDir()
	}
	for _, c := range components {
		for _, p := range c.Parts {
			path := filepath.Join(dirs[p.Source], filepath.FromSlash(p.Path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			n := p.To
			if n == 0 {
				n = 3
			}
			var b strings.Builder
			for i := 1; i <= n+2; i++ {
				b.WriteString(c.Name + " " + p.Path + " line " + strings.Repeat("x", i%3) + "  \r\n")
			}
			if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dirs
}

func TestGenerate(t *testing.T) {
	dirs := fakeSources(t)
	first, err := generate(dirs)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := generate(dirs)
	if err != nil || string(first) != string(second) {
		t.Fatalf("generate is not deterministic (err %v)", err)
	}
	text := string(first)
	lines := strings.Split(text, "\n")
	if lines[3] != tpcLegend {
		t.Fatalf("line 4 = %q, want the TPC legend at the top of the file", lines[3])
	}
	for _, want := range headerLines() {
		if !strings.Contains(text, "\n"+want+"\n") {
			t.Errorf("output lacks header line %q", want)
		}
	}
	for _, c := range components {
		if !strings.Contains(text, "\nComponent: "+c.Name+"\n") {
			t.Errorf("output lacks component %q", c.Name)
		}
	}
	if !strings.Contains(text, "Component: jemalloc\nCompiled into: libduckdb_static\nLinked into: linux/amd64\n") {
		t.Error("jemalloc is not limited to linux/amd64")
	}
	if strings.Contains(text, "\r") {
		t.Error("output keeps CR characters")
	}
	for i, l := range lines {
		if strings.TrimRight(l, " \t") != l {
			t.Fatalf("line %d has trailing whitespace: %q", i+1, l)
		}
	}
	if err := checkPinsText(t, first); err != nil {
		t.Fatalf("checkPins on generated output: %v", err)
	}
}

func checkPinsText(t *testing.T, content []byte) error {
	t.Helper()
	file := filepath.Join(t.TempDir(), "notices.txt")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return checkPins(file)
}

func TestCheckPins(t *testing.T) {
	good, err := generate(fakeSources(t))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string
	}{
		{name: "current", mutate: func(s string) string { return s }},
		{
			name:    "an older bindings version",
			mutate:  func(s string) string { return strings.ReplaceAll(s, bindingsVersion, "v0.10505.0") },
			wantErr: "Reviewed for",
		},
		{
			name: "a platform no longer covered",
			mutate: func(s string) string {
				return strings.Replace(s, "Covers: "+platforms[2].Module, "Covers: elsewhere", 1)
			},
			wantErr: platforms[2].Module,
		},
		{
			name:    "a component missing",
			mutate:  func(s string) string { return strings.ReplaceAll(s, "Component: Zstandard\n", "") },
			wantErr: "Component: Zstandard",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPinsText(t, []byte(tc.mutate(string(good))))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("checkPins: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("checkPins error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
	if err := checkPins(filepath.Join(t.TempDir(), "absent.txt")); err == nil {
		t.Fatal("checkPins accepted a missing file")
	}
}

func TestReadPart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("one\r\ntwo  \nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad"), []byte("caf\xe9"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		p       part
		want    string
		wantErr string
	}{
		{name: "whole file", p: part{Path: "f"}, want: "one\ntwo\nthree\nfour\n"},
		{name: "a line range", p: part{Path: "f", From: 2, To: 3}, want: "two\nthree\n"},
		{name: "a range past the end", p: part{Path: "f", From: 2, To: 9}, wantErr: "out of range"},
		{name: "an inverted range", p: part{Path: "f", From: 3, To: 2}, wantErr: "out of range"},
		{name: "a missing file", p: part{Path: "absent"}, wantErr: "read absent"},
		{name: "invalid UTF-8", p: part{Path: "bad"}, wantErr: "UTF-8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readPart(dir, tc.p)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("readPart error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("readPart = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := readPart("", part{Path: "f"}); err == nil {
		t.Fatal("readPart accepted a missing checkout")
	}
}

func TestVerifyCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitRun("init", "-q")
	gitRun("commit", "-q", "--allow-empty", "-m", "pin")
	head := gitRun("rev-parse", "HEAD")

	if err := verifyCheckout(source{Name: "x", Tag: "v1", Commit: head}, dir); err != nil {
		t.Fatalf("verifyCheckout at the pinned commit: %v", err)
	}
	err := verifyCheckout(source{Name: "x", Tag: "v1", Commit: strings.Repeat("0", 40)}, dir)
	if err == nil || !strings.Contains(err.Error(), head) {
		t.Fatalf("verifyCheckout at another commit = %v, want one naming %s", err, head)
	}
	if err := verifyCheckout(source{Name: "x"}, ""); err == nil {
		t.Fatal("verifyCheckout accepted no directory")
	}
}
