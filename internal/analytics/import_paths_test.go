// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

var selectedRow = Dataset{Columns: []string{"marker", "tag"}, Rows: [][]any{{"selected", "ok"}}}

// writeParquet writes a table of string columns through arrow-go. Path and
// limit tests only; format correctness uses the DuckDB-written fixtures.
func writeParquet(t *testing.T, cols map[string][]string, order []string, codec compress.Compression) []byte {
	t.Helper()
	fields := make([]arrow.Field, len(order))
	arrs := make([]arrow.Array, len(order))
	for i, name := range order {
		fields[i] = arrow.Field{Name: name, Type: arrow.BinaryTypes.String}
		b := array.NewStringBuilder(memory.DefaultAllocator)
		b.AppendValues(cols[name], nil)
		arrs[i] = b.NewArray()
	}
	rec := array.NewRecordBatch(arrow.NewSchema(fields, nil), arrs, int64(len(cols[order[0]])))
	tbl := array.NewTableFromRecords(rec.Schema(), []arrow.RecordBatch{rec})
	var buf bytes.Buffer
	props := parquet.NewWriterProperties(parquet.WithCompression(codec))
	if err := pqarrow.WriteTable(tbl, &buf, 1<<20, props, pqarrow.DefaultWriterProps()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func selectedBodies(t *testing.T) map[string][]byte {
	return map[string][]byte{
		"csv":     []byte("marker,tag\nselected,ok\n"),
		"json":    []byte(`[{"marker":"selected","tag":"ok"}]`),
		"parquet": writeParquet(t, map[string][]string{"marker": {"selected"}, "tag": {"ok"}}, []string{"marker", "tag"}, compress.Codecs.Snappy),
	}
}

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation requires privileges: %v", err)
		}
		t.Fatal(err)
	}
}

func assertImport(t *testing.T, path string, want Dataset) {
	t.Helper()
	ds, err := ImportTabular(context.Background(), path)
	if err != nil {
		t.Fatalf("ImportTabular(%q): %v", path, err)
	}
	if !reflect.DeepEqual(ds, want) {
		t.Fatalf("ImportTabular(%q) = %#v, want %#v", path, ds, want)
	}
}

// Names that DuckDB would have expanded as globs are ordinary file names
// now: the selected file is read and its neighbours are not.
func TestImportTabularReadsLiteralNames(t *testing.T) {
	names := []string{"résumé 2026", "it's-data", "data]", "data{1,2}", "data[12]", "data[", "data*", "data?"}
	for ext, body := range selectedBodies(t) {
		for _, name := range names {
			if runtime.GOOS == "windows" && strings.ContainsAny(name, "*?") {
				continue // invalid in Windows file names
			}
			for _, placement := range []string{"filename", "parent"} {
				t.Run(ext+"/"+name+"/"+placement, func(t *testing.T) {
					dir := t.TempDir()
					selected := filepath.Join(dir, name+"."+ext)
					neighbour := filepath.Join(dir, "data1."+ext)
					if placement == "parent" {
						selected = filepath.Join(dir, name, "data."+ext)
						neighbour = filepath.Join(dir, "data1", "data."+ext)
					}
					writeFile(t, selected, body)
					writeFile(t, neighbour, []byte("unselected-neighbour\x00invalid"))
					assertImport(t, selected, selectedRow)
				})
			}
		}
	}
}

