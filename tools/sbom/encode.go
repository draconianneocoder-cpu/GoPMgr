// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// docMeta is the SBOM metadata shared by both formats.
type docMeta struct {
	Author      string
	Timestamp   string // RFC 3339, UTC
	ToolName    string
	ToolVersion string // "" means unknown
	SBOMVersion int
	Serial      string // RFC 9562 UUID
	Context     string // lifecycle phase: build
	Signature   string // where the detached signature is, or why there is none
}

func (m docMeta) toolVersion() string {
	if m.ToolVersion == "" {
		return "unknown"
	}
	return m.ToolVersion
}

func sortedRefs(deps map[string][]string) []string {
	out := make([]string, 0, len(deps))
	for k := range deps {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// uniqueSorted never returns nil: an empty dependency list must encode as
// [] (no dependencies), not null.
func uniqueSorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	j := 0
	for i, s := range out {
		if i == 0 || s != out[j-1] {
			out[j] = s
			j++
		}
	}
	return out[:j]
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// ---- CycloneDX 1.7 ----

type cdxProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func cdxProperties(c component) []cdxProperty {
	var ps []cdxProperty
	for _, h := range c.Hashes {
		ps = append(ps, cdxProperty{"gopmgr:hash-kind:" + h.Alg, h.Kind})
	}
	for _, id := range c.Identifiers {
		switch id.Kind {
		case "gitCommit":
			ps = append(ps, cdxProperty{"gopmgr:git-commit", id.Value})
		case "goModuleSum":
			ps = append(ps, cdxProperty{"gopmgr:go-module-sum", id.Value})
		case "file":
			ps = append(ps, cdxProperty{"gopmgr:file-name", id.Value})
		}
	}
	if !c.Shipped {
		ps = append(ps, cdxProperty{"gopmgr:shipped", "false"})
	}
	fields := make([]string, 0, len(c.Unknown))
	for f := range c.Unknown {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, f := range fields {
		ps = append(ps, cdxProperty{"gopmgr:unknown:" + f, c.Unknown[f]})
	}
	return ps
}

func cdxComponent(c component) map[string]any {
	types := map[string]string{"application": "application", "executable": "application", "library": "library", "data": "file", "platform": "platform"}
	out := map[string]any{"type": types[c.Kind], "bom-ref": c.Ref, "name": c.Name, "scope": "required"}
	if c.Version != "" {
		out["version"] = c.Version
	}
	if c.Producer != "" {
		out["manufacturer"] = map[string]any{"name": c.Producer}
	}
	if c.License != "" {
		out["licenses"] = []any{map[string]any{"expression": c.License}}
	}
	if c.Note != "" {
		out["description"] = c.Note
	}
	if len(c.Hashes) > 0 {
		var hs []any
		for _, h := range c.Hashes {
			hs = append(hs, map[string]any{"alg": strings.ToUpper(h.Alg), "content": h.Hex})
		}
		out["hashes"] = hs
	}
	for _, id := range c.Identifiers {
		switch id.Kind {
		case "purl":
			out["purl"] = id.Value
		case "cpe23":
			out["cpe"] = id.Value
		}
	}
	if ps := cdxProperties(c); len(ps) > 0 {
		out["properties"] = ps
	}
	return out
}

func encodeCycloneDX(inv *inventory, m docMeta) ([]byte, error) {
	var comps []any
	for _, c := range inv.Components[1:] {
		comps = append(comps, cdxComponent(c))
	}
	var deps []any
	for _, c := range inv.Components {
		deps = append(deps, map[string]any{"ref": c.Ref, "dependsOn": uniqueSorted(inv.Deps[c.Ref])})
	}
	doc := map[string]any{
		"$schema":      "http://cyclonedx.org/schema/bom-1.7.schema.json",
		"bomFormat":    "CycloneDX",
		"specVersion":  "1.7",
		"serialNumber": "urn:uuid:" + m.Serial,
		"version":      m.SBOMVersion,
		"metadata": map[string]any{
			"timestamp":  m.Timestamp,
			"lifecycles": []any{map[string]any{"phase": m.Context}},
			"tools": map[string]any{"components": []any{map[string]any{
				"type": "application", "name": m.ToolName, "version": m.toolVersion(),
				"manufacturer": map[string]any{"name": m.Author},
			}}},
			"authors":   []any{map[string]any{"name": m.Author}},
			"component": cdxComponent(inv.Components[0]),
			"properties": []cdxProperty{
				{"gopmgr:release-target", inv.Target.String()},
				{"gopmgr:signature", m.Signature},
				{"gopmgr:withheld", "none: the SBOM author withholds no component information"},
			},
		},
		"components":   comps,
		"dependencies": deps,
	}
	return marshal(doc)
}

// ---- SPDX 3.0.1 ----

type spdxDoc struct {
	Context string           `json:"@context"`
	Graph   []map[string]any `json:"@graph"`
}

func encodeSPDX(inv *inventory, m docMeta) ([]byte, error) {
	base := "https://github.com/draconianneocoder-cpu/GoPMgr/spdx/sbom/" + m.Serial + "/"
	doc := spdxDoc{Context: "https://spdx.org/rdf/3.0.1/spdx-context.jsonld"}
	var all []string
	ids := map[string]string{}
	add := func(kind, key string, fields map[string]any) string {
		uri := base + kind + "/" + fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
		if ids[uri] != "" {
			return uri
		}
		ids[uri] = key
		fields["type"] = kind
		fields["spdxId"] = uri
		fields["creationInfo"] = "_:creation"
		doc.Graph = append(doc.Graph, fields)
		all = append(all, uri)
		return uri
	}
	author := add("Organization", "author:"+m.Author, map[string]any{"name": m.Author})
	tool := add("Tool", "tool", map[string]any{"name": m.ToolName + " " + m.toolVersion()})
	doc.Graph = append(doc.Graph, map[string]any{"type": "CreationInfo", "@id": "_:creation", "created": m.Timestamp, "createdBy": []string{author}, "createdUsing": []string{tool}, "specVersion": "3.0.1"})
	licenses := map[string]string{}
	license := func(expr string) string {
		if licenses[expr] == "" {
			licenses[expr] = add("simplelicensing_LicenseExpression", "license:"+expr, map[string]any{"simplelicensing_licenseExpression": expr})
		}
		return licenses[expr]
	}
	agents := map[string]string{}
	agent := func(name string) string {
		if agents[name] == "" {
			agents[name] = add("Agent", "agent:"+name, map[string]any{"name": name})
		}
		return agents[name]
	}
	relate := func(from, kind string, to []string, scope, comment string) {
		fields := map[string]any{"from": from, "to": to, "relationshipType": kind, "completeness": "complete"}
		if comment != "" {
			fields["comment"] = comment
		}
		typ := "Relationship"
		if scope != "" {
			typ = "LifecycleScopedRelationship"
			fields["scope"] = scope
		}
		add(typ, from+"|"+kind+"|"+strings.Join(to, "|")+"|"+scope, fields)
	}
	algs := map[string]string{"sha-256": "sha256", "sha-384": "sha384", "sha-512": "sha512"}
	purposes := map[string]string{"application": "install", "executable": "executable", "library": "library", "data": "data", "platform": "platform"}
	elems := map[string]string{}
	var annotate []func()
	for _, c := range inv.Components {
		f := map[string]any{"name": c.Name, "software_primaryPurpose": purposes[c.Kind]}
		// Every component is a software_Package: a software_File has no
		// version field, and embedded files and the executable have versions.
		kind := "software_Package"
		if c.Version != "" {
			f["software_packageVersion"] = c.Version
		}
		if c.Producer != "" {
			f["originatedBy"] = []string{agent(c.Producer)}
		}
		if c.Note != "" {
			f["description"] = c.Note
		}
		var vu []any
		for _, h := range c.Hashes {
			vu = append(vu, map[string]any{"type": "Hash", "algorithm": algs[h.Alg], "hashValue": h.Hex, "comment": "hash of the " + h.Kind})
		}
		if len(vu) > 0 {
			f["verifiedUsing"] = vu
		}
		var ext []any
		for _, id := range c.Identifiers {
			switch id.Kind {
			case "purl":
				f["software_packageUrl"] = id.Value
			case "cpe23":
				ext = append(ext, map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "cpe23", "identifier": id.Value})
			case "gitCommit":
				ext = append(ext, map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "other", "identifier": id.Value, "comment": "git commit of the source"})
			case "goModuleSum":
				ext = append(ext, map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "other", "identifier": id.Value, "comment": "Go module checksum (go.sum h1)"})
			case "file":
				ext = append(ext, map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "other", "identifier": id.Value, "comment": "file name"})
			}
		}
		if len(ext) > 0 {
			f["externalIdentifier"] = ext
		}
		id := add(kind, "component:"+c.Ref, f)
		elems[c.Ref] = id
		if c.License != "" {
			relate(id, "hasDeclaredLicense", []string{license(c.License)}, "", "")
		}
		if !c.Shipped {
			unk := "not shipped with GoPMgr"
			annotate = append(annotate, func() {
				add("Annotation", "shipped:"+c.Ref, map[string]any{"annotationType": "other", "subject": id, "statement": "gopmgr:shipped=false: " + unk})
			})
		}
		fields := make([]string, 0, len(c.Unknown))
		for k := range c.Unknown {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		for _, k := range fields {
			why := c.Unknown[k]
			field := k
			annotate = append(annotate, func() {
				add("Annotation", "unknown:"+c.Ref+":"+field, map[string]any{"annotationType": "other", "subject": id, "statement": "gopmgr:unknown:" + field + ": " + why})
			})
		}
	}
	for _, from := range sortedRefs(inv.Deps) {
		var runtime, build []string
		for _, to := range uniqueSorted(inv.Deps[from]) {
			if strings.HasPrefix(to, "platform:") {
				runtime = append(runtime, elems[to])
			} else {
				build = append(build, elems[to])
			}
		}
		if len(build) > 0 {
			relate(elems[from], "dependsOn", build, "", "")
		}
		if len(runtime) > 0 {
			relate(elems[from], "dependsOn", runtime, "runtime", "provided by the platform, not shipped")
		}
	}
	for _, f := range annotate {
		f()
	}
	sbom := add("software_Sbom", "sbom", map[string]any{
		"name":              m.ToolName + " SBOM for " + inv.Components[0].Name + " (" + inv.Target.String() + ")",
		"software_sbomType": []string{m.Context},
		"rootElement":       []string{elems[refApp]},
	})
	add("Annotation", "withheld", map[string]any{"annotationType": "other", "subject": sbom, "statement": "gopmgr:withheld=none: the SBOM author withholds no component information"})
	add("Annotation", "signature", map[string]any{"annotationType": "other", "subject": sbom, "statement": "gopmgr:signature: " + m.Signature})
	for _, n := range doc.Graph {
		if n["spdxId"] == sbom {
			var els []string
			for _, e := range all {
				if e != sbom {
					els = append(els, e)
				}
			}
			n["element"] = els
		}
	}
	cc0 := license("CC0-1.0")
	add("SpdxDocument", "document", map[string]any{
		"name":               inv.Components[0].Name + " SBOM, version " + strconv.Itoa(m.SBOMVersion),
		"dataLicense":        cc0,
		"profileConformance": []string{"core", "software", "simpleLicensing"},
		"rootElement":        []string{sbom},
		"element":            append([]string(nil), all...),
		"externalIdentifier": []any{
			map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "other", "identifier": "urn:uuid:" + m.Serial, "comment": "SBOM serial number"},
			map[string]any{"type": "ExternalIdentifier", "externalIdentifierType": "other", "identifier": strconv.Itoa(m.SBOMVersion), "comment": "SBOM version"},
		},
	})
	return marshal(doc)
}
