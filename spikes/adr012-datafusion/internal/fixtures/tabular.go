// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Accept says which results are correct for a case.
type Accept string

const (
	// AcceptRows: the file is valid; the expected columns and rows are the truth.
	AcceptRows Accept = "rows"
	// AcceptRowsOrError: the input is degenerate (an empty file, a scalar
	// JSON document). Either the expected rows or a typed error is correct.
	AcceptRowsOrError Accept = "rows_or_error"
	// AcceptError: the file is malformed; only a typed error is correct, and
	// returning rows is a silent-data finding.
	AcceptError Accept = "error"
	// AcceptAmbiguous: the format does not define one answer (duplicate JSON
	// keys). Any result is recorded; none is scored as a failure.
	AcceptAmbiguous Accept = "ambiguous"
)

// TabularCase is one ImportTabular input file and its truth.
type TabularCase struct {
	ID     string
	File   string // file name; its extension selects the reader
	Format string // csv, tsv, txt, json, ndjson, parquet
	Class  Class
	Note   string
	Accept Accept
	// Columns are the expected column names; nil means the file has no
	// header row, so the reader chooses the names.
	Columns []string
	// NamesAdvisory marks header names a reader may legitimately rewrite
	// (duplicates must become distinct); a renamed column is a note.
	NamesAdvisory bool
	// TotalRows is the number of data rows in the file.
	TotalRows int
	// Row returns expected row i (0-based), or nil when Accept is AcceptError.
	Row func(i int) []Cell
	// Large cases are written only on request (performance runs).
	Large bool
	write func(w io.Writer) error
}

// Write creates the file at path.
func (c TabularCase) Write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", c.ID, err)
	}
	// Hide Close: some writers (pqarrow) close an io.Closer sink themselves.
	if err := c.write(struct{ io.Writer }{f}); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", c.ID, err)
	}
	return f.Close()
}

func raw(b []byte) func(io.Writer) error {
	return func(w io.Writer) error { _, err := w.Write(b); return err }
}

// delimited builds a case whose truth is the text of each field.
func delimited(id, file, format string, class Class, note string, content string, header []string, rows [][]string) TabularCase {
	return TabularCase{
		ID: id, File: file, Format: format, Class: class, Note: note, Accept: AcceptRows,
		Columns: header, TotalRows: len(rows),
		Row: func(i int) []Cell {
			out := make([]Cell, len(rows[i]))
			for j, v := range rows[i] {
				out[j] = T(v)
			}
			return out
		},
		write: raw([]byte(content)),
	}
}

func advisory(c TabularCase) TabularCase { c.NamesAdvisory = true; return c }

func csvLines(sep string, eol string, lines ...[]string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(strings.Join(l, sep))
		b.WriteString(eol)
	}
	return b.String()
}

// sizedCSV writes n rows of a five-column table from a seeded generator.
func sizedCSV(n int, large bool) TabularCase {
	gen := func(i int) []string {
		r := rand.New(rand.NewPCG(uint64(i)+1, 0xadc0de))
		cents := r.Int64N(2_000_000_00) - 1_000_000_00
		sign := ""
		if cents < 0 {
			sign, cents = "-", -cents
		}
		return []string{
			strconv.Itoa(i + 1),
			fmt.Sprintf("WBS-%05d", r.IntN(100000)),
			fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100),
			fmt.Sprintf("2026-%02d-%02d", r.IntN(12)+1, r.IntN(28)+1),
			strconv.FormatBool(r.IntN(2) == 0),
		}
	}
	header := []string{"id", "code", "amount", "due", "done"}
	return TabularCase{
		ID: fmt.Sprintf("csv-rows-%d", n), File: fmt.Sprintf("rows-%d.csv", n), Format: "csv",
		Class: map[bool]Class{true: AdversarialStress, false: PlausibleProduction}[n > 10_000], Accept: AcceptRows,
		Note:    "seeded five-column table; truncation boundary when n > 10,000",
		Columns: header, TotalRows: n, Large: large,
		Row: func(i int) []Cell {
			out := []Cell{}
			for _, v := range gen(i) {
				out = append(out, T(v))
			}
			return out
		},
		write: func(w io.Writer) error {
			bw := &bytes.Buffer{}
			bw.WriteString(strings.Join(header, ",") + "\n")
			for i := 0; i < n; i++ {
				bw.WriteString(strings.Join(gen(i), ",") + "\n")
				if bw.Len() > 1<<20 {
					if _, err := w.Write(bw.Bytes()); err != nil {
						return err
					}
					bw.Reset()
				}
			}
			_, err := w.Write(bw.Bytes())
			return err
		},
	}
}

