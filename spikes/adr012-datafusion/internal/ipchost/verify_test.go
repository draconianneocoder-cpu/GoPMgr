// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// zstdGen writes one zstd-compressed generate-schema batch of a single row
// whose payload is width bytes, valid for a GenVerifier of that width.
func zstdGen(t *testing.T, width int) []byte {
	t.Helper()
	b := array.NewRecordBuilder(memory.DefaultAllocator, GenSchema)
	defer b.Release()
	p := make([]byte, width)
	for k := range p {
		p[k] = byte('a' + k%26)
	}
	b.Field(0).(*array.Int64Builder).Append(0)
	b.Field(1).(*array.Int64Builder).Append(0)
	b.Field(2).(*array.StringBuilder).Append(string(p))
	rec := b.NewRecordBatch()
	defer rec.Release()
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(GenSchema), ipc.WithZstd())
	if err := w.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// SEC-003: a batch from the worker must not make the host allocate more
// than the verifier's limit, however small the frame that carries it.
func TestGenVerifierBoundsDecodedMemory(t *testing.T) {
	const width = 8 << 20 // compresses to a few hundred bytes

	t.Run("compressed batch under the limit verifies", func(t *testing.T) {
		g := &GenVerifier{Width: 300}
		if rows, err := g.Check(zstdGen(t, 300)); err != nil || rows != 1 {
			t.Fatalf("rows %d, err %v", rows, err)
		}
	})

	t.Run("compression bomb", func(t *testing.T) {
		bomb := zstdGen(t, width)
		if len(bomb) > 64<<10 {
			t.Fatalf("bomb is %d bytes; the test needs a small frame", len(bomb))
		}
		g := &GenVerifier{Width: width, MaxDecodedBytes: 1 << 20}
		if _, err := g.Check(bomb); !errors.Is(err, ErrDecodeLimit) {
			t.Fatalf("err = %v, want ErrDecodeLimit", err)
		}
	})

	t.Run("forged decoded size", func(t *testing.T) {
		// Each compressed buffer starts with its decoded length as a
		// little-endian int64. Claim 1 GiB for the payload buffer: an
		// unbounded decoder allocates that before reading the data.
		frame := zstdGen(t, 300_017)
		want := binary.LittleEndian.AppendUint64(nil, 300_017)
		if n := bytes.Count(frame, want); n != 1 {
			t.Fatalf("found the payload length %d times, want once", n)
		}
		at := bytes.Index(frame, want)
		binary.LittleEndian.PutUint64(frame[at:], 1<<30)
		g := &GenVerifier{Width: 300_017}
		_, err := g.Check(frame)
		if !errors.Is(err, ErrDecodeLimit) {
			t.Fatalf("err = %v, want ErrDecodeLimit", err)
		}
		if !strings.Contains(err.Error(), "memory limit") {
			t.Fatalf("err = %v does not name the limit", err)
		}
	})
}