func TestImportTabularSymlinksKeepSourceIdentity(t *testing.T) {
	body := []byte("marker,tag\nselected,ok\n")
	dir := t.TempDir()

	// The format comes from the selected name, even if its target has none.
	target := filepath.Join(dir, "source")
	writeFile(t, target, body)
	alias := filepath.Join(dir, "alias.csv")
	symlink(t, target, alias)
	assertImport(t, alias, selectedRow)

	// jump/.. resolves through the symlink, not by cleaning the path first,
	// which would select the decoy beside it.
	realDir := filepath.Join(dir, "real")
	writeFile(t, filepath.Join(realDir, "selected.csv"), body)
	if err := os.MkdirAll(filepath.Join(realDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "selected.csv"), []byte("marker,tag\ndecoy,wrong\n"))
	jump := filepath.Join(dir, "jump")
	symlink(t, filepath.Join(realDir, "nested"), jump)
	viaJump := jump + string(filepath.Separator) + ".." + string(filepath.Separator) + "selected.csv"
	assertImport(t, viaJump, selectedRow)
	if err := os.Remove(filepath.Join(dir, "selected.csv")); err != nil {
		t.Fatal(err)
	}
	assertImport(t, viaJump, selectedRow)
}

// Partition columns come from the path the user selected when it refers to
// the file being read, as with the DuckDB engine.
func TestImportTabularPreservesSelectedPathPartitions(t *testing.T) {
	for ext, body := range selectedBodies(t) {
		for _, c := range []struct {
			name, target, selected string
			year                   int64
		}{
			{"target-partition", "year=2026/data.", "alias.", 0},
			{"selected-partition", "data.", "year=2025/alias.", 2025},
			{"direct-partition", "year=2027/data.", "year=2027/data.", 2027},
		} {
			t.Run(ext+"/"+c.name, func(t *testing.T) {
				dir := t.TempDir()
				target := filepath.Join(dir, c.target+ext)
				selected := filepath.Join(dir, c.selected+ext)
				writeFile(t, target, body)
				if target != selected {
					symlink(t, target, selected)
				}
				want := Dataset{Columns: []string{"marker", "tag"}, Rows: [][]any{{"selected", "ok"}}}
				if c.year != 0 {
					want.Columns = append(want.Columns, "year")
					want.Rows[0] = append(want.Rows[0], c.year)
				}
				assertImport(t, selected, want)
			})
		}
	}
	t.Run("existing column wins", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "marker=x", "data.csv")
		writeFile(t, p, []byte("marker,tag\nselected,ok\n"))
		assertImport(t, p, selectedRow)
	})
}

