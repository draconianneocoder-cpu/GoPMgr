// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"
)

const fixtureReviewed = `{
  "author": "Example Author Organization",
  "app": {"name": "GoPMgr", "producer": "Example Producer", "license": "GPL-3.0-or-later", "purl_base": "pkg:github/example/gopmgr", "repository": "https://example.invalid/gopmgr"},
  "go_stdlib": {"license": "BSD-3-Clause", "producer": "The Go Authors"},
  "go_modules": {
    "example.com/known": {"license": "MIT", "producer": "Known Producer"},
    "example.com/nameless": {"license": "Apache-2.0", "producer": ""},
    "example.com/local": {"license": "MIT AND blessing", "producer": "Local Producer", "local_note": "built from the repository"}
  },
  "native": [
    {"parent_module": "example.com/local", "name": "Native Library", "version": "", "producer": "", "license": "Unlicense", "cpe": "cpe:2.3:a:example:native:*:*:*:*:*:*:*:*"},
    {"parent_module": "example.com/removed", "name": "Removed Native", "version": "1.0", "producer": "Gone", "license": "MIT"},
    {"parent_module": "example.com/known", "parent_package": "example.com/known/winonly", "module_file": "bin/setup.exe", "in_installer": true, "name": "Vendor Bootstrapper", "version": "", "producer": "Vendor Corp", "license_note": "distributed under the vendor's own terms; no SPDX identifier"}
  ],
  "embedded": [{"pattern": "assets/*.ttf", "name": "Example Font", "version": "", "producer": "Font Foundry", "license": "OFL-1.1"}],
  "embedded_roots": ["assets"],
  "embedded_ignore": ["*.md"],
  "npm": {},
  "platform": {"darwin": [{"name": "Example WebView", "producer": "Platform Vendor", "purl": "pkg:generic/vendor/webview"}]}
}`

