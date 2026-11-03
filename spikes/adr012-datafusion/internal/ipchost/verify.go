// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"bytes"
	"errors"
	"fmt"
	"sync/atomic"

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
	// MaxDecodedBytes caps the memory the decoder may hold at once for one
	// Check; 0 means DefaultMaxDecodedBytes. A compressed buffer states its
	// own decoded size and the decoder allocates that much before it
	// decompresses, so a small frame could otherwise claim gigabytes.
	MaxDecodedBytes int64
}

// DefaultMaxDecodedBytes is 16 times the default frame limit. The worker
// never compresses, so a real batch decodes to about its frame size.
const DefaultMaxDecodedBytes = 16 * DefaultMaxFrame

// ErrDecodeLimit reports a batch that would decode past MaxDecodedBytes.
var ErrDecodeLimit = errors.New("arrow batch decodes past the verifier's memory limit")

// Check decodes one Arrow IPC stream and verifies every row. It returns
// the row count. A panic in the decoder is returned as an error: the bytes
// came from another process.
func (g *GenVerifier) Check(b []byte) (rows int, err error) {
	limit := g.MaxDecodedBytes
	if limit <= 0 {
		limit = DefaultMaxDecodedBytes
	}
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && errors.Is(e, ErrDecodeLimit) {
				err = e
				return
			}
			err = fmt.Errorf("arrow decode panicked: %v", r)
		}
	}()
	mem := &cappedAllocator{limit: limit}
	rd, err := ipc.NewReader(bytes.NewReader(b), ipc.WithAllocator(mem))
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

// cappedAllocator refuses, by panicking with ErrDecodeLimit, any allocation
// that would take the bytes it holds past limit. Check recovers the panic.
// The decoder offers no error path from an allocator, and returning a short
// buffer would corrupt memory instead of failing.
type cappedAllocator struct {
	live  atomic.Int64
	limit int64
}

func (a *cappedAllocator) reserve(n int64) {
	if n < 0 || a.live.Add(n) > a.limit {
		panic(ErrDecodeLimit)
	}
}

func (a *cappedAllocator) Allocate(size int) []byte {
	a.reserve(int64(size))
	return memory.DefaultAllocator.Allocate(size)
}

func (a *cappedAllocator) Reallocate(size int, b []byte) []byte {
	if grow := int64(size) - int64(len(b)); grow > 0 {
		a.reserve(grow)
	} else {
		a.live.Add(grow)
	}
	return memory.DefaultAllocator.Reallocate(size, b)
}

func (a *cappedAllocator) Free(b []byte) {
	a.live.Add(-int64(len(b)))
	memory.DefaultAllocator.Free(b)
}