func delimitedCases() []TabularCase {
	wideHeader, wideRow1, wideRow2 := make([]string, 1000), make([]string, 1000), make([]string, 1000)
	for i := range wideHeader {
		wideHeader[i] = "c" + strconv.Itoa(i)
		wideRow1[i] = strconv.Itoa(i)
		wideRow2[i] = "v" + strconv.Itoa(i)
	}
	huge := strings.Repeat("x", 1<<20)
	cases := []TabularCase{
		delimited("csv-basic", "basic.csv", "csv", LegacyParity, "header, integers, a two-decimal amount, booleans",
			csvLines(",", "\n", []string{"id", "name", "amount", "flag"}, []string{"1", "Alpha", "10.50", "true"}, []string{"2", "Beta", "-3", "false"}, []string{"3", "Gamma", "0", "true"}),
			[]string{"id", "name", "amount", "flag"}, [][]string{{"1", "Alpha", "10.50", "true"}, {"2", "Beta", "-3", "false"}, {"3", "Gamma", "0", "true"}}),
		delimited("csv-quoted", "quoted.csv", "csv", LegacyParity, "embedded comma, doubled quote and newline inside quotes",
			"id,text\n1,\"a, b\"\n2,\"say \"\"hi\"\"\"\n3,\"line1\nline2\"\n",
			[]string{"id", "text"}, [][]string{{"1", "a, b"}, {"2", "say \"hi\""}, {"3", "line1\nline2"}}),
		delimited("csv-bom", "bom.csv", "csv", LegacyParity, "UTF-8 byte-order mark before the header",
			"\xef\xbb\xbfid,name\n1,a\n", []string{"id", "name"}, [][]string{{"1", "a"}}),
		delimited("csv-crlf", "crlf.csv", "csv", LegacyParity, "CRLF line endings",
			"id,name\r\n1,a\r\n2,b\r\n", []string{"id", "name"}, [][]string{{"1", "a"}, {"2", "b"}}),
		delimited("csv-unicode", "unicode.csv", "csv", LegacyParity, "non-ASCII, CJK, emoji, right-to-left text",
			"id,name\n1,Café\n2,项目管理\n3,🚀 launch\n4,שלום\n", []string{"id", "name"}, [][]string{{"1", "Café"}, {"2", "项目管理"}, {"3", "🚀 launch"}, {"4", "שלום"}}),
		advisory(delimited("csv-duplicate-names", "dupnames.csv", "csv", AdversarialStress, "repeated header names; a reader may rename them to be distinct",
			"a,a,b\n1,2,3\n", []string{"a", "a", "b"}, [][]string{{"1", "2", "3"}})),
		{ID: "csv-empty", File: "empty.csv", Format: "csv", Class: AdversarialStress, Note: "zero-byte file", Accept: AcceptRowsOrError,
			Columns: []string{}, TotalRows: 0, Row: func(int) []Cell { return nil }, write: raw(nil)},
		delimited("csv-header-only", "headeronly.csv", "csv", LegacyParity, "header and no data rows",
			"id,name\n", []string{"id", "name"}, nil),
		delimited("csv-no-header", "noheader.csv", "csv", LegacyParity, "numeric rows with no header; column names are the reader's choice",
			"1,2,3\n4,5,6\n7,8,9\n", nil, [][]string{{"1", "2", "3"}, {"4", "5", "6"}, {"7", "8", "9"}}),
		delimited("csv-leading-zeros", "zeros.csv", "csv", PlausibleProduction, "codes with leading zeros must keep them",
			"code,qty\n007,1\n0042,2\n", []string{"code", "qty"}, [][]string{{"007", "1"}, {"0042", "2"}}),
		delimited("csv-scientific", "scientific.csv", "csv", LegacyParity, "exponent notation",
			"v\n1e3\n2.5E-2\n-4.0e+1\n", []string{"v"}, [][]string{{"1e3"}, {"2.5E-2"}, {"-4.0e+1"}}),
		delimited("csv-decimal-precision", "decimals.csv", "csv", AdversarialStress, "decimals beyond binary64 precision",
			"v\n12345678901234567890.123456789\n0.1\n0.30000000000000004\n", []string{"v"}, [][]string{{"12345678901234567890.123456789"}, {"0.1"}, {"0.30000000000000004"}}),
		delimited("csv-big-int", "bigint.csv", "csv", AdversarialStress, "integers at and beyond 2^53 and int64",
			"v\n9007199254740993\n9223372036854775807\n9223372036854775808\n-9223372036854775809\n", []string{"v"},
			[][]string{{"9007199254740993"}, {"9223372036854775807"}, {"9223372036854775808"}, {"-9223372036854775809"}}),
		delimited("csv-booleans", "booleans.csv", "csv", LegacyParity, "boolean spellings",
			"v\ntrue\nFALSE\nTrue\n", []string{"v"}, [][]string{{"true"}, {"FALSE"}, {"True"}}),
		delimited("csv-nulls", "nulls.csv", "csv", LegacyParity, "empty fields and the words NULL and null: CSV has no null, so the truth is the text",
			"a,b,c\n1,,NULL\n,x,null\n", []string{"a", "b", "c"}, [][]string{{"1", "", "NULL"}, {"", "x", "null"}}),
		delimited("csv-dates", "dates.csv", "csv", PlausibleProduction, "ISO date, ISO instant and US-style date",
			"d,t,us\n2026-01-02,2026-01-02T03:04:05Z,01/02/2026\n2026-12-31,2026-12-31T23:59:59Z,12/31/2026\n", []string{"d", "t", "us"},
			[][]string{{"2026-01-02", "2026-01-02T03:04:05Z", "01/02/2026"}, {"2026-12-31", "2026-12-31T23:59:59Z", "12/31/2026"}}),
		delimited("csv-wide", "wide.csv", "csv", AdversarialStress, "1,000 columns",
			csvLines(",", "\n", wideHeader, wideRow1, wideRow2), wideHeader, [][]string{wideRow1, wideRow2}),
		delimited("csv-huge-cell", "hugecell.csv", "csv", AdversarialStress, "one 1 MiB field",
			"id,blob\n1,"+huge+"\n", []string{"id", "blob"}, [][]string{{"1", huge}}),
		{ID: "csv-malformed-quote", File: "badquote.csv", Format: "csv", Class: AdversarialStress, Note: "unterminated quote", Accept: AcceptError,
			write: raw([]byte("id,text\n1,\"open\n2,x\n"))},
		{ID: "csv-ragged", File: "ragged.csv", Format: "csv", Class: AdversarialStress, Note: "rows with different field counts", Accept: AcceptError,
			write: raw([]byte("a,b,c\n1,2,3\n4,5\n6,7,8,9\n"))},
		delimited("csv-semicolon", "semicolon.csv", "csv", PlausibleProduction, "semicolon delimiter with decimal commas",
			"id;amount\n1;1,5\n2;-2,25\n", []string{"id", "amount"}, [][]string{{"1", "1,5"}, {"2", "-2,25"}}),
		delimited("tsv-basic", "basic.tsv", "tsv", LegacyParity, "tab-separated, one quoted field containing a tab",
			"id\tname\n1\tAlpha\n2\t\"Be\tta\"\n", []string{"id", "name"}, [][]string{{"1", "Alpha"}, {"2", "Be\tta"}}),
		delimited("txt-pipe", "pipe.txt", "txt", LegacyParity, "pipe-delimited .txt",
			"id|name\n1|a\n2|b\n", []string{"id", "name"}, [][]string{{"1", "a"}, {"2", "b"}}),
		delimited("txt-tab", "tab.txt", "txt", LegacyParity, "tab-delimited .txt",
			"id\tname\n1\ta\n", []string{"id", "name"}, [][]string{{"1", "a"}}),
		delimited("csv-uppercase-ext", "UPPER.CSV", "csv", LegacyParity, "upper-case extension",
			"id\n1\n", []string{"id"}, [][]string{{"1"}}),
		{ID: "csv-latin1", File: "latin1.csv", Format: "csv", Class: AdversarialStress, Note: "ISO-8859-1 bytes, invalid UTF-8", Accept: AcceptRowsOrError,
			Columns: []string{"id", "name"}, TotalRows: 1, Row: func(int) []Cell { return []Cell{T("1"), T("caf\xe9")} },
			write: raw([]byte("id,name\n1,caf\xe9\n"))},
	}
	for _, n := range []int{1, 100, 10_000, 10_001, 50_000, 100_000} {
		cases = append(cases, sizedCSV(n, false))
	}
	cases = append(cases, sizedCSV(1_000_000, true))
	return cases
}

