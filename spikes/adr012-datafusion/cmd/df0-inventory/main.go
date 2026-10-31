// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command df0-inventory summarizes trusted, locked Cargo metadata and available
// notice files. It produces a review input, not a license conclusion or SBOM.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const cratesIO = "registry+https://github.com/rust-lang/crates.io-index"

type cargoPackage struct {
	ID, Name, Version, License string
	Source                     *string
	ManifestPath               string `json:"manifest_path"`
	LicenseFile                string `json:"license_file"`
	Targets                    []cargoTarget
}
type cargoTarget struct{ Kind []string }
type depKind struct {
	Kind   *string
	Target *string
}
type dependency struct {
	Name, Pkg string
	Kinds     []depKind `json:"dep_kinds"`
}
type node struct {
	ID       string
	Features []string
	Deps     []dependency
}
type metadata struct {
	Packages []cargoPackage
	Resolve  struct{ Nodes []node }
}
type fileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type entry struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Version         string     `json:"version"`
	Source          string     `json:"source"`
	DeclaredLicense string     `json:"declared_license"`
	ManifestSHA256  string     `json:"manifest_sha256"`
	Features        []string   `json:"workspace_resolved_features"`
	TargetKinds     []string   `json:"declared_target_kinds"`
	Notices         []fileHash `json:"available_notice_files"`
}
type relationship struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Kind   string  `json:"kind"`
	Target *string `json:"target"`
}
type report struct {
	Scope         string         `json:"scope"`
	Target        string         `json:"metadata_filter_target"`
	Packages      []entry        `json:"packages"`
	Relationships []relationship `json:"relationships"`
}

func main() {
	root := flag.String("repo-root", "", "repository root for local source redaction (required)")
	target := flag.String("target", "", "target used for cargo metadata --filter-platform (required)")
	flag.Parse()
	if err := run(os.Stdin, os.Stdout, *root, *target); err != nil {
		fmt.Fprintln(os.Stderr, "df0-inventory:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer, root, target string) error {
	if root == "" || target == "" {
		return errors.New("-repo-root and -target are required")
	}
	var m metadata
	d := json.NewDecoder(in)
	if err := d.Decode(&m); err != nil {
		return fmt.Errorf("decode metadata: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("expected exactly one metadata document")
	}
	r, err := inventory(m, root, target)
	if err != nil {
		return err
	}
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(r)
}

func inventory(m metadata, root, target string) (report, error) {
	r := report{Scope: "worker-reachable non-dev workspace-resolved graph; declarations only, not linked contents, license conclusions or a complete SBOM", Target: target}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return r, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return r, err
	}
	packages := make(map[string]cargoPackage)
	nodes := make(map[string]node)
	worker := ""
	for _, p := range m.Packages {
		if _, ok := packages[p.ID]; ok || p.ID == "" {
			return r, errors.New("duplicate or empty package identity")
		}
		packages[p.ID] = p
		if p.Name == "df0-worker" {
			if worker != "" {
				return r, errors.New("ambiguous worker package")
			}
			worker = p.ID
		}
	}
	if worker == "" {
		return r, errors.New("worker package missing")
	}
	for _, n := range m.Resolve.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return r, errors.New("duplicate resolve node")
		}
		nodes[n.ID] = n
	}
	selected := make(map[string]entry)
	queue := []string{worker}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := selected[id]; ok {
			continue
		}
		p, ok := packages[id]
		if !ok {
			return r, errors.New("reachable package missing")
		}
		n, ok := nodes[id]
		if !ok {
			return r, errors.New("reachable resolve node missing")
		}
		e, err := describe(p, n, root)
		if err != nil {
			return r, fmt.Errorf("%s@%s: %w", p.Name, p.Version, err)
		}
		selected[id] = e
		for _, dep := range n.Deps {
			if len(dep.Kinds) == 0 {
				return r, errors.New("dependency kind missing")
			}
			for _, k := range dep.Kinds {
				if k.Kind != nil && *k.Kind == "dev" {
					continue
				}
				if k.Kind != nil && *k.Kind != "build" {
					return r, errors.New("unknown dependency kind")
				}
				queue = append(queue, dep.Pkg)
			}
		}
	}
	stable := make(map[string]bool)
	for id, e := range selected {
		if stable[e.ID] {
			return r, errors.New("redacted identity collision")
		}
		stable[e.ID] = true
		r.Packages = append(r.Packages, e)
		for _, dep := range nodes[id].Deps {
			for _, k := range dep.Kinds {
				if k.Kind != nil && *k.Kind == "dev" {
					continue
				}
				kind := "normal"
				if k.Kind != nil {
					kind = *k.Kind
				}
				r.Relationships = append(r.Relationships, relationship{From: e.ID, To: selected[dep.Pkg].ID, Kind: kind, Target: k.Target})
			}
		}
	}
	slices.SortFunc(r.Packages, func(a, b entry) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(r.Relationships, func(a, b relationship) int {
		key := func(v relationship) string {
			target := ""
			if v.Target != nil {
				target = *v.Target
			}
			return v.From + "\x00" + v.To + "\x00" + v.Kind + "\x00" + target
		}
		return strings.Compare(key(a), key(b))
	})
	return r, nil
}

