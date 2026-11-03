// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// reviewed is tools/sbom/components.json: what cannot be read from the build
// itself and was checked by a person.
type reviewed struct {
	Author string `json:"author"`
	App    struct {
		Name       string `json:"name"`
		Producer   string `json:"producer"`
		License    string `json:"license"`
		PurlBase   string `json:"purl_base"`
		Repository string `json:"repository"`
	} `json:"app"`
	GoStdlib  reviewedEntry            `json:"go_stdlib"`
	GoModules map[string]reviewedEntry `json:"go_modules"`
	Native    []reviewedNative         `json:"native"`
	Embedded  []struct {
		Pattern string `json:"pattern"`
		reviewedEntry
	} `json:"embedded"`
	EmbeddedRoots  []string `json:"embedded_roots"`
	EmbeddedIgnore []string `json:"embedded_ignore"`
	// FirstPartyEmbedded lists folders whose embedded files are GoPMgr's own
	// (or, for frontend/dist, its build output, whose copied third-party
	// files are reviewed under their frontend/public source).
	FirstPartyEmbedded []string                   `json:"first_party_embedded"`
	NPM                map[string]reviewedEntry   `json:"npm"`
	Platform           map[string][]reviewedEntry `json:"platform"`
	Comment            string                     `json:"comment"`
}

// reviewedNative is code from another project inside a linked Go module.
type reviewedNative struct {
	ParentModule string `json:"parent_module"`
	// ParentPackage, when set, limits the entry to builds that link this
	// package, for code a module carries for one platform only.
	ParentPackage string `json:"parent_package"`
	// ModuleFile is the component's file inside the module, hashed when set.
	ModuleFile string `json:"module_file"`
	// InInstaller says the installer package also carries the file.
	InInstaller bool `json:"in_installer"`
	// LicenseNote explains a license with no SPDX identifier; it replaces
	// License, which must then be empty.
	LicenseNote string `json:"license_note"`
	// Within names another native entry this code is compiled into (for
	// example a library compiled into another component's static archives). The entry is
	// listed, with a dependency from that component, wherever it is listed.
	Within string `json:"within"`
	reviewedEntry
}

type reviewedEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Producer  string `json:"producer"`
	License   string `json:"license"`
	Purl      string `json:"purl"`
	CPE       string `json:"cpe"`
	Commit    string `json:"commit"`
	Note      string `json:"note"`
	LocalNote string `json:"local_note"`
}

func loadReviewed(name string) (*reviewed, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var r reviewed
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err := r.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &r, nil
}

func (r *reviewed) validate() error {
	if r.Author == "" || r.App.Name == "" || r.App.Producer == "" || r.App.PurlBase == "" {
		return errors.New("author, app name, app producer and app purl_base are required")
	}
	check := func(where, expr string, required bool) error {
		if expr == "" && !required {
			return nil
		}
		if err := checkLicenseExpression(expr); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		return nil
	}
	if err := check("app", r.App.License, true); err != nil {
		return err
	}
	if err := check("go_stdlib", r.GoStdlib.License, true); err != nil {
		return err
	}
	for mod, e := range r.GoModules {
		if err := check(mod, e.License, true); err != nil {
			return err
		}
	}
	natives := map[string]bool{}
	for _, n := range r.Native {
		if n.Within == "" {
			natives[n.Name] = true
		}
	}
	for _, n := range r.Native {
		if n.ParentModule == "" || n.Name == "" {
			return errors.New("every native entry needs parent_module and name")
		}
		if (n.License == "") == (n.LicenseNote == "") {
			return fmt.Errorf("native entry %s needs exactly one of license and license_note", n.Name)
		}
		if err := check(n.Name, n.License, false); err != nil {
			return err
		}
		if n.Within != "" && !natives[n.Within] {
			return fmt.Errorf("native entry %s is within %q, which is not a native entry without within", n.Name, n.Within)
		}
	}
	for _, e := range r.Embedded {
		if e.Pattern == "" || e.Name == "" {
			return errors.New("every embedded entry needs pattern and name")
		}
		if err := check(e.Pattern, e.License, true); err != nil {
			return err
		}
	}
	for name, e := range r.NPM {
		if err := check(name, e.License, false); err != nil {
			return err
		}
	}
	for goos, list := range r.Platform {
		if _, err := findGOOS(goos); err != nil {
			return err
		}
		for _, e := range list {
			if e.Name == "" {
				return fmt.Errorf("platform %s: every entry needs a name", goos)
			}
			if err := check(e.Name, e.License, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func findGOOS(goos string) (target, error) {
	for _, t := range releaseTargets {
		if t.GOOS == goos {
			return t, nil
		}
	}
	return target{}, fmt.Errorf("platform %q is not a release target", goos)
}

var (
	licenseToken = regexp.MustCompile(`^(\(|\)|AND|OR|WITH|(LicenseRef-|DocumentRef-[A-Za-z0-9.-]+:LicenseRef-)?[A-Za-z0-9][A-Za-z0-9.-]*\+?)`)
	licenseIDish = regexp.MustCompile(`^(LicenseRef-|DocumentRef-)?[A-Za-z0-9][A-Za-z0-9.:-]*\+?$`)
)

// checkLicenseExpression checks SPDX license expression syntax (SPDX 3.0.1
// Annex B): identifiers joined by AND, OR and WITH, with parentheses. It does
// not check identifiers against the SPDX License List, which is not available
// offline.
func checkLicenseExpression(expr string) error {
	var toks []string
	rest := strings.TrimSpace(expr)
	for rest != "" {
		m := licenseToken.FindString(rest)
		if m == "" {
			return fmt.Errorf("license expression %q: unexpected %q", expr, rest)
		}
		toks = append(toks, m)
		rest = strings.TrimLeft(rest[len(m):], " ")
	}
	p := &exprParser{toks: toks}
	if err := p.or(); err != nil {
		return fmt.Errorf("license expression %q: %w", expr, err)
	}
	if p.i != len(toks) {
		return fmt.Errorf("license expression %q: unexpected %q", expr, toks[p.i])
	}
	return nil
}

type exprParser struct {
	toks []string
	i    int
}

func (p *exprParser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *exprParser) or() error {
	if err := p.and(); err != nil {
		return err
	}
	for p.peek() == "OR" {
		p.i++
		if err := p.and(); err != nil {
			return err
		}
	}
	return nil
}

func (p *exprParser) and() error {
	if err := p.with(); err != nil {
		return err
	}
	for p.peek() == "AND" {
		p.i++
		if err := p.with(); err != nil {
			return err
		}
	}
	return nil
}

func (p *exprParser) with() error {
	if err := p.atom(); err != nil {
		return err
	}
	if p.peek() == "WITH" {
		p.i++
		return p.id()
	}
	return nil
}

func (p *exprParser) atom() error {
	if p.peek() == "(" {
		p.i++
		if err := p.or(); err != nil {
			return err
		}
		if p.peek() != ")" {
			return errors.New("missing )")
		}
		p.i++
		return nil
	}
	return p.id()
}

func (p *exprParser) id() error {
	t := p.peek()
	switch t {
	case "", "(", ")", "AND", "OR", "WITH":
		return fmt.Errorf("expected a license identifier, got %q", t)
	}
	if !licenseIDish.MatchString(t) {
		return fmt.Errorf("invalid license identifier %q", t)
	}
	p.i++
	return nil
}
