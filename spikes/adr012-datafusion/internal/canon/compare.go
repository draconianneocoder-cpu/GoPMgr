// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package canon compares an engine's imported dataset with the generator's
// truth, cell by cell, and classifies every difference. Engines report cells
// in the same Kind/value form as the fixtures (see fixtures.Cell), plus the
// engine's own type name, so DuckDB and DataFusion are judged by one rule.
package canon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

// Class is a per-cell outcome.
type Class string

const (
	Exact            Class = "exact"             // same kind and value
	ExactValue       Class = "exact_value"       // same value, equivalent kind (int vs dec, date vs midnight)
	TypedExact       Class = "typed_exact"       // text typed by the reader; re-rendering gives the original text
	TypedReformatted Class = "typed_reformatted" // text typed by the reader; value kept, original spelling lost ("007" -> 7)
	FloatRounded     Class = "float_rounded"     // a fractional decimal read as its nearest binary64 (an integer whose value changes is Lossy)
	EmptyAsNull      Class = "empty_as_null"     // an empty delimited field read as null
	WordAsNull       Class = "word_as_null"      // the text NULL/null read as null
	TypeChanged      Class = "type_changed"      // value kept but kind changed (number read as string)
	Lossy            Class = "lossy"             // the value changed
	PrecisionLost    Class = "precision_lost"    // a time read at coarser precision
	SignOfZeroLost   Class = "sign_of_zero_lost" // -0 read as 0
)

// Severity groups classes for the parity ledger.
func (c Class) Severity() string {
	switch c {
	case Exact, ExactValue, TypedExact:
		return "ok"
	case TypedReformatted, FloatRounded, EmptyAsNull, WordAsNull, TypeChanged:
		return "note"
	}
	return "loss"
}

// Cell is an engine's value, already converted to a fixtures.Cell, with the
// engine's own type name.
type Cell struct {
	fixtures.Cell
	EngineType string `json:"t,omitempty"`
}

// Result is a case-level comparison.
type Result struct {
	CaseID        string           `json:"case_id"`
	Verdict       string           `json:"verdict"`
	Error         string           `json:"error,omitempty"`
	Columns       []string         `json:"columns,omitempty"`
	ColumnsMatch  *bool            `json:"columns_match,omitempty"`
	ColumnTypes   []string         `json:"column_types,omitempty"`
	Rows          int              `json:"rows"`
	ExpectedRows  int              `json:"expected_rows"`
	TotalRows     int              `json:"total_rows"`
	Truncated     bool             `json:"truncated"`
	Classes       map[Class]int    `json:"classes,omitempty"`
	Examples      map[Class]string `json:"examples,omitempty"`
	JSUnsafeCells int              `json:"js_unsafe_cells,omitempty"`
}

// Verdicts.
const (
	VMatch          = "match"            // every cell ok
	VMatchWithNotes = "match_with_notes" // no loss, some typed/rounded/null notes
	VLoss           = "loss"             // at least one changed value or row/column mismatch
	VRejected       = "rejected"         // typed error on a file that must be rejected: correct
	VRejectedValid  = "rejected_valid"   // typed error on a valid file
	VSilentAccept   = "silent_accept"    // rows returned for a malformed file
	VAmbiguous      = "ambiguous"        // the format defines no single answer; recorded only
	VDegenerateOK   = "degenerate_ok"    // empty/scalar input answered with an error or the expected rows
)

