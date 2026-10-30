// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command module-notices collects the license and notice files of the Go
// standard library and of every Go module linked into GoPMgr's release
// binaries, and writes them to one text file that the release packages ship.
//
// Usage, from the repository root:
//
//	go run ./tools/module-notices          # rewrite third_party/GO-MODULE-NOTICES.txt
//	go run ./tools/module-notices -check   # fail if that file is out of date
//
// The module set comes from `go list -deps` on the root package, once per
// release target, with the build tags `wails build` uses. The texts come from
// the module cache, or from a local replacement directory. A module
// contributes the license-like files in its root and in each directory
// between its root and a linked package: the places `go mod vendor` copies
// license metadata from. Files in a THIRD_PARTY_LICENSES or LICENSES
// directory in those places count too.
//
// scripts/verify-module-notices.sh checks a built binary against the file.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const defaultOutput = "third_party/GO-MODULE-NOTICES.txt"

// target is one release build. wails build adds the desktop and production
// tags to the ones the release workflow passes.
type target struct {
	GOOS, GOARCH, Tags string
}

func (t target) String() string { return t.GOOS + "/" + t.GOARCH }

// releaseTargets mirrors the package matrix in .github/workflows/release.yml
// and the tags of its wails build steps. A test keeps the platforms in step.
var releaseTargets = []target{
	{GOOS: "darwin", GOARCH: "arm64", Tags: "desktop,production,duckdb"},
	{GOOS: "linux", GOARCH: "amd64", Tags: "desktop,production,duckdb,webkit2_41"},
	{GOOS: "windows", GOARCH: "amd64", Tags: "desktop,production,duckdb"},
}

func main() {
	output := flag.String("o", defaultOutput, "notices file to write or check")
	check := flag.Bool("check", false, "fail if the notices file is out of date instead of rewriting it")
	flag.Parse()
	if err := run(*output, *check); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "module-notices: %v\n", err)
		os.Exit(1)
	}
}

func run(output string, check bool) error {
	content, err := generate(releaseTargets)
	if err != nil {
		return err
	}
	if check {
		return checkCurrent(output, content)
	}
	if err := os.WriteFile(output, content, 0o644); err != nil { // #nosec G306 -- a public notices file.
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("module-notices: wrote %s\n", output)
	return nil
}

func generate(targets []target) ([]byte, error) {
	modules := map[moduleKey]*linkedModule{}
	for _, t := range targets {
		pkgs, err := listPackages(t)
		if err != nil {
			return nil, err
		}
		if err := addLinkedModules(modules, pkgs, t); err != nil {
			return nil, err
		}
	}
	sections := make([]section, 0, len(modules)+1)
	goroot, err := goEnv("GOROOT")
	if err != nil {
		return nil, err
	}
	std, err := standardLibrarySection(goroot, targets)
	if err != nil {
		return nil, err
	}
	sections = append(sections, std)
	for _, m := range modules {
		s, err := moduleSection(m)
		if err != nil {
			return nil, err
		}
		sections = append(sections, s)
	}
	return render(sections, targets), nil
}

// checkCurrent compares the generated content with the committed file.
func checkCurrent(output string, want []byte) error {
	have, err := os.ReadFile(output) // #nosec G304 -- the notices file named by the caller.
	if err != nil {
		return fmt.Errorf("read %s: %w; run `make module-notices`", output, err)
	}
	if bytes.Equal(have, want) {
		fmt.Printf("module-notices: %s is current\n", output)
		return nil
	}
	return fmt.Errorf("%s is out of date (%s); run `make module-notices` and commit the result", output, firstDifference(have, want))
}

func firstDifference(have, want []byte) string {
	haveLines := strings.Split(string(have), "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(haveLines) || i < len(wantLines); i++ {
		var h, w string
		if i < len(haveLines) {
			h = haveLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if h != w || i >= len(haveLines) || i >= len(wantLines) {
			return fmt.Sprintf("line %d: have %q, want %q", i+1, h, w)
		}
	}
	return "files differ"
}

// goModule and goPackage hold the fields read from `go list -json`.
type goModule struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *goModule
}

type goPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	Module     *goModule
	Error      *struct{ Err string }
}

func listPackages(t target) ([]goPackage, error) {
	// -e keeps listing when the root package's //go:embed of frontend/dist
	// has nothing to match because the frontend is not built; addLinkedModules
	// still rejects every other package error.
	cmd := exec.Command("go", "list", "-e", "-deps", "-tags", t.Tags,
		"-json=ImportPath,Dir,Standard,Module,Error", ".")
	cmd.Env = append(os.Environ(), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "CGO_ENABLED=1", "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list for %s: %w: %s", t, err, strings.TrimSpace(stderr.String()))
	}
	pkgs, err := decodePackages(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("go list for %s: %w", t, err)
	}
	return pkgs, nil
}

func decodePackages(r io.Reader) ([]goPackage, error) {
	dec := json.NewDecoder(r)
	var pkgs []goPackage
	for {
		var p goPackage
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return pkgs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode package list: %w", err)
		}
		pkgs = append(pkgs, p)
	}
}

