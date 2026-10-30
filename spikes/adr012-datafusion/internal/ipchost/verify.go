// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"bytes"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// GenSchema is the schema of the generate and duplex operations.
var GenSchema = arrow.NewSchema([]arrow.Field{
	{Name: "i", Type: arrow.PrimitiveTypes.Int64},
	{Name: "v", Type: arrow.PrimitiveTypes.Int64},
	{Name: "s", Type: arrow.BinaryTypes.String},
}, nil)

// GenVerifier checks generated batches against the generation rule,
// independently of the worker: row i has v = i*7919 mod 1000003 and a
// payload whose byte k is 'a' + (i+k) mod 26. Rows must arrive in order.
type GenVerifier struct {
	Width int
	Next  int64
}

// Check decodes one Arrow IPC stream and verifies every row. It returns
// the row count. A panic in the decoder is returned as an error: the bytes
// came from another process.
func (g *GenVerifier) Check(b []byte) (rows int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("arrow decode panicked: %v", r)
		}
	}()
	rd, err := ipc.NewReader(bytes.NewReader(b), ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return 0, fmt.Errorf("arrow stream: %w", err)
	}
	defer rd.Release()
	if !sameFields(rd.Schema(), GenSchema) {
		return 0, fmt.Errorf("schema %v is not the generate schema", rd.Schema())
	}
	for rd.Next() {
		n, err := g.checkBatch(rd.RecordBatch())
		rows += n
		if err != nil {
			return rows, err
		}
	}
	if err := rd.Err(); err != nil {
		return rows, fmt.Errorf("arrow stream: %w", err)
	}
	return rows, nil
}

// sameFields compares names and types; nullability differs between
// writers and carries no meaning here.
func sameFields(have, want *arrow.Schema) bool {
	if have.NumFields() != want.NumFields() {
		return false
	}
	for i := range want.NumFields() {
		h, w := have.Field(i), want.Field(i)
		if h.Name != w.Name || !arrow.TypeEqual(h.Type, w.Type) {
			return false
		}
	}
	return true
}

func (g *GenVerifier) checkBatch(rec arrow.RecordBatch) (int, error) {
	n := int(rec.NumRows())
	is, ok1 := rec.Column(0).(*array.Int64)
	vs, ok2 := rec.Column(1).(*array.Int64)
	ss, ok3 := rec.Column(2).(*array.String)
	if !ok1 || !ok2 || !ok3 {
		return 0, fmt.Errorf("column types %T %T %T", rec.Column(0), rec.Column(1), rec.Column(2))
	}
	want := make([]byte, g.Width)
	for r := range n {
		i := g.Next
		if is.IsNull(r) || is.Value(r) != i {
			return r, fmt.Errorf("row %d: i = %d, want %d", i, is.Value(r), i)
		}
		if vs.IsNull(r) || vs.Value(r) != i*7919%1000003 {
			return r, fmt.Errorf("row %d: v = %d, want %d", i, vs.Value(r), i*7919%1000003)
		}
		for k := range want {
			want[k] = byte('a' + (int(i)+k)%26)
		}
		if ss.IsNull(r) || ss.Value(r) != string(want) {
			return r, fmt.Errorf("row %d: payload differs", i)
		}
		g.Next++
	}
	return n, nil
}
