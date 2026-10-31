// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/decimal256"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// pqColumn is one Parquet column: its Arrow field, a function that appends
// row i to the column builder, and the expected cell for row i.
type pqColumn struct {
	field  arrow.Field
	append func(b array.Builder, i int)
	want   func(i int) Cell
}

type pqOptions struct {
	codec       compress.Compression
	rowGroupLen int64
	dictionary  bool
	metadata    map[string]string
}

func defaultPQ() pqOptions {
	return pqOptions{codec: compress.Codecs.Snappy, rowGroupLen: 64 * 1024, dictionary: true}
}

// parquetCase writes rows through arrow-go's writer. The truth is the typed
// value the generator appended, not anything read back.
func parquetCase(id, file string, class Class, note string, n int, cols []pqColumn, opt pqOptions) TabularCase {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.field.Name
	}
	return TabularCase{
		ID: id, File: file, Format: "parquet", Class: class, Note: note, Accept: AcceptRows,
		Columns: names, TotalRows: n,
		Row: func(i int) []Cell {
			out := make([]Cell, len(cols))
			for j, c := range cols {
				out[j] = c.want(i)
			}
			return out
		},
		write: func(w io.Writer) error { return writeParquet(w, n, cols, opt) },
	}
}

func writeParquet(w io.Writer, n int, cols []pqColumn, opt pqOptions) error {
	fields := make([]arrow.Field, len(cols))
	for i, c := range cols {
		fields[i] = c.field
	}
	var md *arrow.Metadata
	if len(opt.metadata) > 0 {
		keys, vals := []string{}, []string{}
		for k, v := range opt.metadata {
			keys, vals = append(keys, k), append(vals, v)
		}
		sortPairs(keys, vals)
		m := arrow.NewMetadata(keys, vals)
		md = &m
	}
	schema := arrow.NewSchema(fields, md)
	mem := memory.NewGoAllocator()
	rb := array.NewRecordBuilder(mem, schema)
	defer rb.Release()
	for i := 0; i < n; i++ {
		for j, c := range cols {
			c.append(rb.Field(j), i)
		}
	}
	rec := rb.NewRecordBatch()
	defer rec.Release()
	tbl := array.NewTableFromRecords(schema, []arrow.RecordBatch{rec})
	defer tbl.Release()
	props := parquet.NewWriterProperties(
		parquet.WithCompression(opt.codec),
		parquet.WithDictionaryDefault(opt.dictionary),
		parquet.WithMaxRowGroupLength(opt.rowGroupLen),
		parquet.WithCreatedBy("gopmgr-df0-fixtures"),
	)
	chunk := opt.rowGroupLen
	if chunk <= 0 || int64(n) < chunk {
		chunk = int64(max(n, 1))
	}
	return pqarrow.WriteTable(tbl, w, chunk, props, pqarrow.NewArrowWriterProperties(pqarrow.WithStoreSchema()))
}

