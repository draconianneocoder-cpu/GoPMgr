// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// config_check validates GoPMgr's tracked YAML/TOML tool configuration.
//
// Hosted services such as GitHub Actions and Dependabot require YAML, while
// Gitleaks and REUSE use TOML. Keeping the supported inventory here prevents a
// well-intentioned format conversion from silently disabling release tooling.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type configFormat string

const (
	formatYAML configFormat = "YAML"
	formatTOML configFormat = "TOML"
)

type configSpec struct {
	path     string
	format   configFormat
	validate func(map[string]any) error // Optional consumer-specific checks after parsing.
}

var supportedConfigs = []configSpec{
	{path: "website/hugo.yaml", format: formatYAML, validate: validateWebsiteHugo},
	{path: "website/data/release-records.yaml", format: formatYAML, validate: validateWebsiteRelease},
	// Python provisioning checks acquisition policy; this registry owns format inventory.
	{path: "mise.toml", format: formatTOML},
	{path: "mise.lock", format: formatTOML},
	{path: "Taskfile.yml", format: formatYAML, validate: validateTaskfile},
	{path: "build/Taskfile.yml", format: formatYAML, validate: validateTaskfile},
	{path: "build/darwin/Taskfile.yml", format: formatYAML, validate: validateTaskfile},
	{path: "build/linux/Taskfile.yml", format: formatYAML, validate: validateTaskfile},
	{path: "build/windows/Taskfile.yml", format: formatYAML, validate: validateTaskfile},
	{path: "build/config.yml", format: formatYAML, validate: validateWailsBuild},
	{path: ".github/dependabot.yml", format: formatYAML, validate: validateDependabot},
	{path: ".github/actionlint.yaml", format: formatYAML},
	{path: ".github/workflows/ci.yml", format: formatYAML, validate: validateCIAssuranceWorkflow},
	{path: ".github/workflows/release.yml", format: formatYAML, validate: validateWorkflow},
	{path: ".github/workflows/adr010-g0-spike.yml", format: formatYAML, validate: validateWorkflow},
	{path: ".golangci.yml", format: formatYAML, validate: validateGolangCI},
	{path: "build/linux/nfpm.yaml", format: formatYAML, validate: validateNFPM},
	{path: ".gitleaks.toml", format: formatTOML, validate: validateGitleaks},
	{path: "REUSE.toml", format: formatTOML, validate: validateREUSE},
}

func validateTaskfile(config map[string]any) error {
	if config["version"] != "3" {
		return fmt.Errorf("expected Taskfile version 3")
	}
	if tasks, ok := stringMap(config["tasks"]); !ok || len(tasks) == 0 {
		return fmt.Errorf("tasks must be a nonempty mapping")
	}
	return nil
}

func validateWailsBuild(config map[string]any) error {
	if config["version"] != "3" {
		return fmt.Errorf("expected Wails build version 3")
	}
	info, ok := stringMap(config["info"])
	if !ok {
		return fmt.Errorf("info must be a mapping")
	}
	for _, name := range []string{"version", "productName", "productIdentifier"} {
		if value, ok := info[name].(string); !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("info.%s must be a nonempty string", name)
		}
	}
	return nil
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-check: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(root, os.Stdout, os.Stderr))
}

// run validates every YAML/TOML configuration under root and returns the
// process exit status. It reports every validation error, not just the first.
func run(root string, stdout, stderr io.Writer) int {
	tracked, err := trackedConfigPaths(root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "config-check: %v\n", err)
		return 1
	}

	files := make(map[string][]byte, len(tracked))
	for _, path := range tracked {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			_, _ = fmt.Fprintf(stderr, "config-check: read %s: %v\n", path, readErr)
			return 1
		}
		files[path] = data
	}

	errs := validateRepositoryConfigs(files, tracked)
	if len(errs) != 0 {
		for _, checkErr := range errs {
			_, _ = fmt.Fprintf(stderr, "config-check: %v\n", checkErr)
		}
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "config-check: %d tracked YAML/TOML configurations validated.\n", len(supportedConfigs))
	return 0
}

func repositoryRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("locate repository root with git: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func trackedConfigPaths(root string) ([]string, error) {
	// Git is the inventory authority. Walking the filesystem would incorrectly
	// classify ignored build output and private .agent_memory handoff files.
	// Include untracked, non-ignored candidates so a new config is classified
	// before its first commit, and omit working-tree deletions so intentional
	// removals can pass verification before they are staged.
	cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.yml", "*.yaml", "*.toml", "mise.lock")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked YAML/TOML files: %w", err)
	}

	var paths []string
	for _, path := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if path == "" {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(path))); statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, fmt.Errorf("inspect config candidate %s: %w", path, statErr)
		}
		paths = append(paths, filepath.ToSlash(path))
	}
	sort.Strings(paths)
	return paths, nil
}

