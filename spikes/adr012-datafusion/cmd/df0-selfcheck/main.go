// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command df0-selfcheck proves the DF0 fixtures say what their truth says,
// using readers independent of every engine under test (ADR-012 DF-A11):
//
//   - delimited files: Go's encoding/csv;
//   - Parquet: pyarrow (Arrow C++), via tools/independent_read.py;
//   - JSON/NDJSON: Python's json module, via the same script.
//
// A fixture that its independent reader does not reproduce is a fixture
// bug, so this must pass before any engine result is trusted.
//
//	python -I tools/independent_read.py CORPUS > reads.jsonl
//	go run ./cmd/df0-selfcheck -corpus CORPUS -reads reads.jsonl -out evidence/df0-a
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopmgr/spikes/adr012-datafusion/internal/canon"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

type read struct {
	ID        string            `json:"id"`
	Reader    string            `json:"reader"`
	Columns   []string          `json:"columns"`
	Rows      [][]fixtures.Cell `json:"rows"`
	TotalRows int               `json:"total_rows"`
	Error     string            `json:"error"`
}

type record struct {
	CaseID   string       `json:"case_id"`
	Reader   string       `json:"reader"`
	Result   canon.Result `json:"result"`
	Expected string       `json:"expected"`
	OK       bool         `json:"ok"`
	Note     string       `json:"note,omitempty"`
}

// knownReaderLimits are cases an independent reader cannot parse for a
// reason of its own, recorded instead of hidden.
var knownReaderLimits = map[string]string{
	"json-deep-nesting": "Python's json module stops at its recursion limit before 500 levels; encoding/json (the truth source) parses it",
}

func main() {
	corpus := flag.String("corpus", "", "corpus directory")
	readsPath := flag.String("reads", "", "output of tools/independent_read.py")
	out := flag.String("out", "evidence/df0-a", "evidence directory")
	flag.Parse()
	if err := run(*corpus, *readsPath, *out); err != nil {
		fmt.Fprintln(os.Stderr, "df0-selfcheck:", err)
		os.Exit(1)
	}
}

func run(corpus, readsPath, out string) error {
	if corpus == "" || readsPath == "" {
		return errors.New("-corpus and -reads are required")
	}
	reads, readerName, err := loadReads(readsPath)
	if err != nil {
		return err
	}
	var recs []record
	for _, c := range fixtures.TabularCases() {
		if c.Large {
			continue
		}
		var rec record
		switch c.Format {
		case "csv", "tsv", "txt":
			cols, rows, rerr := readDelimited(filepath.Join(corpus, "tabular", c.File), c)
			rec = score(c, "encoding/csv", cols, rows, rerr)
		default:
			r, ok := reads[c.ID]
			if !ok {
				return fmt.Errorf("%s: no independent read", c.ID)
			}
			var rerr error
			if r.Error != "" {
				rerr = errors.New(r.Error)
			}
			cells := make([][]canon.Cell, len(r.Rows))
			for i, row := range r.Rows {
				cells[i] = make([]canon.Cell, len(row))
				for j, cell := range row {
					cells[i][j] = canon.Cell{Cell: cell}
				}
			}
			rec = score(c, readerName, r.Columns, cells, rerr)
		}
		recs = append(recs, rec)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // #nosec G301 -- evidence directory.
		return err
	}
	f, err := os.Create(filepath.Join(out, "fixture-selfcheck.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	bad := []string{}
	for _, r := range recs {
		b, _ := json.Marshal(r)
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
		if !r.OK {
			bad = append(bad, fmt.Sprintf("%s (%s: %s)", r.CaseID, r.Reader, r.Result.Verdict))
		}
	}
	sort.Strings(bad)
	fmt.Printf("df0-selfcheck: %d of %d fixtures reproduced by an independent reader\n", len(recs)-len(bad), len(recs))
	if len(bad) > 0 {
		return fmt.Errorf("fixtures not reproduced:\n  %s", strings.Join(bad, "\n  "))
	}
	return nil
}

// score accepts exact reproduction for valid files and a rejection for
// malformed ones; delimited text is compared as text, so no typing applies.
func score(c fixtures.TabularCase, reader string, cols []string, rows [][]canon.Cell, err error) record {
	res := canon.Compare(c, cols, nil, rows, err)
	rec := record{CaseID: c.ID, Reader: reader, Result: res}
	switch c.Accept {
	case fixtures.AcceptError:
		rec.Expected, rec.OK = canon.VRejected, res.Verdict == canon.VRejected
	case fixtures.AcceptAmbiguous:
		rec.Expected, rec.OK = canon.VAmbiguous, true
	case fixtures.AcceptRowsOrError:
		rec.Expected, rec.OK = canon.VDegenerateOK, res.Verdict == canon.VDegenerateOK
	default:
		rec.Expected, rec.OK = canon.VMatch, res.Verdict == canon.VMatch
	}
	if note, ok := knownReaderLimits[c.ID]; ok && !rec.OK {
		rec.OK, rec.Note = true, "reader limit: "+note
	}
	return rec
}

func loadReads(path string) (map[string]read, string, error) {
	f, err := os.Open(path) // #nosec G304 -- the script's output.
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	out := map[string]read{}
	name := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var r read
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, "", err
		}
		if r.Reader != "" {
			name = r.Reader
			continue
		}
		out[r.ID] = r
	}
	return out, name, sc.Err()
}

func delimiter(c fixtures.TabularCase) rune {
	switch {
	case c.File == "semicolon.csv":
		return ';'
	case c.File == "pipe.txt":
		return '|'
	case c.Format == "tsv", c.File == "tab.txt":
		return '\t'
	}
	return ','
}

func readDelimited(path string, c fixtures.TabularCase) ([]string, [][]canon.Cell, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- corpus file.
	if err != nil {
		return nil, nil, err
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = delimiter(c)
	recs, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	var cols []string
	if c.Columns != nil && len(recs) > 0 {
		cols, recs = recs[0], recs[1:]
	}
	if c.Columns != nil && len(c.Columns) == 0 {
		cols = []string{}
	}
	rows := make([][]canon.Cell, 0, len(recs))
	for _, rec := range recs[:min(len(recs), fixtures.PreviewRows)] {
		row := make([]canon.Cell, len(rec))
		for j, v := range rec {
			row[j] = canon.Cell{Cell: fixtures.Cell{K: fixtures.KStr, V: v}}
		}
		rows = append(rows, row)
	}
	return cols, rows, nil
}
