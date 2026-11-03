// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MaxImportRows caps the rows ImportTabular returns, so a large file cannot
// exhaust memory or the Wails bridge. Dataset.Truncated reports a larger file.
const MaxImportRows = 10000

// maxRecordBytes caps one CSV record or one JSON row, so a single huge line
// cannot exhaust memory. It does not cap the file.
const maxRecordBytes = 16 << 20

// ErrRecordTooLarge reports a single row larger than maxRecordBytes.
var ErrRecordTooLarge = errors.New("analytics: a single row is larger than 16 MiB")

// maxExactJSInteger is the largest integer a JavaScript number holds exactly.
const maxExactJSInteger = 1<<53 - 1

type format int

const (
	formatDelimited format = iota
	formatJSON
	formatParquet
)

func formatFor(ext string) (format, bool) {
	switch strings.ToLower(ext) {
	case ".csv", ".tsv", ".txt":
		return formatDelimited, true
	case ".json", ".ndjson":
		return formatJSON, true
	case ".parquet":
		return formatParquet, true
	}
	return 0, false
}

// ImportTabular reads one user-selected local file into a Dataset of at most
// MaxImportRows rows. The format follows the selected file's extension. A
// symlink is read through to its target. Directories named key=value on the
// selected path add a column per key (Hive-style partitions), as the DuckDB
// engine did. Only regular files are read; a FIFO or device is refused
// without blocking.
func ImportTabular(ctx context.Context, path string) (Dataset, error) {
	f, ok := formatFor(filepath.Ext(path))
	if !ok {
		return Dataset{}, fmt.Errorf("analytics: unsupported file type %q (want .csv, .tsv, .txt, .json, .ndjson or .parquet)", filepath.Ext(path))
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: resolve path: %w", err)
	}
	file, info, err := openRegular(abs)
	if err != nil {
		return Dataset{}, err
	}
	defer func() { _ = file.Close() }() // read-only: a close error loses nothing

	var ds Dataset
	switch f {
	case formatDelimited:
		ds, err = readDelimited(ctx, file, strings.EqualFold(filepath.Ext(path), ".tsv"))
	case formatJSON:
		ds, err = readJSON(ctx, file, strings.EqualFold(filepath.Ext(path), ".ndjson"))
	case formatParquet:
		ds, err = readParquet(ctx, file, info.Size())
	}
	if err != nil {
		return Dataset{}, err
	}
	addPartitions(&ds, partitionPath(path, info))
	return ds, nil
}

// partitionPath is the path whose directories name partitions: the path the
// user selected when it still refers to the file being read, otherwise the
// resolved file. Cleaning ".." or a symlink can select another file.
func partitionPath(selected string, info os.FileInfo) string {
	abs, err := filepath.Abs(selected)
	if err == nil {
		if st, err := os.Stat(abs); err == nil && os.SameFile(st, info) {
			return abs
		}
	}
	if resolved, err := filepath.EvalSymlinks(selected); err == nil {
		if abs, err := filepath.Abs(resolved); err == nil {
			return abs
		}
	}
	return ""
}

// addPartitions appends a column for each key=value directory on path, in
// path order, unless the file already has a column of that name.
func addPartitions(ds *Dataset, path string) {
	if path == "" {
		return
	}
	have := map[string]bool{}
	for _, c := range ds.Columns {
		have[c] = true
	}
	for _, dir := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		key, value, ok := strings.Cut(dir, "=")
		if !ok || key == "" || strings.Contains(value, "=") || have[key] {
			continue
		}
		have[key] = true
		ds.Columns = append(ds.Columns, key)
		cell := partitionValue(value)
		for i := range ds.Rows {
			ds.Rows[i] = append(ds.Rows[i], cell)
		}
	}
}

func partitionValue(s string) any {
	if s == "" {
		return nil
	}
	if v, ok := parseInt(s); ok {
		return intCell(v)
	}
	if v, ok := parseFloat(s); ok {
		return v
	}
	return s
}

// uniqueNames gives empty names a positional name (column0, column1, ...)
// and repeats a suffix (a, a_1, a_2), as the DuckDB engine did, so every
// column is unique: the UI keys columns by name.
func uniqueNames(names []string) []string {
	out := make([]string, len(names))
	seen := map[string]bool{}
	for i, n := range names {
		if n == "" {
			n = "column" + strconv.Itoa(i)
		}
		name := n
		for k := 1; seen[name]; k++ {
			name = n + "_" + strconv.Itoa(k)
		}
		seen[name] = true
		out[i] = name
	}
	return out
}

// parseInt accepts optional "-" and decimal digits without a leading zero.
// A leading zero (as in "007" or a ZIP code) keeps the text, as DuckDB did.
func parseInt(s string) (int64, bool) {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" || len(digits) > 1 && digits[0] == '0' {
		return 0, false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

// isIntegerText reports text that is an integer, whatever its size.
func isIntegerText(s string) bool {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseFloat accepts plain decimal and exponent notation only: no hex,
// underscores, "Inf" or "NaN".
func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	seenDigit, seenDot, seenExp := false, false, false
	for i, c := range s {
		switch {
		case c >= '0' && c <= '9':
			seenDigit = true
		case c == '-' || c == '+':
			if i != 0 && s[i-1] != 'e' && s[i-1] != 'E' {
				return 0, false
			}
		case c == '.':
			if seenDot || seenExp {
				return 0, false
			}
			seenDot = true
		case c == 'e' || c == 'E':
			if seenExp || !seenDigit {
				return 0, false
			}
			seenExp = true
		default:
			return 0, false
		}
	}
	if !seenDigit || s[0] == '+' {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// intCell keeps an integer as a number when JavaScript can hold it exactly,
// and as text otherwise.
func intCell(v int64) any {
	if v > maxExactJSInteger || v < -maxExactJSInteger {
		return strconv.FormatInt(v, 10)
	}
	return v
}

// floatCell keeps a finite float as a number and anything else as text,
// since JSON cannot carry NaN or infinities.
func floatCell(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return v
}

// recordLimiter fails once more than maxRecordBytes have been read past the
// end of the last complete row the caller reports. The check runs after
// each read too: a row can end inside the read that crosses the limit, and
// the error must still surface (readers keep it for their next call).
type recordLimiter struct {
	r        io.Reader
	read     int64
	rowStart func() int64
}

func (l *recordLimiter) Read(p []byte) (int, error) {
	if l.read-l.rowStart() > maxRecordBytes {
		return 0, ErrRecordTooLarge
	}
	n, err := l.r.Read(p)
	l.read += int64(n)
	if l.read-l.rowStart() > maxRecordBytes {
		return n, ErrRecordTooLarge
	}
	return n, err
}
