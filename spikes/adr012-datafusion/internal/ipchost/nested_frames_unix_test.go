// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

func TestNativeNestedFrameRecovery(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the freshly built native worker")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("DF0_WORKER must be absolute")
	}
	const sentinel = "UNSELECTED_NESTED_SENTINEL"
	selected := strings.Repeat("selected nested value", 32)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "nested", Type: arrow.StructOf(arrow.Field{Name: "list", Type: arrow.ListOf(arrow.BinaryTypes.String)})},
	}, nil)
	b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	t.Cleanup(b.Release)
	for i := 0; i < 3; i++ {
		b.Field(0).(*array.Int64Builder).Append(int64(i))
		nested := b.Field(1).(*array.StructBuilder)
		nested.Append(true)
		list := nested.FieldBuilder(0).(*array.ListBuilder)
		list.Append(true)
		value := selected
		if i != 1 {
			value = strings.Repeat(sentinel, 128<<10)
		}
		list.ValueBuilder().(*array.StringBuilder).Append(value)
	}
	rec := b.NewRecordBatch()
	t.Cleanup(rec.Release)
	table := array.NewTableFromRecords(schema, []arrow.RecordBatch{rec})
	t.Cleanup(table.Release)
	var fixture bytes.Buffer
	props := parquet.NewWriterProperties(parquet.WithCompression(compress.Codecs.Uncompressed), parquet.WithDictionaryDefault(false))
	if err := pqarrow.WriteTable(table, &fixture, 3, props, pqarrow.NewArrowWriterProperties()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nested.parquet")
	if err := os.WriteFile(path, fixture.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	type observation struct {
		Name            string `json:"name"`
		Outcome         string `json:"outcome"`
		FailedCode      string `json:"failed_code,omitempty"`
		Detail          string `json:"detail,omitempty"`
		Rows            uint64 `json:"rows"`
		DigestMatch     *bool  `json:"digest_match,omitempty"`
		SourceMatch     *bool  `json:"source_match,omitempty"`
		WorkerExit      string `json:"worker_exit"`
		EndpointRemoved bool   `json:"endpoint_removed"`
		Killed          bool   `json:"killed"`
		MaxArrowBytes   int    `json:"max_arrow_bytes"`
		NestedLeafType  string `json:"nested_leaf_type,omitempty"`
	}
	var all []observation
	for _, tc := range []struct {
		name, operation string
		oversize        bool
	}{{"range", "parquet_range", false}, {"stream", "parquet_stream", false}, {"range_oversize", "parquet_range", true}, {"stream_oversize", "parquet_stream", true}, {"recovery", "parquet_range", false}} {
		snap, err := source.Capture(path)
		if err != nil {
			t.Fatal(err)
		}
		id := int64(1)
		if tc.oversize {
			id = 0
		}
		maxArrow := 0
		leafType := ""
		verify := func(data []byte) (int, error) {
			maxArrow = max(maxArrow, len(data))
			if bytes.Contains(data, []byte(sentinel)) {
				return 0, fmt.Errorf("unselected backing bytes entered a result frame")
			}
			rd, err := ipc.NewReader(bytes.NewReader(data))
			if err != nil {
				return 0, err
			}
			defer rd.Release()
			n := 0
			for rd.Next() {
				r := rd.RecordBatch()
				if r.NumRows() != 1 || r.NumCols() != 2 || r.Schema().Field(0).Name != "id" || r.Schema().Field(1).Name != "nested" {
					return n, fmt.Errorf("wrong nested result shape")
				}
				ids, ok := r.Column(0).(*array.Int64)
				if !ok || ids.IsNull(0) || ids.Value(0) != 1 {
					return n, fmt.Errorf("wrong selected id")
				}
				nested, ok := r.Column(1).(*array.Struct)
				if !ok || nested.NumField() != 1 || nested.IsNull(0) {
					return n, fmt.Errorf("wrong nested struct")
				}
				list, ok := nested.Field(0).(*array.List)
				if !ok || list.IsNull(0) || list.ListValues().DataType().ID() != arrow.STRING {
					return n, fmt.Errorf("ordinary-string compatibility fixture returned a different type")
				}
				leafType = list.ListValues().DataType().String()
				lo, hi := list.ValueOffsets(0)
				if hi-lo != 1 || list.ListValues().ValueStr(int(lo)) != selected {
					return n, fmt.Errorf("wrong nested selected value")
				}
				n++
			}
			return n, rd.Err()
		}
		r := Run(Job{Name: tc.name, Worker: worker, ReadSource: snap, MaxFrame: 64 << 10, Timeout: 30 * time.Second, Verify: verify,
			Execute: &df0pb.Execute{Operation: tc.operation, BatchRows: 3, FilterId: true, LowerId: id, UpperId: id + 1}})
		if !r.EndpointRemoved || r.Killed || r.WorkerExit != "exit 0" || strings.Contains(r.Detail+r.WorkerDiag, sentinel) {
			t.Fatalf("%s: cleanup/diagnostic failure: %+v", tc.name, r)
		}
		if tc.oversize {
			if r.Outcome != "failed" || r.FailedCode != "RESOURCE_LIMIT" || r.Rows != 0 || maxArrow != 0 || !strings.HasPrefix(r.Detail, "one row needs at least ") || !strings.HasSuffix(r.Detail, ", over the frame cap") {
				t.Fatalf("%s: oversized row not refused before output: %+v", tc.name, r)
			}
		} else if r.Outcome != "completed" || r.Rows != 1 || r.DigestMatch == nil || !*r.DigestMatch || r.SourceMatch == nil || !*r.SourceMatch || maxArrow == 0 || maxArrow >= 64<<10 {
			t.Fatalf("%s: incomplete selected nested result: %+v", tc.name, r)
		}
		all = append(all, observation{Name: tc.name, Outcome: r.Outcome, FailedCode: r.FailedCode, Detail: r.Detail, Rows: r.Rows,
			DigestMatch: r.DigestMatch, SourceMatch: r.SourceMatch, WorkerExit: r.WorkerExit,
			EndpointRemoved: r.EndpointRemoved, Killed: r.Killed, MaxArrowBytes: maxArrow, NestedLeafType: leafType})
		t.Logf("%s: outcome=%s rows=%d Arrow bytes=%d cleanup=%v", tc.name, r.Outcome, r.Rows, maxArrow, r.EndpointRemoved)
	}
	if path := os.Getenv("DF0_NESTED_FRAME_EVIDENCE"); path != "" {
		data, err := json.MarshalIndent(all, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
