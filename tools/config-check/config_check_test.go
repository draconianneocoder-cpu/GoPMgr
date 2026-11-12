// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
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
			name: "accepts tracked actionlint configuration",
			mutate: func(_ map[string][]byte, tracked *[]string) {
				if !containsPath(*tracked, ".github/actionlint.yaml") {
					*tracked = append(*tracked, ".github/actionlint.yaml")
				}
			},
		},
		{
			name: "rejects malformed actionlint configuration",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/actionlint.yaml"] = []byte("self-hosted-runner: [\n")
			},
			wantErrText: ".github/actionlint.yaml: parse YAML",
		},
		{
			name: "rejects malformed auxiliary lockfile",
			mutate: func(files map[string][]byte, _ *[]string) {
				files["mise.lock"] = []byte("[broken\n")
			},
			wantErrText: "mise.lock: parse TOML",
		},
		{
			name: "rejects legacy task schema",
			mutate: func(files map[string][]byte, _ *[]string) {
				files["Taskfile.yml"] = []byte("version: '2'\ntasks: {build: {}}\n")
			},
			wantErrText: "expected Taskfile version 3",
		},
		{
			name: "rejects empty build tasks",
			mutate: func(files map[string][]byte, _ *[]string) {
				files["build/Taskfile.yml"] = []byte("version: '3'\ntasks: {}\n")
			},
			wantErrText: "tasks must be a nonempty mapping",
		},
		{
			name: "rejects missing product identity",
			mutate: func(files map[string][]byte, _ *[]string) {
				files["build/config.yml"] = []byte("version: '3'\ninfo: {version: '1.1.0', productName: GoPMgr}\n")
			},
			wantErrText: "info.productIdentifier must be a nonempty string",
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
		{
			name: "rejects a non-blocking CI assurance job",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"  assurance:\n    name: Assurance checks",
					"  assurance:\n    continue-on-error: true\n    name: Assurance checks",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance continue-on-error must be absent or false",
		},
		{
			name: "rejects an assurance job dependency that can propagate a skip",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"  assurance:\n    name: Assurance checks",
					"  assurance:\n    needs: verify\n    name: Assurance checks",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must not define needs",
		},
		{
			name: "rejects CI assurance without the ledger drift gate",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"      - name: Coverage ledger drift\n        run: make coverage-ledger-drift\n",
					"      - name: Coverage ledger drift\n        run: echo skipped\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must have an exact make coverage-ledger-drift step",
		},
		{
			name: "rejects a lookalike license gate",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"        run: make license-check\n",
					"        run: echo 'make license-check'\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must have an exact make license-check step",
		},
		{
			name: "rejects a conditional assurance gate",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"      - name: Coverage ledger drift\n",
					"      - name: Coverage ledger drift\n        if: \"${{ false }}\"\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance steps must not define if",
		},
		{
			name: "rejects a custom shell on an assurance step",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"      - name: Coverage ledger drift\n        run: make coverage-ledger-drift\n",
					"      - name: Coverage ledger drift\n        shell: echo {0}\n        run: make coverage-ledger-drift\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance steps must use the default shell",
		},
		{
			name: "rejects a custom default shell for the assurance job",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"    runs-on: macos-26\n    steps:\n",
					"    runs-on: macos-26\n    defaults:\n      run:\n        shell: echo {0}\n    steps:\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance defaults.run.shell must not be defined",
		},
		{
			name: "rejects a workflow default shell that overrides assurance",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"name: CI\non:\n",
					"name: CI\ndefaults:\n  run:\n    shell: echo {0}\non:\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: workflow defaults.run.shell must not override jobs.assurance",
		},
		{
			name: "rejects an assurance runner older than the macOS 26 minimum",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"    runs-on: macos-26\n",
					"    runs-on: macos-15\n",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must run on macos-26",
		},
		{
			name: "rejects missing pipx availability check",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					"          command -v pipx >/dev/null\n",
					"",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must verify pipx is available",
		},
		{
			// Recurrence guard for the macos-15 assurance failure: reuse
			// depends unconditionally on python-magic, a ctypes binding to
			// the native libmagic library pip does not ship, and aborts with
			// NoEncodingModuleError at import time when no encoding module
			// loads. macOS runners have no libmagic, so a bare reuse==6.2.0
			// made every reuse invocation exit 1 and the assurance job never
			// once passed. Dropping the extra must fail verification.
			name: "rejects a REUSE install without an encoding-module extra",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					`pipx install 'reuse[charset-normalizer]==6.2.0'`,
					"pipx install reuse==6.2.0",
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must install reuse[charset-normalizer]==6.2.0",
		},
		{
			name: "rejects the Python user-base path for pipx binaries",
			mutate: func(files map[string][]byte, _ *[]string) {
				files[".github/workflows/ci.yml"] = []byte(strings.Replace(
					string(files[".github/workflows/ci.yml"]),
					`pipx environment --value PIPX_BIN_DIR >> "$GITHUB_PATH"`,
					`echo "$(python3 -m site --user-base)/bin" >> "$GITHUB_PATH"`,
					1,
				))
			},
			wantErrText: ".github/workflows/ci.yml: jobs.assurance must add the pinned REUSE binary path",
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
	writeTestFile(t, root, "mise.lock", "lockfile_version = 3\n")

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
	if !containsPath(paths, "mise.lock") {
		t.Fatalf("trackedConfigPaths() = %v, want auxiliary lock candidate", paths)
	}
	if containsPath(paths, ".gitlab-ci.yml") {
		t.Fatalf("trackedConfigPaths() = %v, did not want working-tree deletion", paths)
	}
}

func validConfigFixture() (map[string][]byte, []string) {
	files := map[string][]byte{
		".github/actionlint.yaml":    []byte("self-hosted-runner: {labels: [ubuntu-26.04]}\n"),
		"Taskfile.yml":               []byte("version: '3'\ntasks: {build: {cmds: ['bash scripts/wails-build.sh']}}\n"),
		"build/Taskfile.yml":         []byte("version: '3'\ntasks: {frontend: {cmds: ['npm ci']}}\n"),
		"build/darwin/Taskfile.yml":  []byte("version: '3'\ntasks: {compile: {cmds: ['go build']}}\n"),
		"build/linux/Taskfile.yml":   []byte("version: '3'\ntasks: {compile: {cmds: ['go build']}}\n"),
		"build/windows/Taskfile.yml": []byte("version: '3'\ntasks: {compile: {cmds: ['go build']}}\n"),
		"build/config.yml":           []byte("version: '3'\ninfo: {version: '1.1.0', productName: GoPMgr, productIdentifier: dev.gopmgr.GoPMgr}\n"),
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
  pull_request:
    branches: [main, master]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps: []
  assurance:
    name: Assurance checks
    runs-on: macos-26
    steps:
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Install pinned REUSE
        run: |
          command -v pipx >/dev/null
          pipx install 'reuse[charset-normalizer]==6.2.0'
          pipx environment --value PIPX_BIN_DIR >> "$GITHUB_PATH"
      - uses: actions/setup-node@v7
      - name: Install frontend dependencies
        working-directory: frontend
        run: npm ci
      - name: Build frontend
        working-directory: frontend
        run: npm run build
      - name: REUSE check
        run: make license-check
      - name: Coverage ledger drift
        run: make coverage-ledger-drift
  website:
    name: Website checks (source pins + output rules)
    runs-on: ubuntu-26.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
          fetch-depth: 0
      - run: make website-tests
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
        with:
          go-version-file: go.mod
          cache: false
      - run: make website-ci
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
		".github/workflows/adr010-g0-spike.yml": []byte(`
name: ADR-010 G0 spike
on:
  pull_request: {}
jobs:
  evidence:
    runs-on: ubuntu-latest
    steps: []
`),
		".golangci.yml": []byte(`
version: "2"
linters: {}
formatters: {}
`),
		"build/linux/nfpm.yaml": []byte(`
name: gopmgr
arch: amd64
platform: linux
version: ${VERSION}
contents:
  - src: ./build/bin/gopmgr
    dst: /usr/bin/gopmgr
overrides:
  deb: {}
  rpm: {}
`),
		".gitleaks.toml": []byte(`
[extend]
useDefault = true

[allowlist]
paths = ["testdata/public-fixture/.*"]
`),
		"REUSE.toml": []byte(`
version = 1

[[annotations]]
path = "frontend/package.json"
precedence = "aggregate"
SPDX-FileCopyrightText = "The GoPMgr Contributors"
SPDX-License-Identifier = "GPL-3.0-or-later"
`),
	}
	files["website/hugo.yaml"], _ = os.ReadFile("../../website/hugo.yaml")
	files["website/data/release-records.yaml"], _ = os.ReadFile("../../website/data/release-records.yaml")
	files["mise.toml"] = []byte("[tools]\n")
	files["mise.lock"] = []byte("lockfile_version = 3\n")
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

func TestWebsiteConsumerContracts(t *testing.T) {
	for _, tc := range []struct{ name, path, before, after, want string }{
		{"prefix", "website/hugo.yaml", "/GoPMgr/", "/", "baseURL must retain the GitHub Pages project prefix"},
		{"remote search", "website/hugo.yaml", "js/vendor/flexsearch.bundle.min.js", "https://example.com/search.js", "search must use vendored local FlexSearch"},
		{"search off", "website/hugo.yaml", "    enable: true", "    enable: false", "search must use vendored local FlexSearch"},
		{"unsafe Markdown", "website/hugo.yaml", "unsafe: false", "unsafe: true", "unsafe Markdown must remain disabled"},
		{"module", "website/hugo.yaml", "github.com/imfing/hextra", "example.com/other", "unexpected website module"},
		{"second module", "website/hugo.yaml", "    - path: github.com/imfing/hextra", "    - path: example.com/other\n    - path: github.com/imfing/hextra", "import exactly the pinned Hextra module"},
		{"older Hugo", "website/hugo.yaml", `min: "0.169.0"`, `min: "0.100.0"`, "requires Hugo Extended 0.169.0"},
		{"standard edition", "website/hugo.yaml", "extended: true", "extended: false", "requires Hugo Extended 0.169.0"},
		{"template network", "website/hugo.yaml", "urls: []", "urls: ['.*']", "inline shortcodes and template HTTP must remain disabled"},
		{"inline shortcodes", "website/hugo.yaml", "enableInlineShortcodes: false", "enableInlineShortcodes: true", "inline shortcodes and template HTTP must remain disabled"},
		{"build runs git", "website/hugo.yaml", "allow: ['^go$']", "allow: ['^go$', '^git$']", "build may run only go"},
		{"build runs anything", "website/hugo.yaml", "allow: ['^go$']", "allow: ['.*']", "build may run only go"},
		{"release status", "website/data/release-records.yaml", "prerelease: true", "prerelease: false", "release record differs from the reviewed historical release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, tracked := validConfigFixture()
			original := string(files[tc.path])
			if strings.Count(original, tc.before) != 1 {
				t.Fatalf("fixture mutation must match once, matched %d times", strings.Count(original, tc.before))
			}
			files[tc.path] = []byte(strings.Replace(original, tc.before, tc.after, 1))
			// Name the rule: a parse error or another rule failing would
			// otherwise pass for the one under test.
			errs := validateRepositoryConfigs(files, tracked)
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tc.want) {
				t.Fatalf("got %v, want one error containing %q", errs, tc.want)
			}
		})
	}
}

