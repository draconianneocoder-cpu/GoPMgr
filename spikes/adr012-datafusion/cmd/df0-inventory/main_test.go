// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func graph(t *testing.T) (metadata, string) {
	t.Helper()
	root := t.TempDir()
	m := metadata{}
	for _, name := range []string{"df0-worker", "normal", "builder", "dev"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for file, text := range map[string]string{"Cargo.toml": "[package]\n", "LICENSE": "synthetic notice\n"} {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		m.Packages = append(m.Packages, cargoPackage{ID: name, Name: name, Version: "1.0.0", License: "MIT", ManifestPath: filepath.Join(dir, "Cargo.toml")})
		m.Resolve.Nodes = append(m.Resolve.Nodes, node{ID: name, Features: []string{"z", "a"}})
	}
	build, dev := "build", "dev"
	m.Resolve.Nodes[0].Deps = []dependency{
		{Name: "normal", Pkg: "normal", Kinds: []depKind{{}}},
		{Name: "builder", Pkg: "builder", Kinds: []depKind{{Kind: &build}}},
		{Name: "dev", Pkg: "dev", Kinds: []depKind{{Kind: &dev}}},
	}
	m.Packages[2].Targets = []cargoTarget{{Kind: []string{"lib"}}, {Kind: []string{"custom-build"}}}
	return m, root
}

func TestInventoryGraphAndRedaction(t *testing.T) {
	m, root := graph(t)
	r, err := inventory(m, root, "synthetic-target")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Packages) != 3 || len(r.Relationships) != 2 {
		t.Fatalf("wrong non-dev closure: %+v", r)
	}
	kinds := make(map[string]int)
	for _, rel := range r.Relationships {
		kinds[rel.Kind]++
	}
	if !reflect.DeepEqual(kinds, map[string]int{"normal": 1, "build": 1}) {
		t.Fatalf("lost dependency kinds: %v", kinds)
	}
	for _, p := range r.Packages {
		if p.Name == "builder" && !reflect.DeepEqual(p.TargetKinds, []string{"custom-build", "lib"}) {
			t.Fatal("lost declared build targets")
		}
		if p.Name == "dev" || p.DeclaredLicense != "MIT" || len(p.Notices) != 1 || p.Notices[0].SHA256 != "4fe52c2d62620ffad30928d7a0224a6348dff38c6bc2b2f6911375838225602e" {
			t.Fatalf("lost declarations/notice or included dev: %+v", p)
		}
		if !reflect.DeepEqual(p.Features, []string{"a", "z"}) {
			t.Fatal(p.Features)
		}
	}
	a, _ := json.Marshal(r)
	if bytes.Contains(a, []byte(root)) || bytes.Contains(a, []byte("path+file:")) {
		t.Fatalf("private path leaked: %s", a)
	}
	m.Packages[0], m.Packages[3] = m.Packages[3], m.Packages[0]
	m.Resolve.Nodes[0], m.Resolve.Nodes[3] = m.Resolve.Nodes[3], m.Resolve.Nodes[0]
	r2, err := inventory(m, root, "synthetic-target")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r2)
	if !bytes.Equal(a, b) {
		t.Fatal("metadata ordering changed output")
	}
}

func TestInventoryRejectsIncompleteGraph(t *testing.T) {
	for _, mutation := range []string{"missing-package", "missing-node", "duplicate-package", "duplicate-node", "missing-worker", "ambiguous-worker", "missing-kinds"} {
		t.Run(mutation, func(t *testing.T) {
			m, root := graph(t)
			switch mutation {
			case "missing-package":
				m.Packages = append(m.Packages[:1], m.Packages[2:]...)
			case "missing-node":
				m.Resolve.Nodes = append(m.Resolve.Nodes[:1], m.Resolve.Nodes[2:]...)
			case "duplicate-package":
				m.Packages = append(m.Packages, m.Packages[0])
			case "duplicate-node":
				m.Resolve.Nodes = append(m.Resolve.Nodes, m.Resolve.Nodes[0])
			case "missing-worker":
				m.Packages[0].Name = "other"
			case "ambiguous-worker":
				m.Packages[1].Name = "df0-worker"
			case "missing-kinds":
				m.Resolve.Nodes[0].Deps[0].Kinds = nil
			}
			if _, err := inventory(m, root, "synthetic-target"); err == nil {
				t.Fatal("accepted incomplete or ambiguous graph")
			}
		})
	}
}

