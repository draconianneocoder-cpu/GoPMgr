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
	"reflect"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

// These tests launch the actual native worker. Absence is explicitly a skip.
func TestNativeReadAt(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the freshly built native worker")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("DF0_WORKER must be absolute")
	}
	path := filepath.Join(t.TempDir(), "literal?[x]*.parquet")
	var fixture bytes.Buffer
	if err := fixtures.WriteReadAtParquet(&fixture); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fixture.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "decoy.parquet"), []byte("not selected"), 0600); err != nil {
		t.Fatal(err)
	}
	type observation struct {
		Result
		CaptureNS             int64   `json:"capture_ns"`
		EndToEndNS            int64   `json:"end_to_end_ns"`
		ReadAmplification     float64 `json:"read_amplification"`
		PhysicalAmplification float64 `json:"physical_amplification"`
	}
	var evidence []observation
	for _, mode := range []string{"range_all", "range_pruned", "stream_pruned", "range_bad_job", "range_bad_handle", "range_bad_version", "range_bad_range", "range_replay", "range_quota", "range_cancel", "range_corrupt_hash", "range_corrupt_version", "range_corrupt_offset", "range_corrupt_request_id", "range_empty", "range_bad_counts", "range_bad_projection", "range_recovery"} {
		t.Run(mode, func(t *testing.T) {
			captureStart := time.Now()
			snap, err := source.Capture(path)
			if err != nil {
				t.Fatal(err)
			}
			captureNS := time.Since(captureStart).Nanoseconds()
			first, last := 3*fixtures.ReadAtGroupRows, 4*fixtures.ReadAtGroupRows
			ex := &df0pb.Execute{Operation: "parquet_range", BatchRows: 128, Projection: []string{"id", "value"}, FilterId: true, LowerId: fixtures.ReadAtFirstID + int64(first), UpperId: fixtures.ReadAtFirstID + int64(last)}
			if mode == "range_all" {
				ex.FilterId = false
				first, last = 0, fixtures.ReadAtRows
			}
			if mode == "range_empty" || mode == "range_bad_counts" {
				last = first
				ex.UpperId = ex.LowerId
			}
			if mode == "range_bad_projection" {
				ex.Projection = []string{"not_a_column"}
			}
			if mode == "stream_pruned" {
				ex.Operation = "parquet_stream"
			}
			seen := map[int64]bool{}
			job := Job{Name: mode, Worker: worker, Execute: ex, ReadSource: snap, Timeout: 30 * time.Second}
			job.Verify = func(data []byte) (int, error) {
				rd, err := ipc.NewReader(bytes.NewReader(data))
				if err != nil {
					return 0, err
				}
				defer rd.Release()
				n := 0
				for rd.Next() {
					rec := rd.RecordBatch()
					if rec.NumCols() != 2 || rec.Schema().Field(0).Name != "id" || rec.Schema().Field(1).Name != "value" {
						return 0, fmt.Errorf("wrong projection: %v", rec.Schema())
					}
					ids, ok := rec.Column(0).(*array.Int64)
					if !ok {
						return 0, fmt.Errorf("id lost int64 type")
					}
					vals, ok := rec.Column(1).(*array.Int64)
					if !ok {
						return 0, fmt.Errorf("value lost int64 type")
					}
					for i := 0; i < ids.Len(); i++ {
						index := ids.Value(i) - fixtures.ReadAtFirstID
						if ids.IsNull(i) || vals.IsNull(i) || index < int64(first) || index >= int64(last) || seen[index] || vals.Value(i) != index*137-500000 {
							return 0, fmt.Errorf("wrong or duplicate row %d", index)
						}
						seen[index] = true
						n++
					}
				}
				return n, rd.Err()
			}
			switch mode {
			case "range_bad_counts":
				ex.Fault = "source_bad_count"
			case "range_bad_job":
				ex.Fault = "source_bad_job"
			case "range_bad_handle":
				ex.Fault = "source_bad_handle"
			case "range_bad_version":
				ex.Fault = "source_bad_version"
			case "range_bad_range":
				ex.Fault = "source_bad_range"
			case "range_replay":
				ex.Fault = "source_replay"
			case "range_quota":
				job.ReadLimits = &source.Limits{ReadBytes: 4096, TotalBytes: 1, Requests: 1, InFlight: 1}
			case "range_cancel":
				job.CancelAfterRead = 1
			case "range_corrupt_hash":
				job.CorruptReadResponse = "hash"
			case "range_corrupt_version":
				job.CorruptReadResponse = "version"
			case "range_corrupt_offset":
				job.CorruptReadResponse = "offset"
			case "range_corrupt_request_id":
				job.CorruptReadResponse = "request_id"
			}
			r := Run(job)
			if r.SourceIO == nil {
				t.Fatalf("source setup failed: %+v", r)
			}
			o := observation{Result: r, CaptureNS: captureNS, EndToEndNS: time.Since(captureStart).Nanoseconds()}
			if r.SourceIO != nil && r.SourceIO.PhysicalBytes > 0 {
				o.ReadAmplification = float64(r.SourceIO.ActualBytes) / float64(r.SourceIO.PhysicalBytes)
				o.PhysicalAmplification = 1 // capture reads the complete source once
			}
			evidence = append(evidence, o)
			if !r.EndpointRemoved || r.WorkerExit == "" {
				t.Fatalf("not cleaned up: %+v", r)
			}
			success := mode == "range_all" || mode == "range_pruned" || mode == "stream_pruned" || mode == "range_recovery" || mode == "range_empty"
			if success {
				if ex.Operation == "parquet_range" {
					footerRead := false
					for _, read := range r.SourceReads {
						if read.Offset+uint64(read.Length) == r.SourceIO.PhysicalBytes && read.Length >= 8 {
							footerRead = true
						}
					}
					if !footerRead {
						t.Fatalf("no footer range observed: %+v", r.SourceReads)
					}
				}
				if r.Outcome != "completed" || r.Killed || r.WorkerExit != "exit 0" || r.SourceMatch == nil || !*r.SourceMatch || r.DigestMatch == nil || !*r.DigestMatch || len(seen) != last-first {
					t.Fatalf("native oracle failed: %+v; diag=%s; rows=%d", r, r.WorkerDiag, len(seen))
				}
			} else if mode == "range_bad_counts" {
				if r.Outcome != "rejected_counts" || r.Rows != 0 {
					t.Fatalf("invalid empty count accepted: %+v", r)
				}
			} else if mode == "range_cancel" {
				if r.Outcome != "failed" || r.FailedCode != "USER_CANCELLED" || r.Rows != 0 || r.CancelToExitNS < 0 {
					t.Fatalf("cancel: %+v %s", r, r.WorkerDiag)
				}
			} else if job.CorruptReadResponse != "" || mode == "range_bad_projection" {
				if r.Outcome != "failed" || r.FailedCode != "INPUT_INVALID" || r.Rows != 0 {
					t.Fatalf("corrupt reply accepted: %+v %s", r, r.WorkerDiag)
				}
			} else if r.Outcome != "rejected_source" || r.SourceIO == nil || r.SourceIO.Rejected != 1 || r.Rows != 0 {
				t.Fatalf("bad request accepted: %+v %s", r, r.WorkerDiag)
			}
			if r.SourceIO == nil || r.SourceIO.PeakInFlight > 1 {
				t.Fatalf("missing/bad source stats: %+v", r)
			}
			if mode == "range_bad_job" || mode == "range_bad_handle" || mode == "range_bad_version" || mode == "range_bad_range" || mode == "range_quota" {
				if r.SourceIO.ActualBytes != 0 {
					t.Fatalf("refused request disclosed bytes: %+v", r.SourceIO)
				}
			}
		})
	}
	if len(evidence) >= 3 && evidence[0].Outcome == "completed" && evidence[1].Outcome == "completed" && evidence[2].Outcome == "completed" {
		all, pruned, stream := evidence[0].SourceIO, evidence[1].SourceIO, evidence[2].SourceIO
		if pruned.ActualBytes >= all.ActualBytes || pruned.ActualBytes >= stream.ActualBytes || stream.ActualBytes != stream.PhysicalBytes || pruned.PhysicalBytes != stream.PhysicalBytes {
			t.Fatalf("no pruning/wire saving or incorrect physical accounting: %v %v %v", all, pruned, stream)
		}
	}
	if out := os.Getenv("DF0_READAT_EVIDENCE"); out != "" {
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeExistingOperations(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the freshly built native worker")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("DF0_WORKER must be absolute")
	}
	for _, op := range []string{"generate", "duplex", "rollup"} {
		t.Run(op, func(t *testing.T) {
			verifier := &GenVerifier{Width: 8}
			job := Job{Name: "legacy_" + op, Worker: worker, Timeout: 30 * time.Second, Execute: &df0pb.Execute{Operation: op, Rows: 8, BatchRows: 4, PayloadBytes: 8}, Verify: verifier.Check}
			if op == "duplex" {
				job.Source = []byte("selected synthetic source")
			}
			if op == "rollup" {
				job.Verify = nil
				job.Source = []byte(`[{"budgeted_cost_minor_units":"9007199254740993","committed_cost_minor_units":"5","actual_cost_minor_units":"10","earned_value_minor_units":"20","planned_value_minor_units":"40","evm_available":true}]`)
			}
			r := Run(job)
			if r.Outcome != "completed" || r.Killed || r.WorkerExit != "exit 0" || !r.EndpointRemoved || r.DigestMatch == nil || !*r.DigestMatch {
				t.Fatalf("existing operation changed: %+v %s", r, r.WorkerDiag)
			}
			if job.Source != nil && (r.SourceMatch == nil || !*r.SourceMatch) {
				t.Fatalf("source changed: %+v", r)
			}
			if op != "rollup" && (r.Rows != 8 || verifier.Next != 8) {
				t.Fatalf("wrong generated rows: %+v", r)
			}
			if op == "rollup" {
				var result struct {
					ProjectCount int      `json:"project_count"`
					Totals       []string `json:"totals"`
					SPI          float64  `json:"spi"`
					CPI          float64  `json:"cpi"`
				}
				if err := json.Unmarshal([]byte(r.ResultJSON), &result); err != nil {
					t.Fatal(err)
				}
				want := []string{"9007199254740993", "5", "10", "20", "40"}
				if result.ProjectCount != 1 || !reflect.DeepEqual(result.Totals, want) || result.SPI != 0.5 || result.CPI != 2 {
					t.Fatalf("rollup changed: %+v", result)
				}
			}
		})
	}
}
