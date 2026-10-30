// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb

package analytics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func importFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	eng := New().(*duckEngine)
	t.Cleanup(func() { _ = eng.Close() })
	if _, err := eng.db.ExecContext(context.Background(), "SET autoinstall_known_extensions=false"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fixture.parquet")
	if _, err := eng.db.ExecContext(context.Background(), "COPY (SELECT 'selected' AS marker, 'ok' AS tag) TO '"+sqlSingleQuote(p)+"' (FORMAT PARQUET)"); err != nil {
		t.Fatal(err)
	}
	parquet, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"csv":     []byte("marker,tag\nselected,ok\n"),
		"tsv":     []byte("marker\ttag\nselected\tok\n"),
		"txt":     []byte("marker,tag\nselected,ok\n"),
		"json":    []byte(`[{"marker":"selected","tag":"ok"}]`),
		"ndjson":  []byte("{\"marker\":\"selected\",\"tag\":\"ok\"}\n"),
		"parquet": parquet,
	}
}

func writeImportFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertSelectedImport(t *testing.T, eng Engine, path string) {
	t.Helper()
	ds, err := eng.ImportTabular(context.Background(), path)
	if err != nil {
		t.Fatalf("ImportTabular(%q): %v", path, err)
	}
	want := Dataset{Columns: []string{"marker", "tag"}, Rows: [][]any{{"selected", "ok"}}}
	if !reflect.DeepEqual(ds, want) {
		t.Fatalf("ImportTabular(%q) = %#v, want %#v", path, ds, want)
	}
}

func assertAmbiguousImport(t *testing.T, eng Engine, path string) {
	t.Helper()
	ds, err := eng.ImportTabular(context.Background(), path)
	if !errors.Is(err, ErrAmbiguousImportPath) {
		t.Fatalf("ImportTabular(%q): want ErrAmbiguousImportPath, got %v; dataset %#v", path, err, ds)
	}
	if !reflect.DeepEqual(ds, Dataset{}) {
		t.Fatalf("rejected import returned data: %#v", ds)
	}
}

func TestImportTabularRejectsGlobsAndRecoversAcrossFormats(t *testing.T) {
	for ext, body := range importFixtures(t) {
		t.Run(ext, func(t *testing.T) {
			eng := New()
			t.Cleanup(func() { _ = eng.Close() })
			dir := t.TempDir()
			valid := filepath.Join(dir, "valid."+ext)
			writeImportFile(t, valid, body)
			assertSelectedImport(t, eng, valid)
			for _, name := range []string{"data*", "data?", "data[12]", "data["} {
				if runtime.GOOS == "windows" && strings.ContainsAny(name, "*?") {
					continue // These literal filename characters are invalid on Windows.
				}
				for _, placement := range []string{"filename", "parent"} {
					t.Run(name+"/"+placement, func(t *testing.T) {
						selected := filepath.Join(dir, name+"."+ext)
						neighbor := filepath.Join(dir, "data1."+ext)
						if placement == "parent" {
							selected = filepath.Join(dir, name, "data."+ext)
							neighbor = filepath.Join(dir, "data1", "data."+ext)
						}
						writeImportFile(t, selected, body)
						// A corrupt neighbor also detects readers trying to inspect it.
						writeImportFile(t, neighbor, []byte("unselected-neighbor\x00invalid"))
						assertAmbiguousImport(t, eng, selected)
						assertSelectedImport(t, eng, valid)
					})
				}
			}
		})
	}
}

func TestImportTabularSymlinksKeepSourceIdentity(t *testing.T) {
	body := []byte("marker,tag\nselected,ok\n")
	dir := t.TempDir()
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, name); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink creation requires privileges: %v", err)
			}
			t.Fatal(err)
		}
	}
	eng := New()
	t.Cleanup(func() { _ = eng.Close() })

	// Format comes from the selected name, even if its target has no extension.
	target := filepath.Join(dir, "source")
	writeImportFile(t, target, body)
	alias := filepath.Join(dir, "alias.csv")
	link(target, alias)
	assertSelectedImport(t, eng, alias)

	// A normal alias must not hide a glob-bearing canonical source.
	globTarget := filepath.Join(dir, "source[1].csv")
	writeImportFile(t, globTarget, body)
	globAlias := filepath.Join(dir, "normal.csv")
	link(globTarget, globAlias)
	assertAmbiguousImport(t, eng, globAlias)
	globName := filepath.Join(dir, "alias[1].csv")
	link(target, globName)
	assertAmbiguousImport(t, eng, globName)
	globParentTarget := filepath.Join(dir, "folder[1]", "data.csv")
	writeImportFile(t, globParentTarget, body)
	parentAlias := filepath.Join(dir, "parent-alias.csv")
	link(globParentTarget, parentAlias)
	assertAmbiguousImport(t, eng, parentAlias)
	assertSelectedImport(t, eng, alias)

	// Cleaning jump/.. first selects the sibling decoy rather than the target.
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(realDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeImportFile(t, filepath.Join(realDir, "selected.csv"), body)
	writeImportFile(t, filepath.Join(dir, "selected.csv"), []byte("marker,tag\ndecoy,wrong\n"))
	jump := filepath.Join(dir, "jump")
	link(filepath.Join(realDir, "nested"), jump)
	assertSelectedImport(t, eng, jump+string(filepath.Separator)+".."+string(filepath.Separator)+"selected.csv")
	if err := os.Remove(filepath.Join(dir, "selected.csv")); err != nil {
		t.Fatal(err)
	}
	assertSelectedImport(t, eng, jump+string(filepath.Separator)+".."+string(filepath.Separator)+"selected.csv")

	// Canonical-only refusals must precede connection acquisition too.
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
	assertAmbiguousImport(t, eng, globAlias)
	assertAmbiguousImport(t, eng, parentAlias)
}

