// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// parquetMemoryLimit caps allocations routed through Arrow while reading one
// file, including its page buffers. Codec-internal allocations may fall outside
// this cap. A variable only so tests can lower it.
var parquetMemoryLimit int64 = 512 << 20

// ErrParquetMemory reports a Parquet file that needs more than
// parquetMemoryLimit to read its first rows.
var ErrParquetMemory = errors.New("analytics: reading the Parquet file needs more than 512 MiB of memory")

// readParquet reads up to MaxImportRows rows through f, the already checked
// handle, in batches. Truncated comes from the file's own row count.
func readParquet(ctx context.Context, f *os.File, size int64) (ds Dataset, err error) {
	// Arrow can panic on a malformed file, and the capped allocator refuses
	// an allocation by panicking. Neither may take the application down.
	// recover only sees panics on this goroutine, so the reader must stay
	// serial: ArrowReadProperties.Parallel would move decoding (and these
	// panics) onto other goroutines and crash the app instead.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && errors.Is(e, ErrParquetMemory) {
				err = e
				return
			}
			err = fmt.Errorf("analytics: malformed Parquet file: %v", r)
		}
	}()
	if size < 12 {
		return Dataset{}, errors.New("analytics: the file is too short to be Parquet")
	}
	mem := &cappedAllocator{limit: parquetMemoryLimit}
	// Column chunks are read whole (the default). The allocator cap bounds
	// that memory. Buffered streaming in arrow-go v18.5.1 mis-read a
	// multi-row-group file DuckDB wrote ("snappy: corrupt input"), which the
	// many.parquet fixture covers.
	props := parquet.NewReaderProperties(mem)
	pf, err := file.NewParquetReader(readerAtOnly{f}, file.WithReadProps(props))
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: read Parquet: %w", err)
	}
	defer func() { _ = pf.Close() }()
	fr, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{BatchSize: 1024}, mem)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: read Parquet: %w", err)
	}
	rr, err := fr.GetRecordReader(ctx, nil, nil)
	if err != nil {
		return Dataset{}, fmt.Errorf("analytics: read Parquet: %w", err)
	}
	defer rr.Release()

	fields := rr.Schema().Fields()
	names := make([]string, len(fields))
	for i, fl := range fields {
		names[i] = fl.Name
	}
	ds = Dataset{Columns: uniqueNames(names), Rows: [][]any{}, Truncated: pf.NumRows() > MaxImportRows}
	for len(ds.Rows) < MaxImportRows && rr.Next() {
		if err := ctx.Err(); err != nil {
			return Dataset{}, err
		}
		rec := rr.RecordBatch()
		for row := 0; row < int(rec.NumRows()) && len(ds.Rows) < MaxImportRows; row++ {
			cells := make([]any, rec.NumCols())
			for c := range cells {
				cells[c] = arrowCell(rec.Column(c), row)
			}
			ds.Rows = append(ds.Rows, cells)
		}
	}
	if err := rr.Err(); err != nil && !errors.Is(err, io.EOF) {
		return Dataset{}, fmt.Errorf("analytics: read Parquet: %w", err)
	}
	return ds, nil
}

// readerAtOnly hides os.File's Close, so the Parquet reader's Close cannot
// close the caller's handle.
type readerAtOnly struct{ f *os.File }

func (r readerAtOnly) ReadAt(p []byte, off int64) (int, error) { return r.f.ReadAt(p, off) }
func (r readerAtOnly) Seek(off int64, whence int) (int64, error) {
	return r.f.Seek(off, whence)
}
func (r readerAtOnly) Read(p []byte) (int, error) { return r.f.Read(p) }

// arrowCell converts one Arrow value to a JSON-safe cell: numbers JavaScript
// holds exactly stay numbers; decimals, dates, times and nested values are
// text, so nothing is rounded or shown as "[object Object]".
func arrowCell(col arrow.Array, i int) any {
	if col.IsNull(i) {
		return nil
	}
	switch a := col.(type) {
	case *array.Boolean:
		return a.Value(i)
	case *array.Int8:
		return int64(a.Value(i))
	case *array.Int16:
		return int64(a.Value(i))
	case *array.Int32:
		return int64(a.Value(i))
	case *array.Int64:
		return intCell(a.Value(i))
	case *array.Uint8:
		return int64(a.Value(i))
	case *array.Uint16:
		return int64(a.Value(i))
	case *array.Uint32:
		return int64(a.Value(i))
	case *array.Uint64:
		if v := a.Value(i); v <= maxExactJSInteger {
			return int64(v)
		}
		return fmt.Sprint(a.Value(i))
	case *array.Float32:
		return floatCell(float64(a.Value(i)))
	case *array.Float64:
		return floatCell(a.Value(i))
	case *array.String:
		return a.Value(i)
	case *array.LargeString:
		return a.Value(i)
	case *array.Binary:
		return base64.StdEncoding.EncodeToString(a.Value(i))
	case *array.LargeBinary:
		return base64.StdEncoding.EncodeToString(a.Value(i))
	case *array.Date32:
		return a.Value(i).ToTime().Format(time.DateOnly)
	case *array.Date64:
		return a.Value(i).ToTime().Format(time.DateOnly)
	case *array.Timestamp:
		dt := a.DataType().(*arrow.TimestampType)
		t := a.Value(i).ToTime(dt.Unit)
		if dt.TimeZone == "" {
			// A timestamp without a zone is local wall time; do not label it UTC.
			return t.Format("2006-01-02 15:04:05.999999999")
		}
		return t.Format(time.RFC3339Nano)
	case *array.Time32:
		return a.Value(i).ToTime(a.DataType().(*arrow.Time32Type).Unit).Format("15:04:05.999999999")
	case *array.Time64:
		return a.Value(i).ToTime(a.DataType().(*arrow.Time64Type).Unit).Format("15:04:05.999999999")
	case *array.Decimal128:
		return a.Value(i).ToString(a.DataType().(*arrow.Decimal128Type).Scale)
	case *array.Decimal256:
		return a.Value(i).ToString(a.DataType().(*arrow.Decimal256Type).Scale)
	}
	b, err := json.Marshal(col.GetOneForMarshal(i))
	if err != nil {
		return col.ValueStr(i)
	}
	return string(b)
}

// cappedAllocator refuses, by panicking with ErrParquetMemory, any
// allocation that would take the bytes it holds past limit; readParquet
// recovers the panic. Arrow offers no error path from an allocator, and a
// short buffer would corrupt memory instead of failing.
type cappedAllocator struct {
	live  atomic.Int64
	limit int64
}

func (a *cappedAllocator) reserve(n int64) {
	if n < 0 || a.live.Add(n) > a.limit {
		panic(ErrParquetMemory)
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
