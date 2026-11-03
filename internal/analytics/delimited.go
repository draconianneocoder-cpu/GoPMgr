// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

const sniffBytes = 64 << 10

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// readDelimited reads CSV, TSV or delimited TXT. The delimiter is sniffed
// from the start of the file among comma, tab, semicolon and pipe (tab is
// preferred for .tsv). A first row that does not fit the types of the rows
// below it is the header; an all-text file is taken to have a header.
func readDelimited(ctx context.Context, r io.Reader, tsv bool) (Dataset, error) {
	br := bufio.NewReaderSize(r, sniffBytes)
	if b, _ := br.Peek(len(utf8BOM)); bytes.Equal(b, utf8BOM) {
		_, _ = br.Discard(len(utf8BOM))
	}
	sample, err := br.Peek(sniffBytes)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return Dataset{}, fmt.Errorf("analytics: read file: %w", err)
	}
	if len(bytes.TrimSpace(sample)) == 0 {
		return Dataset{}, errors.New("analytics: the file is empty")
	}
	delim := sniffDelimiter(sample, tsv, len(sample) < sniffBytes)

	// The record cap counts from where the current logical record began.
	// cr.InputOffset advances with every physical line, even inside a quoted
	// field that spans many lines, so it cannot be the limiter's start.
	var recStart int64
	limiter := &recordLimiter{r: br, rowStart: func() int64 { return recStart }}
	cr := csv.NewReader(limiter)
	cr.Comma = delim
	var rows [][]string
	for len(rows) < MaxImportRows+2 { // a header, the cap, and one more to detect truncation
		if len(rows)%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return Dataset{}, err
			}
		}
		recStart = cr.InputOffset()
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if cr.InputOffset()-recStart > maxRecordBytes {
			return Dataset{}, ErrRecordTooLarge
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) && errors.Is(pe.Err, csv.ErrFieldCount) {
				return Dataset{}, fmt.Errorf("analytics: line %d has %d fields, but the first line has %d", pe.StartLine, len(rec), len(rows[0]))
			}
			if errors.Is(err, ErrRecordTooLarge) {
				return Dataset{}, err
			}
			return Dataset{}, fmt.Errorf("analytics: read %s: %w", delimiterName(delim), err)
		}
		rows = append(rows, rec)
	}
	if len(rows) == 0 {
		return Dataset{}, errors.New("analytics: the file is empty")
	}

	header := hasHeader(rows)
	var names []string
	data := rows
	if header {
		names, data = rows[0], rows[1:]
	} else {
		names = make([]string, len(rows[0]))
	}
	ds := Dataset{Columns: uniqueNames(names), Rows: [][]any{}}
	if len(data) > MaxImportRows {
		ds.Truncated = true
		data = data[:MaxImportRows]
	}
	types := columnTypes(data, len(names))
	for _, rec := range data {
		row := make([]any, len(rec))
		for i, v := range rec {
			row[i] = textCell(v, types[i])
		}
		ds.Rows = append(ds.Rows, row)
	}
	return ds, nil
}

func delimiterName(d rune) string {
	switch d {
	case '\t':
		return "tab-separated text"
	case ';':
		return "semicolon-separated text"
	case '|':
		return "pipe-separated text"
	}
	return "CSV"
}

// sniffDelimiter picks the candidate that parses the sample without error and
// splits every sampled record into the same number of fields, preferring more
// fields. A parse error that runs to the end of a truncated sample is the
// sample's cut, not the file's fault. With no consistent multi-field candidate
// it returns a comma, so the reader reports a ragged file or reads a single
// column.
func sniffDelimiter(sample []byte, tsv, complete bool) rune {
	if !complete {
		if i := bytes.LastIndexByte(sample, '\n'); i > 0 {
			sample = sample[:i+1]
		}
	}
	candidates := []rune{',', '\t', ';', '|'}
	if tsv {
		candidates = []rune{'\t', ',', ';', '|'}
	}
	best, bestFields := ',', 1
	for _, c := range candidates {
		cr := csv.NewReader(bytes.NewReader(sample))
		cr.Comma = c
		cr.FieldsPerRecord = -1
		fields, consistent, n := 0, true, 0
		for n < 100 {
			rec, err := cr.Read()
			if err != nil {
				if !errors.Is(err, io.EOF) && (complete || cr.InputOffset() < int64(len(sample))) {
					consistent = false
				}
				break
			}
			n++
			if fields == 0 {
				fields = len(rec)
			} else if len(rec) != fields {
				consistent = false
				break
			}
		}
		if n > 0 && consistent && fields > bestFields {
			best, bestFields = c, fields
		}
	}
	return best
}

type colType int

const (
	typeNull colType = iota // no non-empty value yet
	typeBool
	typeInt
	typeFloat
	typeText
)

func classify(v string) colType {
	switch {
	case v == "":
		return typeNull
	case strings.EqualFold(v, "true") || strings.EqualFold(v, "false"):
		return typeBool
	}
	if _, ok := parseInt(v); ok {
		return typeInt
	}
	if isIntegerText(v) {
		// A leading zero or an integer beyond int64: keep the text.
		return typeText
	}
	if _, ok := parseFloat(v); ok {
		return typeFloat
	}
	return typeText
}

func merge(a, b colType) colType {
	switch {
	case a == b || b == typeNull:
		return a
	case a == typeNull:
		return b
	case a == typeInt && b == typeFloat, a == typeFloat && b == typeInt:
		return typeFloat
	}
	return typeText
}

func columnTypes(rows [][]string, n int) []colType {
	types := make([]colType, n)
	for _, rec := range rows {
		for i, v := range rec {
			types[i] = merge(types[i], classify(v))
		}
	}
	return types
}

// hasHeader reports whether the first row names the columns: true when the
// rows below are all text or empty, or when a first-row value does not fit
// its column's type. A single row is a header.
func hasHeader(rows [][]string) bool {
	if len(rows) < 2 {
		return true
	}
	types := columnTypes(rows[1:], len(rows[0]))
	allText := true
	for _, t := range types {
		if t != typeText && t != typeNull {
			allText = false
		}
	}
	if allText {
		return true
	}
	for i, v := range rows[0] {
		if v != "" && merge(types[i], classify(v)) != types[i] {
			return true
		}
	}
	return false
}

func textCell(v string, t colType) any {
	if v == "" {
		return nil
	}
	switch t {
	case typeBool:
		return strings.EqualFold(v, "true")
	case typeInt:
		n, _ := parseInt(v)
		return intCell(n)
	case typeFloat:
		// A fractional value elsewhere in this column must not round an
		// integer that JavaScript cannot carry safely as a number.
		if n, ok := parseInt(v); ok && (n > maxExactJSInteger || n < -maxExactJSInteger) {
			return intCell(n)
		}
		f, _ := parseFloat(v)
		return floatCell(f)
	}
	return v
}