// jsonCase derives the truth by parsing the document with an ordered,
// number-preserving decoder: encoding/json, not an engine under test.
func jsonCase(id, file, format string, class Class, note string, accept Accept, doc string) TabularCase {
	c := TabularCase{ID: id, File: file, Format: format, Class: class, Note: note, Accept: accept, write: raw([]byte(doc))}
	if accept == AcceptError {
		return c
	}
	objs, err := parseRecords(doc)
	if err != nil {
		panic(fmt.Sprintf("fixture %s: %v", id, err))
	}
	var cols []string
	seen := map[string]bool{}
	for _, o := range objs {
		for _, kv := range o {
			if !seen[kv.key] {
				seen[kv.key] = true
				cols = append(cols, kv.key)
			}
		}
	}
	c.Columns, c.TotalRows = cols, len(objs)
	c.Row = func(i int) []Cell {
		out := make([]Cell, len(cols))
		for j, name := range cols {
			out[j] = N()
			for _, kv := range objs[i] {
				if kv.key == name {
					out[j] = kv.val // the last duplicate wins, as encoding/json does
				}
			}
		}
		return out
	}
	return c
}

type kv struct {
	key string
	val Cell
}

// parseRecords reads a JSON array of objects, a single object, or
// newline-delimited objects.
func parseRecords(doc string) ([][]kv, error) {
	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	var out [][]kv
	for {
		var v any
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := v.(type) {
		case []any:
			for _, e := range t {
				o, ok := e.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("array element is not an object")
				}
				out = append(out, ordered(doc, o))
			}
		case map[string]any:
			out = append(out, ordered(doc, t))
		default:
			return nil, fmt.Errorf("top-level %T", v)
		}
	}
	return out, nil
}