func sortPairs(k, v []string) {
	for i := 1; i < len(k); i++ {
		for j := i; j > 0 && k[j] < k[j-1]; j-- {
			k[j], k[j-1] = k[j-1], k[j]
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// nullable wraps a column so that the rows in nulls are null.
func nullable(c pqColumn, nulls map[int]bool) pqColumn {
	c.field.Nullable = true
	inner, want := c.append, c.want
	c.append = func(b array.Builder, i int) {
		if nulls[i] {
			b.AppendNull()
			return
		}
		inner(b, i)
	}
	c.want = func(i int) Cell {
		if nulls[i] {
			return N()
		}
		return want(i)
	}
	return c
}

func int64Col(name string, v func(i int) int64) pqColumn {
	return pqColumn{field: arrow.Field{Name: name, Type: arrow.PrimitiveTypes.Int64},
		append: func(b array.Builder, i int) { b.(*array.Int64Builder).Append(v(i)) },
		want:   func(i int) Cell { return I(strconv.FormatInt(v(i), 10)) }}
}

func strCol(name string, v func(i int) string) pqColumn {
	return pqColumn{field: arrow.Field{Name: name, Type: arrow.BinaryTypes.String},
		append: func(b array.Builder, i int) { b.(*array.StringBuilder).Append(v(i)) },
		want:   func(i int) Cell { return S(v(i)) }}
}

func floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	case f == 0 && math.Signbit(f):
		return "-0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func pick[T any](vals []T) func(i int) T { return func(i int) T { return vals[i%len(vals)] } }

func scalarColumns() []pqColumn {
	ints := []int64{0, 1, -1, math.MaxInt64, math.MinInt64, 1<<53 + 1}
	floats := []float64{0.1, -2.5, math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1)}
	strs := []string{"Alpha", "Café", "项目", "🚀", "", "tab\tnew\nline"}
	bins := [][]byte{{0x00}, {0xff, 0xfe}, []byte("abc"), {}, {0x80}, bytes.Repeat([]byte{0x5a}, 3)}
	u64 := []uint64{0, 1, math.MaxUint64, 1 << 63, 42, 7}
	i32 := []int32{0, math.MaxInt32, math.MinInt32, 5, -5, 9}
	return []pqColumn{
		int64Col("i64", pick(ints)),
		{field: arrow.Field{Name: "i32", Type: arrow.PrimitiveTypes.Int32},
			append: func(b array.Builder, i int) { b.(*array.Int32Builder).Append(i32[i%6]) },
			want:   func(i int) Cell { return I(strconv.FormatInt(int64(i32[i%6]), 10)) }},
		{field: arrow.Field{Name: "u64", Type: arrow.PrimitiveTypes.Uint64},
			append: func(b array.Builder, i int) { b.(*array.Uint64Builder).Append(u64[i%6]) },
			want:   func(i int) Cell { return I(strconv.FormatUint(u64[i%6], 10)) }},
		{field: arrow.Field{Name: "f64", Type: arrow.PrimitiveTypes.Float64},
			append: func(b array.Builder, i int) { b.(*array.Float64Builder).Append(floats[i%6]) },
			want:   func(i int) Cell { return F(floatText(floats[i%6])) }},
		{field: arrow.Field{Name: "flag", Type: arrow.FixedWidthTypes.Boolean},
			append: func(b array.Builder, i int) { b.(*array.BooleanBuilder).Append(i%2 == 0) },
			want:   func(i int) Cell { return B(i%2 == 0) }},
		strCol("s", pick(strs)),
		{field: arrow.Field{Name: "bin", Type: arrow.BinaryTypes.Binary},
			append: func(b array.Builder, i int) { b.(*array.BinaryBuilder).Append(bins[i%6]) },
			want:   func(i int) Cell { return Hex(hex.EncodeToString(bins[i%6])) }},
	}
}

func decimalColumns() []pqColumn {
	d128 := []string{"12345.6789", "-0.0001", "9999999999999999999999999999999999.9999", "0.0000", "-42.5000", "1.0000"}
	d9 := []string{"1.23", "-1.23", "9999999.99", "0.00", "0.01", "-0.01"}
	d256 := []string{"12345678901234567890123456789012345678901234567890.1234567890", "-1.0000000000", "0.0000000001", "0.0000000000", "7.0000000000", "-7.5000000000"}
	col128 := func(name string, prec, scale int32, vals []string) pqColumn {
		return pqColumn{field: arrow.Field{Name: name, Type: &arrow.Decimal128Type{Precision: prec, Scale: scale}},
			append: func(b array.Builder, i int) {
				v, err := decimal128.FromString(vals[i%len(vals)], prec, scale)
				if err != nil {
					panic(err)
				}
				b.(*array.Decimal128Builder).Append(v)
			},
			want: func(i int) Cell { return D(vals[i%len(vals)]) }}
	}
	return []pqColumn{
		col128("d38_4", 38, 4, d128),
		col128("d9_2", 9, 2, d9),
		{field: arrow.Field{Name: "d60_10", Type: &arrow.Decimal256Type{Precision: 60, Scale: 10}},
			append: func(b array.Builder, i int) {
				v, err := decimal256.FromString(d256[i%6], 60, 10)
				if err != nil {
					panic(err)
				}
				b.(*array.Decimal256Builder).Append(v)
			},
			want: func(i int) Cell { return D(d256[i%6]) }},
	}
}

func timeColumns() []pqColumn {
	instants := []time.Time{
		time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC),
		time.Date(2026, 3, 8, 6, 59, 59, 0, time.UTC), // one second before US DST starts
		time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC),
		time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2262, 4, 11, 0, 0, 0, 0, time.UTC),
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	nanos := []time.Time{instants[0].Add(789 * time.Nanosecond), instants[1], instants[2], instants[3], time.Unix(0, math.MaxInt64).UTC(), time.Unix(0, math.MinInt64).UTC()}
	us := &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
	ns := &arrow.TimestampType{Unit: arrow.Nanosecond}
	ms := &arrow.TimestampType{Unit: arrow.Millisecond, TimeZone: "America/New_York"}
	return []pqColumn{
		{field: arrow.Field{Name: "ts_us_utc", Type: us},
			append: func(b array.Builder, i int) {
				b.(*array.TimestampBuilder).Append(arrow.Timestamp(instants[i%6].UnixMicro()))
			},
			want: func(i int) Cell { return Tm(instants[i%6].UTC().Format(time.RFC3339Nano)) }},
		{field: arrow.Field{Name: "ts_ns_naive", Type: ns},
			append: func(b array.Builder, i int) {
				b.(*array.TimestampBuilder).Append(arrow.Timestamp(nanos[i%6].UnixNano()))
			},
			// A timestamp without a time zone is a wall-clock value; it is
			// written here as the UTC wall clock with no zone suffix.
			want: func(i int) Cell { return Tm(strings.TrimSuffix(nanos[i%6].UTC().Format(time.RFC3339Nano), "Z")) }},
		{field: arrow.Field{Name: "ts_ms_ny", Type: ms},
			append: func(b array.Builder, i int) {
				b.(*array.TimestampBuilder).Append(arrow.Timestamp(instants[i%6].UnixMilli()))
			},
			want: func(i int) Cell { return Tm(instants[i%6].Truncate(time.Millisecond).UTC().Format(time.RFC3339Nano)) }},
		{field: arrow.Field{Name: "day", Type: arrow.FixedWidthTypes.Date32},
			append: func(b array.Builder, i int) { b.(*array.Date32Builder).Append(arrow.Date32FromTime(instants[i%6])) },
			want:   func(i int) Cell { return Dt(instants[i%6].Format("2006-01-02")) }},
	}
}

