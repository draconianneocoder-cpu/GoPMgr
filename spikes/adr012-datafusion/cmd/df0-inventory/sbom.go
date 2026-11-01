// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type nativeEvidence struct {
	WorkerSHA256 string   `json:"worker_sha256"`
	Libraries    []string `json:"dynamic_libraries"`
}
type spdxDocument struct {
	Context string           `json:"@context"`
	Graph   []map[string]any `json:"@graph"`
}
type cargoEvent struct {
	Reason    string
	PackageID string `json:"package_id"`
	Target    struct {
		Name string
		Kind []string
	}
	Features    []string
	Executable  *string
	Profile     struct{ Test bool }
	LinkedLibs  []string `json:"linked_libs"`
	LinkedPaths []string `json:"linked_paths"`
	Success     bool
}
type archiveInput struct{ Name, SHA256 string }

func runSPDX(in io.Reader, out io.Writer, root, target, logPath, worker, nativePath, created, revision string) error {
	r, err := readInventory(in, root, target)
	if err != nil {
		return err
	}
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open Cargo build log: %w", err)
	}
	defer f.Close()
	b, err := os.ReadFile(nativePath)
	if err != nil {
		return fmt.Errorf("read native observations: %w", err)
	}
	var native nativeEvidence
	if err := json.Unmarshal(b, &native); err != nil {
		return fmt.Errorf("decode native observations: %w", err)
	}
	doc, err := buildSBOM(r, f, worker, native, created, revision)
	if err != nil {
		return err
	}
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(doc)
}

