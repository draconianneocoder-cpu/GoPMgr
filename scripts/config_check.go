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
	validate func(map[string]any) error
}

var supportedConfigs = []configSpec{
	{path: ".github/dependabot.yml", format: formatYAML, validate: validateDependabot},
	{path: ".github/workflows/ci.yml", format: formatYAML, validate: validateWorkflow},
	{path: ".github/workflows/release.yml", format: formatYAML, validate: validateWorkflow},
	{path: ".golangci.yml", format: formatYAML, validate: validateGolangCI},
	{path: "build/linux/nfpm.yaml", format: formatYAML, validate: validateNFPM},
	{path: ".gitleaks.toml", format: formatTOML, validate: validateGitleaks},
	{path: "REUSE.toml", format: formatTOML, validate: validateREUSE},
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-check: %v\n", err)
		os.Exit(1)
	}

	tracked, err := trackedConfigPaths(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-check: %v\n", err)
		os.Exit(1)
	}

	files := make(map[string][]byte, len(tracked))
	for _, path := range tracked {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "config-check: read %s: %v\n", path, readErr)
			os.Exit(1)
		}
		files[path] = data
	}

	errs := validateRepositoryConfigs(files, tracked)
	if len(errs) != 0 {
		for _, checkErr := range errs {
			fmt.Fprintf(os.Stderr, "config-check: %v\n", checkErr)
		}
		os.Exit(1)
	}

	fmt.Printf("config-check: %d tracked YAML/TOML configurations validated.\n", len(supportedConfigs))
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
	cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.yml", "*.yaml", "*.toml")
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
			errs = append(errs, fmt.Errorf("%s: tracked YAML/TOML file is not classified in scripts/config_check.go", path))
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
		if validateErr := spec.validate(config); validateErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", spec.path, validateErr))
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
	if jobs, ok := stringMap(config["jobs"]); !ok || len(jobs) == 0 {
		return fmt.Errorf("jobs must contain at least one job")
	}
	return nil
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