// Mutate parsed workflow fields so comments or lookalike command text cannot
// satisfy the execution contract. Fixtures never execute hosted workflows.
func TestCIWebsiteExecutionContract(t *testing.T) {
	type workflowCase struct {
		name   string
		mutate func(map[string]any, map[string]any, []any)
		want   string
	}
	tests := []workflowCase{
		{"valid", nil, ""},
		{"workflow root directory", func(c, j map[string]any, s []any) {
			c["defaults"] = map[string]any{"run": map[string]any{"working-directory": "."}}
		}, ""},
		{"job root directory", func(c, j map[string]any, s []any) {
			j["defaults"] = map[string]any{"run": map[string]any{"working-directory": "."}}
		}, ""},
		{"job inherited shell", func(c, j map[string]any, s []any) {
			j["defaults"] = map[string]any{"run": map[string]any{"shell": "echo {0}"}}
		}, "defaults.run.shell"},
		{"missing job", func(c, j map[string]any, s []any) { delete(c["jobs"].(map[string]any), "website") }, "jobs.website"},
		{"missing PR trigger", func(c, j map[string]any, s []any) { delete(c["on"].(map[string]any), "pull_request") }, "pull_request"},
		{"PR path filter", func(c, j map[string]any, s []any) {
			c["on"].(map[string]any)["pull_request"].(map[string]any)["paths"] = []any{"website/**"}
		}, "pull_request"},
		{"PR type filter", func(c, j map[string]any, s []any) {
			c["on"].(map[string]any)["pull_request"].(map[string]any)["types"] = []any{"opened"}
		}, "pull_request"},
		{"PR branch filter", func(c, j map[string]any, s []any) {
			c["on"].(map[string]any)["pull_request"].(map[string]any)["branches"] = []any{"main"}
		}, "pull_request"},
		{"renamed check", func(c, j map[string]any, s []any) { j["name"] = "Different" }, "name"},
		{"wrong runner", func(c, j map[string]any, s []any) { j["runs-on"] = "ubuntu-latest" }, "runs-on"},
		{"job condition", func(c, j map[string]any, s []any) { j["if"] = "${{ false }}" }, "must not define if"},
		{"skip dependency", func(c, j map[string]any, s []any) { j["needs"] = "verify" }, "must not define needs"},
		{"tolerated job failure", func(c, j map[string]any, s []any) { j["continue-on-error"] = true }, "continue-on-error"},
		{"extra step", func(c, j map[string]any, s []any) { j["steps"] = append(s, map[string]any{"run": "echo extra"}) }, "four ordered steps"},
		{"reordered steps", func(c, j map[string]any, s []any) { s[1], s[3] = s[3], s[1] }, "make website-tests"},
		{"comment-only build", func(c, j map[string]any, s []any) { s[3].(map[string]any)["run"] = "echo skipped # make website-ci" }, "make website-ci"},
		{"masked build failure", func(c, j map[string]any, s []any) { s[3].(map[string]any)["run"] = "make website-ci || true" }, "make website-ci"},
		{"shallow checkout", func(c, j map[string]any, s []any) { s[0].(map[string]any)["with"].(map[string]any)["fetch-depth"] = 1 }, "checkout inputs"},
		{"another source", func(c, j map[string]any, s []any) { s[0].(map[string]any)["with"].(map[string]any)["REF"] = "main" }, "checkout inputs"},
		{"conflicting depth", func(c, j map[string]any, s []any) { s[0].(map[string]any)["with"].(map[string]any)["FETCH-DEPTH"] = 1 }, "checkout inputs"},
		{"setup version override", func(c, j map[string]any, s []any) {
			s[2].(map[string]any)["with"].(map[string]any)["GO-VERSION"] = "1.0"
		}, "setup-go inputs"},
		{"setup cache override", func(c, j map[string]any, s []any) { s[2].(map[string]any)["with"].(map[string]any)["CACHE"] = true }, "setup-go inputs"},
		{"unpinned action", func(c, j map[string]any, s []any) { s[0].(map[string]any)["uses"] = "actions/checkout@v7" }, "pinned actions/checkout"},
	}
	for _, key := range []string{"repository", "ref", "path", "sparse-checkout"} {
		tests = append(tests, workflowCase{
			"checkout override " + key, func(c, j map[string]any, s []any) { s[0].(map[string]any)["with"].(map[string]any)[key] = "override" }, "checkout inputs"})
	}
	for _, scope := range []string{"workflow", "job", "step"} {
		for _, key := range []string{"MAKEFLAGS", "GNUMAKEFLAGS", "MAKEFILES"} {
			tests = append(tests, workflowCase{
				scope + " " + key, func(c, j map[string]any, s []any) {
					target := c
					if scope == "job" {
						target = j
					}
					if scope == "step" {
						target = s[3].(map[string]any)
					}
					target["env"] = map[string]any{key: "-n"}
				}, "Make execution override"})
		}
		if scope != "step" {
			tests = append(tests, workflowCase{
				scope + " working directory", func(c, j map[string]any, s []any) {
					target := c
					if scope == "job" {
						target = j
					}
					target["defaults"] = map[string]any{"run": map[string]any{"working-directory": "website"}}
				}, "working-directory"})
		}
	}
	for i := range 4 {
		for _, key := range []string{"if", "continue-on-error", "shell", "working-directory"} {
			tests = append(tests, workflowCase{
				fmt.Sprintf("step %d %s", i, key), func(c, j map[string]any, s []any) {
					value := any("override")
					if key == "continue-on-error" {
						value = true
					}
					s[i].(map[string]any)[key] = value
				}, "jobs.website.steps"})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, _ := validConfigFixture()
			config, err := parseConfig(configSpec{path: ".github/workflows/ci.yml", format: formatYAML}, files[".github/workflows/ci.yml"])
			if err != nil {
				t.Fatal(err)
			}
			job := config["jobs"].(map[string]any)["website"].(map[string]any)
			if tt.mutate != nil {
				tt.mutate(config, job, job["steps"].([]any))
			}
			err = validateCIAssuranceWorkflow(config)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}