func nestedColumns() []pqColumn {
	lists := [][]int64{{1, 2, 3}, {}, {-1}, {math.MaxInt64}, {0, 0}, {7}}
	structs := []struct {
		a int64
		b string
	}{{1, "x"}, {2, "y"}, {0, ""}, {-3, "z"}, {4, "é"}, {5, "🚀"}}
	maps := [][][2]any{{{"k1", int64(1)}, {"k2", int64(2)}}, {}, {{"only", int64(-5)}}, {{"a", int64(0)}}, {{"b", int64(9)}, {"c", int64(10)}}, {{"z", int64(1)}}}
	nested := [][][]string{{{"a"}, {"b", "c"}}, {}, {{}}, {{"x"}}, {{"y", "z"}}, {{"é"}}}
	listJSON := func(v []int64) string {
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = strconv.FormatInt(x, 10)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	return []pqColumn{
		{field: arrow.Field{Name: "ints", Type: arrow.ListOf(arrow.PrimitiveTypes.Int64)},
			append: func(b array.Builder, i int) {
				lb := b.(*array.ListBuilder)
				lb.Append(true)
				for _, x := range lists[i%6] {
					lb.ValueBuilder().(*array.Int64Builder).Append(x)
				}
			},
			want: func(i int) Cell { return J(listJSON(lists[i%6])) }},
		{field: arrow.Field{Name: "rec", Type: arrow.StructOf(arrow.Field{Name: "a", Type: arrow.PrimitiveTypes.Int64}, arrow.Field{Name: "b", Type: arrow.BinaryTypes.String})},
			append: func(b array.Builder, i int) {
				sb := b.(*array.StructBuilder)
				sb.Append(true)
				sb.FieldBuilder(0).(*array.Int64Builder).Append(structs[i%6].a)
				sb.FieldBuilder(1).(*array.StringBuilder).Append(structs[i%6].b)
			},
			want: func(i int) Cell {
				return J(fmt.Sprintf("{\"a\":%d,\"b\":%s}", structs[i%6].a, strconv.Quote(structs[i%6].b)))
			}},
		{field: arrow.Field{Name: "kv", Type: arrow.MapOf(arrow.BinaryTypes.String, arrow.PrimitiveTypes.Int64)},
			append: func(b array.Builder, i int) {
				mb := b.(*array.MapBuilder)
				mb.Append(true)
				for _, e := range maps[i%6] {
					mb.KeyBuilder().(*array.StringBuilder).Append(e[0].(string))
					mb.ItemBuilder().(*array.Int64Builder).Append(e[1].(int64))
				}
			},
			want: func(i int) Cell {
				parts := []string{}
				for _, e := range maps[i%6] {
					parts = append(parts, fmt.Sprintf("%s:%d", strconv.Quote(e[0].(string)), e[1].(int64)))
				}
				return J("{" + strings.Join(parts, ",") + "}")
			}},
		{field: arrow.Field{Name: "nested", Type: arrow.ListOf(arrow.ListOf(arrow.BinaryTypes.String))},
			append: func(b array.Builder, i int) {
				lb := b.(*array.ListBuilder)
				lb.Append(true)
				inner := lb.ValueBuilder().(*array.ListBuilder)
				for _, l := range nested[i%6] {
					inner.Append(true)
					for _, s := range l {
						inner.ValueBuilder().(*array.StringBuilder).Append(s)
					}
				}
			},
			want: func(i int) Cell {
				outer := []string{}
				for _, l := range nested[i%6] {
					inner := []string{}
					for _, s := range l {
						inner = append(inner, strconv.Quote(s))
					}
					outer = append(outer, "["+strings.Join(inner, ",")+"]")
				}
				return J("[" + strings.Join(outer, ",") + "]")
			}},
	}
}

func sizedParquetColumns() []pqColumn {
	return []pqColumn{
		int64Col("id", func(i int) int64 { return int64(i + 1) }),
		strCol("wbs", func(i int) string { return fmt.Sprintf("WBS-%03d", i%250) }), // low cardinality: dictionary pages
		{field: arrow.Field{Name: "cost", Type: &arrow.Decimal128Type{Precision: 18, Scale: 2}},
			append: func(b array.Builder, i int) {
				b.(*array.Decimal128Builder).Append(decimal128.FromI64(int64(i)*137 - 500_000))
			},
			want: func(i int) Cell { return D(centsText(int64(i)*137 - 500_000)) }},
	}
}

func centsText(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

func parquetCases() []TabularCase {
	d := defaultPQ()
	nulls := map[int]bool{1: true, 4: true}
	var scalarsWithNulls []pqColumn
	for _, c := range scalarColumns() {
		scalarsWithNulls = append(scalarsWithNulls, nullable(c, nulls))
	}
	cases := []TabularCase{
		parquetCase("pq-scalars", "scalars.parquet", LegacyParity, "int64/int32/uint64/float64 (NaN, ±Inf, -0)/bool/string/binary", 6, scalarColumns(), d),
		parquetCase("pq-scalars-nulls", "scalars-nulls.parquet", LegacyParity, "the scalar table with null rows", 6, scalarsWithNulls, d),
		parquetCase("pq-decimals", "decimals.parquet", PlausibleProduction, "decimal128(38,4), decimal128(9,2), decimal256(60,10)", 6, decimalColumns(), d),
		parquetCase("pq-timestamps", "timestamps.parquet", PlausibleProduction, "UTC µs, naive ns at the int64 limits, New York ms, date32", 6, timeColumns(), d),
		parquetCase("pq-nested", "nested.parquet", FutureS7A, "list, struct, map, list of lists", 6, nestedColumns(), d),
		parquetCase("pq-rowgroups", "rowgroups.parquet", PlausibleProduction, "25,000 rows in 5 row groups, dictionary-encoded strings", 25_000, sizedParquetColumns(),
			pqOptions{codec: compress.Codecs.Snappy, rowGroupLen: 5_000, dictionary: true}),
		parquetCase("pq-plain", "plain.parquet", LegacyParity, "dictionary encoding disabled", 1_000, sizedParquetColumns(),
			pqOptions{codec: compress.Codecs.Uncompressed, rowGroupLen: 64 * 1024, dictionary: false}),
		parquetCase("pq-metadata", "metadata.parquet", AdversarialStress, "unusual key-value metadata: Unicode keys, a 64 KiB value", 3, sizedParquetColumns(),
			pqOptions{codec: compress.Codecs.Snappy, rowGroupLen: 64 * 1024, dictionary: true,
				metadata: map[string]string{"gopmgr.ünïcode": "✓", "gopmgr.large": strings.Repeat("m", 64*1024), "": "empty key"}}),
		parquetCase("pq-empty", "empty.parquet", AdversarialStress, "a schema and zero rows", 0, sizedParquetColumns(), d),
		parquetCase("pq-rows-10001", "rows-10001.parquet", PlausibleProduction, "truncation boundary", 10_001, sizedParquetColumns(), d),
		parquetCase("pq-rows-100000", "rows-100000.parquet", AdversarialStress, "100,000 rows", 100_000, sizedParquetColumns(), d),
	}
	for _, c := range []struct {
		name  string
		codec compress.Compression
	}{{"gzip", compress.Codecs.Gzip}, {"zstd", compress.Codecs.Zstd}, {"brotli", compress.Codecs.Brotli}, {"lz4raw", compress.Codecs.Lz4Raw}, {"none", compress.Codecs.Uncompressed}} {
		cases = append(cases, parquetCase("pq-codec-"+c.name, "codec-"+c.name+".parquet", LegacyParity, "compression codec "+c.name, 2_000, sizedParquetColumns(),
			pqOptions{codec: c.codec, rowGroupLen: 64 * 1024, dictionary: true}))
	}
	truncated := TabularCase{ID: "pq-corrupt-footer", File: "corrupt.parquet", Format: "parquet", Class: AdversarialStress,
		Note: "a valid file with its last 7 bytes removed", Accept: AcceptError,
		write: func(w io.Writer) error {
			var buf bytes.Buffer
			if err := writeParquet(&buf, 100, sizedParquetColumns(), d); err != nil {
				return err
			}
			_, err := w.Write(buf.Bytes()[:buf.Len()-7])
			return err
		}}
	notParquet := TabularCase{ID: "pq-not-parquet", File: "text.parquet", Format: "parquet", Class: AdversarialStress,
		Note: "CSV text with a .parquet extension", Accept: AcceptError, write: raw([]byte("id,name\n1,a\n"))}
	return append(cases, truncated, notParquet)
}

// ReadAtRows is the bounded DF0-C2 comparison corpus, separate from DF0-A.
const ReadAtRows = 2048
const ReadAtGroupRows = 256
const ReadAtFirstID int64 = 1<<53 + 1

// WriteReadAtParquet writes eight sorted row groups and an unprojected payload.
// The oracle is these generator values, never a second Parquet reader.
func WriteReadAtParquet(w io.Writer) error {
	cols := []pqColumn{
		int64Col("id", func(i int) int64 { return ReadAtFirstID + int64(i) }),
		int64Col("value", func(i int) int64 { return int64(i)*137 - 500000 }),
		strCol("payload", func(i int) string { return strings.Repeat(fmt.Sprintf("%08x", i*7919), 128) }),
	}
	return writeParquet(w, ReadAtRows, cols, pqOptions{codec: compress.Codecs.Uncompressed, rowGroupLen: ReadAtGroupRows, dictionary: false})
}