// Compare classifies an engine result for case c. cols and rows are what
// the engine returned; err is its error, if any.
func Compare(c fixtures.TabularCase, cols []string, types []string, rows [][]Cell, err error) Result {
	r := Result{CaseID: c.ID, Columns: cols, ColumnTypes: types, Rows: len(rows), TotalRows: c.TotalRows,
		ExpectedRows: min(c.TotalRows, fixtures.PreviewRows), Truncated: c.TotalRows > fixtures.PreviewRows,
		Classes: map[Class]int{}, Examples: map[Class]string{}}
	if err != nil {
		r.Error = err.Error()
	}
	switch c.Accept {
	case fixtures.AcceptError:
		if err != nil {
			r.Verdict = VRejected
		} else {
			r.Verdict = VSilentAccept
		}
		return r
	case fixtures.AcceptAmbiguous:
		r.Verdict = VAmbiguous
		return r
	case fixtures.AcceptRowsOrError:
		if err != nil {
			r.Verdict = VDegenerateOK
			return r
		}
	case fixtures.AcceptRows:
		if err != nil {
			r.Verdict = VRejectedValid
			return r
		}
	}
	loss := false
	notes := false
	if c.Columns != nil {
		ok := equalStrings(c.Columns, cols)
		r.ColumnsMatch = &ok
		switch {
		case !ok && c.NamesAdvisory && len(cols) == len(c.Columns):
			notes = true
			r.Examples["columns_renamed"] = fmt.Sprintf("got %q, file has %q", cols, c.Columns)
		case !ok:
			loss = true
			r.Examples["columns"] = fmt.Sprintf("got %q, want %q", cols, c.Columns)
		}
	}
	if len(rows) != r.ExpectedRows {
		loss = true
		r.Examples["row_count"] = fmt.Sprintf("got %d rows, want %d", len(rows), r.ExpectedRows)
	}
	for i := 0; i < min(len(rows), r.ExpectedRows) && c.Row != nil; i++ {
		want := c.Row(i)
		if len(want) != len(rows[i]) {
			loss = true
			r.Examples["row_width"] = fmt.Sprintf("row %d: got %d cells, want %d", i, len(rows[i]), len(want))
			continue
		}
		for j := range want {
			cl := Classify(want[j], rows[i][j].Cell)
			r.Classes[cl]++
			if _, seen := r.Examples[cl]; !seen && cl != Exact {
				r.Examples[cl] = fmt.Sprintf("row %d col %d: want %s %q, got %s %q (%s)", i, j, want[j].K, clip(want[j].V), rows[i][j].K, clip(rows[i][j].V), rows[i][j].EngineType)
			}
			if cl.Severity() == "loss" {
				loss = true
			}
		}
	}
	switch {
	case loss:
		r.Verdict = VLoss
	case !notes && (len(r.Classes) == 0 || allOK(r.Classes)):
		r.Verdict = VMatch
	default:
		r.Verdict = VMatchWithNotes
	}
	if c.Accept == fixtures.AcceptRowsOrError && r.Verdict != VLoss {
		r.Verdict = VDegenerateOK
	}
	return r
}

func allOK(m map[Class]int) bool {
	for c := range m {
		if c.Severity() != "ok" {
			return false
		}
	}
	return true
}

