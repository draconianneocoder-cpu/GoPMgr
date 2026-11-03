// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// readJSON reads a JSON array of rows, or a sequence of JSON values such as
// newline-delimited JSON (NDJSON). Each object is one row; its keys are
// columns, in the order they first appear. A file of non-object values has
// one column named "json", as the DuckDB engine had. Rows are decoded one at
// a time, so a large file is read only as far as the row cap.
func readJSON(ctx context.Context, r io.Reader, ndjson bool) (Dataset, error) {
	br := bufio.NewReader(r)
	if b, _ := br.Peek(len(utf8BOM)); bytes.Equal(b, utf8BOM) {
		_, _ = br.Discard(len(utf8BOM))
	}
	first, err := firstNonSpace(br)
	if errors.Is(err, io.EOF) {
		return Dataset{}, errors.New("analytics: the file is empty")
	}
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: read file: %w", err)
	}
	limiter := &recordLimiter{r: br}
	dec := json.NewDecoder(limiter)
	dec.UseNumber()
	limiter.rowStart = dec.InputOffset
	array := first == '[' && !ndjson
	if array {
		if _, err := dec.Token(); err != nil {
			return Dataset{}, jsonError(err)
		}
	}

	var raws []json.RawMessage
	truncated := false
	for {
		if len(raws)%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return Dataset{}, err
			}
		}
		if array && !dec.More() {
			break
		}
		var raw json.RawMessage
		start := dec.InputOffset()
		if err := dec.Decode(&raw); errors.Is(err, io.EOF) && !array {
			break
		} else if err != nil {
			return Dataset{}, jsonError(err)
		}
		// The decoder drops a read error when the value it is reading ends in
		// the same read, so the limiter alone can miss a row that crosses the
		// cap; measure the row as well.
		if dec.InputOffset()-start > maxRecordBytes {
			return Dataset{}, ErrRecordTooLarge
		}
		if len(raws) == MaxImportRows {
			truncated = true
			break
		}
		raws = append(raws, raw)
	}
	// dec.More reports false on a read error as well as at the end, so an
	// array must be seen to close.
	if array && !truncated {
		if _, err := dec.Token(); err != nil {
			return Dataset{}, jsonError(err)
		}
	}
	if len(raws) == 0 {
		return Dataset{}, errors.New("analytics: the file contains no rows")
	}

	objects := raws[0][0] == '{'
	var columns []string
	index := map[string]int{}
	var rows [][]keyValue
	for i, raw := range raws {
		if (raw[0] == '{') != objects {
			return Dataset{}, fmt.Errorf("analytics: row %d mixes objects with other JSON values", i+1)
		}
		if !objects {
			rows = append(rows, []keyValue{{"json", raw}})
			continue
		}
		kvs, err := orderedObject(raw)
		if err != nil {
			return Dataset{}, jsonError(err)
		}
		rows = append(rows, kvs)
	}
	for _, kvs := range rows {
		for _, kv := range kvs {
			if _, ok := index[kv.key]; !ok {
				index[kv.key] = len(columns)
				columns = append(columns, kv.key)
			}
		}
	}
	if len(columns) == 0 {
		return Dataset{}, errors.New("analytics: the file's objects have no fields")
	}
	ds := Dataset{Columns: uniqueNames(columns), Rows: make([][]any, 0, len(rows)), Truncated: truncated}
	for _, kvs := range rows {
		row := make([]any, len(columns))
		for _, kv := range kvs {
			cell, err := jsonCell(kv.value)
			if err != nil {
				return Dataset{}, jsonError(err)
			}
			row[index[kv.key]] = cell
		}
		ds.Rows = append(ds.Rows, row)
	}
	return ds, nil
}

func jsonError(err error) error {
	if errors.Is(err, ErrRecordTooLarge) {
		return err
	}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return fmt.Errorf("analytics: invalid JSON at byte %d: %w", se.Offset, err)
	}
	return fmt.Errorf("analytics: read JSON: %w", err)
}

func firstNonSpace(br *bufio.Reader) (byte, error) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		}
		return b, br.UnreadByte()
	}
}

type keyValue struct {
	key   string
	value json.RawMessage
}

// orderedObject returns an object's fields in source order. A repeated key
// keeps its first position and its last value.
func orderedObject(raw json.RawMessage) ([]keyValue, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil { // '{'
		return nil, err
	}
	var out []keyValue
	pos := map[string]int{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		if i, ok := pos[key]; ok {
			out[i].value = v
			continue
		}
		pos[key] = len(out)
		out = append(out, keyValue{key, v})
	}
	return out, nil
}

// jsonCell converts one JSON value. Integers stay exact (as text beyond
// what JavaScript holds exactly); objects and arrays become compact JSON.
func jsonCell(raw json.RawMessage) (any, error) {
	switch raw[0] {
	case 'n':
		return nil, nil
	case 't':
		return true, nil
	case 'f':
		return false, nil
	case '"':
		var s string
		err := json.Unmarshal(raw, &s)
		return s, err
	case '{', '[':
		var b bytes.Buffer
		if err := json.Compact(&b, raw); err != nil {
			return nil, err
		}
		return b.String(), nil
	}
	text := string(raw)
	if isIntegerText(text) {
		if v, err := strconv.ParseInt(text, 10, 64); err == nil {
			return intCell(v), nil
		}
		return text, nil // beyond int64: keep every digit
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text, nil
	}
	return floatCell(v), nil
}
