// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replaceIn returns a fixture mutation that edits one file. A replacement
// that no longer matches leaves the fixture valid, which fails the case.
func replaceIn(path, old, replacement string) func(map[string][]byte, *[]string) {
	return func(files map[string][]byte, _ *[]string) {
		files[path] = []byte(strings.Replace(string(files[path]), old, replacement, 1))
	}
}

func setFile(path, content string) func(map[string][]byte, *[]string) {
	return func(files map[string][]byte, _ *[]string) {
		files[path] = []byte(content)
	}
}

// Each case goes through validateRepositoryConfigs, so it also proves the
// file is routed to the validator that owns the rule: every expected error
// starts with the file's path.
func TestValidateRepositoryConfigsEnforcesEachPolicyRule(t *testing.T) {
	const ci = ".github/workflows/ci.yml"
	const release = ".github/workflows/release.yml"
	tests := []struct {
		name        string
		mutate      func(map[string][]byte, *[]string)
		wantErrText string // empty: the mutation must be accepted
	}{
		// Parsing and inventory.
		{"rejects a YAML sequence root", setFile("Taskfile.yml", "- build\n"), "Taskfile.yml: decode YAML mapping"},
		{"rejects a null YAML document", setFile("Taskfile.yml", "~\n"), "Taskfile.yml: configuration root must be a mapping"},
		{"rejects a malformed second YAML document", setFile("Taskfile.yml", "version: '3'\n---\n[\n"), "Taskfile.yml: parse YAML"},
		{"rejects a non-scalar mapping key", setFile("Taskfile.yml", "? [a]\n: 1\n"), "Taskfile.yml: mapping keys must be scalar values"},
		{
			name: "rejects a tracked configuration that could not be read",
			mutate: func(files map[string][]byte, _ *[]string) {
				delete(files, ".gitleaks.toml")
			},
			wantErrText: ".gitleaks.toml: tracked configuration could not be read",
		},

		// Wails build metadata.
		{"rejects a Wails build version other than 3", replaceIn("build/config.yml", "version: '3'", "version: '2'"), "build/config.yml: expected Wails build version 3"},
		{"rejects non-mapping Wails info", setFile("build/config.yml", "version: '3'\ninfo: GoPMgr\n"), "build/config.yml: info must be a mapping"},

		// Dependabot.
		{"rejects Dependabot version 1", replaceIn(".github/dependabot.yml", "version: 2", "version: 1"), ".github/dependabot.yml: expected version 2"},
		{"rejects a quoted Dependabot version", replaceIn(".github/dependabot.yml", "version: 2", `version: "2"`), ".github/dependabot.yml: expected version 2"},
		{"rejects Dependabot without ecosystems", setFile(".github/dependabot.yml", "version: 2\nupdates: []\n"), ".github/dependabot.yml: updates must contain at least one ecosystem"},

		// Every workflow.
		{"rejects an unnamed workflow", replaceIn(release, "name: Release", "name: ' '"), release + ": name must be a non-empty string"},
		{"rejects a workflow without a trigger", replaceIn(release, "on:\n  push:\n    tags: [\"v*\"]\n", ""), release + ": on trigger is required"},
		{"rejects a workflow without jobs", setFile(release, "name: Release\non: push\njobs: {}\n"), release + ": jobs must contain at least one job"},

		// The CI assurance job.
		{"rejects CI without an assurance job", replaceIn(ci, "  assurance:\n", "  assured:\n"), ci + ": jobs.assurance is required"},
		{"rejects a non-mapping assurance job", setFile(ci, "name: CI\non: push\njobs:\n  verify: {}\n  assurance: macos-26\n"), ci + ": jobs.assurance must be a mapping"},
		{"rejects a conditional assurance job", replaceIn(ci, "    name: Assurance checks\n", "    name: Assurance checks\n    if: false\n"), ci + ": jobs.assurance must not define if"},
		{"accepts an explicit continue-on-error false", replaceIn(ci, "    name: Assurance checks\n", "    name: Assurance checks\n    continue-on-error: false\n"), ""},
		{"rejects an assurance job without steps", setFile(ci, "name: CI\non: push\njobs:\n  assurance:\n    runs-on: macos-26\n    steps: []\n"), ci + ": jobs.assurance.steps must contain at least one step"},
		{"rejects a non-mapping assurance step", replaceIn(ci, "    steps:\n", "    steps:\n      - make license-check\n"), ci + ": jobs.assurance.steps[0] must be a mapping"},
		{"rejects a continue-on-error assurance step", replaceIn(ci, "        run: make license-check\n", "        run: make license-check\n        continue-on-error: true\n"), ci + ": jobs.assurance steps continue-on-error must be absent or false"},
		{"rejects a license gate run from a subdirectory", replaceIn(ci, "        run: make license-check\n", "        working-directory: frontend\n        run: make license-check\n"), ci + ": jobs.assurance must have an exact make license-check step"},
		{"accepts workflow defaults without a run section", replaceIn(ci, "name: CI\non:\n", "name: CI\ndefaults: {}\non:\n"), ""},
		{"accepts workflow run defaults without a shell", replaceIn(ci, "name: CI\non:\n", "name: CI\ndefaults:\n  run:\n    working-directory: .\non:\n"), ""},
		{"rejects a frontend install that is not npm ci", replaceIn(ci, "run: npm ci", "run: npm install"), ci + ": jobs.assurance must have an exact frontend npm ci step"},
		{"rejects a frontend build outside frontend/", replaceIn(ci, "        working-directory: frontend\n        run: npm run build\n", "        run: npm run build\n"), ci + ": jobs.assurance must have an exact frontend npm run build step"},
		{"rejects a REUSE install that does not reach PATH", replaceIn(ci, "          pipx environment --value PIPX_BIN_DIR >> \"$GITHUB_PATH\"\n", ""), ci + ": jobs.assurance must add the pinned REUSE binary path"},
		{"rejects an assurance job without Go", replaceIn(ci, "actions/setup-go@v7", "example/setup-go@v7"), ci + ": jobs.assurance must set up Go"},
		{"rejects an assurance job without Node", replaceIn(ci, "actions/setup-node@v7", "example/setup-node@v7"), ci + ": jobs.assurance must set up Node"},

		// golangci-lint.
		{"rejects non-mapping golangci linters", setFile(".golangci.yml", "version: \"2\"\nlinters: []\nformatters: {}\n"), ".golangci.yml: linters must be a mapping"},
		{"rejects golangci without formatters", setFile(".golangci.yml", "version: \"2\"\nlinters: {}\n"), ".golangci.yml: formatters must be a mapping"},

		// Linux packages.
		{"rejects an unnamed package", replaceIn("build/linux/nfpm.yaml", "name: gopmgr", "name: ''"), "build/linux/nfpm.yaml: name must be a non-empty string"},
		{"rejects a package without a version", replaceIn("build/linux/nfpm.yaml", "version: ${VERSION}\n", ""), "build/linux/nfpm.yaml: version must be a non-empty string"},
		{"rejects a package without contents", replaceIn("build/linux/nfpm.yaml", "contents:\n  - src: ./build/bin/gopmgr\n    dst: /usr/bin/gopmgr\n", "contents: []\n"), "build/linux/nfpm.yaml: contents must contain at least one package entry"},
		{"rejects a package without overrides", replaceIn("build/linux/nfpm.yaml", "overrides:\n  deb: {}\n  rpm: {}\n", ""), "build/linux/nfpm.yaml: overrides must be a mapping"},
		{"rejects a package without RPM overrides", replaceIn("build/linux/nfpm.yaml", "  rpm: {}\n", ""), "build/linux/nfpm.yaml: overrides.rpm is required"},

		// Secret scanning must keep gitleaks' default rules.
		{"rejects gitleaks without extend", setFile(".gitleaks.toml", "[allowlist]\npaths = []\n"), ".gitleaks.toml: extend must be a mapping"},
		{"rejects gitleaks default rules turned off", replaceIn(".gitleaks.toml", "useDefault = true", "useDefault = false"), ".gitleaks.toml: extend.useDefault must be true"},
		{"rejects a quoted gitleaks useDefault", replaceIn(".gitleaks.toml", "useDefault = true", `useDefault = "true"`), ".gitleaks.toml: extend.useDefault must be true"},

		// REUSE.
		{"rejects REUSE version 2", replaceIn("REUSE.toml", "version = 1", "version = 2"), "REUSE.toml: expected version 1"},
		{"rejects REUSE without annotations", setFile("REUSE.toml", "version = 1\n"), "REUSE.toml: annotations must contain at least one entry"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, tracked := validConfigFixture()
			tt.mutate(files, &tracked)
			errs := validateRepositoryConfigs(files, tracked)
			if tt.wantErrText == "" {
				if len(errs) != 0 {
					t.Fatalf("validateRepositoryConfigs() returned unexpected errors: %v", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), tt.wantErrText) {
				t.Fatalf("validateRepositoryConfigs() errors = %v, want exactly one containing %q", errs, tt.wantErrText)
			}
		})
	}
}