type moduleKey struct{ Path, Version string }

type linkedModule struct {
	Path, Version string
	// Replace is the replacement as go.mod writes it: a directory, or a
	// module path and version. Empty when the module is not replaced.
	Replace string
	Dir     string
	PkgDirs map[string]bool
	Targets map[string]bool
}

func addLinkedModules(modules map[moduleKey]*linkedModule, pkgs []goPackage, t target) error {
	for _, p := range pkgs {
		if p.Error != nil {
			if p.Module != nil && p.Module.Main && isUnbuiltEmbedError(p.Error.Err) {
				continue
			}
			return fmt.Errorf("go list for %s: package %s: %s", t, p.ImportPath, p.Error.Err)
		}
		if p.Standard {
			continue
		}
		if p.Module == nil {
			return fmt.Errorf("go list for %s: package %s has no module", t, p.ImportPath)
		}
		if p.Module.Main {
			continue
		}
		m, err := linkedModuleFor(modules, p.Module)
		if err != nil {
			return fmt.Errorf("go list for %s: package %s: %w", t, p.ImportPath, err)
		}
		m.PkgDirs[p.Dir] = true
		m.Targets[t.String()] = true
	}
	return nil
}

// isUnbuiltEmbedError matches the error an embed pattern reports when it has
// nothing to match, as the root package's frontend/dist does before a
// frontend build. It does not change the import graph.
func isUnbuiltEmbedError(msg string) bool {
	return strings.Contains(msg, "pattern ") && strings.Contains(msg, "no matching files found")
}

func linkedModuleFor(modules map[moduleKey]*linkedModule, gm *goModule) (*linkedModule, error) {
	key := moduleKey{gm.Path, gm.Version}
	if m, ok := modules[key]; ok {
		return m, nil
	}
	dir := gm.Dir
	replace := ""
	if gm.Replace != nil {
		replace = strings.TrimSpace(gm.Replace.Path + " " + gm.Replace.Version)
		if dir == "" {
			dir = gm.Replace.Dir
		}
	}
	if dir == "" {
		return nil, fmt.Errorf("module %s %s is not downloaded; run `go mod download`", gm.Path, gm.Version)
	}
	m := &linkedModule{
		Path: gm.Path, Version: gm.Version, Replace: replace, Dir: dir,
		PkgDirs: map[string]bool{}, Targets: map[string]bool{},
	}
	modules[key] = m
	return m, nil
}

// noticeFile is one reproduced file. Name is relative to the module root,
// with forward slashes.
type noticeFile struct {
	Name string
	Text string
}

// section is one module's entry in the notices file.
type section struct {
	Heading     string // "std", or "<module path> <version>"
	Description string
	Replace     string
	Targets     []string
	Files       []noticeFile
}

func standardLibrarySection(goroot string, targets []target) (section, error) {
	s := section{Heading: "std", Description: "the Go standard library and runtime"}
	for _, t := range targets {
		s.Targets = append(s.Targets, t.String())
	}
	sort.Strings(s.Targets)
	for _, name := range []string{"LICENSE", "PATENTS"} {
		text, err := readNoticeText(filepath.Join(goroot, name))
		// Homebrew relocates LICENSE above its libexec GOROOT.
		if errors.Is(err, os.ErrNotExist) && filepath.Base(filepath.Clean(goroot)) == "libexec" {
			text, err = readNoticeText(filepath.Join(filepath.Dir(goroot), name))
		}
		if err != nil {
			return section{}, fmt.Errorf("Go standard library: %w", err)
		}
		s.Files = append(s.Files, noticeFile{Name: name, Text: text})
	}
	return s, nil
}

func moduleSection(m *linkedModule) (section, error) {
	names, err := noticeFileNames(m.Dir, m.PkgDirs)
	if err != nil {
		return section{}, fmt.Errorf("module %s %s: %w", m.Path, m.Version, err)
	}
	if len(names) == 0 {
		return section{}, fmt.Errorf("module %s %s: no license or notice file in %s; review its license by hand before it ships", m.Path, m.Version, m.Dir)
	}
	s := section{Heading: m.Path + " " + m.Version, Replace: m.Replace}
	for t := range m.Targets {
		s.Targets = append(s.Targets, t)
	}
	sort.Strings(s.Targets)
	for _, name := range names {
		text, err := readNoticeText(filepath.Join(m.Dir, filepath.FromSlash(name)))
		if err != nil {
			return section{}, fmt.Errorf("module %s %s: %w", m.Path, m.Version, err)
		}
		s.Files = append(s.Files, noticeFile{Name: name, Text: text})
	}
	return s, nil
}

