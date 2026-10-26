// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command duckdb-notices writes third_party/DUCKDB-NATIVE-NOTICES.txt: the
// license and notice texts of the third-party code compiled into DuckDB's
// prebuilt static libraries, which GoPMgr links through the
// github.com/duckdb/duckdb-go-bindings platform modules. Those modules ship
// only DuckDB's own license, so the texts come from the DuckDB source the
// libraries were built from, at a pinned commit.
//
// Usage, from the repository root:
//
//	go run ./tools/duckdb-notices -check    # offline: pins and file still match go.mod
//	go run ./tools/duckdb-notices -fetch    # clone the pinned sources and rewrite the file
//	go run ./tools/duckdb-notices -fetch -check   # same, but compare instead of writing
//	go run ./tools/duckdb-notices -duckdb DIR -robin-hood DIR [-check]
//
// The component list below was reviewed by hand against the symbols in each
// linked library and the sources of DuckDB v1.5.6 (see LICENSES.md). When
// duckdb-go-bindings changes version, the offline check fails until that
// review is repeated for the new DuckDB release and the pins are updated.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	defaultOutput = "third_party/DUCKDB-NATIVE-NOTICES.txt"

	bindingsModule  = "github.com/duckdb/duckdb-go-bindings"
	bindingsVersion = "v0.10506.0"
)

// source is one pinned upstream checkout the texts come from.
type source struct {
	Name, URL, Tag, Commit string
}

var sources = map[string]source{
	"duckdb": {
		Name: "duckdb", URL: "https://github.com/duckdb/duckdb",
		Tag: "v1.5.6", Commit: "069cc9f9b5be802405797faecc284961b07c70ef",
	},
	// DuckDB credits robin-hood-hashing 3.11.5 in src/common/checksum.cpp and
	// src/common/types/hash.cpp but does not carry its license file.
	"robin-hood-hashing": {
		Name: "robin-hood-hashing", URL: "https://github.com/martinus/robin-hood-hashing",
		Tag: "3.11.5", Commit: "9145f963d80d6a02f0f96a47758050a89184a3ed",
	},
}

// platforms maps each release platform to its duckdb-go-bindings library
// module.
var platforms = []struct{ Name, Module string }{
	{"darwin/arm64", bindingsModule + "/lib/darwin-arm64"},
	{"linux/amd64", bindingsModule + "/lib/linux-amd64"},
	{"windows/amd64", bindingsModule + "/lib/windows-amd64"},
}

// part is one reproduced text: a whole file, or lines From to To of it.
type part struct {
	Source, Path string
	From, To     int
}

type component struct {
	Name string
	// Libraries are the linked static libraries that contain the code.
	Libraries []string
	// OnlyOn limits the platforms; empty means every platform.
	OnlyOn []string
	Note   string
	Parts  []part
}

func file(path string) part { return part{Source: "duckdb", Path: path} }

func lines(path string, from, to int) part {
	return part{Source: "duckdb", Path: path, From: from, To: to}
}

const tpcLegend = "THE TPC SOFTWARE IS AVAILABLE WITHOUT CHARGE FROM TPC."