func fixtureReviewedValue(t *testing.T) *reviewed {
	t.Helper()
	name := filepath.Join(t.TempDir(), "components.json")
	if err := os.WriteFile(name, []byte(fixtureReviewed), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := loadReviewed(name)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func pkg(path, mod string, imports ...string) goPackage {
	p := goPackage{ImportPath: path, Imports: imports}
	switch mod {
	case "std":
		p.Standard = true
	case "":
		p.Module = &struct {
			Path    string
			Version string
			Main    bool
		}{Path: "gopmgr", Main: true}
	default:
		p.Module = &struct {
			Path    string
			Version string
			Main    bool
		}{Path: mod, Version: "v1.0.0"}
	}
	return p
}

// fixtureInput is a darwin/arm64 build linking a module with a named
// producer, one without, and a local replacement carrying native code, plus
// two npm packages (one without a license or author), an embedded file and
// a platform component.
func fixtureInput(t *testing.T) buildInput {
	t.Helper()
	bi := &buildinfo.BuildInfo{GoVersion: "go1.27.3", Settings: []debug.BuildSetting{{Key: "GOOS", Value: "darwin"}, {Key: "GOARCH", Value: "arm64"}, {Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"}},
		Deps: []*debug.Module{
			{Path: "example.com/known", Version: "v1.0.0", Sum: "h1:known="},
			{Path: "example.com/nameless", Version: "v1.0.0", Sum: "h1:nameless="},
			{Path: "example.com/local", Version: "v1.0.0", Replace: &debug.Module{Path: "./third_party/local", Version: "(devel)"}},
		}}
	return buildInput{
		Repo: t.TempDir(), Target: target{GOOS: "darwin", GOARCH: "arm64"}, Reviewed: fixtureReviewedValue(t), Binary: bi,
		BinaryHex: strings.Repeat("ab", 32), Artifact: "GoPMgr-1.2.0.dmg", ArtifactHex: strings.Repeat("cd", 32), Version: "1.2.0", Commit: "0123456789abcdef0123456789abcdef01234567",
		Packages: []goPackage{
			pkg("gopmgr", "", "example.com/known/k", "example.com/local/l", "fmt"),
			pkg("example.com/known/k", "example.com/known", "example.com/nameless/n", "fmt"),
			pkg("example.com/nameless/n", "example.com/nameless"),
			pkg("example.com/local/l", "example.com/local", "fmt"),
			pkg("fmt", "std"),
		},
		Frontend: []frontendPackage{{"chart.js", "4.6.1"}, {"@scope/bare", "0.1.0"}},
		Lock: map[string]lockPackage{
			"node_modules/chart.js":    {Version: "4.6.1", License: "MIT", Integrity: "sha512-" + b64(bytes.Repeat([]byte{1}, 64)), Dependencies: map[string]string{"@scope/bare": "^0.1.0"}},
			"node_modules/@scope/bare": {Version: "0.1.0"},
		},
		Embedded:    []embeddedFile{{Rel: "assets/Example-Regular.ttf", Entry: 0}},
		EmbeddedHex: map[string]string{"assets/Example-Regular.ttf": strings.Repeat("ef", 32)},
		ModuleArchive: func(mod, version, sum string) (string, error) {
			s := sha256.Sum256([]byte(mod + "@" + version))
			return hex.EncodeToString(s[:]), nil
		},
		ModuleFile: func(mod, version, rel string) (string, error) {
			s := sha256.Sum256([]byte(mod + "@" + version + "/" + rel))
			return hex.EncodeToString(s[:]), nil
		},
	}
}

func b64(b []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	for i := 0; i < len(b); i += 3 {
		var n uint32
		k := 0
		for ; k < 3 && i+k < len(b); k++ {
			n |= uint32(b[i+k]) << (16 - 8*k)
		}
		for j := 0; j < 4; j++ {
			if j > k {
				out.WriteByte('=')
			} else {
				out.WriteByte(enc[n>>(18-6*j)&63])
			}
		}
	}
	return out.String()
}

func fixtureMeta() docMeta {
	return docMeta{Author: "Example Author Organization", Timestamp: "2026-11-02T12:00:00Z", ToolName: toolName, ToolVersion: "0123456", SBOMVersion: 2,
		Serial: "1b4e28ba-2fa1-4d2b-883f-0016d3cca427", Context: "build", Signature: "detached Ed25519 signature, by key sha-256:00"}
}

func encodeFixture(t *testing.T) (*inventory, map[string]any, map[string]any) {
	t.Helper()
	inv, err := assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	cdxBytes, err := encodeCycloneDX(inv, fixtureMeta())
	if err != nil {
		t.Fatal(err)
	}
	spdxBytes, err := encodeSPDX(inv, fixtureMeta())
	if err != nil {
		t.Fatal(err)
	}
	var cdx, spdx map[string]any
	if err := json.Unmarshal(cdxBytes, &cdx); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(spdxBytes, &spdx); err != nil {
		t.Fatal(err)
	}
	return inv, cdx, spdx
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func arr(v any) []any          { a, _ := v.([]any); return a }
func str(v any) string         { s, _ := v.(string); return s }

// TestCycloneDXHasEveryCISAElement walks every component of the CycloneDX
// output and requires each CISA 2026 component element to be present or
// explicitly marked unknown, then checks the SBOM metadata elements.
func TestCycloneDXHasEveryCISAElement(t *testing.T) {
	inv, cdx, _ := encodeFixture(t)
	meta := obj(cdx["metadata"])
	comps := append([]any{meta["component"]}, arr(cdx["components"])...)
	if len(comps) != len(inv.Components) {
		t.Fatalf("CycloneDX has %d components, inventory %d", len(comps), len(inv.Components))
	}
	deps := map[string]bool{}
	for _, d := range arr(cdx["dependencies"]) {
		deps[str(obj(d)["ref"])] = true
		if _, ok := obj(d)["dependsOn"].([]any); !ok {
			t.Errorf("%v: dependsOn is %T, not an array", obj(d)["ref"], obj(d)["dependsOn"])
		}
	}
	for _, v := range comps {
		c := obj(v)
		ref := str(c["bom-ref"])
		unknown := map[string]bool{}
		for _, p := range arr(c["properties"]) {
			if name := str(obj(p)["name"]); strings.HasPrefix(name, "gopmgr:unknown:") && str(obj(p)["value"]) != "" {
				unknown[strings.TrimPrefix(name, "gopmgr:unknown:")] = true
			}
		}
		present := map[string]bool{
			"name":     str(c["name"]) != "",
			"version":  str(c["version"]) != "",
			"producer": str(obj(c["manufacturer"])["name"]) != "",
			"license":  len(arr(c["licenses"])) > 0 && str(obj(arr(c["licenses"])[0])["expression"]) != "",
			"hash":     len(arr(c["hashes"])) > 0,
		}
		for field, ok := range present {
			if !ok && !unknown[field] {
				t.Errorf("%s: %s is neither present nor marked unknown", ref, field)
			}
			if ok && unknown[field] {
				t.Errorf("%s: %s is present and also marked unknown", ref, field)
			}
		}
		if str(c["purl"]) == "" && str(c["cpe"]) == "" && !hasProperty(c, "gopmgr:file-name") && !hasProperty(c, "gopmgr:git-commit") {
			t.Errorf("%s: no component identifier", ref)
		}
		for _, h := range arr(c["hashes"]) {
			if alg := str(obj(h)["alg"]); alg != "SHA-256" && alg != "SHA-512" {
				t.Errorf("%s: hash algorithm %q", ref, alg)
			}
			if !regexp.MustCompile(`^[0-9a-f]{64}([0-9a-f]{64})?$`).MatchString(str(obj(h)["content"])) {
				t.Errorf("%s: hash %q is not lowercase hex", ref, obj(h)["content"])
			}
			if !hasProperty(c, "gopmgr:hash-kind:"+strings.ToLower(str(obj(h)["alg"]))) {
				t.Errorf("%s: hash kind not stated", ref)
			}
		}
		if !deps[ref] {
			t.Errorf("%s: no dependency entry", ref)
		}
	}
	// SBOM metadata elements.
	if str(cdx["bomFormat"]) != "CycloneDX" || str(cdx["specVersion"]) != "1.7" {
		t.Errorf("data format name/version = %v %v", cdx["bomFormat"], cdx["specVersion"])
	}
	if cdx["version"] != float64(2) || !regexp.MustCompile(`^urn:uuid:[0-9a-f-]{36}$`).MatchString(str(cdx["serialNumber"])) {
		t.Errorf("SBOM version %v, serial %v", cdx["version"], cdx["serialNumber"])
	}
	if str(obj(arr(meta["authors"])[0])["name"]) != "Example Author Organization" {
		t.Errorf("SBOM author = %v", meta["authors"])
	}
	if _, err := time.Parse(time.RFC3339, str(meta["timestamp"])); err != nil {
		t.Errorf("timestamp: %v", err)
	}
	if str(obj(arr(meta["lifecycles"])[0])["phase"]) != "build" {
		t.Errorf("generation context = %v", meta["lifecycles"])
	}
	tool := obj(arr(obj(meta["tools"])["components"])[0])
	if str(tool["name"]) != toolName || str(tool["version"]) != "0123456" {
		t.Errorf("tool = %v", tool)
	}
	if !hasProperty(meta, "gopmgr:signature") || !hasProperty(meta, "gopmgr:withheld") {
		t.Errorf("metadata lacks the signature or withheld statement: %v", meta["properties"])
	}
}

func hasProperty(c map[string]any, name string) bool {
	for _, p := range arr(c["properties"]) {
		if str(obj(p)["name"]) == name {
			return true
		}
	}
	return false
}

// TestSPDXHasEveryCISAElement does the same for the SPDX 3.0.1 graph.
func TestSPDXHasEveryCISAElement(t *testing.T) {
	inv, _, spdx := encodeFixture(t)
	graph := arr(spdx["@graph"])
	byID := map[string]map[string]any{}
	unknown := map[string]map[string]bool{}
	licensed := map[string]bool{}
	depends := map[string]bool{}
	var pkgs []map[string]any
	var creation, document, sbom map[string]any
	for _, v := range graph {
		e := obj(v)
		byID[str(e["spdxId"])] = e
		switch str(e["type"]) {
		case "software_Package":
			pkgs = append(pkgs, e)
		case "CreationInfo":
			creation = e
		case "SpdxDocument":
			document = e
		case "software_Sbom":
			sbom = e
		case "Annotation":
			if field, ok := strings.CutPrefix(str(e["statement"]), "gopmgr:unknown:"); ok {
				subject := str(e["subject"])
				if unknown[subject] == nil {
					unknown[subject] = map[string]bool{}
				}
				unknown[subject][field[:strings.Index(field, ":")]] = true
			}
		case "Relationship", "LifecycleScopedRelationship":
			switch str(e["relationshipType"]) {
			case "hasDeclaredLicense":
				licensed[str(e["from"])] = true
			case "dependsOn":
				depends[str(e["from"])] = true
				for _, to := range arr(e["to"]) {
					depends[str(to)] = true
				}
			}
		}
	}
	if len(pkgs) != len(inv.Components) {
		t.Fatalf("SPDX has %d packages, inventory %d", len(pkgs), len(inv.Components))
	}
	for _, p := range pkgs {
		id, name := str(p["spdxId"]), str(p["name"])
		present := map[string]bool{
			"version":  str(p["software_packageVersion"]) != "",
			"producer": len(arr(p["originatedBy"])) > 0,
			"license":  licensed[id],
			"hash":     len(arr(p["verifiedUsing"])) > 0,
		}
		for field, ok := range present {
			if !ok && !unknown[id][field] {
				t.Errorf("%s: %s is neither present nor marked unknown", name, field)
			}
		}
		if name == "" {
			t.Errorf("%s: no name", id)
		}
		if str(p["software_packageUrl"]) == "" && len(arr(p["externalIdentifier"])) == 0 {
			t.Errorf("%s: no component identifier", name)
		}
		for _, o := range arr(p["originatedBy"]) {
			if str(byID[str(o)]["name"]) == "" {
				t.Errorf("%s: producer agent has no name", name)
			}
		}
		if !depends[id] {
			t.Errorf("%s: in no dependency relationship", name)
		}
	}
	if str(creation["specVersion"]) != "3.0.1" {
		t.Errorf("data format version = %v", creation["specVersion"])
	}
	if !strings.HasPrefix(str(spdx["@context"]), "https://spdx.org/rdf/3.0.1/") {
		t.Errorf("data format name = %v", spdx["@context"])
	}
	if _, err := time.Parse(time.RFC3339, str(creation["created"])); err != nil {
		t.Errorf("timestamp: %v", err)
	}
	if str(byID[str(arr(creation["createdBy"])[0])]["name"]) != "Example Author Organization" {
		t.Error("SBOM author missing")
	}
	if tool := str(byID[str(arr(creation["createdUsing"])[0])]["name"]); tool != toolName+" 0123456" {
		t.Errorf("tool = %q", tool)
	}
	if !reflect.DeepEqual(sbom["software_sbomType"], []any{"build"}) {
		t.Errorf("generation context = %v", sbom["software_sbomType"])
	}
	var version, serial bool
	for _, x := range arr(document["externalIdentifier"]) {
		switch str(obj(x)["comment"]) {
		case "SBOM version":
			version = str(obj(x)["identifier"]) == "2"
		case "SBOM serial number":
			serial = strings.HasPrefix(str(obj(x)["identifier"]), "urn:uuid:")
		}
	}
	if !version || !serial {
		t.Errorf("SBOM version or serial not machine-readable: %v", document["externalIdentifier"])
	}
}

// TestInventoryContents pins what the fixture build must list, and the hash
// rules: executable hashes only for the installer and executable, archive
// hashes labelled as such, no go.sum value in a hash field.
func TestInventoryContents(t *testing.T) {
	inv, err := assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	byRef := map[string]component{}
	for _, c := range inv.Components {
		byRef[c.Ref] = c
		for _, h := range c.Hashes {
			if strings.HasPrefix(h.Hex, "h1:") || (h.Alg != "sha-256" && h.Alg != "sha-512") {
				t.Errorf("%s: bad hash %+v", c.Ref, h)
			}
		}
	}
	want := []string{"app", "executable", "go:std", "go:example.com/known@v1.0.0", "go:example.com/nameless@v1.0.0", "go:example.com/local@v1.0.0",
		"native:example.com/local:Native Library", "npm:chart.js@4.6.1", "npm:@scope/bare@0.1.0", "file:assets/Example-Regular.ttf", "platform:darwin:0"}
	var got []string
	for _, c := range inv.Components {
		got = append(got, c.Ref)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("components = %v, want %v", got, want)
	}
	if _, ok := byRef["native:example.com/removed:Removed Native"]; ok {
		t.Error("a native entry whose parent module is not linked was listed")
	}
	if h := byRef["app"].Hashes; len(h) != 1 || h[0].Kind != hashInstaller {
		t.Errorf("app hashes = %+v", h)
	}
	if h := byRef["executable"].Hashes; len(h) != 1 || h[0].Kind != hashExecutable {
		t.Errorf("executable hashes = %+v", h)
	}
	if h := byRef["go:example.com/known@v1.0.0"].Hashes; len(h) != 1 || h[0].Kind != hashModuleZip {
		t.Errorf("module hashes = %+v", h)
	}
	if h := byRef["npm:chart.js@4.6.1"].Hashes; len(h) != 1 || h[0].Alg != "sha-512" || h[0].Hex != strings.Repeat("01", 64) || h[0].Kind != hashNPMTarball {
		t.Errorf("npm hashes = %+v", h)
	}
	local := byRef["go:example.com/local@v1.0.0"]
	if len(local.Hashes) != 0 || local.Unknown["hash"] == "" {
		t.Errorf("local replacement must have no hash and say why: %+v", local)
	}
	if why := byRef["native:example.com/local:Native Library"].Unknown["hash"]; !strings.Contains(why, "repository") {
		t.Errorf("native code in a local module claims an archive hash covers it: %q", why)
	}
	bare := byRef["npm:@scope/bare@0.1.0"]
	if bare.Unknown["license"] == "" || bare.Unknown["producer"] == "" || bare.Unknown["hash"] == "" {
		t.Errorf("unlicensed npm package unknowns = %v", bare.Unknown)
	}
	if byRef["platform:darwin:0"].Shipped {
		t.Error("platform component marked shipped")
	}
	if p := byRef["npm:@scope/bare@0.1.0"].Identifiers[0].Value; p != "pkg:npm/%40scope/bare@0.1.0" {
		t.Errorf("scoped npm purl = %q", p)
	}
	edges := map[string][]string{}
	for k, v := range inv.Deps {
		edges[k] = uniqueSorted(v)
	}
	if want := []string{"go:example.com/nameless@v1.0.0", "go:std"}; !reflect.DeepEqual(edges["go:example.com/known@v1.0.0"], want) {
		t.Errorf("known module depends on %v, want %v", edges["go:example.com/known@v1.0.0"], want)
	}
	if want := []string{"npm:@scope/bare@0.1.0"}; !reflect.DeepEqual(edges["npm:chart.js@4.6.1"], want) {
		t.Errorf("chart.js depends on %v, want %v", edges["npm:chart.js@4.6.1"], want)
	}
}

// TestAssembleRefusesDisagreeingSources: the SBOM is not written when its
// sources contradict each other or something shipped is unreviewed.
func TestAssembleRefusesDisagreeingSources(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*buildInput)
		want   string
	}{
		{"binary links a module go list does not", func(in *buildInput) {
			in.Binary.Deps = append(in.Binary.Deps, &debug.Module{Path: "example.com/extra", Version: "v1.0.0", Sum: "h1:x="})
		}, "is linked but not listed"},
		{"go list has a module the binary lacks", func(in *buildInput) { in.Binary.Deps = in.Binary.Deps[:2] }, "is listed but not linked"},
		{"versions differ", func(in *buildInput) {
			in.Binary.Deps[0] = &debug.Module{Path: "example.com/known", Version: "v1.0.1", Sum: "h1:k="}
		}, "linked at v1.0.1 but listed at v1.0.0"},
		{"binary for another target", func(in *buildInput) { in.Target = target{GOOS: "linux", GOARCH: "amd64"} }, "was built for darwin/arm64"},
		{"unreviewed module", func(in *buildInput) { delete(in.Reviewed.GoModules, "example.com/nameless") }, "example.com/nameless has no reviewed entry"},
		{"replaced by a published module", func(in *buildInput) {
			in.Binary.Deps[2].Replace = &debug.Module{Path: "example.com/fork", Version: "v1.0.0"}
		}, "replaced by a published module"},
		{"npm version mismatch", func(in *buildInput) { in.Frontend[0].Version = "4.6.0" }, "does not match package-lock.json"},
		{"module archive mismatch", func(in *buildInput) {
			in.ModuleArchive = func(string, string, string) (string, error) { return "", os.ErrNotExist }
		}, "file does not exist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixtureInput(t)
			tc.mutate(&in)
			if _, err := assemble(in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("assemble = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

// TestCheckReviewedFindsGaps covers the review gate, including the removal
// of a module that carries native code (as DuckDB's removal will be): the
// native entry then becomes stale and the check says so.
func TestCheckReviewedFindsGaps(t *testing.T) {
	ok := func() (map[string]map[string]string, []embeddedFile, []frontendPackage, map[string]lockPackage) {
		return map[string]map[string]string{"darwin/arm64": {"example.com/known": "v1", "example.com/nameless": "v1", "example.com/local": "v1", "example.com/removed": "v1"}},
			[]embeddedFile{{"assets/A.ttf", 0}}, []frontendPackage{{"chart.js", "4.6.1"}}, map[string]lockPackage{"node_modules/chart.js": {Version: "4.6.1", License: "MIT"}}
	}
	r := fixtureReviewedValue(t)
	r.GoModules["example.com/removed"] = reviewedEntry{License: "MIT"}
	l, e, f, lock := ok()
	pk := map[string]bool{"example.com/known/winonly": true}
	if err := checkReviewed(r, l, pk, e, f, lock); err != nil {
		t.Fatalf("a complete review failed: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(r *reviewed, l map[string]map[string]string, e *[]embeddedFile, f *[]frontendPackage, lock map[string]lockPackage)
		want   string
	}{
		{"new module", func(_ *reviewed, l map[string]map[string]string, _ *[]embeddedFile, _ *[]frontendPackage, _ map[string]lockPackage) {
			l["darwin/arm64"]["example.com/new"] = "v1"
		}, "example.com/new has no reviewed entry"},
		{"module with native code removed", func(_ *reviewed, l map[string]map[string]string, _ *[]embeddedFile, _ *[]frontendPackage, _ map[string]lockPackage) {
			delete(l["darwin/arm64"], "example.com/removed")
		}, "Removed Native names parent module example.com/removed, which no release target links"},
		{"platform-only package no longer linked", func(_ *reviewed, _ map[string]map[string]string, _ *[]embeddedFile, _ *[]frontendPackage, _ map[string]lockPackage) {
		}, "Vendor Bootstrapper names parent package example.com/known/winonly, which no release target links"},
		{"embedded entry unused", func(_ *reviewed, _ map[string]map[string]string, e *[]embeddedFile, _ *[]frontendPackage, _ map[string]lockPackage) {
			*e = nil
		}, "assets/*.ttf matches no file"},
		{"bundle and lockfile disagree", func(_ *reviewed, _ map[string]map[string]string, _ *[]embeddedFile, f *[]frontendPackage, _ map[string]lockPackage) {
			(*f)[0].Version = "9"
		}, "does not match package-lock.json"},
		{"unlicensed npm package", func(_ *reviewed, _ map[string]map[string]string, _ *[]embeddedFile, _ *[]frontendPackage, lock map[string]lockPackage) {
			lock["node_modules/chart.js"] = lockPackage{Version: "4.6.1"}
		}, "declares no license"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixtureReviewedValue(t)
			r.GoModules["example.com/removed"] = reviewedEntry{License: "MIT"}
			l, e, f, lock := ok()
			pk := map[string]bool{"example.com/known/winonly": true}
			if tc.name == "platform-only package no longer linked" {
				pk = map[string]bool{}
			}
			tc.mutate(r, l, &e, &f, lock)
			if err := checkReviewed(r, l, pk, e, f, lock); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("checkReviewed = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestEmbeddedFilesNeedExactlyOneEntry(t *testing.T) {
	repo := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("assets/A.ttf")
	write("assets/README.md")
	r := fixtureReviewedValue(t)
	files, err := embeddedFiles(repo, r)
	if err != nil || len(files) != 1 || files[0].Rel != "assets/A.ttf" {
		t.Fatalf("embeddedFiles = %v, %v", files, err)
	}
	write("assets/unreviewed.woff2")
	if _, err := embeddedFiles(repo, r); err == nil || !strings.Contains(err.Error(), "unreviewed.woff2 has no reviewed entry") {
		t.Fatalf("an unreviewed file was accepted: %v", err)
	}
	os.Remove(filepath.Join(repo, "assets", "unreviewed.woff2"))
	r.Embedded = append(r.Embedded, r.Embedded[0])
	if _, err := embeddedFiles(repo, r); err == nil || !strings.Contains(err.Error(), "matches two reviewed entries") {
		t.Fatalf("a doubly matched file was accepted: %v", err)
	}
}

func TestLicenseExpressionSyntax(t *testing.T) {
	for _, ok := range []string{"MIT", "GPL-3.0-or-later", "MIT AND BSD-3-Clause AND blessing AND Unlicense", "(MIT OR Apache-2.0) AND BSD-2-Clause", "GPL-2.0-only WITH Classpath-exception-2.0", "LicenseRef-Example", "Apache-2.0+"} {
		if err := checkLicenseExpression(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "MIT AND", "AND MIT", "(MIT", "MIT)", "MIT OR OR BSD", "MIT/Apache-2.0", "MIT, BSD", "WITH MIT"} {
		if err := checkLicenseExpression(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestPackageURLs(t *testing.T) {
	for in, want := range map[string]string{
		purlGo("git.sr.ht/~jackmordaunt/go-toast/v2", "v2.0.3"):               "pkg:golang/git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3",
		purlGo("github.com/Microsoft/go-winio", "v0.6.4"):                     "pkg:golang/github.com/microsoft/go-winio@v0.6.4",
		purlGo("github.com/xuri/nfp", "v0.0.2-0.20250530014748-2ddeb826f9a9"): "pkg:golang/github.com/xuri/nfp@v0.0.2-0.20250530014748-2ddeb826f9a9",
		purlGo("example.com/m", "v1.0.0+incompatible"):                        "pkg:golang/example.com/m@v1.0.0%2Bincompatible",
		purlNPM("@kurkle/color", "0.3.4"):                                     "pkg:npm/%40kurkle/color@0.3.4",
	} {
		if in != want {
			t.Errorf("purl = %q, want %q", in, want)
		}
	}
}

func TestSerialNumbersAreVersion4UUIDs(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 64 {
		u, err := newUUID()
		if err != nil || !re.MatchString(u) || seen[u] {
			t.Fatalf("newUUID = %q, %v", u, err)
		}
		seen[u] = true
	}
}

// TestOutputIsDeterministic: the same inventory and metadata give the same
// bytes, so a reissued SBOM differs only where its metadata does.
func TestOutputIsDeterministic(t *testing.T) {
	inv, err := assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, enc := range []func(*inventory, docMeta) ([]byte, error){encodeCycloneDX, encodeSPDX} {
		a, err := enc(inv, fixtureMeta())
		if err != nil {
			t.Fatal(err)
		}
		inv2, _ := assemble(fixtureInput(t))
		b, _ := enc(inv2, fixtureMeta())
		if !bytes.Equal(a, b) {
			t.Fatal("two encodings of the same inventory differ")
		}
	}
}

func TestSignAndVerify(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "sbom.key")
	pubText, err := generateKey(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(keyFile); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode: %v %v", fi, err)
	}
	if _, err := generateKey(keyFile); err == nil {
		t.Fatal("keygen overwrote an existing private key")
	}
	pubFile := filepath.Join(dir, "signing-key.pub")
	if err := os.WriteFile(pubFile, []byte(pubText), 0o644); err != nil {
		t.Fatal(err)
	}
	pub, err := readPublicKey(pubFile)
	if err != nil {
		t.Fatal(err)
	}
	privText, _ := os.ReadFile(keyFile)
	priv, err := decodeKey(string(privText), 64, "key")
	if err != nil {
		t.Fatal(err)
	}
	sbom := filepath.Join(dir, "a.cdx.json")
	if err := os.WriteFile(sbom, []byte(`{"signature":"by key `+keyID(pub)+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := signFiles(priv, pub, []string{sbom}); err != nil {
		t.Fatal(err)
	}
	if err := verifyFiles(pub, []string{sbom}); err != nil {
		t.Fatalf("a fresh signature did not verify: %v", err)
	}
	// Any changed byte fails.
	b, _ := os.ReadFile(sbom)
	b[2] ^= 1
	os.WriteFile(sbom, b, 0o644)
	if err := verifyFiles(pub, []string{sbom}); err == nil {
		t.Fatal("a tampered SBOM verified")
	}
	b[2] ^= 1
	os.WriteFile(sbom, b, 0o644)
	// Another key neither signs nor verifies.
	otherPubText, err := generateKey(filepath.Join(dir, "other.key"))
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _ := decodeKey(otherPubText, 32, "pub")
	if err := verifyFiles(otherPub, []string{sbom}); err == nil {
		t.Fatal("a signature verified with the wrong key")
	}
	if err := signFiles(priv, otherPub, []string{sbom}); err == nil || !strings.Contains(err.Error(), "does not match the committed public key") {
		t.Fatalf("signed with a key other than the committed one: %v", err)
	}
	// An SBOM that does not name the key fails even with a valid signature.
	plain := filepath.Join(dir, "b.spdx.json")
	os.WriteFile(plain, []byte(`{}`), 0o644)
	if err := signFiles(priv, pub, []string{plain}); err != nil {
		t.Fatal(err)
	}
	if err := verifyFiles(pub, []string{plain}); err == nil || !strings.Contains(err.Error(), "does not name key") {
		t.Fatalf("an SBOM that does not name its key verified: %v", err)
	}
}

// TestReleaseTargetsMatchModuleNotices keeps the two tools' target lists equal.
func TestReleaseTargetsMatchModuleNotices(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "module-notices", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\{GOOS: "([a-z0-9]+)", GOARCH: "([a-z0-9]+)", Tags: "([a-z0-9_,]+)"\}`)
	var theirs, ours []string
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		theirs = append(theirs, m[1]+"/"+m[2]+" "+m[3])
	}
	for _, tg := range releaseTargets {
		ours = append(ours, tg.String()+" "+tg.Tags)
	}
	if len(theirs) == 0 || !reflect.DeepEqual(theirs, ours) {
		t.Fatalf("tools/sbom targets %v, tools/module-notices targets %v", ours, theirs)
	}
}

// TestReviewedMetadataIsValid loads the committed file.
func TestReviewedMetadataIsValid(t *testing.T) {
	if _, err := loadReviewed("components.json"); err != nil {
		t.Fatal(err)
	}
}

// TestSignatureStatementNeverFallsBackSilently: an SBOM says "unsigned" only
// when no key is published and none was asked for.
func TestSignatureStatementNeverFallsBackSilently(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.pub")
	if s, err := signatureStatement(missing, false); err != nil || !strings.HasPrefix(s, "unsigned:") {
		t.Fatalf("default key absent = %q, %v", s, err)
	}
	if _, err := signatureStatement(missing, true); err == nil {
		t.Fatal("a requested key that is missing was treated as unsigned")
	}
	bad := filepath.Join(dir, "bad.pub")
	os.WriteFile(bad, []byte("not a key"), 0o644)
	if _, err := signatureStatement(bad, false); err == nil {
		t.Fatal("a malformed published key was treated as unsigned")
	}
	pubText, err := generateKey(filepath.Join(dir, "k"))
	if err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.pub")
	os.WriteFile(good, []byte(pubText), 0o644)
	if s, err := signatureStatement(good, true); err != nil || !strings.Contains(s, "sha-256:") {
		t.Fatalf("published key = %q, %v", s, err)
	}
	if got := inRepo("/repo", "/abs/key.pub"); got != "/abs/key.pub" {
		t.Errorf("inRepo kept an absolute path as %q", got)
	}
	if got := inRepo("/repo", "tools/sbom/x"); got != filepath.Join("/repo", "tools/sbom/x") {
		t.Errorf("inRepo relative = %q", got)
	}
}

// TestRenderFromSavedInventoryMatchesDirectGeneration: the release workflow
// records the inventory before packaging and renders after; the result must
// equal generating in one step.
func TestRenderFromSavedInventoryMatchesDirectGeneration(t *testing.T) {
	direct, err := assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	in := fixtureInput(t)
	in.Artifact, in.ArtifactHex = "", ""
	early, err := assemble(in)
	if err != nil {
		t.Fatal(err)
	}
	if early.Components[0].Unknown["hash"] == "" {
		t.Fatal("an inventory recorded before packaging claims an installer hash")
	}
	b, err := json.Marshal(early)
	if err != nil {
		t.Fatal(err)
	}
	var saved inventory
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	f := fixtureInput(t)
	saved.applyPackage(f.BinaryHex, f.Artifact, f.ArtifactHex)
	for _, enc := range []func(*inventory, docMeta) ([]byte, error){encodeCycloneDX, encodeSPDX} {
		want, _ := enc(direct, fixtureMeta())
		got, err := enc(&saved, fixtureMeta())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("rendering a saved inventory differs from direct generation")
		}
	}
}

func TestFingerprintFollowsBuildInformation(t *testing.T) {
	base := fixtureInput(t).Binary
	fp := fingerprint(base)
	for name, mutate := range map[string]func(*buildinfo.BuildInfo){
		"module version": func(b *buildinfo.BuildInfo) {
			b.Deps[0] = &debug.Module{Path: b.Deps[0].Path, Version: "v9.9.9", Sum: b.Deps[0].Sum}
		},
		"module sum": func(b *buildinfo.BuildInfo) {
			b.Deps[1] = &debug.Module{Path: b.Deps[1].Path, Version: b.Deps[1].Version, Sum: "h1:other="}
		},
		"toolchain": func(b *buildinfo.BuildInfo) { b.GoVersion = "go1.27.4" },
		"build setting": func(b *buildinfo.BuildInfo) {
			b.Settings = append(b.Settings, debug.BuildSetting{Key: "-tags", Value: "x"})
		},
	} {
		b := fixtureInput(t).Binary
		mutate(b)
		if fingerprint(b) == fp {
			t.Errorf("%s changed but the fingerprint did not", name)
		}
	}
	if fingerprint(fixtureInput(t).Binary) != fp {
		t.Error("the fingerprint is not stable")
	}
}

// TestPackageScopedNativeFile: a file a module carries for one platform is
// listed, hashed and tied to the installer only when that platform's build
// links the package that embeds it; its proprietary license is an explained
// unknown, not an invented identifier.
func TestPackageScopedNativeFile(t *testing.T) {
	inv, err := assemble(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range inv.Components {
		if c.Name == "Vendor Bootstrapper" {
			t.Fatal("listed for a build that does not link its package")
		}
	}
	in := fixtureInput(t)
	in.Packages = append(in.Packages, pkg("example.com/known/winonly", "example.com/known"))
	in.Packages[1].Imports = append(in.Packages[1].Imports, "example.com/known/winonly")
	inv, err = assemble(in)
	if err != nil {
		t.Fatal(err)
	}
	var boot *component
	for i := range inv.Components {
		if inv.Components[i].Name == "Vendor Bootstrapper" {
			boot = &inv.Components[i]
		}
	}
	if boot == nil {
		t.Fatal("not listed for a build that links its package")
	}
	if len(boot.Hashes) != 1 || boot.Hashes[0].Kind != hashModuleFile || boot.License != "" || !strings.Contains(boot.Unknown["license"], "own terms") {
		t.Fatalf("bootstrapper = %+v", boot)
	}
	if !slicesContain(inv.Deps[refApp], boot.Ref) || !slicesContain(inv.Deps["go:example.com/known@v1.0.0"], boot.Ref) {
		t.Fatalf("bootstrapper is not tied to both the installer and its module: %v", inv.Deps)
	}
	in.ModuleFile = func(string, string, string) (string, error) { return "", os.ErrNotExist }
	if _, err := assemble(in); err == nil {
		t.Fatal("an unhashable module file was accepted")
	}
}

func slicesContain(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestNativeLicenseNeedsExactlyOneOfLicenseAndNote(t *testing.T) {
	for _, tc := range []struct{ license, note string }{{"", ""}, {"MIT", "also a note"}} {
		r := fixtureReviewedValue(t)
		r.Native[0].License, r.Native[0].LicenseNote = tc.license, tc.note
		if err := r.validate(); err == nil {
			t.Errorf("license %q with note %q was accepted", tc.license, tc.note)
		}
	}
}

func TestUnreviewedEmbeddedBinariesAreRefused(t *testing.T) {
	in := fixtureInput(t)
	extra := pkg("example.com/known/blob", "example.com/known")
	extra.EmbedFiles = []string{"data.json", "lib/helper.DLL"}
	in.Packages = append(in.Packages, extra)
	if _, err := assemble(in); err == nil || !strings.Contains(err.Error(), "example.com/known/blob embeds lib/helper.DLL") {
		t.Fatalf("assemble = %v, want the unreviewed DLL named", err)
	}
	reviewedOne := pkg("example.com/known/winonly", "example.com/known")
	reviewedOne.EmbedFiles = []string{"bin/setup.exe"}
	if got := unreviewedEmbeddedBinaries([]goPackage{reviewedOne}, fixtureReviewedValue(t)); len(got) != 0 {
		t.Fatalf("a reviewed embedded binary was reported: %v", got)
	}
}

// TestMainModuleEmbedsMustBeReviewed: a file GoPMgr embeds outside the
// reviewed third-party roots and the declared first-party folders stops the
// SBOM.
func TestMainModuleEmbedsMustBeReviewed(t *testing.T) {
	in := fixtureInput(t)
	in.Reviewed.FirstPartyEmbedded = []string{"internal/templates"}
	main := pkg("gopmgr/internal/x", "")
	main.Dir = filepath.Join(in.Repo, "internal", "x")
	main.EmbedFiles = []string{"vendor-cert.pem"}
	ok := pkg("gopmgr/internal/templates", "")
	ok.Dir = filepath.Join(in.Repo, "internal", "templates")
	ok.EmbedFiles = []string{"seeds.json"}
	fonts := pkg("gopmgr/assetsfs", "")
	fonts.Dir = in.Repo
	fonts.EmbedFiles = []string{"assets/Example-Regular.ttf"}
	got, err := unreviewedMainEmbeds(in.Repo, []goPackage{main, ok, fonts}, in.Reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gopmgr/internal/x embeds internal/x/vendor-cert.pem"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unreviewedMainEmbeds = %v, want %v", got, want)
	}
	in.Packages = append(in.Packages, main)
	if _, err := assemble(in); err == nil || !strings.Contains(err.Error(), "vendor-cert.pem") {
		t.Fatalf("assemble = %v, want the unreviewed file named", err)
	}
	if underAny("internal/templatesX/a", []string{"internal/templates"}) {
		t.Error("a sibling folder counted as inside a reviewed folder")
	}
}