// noticeFileNames lists the notice files of a module whose linked packages
// live in pkgDirs. Names are relative to moduleDir, root files first.
func noticeFileNames(moduleDir string, pkgDirs map[string]bool) ([]string, error) {
	dirs := map[string]bool{}
	for pkgDir := range pkgDirs {
		rel, err := filepath.Rel(moduleDir, pkgDir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("package directory %s is outside the module root %s", pkgDir, moduleDir)
		}
		for d := rel; ; d = filepath.Dir(d) {
			dirs[filepath.ToSlash(d)] = true
			if d == "." {
				break
			}
		}
	}
	seen := map[string]bool{}
	for dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(moduleDir, filepath.FromSlash(dir)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			name := path.Join(dir, e.Name())
			switch {
			case e.Type().IsRegular() && isNoticeName(e.Name()):
				seen[name] = true
			case e.IsDir() && isNoticeDirName(e.Name()):
				files, err := regularFiles(filepath.Join(moduleDir, filepath.FromSlash(name)))
				if err != nil {
					return nil, err
				}
				for _, f := range files {
					seen[path.Join(name, f)] = true
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		di, dj := strings.Count(names[i], "/"), strings.Count(names[j], "/")
		if di != dj {
			return di < dj
		}
		return names[i] < names[j]
	})
	return names, nil
}

func regularFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

// noticePrefixes are the license metadata names `go mod vendor` keeps, less
// the AUTHORS and CONTRIBUTORS lists, plus UNLICENSE and third-party notice
// files. Matching ignores case.
var noticePrefixes = []string{
	"copying", "copyleft", "copyright", "legal", "licence", "license",
	"notice", "patents", "third_party_notices", "third-party-notices", "unlicense",
}

// noticeExtensions keeps source files such as license.go out.
var noticeExtensions = map[string]bool{"": true, ".txt": true, ".md": true, ".markdown": true, ".rst": true}

func isNoticeName(name string) bool {
	lower := strings.ToLower(name)
	ext := path.Ext(lower)
	if !noticeExtensions[ext] {
		return false
	}
	stem := strings.TrimSuffix(lower, ext)
	for _, p := range noticePrefixes {
		if stem == p || strings.HasPrefix(stem, p+"-") || strings.HasPrefix(stem, p+"_") {
			return true
		}
	}
	return false
}

func isNoticeDirName(name string) bool {
	switch strings.ToLower(name) {
	case "licenses", "third_party_licenses", "third-party-licenses":
		return true
	}
	return false
}

// readNoticeText reads a license file and normalizes only its whitespace:
// LF line endings, no trailing spaces, no byte-order mark, one final newline.
func readNoticeText(file string) (string, error) {
	data, err := os.ReadFile(file) // #nosec G304 -- a license file inside the module cache or a replacement directory.
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file, err)
	}
	text, err := normalizeText(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	return text, nil
}

func normalizeText(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", errors.New("not valid UTF-8 text")
	}
	text := strings.TrimPrefix(string(data), "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\f\v")
	}
	text = strings.Trim(strings.Join(lines, "\n"), "\n")
	if text == "" {
		return "", errors.New("empty license file")
	}
	return text + "\n", nil
}

func goEnv(name string) (string, error) {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return "", fmt.Errorf("go env %s: %w", name, err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf("go env %s is empty", name)
	}
	return v, nil
}

const rule = "================================================================================"

// render writes the notices file. The order of sections in the input does
// not matter: std comes first, then modules by path and version.
func render(sections []section, targets []target) []byte {
	sorted := append([]section(nil), sections...)
	sort.Slice(sorted, func(i, j int) bool {
		if (sorted[i].Heading == "std") != (sorted[j].Heading == "std") {
			return sorted[i].Heading == "std"
		}
		return sorted[i].Heading < sorted[j].Heading
	})
	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, t.String())
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Go module notices for GoPMgr\n")
	b.WriteString("============================\n\n")
	b.WriteString("GoPMgr's release binaries contain code from the Go standard library and\n")
	b.WriteString("from the Go modules listed below. Each module's license and notice files\n")
	b.WriteString("follow, as the module publishes them; only line endings and trailing\n")
	b.WriteString("spaces are normalized.\n\n")
	fmt.Fprintf(&b, "Release targets: %s.\n\n", strings.Join(names, ", "))
	b.WriteString("Generated by tools/module-notices. Do not edit this file by hand; after\n")
	b.WriteString("changing go.mod, run `make module-notices`.\n\n")
	b.WriteString("Contents\n--------\n\n")
	for _, s := range sorted {
		fmt.Fprintf(&b, "  %s\n", s.Heading)
	}
	for _, s := range sorted {
		b.WriteString("\n" + rule + "\n")
		fmt.Fprintf(&b, "Module: %s\n", s.Heading)
		if s.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", s.Description)
		}
		if s.Replace != "" {
			fmt.Fprintf(&b, "Replaced by: %s\n", s.Replace)
		}
		fmt.Fprintf(&b, "Linked into: %s\n", strings.Join(s.Targets, ", "))
		for _, f := range s.Files {
			fmt.Fprintf(&b, "\n----- %s -----\n\n", f.Name)
			b.WriteString(f.Text)
		}
	}
	return []byte(b.String())
}