func TestParseConfigRejectsAnUnknownFormat(t *testing.T) {
	_, err := parseConfig(configSpec{path: "x.ini", format: "INI"}, []byte("a = 1\n"))
	if err == nil || !strings.Contains(err.Error(), `x.ini: unsupported config format "INI"`) {
		t.Fatalf("parseConfig() error = %v, want unsupported format", err)
	}
}

func TestIntegerValueAcceptsOnlyRepresentableIntegers(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   int64
		wantOK bool
	}{
		{"int", 2, 2, true},
		{"int64", int64(-3), -3, true},
		{"uint64 at the int64 limit", uint64(math.MaxInt64), math.MaxInt64, true},
		{"uint64 above the int64 limit", uint64(math.MaxInt64) + 1, 0, false},
		{"float", 2.0, 0, false},
		{"string", "2", 0, false},
		{"missing", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := integerValue(tt.value)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("integerValue(%#v) = %d, %v; want %d, %v", tt.value, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// writeFixtureRepository commits the valid fixture to a new git repository.
func writeFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files, _ := validConfigFixture()
	runGit(t, root, "init", "-q")
	for path, data := range files {
		writeTestFile(t, root, path, string(data))
	}
	runGit(t, root, "add", ".")
	return root
}

func TestRunReportsSuccessAndEveryFailure(t *testing.T) {
	root := writeFixtureRepository(t)
	var stdout, stderr bytes.Buffer
	if status := run(root, &stdout, &stderr); status != 0 {
		t.Fatalf("run() = %d, stderr %q; want 0", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tracked YAML/TOML configurations validated") || stderr.Len() != 0 {
		t.Fatalf("run() stdout %q, stderr %q; want only the success summary", stdout.String(), stderr.String())
	}

	writeTestFile(t, root, ".gitleaks.toml", "[extend]\nuseDefault = false\n")
	writeTestFile(t, root, "extra.yml", "a: 1\n")
	stdout.Reset()
	stderr.Reset()
	if status := run(root, &stdout, &stderr); status != 1 {
		t.Fatalf("run() = %d; want 1", status)
	}
	for _, want := range []string{
		"config-check: .gitleaks.toml: extend.useDefault must be true",
		"config-check: extra.yml: tracked YAML/TOML file is not classified",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("run() stderr %q; want %q", stderr.String(), want)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("run() stdout %q on failure; want none", stdout.String())
	}
}

func TestRunFailsWhenItCannotReadTheInventory(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(t.TempDir(), &stdout, &stderr); status != 1 {
		t.Fatalf("run() outside a repository = %d; want 1", status)
	}
	if !strings.Contains(stderr.String(), "list tracked YAML/TOML files") {
		t.Fatalf("run() stderr %q; want the inventory error", stderr.String())
	}
}

func TestRunFailsWhenATrackedConfigurationIsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode-000 file")
	}
	root := writeFixtureRepository(t)
	path := filepath.Join(root, ".gitleaks.toml")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	var stdout, stderr bytes.Buffer
	if status := run(root, &stdout, &stderr); status != 1 {
		t.Fatalf("run() = %d; want 1", status)
	}
	if !strings.Contains(stderr.String(), "config-check: read .gitleaks.toml:") {
		t.Fatalf("run() stderr %q; want the read error", stderr.String())
	}
}

func TestRepositoryRootFindsTheEnclosingRepository(t *testing.T) {
	root := writeFixtureRepository(t)
	nested := filepath.Join(root, "build", "linux")
	t.Chdir(nested)
	got, err := repositoryRoot()
	if err != nil {
		t.Fatalf("repositoryRoot() error = %v", err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err = filepath.EvalSymlinks(got); err != nil || got != want {
		t.Fatalf("repositoryRoot() = %q, %v; want %q", got, err, want)
	}

	t.Chdir(t.TempDir())
	if _, err := repositoryRoot(); err == nil {
		t.Fatal("repositoryRoot() outside a repository succeeded")
	}
}

func TestRunReportsEveryMissingConfigurationInAnEmptyRepository(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	var stdout, stderr bytes.Buffer
	if status := run(root, &stdout, &stderr); status != 1 {
		t.Fatalf("run() = %d; want 1", status)
	}
	for _, spec := range supportedConfigs {
		if want := "config-check: " + spec.path + ": required configuration is not tracked"; !strings.Contains(stderr.String(), want) {
			t.Fatalf("run() stderr %q; want %q", stderr.String(), want)
		}
	}
}

func TestRunFailsWhenAConfigurationCannotBeInspected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can search a mode-000 directory")
	}
	root := writeFixtureRepository(t)
	dir := filepath.Join(root, "build", "linux")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	var stdout, stderr bytes.Buffer
	if status := run(root, &stdout, &stderr); status != 1 {
		t.Fatalf("run() = %d; want 1", status)
	}
	if !strings.Contains(stderr.String(), "inspect config candidate build/linux/") {
		t.Fatalf("run() stderr %q; want the inspection error", stderr.String())
	}
}
