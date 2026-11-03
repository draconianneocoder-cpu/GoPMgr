// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command sbom writes, checks, signs and verifies GoPMgr's software bills of
// materials, following CISA's 2026 Minimum Elements for an SBOM. For each
// release package it writes one CycloneDX 1.7 and one SPDX 3.0.1 document
// from the same inventory. See docs/SBOM.md.
//
// From the repository root:
//
//	go run ./tools/sbom check
//	go run ./tools/sbom generate -target darwin/arm64 -binary <executable> -artifact <installer> -version 1.2.0 -out-dir dist
//	go run ./tools/sbom generate ... -inventory-out inv.json   # then, after packaging:
//	go run ./tools/sbom render -inventory inv.json -binary <shipped executable> -artifact <installer> -out-dir dist
//	go run ./tools/sbom sign dist/*.cdx.json dist/*.spdx.json     # key in $GOPMGR_SBOM_PRIVATE_KEY
//	go run ./tools/sbom verify dist/*.cdx.json dist/*.spdx.json
//	go run ./tools/sbom keygen -private-out sbom-signing.key        # the owner, once
//
// The inventory comes from the executable's own build information (the
// modules it links), cross-checked with `go list` for the same target, from
// the frontend bundle list and package-lock.json, from the embedded asset
// folders, and from reviewed metadata in tools/sbom/components.json. Any
// disagreement, or any shipped part without a reviewed entry, is an error.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

const (
	defaultReviewed  = "tools/sbom/components.json"
	defaultPublicKey = "tools/sbom/signing-key.pub"
	toolName         = "GoPMgr SBOM generator (tools/sbom)"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sbom:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: sbom check|generate|render|sign|verify|keygen [flags]")
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout)
	case "generate":
		return runGenerate(args[1:], stdout)
	case "render":
		return runRender(args[1:], stdout)
	case "sign":
		return runSign(args[1:])
	case "verify":
		return runVerify(args[1:], stdout)
	case "keygen":
		return runKeygen(args[1:], stdout)
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func runCheck(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root")
	reviewedPath := fs.String("reviewed", defaultReviewed, "reviewed metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := loadReviewed(inRepo(*repo, *reviewedPath))
	if err != nil {
		return err
	}
	listings := map[string]map[string]string{}
	linkedPackages := map[string]bool{}
	var embeddedBinaries []string
	for _, t := range releaseTargets {
		pkgs, err := listPackages(*repo, t)
		if err != nil {
			return err
		}
		mods, _ := moduleGraph(pkgs)
		listings[t.String()] = mods
		for _, p := range pkgs {
			linkedPackages[p.ImportPath] = true
		}
		for _, b := range unreviewedEmbeddedBinaries(pkgs, r) {
			embeddedBinaries = append(embeddedBinaries, t.String()+": "+b)
		}
		mainEmbeds, err := unreviewedMainEmbeds(*repo, pkgs, r)
		if err != nil {
			return err
		}
		for _, b := range mainEmbeds {
			embeddedBinaries = append(embeddedBinaries, t.String()+": "+b)
		}
	}
	if len(embeddedBinaries) > 0 {
		return errors.New("tools/sbom/components.json does not review these embedded files:\n  " + strings.Join(embeddedBinaries, "\n  "))
	}
	embedded, err := embeddedFiles(*repo, r)
	if err != nil {
		return err
	}
	frontend, err := bundledPackages(*repo)
	if err != nil {
		return err
	}
	lock, err := readLock(*repo)
	if err != nil {
		return err
	}
	if err := checkReviewed(r, listings, linkedPackages, embedded, frontend, lock); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "sbom: reviewed metadata covers every release target (%d targets, %d embedded files, %d bundled npm packages)\n", len(releaseTargets), len(embedded), len(frontend))
	return nil
}