func TestInventoryNestedNoticeHashChanges(t *testing.T) {
	m, root := graph(t)
	dir := filepath.Join(root, "normal", "LICENSES")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "embedded.txt")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := inventory(m, root, "synthetic-target")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := inventory(m, root, "synthetic-target")
	if err != nil {
		t.Fatal(err)
	}
	var first, second string
	for _, p := range a.Packages {
		for _, f := range p.Notices {
			if f.Path == "LICENSES/embedded.txt" {
				first = f.SHA256
			}
		}
	}
	for _, p := range b.Packages {
		for _, f := range p.Notices {
			if f.Path == "LICENSES/embedded.txt" {
				second = f.SHA256
			}
		}
	}
	if first == "" || second == "" || first == second {
		t.Fatal("nested notice omission or content change escaped inventory")
	}
}

func TestInventoryPreservesSourceAndUnknownLicense(t *testing.T) {
	m, root := graph(t)
	source := cratesIO
	m.Packages[1].Source = &source
	m.Packages[1].License = ""
	r, err := inventory(m, root, "synthetic-target")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range r.Packages {
		if p.Name == "normal" {
			found = true
			if p.Source != cratesIO || p.DeclaredLicense != "" {
				t.Fatalf("invented provenance/license: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("normal package omitted")
	}
	unreviewed := "git+https://invalid.example/unreviewed"
	m.Packages[1].Source = &unreviewed
	if _, err := inventory(m, root, "synthetic-target"); err == nil || !strings.Contains(err.Error(), "unreviewed registry/git source") {
		t.Fatalf("accepted unreviewed source: %v", err)
	}
	m.Packages[1].Source = &source
	outside := filepath.Join(t.TempDir(), "Cargo.toml")
	if err := os.WriteFile(outside, []byte("[package]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.Packages[0].ManifestPath = outside
	if _, err := inventory(m, root, "synthetic-target"); err == nil || !strings.Contains(err.Error(), "local source outside repository") {
		t.Fatalf("accepted unexplained local source outside repository: %v", err)
	}
}

func TestInventoryRejectsUnsafeNotice(t *testing.T) {
	for _, mode := range []string{"symlink", "license-escape", "license-parent-symlink", "missing-declared-file"} {
		t.Run(mode, func(t *testing.T) {
			m, root := graph(t)
			if mode == "symlink" {
				if err := os.Symlink("Cargo.toml", filepath.Join(root, "df0-worker", "NOTICE")); err != nil {
					t.Skip(err)
				}
			} else if mode == "license-escape" {
				m.Packages[0].LicenseFile = "../../private"
			} else if mode == "license-parent-symlink" {
				outside := t.TempDir()
				if err := os.WriteFile(filepath.Join(outside, "LICENSE"), []byte("outside"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "df0-worker", "other")); err != nil {
					t.Skip(err)
				}
				m.Packages[0].LicenseFile = "other/LICENSE"
			} else {
				m.Packages[0].LicenseFile = "absent"
			}
			if _, err := inventory(m, root, "synthetic-target"); err == nil {
				t.Fatalf("accepted unsafe notice: %s", mode)
			}
		})
	}
}

func TestInventoryRejectsTrailingJSON(t *testing.T) {
	var out bytes.Buffer
	if err := run(strings.NewReader(`{} {}`), &out, t.TempDir(), "target"); err == nil || out.Len() != 0 {
		t.Fatalf("accepted ambiguous input or wrote success: %v", err)
	}
}