func clip(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Classify compares one expected cell with one engine cell.
func Classify(want, got fixtures.Cell) Class {
	if want == got {
		return Exact
	}
	switch want.K {
	case fixtures.KText:
		return classifyText(want.V, got)
	case fixtures.KNull:
		return Lossy
	case fixtures.KInt, fixtures.KDec:
		return classifyNumber(want.V, got)
	case fixtures.KFloat:
		return classifyFloat(want.V, got)
	case fixtures.KStr:
		if got.K == fixtures.KStr || got.K == fixtures.KText {
			if got.V == want.V {
				return Exact
			}
		}
		return Lossy
	case fixtures.KBool:
		if got.K == fixtures.KBool && got.V == want.V {
			return Exact
		}
		return Lossy
	case fixtures.KTime:
		return classifyTime(want.V, got)
	case fixtures.KDate:
		if got.K == fixtures.KDate && got.V == want.V {
			return Exact
		}
		if got.K == fixtures.KTime && (got.V == want.V+"T00:00:00Z" || got.V == want.V+"T00:00:00") {
			return ExactValue
		}
		return Lossy
	case fixtures.KBytes:
		if got.K == fixtures.KBytes && got.V == want.V {
			return Exact
		}
		if got.K == fixtures.KStr && fmt.Sprintf("%x", got.V) == want.V {
			return TypeChanged
		}
		return Lossy
	case fixtures.KJSON:
		if got.K == fixtures.KJSON && CanonicalJSON(got.V) == CanonicalJSON(want.V) {
			return Exact
		}
		return Lossy
	}
	return Lossy
}

func rat(s string) (*big.Rat, bool) {
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

func classifyText(text string, got fixtures.Cell) Class {
	switch got.K {
	case fixtures.KStr, fixtures.KText:
		if got.V == text {
			return Exact
		}
		return Lossy
	case fixtures.KNull:
		switch text {
		case "":
			return EmptyAsNull
		case "NULL", "null":
			return WordAsNull
		}
		return Lossy
	case fixtures.KInt, fixtures.KDec:
		w, ok1 := rat(text)
		g, ok2 := rat(got.V)
		if !ok1 || !ok2 || w.Cmp(g) != 0 {
			return Lossy
		}
		if got.V == text {
			return TypedExact
		}
		return TypedReformatted
	case fixtures.KFloat:
		g, err := strconv.ParseFloat(got.V, 64)
		if err != nil {
			return Lossy
		}
		w, err := strconv.ParseFloat(text, 64)
		if err != nil || w != g {
			return Lossy
		}
		if wr, ok := rat(text); ok && wr.IsInt() && new(big.Rat).SetFloat64(g).Cmp(wr) != 0 {
			return Lossy // an integer whose value the float changed
		}
		if wr, ok := rat(text); ok {
			if gr := new(big.Rat); !math.IsInf(g, 0) && gr.SetFloat64(g) != nil && gr.Cmp(wr) == 0 {
				if strconv.FormatFloat(g, 'g', -1, 64) == text {
					return TypedExact
				}
				return TypedReformatted
			}
		}
		return FloatRounded
	case fixtures.KBool:
		if strings.EqualFold(text, got.V) {
			if text == got.V {
				return TypedExact
			}
			return TypedReformatted
		}
		return Lossy
	case fixtures.KDate:
		if text == got.V {
			return TypedExact
		}
		return Lossy
	case fixtures.KTime:
		t, err := time.Parse(time.RFC3339Nano, text)
		if err == nil && sameInstant(t, got.V) {
			return TypedReformatted
		}
		if d, err := time.Parse("2006-01-02", text); err == nil && sameInstant(d, got.V) {
			return TypedReformatted
		}
		return Lossy
	}
	return Lossy
}

func sameInstant(t time.Time, v string) bool {
	g, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		g, err = time.Parse("2006-01-02T15:04:05.999999999", v)
	}
	return err == nil && g.Equal(t)
}

func classifyNumber(text string, got fixtures.Cell) Class {
	w, _ := rat(text)
	switch got.K {
	case fixtures.KInt, fixtures.KDec:
		g, ok := rat(got.V)
		if ok && w != nil && g.Cmp(w) == 0 {
			if got.V == text {
				return Exact
			}
			return ExactValue
		}
		return Lossy
	case fixtures.KFloat:
		g, err := strconv.ParseFloat(got.V, 64)
		if err != nil || math.IsInf(g, 0) || math.IsNaN(g) || w == nil {
			return Lossy
		}
		nearest, _ := w.Float64()
		if g != nearest {
			return Lossy
		}
		if new(big.Rat).SetFloat64(g).Cmp(w) == 0 {
			return ExactValue
		}
		if w.IsInt() {
			return Lossy // an integer whose value the float changed
		}
		return FloatRounded
	case fixtures.KStr, fixtures.KText:
		if got.V == text {
			return TypeChanged
		}
	}
	return Lossy
}

func classifyFloat(text string, got fixtures.Cell) Class {
	w, err := strconv.ParseFloat(text, 64)
	if text == "-0" {
		w = math.Copysign(0, -1)
		err = nil
	}
	if err != nil {
		return Lossy
	}
	switch got.K {
	case fixtures.KFloat:
		g, err := strconv.ParseFloat(got.V, 64)
		if got.V == "-0" {
			g = math.Copysign(0, -1)
			err = nil
		}
		if err != nil {
			return Lossy
		}
		if math.IsNaN(w) && math.IsNaN(g) {
			return Exact
		}
		if w == g {
			if math.Signbit(w) != math.Signbit(g) {
				return SignOfZeroLost
			}
			return Exact
		}
	case fixtures.KNull:
		return Lossy
	}
	return Lossy
}

func classifyTime(text string, got fixtures.Cell) Class {
	if got.K != fixtures.KTime {
		return Lossy
	}
	naive := !strings.HasSuffix(text, "Z")
	layout := time.RFC3339Nano
	if naive {
		layout = "2006-01-02T15:04:05.999999999"
	}
	w, err := time.Parse(layout, text)
	if err != nil {
		return Lossy
	}
	g, err := time.Parse(time.RFC3339Nano, got.V)
	if err != nil {
		g, err = time.Parse("2006-01-02T15:04:05.999999999", got.V)
	}
	if err != nil {
		return Lossy
	}
	if g.Equal(w) {
		if got.V == text {
			return Exact
		}
		return ExactValue
	}
	for _, unit := range []time.Duration{time.Microsecond, time.Millisecond, time.Second} {
		if w.Truncate(unit).Equal(g) {
			return PrecisionLost
		}
	}
	return Lossy
}

// CanonicalJSON parses s with numbers kept as written and re-encodes it with
// sorted object keys and no spaces. Invalid JSON is returned unchanged.
func CanonicalJSON(s string) string {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return s
	}
	var buf bytes.Buffer
	writeCanonical(&buf, v)
	return buf.String()
}

func writeCanonical(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeCanonical(buf, t[k])
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonical(buf, e)
		}
		buf.WriteByte(']')
	case json.Number:
		// Normalize the spelling of numerically equal values (2 vs 2.0).
		if r, ok := new(big.Rat).SetString(t.String()); ok {
			buf.WriteString(r.RatString())
			return
		}
		buf.WriteString(t.String())
	default:
		b, _ := json.Marshal(t)
		buf.Write(b)
	}
}
