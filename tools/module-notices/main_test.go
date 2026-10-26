// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestIsNoticeName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"LICENSE", true},
		{"LICENSE.txt", true},
		{"License.md", true},
		{"LICENCE", true},
		{"LICENSE-MIT", true},
		{"LICENSE_APACHE", true},
		{"COPYING", true},
		{"NOTICE", true},
		{"NOTICE.txt", true},
		{"PATENTS", true},
		{"UNLICENSE", true},
		{"THIRD_PARTY_NOTICES.txt", true},
		{"third-party-notices.md", true},
		{"license.go", false},
		{"notice_test.go", false},
		{"LICENSE.json", false},
		{"licensed.txt", false},
		{"README.md", false},
		{"AUTHORS", false},
		{"CONTRIBUTORS", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNoticeName(tc.name); got != tc.want {
				t.Fatalf("isNoticeName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "adds the final newline", in: "MIT", want: "MIT\n"},
		{name: "converts CRLF and CR", in: "a\r\nb\rc\r\n", want: "a\nb\nc\n"},
		{name: "drops trailing spaces and blank edges", in: "\n\n a \t\n\f\nb  \n\n\n", want: " a\n\nb\n"},
		{name: "drops a byte-order mark", in: "\uFEFFCopyright", want: "Copyright\n"},
		{name: "keeps inner text", in: "x\n\n  indented\n", want: "x\n\n  indented\n"},
		{name: "rejects invalid UTF-8", in: "caf\xe9", wantErr: "UTF-8"},
		{name: "rejects whitespace only", in: " \r\n\t\n", wantErr: "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeText([]byte(tc.in))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("normalizeText error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeText: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAddLinkedModules(t *testing.T) {
	linux := target{GOOS: "linux", GOARCH: "amd64"}
	mod := func(path, version, dir string) *goModule {
		return &goModule{Path: path, Version: version, Dir: dir}
	}
	tests := []struct {
		name     string
		pkgs     []goPackage
		want     map[moduleKey][]string // module -> package dirs
		wantRepl map[moduleKey]string
		wantErr  string
	}{
		{
			name: "skips std and the main module, groups packages by module",
			pkgs: []goPackage{
				{ImportPath: "fmt", Standard: true},
				{ImportPath: "gopmgr", Module: &goModule{Path: "gopmgr", Main: true}},
				{ImportPath: "example.com/a", Dir: "/m/a", Module: mod("example.com/a", "v1.0.0", "/m/a")},
				{ImportPath: "example.com/a/sub", Dir: "/m/a/sub", Module: mod("example.com/a", "v1.0.0", "/m/a")},
				{ImportPath: "example.com/b", Dir: "/m/b", Module: mod("example.com/b", "v0.2.0", "/m/b")},
			},
			want: map[moduleKey][]string{
				{"example.com/a", "v1.0.0"}: {"/m/a", "/m/a/sub"},
				{"example.com/b", "v0.2.0"}: {"/m/b"},
			},
		},
		{
			name: "records a local replacement by its go.mod path",
			pkgs: []goPackage{{
				ImportPath: "example.com/c", Dir: "/repo/third_party/c",
				Module: &goModule{Path: "example.com/c", Version: "v4.4.2", Dir: "/repo/third_party/c",
					Replace: &goModule{Path: "./third_party/c", Dir: "/repo/third_party/c"}},
			}},
			want:     map[moduleKey][]string{{"example.com/c", "v4.4.2"}: {"/repo/third_party/c"}},
			wantRepl: map[moduleKey]string{{"example.com/c", "v4.4.2"}: "./third_party/c"},
		},
		{
			name: "tolerates the unbuilt frontend embed in the main module",
			pkgs: []goPackage{{
				ImportPath: "gopmgr", Module: &goModule{Path: "gopmgr", Main: true},
				Error: &struct{ Err string }{"main.go:55:12: pattern all:frontend/dist: no matching files found"},
			}},
			want: map[moduleKey][]string{},
		},
		{
			name: "rejects any other main-module error",
			pkgs: []goPackage{{
				ImportPath: "gopmgr", Module: &goModule{Path: "gopmgr", Main: true},
				Error: &struct{ Err string }{"main.go:3:2: syntax error"},
			}},
			wantErr: "syntax error",
		},
		{
			name: "rejects a dependency error",
			pkgs: []goPackage{{
				ImportPath: "example.com/missing",
				Error:      &struct{ Err string }{"no required module provides package example.com/missing"},
			}},
			wantErr: "no required module provides",
		},
		{
			name:    "rejects a package without a module",
			pkgs:    []goPackage{{ImportPath: "example.com/orphan", Dir: "/x"}},
			wantErr: "has no module",
		},
		{
			name:    "rejects a module that is not downloaded",
			pkgs:    []goPackage{{ImportPath: "example.com/d", Module: mod("example.com/d", "v1.0.0", "")}},
			wantErr: "go mod download",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modules := map[moduleKey]*linkedModule{}
			err := addLinkedModules(modules, tc.pkgs, linux)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("addLinkedModules error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("addLinkedModules: %v", err)
			}
			got := map[moduleKey][]string{}
			for k, m := range modules {
				got[k] = sortedKeys(m.PkgDirs)
				if !reflect.DeepEqual(sortedKeys(m.Targets), []string{"linux/amd64"}) {
					t.Errorf("%v targets = %v, want [linux/amd64]", k, sortedKeys(m.Targets))
				}
				if m.Replace != tc.wantRepl[k] {
					t.Errorf("%v replace = %q, want %q", k, m.Replace, tc.wantRepl[k])
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("modules = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecodePackages(t *testing.T) {
	stream := `{"ImportPath":"fmt","Standard":true}
{"ImportPath":"example.com/a","Dir":"/m/a","Module":{"Path":"example.com/a","Version":"v1.0.0","Dir":"/m/a"}}`
	pkgs, err := decodePackages(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("decodePackages: %v", err)
	}
	if len(pkgs) != 2 || !pkgs[0].Standard || pkgs[1].Module == nil || pkgs[1].Module.Version != "v1.0.0" {
		t.Fatalf("decodePackages = %+v", pkgs)
	}
	if _, err := decodePackages(strings.NewReader(`{"ImportPath":`)); err == nil {
		t.Fatal("decodePackages accepted truncated JSON")
	}
}

func TestNoticeFileNames(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("LICENSE", "root")
	write("PATENTS", "grant")
	write("license.go", "package x")
	write("README.md", "readme")
	write("THIRD_PARTY_LICENSES/Boost-LICENSE", "boost")
	write("THIRD_PARTY_LICENSES/.hidden", "skip")
	write("internal/vendored/LICENSE.txt", "nested, on the path to a linked package")
	write("internal/vendored/pkg/x.go", "package pkg")
	write("unlinked/LICENSE", "not on the path to a linked package")
	write("unlinked/y.go", "package unlinked")
	if err := os.Symlink(filepath.Join(root, "LICENSE"), filepath.Join(root, "NOTICE")); err != nil {
		t.Fatal(err)
	}

	pkgDirs := map[string]bool{
		root: true,
		filepath.Join(root, "internal", "vendored", "pkg"): true,
	}
	got, err := noticeFileNames(root, pkgDirs)
	if err != nil {
		t.Fatalf("noticeFileNames: %v", err)
	}
	want := []string{"LICENSE", "PATENTS", "THIRD_PARTY_LICENSES/Boost-LICENSE", "internal/vendored/LICENSE.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("noticeFileNames = %v, want %v", got, want)
	}

	if _, err := noticeFileNames(root, map[string]bool{filepath.Dir(root): true}); err == nil {
		t.Fatal("noticeFileNames accepted a package outside the module root")
	}
}

func TestModuleSectionRequiresANotice(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &linkedModule{Path: "example.com/x", Version: "v1.0.0", Dir: root,
		PkgDirs: map[string]bool{root: true}, Targets: map[string]bool{"linux/amd64": true}}
	if _, err := moduleSection(m); err == nil || !strings.Contains(err.Error(), "no license or notice file") {
		t.Fatalf("moduleSection error = %v, want a missing-license error", err)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	targets := []target{{GOOS: "windows", GOARCH: "amd64"}, {GOOS: "linux", GOARCH: "amd64"}}
	std := section{Heading: "std", Description: "the Go standard library and runtime",
		Targets: []string{"linux/amd64", "windows/amd64"}, Files: []noticeFile{{"LICENSE", "Go\n"}}}
	b := section{Heading: "example.com/b v1.0.0", Targets: []string{"linux/amd64"},
		Files: []noticeFile{{"LICENSE", "B\n"}}}
	a := section{Heading: "example.com/a v0.1.0", Replace: "./third_party/a", Targets: []string{"windows/amd64"},
		Files: []noticeFile{{"LICENSE", "A\n"}, {"sub/NOTICE", "A notice\n"}}}

	first := string(render([]section{b, std, a}, targets))
	second := string(render([]section{a, b, std}, targets))
	if first != second {
		t.Fatal("render output depends on input order")
	}
	for _, want := range []string{
		"Release targets: linux/amd64, windows/amd64.\n",
		"Contents\n--------\n\n  std\n  example.com/a v0.1.0\n  example.com/b v1.0.0\n",
		"Module: example.com/a v0.1.0\nReplaced by: ./third_party/a\nLinked into: windows/amd64\n\n----- LICENSE -----\n\nA\n\n----- sub/NOTICE -----\n\nA notice\n",
		"Module: example.com/b v1.0.0\nLinked into: linux/amd64\n\n----- LICENSE -----\n\nB\n",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("render output lacks %q:\n%s", want, first)
		}
	}
	if strings.Index(first, "Module: std") > strings.Index(first, "Module: example.com/a") {
		t.Fatal("render did not put the standard library first")
	}
	for i, line := range strings.Split(first, "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Fatalf("line %d has trailing whitespace: %q", i+1, line)
		}
	}
}

func TestCheckCurrent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notices.txt")
	if err := checkCurrent(file, []byte("x\n")); err == nil || !strings.Contains(err.Error(), "make module-notices") {
		t.Fatalf("checkCurrent on a missing file = %v, want a regenerate hint", err)
	}
	if err := os.WriteFile(file, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkCurrent(file, []byte("a\nb\n")); err != nil {
		t.Fatalf("checkCurrent on a current file: %v", err)
	}
	err := checkCurrent(file, []byte("a\nc\n"))
	if err == nil || !strings.Contains(err.Error(), `line 2: have "b", want "c"`) {
		t.Fatalf("checkCurrent on a stale file = %v, want the first differing line", err)
	}
	err = checkCurrent(file, []byte("a\nb\nextra\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("checkCurrent on a shorter file = %v, want line 3", err)
	}
}

// TestReleaseTargetsMatchWorkflow keeps releaseTargets in step with the
// release workflow's package matrix.
func TestReleaseTargetsMatchWorkflow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s+platform:\s*([a-z0-9]+/[a-z0-9]+)\s*$`)
	var workflow []string
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		workflow = append(workflow, m[1])
	}
	sort.Strings(workflow)
	var tool []string
	for _, tg := range releaseTargets {
		tool = append(tool, tg.String())
		if !strings.Contains(","+tg.Tags+",", ",desktop,") || !strings.Contains(","+tg.Tags+",", ",production,") ||
			!strings.Contains(","+tg.Tags+",", ",duckdb,") {
			t.Errorf("%s tags %q lack desktop, production or duckdb", tg, tg.Tags)
		}
	}
	sort.Strings(tool)
	if len(workflow) == 0 || !reflect.DeepEqual(tool, workflow) {
		t.Fatalf("releaseTargets = %v, release workflow platforms = %v", tool, workflow)
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