func TestImportTabularLiteralNonGlobNames(t *testing.T) {
	eng := New()
	t.Cleanup(func() { _ = eng.Close() })
	for _, name := range []string{"résumé 2026.csv", "it's-data.csv", "data].csv", "data{1,2}.csv", "data\\literal.csv"} {
		if runtime.GOOS == "windows" && strings.Contains(name, "\\") {
			continue // A backslash is a path separator on Windows.
		}
		p := filepath.Join(t.TempDir(), name)
		writeImportFile(t, p, []byte("marker,tag\nselected,ok\n"))
		assertSelectedImport(t, eng, p)
	}
}

func TestImportTabularFailureRecoveryAndRowLimit(t *testing.T) {
	eng := New()
	t.Cleanup(func() { _ = eng.Close() })
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.csv")
	writeImportFile(t, valid, []byte("marker,tag\nselected,ok\n"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ds, err := eng.ImportTabular(ctx, valid); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(ds, Dataset{}) {
		t.Fatalf("cancelled import = %#v, %v", ds, err)
	}
	assertSelectedImport(t, eng, valid)
	broken := filepath.Join(dir, "broken.parquet")
	writeImportFile(t, broken, []byte("not parquet"))
	if ds, err := eng.ImportTabular(context.Background(), broken); err == nil || !reflect.DeepEqual(ds, Dataset{}) {
		t.Fatalf("broken parquet = %#v, %v", ds, err)
	}
	assertSelectedImport(t, eng, valid)
	large := filepath.Join(dir, "large.csv")
	writeImportFile(t, large, []byte("marker,tag\n"+strings.Repeat("selected,ok\n", 10001)))
	ds, err := eng.ImportTabular(context.Background(), large)
	if err != nil || len(ds.Rows) != 10000 {
		t.Fatalf("large import: rows=%d, err=%v", len(ds.Rows), err)
	}
}

func TestImportTabularRejectsGlobsBeforeLookupOrConnection(t *testing.T) {
	eng := New()
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
	// Neither a missing source nor a closed engine should hide the policy error.
	assertAmbiguousImport(t, eng, filepath.Join(t.TempDir(), "missing[1].csv"))
	t.Run("working-directory", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "source.csv")
		writeImportFile(t, target, []byte("marker,tag\nselected,ok\n"))
		working := filepath.Join(dir, "folder[1]")
		if err := os.Mkdir(working, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(working, "alias.csv")); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink creation requires privileges: %v", err)
			}
			t.Fatal(err)
		}
		t.Chdir(working)
		assertAmbiguousImport(t, eng, "alias.csv")
	})
}

func TestImportTabularPreservesSelectedPathPartitions(t *testing.T) {
	for ext, body := range importFixtures(t) {
		t.Run(ext, func(t *testing.T) {
			for _, c := range []struct {
				name, target, selected string
				year                   int64
			}{
				{"target-partition", "year=2026/data.", "alias.", 0},
				{"selected-partition", "data.", "year=2025/alias.", 2025},
				{"direct-partition", "year=2027/data.", "year=2027/data.", 2027},
			} {
				t.Run(c.name, func(t *testing.T) {
					dir := t.TempDir()
					target := filepath.Join(dir, c.target+ext)
					selected := filepath.Join(dir, c.selected+ext)
					writeImportFile(t, target, body)
					if target != selected {
						if err := os.MkdirAll(filepath.Dir(selected), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(target, selected); err != nil {
							if runtime.GOOS == "windows" {
								t.Skipf("symlink creation requires privileges: %v", err)
							}
							t.Fatal(err)
						}
					}
					eng := New()
					t.Cleanup(func() { _ = eng.Close() })
					ds, err := eng.ImportTabular(context.Background(), selected)
					if err != nil {
						t.Fatal(err)
					}
					want := Dataset{Columns: []string{"marker", "tag"}, Rows: [][]any{{"selected", "ok"}}}
					if c.year != 0 {
						want.Columns = append(want.Columns, "year")
						want.Rows[0] = append(want.Rows[0], c.year)
					}
					if !reflect.DeepEqual(ds, want) {
						t.Fatalf("selected-path partition result = %#v, want %#v", ds, want)
					}
				})
			}
		})
	}
}
