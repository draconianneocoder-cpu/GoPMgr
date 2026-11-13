// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// recordPath is the reviewed record, relative to the repository root.
const recordPath = "tools/engine-advisories/reviewed.json"

// sbomPath holds the SBOM's reviewed metadata, whose native entries must
// name the same components, versions and CPEs as the record.
const sbomPath = "tools/sbom/components.json"

// engineFiles are the vendored files that make up the compiled engine: the
// amalgamation, its header, the build flags and GoPMgr's random source
// (internal/sqlitedriver/provenance_test.go pins their provenance). The
// review must cover each of them.
var engineFiles = []string{
	"third_party/go-sqlcipher/flags.go",
	"third_party/go-sqlcipher/gopmgr_rng.c",
	"third_party/go-sqlcipher/sqlite3.c",
	"third_party/go-sqlcipher/sqlite3.h",
}

// statusNotAffected is the only disposition. An advisory that affects the
// engine has none: it fails the scan until the engine is upgraded.
const statusNotAffected = "not_affected"

type record struct {
	SchemaVersion int               `json:"schema_version"`
	ReviewedOn    string            `json:"reviewed_on"`
	EngineFiles   map[string]string `json:"engine_files"`
	Components    []component       `json:"components"`
	Dispositions  []disposition     `json:"dispositions"`
}

type component struct {
	Name           string          `json:"name"`
	CPEVendor      string          `json:"cpe_vendor"`
	CPEProduct     string          `json:"cpe_product"`
	Version        string          `json:"version"`
	VersionSources []versionSource `json:"version_sources"`
}

type versionSource struct {
	File  string `json:"file"`
	Macro string `json:"macro"`
}

type disposition struct {
	ID        string   `json:"id"`
	Component string   `json:"component"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason"`
	Evidence  []string `json:"evidence"`
}

// cpe is the CPE 2.3 name NVD and the SBOM use for the component's version.
func (c component) cpe() string {
	return fmt.Sprintf("cpe:2.3:a:%s:%s:%s:*:*:*:*:*:*:*", c.CPEVendor, c.CPEProduct, c.Version)
}

// reviewDate parses reviewed_on.
func (r record) reviewDate() (time.Time, error) {
	return time.Parse(time.DateOnly, r.ReviewedOn)
}

var cveID = regexp.MustCompile(`^CVE-[0-9]{4}-[0-9]{4,}$`)

// loadRecord reads the record strictly: unknown fields and trailing data
// are errors, so a misspelt key cannot silently drop a check.
func loadRecord(root string) (record, error) {
	var r record
	data, err := os.ReadFile(filepath.Join(root, recordPath))
	if err != nil {
		return r, fmt.Errorf("read %s: %w", recordPath, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("parse %s: %w", recordPath, err)
	}
	if dec.More() {
		return r, fmt.Errorf("parse %s: trailing data after the record", recordPath)
	}
	return r, nil
}

// validate checks the record's own consistency.
func (r record) validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if r.SchemaVersion != 1 {
		add("schema_version is %d, want 1", r.SchemaVersion)
	}
	if _, err := r.reviewDate(); err != nil {
		add("reviewed_on %q is not a YYYY-MM-DD date", r.ReviewedOn)
	}
	for _, file := range engineFiles {
		if _, ok := r.EngineFiles[file]; !ok {
			add("engine_files does not pin %s", file)
		}
	}
	if len(r.EngineFiles) != len(engineFiles) {
		add("engine_files pins %d files; the engine is exactly %v", len(r.EngineFiles), engineFiles)
	}
	for file, sum := range r.EngineFiles {
		if len(sum) != 64 || strings.Trim(sum, "0123456789abcdef") != "" {
			add("engine_files[%s] is not a lowercase SHA-256", file)
		}
	}
	if len(r.Components) == 0 {
		add("components is empty")
	}
	names := map[string]bool{}
	for i, c := range r.Components {
		if c.Name == "" || c.CPEVendor == "" || c.CPEProduct == "" {
			add("components[%d] needs name, cpe_vendor and cpe_product", i)
		}
		if names[c.Name] {
			add("component %q is listed twice", c.Name)
		}
		names[c.Name] = true
		if _, err := parseVersion(c.Version); err != nil {
			add("component %q: %v", c.Name, err)
		}
		if len(c.VersionSources) == 0 {
			add("component %q has no version_sources", c.Name)
		}
		for _, s := range c.VersionSources {
			if _, ok := r.EngineFiles[s.File]; !ok {
				add("component %q reads its version from %s, which engine_files does not pin", c.Name, s.File)
			}
			if s.Macro == "" {
				add("component %q has a version source without a macro", c.Name)
			}
		}
	}
	ids := map[string]bool{}
	for i, d := range r.Dispositions {
		if !cveID.MatchString(d.ID) {
			add("dispositions[%d]: id %q is not a CVE ID", i, d.ID)
		}
		if ids[d.ID] {
			add("%s has two dispositions", d.ID)
		}
		ids[d.ID] = true
		if !names[d.Component] {
			add("%s names unknown component %q", d.ID, d.Component)
		}
		if d.Status != statusNotAffected {
			add("%s: status %q; the only disposition is %q, and an advisory that affects the engine needs an upgrade instead", d.ID, d.Status, statusNotAffected)
		}
		if strings.TrimSpace(d.Reason) == "" {
			add("%s has no reason", d.ID)
		}
		if len(d.Evidence) == 0 {
			add("%s has no evidence", d.ID)
		}
		for _, e := range d.Evidence {
			if strings.TrimSpace(e) == "" {
				add("%s has an empty evidence entry", d.ID)
			}
		}
	}
	return joinProblems(recordPath, problems)
}

