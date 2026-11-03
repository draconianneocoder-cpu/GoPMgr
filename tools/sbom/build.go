// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"path"
	"runtime/debug"
	"sort"
	"strings"
)

const (
	refApp        = "app"
	refExecutable = "executable"
	refStd        = "go:std"
)

func refGo(mod, version string) string { return "go:" + mod + "@" + version }

// buildInput is everything one inventory is made from.
type buildInput struct {
	Repo        string
	Target      target
	Reviewed    *reviewed
	Binary      *buildinfo.BuildInfo
	BinaryHex   string // SHA-256 of the executable
	Artifact    string // installer file name, "" if not built
	ArtifactHex string
	Version     string // release version, e.g. 1.2.0
	Commit      string // full source commit
	Packages    []goPackage
	Frontend    []frontendPackage
	Lock        map[string]lockPackage
	Embedded    []embeddedFile
	EmbeddedHex map[string]string
	// moduleArchive is moduleArchive, replaceable in tests.
	ModuleArchive func(mod, version, sum string) (string, error)
	// ModuleFile hashes a file inside a module, replaceable in tests.
	ModuleFile func(mod, version, rel string) (string, error)
}

// assemble builds the inventory and fails when the sources disagree: the
// binary's module set against `go list`, the binary's target against the
// requested one, the bundle list against the lockfile, and any shipped part
// without a reviewed entry.
func assemble(in buildInput) (*inventory, error) {
	r := in.Reviewed
	if goos, goarch := setting(in.Binary, "GOOS"), setting(in.Binary, "GOARCH"); goos != in.Target.GOOS || goarch != in.Target.GOARCH {
		return nil, fmt.Errorf("the executable was built for %s/%s, not %s", goos, goarch, in.Target)
	}
	inv := &inventory{Target: in.Target, Deps: map[string][]string{}, Fingerprint: fingerprint(in.Binary)}

	app := component{Ref: refApp, Kind: "application", Name: r.App.Name, Version: in.Version, Producer: r.App.Producer, License: r.App.License, Shipped: true}
	if in.Version == "" {
		app.unknown("version", "no release version was given")
	} else {
		app.Identifiers = append(app.Identifiers, identifier{"purl", r.App.PurlBase + "@" + purlEscape("v"+in.Version)})
	}
	if in.Commit != "" {
		app.Identifiers = append(app.Identifiers, identifier{"gitCommit", in.Commit})
	}
	app.unknown("hash", "the installer was not built when this SBOM was generated")
	inv.add(app)

	exe := component{Ref: refExecutable, Kind: "executable", Name: r.App.Name + " executable (" + in.Target.String() + ")", Version: in.Version, Producer: r.App.Producer, License: r.App.License, Shipped: true}
	if in.Version == "" {
		exe.unknown("version", "no release version was given")
	}
	if in.Commit != "" {
		exe.Identifiers = append(exe.Identifiers, identifier{"gitCommit", in.Commit})
	}
	inv.add(exe)
	inv.depend(refApp, refExecutable)

	// Go: the binary says what was linked; `go list` says who imports whom.
	listed, edges := moduleGraph(in.Packages)
	linked := map[string]*debug.Module{}
	for _, m := range in.Binary.Deps {
		linked[m.Path] = m
	}
	if err := sameModules(linked, listed); err != nil {
		return nil, err
	}
	if bins := unreviewedEmbeddedBinaries(in.Packages, r); len(bins) > 0 {
		return nil, errors.New("unreviewed binaries embedded by dependencies: " + strings.Join(bins, "; "))
	}
	mainEmbeds, err := unreviewedMainEmbeds(in.Repo, in.Packages, r)
	if err != nil {
		return nil, err
	}
	if len(mainEmbeds) > 0 {
		return nil, errors.New("GoPMgr embeds files outside the reviewed folders: " + strings.Join(mainEmbeds, "; "))
	}
	goVersion := strings.TrimPrefix(in.Binary.GoVersion, "go")
	inv.add(component{Ref: refStd, Kind: "library", Name: "Go standard library", Version: goVersion, Producer: r.GoStdlib.Producer, License: r.GoStdlib.License, Shipped: true,
		Identifiers: []identifier{{"purl", "pkg:golang/stdlib@" + purlEscape(goVersion)}},
		Note:        "Compiled into the executable by the Go toolchain " + in.Binary.GoVersion})
	inv.Components[len(inv.Components)-1].unknown("hash", "the standard library is compiled from the Go toolchain's sources, not from a separate archive")
	paths := make([]string, 0, len(linked))
	for p := range linked {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	refs := map[string]string{"": refExecutable, "std": refStd}
	for _, p := range paths {
		m := linked[p]
		e, ok := r.GoModules[p]
		if !ok {
			return nil, fmt.Errorf("Go module %s has no reviewed entry in tools/sbom/components.json", p)
		}
		c := component{Ref: refGo(p, m.Version), Kind: "library", Name: p, Version: m.Version, Producer: e.Producer, License: e.License, Shipped: true,
			Identifiers: []identifier{{"purl", purlGo(p, m.Version)}}}
		switch {
		case m.Replace != nil && isLocalReplacement(m.Replace):
			// A local directory replacement: no published archive to hash.
			c.Note = e.LocalNote
			c.unknown("hash", "built from a directory in the GoPMgr repository, not from a published module archive")
			if in.Commit != "" {
				c.Identifiers = append(c.Identifiers, identifier{"gitCommit", in.Commit})
			}
		case m.Replace != nil:
			return nil, fmt.Errorf("Go module %s is replaced by a published module, which tools/sbom does not model", p)
		default:
			c.Identifiers = append(c.Identifiers, identifier{"goModuleSum", m.Sum})
			sum, err := in.ModuleArchive(p, m.Version, m.Sum)
			if err != nil {
				return nil, err
			}
			c.Hashes = []hash{{"sha-256", sum, hashModuleZip}}
		}
		if c.Producer == "" {
			c.unknown("producer", "the module's license file names no copyright holder")
		}
		inv.add(c)
		refs[p] = c.Ref
	}
	for from, tos := range edges {
		var list []string
		for to := range tos {
			list = append(list, refs[to])
		}
		sort.Strings(list)
		inv.depend(refs[from], list...)
	}

	// Native code carried inside a linked module.
	packages := map[string]bool{}
	for _, p := range in.Packages {
		packages[p.ImportPath] = true
	}
	for _, n := range r.Native {
		parent, ok := linked[n.ParentModule]
		if !ok || (n.ParentPackage != "" && !packages[n.ParentPackage]) {
			continue
		}
		c := component{Ref: "native:" + n.ParentModule + ":" + n.Name, Kind: "library", Name: n.Name, Version: n.Version, Producer: n.Producer, License: n.License, Shipped: true, Note: n.Note}
		if n.LicenseNote != "" {
			c.unknown("license", n.LicenseNote)
		}
		if n.Purl != "" {
			c.Identifiers = append(c.Identifiers, identifier{"purl", n.Purl})
		}
		if n.CPE != "" {
			c.Identifiers = append(c.Identifiers, identifier{"cpe23", n.CPE})
		}
		if n.Commit != "" {
			c.Identifiers = append(c.Identifiers, identifier{"gitCommit", n.Commit})
		}
		if n.Version == "" {
			c.unknown("version", "the copy GoPMgr builds carries no release version")
		}
		if n.Producer == "" {
			c.unknown("producer", "the component's notice names no single producer")
		}
		switch {
		case n.ModuleFile != "":
			sum, err := in.ModuleFile(n.ParentModule, parent.Version, n.ModuleFile)
			if err != nil {
				return nil, err
			}
			c.Hashes = []hash{{"sha-256", sum, hashModuleFile}}
			c.Identifiers = append(c.Identifiers, identifier{"file", path.Base(n.ModuleFile)})
		case parent.Replace != nil && isLocalReplacement(parent.Replace):
			c.unknown("hash", "compiled from the GoPMgr repository's copy of its parent Go module; no separate archive exists")
		default:
			c.unknown("hash", "linked as part of its parent Go module, whose archive hash covers it")
		}
		inv.add(c)
		inv.depend(refGo(n.ParentModule, parent.Version), c.Ref)
		if n.InInstaller {
			inv.depend(refApp, c.Ref)
		}
	}

	// Frontend bundle.
	byName := map[string]string{}
	for _, f := range in.Frontend {
		lp, ok := in.Lock["node_modules/"+f.Name]
		if !ok || lp.Version != f.Version {
			return nil, fmt.Errorf("bundled npm package %s %s does not match package-lock.json (%q)", f.Name, f.Version, lp.Version)
		}
		e := r.NPM[f.Name]
		c := component{Ref: "npm:" + f.Name + "@" + f.Version, Kind: "library", Name: f.Name, Version: f.Version, Producer: e.Producer, License: lp.License, Shipped: true,
			Identifiers: []identifier{{"purl", purlNPM(f.Name, f.Version)}},
			Note:        "Code from this package is in the web interface bundle embedded in the executable"}
		if e.License != "" {
			c.License = e.License
		}
		if c.License == "" {
			c.unknown("license", "package-lock.json declares no license")
		} else if err := checkLicenseExpression(c.License); err != nil {
			return nil, fmt.Errorf("npm package %s: %w", f.Name, err)
		}
		if c.Producer == "" {
			c.Producer = npmAuthor(in.Repo, f.Name)
		}
		if c.Producer == "" {
			c.unknown("producer", "the package declares no author")
		}
		if lp.Integrity == "" {
			c.unknown("hash", "package-lock.json records no integrity value")
		} else {
			h, err := integrityHash(lp.Integrity)
			if err != nil {
				return nil, fmt.Errorf("npm package %s: %w", f.Name, err)
			}
			c.Hashes = []hash{h}
		}
		inv.add(c)
		byName[f.Name] = c.Ref
	}
	for _, f := range in.Frontend {
		inv.depend(refExecutable, byName[f.Name])
		var list []string
		for dep := range in.Lock["node_modules/"+f.Name].Dependencies {
			if ref, ok := byName[dep]; ok {
				list = append(list, ref)
			}
		}
		sort.Strings(list)
		inv.depend(byName[f.Name], list...)
	}

	// Embedded third-party files.
	for _, f := range in.Embedded {
		e := r.Embedded[f.Entry]
		c := component{Ref: "file:" + f.Rel, Kind: "data", Name: e.Name, Version: e.Version, Producer: e.Producer, License: e.License, Shipped: true,
			Identifiers: []identifier{{"file", path.Base(f.Rel)}},
			Hashes:      []hash{{"sha-256", in.EmbeddedHex[f.Rel], hashFile}},
			Note:        "Embedded in the executable from " + f.Rel}
		if e.Version == "" {
			c.unknown("version", "the repository does not record which upstream release this file came from")
		}
		inv.add(c)
		inv.depend(refExecutable, c.Ref)
	}

	// Platform components the application needs but does not ship.
	for i, e := range r.Platform[in.Target.GOOS] {
		c := component{Ref: fmt.Sprintf("platform:%s:%d", in.Target.GOOS, i), Kind: "platform", Name: e.Name, Version: e.Version, Producer: e.Producer, License: e.License, Shipped: false, Note: e.Note}
		if e.Purl == "" {
			return nil, fmt.Errorf("platform component %s has no package URL", e.Name)
		}
		c.Identifiers = append(c.Identifiers, identifier{"purl", e.Purl})
		if e.Version == "" {
			c.unknown("version", "provided by the operating system or its package manager; the version depends on the installation")
		}
		if e.License == "" {
			c.unknown("license", "not shipped with GoPMgr; licensed by its producer")
		}
		c.unknown("hash", "not shipped with GoPMgr")
		inv.add(c)
		inv.depend(refExecutable, c.Ref)
	}
	inv.applyPackage(in.BinaryHex, in.Artifact, in.ArtifactHex)
	return inv, nil
}

// sameModules fails unless the executable links exactly the modules `go list`
// reports for the same target.
func sameModules(linked map[string]*debug.Module, listed map[string]string) error {
	var problems []string
	for p, m := range linked {
		v, ok := listed[p]
		switch {
		case !ok:
			problems = append(problems, p+" is linked but not listed")
		case v != m.Version:
			problems = append(problems, fmt.Sprintf("%s is linked at %s but listed at %s", p, m.Version, v))
		}
	}
	for p := range listed {
		if linked[p] == nil {
			problems = append(problems, p+" is listed but not linked")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New("the executable and `go list` disagree: " + strings.Join(problems, "; "))
	}
	return nil
}

// isLocalReplacement reports a replace directive that points at a directory
// rather than at a published module; Go records such a replacement with the
// version "(devel)" or none.
func isLocalReplacement(r *debug.Module) bool {
	return r.Version == "" || r.Version == "(devel)" || strings.HasPrefix(r.Path, "./") || strings.HasPrefix(r.Path, "../") || strings.HasPrefix(r.Path, "/")
}
