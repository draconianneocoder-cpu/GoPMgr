// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"
)

// target is one release build, with the tags its `wails build` step uses.
// tools/module-notices keeps the same list; a test holds them in step.
type target struct {
	GOOS, GOARCH, Tags string
}

func (t target) String() string { return t.GOOS + "/" + t.GOARCH }

var releaseTargets = []target{
	{GOOS: "darwin", GOARCH: "arm64", Tags: "desktop,production"},
	{GOOS: "linux", GOARCH: "amd64", Tags: "desktop,production,webkit2_41"},
	{GOOS: "windows", GOARCH: "amd64", Tags: "desktop,production"},
}

func findTarget(name string) (target, error) {
	for _, t := range releaseTargets {
		if t.String() == name {
			return t, nil
		}
	}
	return target{}, fmt.Errorf("unknown release target %q", name)
}

// Hash kinds say what a hash was computed over. Only the installer and the
// executable are executable artifacts in the sense of the CISA Component
// Hash element; archive and file hashes identify the inputs that went in.
const (
	hashInstaller  = "installer"
	hashExecutable = "executable"
	hashModuleZip  = "go-module-archive"
	hashNPMTarball = "npm-package-archive"
	hashFile       = "embedded-file"
	hashModuleFile = "file-in-go-module"
)

type hash struct {
	Alg  string // IANA hash function textual name: sha-256, sha-512
	Hex  string
	Kind string
}

type identifier struct {
	Kind  string // purl, cpe23, gitCommit, goModuleSum, file
	Value string
}

// component is one entry of the inventory. Empty Version, Producer or License
// means unknown; Unknown says why, for each such field.
type component struct {
	Ref         string
	Kind        string // application, executable, library, data, platform
	Name        string
	Version     string
	Producer    string
	License     string
	Identifiers []identifier
	Hashes      []hash
	Shipped     bool
	Unknown     map[string]string
	Note        string
}

func (c *component) unknown(field, why string) {
	if c.Unknown == nil {
		c.Unknown = map[string]string{}
	}
	c.Unknown[field] = why
}

type inventory struct {
	Target     target
	Components []component         // first is the application, second the executable
	Deps       map[string][]string // ref -> refs it needs to operate
	// Fingerprint summarizes the executable's build information, so a later
	// render can confirm it is given the same build (code signing changes an
	// executable's bytes, not its build information).
	Fingerprint string
	// Author and Commit carry what render needs without the reviewed file.
	Author string
	Commit string
}

func (inv *inventory) add(c component) {
	inv.Components = append(inv.Components, c)
}

func (inv *inventory) depend(from string, to ...string) {
	inv.Deps[from] = append(inv.Deps[from], to...)
}

// goPackage is the subset of `go list -json` output the generator reads.
type goPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	Imports    []string
	EmbedFiles []string
	Module     *struct {
		Path    string
		Version string
		Main    bool
	}
	Error *struct{ Err string }
}

// listPackages runs `go list -deps` on the root package for t, offline.
func listPackages(repo string, t target) ([]goPackage, error) {
	cmd := exec.Command("go", "list", "-e", "-deps", "-json", "-tags", t.Tags, ".")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "CGO_ENABLED=1", "GOWORK=off", "GOFLAGS=-mod=readonly", "GOPROXY=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list for %s: %w: %s", t, err, strings.TrimSpace(stderr.String()))
	}
	var pkgs []goPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list for %s: %w", t, err)
		}
		if p.Error != nil {
			return nil, fmt.Errorf("go list for %s: %s: %s", t, p.ImportPath, p.Error.Err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// moduleGraph turns a package list into module-level edges. The main module
// is "" and the standard library "std".
func moduleGraph(pkgs []goPackage) (modules map[string]string, edges map[string]map[string]bool) {
	modOf := map[string]string{}
	modules = map[string]string{}
	for _, p := range pkgs {
		switch {
		case p.Standard:
			modOf[p.ImportPath] = "std"
		case p.Module == nil:
			modOf[p.ImportPath] = ""
		case p.Module.Main:
			modOf[p.ImportPath] = ""
		default:
			modOf[p.ImportPath] = p.Module.Path
			modules[p.Module.Path] = p.Module.Version
		}
	}
	edges = map[string]map[string]bool{}
	for _, p := range pkgs {
		from := modOf[p.ImportPath]
		for _, imp := range p.Imports {
			to, ok := modOf[imp]
			if !ok || to == from {
				continue
			}
			if edges[from] == nil {
				edges[from] = map[string]bool{}
			}
			edges[from][to] = true
		}
	}
	return modules, edges
}

// moduleArchive returns the SHA-256 of the module zip in the module cache,
// after checking that the zip is the one whose go.sum hash the binary
// records.
func moduleArchive(repo, mod, version, sum string) (string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", mod+"@"+version)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("module archive %s@%s is not in the module cache: %w", mod, version, err)
	}
	var info struct{ Zip, Sum string }
	if err := json.Unmarshal(out, &info); err != nil || info.Zip == "" {
		return "", fmt.Errorf("module archive %s@%s: no zip reported", mod, version)
	}
	got, err := dirhash.HashZip(info.Zip, dirhash.Hash1)
	if err != nil {
		return "", fmt.Errorf("module archive %s@%s: %w", mod, version, err)
	}
	if got != sum {
		return "", fmt.Errorf("module archive %s@%s hashes to %s, but the binary records %s", mod, version, got, sum)
	}
	return fileSHA256(info.Zip)
}

func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", name, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// purlGo builds a Package URL for a Go module (ECMA-427, type golang).
func purlGo(mod, version string) string {
	return "pkg:golang/" + purlPath(strings.ToLower(mod)) + "@" + purlEscape(version)
}

func purlPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = purlEscape(s)
	}
	return strings.Join(parts, "/")
}

