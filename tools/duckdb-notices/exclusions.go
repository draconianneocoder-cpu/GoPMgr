// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const exclusionsFile = "third_party/EXCLUDED-NATIVE-CODE.txt"

// cgoModules are the Go modules whose cgo code calls DuckDB's C API
// directly. Their calls are link inputs too, beside the archives.
var cgoModules = []string{bindingsModule, "github.com/duckdb/duckdb-go/v2"}

// exclusions is the parsed third_party/EXCLUDED-NATIVE-CODE.txt.
type exclusions struct {
	Archives []string
	Markers  []string
}

func readExclusions(path string) (exclusions, error) {
	f, err := os.Open(path) // #nosec G304 -- the repository's exclusion list.
	if err != nil {
		return exclusions{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()
	var ex exclusions
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, value, ok := strings.Cut(line, " ")
		value = strings.TrimSpace(value)
		switch {
		case ok && kind == "archive" && value != "":
			ex.Archives = append(ex.Archives, value)
		case ok && kind == "marker" && value != "":
			ex.Markers = append(ex.Markers, value)
		default:
			return exclusions{}, fmt.Errorf("%s:%d: want \"archive <file>\" or \"marker <text>\", have %q", path, n, line)
		}
	}
	if err := sc.Err(); err != nil {
		return exclusions{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(ex.Archives) == 0 || len(ex.Markers) == 0 {
		return exclusions{}, fmt.Errorf("%s names no archive or no marker", path)
	}
	return ex, nil
}

// linkedArchives returns the archive file names a platform module's cgo link
// flags name, in link order.
func linkedArchives(moduleDir string) ([]string, error) {
	src, err := os.ReadFile(filepath.Join(moduleDir, "prebuilt.go")) // #nosec G304 -- a file in the module cache.
	if err != nil {
		return nil, fmt.Errorf("read link flags: %w", err)
	}
	var libs []string
	library := regexp.MustCompile(`^-l([A-Za-z0-9_]+)$`)
	system := map[string]bool{"-lc++": true, "-lstdc++": true, "-lm": true, "-ldl": true, "-lws2_32": true, "-lwsock32": true, "-lrstrtmgr": true}
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#cgo ") || !strings.Contains(line, "LDFLAGS:") {
			continue
		}
		const prefix = "#cgo LDFLAGS:"
		if !strings.HasPrefix(line, prefix) {
			return nil, fmt.Errorf("unreviewed conditional link flags: %s", line)
		}
		for _, flag := range strings.Fields(strings.TrimPrefix(line, prefix)) {
			switch flag {
			case "-rdynamic", "--static", "-L${SRCDIR}":
				continue
			}
			m := library.FindStringSubmatch(flag)
			if system[flag] {
				name := "lib" + strings.TrimPrefix(flag, "-l") + ".a"
				if _, err := os.Lstat(filepath.Join(moduleDir, name)); !errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("system library %s may be shadowed in module: %v", flag, err)
				}
				continue
			}
			if m == nil {
				return nil, fmt.Errorf("unreviewed link flag %q", flag)
			}
			name := "lib" + m[1] + ".a"
			info, err := os.Lstat(filepath.Join(moduleDir, name))
			if err != nil {
				return nil, fmt.Errorf("stat required archive %s: %w", name, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("required archive %s is not regular", name)
			}
			libs = append(libs, name)
		}
	}
	if len(libs) == 0 {
		return nil, fmt.Errorf("%s/prebuilt.go names no archive in the module", moduleDir)
	}
	return libs, nil
}

// symbolSets holds the external symbols an archive's members define and
// leave undefined.
type symbolSets struct {
	Defined, Undefined map[string]bool
}

// archiveSymbols reads a System V / GNU or BSD ar archive and collects the
// external symbols of its ELF, Mach-O and COFF members.
func archiveSymbols(path string) (symbolSets, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- an archive in the module cache.
	if err != nil {
		return symbolSets{}, fmt.Errorf("read %s: %w", path, err)
	}
	members, err := arMembers(data)
	if err != nil {
		return symbolSets{}, fmt.Errorf("%s: %w", path, err)
	}
	sets := symbolSets{Defined: map[string]bool{}, Undefined: map[string]bool{}}
	for _, m := range members {
		if err := objectSymbols(m.data, sets); err != nil {
			return symbolSets{}, fmt.Errorf("%s(%s): %w", path, m.name, err)
		}
	}
	return sets, nil
}

type arMember struct {
	name string
	data []byte
}

func arMembers(data []byte) ([]arMember, error) {
	const magic = "!<arch>\n"
	if !bytes.HasPrefix(data, []byte(magic)) {
		return nil, errors.New("not an ar archive")
	}
	var members []arMember
	var longNames []byte
	for off := len(magic); off < len(data); {
		if off+60 > len(data) {
			return nil, fmt.Errorf("truncated member header at offset %d", off)
		}
		hdr := data[off : off+60]
		if string(hdr[58:60]) != "`\n" {
			return nil, fmt.Errorf("bad member header at offset %d", off)
		}
		size, err := strconv.Atoi(strings.TrimSpace(string(hdr[48:58])))
		if err != nil || size < 0 || size > len(data)-off-60 {
			return nil, fmt.Errorf("bad member size at offset %d", off)
		}
		name := strings.TrimRight(string(hdr[0:16]), " ")
		body := data[off+60 : off+60+size]
		next := off + 60 + size
		if size%2 != 0 {
			if next >= len(data) || data[next] != '\n' {
				return nil, fmt.Errorf("missing or invalid archive padding at offset %d", next)
			}
			next++
		}
		switch {
		case name == "//": // GNU long-name table
			longNames = body
		case name == "/" || name == "/SYM64/" || strings.HasPrefix(name, "__.SYMDEF"):
			// symbol index
		case strings.HasPrefix(name, "#1/"): // BSD: the name opens the body
			n, err := strconv.Atoi(name[3:])
			if err != nil || n < 0 || n > len(body) {
				return nil, fmt.Errorf("bad BSD long name at offset %d", off)
			}
			realName := strings.TrimRight(string(body[:n]), "\x00")
			if !strings.HasPrefix(realName, "__.SYMDEF") {
				members = append(members, arMember{name: realName, data: body[n:]})
			}
		case strings.HasPrefix(name, "/") && len(name) > 1: // GNU: offset into the long-name table
			i, err := strconv.Atoi(name[1:])
			if err != nil || i < 0 || i >= len(longNames) {
				return nil, fmt.Errorf("bad long-name reference %q", name)
			}
			end := bytes.IndexAny(longNames[i:], "/\n")
			if end < 0 {
				end = len(longNames) - i
			}
			members = append(members, arMember{name: string(longNames[i : i+end]), data: body})
		default:
			members = append(members, arMember{name: strings.TrimSuffix(name, "/"), data: body})
		}
		off = next
	}
	return members, nil
}

func objectSymbols(obj []byte, sets symbolSets) error {
	r := bytes.NewReader(obj)
	switch {
	case bytes.HasPrefix(obj, []byte(elf.ELFMAG)):
		f, err := elf.NewFile(r)
		if err != nil {
			return err
		}
		syms, err := f.Symbols()
		if err != nil {
			return err
		}
		for _, s := range syms {
			bind := elf.ST_BIND(s.Info)
			if s.Name == "" || (bind != elf.STB_GLOBAL && bind != elf.STB_WEAK) {
				continue
			}
			if s.Section == elf.SHN_UNDEF {
				sets.Undefined[s.Name] = true
			} else {
				sets.Defined[s.Name] = true
			}
		}
		return nil
	case len(obj) >= 4 && (bytes.Equal(obj[:4], []byte{0xcf, 0xfa, 0xed, 0xfe}) || bytes.Equal(obj[:4], []byte{0xce, 0xfa, 0xed, 0xfe})):
		f, err := macho.NewFile(r)
		if err != nil {
			return err
		}
		if f.Symtab == nil {
			return errors.New("Mach-O member has no symbol table")
		}
		const nExt, nType, nUndf = 0x01, 0x0e, 0x00
		for _, s := range f.Symtab.Syms {
			if s.Name == "" || s.Type&nExt == 0 || s.Type&0xe0 != 0 {
				continue // local or debugging symbol
			}
			if s.Type&nType == nUndf && s.Value == 0 {
				sets.Undefined[s.Name] = true
			} else {
				sets.Defined[s.Name] = true
			}
		}
		return nil
	case len(obj) >= 2 && obj[0] == 0x64 && obj[1] == 0x86: // COFF, AMD64
		f, err := pe.NewFile(r)
		if err != nil {
			return err
		}
		if len(f.Symbols) == 0 {
			return errors.New("COFF member has no symbol table")
		}
		const classExternal, classWeakExternal = 2, 105
		for _, s := range f.Symbols {
			if s.Name == "" || (s.StorageClass != classExternal && s.StorageClass != classWeakExternal) {
				continue
			}
			if s.SectionNumber == 0 && s.Value == 0 && s.StorageClass == classExternal {
				sets.Undefined[s.Name] = true
			} else {
				sets.Defined[s.Name] = true
			}
		}
		return nil
	}
	return errors.New("unsupported archive member format")
}

// exclusionReport is the result of auditing one platform module.
type exclusionReport struct {
	Platform string
	// Referenced lists symbols that an excluded archive defines and some
	// linked archive, or the bindings' cgo code, references. Any entry
	// means a linker could pull the excluded code in.
	Referenced []string
	// MissingMarkers are markers absent from the excluded archives, so they
	// would not detect the code; LeakedMarkers are markers found in another
	// linked archive, so they would raise false alarms.
	MissingMarkers, LeakedMarkers []string
	ExcludedDefs                  int
}

func (r exclusionReport) problems() []string {
	var out []string
	if len(r.Referenced) > 0 {
		out = append(out, fmt.Sprintf("%s: linked libraries reference %d symbol(s) that only the excluded archives define, for example %q", r.Platform, len(r.Referenced), r.Referenced[0]))
	}
	if len(r.MissingMarkers) > 0 {
		out = append(out, fmt.Sprintf("%s: markers absent from the excluded archives: %q", r.Platform, r.MissingMarkers))
	}
	if len(r.LeakedMarkers) > 0 {
		out = append(out, fmt.Sprintf("%s: markers present in other linked libraries: %q", r.Platform, r.LeakedMarkers))
	}
	return out
}

// auditExclusions checks one platform module against the exclusion list.
// cgo holds the C names the Go bindings call.
func auditExclusions(platform, moduleDir string, ex exclusions, cgo map[string]bool) (exclusionReport, error) {
	rep := exclusionReport{Platform: platform}
	libs, err := linkedArchives(moduleDir)
	if err != nil {
		return rep, err
	}
	excluded := map[string]bool{}
	for _, a := range ex.Archives {
		excluded[a] = true
	}
	excludedDefs := map[string]bool{}
	undefined := map[string]bool{}
	var excludedBlobs, otherBlobs [][]byte
	for _, lib := range libs {
		path := filepath.Join(moduleDir, lib)
		sets, err := archiveSymbols(path)
		if err != nil {
			return rep, err
		}
		blob, err := os.ReadFile(path) // #nosec G304 -- an archive in the module cache.
		if err != nil {
			return rep, err
		}
		if excluded[lib] {
			if len(sets.Defined) == 0 {
				return rep, fmt.Errorf("excluded archive %s has no external definitions", lib)
			}
			for s := range sets.Defined {
				excludedDefs[s] = true
			}
			excludedBlobs = append(excludedBlobs, blob)
			continue
		}
		for s := range sets.Undefined {
			undefined[s] = true
		}
		otherBlobs = append(otherBlobs, blob)
	}
	rep.ExcludedDefs = len(excludedDefs)
	if len(excludedBlobs) != len(ex.Archives) {
		return rep, fmt.Errorf("%s: the link flags name %d of the %d excluded archives; review %s", platform, len(excludedBlobs), len(ex.Archives), exclusionsFile)
	}
	for s := range excludedDefs {
		// Mach-O prefixes C names with an underscore.
		if undefined[s] || cgo[s] || cgo[strings.TrimPrefix(s, "_")] {
			rep.Referenced = append(rep.Referenced, s)
		}
	}
	sort.Strings(rep.Referenced)
	for _, m := range ex.Markers {
		if !containsAny(excludedBlobs, m) {
			rep.MissingMarkers = append(rep.MissingMarkers, m)
		}
		if containsAny(otherBlobs, m) {
			rep.LeakedMarkers = append(rep.LeakedMarkers, m)
		}
	}
	return rep, nil
}

func containsAny(blobs [][]byte, marker string) bool {
	for _, b := range blobs {
		if bytes.Contains(b, []byte(marker)) {
			return true
		}
	}
	return false
}

// libModules lists the DuckDB platform library modules go.mod requires.
// The audit covers all of them, not only the release platforms, because a
// source build (such as the Debian package) can target any of them.
func libModules(gomod []byte) []string {
	re := regexp.MustCompile(`(?m)^\s*(?:require\s+)?(` + regexp.QuoteMeta(bindingsModule) + `/lib/[\w.-]+)\s+v\S+`)
	var mods []string
	for _, m := range re.FindAllSubmatch(gomod, -1) {
		mods = append(mods, string(m[1]))
	}
	sort.Strings(mods)
	return mods
}

// cgoNames collects the C identifiers (C.name) that the Go files in dirs use.
func cgoNames(dirs []string) (map[string]bool, error) {
	names := map[string]bool{}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", dir, err)
		}
		for _, f := range files {
			tree, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", f, err)
			}
			cgo := false
			for _, imp := range tree.Imports {
				cgo = cgo || imp.Path.Value == `"C"`
			}
			if !cgo {
				continue
			}
			ast.Inspect(tree, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "C" {
						names[sel.Sel.Name] = true
					}
				}
				return true
			})
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("found no cgo calls in %q; review cgoModules", dirs)
	}
	return names, nil
}