// ordered returns an object's fields in document order: keys are sorted by
// their first position in the document, which is exact for the fixtures
// here because no key name repeats across different objects' positions in
// a conflicting order.
func ordered(doc string, o map[string]any) []kv {
	out := make([]kv, 0, len(o))
	for k, v := range o {
		out = append(out, kv{k, jsonCell(v)})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Index(doc, strconv.Quote(out[i].key)) < strings.Index(doc, strconv.Quote(out[j].key))
	})
	return out
}

func jsonCell(v any) Cell {
	switch t := v.(type) {
	case nil:
		return N()
	case bool:
		return B(t)
	case string:
		return S(t)
	case json.Number:
		if strings.ContainsAny(t.String(), ".eE") {
			return D(t.String())
		}
		return I(t.String())
	default:
		b, _ := json.Marshal(t)
		return J(string(b))
	}
}

func jsonCases() []TabularCase {
	big := func(n int, ext string) TabularCase {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "{\"id\":%d,\"name\":\"n%d\",\"amount\":%d.%02d,\"ok\":%t}\n", i+1, i, i%997, i%100, i%3 == 0)
		}
		return jsonCase(fmt.Sprintf("ndjson-rows-%d", n), fmt.Sprintf("rows-%d.%s", n, ext), ext,
			map[bool]Class{true: AdversarialStress, false: PlausibleProduction}[n > 10_000], "seeded records", AcceptRows, b.String())
	}
	deep := strings.Repeat("[", 500) + strings.Repeat("]", 500)
	return []TabularCase{
		jsonCase("ndjson-basic", "basic.ndjson", "ndjson", LegacyParity, "newline-delimited objects", AcceptRows,
			"{\"id\":1,\"name\":\"a\",\"amount\":1.5,\"ok\":true}\n{\"id\":2,\"name\":\"b\",\"amount\":-2,\"ok\":false}\n"),
		jsonCase("json-array", "array.json", "json", LegacyParity, "a JSON array of objects", AcceptRows,
			"[{\"id\":1,\"name\":\"a\"},{\"id\":2,\"name\":\"b\"}]"),
		jsonCase("json-single-object", "single.json", "json", LegacyParity, "one top-level object", AcceptRows,
			"{\"id\":1,\"name\":\"only\"}"),
		jsonCase("json-ndjson-content", "lines.json", "json", LegacyParity, ".json file holding newline-delimited objects", AcceptRows,
			"{\"id\":1}\n{\"id\":2}\n"),
		jsonCase("ndjson-array-content", "array.ndjson", "ndjson", AdversarialStress, ".ndjson file holding one JSON array", AcceptRowsOrError,
			"[{\"id\":1},{\"id\":2}]"),
		jsonCase("ndjson-nested", "nested.ndjson", "ndjson", PlausibleProduction, "arrays and objects inside records", AcceptRows,
			"{\"id\":1,\"tags\":[\"x\",\"y\"],\"meta\":{\"k\":\"v\",\"n\":2}}\n{\"id\":2,\"tags\":[],\"meta\":{\"k\":\"w\",\"n\":3}}\n"),
		jsonCase("json-big-int", "bigint.json", "json", AdversarialStress, "integers at and beyond 2^53 and int64", AcceptRows,
			"[{\"v\":9007199254740993},{\"v\":9223372036854775807},{\"v\":9223372036854775808},{\"v\":-9223372036854775809}]"),
		jsonCase("json-mixed-types", "mixed.json", "json", AdversarialStress, "one field holding int, string, float, null and bool", AcceptRows,
			"[{\"v\":1},{\"v\":\"two\"},{\"v\":3.5},{\"v\":null},{\"v\":true}]"),
		jsonCase("json-duplicate-keys", "dupkeys.json", "json", AdversarialStress, "the same key twice in one object", AcceptAmbiguous,
			"[{\"a\":1,\"a\":2}]"),
		jsonCase("json-escaped-unicode", "escapes.json", "json", LegacyParity, "escaped BMP and surrogate-pair characters", AcceptRows,
			"[{\"s\":\"caf\\u00e9 \\ud83d\\ude80 tab\\tend\"}]"),
		jsonCase("json-invalid", "invalid.json", "json", AdversarialStress, "trailing comma", AcceptError, "[{\"a\":1,}]"),
		jsonCase("json-empty-array", "emptyarray.json", "json", AdversarialStress, "an empty array", AcceptRowsOrError, "[]"),
		jsonCase("ndjson-missing-fields", "sparse.ndjson", "ndjson", PlausibleProduction, "records with different keys", AcceptRows,
			"{\"a\":1}\n{\"b\":2}\n{\"a\":3,\"b\":4}\n"),
		{ID: "json-deep-nesting", File: "deep.json", Format: "json", Class: AdversarialStress, Note: "one record whose field is 500 nested arrays: fail safely or return it intact",
			Accept: AcceptRowsOrError, Columns: []string{"v"}, TotalRows: 1, Row: func(int) []Cell { return []Cell{J(deep)} },
			write: raw([]byte("[{\"v\":" + deep + "}]"))},
		jsonCase("json-decimals", "decimals.json", "json", AdversarialStress, "decimals beyond binary64 precision", AcceptRows,
			"[{\"v\":0.1},{\"v\":123456789.123456789},{\"v\":1e400}]"),
		jsonCase("ndjson-blank-lines", "blank.ndjson", "ndjson", LegacyParity, "blank lines between records", AcceptRows,
			"{\"id\":1}\n\n{\"id\":2}\n\n"),
		{ID: "json-top-level-scalar", File: "scalar.json", Format: "json", Class: AdversarialStress,
			Note: "a bare number: not a record; a typed error, or one row holding 42 under a reader-chosen name", Accept: AcceptRowsOrError,
			Columns: nil, TotalRows: 1, Row: func(int) []Cell { return []Cell{I("42")} }, write: raw([]byte("42"))},
		big(10_001, "ndjson"),
		big(50_000, "ndjson"),
	}
}

// TabularCases returns the delimited, JSON and Parquet corpus.
func TabularCases() []TabularCase {
	out := append(delimitedCases(), jsonCases()...)
	return append(out, parquetCases()...)
}