// purlEscape percent-encodes everything outside the purl unreserved set.
func purlEscape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func purlNPM(name, version string) string {
	if scope, rest, ok := strings.Cut(name, "/"); ok && strings.HasPrefix(scope, "@") {
		return "pkg:npm/" + purlEscape(scope) + "/" + purlEscape(rest) + "@" + purlEscape(version)
	}
	return "pkg:npm/" + purlEscape(name) + "@" + purlEscape(version)
}

type frontendPackage struct {
	Name, Version string
}

// bundledPackages reads the package list from FRONTEND-NOTICES.txt, which
// `make frontend-notices-check` keeps equal to the production bundle.
func bundledPackages(repo string) ([]frontendPackage, error) {
	b, err := os.ReadFile(filepath.Join(repo, "third_party", "FRONTEND-NOTICES.txt"))
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(bytes.NewReader(b))
	var out []frontendPackage
	for state := 0; s.Scan(); {
		line := s.Text()
		switch {
		case state == 0 && line == "Contents":
			state = 1
		case state == 1 && strings.HasPrefix(line, "---"):
			state = 2
		case state == 2 && line == "" && len(out) > 0:
			return out, nil
		case state == 2 && strings.HasPrefix(line, "  "):
			f := strings.Fields(line)
			if len(f) != 2 {
				return nil, fmt.Errorf("FRONTEND-NOTICES.txt: unexpected contents line %q", line)
			}
			out = append(out, frontendPackage{f[0], f[1]})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("FRONTEND-NOTICES.txt: no package list")
	}
	return out, nil
}

type lockPackage struct {
	Version      string            `json:"version"`
	License      string            `json:"license"`
	Integrity    string            `json:"integrity"`
	Dependencies map[string]string `json:"dependencies"`
}

func readLock(repo string) (map[string]lockPackage, error) {
	b, err := os.ReadFile(filepath.Join(repo, "frontend", "package-lock.json"))
	if err != nil {
		return nil, err
	}
	var lock struct {
		Packages map[string]lockPackage `json:"packages"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return nil, fmt.Errorf("package-lock.json: %w", err)
	}
	return lock.Packages, nil
}

// integrityHash converts an npm Subresource Integrity value to an IANA name
// and hex digest.
func integrityHash(sri string) (hash, error) {
	alg, b64, ok := strings.Cut(sri, "-")
	names := map[string]string{"sha512": "sha-512", "sha384": "sha-384", "sha256": "sha-256"}
	if !ok || names[alg] == "" {
		return hash{}, fmt.Errorf("unsupported integrity value %q", sri)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return hash{}, fmt.Errorf("integrity value %q: %w", sri, err)
	}
	return hash{Alg: names[alg], Hex: hex.EncodeToString(raw), Kind: hashNPMTarball}, nil
}

// npmAuthor returns the author a package's own package.json declares, when
// node_modules is present.
func npmAuthor(repo, name string) string {
	b, err := os.ReadFile(filepath.Join(repo, "frontend", "node_modules", filepath.FromSlash(name), "package.json"))
	if err != nil {
		return ""
	}
	var p struct {
		Author json.RawMessage `json:"author"`
	}
	if json.Unmarshal(b, &p) != nil || len(p.Author) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(p.Author, &s) == nil {
		return cleanAuthor(s)
	}
	var o struct{ Name string }
	if json.Unmarshal(p.Author, &o) == nil {
		return cleanAuthor(o.Name)
	}
	return ""
}

var authorExtras = regexp.MustCompile(`\s*[<(].*$`)

func cleanAuthor(s string) string {
	return strings.TrimSpace(authorExtras.ReplaceAllString(s, ""))
}

// embeddedFile is one third-party file the binary embeds.
type embeddedFile struct {
	Rel   string
	Entry int
}

// embeddedFiles walks the reviewed roots and assigns every file to exactly
// one reviewed entry.
func embeddedFiles(repo string, r *reviewed) ([]embeddedFile, error) {
	var out []embeddedFile
	for _, root := range r.EmbeddedRoots {
		err := filepath.WalkDir(filepath.Join(repo, filepath.FromSlash(root)), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(repo, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			for _, pat := range r.EmbeddedIgnore {
				if ok, _ := path.Match(pat, path.Base(rel)); ok {
					return nil
				}
			}
			match := -1
			for i, e := range r.Embedded {
				if ok, _ := path.Match(e.Pattern, rel); ok {
					if match >= 0 {
						return fmt.Errorf("embedded file %s matches two reviewed entries", rel)
					}
					match = i
				}
			}
			if match < 0 {
				return fmt.Errorf("embedded file %s has no reviewed entry in tools/sbom/components.json", rel)
			}
			out = append(out, embeddedFile{rel, match})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// readBinary returns the build information Go records in an executable.
func readBinary(name string) (*buildinfo.BuildInfo, error) {
	bi, err := buildinfo.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read build information from %s: %w", name, err)
	}
	return bi, nil
}

func setting(bi *buildinfo.BuildInfo, key string) string {
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// fingerprint summarizes build information: toolchain, settings and every
// linked module with its sum and replacement.
func fingerprint(bi *buildinfo.BuildInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "go %s\nmain %s\n", bi.GoVersion, bi.Path)
	for _, s := range bi.Settings {
		fmt.Fprintf(&b, "set %s=%s\n", s.Key, s.Value)
	}
	for _, m := range bi.Deps {
		fmt.Fprintf(&b, "dep %s %s %s", m.Path, m.Version, m.Sum)
		if m.Replace != nil {
			fmt.Fprintf(&b, " => %s %s %s", m.Replace.Path, m.Replace.Version, m.Replace.Sum)
		}
		b.WriteString("\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// applyPackage records the shipped executable's hash and the installer
// package that carries it.
func (inv *inventory) applyPackage(exeHex, artifactName, artifactHex string) {
	for i := range inv.Components {
		c := &inv.Components[i]
		switch c.Ref {
		case refExecutable:
			c.Hashes = []hash{{"sha-256", exeHex, hashExecutable}}
		case refApp:
			if artifactHex == "" {
				continue
			}
			c.Hashes = []hash{{"sha-256", artifactHex, hashInstaller}}
			c.Identifiers = append(c.Identifiers, identifier{"file", artifactName})
			delete(c.Unknown, "hash")
		}
	}
}

// moduleFile returns the SHA-256 of a file inside a module in the module
// cache. The go command extracts the cache from the archive whose hash the
// executable records, and keeps it read-only.
func moduleFile(repo, mod, version, rel string) (string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", mod+"@"+version)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("module %s@%s is not in the module cache: %w", mod, version, err)
	}
	var info struct{ Dir string }
	if err := json.Unmarshal(out, &info); err != nil || info.Dir == "" {
		return "", fmt.Errorf("module %s@%s: no directory reported", mod, version)
	}
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", fmt.Errorf("module file %q is not inside the module", rel)
	}
	return fileSHA256(filepath.Join(info.Dir, filepath.FromSlash(rel)))
}

var binaryExtensions = map[string]bool{".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".lib": true}

// unreviewedEmbeddedBinaries lists executable or library files that a
// dependency embeds with //go:embed and that no reviewed native entry names.
// Such a file is usually another producer's binary inside a module.
func unreviewedEmbeddedBinaries(pkgs []goPackage, r *reviewed) []string {
	reviewedFiles := map[string]bool{}
	for _, n := range r.Native {
		if n.ParentPackage != "" && n.ModuleFile != "" {
			reviewedFiles[n.ParentPackage+"|"+path.Base(n.ModuleFile)] = true
		}
	}
	var out []string
	for _, p := range pkgs {
		if p.Standard || p.Module == nil || p.Module.Main {
			continue
		}
		for _, f := range p.EmbedFiles {
			if binaryExtensions[strings.ToLower(path.Ext(f))] && !reviewedFiles[p.ImportPath+"|"+path.Base(f)] {
				out = append(out, p.ImportPath+" embeds "+f)
			}
		}
	}
	return out
}

// unreviewedMainEmbeds lists files GoPMgr's own packages embed that lie
// neither under a reviewed third-party root nor under a declared first-party
// prefix, so new embedded third-party content cannot go unlisted.
func unreviewedMainEmbeds(repo string, pkgs []goPackage, r *reviewed) ([]string, error) {
	// go list reports package folders under the same root path it was run
	// from; a file that still resolves outside the repository is reported.
	root, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range pkgs {
		if p.Module == nil || !p.Module.Main {
			continue
		}
		for _, f := range p.EmbedFiles {
			rel, err := filepath.Rel(root, filepath.Join(p.Dir, filepath.FromSlash(f)))
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			if !underAny(rel, r.EmbeddedRoots) && !underAny(rel, r.FirstPartyEmbedded) {
				out = append(out, p.ImportPath+" embeds "+rel)
			}
		}
	}
	return out, nil
}

func underAny(rel string, dirs []string) bool {
	for _, d := range dirs {
		if rel == d || strings.HasPrefix(rel, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}
