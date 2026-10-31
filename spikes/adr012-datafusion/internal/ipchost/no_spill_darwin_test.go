// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

// Re-exec only the permission probe; normal tests remain owned by testing.M.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "-df0-write-probe" {
		err := os.WriteFile(os.Args[2], []byte(fixtures.NoSpillSentinel), 0600)
		if err == nil {
			os.Exit(0)
		}
		if errors.Is(err, os.ErrPermission) {
			os.Exit(41)
		}
		os.Exit(42)
	}
	os.Exit(m.Run())
}

func TestNativeWriteDenialControl(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, "probe")
	run := func(path string, deny bool) error {
		cmd, err := workerCommand(Job{Worker: exe, NoFileWrites: deny})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		args := append(cmd.Args[1:], "-df0-write-probe", path)
		probe := exec.CommandContext(ctx, cmd.Path, args...)
		probe.Env = []string{}
		return probe.Run()
	}
	if err := run(target, false); err != nil {
		t.Fatalf("positive write control: %v", err)
	}
	original, err := os.ReadFile(target)
	if err != nil || !bytes.Contains(original, []byte(fixtures.NoSpillSentinel)) {
		t.Fatalf("sentinel detector control: %v", err)
	}
	for _, path := range []string{filepath.Join(root, "new"), target} {
		err := run(path, true)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 41 {
			t.Fatalf("write denial not enforced: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied create persisted: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("denied overwrite changed file: %v", err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	err = run(alias, true)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 41 {
		t.Fatalf("alias bypassed denial: %v", err)
	}
}

func sentinelInFiles(root string) (bool, int, error) {
	found, count := false, 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// The control removes temporary children while results drain.
			// Keep root/permission errors visible; a vanished child is not a file.
			if path != root && errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		count++
		found = found || bytes.Contains(data, []byte(fixtures.NoSpillSentinel))
		return nil
	})
	return found, count, err
}