// checkReviewed fails on anything shipped without a reviewed entry and on
// reviewed entries nothing ships, such as a native entry whose parent module
// is no longer linked.
func checkReviewed(r *reviewed, listings map[string]map[string]string, linkedPackages map[string]bool, embedded []embeddedFile, frontend []frontendPackage, lock map[string]lockPackage) error {
	var problems []string
	used := map[string]bool{}
	for t, mods := range listings {
		for mod := range mods {
			used[mod] = true
			if _, ok := r.GoModules[mod]; !ok {
				problems = append(problems, fmt.Sprintf("%s: Go module %s has no reviewed entry", t, mod))
			}
		}
	}
	for mod := range r.GoModules {
		if !used[mod] {
			problems = append(problems, "reviewed Go module "+mod+" is not linked into any release target")
		}
	}
	for _, n := range r.Native {
		if !used[n.ParentModule] {
			problems = append(problems, fmt.Sprintf("reviewed native component %s names parent module %s, which no release target links", n.Name, n.ParentModule))
		}
		if n.ParentPackage != "" && !linkedPackages[n.ParentPackage] {
			problems = append(problems, fmt.Sprintf("reviewed native component %s names parent package %s, which no release target links", n.Name, n.ParentPackage))
		}
	}
	entryUsed := make([]bool, len(r.Embedded))
	for _, f := range embedded {
		entryUsed[f.Entry] = true
	}
	for i, e := range r.Embedded {
		if !entryUsed[i] {
			problems = append(problems, "reviewed embedded entry "+e.Pattern+" matches no file")
		}
	}
	bundled := map[string]bool{}
	for _, f := range frontend {
		bundled[f.Name] = true
		if lp, ok := lock["node_modules/"+f.Name]; !ok || lp.Version != f.Version {
			problems = append(problems, fmt.Sprintf("bundled npm package %s %s does not match package-lock.json", f.Name, f.Version))
		} else if lp.License == "" && r.NPM[f.Name].License == "" {
			problems = append(problems, "bundled npm package "+f.Name+" declares no license and has no reviewed one")
		}
	}
	for name := range r.NPM {
		if !bundled[name] {
			problems = append(problems, "reviewed npm package "+name+" is not in the bundle")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New("tools/sbom/components.json is out of date:\n  " + strings.Join(problems, "\n  "))
	}
	return nil
}

func runGenerate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root")
	reviewedPath := fs.String("reviewed", defaultReviewed, "reviewed metadata")
	targetName := fs.String("target", "", "release target, e.g. darwin/arm64")
	binary := fs.String("binary", "", "the built executable")
	artifact := fs.String("artifact", "", "the installer package; its hash identifies the release")
	version := fs.String("version", "", "release version without the leading v")
	commit := fs.String("commit", "", "source commit (default: from the executable's build information)")
	sbomVersion := fs.Int("sbom-version", 1, "SBOM version; raise it to reissue a corrected SBOM for the same release")
	timestamp := fs.String("time", "", "SBOM timestamp, RFC 3339 (default: now)")
	toolVersion := fs.String("tool-version", "", "version of this tool (default: the source commit)")
	outDir := fs.String("out-dir", ".", "output directory")
	publicKey := fs.String("public-key", defaultPublicKey, "committed SBOM signing public key; recorded when it exists")
	inventoryOut := fs.String("inventory-out", "", "write the inventory here instead of SBOMs, for a later render")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *targetName == "" || *binary == "" {
		return errors.New("generate needs -target and -binary")
	}
	if *sbomVersion < 1 {
		return errors.New("-sbom-version must be at least 1")
	}
	t, err := findTarget(*targetName)
	if err != nil {
		return err
	}
	r, err := loadReviewed(inRepo(*repo, *reviewedPath))
	if err != nil {
		return err
	}
	bi, err := readBinary(*binary)
	if err != nil {
		return err
	}
	in := buildInput{Repo: *repo, Target: t, Reviewed: r, Binary: bi, Version: strings.TrimPrefix(*version, "v"), Commit: *commit}
	if in.Commit == "" {
		in.Commit = setting(bi, "vcs.revision")
	}
	if in.BinaryHex, err = fileSHA256(*binary); err != nil {
		return err
	}
	if *artifact != "" {
		if in.ArtifactHex, err = fileSHA256(*artifact); err != nil {
			return err
		}
		in.Artifact = filepath.Base(*artifact)
	}
	// List packages with the executable's own build tags, so the build
	// constraints match what was linked.
	if tags := setting(bi, "-tags"); tags != "" {
		t.Tags = tags
	}
	if in.Packages, err = listPackages(*repo, t); err != nil {
		return err
	}
	if in.Frontend, err = bundledPackages(*repo); err != nil {
		return err
	}
	if in.Lock, err = readLock(*repo); err != nil {
		return err
	}
	if in.Embedded, err = embeddedFiles(*repo, r); err != nil {
		return err
	}
	in.EmbeddedHex = map[string]string{}
	for _, f := range in.Embedded {
		if in.EmbeddedHex[f.Rel], err = fileSHA256(filepath.Join(*repo, filepath.FromSlash(f.Rel))); err != nil {
			return err
		}
	}
	in.ModuleArchive = func(mod, version, sum string) (string, error) { return moduleArchive(*repo, mod, version, sum) }
	in.ModuleFile = func(mod, version, rel string) (string, error) { return moduleFile(*repo, mod, version, rel) }
	inv, err := assemble(in)
	if err != nil {
		return err
	}
	inv.Author, inv.Commit = r.Author, in.Commit
	if *inventoryOut != "" {
		b, err := json.MarshalIndent(inv, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*inventoryOut, append(b, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "sbom: wrote inventory", *inventoryOut)
		return nil
	}
	return writeSBOMs(inv, sbomOptions{SBOMVersion: *sbomVersion, Timestamp: *timestamp, ToolVersion: *toolVersion,
		PublicKey: inRepo(*repo, *publicKey), ExplicitKey: flagSet(fs, "public-key"), Artifact: in.Artifact, OutDir: *outDir}, stdout)
}

// runRender writes the SBOMs for one installer package from an inventory
// recorded by `generate -inventory-out`. The executable given here must have
// the same build information as the one the inventory was recorded from.
func runRender(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root")
	inventoryPath := fs.String("inventory", "", "inventory written by generate -inventory-out")
	binary := fs.String("binary", "", "the executable as shipped in the package")
	artifact := fs.String("artifact", "", "the installer package")
	sbomVersion := fs.Int("sbom-version", 1, "SBOM version; raise it to reissue a corrected SBOM for the same release")
	timestamp := fs.String("time", "", "SBOM timestamp, RFC 3339 (default: now)")
	toolVersion := fs.String("tool-version", "", "version of this tool (default: the source commit)")
	outDir := fs.String("out-dir", ".", "output directory")
	publicKey := fs.String("public-key", defaultPublicKey, "committed SBOM signing public key; recorded when it exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inventoryPath == "" || *binary == "" || *artifact == "" {
		return errors.New("render needs -inventory, -binary and -artifact")
	}
	b, err := os.ReadFile(*inventoryPath)
	if err != nil {
		return err
	}
	var inv inventory
	if err := json.Unmarshal(b, &inv); err != nil || len(inv.Components) < 2 || inv.Fingerprint == "" {
		return fmt.Errorf("%s is not an inventory written by generate -inventory-out", *inventoryPath)
	}
	bi, err := readBinary(*binary)
	if err != nil {
		return err
	}
	if fingerprint(bi) != inv.Fingerprint {
		return fmt.Errorf("%s is not the build the inventory was recorded from", *binary)
	}
	exeHex, err := fileSHA256(*binary)
	if err != nil {
		return err
	}
	artifactHex, err := fileSHA256(*artifact)
	if err != nil {
		return err
	}
	inv.applyPackage(exeHex, filepath.Base(*artifact), artifactHex)
	return writeSBOMs(&inv, sbomOptions{SBOMVersion: *sbomVersion, Timestamp: *timestamp, ToolVersion: *toolVersion,
		PublicKey: inRepo(*repo, *publicKey), ExplicitKey: flagSet(fs, "public-key"), Artifact: filepath.Base(*artifact), OutDir: *outDir}, stdout)
}

func flagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

type sbomOptions struct {
	SBOMVersion int
	Timestamp   string
	ToolVersion string
	PublicKey   string
	ExplicitKey bool
	Artifact    string
	OutDir      string
}

// writeSBOMs writes <artifact>.cdx.json and <artifact>.spdx.json.
func writeSBOMs(inv *inventory, o sbomOptions, stdout io.Writer) error {
	if o.SBOMVersion < 1 {
		return errors.New("-sbom-version must be at least 1")
	}
	m := docMeta{Author: inv.Author, ToolName: toolName, ToolVersion: o.ToolVersion, SBOMVersion: o.SBOMVersion, Context: "build"}
	if m.ToolVersion == "" {
		m.ToolVersion = toolBuildRevision(inv.Commit)
	}
	if o.Timestamp == "" {
		m.Timestamp = time.Now().UTC().Format(time.RFC3339)
	} else {
		ts, err := time.Parse(time.RFC3339, o.Timestamp)
		if err != nil {
			return fmt.Errorf("-time: %w", err)
		}
		m.Timestamp = ts.UTC().Format(time.RFC3339)
	}
	var err error
	if m.Serial, err = newUUID(); err != nil {
		return err
	}
	if m.Signature, err = signatureStatement(o.PublicKey, o.ExplicitKey); err != nil {
		return err
	}
	base := "gopmgr-" + strings.ReplaceAll(inv.Target.String(), "/", "-")
	if o.Artifact != "" {
		base = o.Artifact
	}
	cdx, err := encodeCycloneDX(inv, m)
	if err != nil {
		return err
	}
	spdx, err := encodeSPDX(inv, m)
	if err != nil {
		return err
	}
	for _, out := range []struct {
		name string
		b    []byte
	}{{base + ".cdx.json", cdx}, {base + ".spdx.json", spdx}} {
		if err := os.WriteFile(filepath.Join(o.OutDir, out.name), out.b, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "sbom: wrote", filepath.Join(o.OutDir, out.name))
	}
	return nil
}

// inRepo resolves a path flag: relative paths are relative to the
// repository root, absolute ones are used as given.
func inRepo(repo, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repo, p)
}