func TestImportTabularFailures(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.csv")
	writeFile(t, valid, []byte("marker,tag\nselected,ok\n"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, p := range []string{valid, filepath.Join("testdata", "import", "array.json"), filepath.Join("testdata", "import", "types-zstd.parquet")} {
		if ds, err := ImportTabular(ctx, p); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(ds, Dataset{}) {
			t.Errorf("cancelled import of %s = %#v, %v", p, ds, err)
		}
	}
	for name, body := range map[string][]byte{
		"broken.parquet":  []byte("not parquet"),
		"garbage.parquet": append(append([]byte("PAR1"), bytes.Repeat([]byte{0xff}, 64)...), []byte("\x10\x00\x00\x00PAR1")...),
		"broken.json":     []byte(`[{"a":1},`),
		"trailing.json":   []byte(`[{"a":1},]`),
		"mixed.json":      []byte(`[{"a":1}, 2]`),
		"empty.json":      []byte(`[]`),
		"noquote.csv":     []byte("a,b\n\"x,1\n"),
	} {
		p := filepath.Join(dir, name)
		writeFile(t, p, body)
		if ds, err := ImportTabular(context.Background(), p); err == nil || !reflect.DeepEqual(ds, Dataset{}) {
			t.Errorf("%s = %#v, %v; want an error and no data", name, ds, err)
		}
	}
	for _, p := range []string{filepath.Join(dir, "data.xlsx"), filepath.Join(dir, "missing.csv")} {
		if _, err := ImportTabular(context.Background(), p); err == nil {
			t.Errorf("%s was accepted", p)
		}
	}
	folder := filepath.Join(dir, "folder.csv")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportTabular(context.Background(), folder); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("a directory was accepted: %v", err)
	}
	assertImport(t, valid, selectedRow) // nothing above left state behind
}

func TestImportTabularRefusesFIFOWithoutBlocking(t *testing.T) {
	fifo := makeFIFO(t)
	done := make(chan error, 1)
	go func() {
		_, err := ImportTabular(context.Background(), fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("FIFO import err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ImportTabular blocked on a FIFO")
	}
}

func TestImportTabularCapsOneRecordNotTheFile(t *testing.T) {
	dir := t.TempDir()
	huge := strings.Repeat("x", maxRecordBytes+1)
	for name, body := range map[string]string{
		"line.csv":    "a\n" + huge + "\n",
		"value.json":  `[{"a":"` + huge + `"}]`,
		"line.ndjson": `{"a":"` + huge + `"}` + "\n",
	} {
		p := filepath.Join(dir, name)
		writeFile(t, p, []byte(body))
		if _, err := ImportTabular(context.Background(), p); !errors.Is(err, ErrRecordTooLarge) {
			t.Errorf("%s: err = %v, want ErrRecordTooLarge", name, err)
		}
	}
	// Many ordinary rows adding up to more than the record cap are fine.
	var b strings.Builder
	b.WriteString("a\n")
	row := strings.Repeat("y", 4096) + "\n"
	for b.Len() < 2*maxRecordBytes {
		b.WriteString(row)
	}
	p := filepath.Join(dir, "many.csv")
	writeFile(t, p, []byte(b.String()))
	ds, err := ImportTabular(context.Background(), p)
	if err != nil || len(ds.Rows) == 0 {
		t.Fatalf("a file larger than the record cap: rows=%d err=%v", len(ds.Rows), err)
	}
}

func TestImportTabularTruncationIsExact(t *testing.T) {
	dir := t.TempDir()
	for rows, want := range map[int]bool{MaxImportRows: false, MaxImportRows + 1: true} {
		var csvBody, ndjson strings.Builder
		csvBody.WriteString("n\n")
		for i := 0; i < rows; i++ {
			csvBody.WriteString("1\n")
			ndjson.WriteString(`{"n":1}` + "\n")
		}
		array := "[" + strings.TrimSuffix(strings.ReplaceAll(ndjson.String(), "\n", ","), ",") + "]"
		for name, body := range map[string]string{"t.csv": csvBody.String(), "t.ndjson": ndjson.String(), "t.json": array} {
			p := filepath.Join(dir, name)
			writeFile(t, p, []byte(body))
			ds, err := ImportTabular(context.Background(), p)
			if err != nil || ds.Truncated != want || len(ds.Rows) != MaxImportRows {
				t.Errorf("%s with %d rows: truncated=%t rows=%d err=%v; want truncated=%t", name, rows, ds.Truncated, len(ds.Rows), err, want)
			}
		}
	}
}

// A Parquet file whose pages need more memory than the cap is refused
// rather than allowed to exhaust memory.
func TestImportTabularParquetMemoryCap(t *testing.T) {
	big := strings.Repeat("z", 4<<20)
	body := writeParquet(t, map[string][]string{"blob": {big}}, []string{"blob"}, compress.Codecs.Zstd)
	if len(body) > 1<<20 {
		t.Fatalf("fixture is not compressed: %d bytes", len(body))
	}
	p := filepath.Join(t.TempDir(), "bomb.parquet")
	writeFile(t, p, body)
	old := parquetMemoryLimit
	t.Cleanup(func() { parquetMemoryLimit = old })
	parquetMemoryLimit = 1 << 20
	if _, err := ImportTabular(context.Background(), p); !errors.Is(err, ErrParquetMemory) {
		t.Fatalf("err = %v, want ErrParquetMemory", err)
	}
	parquetMemoryLimit = old
	ds, err := ImportTabular(context.Background(), p)
	if err != nil || len(ds.Rows) != 1 || ds.Rows[0][0] != big {
		t.Fatalf("under the normal cap: rows=%d err=%v", len(ds.Rows), err)
	}
}