// checkExclusions audits every DuckDB platform module that go.mod requires.
func checkExclusions(moduleDir func(string) (string, error)) error {
	ex, err := readExclusions(exclusionsFile)
	if err != nil {
		return err
	}
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	mods := libModules(gomod)
	if len(mods) == 0 {
		return fmt.Errorf("go.mod requires no %s/lib/* module; review tools/duckdb-notices", bindingsModule)
	}
	var cgoDirs []string
	for _, m := range cgoModules {
		dir, err := moduleDir(m)
		if err != nil {
			return err
		}
		if err := verifyModuleInputs(m, dir); err != nil {
			return err
		}
		cgoDirs = append(cgoDirs, dir)
	}
	cgo, err := cgoNames(cgoDirs)
	if err != nil {
		return err
	}
	var problems []string
	for _, m := range mods {
		dir, err := moduleDir(m)
		if err != nil {
			return err
		}
		if err := verifyModuleInputs(m, dir); err != nil {
			return err
		}
		name := strings.TrimPrefix(m, bindingsModule+"/lib/")
		rep, err := auditExclusions(name, dir, ex, cgo)
		if err != nil {
			return err
		}
		problems = append(problems, rep.problems()...)
		if len(rep.problems()) == 0 {
			fmt.Printf("duckdb-notices: %s: the %d symbols the excluded archives define are unreferenced, and all %d markers are specific to them\n", name, rep.ExcludedDefs, len(ex.Markers))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("excluded native code (%s) failed its reviewed-input audit:\n  %s", exclusionsFile, strings.Join(problems, "\n  "))
	}
	return nil
}

// moduleDir reports where the module cache holds a module at the version
// go.mod selects, downloading it first if needed.
func moduleDir(module string) (string, error) {
	out, err := exec.Command("go", "mod", "download", "-json", module).Output()
	var info struct{ Dir, Error string }
	if jerr := json.Unmarshal(out, &info); jerr != nil && err == nil {
		err = jerr
	}
	switch {
	case info.Error != "":
		return "", fmt.Errorf("go mod download %s: %s", module, info.Error)
	case err != nil:
		return "", fmt.Errorf("go mod download %s: %w", module, err)
	case info.Dir == "":
		return "", fmt.Errorf("go mod download %s reported no directory", module)
	}
	return info.Dir, nil
}
