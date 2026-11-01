// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sbomFixture(t *testing.T) (report, []byte, string, nativeEvidence) {
	t.Helper()
	m, root := graph(t)
	r, err := inventory(m, root, "aarch64-apple-darwin")
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "worker-bin")
	if err := os.WriteFile(worker, []byte("synthetic executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	h, err := hashFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	e := json.NewEncoder(&log)
	for _, id := range []string{"normal", "df0-worker"} {
		var executable any
		if id == "df0-worker" {
			executable = worker
		}
		if err := e.Encode(map[string]any{"reason": "compiler-artifact", "package_id": id,
			"target": map[string]any{"name": id, "kind": []string{"bin"}}, "features": []string{"observed"}, "executable": executable, "fresh": true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Encode(map[string]any{"reason": "build-finished", "success": true}); err != nil {
		t.Fatal(err)
	}
	return r, log.Bytes(), worker, nativeEvidence{WorkerSHA256: h, Libraries: []string{"/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1.0.0)"}}
}

func TestSBOMBuildInputsAndArtifactBinding(t *testing.T) {
	r, log, worker, native := sbomFixture(t)
	doc, err := buildSBOM(r, bytes.NewReader(log), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(filepath.Dir(worker))) || bytes.Contains(b, []byte("path+file:")) {
		t.Fatal("private path leaked")
	}
	packages, output := 0, false
	for _, n := range doc.Graph {
		if n["type"] == "software_Package" {
			packages++
			if n["name"] == "builder" {
				t.Fatal("resolved but unobserved package asserted as build input")
			}
		}
		if n["name"] == "df0-worker" && n["type"] == "software_File" {
			output = true
			if !strings.Contains(string(mustJSON(t, n)), native.WorkerSHA256) {
				t.Fatal("worker hash missing")
			}
		}
		if n["relationshipType"] == "hasConcludedLicense" {
			t.Fatal("invented license conclusion")
		}
	}
	if packages != 3 || !output {
		t.Fatalf("wrong build/system package count or output: %d %v", packages, output)
	}
	doc2, err := buildSBOM(r, bytes.NewReader(log), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
	if err != nil || !bytes.Equal(b, mustJSON(t, doc2)) {
		t.Fatalf("non-deterministic export: %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSBOMRejectsFailedOrUnboundBuild(t *testing.T) {
	for _, mode := range []string{"failed", "missing-finish", "unknown-package", "wrong-executable", "wrong-worker-hash", "trailing-event"} {
		t.Run(mode, func(t *testing.T) {
			r, log, worker, native := sbomFixture(t)
			s := string(log)
			switch mode {
			case "failed":
				s = strings.ReplaceAll(s, `"success":true`, `"success":false`)
			case "missing-finish":
				s = s[:strings.LastIndex(strings.TrimSpace(s), "\n")+1]
			case "unknown-package":
				s = strings.ReplaceAll(s, `"package_id":"normal"`, `"package_id":"unknown"`)
			case "wrong-executable":
				other := filepath.Join(t.TempDir(), "other")
				data, err := os.ReadFile(worker)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(other, data, 0o700); err != nil {
					t.Fatal(err)
				}
				worker = other
			case "wrong-worker-hash":
				native.WorkerSHA256 = strings.Repeat("0", 64)
			case "trailing-event":
				s += `{"reason":"compiler-artifact","package_id":"normal"}`
			}
			_, err := buildSBOM(r, strings.NewReader(s), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
			if mode == "wrong-executable" && (err == nil || !strings.Contains(err.Error(), "mismatched worker executable")) {
				t.Fatalf("wrong executable boundary: %v", err)
			}
			if err == nil {
				t.Fatal("accepted failed/unbound build:", mode)
			}
		})
	}
}

func TestSBOMRequiresNativeArchive(t *testing.T) {
	r, log, worker, native := sbomFixture(t)
	archive := filepath.Join(filepath.Dir(worker), "libexample.a")
	if err := os.WriteFile(archive, []byte("synthetic archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	event := map[string]any{"reason": "build-script-executed", "package_id": "normal", "linked_libs": []string{"static=example"}, "linked_paths": []string{"native=" + filepath.Dir(worker)}}
	data := append(mustJSON(t, event), '\n')
	data = append(data, log...)
	if _, err := buildSBOM(r, bytes.NewReader(data), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if _, err := buildSBOM(r, bytes.NewReader(data), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40)); err == nil {
		t.Fatal("missing static archive silently omitted")
	}
}

func TestSBOMPreservesDeclaredLicenseWithoutConclusion(t *testing.T) {
	r, log, worker, native := sbomFixture(t)
	for i := range r.Packages {
		if r.Packages[i].Name == "normal" {
			r.Packages[i].DeclaredLicense = "MIT/Apache-2.0"
		}
	}
	doc, err := buildSBOM(r, bytes.NewReader(log), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	found, raw := false, false
	for _, n := range doc.Graph {
		if n["simplelicensing_licenseExpression"] == "MIT OR Apache-2.0" {
			found = true
		}
		if n["name"] == "normal" {
			raw = strings.Contains(n["software_sourceInfo"].(string), "MIT/Apache-2.0")
		}
	}
	if !found || !raw {
		t.Fatal("legacy alternatives or original declaration lost")
	}
	for i := range r.Packages {
		if r.Packages[i].Name == "normal" {
			r.Packages[i].DeclaredLicense = ""
		}
	}
	doc, err = buildSBOM(r, bytes.NewReader(log), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	var pkg string
	for _, n := range doc.Graph {
		if n["name"] == "normal" {
			pkg = n["spdxId"].(string)
		}
	}
	if pkg == "" {
		t.Fatal("package omitted")
	}
	for _, n := range doc.Graph {
		if n["from"] == pkg && n["relationshipType"] == "hasDeclaredLicense" {
			t.Fatal("invented a license for unknown declaration")
		}
	}
}

func TestSBOMRepeatedAndConflictingNativeInputs(t *testing.T) {
	r, log, worker, native := sbomFixture(t)
	dir := filepath.Dir(worker)
	if err := os.WriteFile(filepath.Join(dir, "libexample.a"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	event := map[string]any{"reason": "build-script-executed", "package_id": "normal", "linked_libs": []string{"static=example", "static=example"}, "linked_paths": []string{"native=" + dir, "native=" + t.TempDir(), "native=" + dir}}
	data := append(mustJSON(t, event), '\n')
	data = append(data, log...)
	native.Libraries = append(native.Libraries, native.Libraries[0])
	doc, err := buildSBOM(r, bytes.NewReader(data), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, n := range doc.Graph {
		id, ok := n["spdxId"].(string)
		if !ok {
			continue
		}
		if seen[id] {
			t.Fatal("duplicate identity", id)
		}
		seen[id] = true
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "libexample.a"), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	event["linked_paths"] = []string{"native=" + other}
	changed := append(mustJSON(t, event), '\n')
	changed = append(changed, data...)
	if _, err := buildSBOM(r, bytes.NewReader(changed), worker, native, "2026-10-31T23:00:00Z", strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "conflicting native archive") {
		t.Fatalf("conflicting archive accepted: %v", err)
	}
}