func validateRepositoryConfigs(files map[string][]byte, tracked []string) []error {
	specByPath := make(map[string]configSpec, len(supportedConfigs))
	for _, spec := range supportedConfigs {
		specByPath[spec.path] = spec
	}

	trackedSet := make(map[string]struct{}, len(tracked))
	var errs []error
	for _, path := range tracked {
		trackedSet[path] = struct{}{}
		if path == ".gitlab-ci.yml" {
			errs = append(errs, fmt.Errorf("%s: legacy GitLab CI configuration is not supported; GitHub Actions is the CI authority", path))
			continue
		}
		if _, ok := specByPath[path]; !ok {
			errs = append(errs, fmt.Errorf("%s: tracked YAML/TOML file is not classified in tools/config-check/config_check.go", path))
		}
	}

	for _, spec := range supportedConfigs {
		if _, ok := trackedSet[spec.path]; !ok {
			errs = append(errs, fmt.Errorf("%s: required configuration is not tracked", spec.path))
			continue
		}

		data, ok := files[spec.path]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: tracked configuration could not be read", spec.path))
			continue
		}

		config, parseErr := parseConfig(spec, data)
		if parseErr != nil {
			errs = append(errs, parseErr)
			continue
		}
		if spec.validate != nil {
			if validateErr := spec.validate(config); validateErr != nil {
				errs = append(errs, fmt.Errorf("%s: %w", spec.path, validateErr))
			}
		}
	}
	return errs
}

func parseConfig(spec configSpec, data []byte) (map[string]any, error) {
	var config map[string]any
	switch spec.format {
	case formatYAML:
		var document yaml.Node
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&document); err != nil {
			return nil, fmt.Errorf("%s: parse YAML: %w", spec.path, err)
		}
		var trailingDocument yaml.Node
		switch err := decoder.Decode(&trailingDocument); {
		case errors.Is(err, io.EOF):
		case err != nil:
			return nil, fmt.Errorf("%s: parse YAML: %w", spec.path, err)
		default:
			// Every supported consumer expects one configuration mapping. A
			// second document can be ignored by one parser and honored by
			// another, creating an unsafe review/runtime interpretation split.
			return nil, fmt.Errorf("%s: multiple YAML documents are not supported", spec.path)
		}
		if err := rejectDuplicateYAMLKeys(&document); err != nil {
			return nil, fmt.Errorf("%s: %w", spec.path, err)
		}
		if err := document.Decode(&config); err != nil {
			return nil, fmt.Errorf("%s: decode YAML mapping: %w", spec.path, err)
		}
	case formatTOML:
		if err := toml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("%s: parse TOML: %w", spec.path, err)
		}
	default:
		return nil, fmt.Errorf("%s: unsupported config format %q", spec.path, spec.format)
	}
	if config == nil {
		return nil, fmt.Errorf("%s: configuration root must be a mapping", spec.path)
	}
	return config, nil
}

func rejectDuplicateYAMLKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode {
				return fmt.Errorf("mapping keys must be scalar values")
			}
			if _, ok := seen[key.Value]; ok {
				return fmt.Errorf("duplicate key %q", key.Value)
			}
			seen[key.Value] = struct{}{}
		}
	}
	for _, child := range node.Content {
		if err := rejectDuplicateYAMLKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func validateDependabot(config map[string]any) error {
	if version, ok := integerValue(config["version"]); !ok || version != 2 {
		return fmt.Errorf("expected version 2")
	}
	if updates, ok := config["updates"].([]any); !ok || len(updates) == 0 {
		return fmt.Errorf("updates must contain at least one ecosystem")
	}
	return nil
}

func validateWorkflow(config map[string]any) error {
	if name, ok := config["name"].(string); !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must be a non-empty string")
	}
	if _, ok := config["on"]; !ok {
		return fmt.Errorf("on trigger is required")
	}
	jobs, ok := stringMap(config["jobs"])
	if !ok || len(jobs) == 0 {
		return fmt.Errorf("jobs must contain at least one job")
	}
	names := make([]string, 0, len(jobs))
	for name := range jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		job, ok := stringMap(jobs[name])
		if !ok {
			return fmt.Errorf("jobs.%s must be a mapping", name)
		}
		rawSteps, present := job["steps"]
		if !present {
			continue
		}
		steps, ok := rawSteps.([]any)
		if !ok {
			return fmt.Errorf("jobs.%s.steps must be a sequence", name)
		}
		for i, rawStep := range steps {
			if err := validateCheckoutStep(rawStep); err != nil {
				return fmt.Errorf("jobs.%s.steps[%d] %w", name, i, err)
			}
		}
	}
	return nil
}

