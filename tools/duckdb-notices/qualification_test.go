// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkedArchivesRejectsUnreviewedInputs(t *testing.T) {
	for _, flags := range []string{
		"-lmissing", "/tmp/direct.a", "-l:direct.a", "-Wl,--whole-archive", "-all_load",
		"-Wl,-force_load,/tmp/direct.a", "-Wl,-u,tpch_generate", "-L/tmp/other", "-unknown",
	} {
		t.Run(flags, func(t *testing.T) {
			dir := fakeModule(t, [][2]string{{"core", "int query(void) { return 1; }"}})
			p := filepath.Join(dir, "prebuilt.go")
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			src = []byte(strings.Replace(string(src), "LDFLAGS: -lcore", "LDFLAGS: -lcore "+flags, 1))
			if err := os.WriteFile(p, src, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := linkedArchives(dir); err == nil {
				t.Fatal("unreviewed link input passed")
			}
		})
	}
	t.Run("conditional directive", func(t *testing.T) {
		dir := fakeModule(t, [][2]string{{"core", "int query(void) { return 1; }"}})
		p := filepath.Join(dir, "prebuilt.go")
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(src), "#cgo LDFLAGS:", "#cgo darwin LDFLAGS:", 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := linkedArchives(dir); err == nil {
			t.Fatal("conditional flags passed without review")
		}
	})
	t.Run("missing previously linked archive", func(t *testing.T) {
		dir := fakeModule(t, [][2]string{{"core", "int query(void) { return 1; }"}, {"other", "int other(void) { return 2; }"}})
		if err := os.Remove(filepath.Join(dir, "libother.a")); err != nil {
			t.Fatal(err)
		}
		if _, err := linkedArchives(dir); err == nil {
			t.Fatal("missing required archive was ignored")
		}
	})
}

func TestObjectSymbolsRejectsMissingEvidence(t *testing.T) {
	elf := make([]byte, 64)
	copy(elf, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(elf[16:], 1)
	binary.LittleEndian.PutUint16(elf[18:], 62)
	binary.LittleEndian.PutUint32(elf[20:], 1)
	binary.LittleEndian.PutUint16(elf[52:], 64)
	mach := make([]byte, 32)
	copy(mach, []byte{0xcf, 0xfa, 0xed, 0xfe})
	binary.LittleEndian.PutUint32(mach[12:], 1)
	coff := make([]byte, 20)
	binary.LittleEndian.PutUint16(coff, 0x8664)
	for name, body := range map[string][]byte{"unknown": []byte("opaque object"), "truncated": {0xff}, "ELF without symbols": elf, "Mach-O without symbols": mach, "COFF without symbols": coff} {
		t.Run(name, func(t *testing.T) {
			sets := symbolSets{Defined: map[string]bool{}, Undefined: map[string]bool{}}
			if err := objectSymbols(body, sets); err == nil {
				t.Fatal("missing symbol evidence passed")
			}
		})
	}
}

func TestAuditExclusionsRejectsUnreadableMembersAndEmptyDefinitions(t *testing.T) {
	for _, ordinary := range []bool{false, true} {
		t.Run(map[bool]string{false: "excluded", true: "ordinary"}[ordinary], func(t *testing.T) {
			dir := fakeModule(t, [][2]string{{"core", "int query(void) {return 1;}"}, {"tpch_extension", `const char note[]="TPC-MARK"; int tpch_generate(void){return 1;}`}})
			name := "libtpch_extension.a"
			if ordinary {
				name = "libcore.a"
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("!<arch>\n"+arEntry("opaque.o", "unknown TPC-MARK")), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := auditExclusions("fake", dir, exclusions{Archives: []string{"libtpch_extension.a"}, Markers: []string{"TPC-MARK"}}, nil); err == nil {
				t.Fatal("unreadable archive member passed")
			}
		})
	}
	t.Run("no external definitions", func(t *testing.T) {
		dir := fakeModule(t, [][2]string{{"core", "int query(void){return 1;}"}, {"tpch_extension", `__attribute__((used)) static const char note[]="TPC-MARK";`}})
		if _, err := auditExclusions("fake", dir, exclusions{Archives: []string{"libtpch_extension.a"}, Markers: []string{"TPC-MARK"}}, nil); err == nil {
			t.Fatal("excluded archive without external definitions passed")
		}
	})
}

func TestArMembersRejectsUnsafeBounds(t *testing.T) {
	for name, body := range map[string]string{
		"negative BSD length": "!<arch>\n" + arEntry("#1/-1", "abc"),
		"negative GNU offset": "!<arch>\n" + arEntry("//", "name.o/\n") + arEntry("/-1", "obj"),
		"missing odd padding": "!<arch>\n" + arHeader("x.o", 1) + "x",
		"invalid odd padding": "!<arch>\n" + arHeader("x.o", 1) + "x?",
		"oversized body":      "!<arch>\n" + arHeader("x.o", 9999999999) + "x",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := arMembers([]byte(body)); err == nil {
				t.Fatal("malformed archive accepted")
			}
		})
	}
}

func TestCheckPinsRejectsMissingNoticeBodies(t *testing.T) {
	var b strings.Builder
	for _, line := range headerLines() {
		b.WriteString(line + "\n")
	}
	for _, c := range components {
		b.WriteString("Component: " + c.Name + "\n")
	}
	if err := checkPinsText(t, []byte(b.String())); err == nil {
		t.Fatal("headers without license text passed the notice check")
	}
}

func TestCgoNamesFindsSpacedSelectors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wrapper.go"), []byte("package p\nimport \"C\"\nfunc call(){ C. /* wrapper */ tpch_generate() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	names, err := cgoNames([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !names["tpch_generate"] {
		t.Fatalf("spaced selector omitted: %v", names)
	}
}