func buildSBOM(r report, log io.Reader, worker string, native nativeEvidence, created, revision string) (spdxDocument, error) {
	var doc spdxDocument
	when, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return doc, errors.New("created must be an RFC3339 observation time")
	}
	created = when.UTC().Format(time.RFC3339)
	rev, err := hex.DecodeString(revision)
	if err != nil || len(rev) != 20 {
		return doc, errors.New("revision must be a full source commit")
	}
	workerHash, err := hashFile(worker)
	if err != nil {
		return doc, err
	}
	if workerHash != native.WorkerSHA256 {
		return doc, errors.New("native observations do not match worker hash")
	}
	workerPath, err := filepath.EvalSymlinks(worker)
	if err != nil {
		return doc, err
	}
	workerPath, err = filepath.Abs(workerPath)
	if err != nil {
		return doc, err
	}
	byCargo := make(map[string]entry)
	for _, p := range r.Packages {
		byCargo[p.cargoID] = p
	}
	observed := make(map[string]bool)
	features := make(map[string][]string)
	var archives []archiveInput
	finished, workerSeen := false, false
	d := json.NewDecoder(log)
	for {
		var event cargoEvent
		if err := d.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return doc, fmt.Errorf("decode build log: %w", err)
		}
		if finished {
			return doc, errors.New("build event after completion")
		}
		switch event.Reason {
		case "compiler-artifact", "build-script-executed":
			p, ok := byCargo[event.PackageID]
			if !ok {
				return doc, errors.New("build package absent from resolved worker inventory")
			}
			if event.Reason == "compiler-artifact" {
				observed[p.ID] = true
				features[p.ID] = append(features[p.ID], event.Features...)
				if event.Executable != nil && p.Name == "df0-worker" && event.Target.Name == "df0-worker" && slices.Contains(event.Target.Kind, "bin") && !event.Profile.Test {
					path, err := filepath.EvalSymlinks(*event.Executable)
					if err != nil {
						return doc, err
					}
					path, err = filepath.Abs(path)
					if err != nil || path != workerPath || workerSeen {
						return doc, errors.New("ambiguous or mismatched worker executable")
					}
					workerSeen = true
				}
			} else {
				for _, lib := range event.LinkedLibs {
					if !strings.HasPrefix(lib, "static=") {
						return doc, errors.New("unsupported native link directive")
					}
					name := strings.TrimPrefix(lib, "static=")
					if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") {
						return doc, errors.New("invalid native archive name")
					}
					var paths []string
					for _, search := range event.LinkedPaths {
						if !strings.HasPrefix(search, "native=") {
							continue
						}
						path := filepath.Join(strings.TrimPrefix(search, "native="), "lib"+name+".a")
						if _, err := os.Lstat(path); err == nil {
							paths = append(paths, path)
						} else if !errors.Is(err, os.ErrNotExist) {
							return doc, err
						}
					}
					slices.Sort(paths)
					paths = slices.Compact(paths)
					if len(paths) != 1 {
						return doc, errors.New("missing or ambiguous static archive")
					}
					h, err := hashFile(paths[0])
					if err != nil {
						return doc, err
					}
					archives = append(archives, archiveInput{Name: p.Name + "@" + p.Version + "/lib" + name + ".a", SHA256: h})
				}
			}
		case "build-finished":
			if !event.Success {
				return doc, errors.New("Cargo build failed")
			}
			finished = true
		case "compiler-message": // Diagnostics are not build-input evidence.
		default:
			return doc, errors.New("unknown Cargo build event")
		}
	}
	if !finished || !workerSeen {
		return doc, errors.New("successful completed worker build required")
	}
	for k, v := range features {
		slices.Sort(v)
		features[k] = slices.Compact(v)
	}
	slices.SortFunc(archives, func(a, b archiveInput) int { return strings.Compare(a.Name, b.Name) })
	for i := 1; i < len(archives); i++ {
		if archives[i].Name == archives[i-1].Name && archives[i].SHA256 != archives[i-1].SHA256 {
			return doc, errors.New("conflicting native archive observations")
		}
	}
	archives = slices.Compact(archives)
	native.Libraries = slices.Clone(native.Libraries)
	slices.Sort(native.Libraries)
	native.Libraries = slices.Compact(native.Libraries)
	var inputs []entry
	for _, p := range r.Packages {
		if observed[p.ID] {
			inputs = append(inputs, p)
		}
	}
	payload, err := json.Marshal([]any{created, revision, r.Target, workerHash, inputs, features, archives, native.Libraries})
	if err != nil {
		return doc, err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(payload))
	base := "https://github.com/draconianneocoder-cpu/GoPMgr/spdx/df0/" + fingerprint + "/"
	id := func(kind, key string) string {
		return base + kind + "/" + fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	}
	doc.Context = "https://spdx.org/rdf/3.0.1/spdx-context.jsonld"
	var all []string
	add := func(kind, key string, fields map[string]any) string {
		uri := id(kind, key)
		fields["type"] = kind
		fields["spdxId"] = uri
		fields["creationInfo"] = "_:creation"
		doc.Graph = append(doc.Graph, fields)
		all = append(all, uri)
		return uri
	}
	relate := func(from, kind string, to []string, scope string, comment string) {
		fields := map[string]any{"from": from, "to": to, "relationshipType": kind, "completeness": "incomplete", "comment": comment}
		typ := "Relationship"
		if scope != "" {
			typ = "LifecycleScopedRelationship"
			fields["scope"] = scope
		}
		add(typ, from+kind+strings.Join(to, "\n")+scope, fields)
	}
	agent := add("Agent", "automation", map[string]any{"name": "GoPMgr DF0 automated evidence generation"})
	tool := add("Tool", "exporter", map[string]any{"name": "GoPMgr df0-inventory SPDX exporter"})
	doc.Graph = append(doc.Graph, map[string]any{"type": "CreationInfo", "@id": "_:creation", "created": created, "createdBy": []string{agent}, "createdUsing": []string{tool}, "specVersion": "3.0.1"})
	cc0 := add("simplelicensing_LicenseExpression", "CC0-1.0", map[string]any{"simplelicensing_licenseExpression": "CC0-1.0"})
	file := func(name, hash, purpose string) string {
		return add("software_File", name, map[string]any{"name": name, "software_primaryPurpose": purpose, "verifiedUsing": []any{map[string]any{"type": "Hash", "algorithm": "sha256", "hashValue": hash}}})
	}
	output := file("df0-worker", workerHash, "executable")
	build := add("build_Build", "cargo", map[string]any{"name": "Observed native Cargo worker build", "build_buildType": "https://doc.rust-lang.org/cargo/commands/cargo-build.html", "build_buildId": workerHash, "description": "Cargo JSON build observations, including cached artifacts; not a clean rebuild or full final linker composition. Source revision " + revision + "; target " + r.Target})
	var buildInputs []string
	licenses := map[string]string{"CC0-1.0": cc0}
	for _, p := range inputs {
		pkg := add("software_Package", p.ID, map[string]any{"name": p.Name, "software_packageVersion": p.Version, "software_sourceInfo": p.Source + "; Cargo.toml sha256=" + p.ManifestSHA256 + "; raw declared license=" + p.DeclaredLicense + "; observed build-unit feature union=" + strings.Join(features[p.ID], ","), "comment": "Observed compiler input package, not proof every package byte is linked. Repository sources/overrides are not upstream registry archives."})
		buildInputs = append(buildInputs, pkg)
		// Cargo's historical slash separator denotes alternatives. Preserve the
		// raw declaration above; validation separately checks SPDX expressions.
		expr := strings.Join(strings.Fields(strings.ReplaceAll(p.DeclaredLicense, "/", " OR ")), " ")
		if expr != "" {
			license := licenses[expr]
			if license == "" {
				license = add("simplelicensing_LicenseExpression", expr, map[string]any{"simplelicensing_licenseExpression": expr})
				licenses[expr] = license
			}
			relate(pkg, "hasDeclaredLicense", []string{license}, "", "Cargo declaration only; no concluded license or distribution clearance asserted.")
		}
	}
	for _, a := range archives {
		buildInputs = append(buildInputs, file(a.Name, a.SHA256, "archive"))
	}
	relate(build, "hasInput", buildInputs, "build", "Observed Cargo packages and declared static archive inputs only. Compiler standard library, full native-source constituents and SDK inputs are not exhaustively inventoried.")
	relate(build, "hasOutput", []string{output}, "build", "Worker executable matched Cargo output path and hash-bound native observations.")
	for _, lib := range native.Libraries {
		path, _, _ := strings.Cut(lib, " (compatibility")
		if !strings.HasPrefix(path, "/") {
			return doc, errors.New("invalid dynamic library observation")
		}
		pkg := add("software_Package", lib, map[string]any{"name": filepath.Base(path), "software_sourceInfo": lib, "comment": "System dynamic-library load reference; not a bundled package or runtime-version/license conclusion."})
		relate(output, "hasDynamicLink", []string{pkg}, "runtime", "Observed Mach-O load reference for this exact executable.")
	}
	sbom := add("software_Sbom", "build-sbom", map[string]any{"name": "DF0 macOS worker build SBOM", "software_sbomType": []string{"build"}, "rootElement": []string{output}, "description": "Partial build-input SBOM. Not an installed-package SBOM, complete binary composition, legal conclusion or LC-GATE clearance."})
	for _, n := range doc.Graph {
		if n["spdxId"] == sbom {
			n["element"] = slices.Clone(all[:len(all)-1])
		}
	}
	add("SpdxDocument", "document", map[string]any{"name": "GoPMgr isolated DF0 worker evidence", "dataLicense": cc0, "profileConformance": []string{"core", "software", "build", "simpleLicensing"}, "rootElement": []string{sbom}, "element": slices.Clone(all)})
	return doc, nil
}