func describe(p cargoPackage, n node, root string) (entry, error) {
	e := entry{Name: p.Name, Version: p.Version, DeclaredLicense: p.License, Features: slices.Clone(n.Features), Notices: []fileHash{}}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p.ManifestPath))
	if err != nil {
		return e, err
	}
	if p.Source == nil {
		rel, err := within(root, dir)
		if err != nil {
			return e, errors.New("local source outside repository")
		}
		e.Source = "repo:" + filepath.ToSlash(rel)
	} else {
		if *p.Source != cratesIO {
			return e, errors.New("unreviewed registry/git source")
		}
		e.Source = *p.Source
	}
	e.ID = p.Name + "@" + p.Version + "#" + e.Source
	slices.Sort(e.Features)
	kinds := make(map[string]bool)
	for _, target := range p.Targets {
		for _, kind := range target.Kind {
			kinds[kind] = true
		}
	}
	for kind := range kinds {
		e.TargetKinds = append(e.TargetKinds, kind)
	}
	slices.Sort(e.TargetKinds)
	e.ManifestSHA256, err = hashFile(p.ManifestPath)
	if err != nil {
		return e, err
	}
	files := make(map[string]bool)
	children, err := os.ReadDir(dir)
	if err != nil {
		return e, err
	}
	for _, f := range children {
		upper := strings.ToUpper(f.Name())
		if strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") || strings.HasPrefix(upper, "COPYING") || strings.HasPrefix(upper, "NOTICE") {
			if err := filepath.WalkDir(filepath.Join(dir, f.Name()), func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !d.IsDir() {
					files[path] = true
				}
				return nil
			}); err != nil {
				return e, err
			}
		}
	}
	if p.LicenseFile != "" {
		path := p.LicenseFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := within(dir, path); err != nil {
			return e, errors.New("declared license file outside package")
		}
		files[path] = true
	}
	for path := range files {
		actual, err := filepath.EvalSymlinks(path)
		if err != nil {
			return e, err
		}
		if _, err := within(dir, actual); err != nil {
			return e, errors.New("notice resolves outside package")
		}
		rel, err := within(dir, path)
		if err != nil {
			return e, err
		}
		h, err := hashFile(path)
		if err != nil {
			return e, err
		}
		e.Notices = append(e.Notices, fileHash{Path: filepath.ToSlash(rel), SHA256: h})
	}
	slices.SortFunc(e.Notices, func(a, b fileHash) int { return strings.Compare(a.Path, b.Path) })
	return e, nil
}

func within(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes root")
	}
	return rel, nil
}

func hashFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("notice/manifest is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