// signatureStatement says how the SBOM is signed. Without a published public
// key the SBOM says it is unsigned; a key that was asked for by flag but is
// missing or malformed is an error, never a silent "unsigned".
func signatureStatement(publicKey string, explicit bool) (string, error) {
	pub, err := readPublicKey(publicKey)
	switch {
	case err == nil:
		return fmt.Sprintf("detached Ed25519 signature in <this file>%s, by key %s", signatureSuffix, keyID(pub)), nil
	case errors.Is(err, os.ErrNotExist) && !explicit:
		return "unsigned: no SBOM signing key has been published yet (see docs/SBOM.md)", nil
	default:
		return "", fmt.Errorf("SBOM signing public key: %w", err)
	}
}

// toolBuildRevision is the version of this tool: the commit of the source it
// was run from, which `go run` does not always stamp.
func toolBuildRevision(fallback string) string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value
			}
		}
	}
	return fallback
}

// newUUID returns a random RFC 9562 version 4 UUID.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", fmt.Errorf("SBOM serial number: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	publicKey := fs.String("public-key", defaultPublicKey, "committed SBOM signing public key")
	keyEnv := fs.String("private-key-env", "GOPMGR_SBOM_PRIVATE_KEY", "environment variable holding the base64 private key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("sign needs SBOM files")
	}
	pub, err := readPublicKey(*publicKey)
	if err != nil {
		return err
	}
	raw, err := decodeKey(os.Getenv(*keyEnv), ed25519.PrivateKeySize, *keyEnv)
	if err != nil {
		return err
	}
	return signFiles(ed25519.PrivateKey(raw), pub, fs.Args())
}

func runVerify(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	publicKey := fs.String("public-key", defaultPublicKey, "SBOM signing public key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("verify needs SBOM files")
	}
	pub, err := readPublicKey(*publicKey)
	if err != nil {
		return err
	}
	if err := verifyFiles(pub, fs.Args()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "sbom: %d signatures verify with key %s\n", fs.NArg(), keyID(pub))
	return nil
}

func runKeygen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privateOut := fs.String("private-out", "", "file to create for the private key (kept secret, never committed)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *privateOut == "" {
		return errors.New("keygen needs -private-out")
	}
	pub, err := generateKey(*privateOut)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s", pub)
	return nil
}