func TestNativeNoSpillPressure(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the fresh native worker")
	}
	var buf bytes.Buffer
	if err := fixtures.WriteNoSpillParquet(&buf); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "selected.parquet")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DF0_PARENT_SENTINEL", fixtures.NoSpillSentinel)
	type observation struct {
		Result
		WriteDenialRequested    bool   `json:"write_denial_requested"`
		SentinelObservedInSpill bool   `json:"sentinel_observed_in_spill"`
		ScratchFilesAfter       int    `json:"scratch_files_after"`
		QueryMemoryBytes        uint64 `json:"query_memory_bytes"`
	}
	var evidence []observation
	for _, mode := range []string{"spill_control", "unauthorized_spill", "memory_only", "os_denial", "normal_guarded", "cancel_guarded", "kill_guarded", "recovery_guarded"} {
		t.Run(mode, func(t *testing.T) {
			snap, err := source.Capture(path)
			if err != nil {
				t.Fatal(err)
			}
			scratch := t.TempDir()
			seenSpill := false
			ex := &df0pb.Execute{Operation: "parquet_range", BatchRows: 64, SortSource: true, QueryMemoryBytes: 1 << 20}
			job := Job{Name: mode, Worker: worker, ReadSource: snap, Execute: ex, Timeout: 30 * time.Second}
			if mode == "spill_control" || mode == "os_denial" {
				ex.SpillControlDirectory = scratch
				ex.Fault = "source_spill_control"
			}
			if mode == "unauthorized_spill" {
				ex.SpillControlDirectory = scratch
			}
			if mode != "spill_control" && mode != "unauthorized_spill" {
				job.NoFileWrites = true
			}
			if mode == "normal_guarded" || mode == "cancel_guarded" || mode == "recovery_guarded" {
				ex.QueryMemoryBytes = 64 << 20
			}
			if mode == "kill_guarded" {
				job.KillAfterRead = 1
			}
			if mode == "cancel_guarded" {
				job.CancelAfter = 1
			}
			next := fixtures.NoSpillRows - 1
			job.Verify = func(data []byte) (int, error) {
				if mode == "spill_control" {
					found, _, err := sentinelInFiles(scratch)
					if err != nil {
						return 0, err
					}
					seenSpill = seenSpill || found
				}
				rd, err := ipc.NewReader(bytes.NewReader(data))
				if err != nil {
					return 0, err
				}
				defer rd.Release()
				rows := 0
				for rd.Next() {
					rec := rd.RecordBatch()
					if rec.NumCols() != 2 {
						return 0, errors.New("wrong schema")
					}
					ids, ok := rec.Column(0).(*array.Int64)
					if !ok {
						return 0, errors.New("id lost int64 type")
					}
					payload, ok := rec.Column(1).(array.StringLike)
					if !ok {
						return 0, errors.New("payload lost string type")
					}
					for i := 0; i < ids.Len(); i++ {
						if ids.IsNull(i) || payload.IsNull(i) || ids.Value(i) != fixtures.ReadAtFirstID+int64(next) || !strings.Contains(payload.Value(i), fixtures.NoSpillSentinel) {
							return 0, errors.New("source sort oracle mismatch")
						}
						next--
						rows++
					}
				}
				return rows, rd.Err()
			}
			r := Run(job)
			if r.WorkerPeakRSS <= 0 || r.WorkerUserNS < 0 || r.WorkerSystemNS < 0 || r.WorkerResourceMethod != "host wait4 child rusage" {
				t.Fatalf("missing post-reap child accounting: %+v", r)
			}
			if !r.EndpointRemoved || r.WorkerExit == "" {
				t.Fatalf("cleanup failed: %+v", r)
			}
			if len(r.WorkerEnvKeys) != r.WorkerEnvNames || len(r.WorkerEnvKeys) > 1 || (len(r.WorkerEnvKeys) == 1 && r.WorkerEnvKeys[0] != "__CF_USER_TEXT_ENCODING") {
				t.Fatalf("unexpected worker environment names: %v", r.WorkerEnvKeys)
			}
			if strings.Contains(r.Detail+r.WorkerDiag, fixtures.NoSpillSentinel) {
				t.Fatalf("sentinel escaped in diagnostics")
			}
			switch mode {
			case "spill_control":
				var metrics struct {
					SpillCount   uint64 `json:"spill_count"`
					SpilledBytes uint64 `json:"spilled_bytes"`
				}
				if err := json.Unmarshal([]byte(r.ResultJSON), &metrics); err != nil {
					t.Fatalf("control metrics unavailable: %s/%s/%s", r.Outcome, r.FailedCode, r.Detail)
				}
				if r.Outcome != "completed" || r.Rows != fixtures.NoSpillRows || metrics.SpillCount == 0 || metrics.SpilledBytes == 0 || !seenSpill {
					t.Fatalf("positive control did not demonstrate plaintext spill: %+v metrics=%+v seen=%v diag=%s", r, metrics, seenSpill, r.WorkerDiag)
				}
			case "unauthorized_spill":
				if r.Outcome != "failed" || r.FailedCode != "INPUT_INVALID" || r.Rows != 0 {
					t.Fatalf("spill opt-in bypassed: %+v", r)
				}
			case "memory_only", "os_denial":
				if r.Outcome != "failed" || r.FailedCode != "RESOURCE_LIMIT" || r.Rows != 0 {
					t.Fatalf("pressure did not fail closed: %+v diag=%s", r, r.WorkerDiag)
				}
			case "kill_guarded":
				if r.Outcome != "fault_killed" || !r.Killed || r.Rows != 0 {
					t.Fatalf("kill cleanup failed: %+v", r)
				}
			case "cancel_guarded":
				if r.Outcome != "failed" || r.FailedCode != "USER_CANCELLED" || r.CancelToExitNS < 0 {
					t.Fatalf("cancel failed: %+v", r)
				}
			default:
				if r.Outcome != "completed" || r.Rows != fixtures.NoSpillRows || next != -1 {
					t.Fatalf("query failed: %+v diag=%s", r, r.WorkerDiag)
				}
			}
			found, count, err := sentinelInFiles(scratch)
			evidence = append(evidence, observation{Result: r, WriteDenialRequested: job.NoFileWrites, SentinelObservedInSpill: seenSpill, ScratchFilesAfter: count, QueryMemoryBytes: ex.QueryMemoryBytes})
			if err != nil || found || count != 0 {
				t.Fatalf("scratch not clean: files=%d sentinel=%v err=%v", count, found, err)
			}
		})
	}
	if out := os.Getenv("DF0_NO_SPILL_EVIDENCE"); out != "" {
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