// components lists every third-party work found in the linked libraries.
// Not listed because not linked: jemalloc's separate archive on macOS and
// Windows, Snowball (the fts extension), Catch (tests) and the IMDB data
// (benchmarks).
var components = []component{
	{Name: "Apache Arrow ADBC and nanoarrow", Libraries: []string{"libduckdb_static"},
		Note: "Apache License 2.0; its full text is in the Apache Parquet section.",
		Parts: []part{lines("src/include/duckdb/common/adbc/adbc.h", 1, 22),
			lines("src/common/adbc/nanoarrow/allocator.cpp", 1, 16)}},
	{Name: "ALP and ALP-RD compression", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("src/include/duckdb/storage/compression/alp/algorithm/LICENSE"),
			file("src/include/duckdb/storage/compression/alprd/algorithm/LICENSE")}},
	{Name: "Brotli", Libraries: []string{"libparquet_extension"},
		Parts: []part{file("third_party/brotli/LICENSE")}},
	{Name: "concurrentqueue", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/concurrentqueue/LICENSE")}},
	{Name: "fast_float", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/fast_float/LICENSE")}},
	{Name: "FastPFor", Libraries: []string{"libduckdb_fastpforlib", "libparquet_extension"},
		Parts: []part{file("third_party/fastpforlib/LICENSE")}},
	{Name: "{fmt}", Libraries: []string{"libduckdb_fmt", "libcore_functions_extension"},
		Parts: []part{file("third_party/fmt/LICENSE")}},
	{Name: "FSST", Libraries: []string{"libduckdb_fsst"},
		Parts: []part{file("third_party/fsst/LICENSE")}},
	{Name: "cpp-httplib", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/httplib/LICENSE")}},
	{Name: "HyperLogLog", Libraries: []string{"libduckdb_hyperloglog"},
		Parts: []part{file("third_party/hyperloglog/LICENSE")}},
	{Name: "ICU", Libraries: []string{"libicu_extension"},
		Parts: []part{file("extension/icu/third_party/icu/LICENSE")}},
	{Name: "jaro_winkler", Libraries: []string{"libduckdb_static", "libcore_functions_extension"},
		Parts: []part{file("third_party/jaro_winkler/LICENSE")}},
	{Name: "jemalloc", Libraries: []string{"libduckdb_static"}, OnlyOn: []string{"linux/amd64"},
		Parts: []part{file("third_party/jemalloc/LICENSE")}},
	{Name: "libpg_query", Libraries: []string{"libduckdb_pg_query"},
		Parts: []part{file("third_party/libpg_query/LICENSE")}},
	{Name: "LZ4", Libraries: []string{"libparquet_extension"},
		Parts: []part{file("third_party/lz4/LICENSE")}},
	{Name: "Mbed TLS", Libraries: []string{"libduckdb_mbedtls"},
		Parts: []part{file("third_party/mbedtls/LICENSE")}},
	{Name: "MD5", Libraries: []string{"libduckdb_static"},
		Parts: []part{lines("src/common/crypto/md5.cpp", 1, 21)}},
	{Name: "Merge sort tree", Libraries: []string{"libduckdb_static"},
		Parts: []part{lines("src/include/duckdb/execution/merge_sort_tree.hpp", 25, 47)}},
	{Name: "miniz", Libraries: []string{"libduckdb_miniz"},
		Parts: []part{file("third_party/miniz/LICENSE")}},
	{Name: "Apache Parquet", Libraries: []string{"libparquet_extension"},
		Parts: []part{file("third_party/parquet/LICENSE")}},
	{Name: "PCG random number generation", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/pcg/LICENSE")}},
	{Name: "pdqsort", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/pdqsort/LICENSE")}},
	{Name: "RE2", Libraries: []string{"libduckdb_re2"},
		Parts: []part{file("third_party/re2/LICENSE")}},
	{Name: "robin-hood-hashing", Libraries: []string{"libduckdb_static"},
		Parts: []part{lines("src/common/checksum.cpp", 11, 13),
			{Source: "robin-hood-hashing", Path: "LICENSE"}}},
	{Name: "ska_sort", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/ska_sort/LICENSE")}},
	{Name: "skiplist", Libraries: []string{"libduckdb_skiplistlib", "libcore_functions_extension"},
		Parts: []part{file("third_party/skiplist/LICENSE")}},
	{Name: "Snappy", Libraries: []string{"libparquet_extension"},
		Parts: []part{file("third_party/snappy/LICENSE")}},
	{Name: "t-digest", Libraries: []string{"libcore_functions_extension"},
		Parts: []part{file("third_party/tdigest/LICENSE"), file("third_party/tdigest/NOTICES")}},
	{Name: "Apache Thrift", Libraries: []string{"libparquet_extension"},
		Parts: []part{file("third_party/thrift/thrift/LICENSE")}},
	{Name: "TPC-DS data generator (dsdgen)", Libraries: []string{"libtpcds_extension"},
		Note:  tpcLegend + " DuckDB's copy of dsdgen has no separate license file; its sources carry the legal notice below.",
		Parts: []part{lines("extension/tpcds/dsdgen/include/dsdgen-c/decimal.h", 1, 35)}},
	{Name: "TPC-H data generator (dbgen)", Libraries: []string{"libtpch_extension"},
		Note:  tpcLegend,
		Parts: []part{file("extension/tpch/dbgen/LICENSE")}},
	{Name: "utf8proc", Libraries: []string{"libduckdb_utf8proc"},
		Parts: []part{file("third_party/utf8proc/LICENSE")}},
	{Name: "vergesort", Libraries: []string{"libduckdb_static"},
		Parts: []part{file("third_party/vergesort/LICENSE")}},
	{Name: "yyjson", Libraries: []string{"libduckdb_yyjson", "libjson_extension"},
		Parts: []part{file("third_party/yyjson/LICENSE")}},
	{Name: "Zstandard", Libraries: []string{"libduckdb_zstd"},
		Parts: []part{file("third_party/zstd/LICENSE")}},
}

func main() {
	output := flag.String("o", defaultOutput, "notices file to write or check")
	check := flag.Bool("check", false, "compare instead of writing; without sources, check only the pins")
	fetch := flag.Bool("fetch", false, "clone the pinned sources into a temporary directory")
	duckdbDir := flag.String("duckdb", "", "checkout of "+sources["duckdb"].URL+" at the pinned commit")
	robinDir := flag.String("robin-hood", "", "checkout of "+sources["robin-hood-hashing"].URL+" at the pinned commit")
	flag.Parse()
	if err := run(*output, *check, *fetch, map[string]string{"duckdb": *duckdbDir, "robin-hood-hashing": *robinDir}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "duckdb-notices: %v\n", err)
		os.Exit(1)
	}
}

func run(output string, check, fetch bool, dirs map[string]string) error {
	if err := checkModuleVersions(moduleVersion); err != nil {
		return err
	}
	if fetch {
		tmp, err := os.MkdirTemp("", "duckdb-notices-")
		if err != nil {
			return fmt.Errorf("create a temporary directory: %w", err)
		}
		defer os.RemoveAll(tmp)
		for name, src := range sources {
			dir := filepath.Join(tmp, name)
			if err := cloneSource(src, dir); err != nil {
				return err
			}
			dirs[name] = dir
		}
	}
	haveSources := dirs["duckdb"] != "" || dirs["robin-hood-hashing"] != ""
	if !haveSources {
		if !check {
			return errors.New("give -fetch, or -duckdb and -robin-hood, to regenerate; or -check to verify the pins")
		}
		return checkPins(output)
	}
	for name, src := range sources {
		if err := verifyCheckout(src, dirs[name]); err != nil {
			return err
		}
	}
	content, err := generate(dirs)
	if err != nil {
		return err
	}
	if check {
		have, err := os.ReadFile(output) // #nosec G304 -- the notices file named by the caller.
		if err != nil {
			return fmt.Errorf("read %s: %w", output, err)
		}
		if !bytes.Equal(have, content) {
			return fmt.Errorf("%s does not match the pinned sources; run `make duckdb-notices` and review the diff", output)
		}
		fmt.Printf("duckdb-notices: %s matches the pinned sources\n", output)
		return nil
	}
	if err := os.WriteFile(output, content, 0o644); err != nil { // #nosec G306 -- a public notices file.
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("duckdb-notices: wrote %s\n", output)
	return nil
}

// moduleVersion reports the version go.mod selects for a module.
func moduleVersion(module string) (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", module).Output()
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w", module, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// checkModuleVersions fails when go.mod moves duckdb-go-bindings away from
// the version whose libraries were reviewed.
func checkModuleVersions(version func(string) (string, error)) error {
	modules := []string{bindingsModule}
	for _, p := range platforms {
		modules = append(modules, p.Module)
	}
	for _, m := range modules {
		v, err := version(m)
		if err != nil {
			return err
		}
		if v != bindingsVersion {
			return fmt.Errorf("go.mod selects %s %s, but the DuckDB native notices were reviewed for %s; "+
				"review the bundled components of the new DuckDB release (LICENSES.md), update the pins in tools/duckdb-notices, then run `make duckdb-notices`",
				m, v, bindingsVersion)
		}
	}
	return nil
}

// checkPins verifies, without the sources, that the committed file was
// generated for the current pins and component list.
func checkPins(output string) error {
	have, err := os.ReadFile(output) // #nosec G304 -- the notices file named by the caller.
	if err != nil {
		return fmt.Errorf("read %s: %w; run `make duckdb-notices`", output, err)
	}
	lines := map[string]bool{}
	for _, l := range strings.Split(string(have), "\n") {
		lines[l] = true
	}
	var missing []string
	for _, want := range headerLines() {
		if !lines[want] {
			missing = append(missing, want)
		}
	}
	for _, c := range components {
		if want := "Component: " + c.Name; !lines[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s is stale; it lacks %q; run `make duckdb-notices`", output, missing)
	}
	fmt.Printf("duckdb-notices: %s matches the pins for %s %s\n", output, bindingsModule, bindingsVersion)
	return nil
}

func cloneSource(src source, dir string) error {
	cmd := exec.Command("git", "clone", "--quiet", "--depth", "1", "--branch", src.Tag, src.URL, dir)
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1", "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clone %s %s: %w: %s", src.URL, src.Tag, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func verifyCheckout(src source, dir string) error {
	if dir == "" {
		return fmt.Errorf("no checkout given for %s", src.Name)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("read the commit of %s: %w", dir, err)
	}
	if got := strings.TrimSpace(string(out)); got != src.Commit {
		return fmt.Errorf("%s is at %s; %s %s is %s", dir, got, src.Name, src.Tag, src.Commit)
	}
	return nil
}

func generate(dirs map[string]string) ([]byte, error) {
	var b strings.Builder
	for _, l := range preamble() {
		b.WriteString(l + "\n")
	}
	for _, c := range sortedComponents() {
		b.WriteString("\n" + rule + "\n")
		fmt.Fprintf(&b, "Component: %s\n", c.Name)
		fmt.Fprintf(&b, "Compiled into: %s\n", strings.Join(c.Libraries, ", "))
		fmt.Fprintf(&b, "Linked into: %s\n", strings.Join(c.platforms(), ", "))
		if c.Note != "" {
			fmt.Fprintf(&b, "Note: %s\n", c.Note)
		}
		for _, p := range c.Parts {
			text, err := readPart(dirs[p.Source], p)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.Name, err)
			}
			fmt.Fprintf(&b, "\n----- %s -----\n\n", p.label())
			b.WriteString(text)
		}
	}
	return []byte(b.String()), nil
}

const rule = "================================================================================"

func headerLines() []string {
	duck := sources["duckdb"]
	lines := []string{
		fmt.Sprintf("Reviewed for: %s %s", bindingsModule, bindingsVersion),
		fmt.Sprintf("DuckDB source: %s %s, commit %s", duck.URL, duck.Tag, duck.Commit),
	}
	for _, p := range platforms {
		lines = append(lines, fmt.Sprintf("Covers: %s %s", p.Module, bindingsVersion))
	}
	return lines
}

func preamble() []string {
	out := []string{
		"DuckDB native library notices for GoPMgr",
		"========================================",
		"",
		tpcLegend,
		"",
		"GoPMgr's release binaries embed DuckDB through the Go module",
		bindingsModule + ", whose platform modules ship DuckDB as",
		"prebuilt static libraries. Besides DuckDB itself, whose MIT license is in",
		"GO-MODULE-NOTICES.txt, those libraries contain the third-party code listed",
		"below. Each section reproduces that code's license and notice texts,",
		"unchanged except for line endings and trailing spaces, from the DuckDB",
		"source the libraries were built from.",
		"",
	}
	out = append(out, headerLines()...)
	out = append(out, "",
		"Generated by tools/duckdb-notices. Do not edit this file by hand; see",
		"LICENSES.md for how to update it.",
		"",
		"Contents",
		"--------",
		"")
	for _, c := range sortedComponents() {
		out = append(out, "  "+c.Name)
	}
	return out
}

func sortedComponents() []component {
	sorted := append([]component(nil), components...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

func (c component) platforms() []string {
	if len(c.OnlyOn) > 0 {
		return c.OnlyOn
	}
	names := make([]string, 0, len(platforms))
	for _, p := range platforms {
		names = append(names, p.Name)
	}
	return names
}

func (p part) label() string {
	if p.Source != "duckdb" {
		s := sources[p.Source]
		return fmt.Sprintf("%s %s: %s", s.Name, s.Tag, p.Path)
	}
	if p.From > 0 {
		return fmt.Sprintf("%s, lines %d-%d", p.Path, p.From, p.To)
	}
	return p.Path
}

// readPart reads a part and normalizes only its whitespace.
func readPart(dir string, p part) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("no checkout for source %q", p.Source)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p.Path))) // #nosec G304 -- a path in the pinned source table.
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p.Path, err)
	}
	if p.From > 0 {
		all := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if p.From > p.To || p.To > len(all) {
			return "", fmt.Errorf("%s has %d lines; lines %d-%d are out of range", p.Path, len(all), p.From, p.To)
		}
		data = []byte(strings.Join(all[p.From-1:p.To], "\n"))
	}
	text, err := normalizeText(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", p.Path, err)
	}
	return text, nil
}

// normalizeText matches tools/module-notices: LF line endings, no trailing
// spaces, no byte-order mark, one final newline.
func normalizeText(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", errors.New("not valid UTF-8 text")
	}
	text := strings.TrimPrefix(string(data), "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\f\v")
	}
	text = strings.Trim(strings.Join(lines, "\n"), "\n")
	if text == "" {
		return "", errors.New("empty text")
	}
	return text + "\n", nil
}
