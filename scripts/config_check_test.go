// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRepositoryConfigs(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(map[string][]byte, *[]string)
		wantErrText string
	}{
		{
			name: "accepts the supported format inventory",
		},
		{
			name: "rejects malformed YAML",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte("name: CI\njobs: [\n")
			},
			wantErrText: ".github/workflows/ci.yml: parse YAML",
		},
		{
			name: "rejects duplicate YAML keys",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/dependabot.yml"] = []byte("version: 2\nversion: 2\nupdates: []\n")
			},
			wantErrText: `.github/dependabot.yml: duplicate key "version"`,
		},
		{
			name: "rejects multiple YAML documents",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte("name: CI\non: push\njobs: {verify: {}}\n---\nname: shadow\n")
			},
			wantErrText: ".github/workflows/ci.yml: multiple YAML documents are not supported",
		},
		{
			name: "rejects malformed TOML",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".gitleaks.toml"] = []byte("[extend\nuseDefault = true\n")
			},
			wantErrText: ".gitleaks.toml: parse TOML",
		},
		{
			name: "rejects a stale GitLab workflow",
			mutate: func(files map[string][]byte, tracked *[]string) {
				files[".gitlab-ci.yml"] = []byte("test:\n  script: make verify\n")
				*tracked = append(*tracked, ".gitlab-ci.yml")
			},
			wantErrText: ".gitlab-ci.yml: legacy GitLab CI configuration is not supported",
		},
		{
			name: "rejects an unclassified configuration",
			mutate: func(files map[string][]byte, tracked *[]string) {
				files["config/extra.yaml"] = []byte("enabled: true\n")
				*tracked = append(*tracked, "config/extra.yaml")
			},
			wantErrText: "config/extra.yaml: tracked YAML/TOML file is not classified",
		},
		{
			name: "rejects a missing required configuration",
			mutate: func(files map[string][]byte, tracked *[]string) {
				delete(files, "REUSE.toml")
				*tracked = removePath(*tracked, "REUSE.toml")
			},
			wantErrText: "REUSE.toml: required configuration is not tracked",
		},
		{
			name: "rejects the wrong golangci schema version",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".golangci.yml"] = []byte("version: \"1\"\nlinters: {}\nformatters: {}\n")
			},
			wantErrText: `.golangci.yml: expected version "2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, tracked := validConfigFixture()
			if tt.mutate != nil {
				tt.mutate(files, &tracked)
			}

			errs := validateRepositoryConfigs(files, tracked)
			if tt.wantErrText == "" {
				if len(errs) != 0 {
					t.Fatalf("validateRepositoryConfigs() returned unexpected errors: %v", errs)
				}
				return
			}
			if !errorsContain(errs, tt.wantErrText) {
				t.Fatalf("validateRepositoryConfigs() errors = %v, want text %q", errs, tt.wantErrText)
			}
		})
	}
}

func TestTrackedConfigPathsUsesPresentVersionControlCandidates(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, ".github/dependabot.yml", "version: 2\nupdates: []\n")
	writeTestFile(t, root, ".gitlab-ci.yml", "test:\n  script: make verify\n")
	runGit(t, root, "init")
	runGit(t, root, "add", ".github/dependabot.yml", ".gitlab-ci.yml")

	// A pre-commit check must see a new config but must not try to parse a
	// tracked file that this same change intentionally removed.
	if err := os.Remove(filepath.Join(root, ".gitlab-ci.yml")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "extra.toml", "enabled = true\n")

	paths, err := trackedConfigPaths(root)
	if err != nil {
		t.Fatalf("trackedConfigPaths() error = %v", err)
	}
	if !containsPath(paths, ".github/dependabot.yml") {
		t.Fatalf("trackedConfigPaths() = %v, want tracked present config", paths)
	}
	if !containsPath(paths, "extra.toml") {
		t.Fatalf("trackedConfigPaths() = %v, want untracked config candidate", paths)
	}
	if containsPath(paths, ".gitlab-ci.yml") {
		t.Fatalf("trackedConfigPaths() = %v, did not want working-tree deletion", paths)
	}
}

func validConfigFixture() (map[string][]byte, []string) {
	files := map[string][]byte{
		".github/dependabot.yml": []byte(`
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
`),
		".github/workflows/ci.yml": []byte(`
name: CI
on:
  push:
jobs:
  verify:
    runs-on: ubuntu-latest
    steps: []
`),
		".github/workflows/release.yml": []byte(`
name: Release
on:
  push:
    tags: ["v*"]
jobs:
  build:
    runs-on: ubuntu-latest
    steps: []
`),
		".golangci.yml": []byte(`
version: "2"
linters: {}
formatters: {}
`),
		"build/linux/nfpm.yaml": []byte(`
name: pmforge
arch: amd64
platform: linux
version: ${VERSION}
contents:
  - src: ./build/bin/pmforge
    dst: /usr/bin/pmforge
overrides:
  deb: {}
  rpm: {}
`),
		".gitleaks.toml": []byte(`
[extend]
useDefault = true

[allowlist]
paths = ["docs/design/spike-sqlcipher/.*"]
`),
		"REUSE.toml": []byte(`
version = 1

[[annotations]]
path = "frontend/package.json"
precedence = "aggregate"
SPDX-FileCopyrightText = "The PMForge Contributors"
SPDX-License-Identifier = "GPL-3.0-or-later"
`),
	}
	tracked := make([]string, 0, len(files))
	for _, spec := range supportedConfigs {
		tracked = append(tracked, spec.path)
	}
	return files, tracked
}

func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

func removePath(paths []string, target string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != target {
			result = append(result, path)
		}
	}
	return result
}

func errorsContain(errs []error, text string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), text) {
			return true
		}
	}
	return false
}
