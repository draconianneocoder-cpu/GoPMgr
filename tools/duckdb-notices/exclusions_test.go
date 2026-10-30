// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadExclusions(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    exclusions
		wantErr string
	}{
		{
			name: "archives and markers, comments and blank lines skipped",
			text: "# comment\n\narchive liba.a\n  marker some text  \narchive libb.a\n",
			want: exclusions{Archives: []string{"liba.a", "libb.a"}, Markers: []string{"some text"}},
		},
		{name: "unknown kind", text: "archive liba.a\nmarker m\nsymbol x\n", wantErr: ":3: want"},
		{name: "missing value", text: "archive liba.a\nmarker\n", wantErr: ":2: want"},
		{name: "no marker", text: "archive liba.a\n", wantErr: "no archive or no marker"},
		{name: "no archive", text: "marker m\n", wantErr: "no archive or no marker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x.txt")
			if err := os.WriteFile(path, []byte(tt.text), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := readExclusions(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
	if _, err := readExclusions(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("a missing file must be an error")
	}
}

// The checked-in list must keep excluding both TPC generators.
func TestRepositoryExclusions(t *testing.T) {
	ex, err := readExclusions(filepath.Join("..", "..", exclusionsFile))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"libtpch_extension.a", "libtpcds_extension.a"}
	if !reflect.DeepEqual(ex.Archives, want) {
		t.Fatalf("archives = %q, want %q", ex.Archives, want)
	}
	for _, m := range []string{"N6duckdb13TpchExtensionE", "N6duckdb14TpcdsExtensionE"} {
		found := false
		for _, have := range ex.Markers {
			found = found || have == m
		}
		if !found {
			t.Errorf("markers lack %q", m)
		}
	}
	notices, err := os.ReadFile(filepath.Join("..", "..", "third_party", "DUCKDB-NATIVE-NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(notices), "Component: TPC") {
		t.Fatal("DUCKDB-NATIVE-NOTICES.txt lists a TPC component; the generators are excluded, not shipped")
	}
}

func arHeader(name string, size int) string {
	return fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10d`\n", name, "0", "0", "0", "644", size)
}

func arEntry(name, body string) string {
	s := arHeader(name, len(body)) + body
	if len(body)%2 == 1 {
		s += "\n"
	}
	return s
}

func TestArMembers(t *testing.T) {
	longName := "a_very_long_object_name.o"
	gnu := "!<arch>\n" +
		arEntry("/", "symbol index") +
		arEntry("//", longName+"/\n") +
		arEntry("short.o/", "one") +
		arEntry("/0", "two!")
	bsdName := "a_long_bsd_member_name.o"
	bsd := "!<arch>\n" +
		arEntry("#1/20", "__.SYMDEF SORTED\x00\x00\x00\x00index") +
		arEntry(fmt.Sprintf("#1/%d", len(bsdName)), bsdName+"three") +
		arEntry("plain.o", "four")
	tests := []struct {
		name    string
		data    string
		want    []string // name=body pairs
		wantErr string
	}{
		{name: "GNU long names and symbol index", data: gnu, want: []string{"short.o=one", longName + "=two!"}},
		{name: "BSD long names and __.SYMDEF", data: bsd, want: []string{bsdName + "=three", "plain.o=four"}},
		{name: "not an archive", data: "ELF", wantErr: "not an ar archive"},
		{name: "truncated header", data: "!<arch>\nshort", wantErr: "truncated member header"},
		{name: "bad size", data: "!<arch>\n" + arHeader("x.o", 99) + "abc", wantErr: "bad member size"},
		{name: "bad long-name reference", data: "!<arch>\n" + arEntry("/7", "x"), wantErr: "bad long-name reference"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members, err := arMembers([]byte(tt.data))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, m := range members {
				got = append(got, m.name+"="+string(m.data))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLibModules(t *testing.T) {
	gomod := []byte("module x\n\nrequire (\n" +
		"\tgithub.com/duckdb/duckdb-go-bindings v0.1.0 // indirect\n" +
		"\tgithub.com/duckdb/duckdb-go-bindings/lib/linux-amd64 v0.1.0 // indirect\n" +
		"\tgithub.com/duckdb/duckdb-go-bindings/lib/darwin-arm64 v0.1.0 // indirect\n" +
		")\n" +
		"require github.com/duckdb/duckdb-go-bindings/lib/windows-amd64 v0.1.0\n")
	want := []string{
		bindingsModule + "/lib/darwin-arm64",
		bindingsModule + "/lib/linux-amd64",
		bindingsModule + "/lib/windows-amd64",
	}
	if got := libModules(gomod); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	repoMod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	got := libModules(repoMod)
	for _, p := range platforms {
		found := false
		for _, m := range got {
			found = found || m == p.Module
		}
		if !found {
			t.Errorf("go.mod modules %q lack release platform %s", got, p.Module)
		}
	}
}

func TestCgoNames(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package a\n\n/*\n#include <x.h>\n*/\nimport \"C\"\n\nfunc f() { C.duckdb_open(nil); _ = C.size_t(1) }\n")
	write("b.go", "package a\n\n// Not cgo: C.not_counted\nfunc g() {}\n")
	got, err := cgoNames([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"duckdb_open": true, "size_t": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := cgoNames([]string{t.TempDir()}); err == nil {
		t.Fatal("a directory without cgo calls must be an error")
	}
}

// TestExclusionsHold runs the release-blocking audit against the pinned
// platform modules. It fails, rather than skips, when a module is missing:
// the maintainer's decision of 2026-10-26 makes this gate release-blocking.
func TestExclusionsHold(t *testing.T) {
	ex, err := readExclusions(filepath.Join("..", "..", exclusionsFile))
	if err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var cgoDirs []string
	for _, m := range cgoModules {
		dir, err := moduleDir(m)
		if err != nil {
			t.Fatal(err)
		}
		cgoDirs = append(cgoDirs, dir)
	}
	cgo, err := cgoNames(cgoDirs)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range libModules(gomod) {
		t.Run(m, func(t *testing.T) {
			dir, err := moduleDir(m)
			if err != nil {
				t.Fatal(err)
			}
			rep, err := auditExclusions(m, dir, ex, cgo)
			if err != nil {
				t.Fatal(err)
			}
			if p := rep.problems(); len(p) > 0 {
				t.Fatalf("problems: %q", p)
			}
			if rep.ExcludedDefs == 0 {
				t.Fatal("the excluded archives define no symbols; the audit read nothing")
			}
		})
	}
}

// fakeModule builds a platform module from C sources: one archive per
// entry, all named on a cgo LDFLAGS line in link order.
func fakeModule(t *testing.T, archives [][2]string) string {
	t.Helper()
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	dir := t.TempDir()
	var flags []string
	for i, a := range archives {
		lib, src := a[0], a[1]
		c := filepath.Join(dir, fmt.Sprintf("src%d.c", i))
		o := filepath.Join(dir, fmt.Sprintf("obj%d.o", i))
		if err := os.WriteFile(c, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(cc, "-c", "-o", o, c).CombinedOutput(); err != nil {
			t.Fatalf("compile %s (CGO is required to build GoPMgr): %v\n%s", lib, err, out)
		}
		if out, err := exec.Command("ar", "rcs", filepath.Join(dir, "lib"+lib+".a"), o).CombinedOutput(); err != nil {
			t.Fatalf("ar %s: %v\n%s", lib, err, out)
		}
		flags = append(flags, "-l"+lib)
	}
	prebuilt := "package lib\n\n/*\n#cgo LDFLAGS: " + strings.Join(flags, " ") + "\n*/\nimport \"C\"\n"
	if err := os.WriteFile(filepath.Join(dir, "prebuilt.go"), []byte(prebuilt), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAuditExclusionsFindsProblems(t *testing.T) {
	const (
		tpch     = `const char tpc_note[] = "TPC-SECRET-MARKER"; int tpch_generate(void) { return 1; }`
		core     = `const char core_note[] = "core text"; int core_query(void) { return 2; }`
		coreUses = `int tpch_generate(void); int core_query(void) { return tpch_generate(); }`
		leaky    = `const char core_note[] = "TPC-SECRET-MARKER"; int core_query(void) { return 2; }`
	)
	ex := exclusions{Archives: []string{"libtpch_extension.a"}, Markers: []string{"TPC-SECRET-MARKER"}}
	tests := []struct {
		name     string
		archives [][2]string
		ex       exclusions
		cgo      map[string]bool
		want     exclusionReport
		wantErr  string
	}{
		{
			name:     "excluded archive on the link line but unreferenced",
			archives: [][2]string{{"core", core}, {"tpch_extension", tpch}},
			ex:       ex,
		},
		{
			name:     "a linked archive calls into the excluded one",
			archives: [][2]string{{"core", coreUses}, {"tpch_extension", tpch}},
			ex:       ex,
			want:     exclusionReport{Referenced: []string{"tpch_generate"}},
		},
		{
			name:     "the Go bindings call into the excluded archive",
			archives: [][2]string{{"core", core}, {"tpch_extension", tpch}},
			ex:       ex,
			cgo:      map[string]bool{"tpch_generate": true},
			want:     exclusionReport{Referenced: []string{"tpch_generate"}},
		},
		{
			name:     "a marker the excluded code lacks",
			archives: [][2]string{{"core", core}, {"tpch_extension", tpch}},
			ex:       exclusions{Archives: ex.Archives, Markers: []string{"TPC-SECRET-MARKER", "not in any archive"}},
			want:     exclusionReport{MissingMarkers: []string{"not in any archive"}},
		},
		{
			name:     "a marker another linked archive contains",
			archives: [][2]string{{"core", leaky}, {"tpch_extension", tpch}},
			ex:       ex,
			want:     exclusionReport{LeakedMarkers: []string{"TPC-SECRET-MARKER"}},
		},
		{
			name:     "excluded archive missing from the link line",
			archives: [][2]string{{"core", core}},
			ex:       ex,
			wantErr:  "name 0 of the 1 excluded archives",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fakeModule(t, tt.archives)
			rep, err := auditExclusions("fake", dir, tt.ex, tt.cgo)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rep.ExcludedDefs == 0 {
				t.Fatal("read no symbols from the excluded archive")
			}
			// Mach-O names carry a leading underscore.
			for i, s := range rep.Referenced {
				rep.Referenced[i] = strings.TrimPrefix(s, "_")
			}
			tt.want.Platform, tt.want.ExcludedDefs = "fake", rep.ExcludedDefs
			if !reflect.DeepEqual(rep, tt.want) {
				t.Fatalf("got %+v, want %+v", rep, tt.want)
			}
			if wantClean := tt.want.Referenced == nil && tt.want.MissingMarkers == nil && tt.want.LeakedMarkers == nil; wantClean != (len(rep.problems()) == 0) {
				t.Fatalf("problems() = %q, want clean=%v", rep.problems(), wantClean)
			}
		})
	}
}