func validateCheckoutStep(rawStep any) error {
	step, ok := stringMap(rawStep)
	if !ok {
		return fmt.Errorf("must be a mapping")
	}
	rawUses, present := step["uses"]
	if !present {
		return nil
	}
	uses, ok := rawUses.(string)
	if !ok || strings.TrimSpace(uses) == "" {
		return fmt.Errorf("uses must be a nonempty string")
	}
	if uses != strings.TrimSpace(uses) {
		return fmt.Errorf("uses must not contain surrounding whitespace")
	}
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, `.\`) || strings.HasPrefix(uses, "docker://") {
		return nil
	}
	repository, ref, hasRef := strings.Cut(uses, "@")
	// Match the runner's owner/repository split, including optional action paths.
	parts := strings.FieldsFunc(repository, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) < 2 || !strings.EqualFold(parts[0], "actions") || !strings.EqualFold(parts[1], "checkout") {
		return nil
	}
	if !hasRef || strings.TrimSpace(ref) == "" || strings.Contains(ref, "@") {
		return fmt.Errorf("checkout reference must include a nonempty ref")
	}
	rawInputs, present := step["with"]
	if !present {
		return fmt.Errorf("checkout persist-credentials must be YAML boolean false")
	}
	inputs, ok := stringMap(rawInputs)
	if !ok {
		return fmt.Errorf("checkout with must be a mapping")
	}
	found := false
	for name, value := range inputs {
		if !strings.EqualFold(name, "persist-credentials") {
			continue
		}
		// The runner treats input names case-insensitively. Require every
		// spelling to be safe so map order cannot hide a conflicting value.
		persist, ok := value.(bool)
		if !ok || persist {
			return fmt.Errorf("checkout persist-credentials must be YAML boolean false")
		}
		found = true
	}
	if !found {
		return fmt.Errorf("checkout persist-credentials must be YAML boolean false")
	}
	return nil
}

func validateCIAssuranceWorkflow(config map[string]any) error {
	if err := validateWorkflow(config); err != nil {
		return err
	}
	if hasDefaultRunShell(config) {
		return fmt.Errorf("workflow defaults.run.shell must not override jobs.assurance")
	}

	jobs, ok := stringMap(config["jobs"])
	if !ok {
		return fmt.Errorf("jobs must be a mapping")
	}
	assuranceRaw, ok := jobs["assurance"]
	if !ok {
		return fmt.Errorf("jobs.assurance is required")
	}
	assurance, ok := stringMap(assuranceRaw)
	if !ok {
		return fmt.Errorf("jobs.assurance must be a mapping")
	}
	if _, ok := assurance["if"]; ok {
		return fmt.Errorf("jobs.assurance must not define if")
	}
	if _, ok := assurance["needs"]; ok {
		return fmt.Errorf("jobs.assurance must not define needs")
	}
	if value, ok := assurance["continue-on-error"]; ok && value != false {
		return fmt.Errorf("jobs.assurance continue-on-error must be absent or false")
	}
	// macOS 26 is the supported minimum; test binaries built for it do not
	// launch on an older runner.
	if runsOn, ok := assurance["runs-on"].(string); !ok || runsOn != "macos-26" {
		return fmt.Errorf("jobs.assurance must run on macos-26")
	}
	if hasDefaultRunShell(assurance) {
		return fmt.Errorf("jobs.assurance defaults.run.shell must not be defined")
	}

	steps, ok := assurance["steps"].([]any)
	if !ok || len(steps) == 0 {
		return fmt.Errorf("jobs.assurance.steps must contain at least one step")
	}
	var stepsWithRuns []map[string]any
	for _, rawStep := range steps {
		step, ok := stringMap(rawStep)
		if !ok {
			return fmt.Errorf("jobs.assurance.steps entries must be mappings")
		}
		if _, ok := step["if"]; ok {
			return fmt.Errorf("jobs.assurance steps must not define if")
		}
		if value, ok := step["continue-on-error"]; ok && value != false {
			return fmt.Errorf("jobs.assurance steps continue-on-error must be absent or false")
		}
		if _, ok := step["shell"]; ok {
			return fmt.Errorf("jobs.assurance steps must use the default shell")
		}
		stepsWithRuns = append(stepsWithRuns, step)
	}

	if !hasExactRun(stepsWithRuns, "make license-check", "") {
		return fmt.Errorf("jobs.assurance must have an exact make license-check step")
	}
	if !hasExactRun(stepsWithRuns, "make coverage-ledger-drift", "") {
		return fmt.Errorf("jobs.assurance must have an exact make coverage-ledger-drift step")
	}
	if !hasExactRun(stepsWithRuns, "npm ci", "frontend") {
		return fmt.Errorf("jobs.assurance must have an exact frontend npm ci step")
	}
	if !hasExactRun(stepsWithRuns, "npm run build", "frontend") {
		return fmt.Errorf("jobs.assurance must have an exact frontend npm run build step")
	}
	// The charset-normalizer extra is load-bearing on this macOS job, not
	// cosmetic: reuse depends unconditionally on python-magic, a ctypes
	// binding to the native libmagic library that pip does not ship, and
	// raises NoEncodingModuleError at import time when no encoding module
	// loads. macOS runners have no libmagic, so a bare `reuse==6.2.0` makes
	// every reuse invocation exit 1. Assert the extra, not just the version.
	if !hasRunLine(stepsWithRuns, `pipx install 'reuse[charset-normalizer]==6.2.0'`) {
		return fmt.Errorf("jobs.assurance must install reuse[charset-normalizer]==6.2.0")
	}
	if !hasRunLine(stepsWithRuns, `command -v pipx >/dev/null`) {
		return fmt.Errorf("jobs.assurance must verify pipx is available")
	}
	if !hasRunLine(stepsWithRuns, `pipx environment --value PIPX_BIN_DIR >> "$GITHUB_PATH"`) {
		return fmt.Errorf("jobs.assurance must add the pinned REUSE binary path")
	}
	if !hasUsesPrefix(stepsWithRuns, "actions/setup-go@") {
		return fmt.Errorf("jobs.assurance must set up Go")
	}
	if !hasUsesPrefix(stepsWithRuns, "actions/setup-node@") {
		return fmt.Errorf("jobs.assurance must set up Node")
	}
	return validateCIWebsiteWorkflow(config)
}

// Preserve the reviewed four-step pipeline, not arbitrary workflow semantics.
// Changes to its commands, inputs or execution context require deliberate review.
func validateCIWebsiteWorkflow(config map[string]any) error {
	on, _ := stringMap(config["on"])
	pr, ok := stringMap(on["pull_request"])
	branches, _ := pr["branches"].([]any)
	if !ok || len(pr) != 1 || len(branches) != 2 ||
		((branches[0] != "main" || branches[1] != "master") &&
			(branches[0] != "master" || branches[1] != "main")) {
		return fmt.Errorf("website pull_request trigger must cover main and master without other filters")
	}
	jobs, _ := stringMap(config["jobs"])
	job, ok := stringMap(jobs["website"])
	if !ok {
		return fmt.Errorf("jobs.website must be a mapping")
	}
	if job["name"] != "Website checks (source pins + output rules)" {
		return fmt.Errorf("jobs.website name must retain the reviewed check identity")
	}
	if job["runs-on"] != "ubuntu-26.04" {
		return fmt.Errorf("jobs.website runs-on must be ubuntu-26.04")
	}
	for _, key := range []string{"if", "needs"} {
		if _, present := job[key]; present {
			return fmt.Errorf("jobs.website must not define %s", key)
		}
	}
	if value, present := job["continue-on-error"]; present && value != false {
		return fmt.Errorf("jobs.website continue-on-error must be absent or false")
	}
	for _, scope := range []map[string]any{config, job} {
		defaults, _ := stringMap(scope["defaults"])
		run, _ := stringMap(defaults["run"])
		for _, key := range []string{"shell", "working-directory"} {
			if value, present := run[key]; present && (key != "working-directory" || value != ".") {
				return fmt.Errorf("website must not inherit defaults.run.%s", key)
			}
		}
		if err := validateWebsiteMakeEnvironment(scope); err != nil {
			return err
		}
	}
	steps, ok := job["steps"].([]any)
	if !ok || len(steps) != 4 {
		return fmt.Errorf("jobs.website must retain four ordered steps: checkout, tests, Go setup, production checks")
	}
	for i, raw := range steps {
		step, ok := stringMap(raw)
		if !ok {
			return fmt.Errorf("jobs.website.steps[%d] must be a mapping", i)
		}
		for _, key := range []string{"if", "shell", "working-directory"} {
			if _, present := step[key]; present {
				return fmt.Errorf("jobs.website.steps[%d] must not define %s", i, key)
			}
		}
		if value, present := step["continue-on-error"]; present && value != false {
			return fmt.Errorf("jobs.website.steps[%d] continue-on-error must be absent or false", i)
		}
		if err := validateWebsiteMakeEnvironment(step); err != nil {
			return err
		}
		switch i {
		case 0:
			if err := validateWebsiteAction(step, "actions/checkout", map[string]any{"persist-credentials": false, "fetch-depth": 0}); err != nil {
				return err
			}
		case 2:
			if err := validateWebsiteAction(step, "actions/setup-go", map[string]any{"go-version-file": "go.mod", "cache": false}); err != nil {
				return err
			}
		default:
			command := "make website-tests"
			if i == 3 {
				command = "make website-ci"
			}
			if _, present := step["uses"]; present || !hasExactRun([]map[string]any{step}, command, "") {
				return fmt.Errorf("jobs.website.steps[%d] must run exactly %s", i, command)
			}
		}
	}
	return nil
}

func validateWebsiteMakeEnvironment(scope map[string]any) error {
	env, _ := stringMap(scope["env"])
	for _, key := range []string{"MAKEFLAGS", "GNUMAKEFLAGS", "MAKEFILES"} {
		if _, present := env[key]; present {
			return fmt.Errorf("website Make execution override %s is forbidden", key)
		}
	}
	return nil
}

func validateWebsiteAction(step map[string]any, action string, expected map[string]any) error {
	uses, _ := step["uses"].(string)
	name, ref, _ := strings.Cut(uses, "@")
	_, hasRun := step["run"]
	if name != action || len(ref) != 40 || strings.Trim(ref, "0123456789abcdef") != "" || hasRun {
		return fmt.Errorf("jobs.website requires pinned %s", action)
	}
	inputs, _ := stringMap(step["with"])
	// Exact lowercase keys reject case-insensitive aliases and conflicting inputs,
	// as well as checkout ref/repository/path/sparse overrides.
	if len(inputs) != len(expected) {
		return fmt.Errorf("jobs.website %s inputs must match the reviewed pipeline", strings.TrimPrefix(action, "actions/"))
	}
	for key, value := range expected {
		if inputs[key] != value {
			return fmt.Errorf("jobs.website %s inputs must match the reviewed pipeline", strings.TrimPrefix(action, "actions/"))
		}
	}
	return nil
}

func hasDefaultRunShell(config map[string]any) bool {
	defaults, ok := stringMap(config["defaults"])
	if !ok {
		return false
	}
	run, ok := stringMap(defaults["run"])
	if !ok {
		return false
	}
	_, ok = run["shell"]
	return ok
}

func hasExactRun(steps []map[string]any, command, workingDirectory string) bool {
	for _, step := range steps {
		run, ok := step["run"].(string)
		if !ok || strings.TrimSpace(run) != command {
			continue
		}
		if workingDirectory == "" {
			if _, ok := step["working-directory"]; !ok {
				return true
			}
			continue
		}
		if directory, ok := step["working-directory"].(string); ok && directory == workingDirectory {
			return true
		}
	}
	return false
}

func hasRunLine(steps []map[string]any, required string) bool {
	for _, step := range steps {
		run, ok := step["run"].(string)
		if !ok {
			continue
		}
		for _, line := range strings.Split(run, "\n") {
			if strings.TrimSpace(line) == required {
				return true
			}
		}
	}
	return false
}

func hasUsesPrefix(steps []map[string]any, prefix string) bool {
	for _, step := range steps {
		uses, ok := step["uses"].(string)
		if ok && strings.HasPrefix(uses, prefix) {
			return true
		}
	}
	return false
}

func validateGolangCI(config map[string]any) error {
	if version, ok := config["version"].(string); !ok || version != "2" {
		return fmt.Errorf(`expected version "2"`)
	}
	if _, ok := stringMap(config["linters"]); !ok {
		return fmt.Errorf("linters must be a mapping")
	}
	if _, ok := stringMap(config["formatters"]); !ok {
		return fmt.Errorf("formatters must be a mapping")
	}
	return nil
}

func validateNFPM(config map[string]any) error {
	if name, ok := config["name"].(string); !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must be a non-empty string")
	}
	if version, ok := config["version"].(string); !ok || strings.TrimSpace(version) == "" {
		return fmt.Errorf("version must be a non-empty string")
	}
	if contents, ok := config["contents"].([]any); !ok || len(contents) == 0 {
		return fmt.Errorf("contents must contain at least one package entry")
	}
	overrides, ok := stringMap(config["overrides"])
	if !ok {
		return fmt.Errorf("overrides must be a mapping")
	}
	for _, packager := range []string{"deb", "rpm"} {
		if _, ok := overrides[packager]; !ok {
			return fmt.Errorf("overrides.%s is required", packager)
		}
	}
	return nil
}

func validateGitleaks(config map[string]any) error {
	extend, ok := stringMap(config["extend"])
	if !ok {
		return fmt.Errorf("extend must be a mapping")
	}
	if useDefault, ok := extend["useDefault"].(bool); !ok || !useDefault {
		return fmt.Errorf("extend.useDefault must be true")
	}
	return nil
}

func validateREUSE(config map[string]any) error {
	if version, ok := integerValue(config["version"]); !ok || version != 1 {
		return fmt.Errorf("expected version 1")
	}
	if annotations, ok := config["annotations"].([]any); !ok || len(annotations) == 0 {
		return fmt.Errorf("annotations must contain at least one entry")
	}
	return nil
}

func stringMap(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func integerValue(value any) (int64, bool) {
	switch number := value.(type) {
	case int:
		return int64(number), true
	case int64:
		return number, true
	case uint64:
		if number > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(number), true
	default:
		return 0, false
	}
}

// Website settings remain opt-in and cannot relax the application workflow checks.
func validateWebsiteHugo(config map[string]any) error {
	if config["baseURL"] != "https://draconianneocoder-cpu.github.io/GoPMgr/" {
		return fmt.Errorf("website baseURL must retain the GitHub Pages project prefix")
	}
	module, _ := stringMap(config["module"])
	imports, ok := module["imports"].([]any)
	if !ok || len(imports) != 1 {
		return fmt.Errorf("website must import exactly the pinned Hextra module")
	}
	item, _ := stringMap(imports[0])
	if item["path"] != "github.com/imfing/hextra" {
		return fmt.Errorf("unexpected website module")
	}
	version, _ := stringMap(module["hugoVersion"])
	if version["min"] != "0.169.0" || version["extended"] != true {
		return fmt.Errorf("website requires Hugo Extended 0.169.0")
	}
	markup, _ := stringMap(config["markup"])
	goldmark, _ := stringMap(markup["goldmark"])
	renderer, _ := stringMap(goldmark["renderer"])
	if renderer["unsafe"] != false {
		return fmt.Errorf("website unsafe Markdown must remain disabled")
	}
	params, _ := stringMap(config["params"])
	search, _ := stringMap(params["search"])
	flex, _ := stringMap(search["flexsearch"])
	if search["enable"] != true || search["type"] != "flexsearch" || flex["js"] != "js/vendor/flexsearch.bundle.min.js" {
		return fmt.Errorf("website search must use vendored local FlexSearch")
	}
	security, _ := stringMap(config["security"])
	http, _ := stringMap(security["http"])
	urls, ok := http["urls"].([]any)
	if security["enableInlineShortcodes"] != false || !ok || len(urls) != 0 {
		return fmt.Errorf("website inline shortcodes and template HTTP must remain disabled")
	}
	// The build may start one program: go, to fetch the pinned theme module.
	exec, _ := stringMap(security["exec"])
	allow, ok := exec["allow"].([]any)
	if !ok || len(allow) != 1 || allow[0] != "^go$" {
		return fmt.Errorf("website build may run only go")
	}
	return nil
}

func validateWebsiteRelease(config map[string]any) error {
	// The initial site describes one historical release. Updating this record
	// requires reviewing the matching evidence and public downloads page together.
	if config["tag"] != "v1.1.0-alpha.2" || config["prerelease"] != true ||
		config["published_at"] != "2026-09-08T22:00:32Z" ||
		config["source_commit"] != "ad5c134a13335f5bfa69e147d7bc7390f5d0a76b" ||
		config["release_url"] != "https://github.com/draconianneocoder-cpu/GoPMgr/releases/tag/v1.1.0-alpha.2" {
		return fmt.Errorf("website release record differs from the reviewed historical release")
	}
	return nil
}