// checkAgainstTree fails when the vendored engine is not the reviewed one:
// a pinned file changed (even at the same version), a version macro
// disagrees with the record, or the SBOM names other versions or CPEs.
func (r record) checkAgainstTree(root string) error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	files := make([]string, 0, len(r.EngineFiles))
	for file := range r.EngineFiles {
		files = append(files, file)
	}
	sort.Strings(files)
	contents := map[string][]byte{}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			add("read %s: %v", file, err)
			continue
		}
		contents[file] = data
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != r.EngineFiles[file] {
			add("%s changed since the review (SHA-256 %s, reviewed %s): review the engine's advisories again", file, got, r.EngineFiles[file])
		}
	}
	for _, c := range r.Components {
		for _, s := range c.VersionSources {
			data, ok := contents[s.File]
			if !ok {
				continue
			}
			got, err := macroVersion(data, s.Macro)
			if err != nil {
				add("%s: %v", s.File, err)
				continue
			}
			if got != c.Version {
				add("%s defines %s as %s, but the review is of %s %s", s.File, s.Macro, got, c.Name, c.Version)
			}
		}
	}
	if err := r.checkSBOM(root); err != nil {
		add("%v", err)
	}
	return joinProblems("the vendored engine is not the reviewed one", problems)
}

type sbomNative struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	CPE     string `json:"cpe"`
}

func (r record) checkSBOM(root string) error {
	data, err := os.ReadFile(filepath.Join(root, sbomPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", sbomPath, err)
	}
	var sbom struct {
		Native []sbomNative `json:"native"`
	}
	if err := json.Unmarshal(data, &sbom); err != nil {
		return fmt.Errorf("parse %s: %w", sbomPath, err)
	}
	var problems []string
	for _, c := range r.Components {
		found := false
		for _, n := range sbom.Native {
			if n.Name != c.Name {
				continue
			}
			found = true
			if n.Version != c.Version || n.CPE != c.cpe() {
				problems = append(problems, fmt.Sprintf("%s lists %s %s (%s); the review is of %s", sbomPath, n.Name, n.Version, n.CPE, c.cpe()))
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s has no native entry named %s", sbomPath, c.Name))
		}
	}
	return joinProblems("SBOM", problems)
}

// macroVersion returns the value of `#define <macro> <value>` (quoted or
// not). Every definition in the file must agree.
func macroVersion(source []byte, macro string) (string, error) {
	pattern := regexp.MustCompile(`(?m)^#define[ \t]+` + regexp.QuoteMeta(macro) + `[ \t]+"?([0-9][0-9.]*)"?[ \t]*$`)
	matches := pattern.FindAllSubmatch(source, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no #define %s with a version", macro)
	}
	value := string(matches[0][1])
	for _, m := range matches[1:] {
		if string(m[1]) != value {
			return "", fmt.Errorf("#define %s has two values, %s and %s", macro, value, m[1])
		}
	}
	return value, nil
}

func joinProblems(context string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(context + ":\n  - " + strings.Join(problems, "\n  - "))
}
