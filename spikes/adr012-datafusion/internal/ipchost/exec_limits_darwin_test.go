// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

// Configuration and measured effects are separate. These are feasibility
// observations, not host-wide RSS, CPU-rate or process-tree qualification.
type execLimitObservation struct {
	Mode         string `json:"mode"`
	Stage        string `json:"stage"`
	RequestedAS  uint64 `json:"requested_as_bytes"`
	InstallErrno int    `json:"install_errno"`
	ReadErrno    int    `json:"read_errno"`
	AS           uint64 `json:"as_soft_bytes"`
	ASHard       uint64 `json:"as_hard_bytes"`
	NProc        uint64 `json:"nproc_soft"`
	NProcHard    uint64 `json:"nproc_hard"`
	VM           uint64 `json:"virtual_bytes"`
	Peak         int64  `json:"peak_rss_bytes"`
	MappingErrno int    `json:"mapping_errno"`
	Checksum     uint64 `json:"checksum"`
	ForkErrno    int    `json:"fork_errno"`
	SpawnErrno   int    `json:"spawn_errno"`
	WaitOK       bool   `json:"wait_ok"`
}

func compileExecLimitProbe(t *testing.T, worker string, asMiB int, denyChildren bool) string {
	t.Helper()
	src, err := filepath.Abs("../../tools/macos_exec_limits_probe.c")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "exec-limit-probe")
	args := []string{"clang", "-std=c11", "-Wall", "-Wextra", "-Werror", src, "-o", out}
	if worker != "" {
		args = append(args, "-DDF0_WORKER="+strconv.Quote(worker), fmt.Sprintf("-DDF0_AS_MIB=%d", asMiB))
		if denyChildren {
			args = append(args, "-DDF0_DENY_CHILDREN=1")
		}
	}
	cmd := exec.Command("xcrun", args...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile exec probe: %v: %s", err, b)
	}
	return out
}

func readExecLimitObservations(t *testing.T, data string) []execLimitObservation {
	t.Helper()
	var observations []execLimitObservation
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		var observed execLimitObservation
		if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Mode == "" || observed.Stage == "" {
			t.Fatalf("invalid probe observation: %v: %s", err, line)
		}
		observations = append(observations, observed)
	}
	return observations
}

func runExecLimitProbe(t *testing.T, probe, mode string) []execLimitObservation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, probe, mode)
	cmd.Env = []string{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec probe %s: %v: %s", mode, err, b)
	}
	return readExecLimitObservations(t, string(b))
}

func saveExecLimitEvidence(t *testing.T, name string, value any) {
	t.Helper()
	if dir := os.Getenv("DF0_EXEC_LIMIT_EVIDENCE"); dir != "" {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMacOSAbsoluteExecLimits(t *testing.T) {
	var before, after unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_AS, &before); err != nil {
		t.Fatal(err)
	}
	probe := compileExecLimitProbe(t, "", 0, false)
	var all [][]execLimitObservation
	for _, tc := range []struct {
		mode  string
		asMiB uint64
	}{{"control", 0}, {"as64", 64}, {"as512", 512}, {"as8192", 8192}, {"as-headroom", 0}} {
		mode := tc.mode
		observations := runExecLimitProbe(t, probe, mode)
		all = append(all, observations)
		first := observations[0]
		if first.Mode != mode || (mode != "as-headroom" && first.RequestedAS != tc.asMiB<<20) {
			t.Fatalf("probe mode mismatch: %+v", observations)
		}
		if mode == "as-headroom" && first.InstallErrno != 0 {
			t.Fatalf("positive AS installation control failed: %+v", observations)
		}
		if first.ReadErrno != 0 || first.VM == 0 || first.Peak <= 0 {
			t.Fatalf("invalid measurement control: %+v", observations)
		}
		if mode == "control" {
			if len(observations) != 1 || first.MappingErrno != 0 || first.Checksum != (128<<20)/4096*0x5a || first.Peak < 128<<20 {
				t.Fatalf("allocation control ineffective: %+v", observations)
			}
		} else if first.InstallErrno != 0 {
			if len(observations) != 1 || first.Stage != "installation" {
				t.Fatalf("installation refusal must not exec: %+v", observations)
			}
		} else {
			if len(observations) != 2 {
				t.Fatalf("missing post-exec observation: %+v", observations)
			}
			for _, o := range observations {
				if o.ReadErrno != 0 || o.AS != first.RequestedAS || o.ASHard != first.RequestedAS {
					t.Fatalf("limit did not survive exec: %+v", observations)
				}
			}
			last := observations[1]
			if mode == "as-headroom" && last.MappingErrno != int(unix.ENOMEM) {
				t.Fatalf("AS denial control failed after exec: %+v", observations)
			}
			if last.Stage != "payload" || (last.MappingErrno != 0 && last.MappingErrno != int(unix.ENOMEM)) {
				t.Fatalf("unexpected allocation outcome: %+v", observations)
			}
			if last.MappingErrno == 0 && (last.Checksum != (128<<20)/4096*0x5a || last.Peak < 128<<20) {
				t.Fatalf("mapping success without resident positive control: %+v", observations)
			}
		}
		t.Logf("%s: %+v", mode, observations)
	}
	if err := unix.Getrlimit(unix.RLIMIT_AS, &after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("parent address-space limit changed: %v -> %v", before, after)
	}
	saveExecLimitEvidence(t, "absolute-exec.json", all)
}

func TestMacOSChildCreationLimit(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("unprivileged NPROC experiment requires non-root")
	}
	var before, after unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NPROC, &before); err != nil {
		t.Fatal(err)
	}
	probe := compileExecLimitProbe(t, "", 0, false)
	control := runExecLimitProbe(t, probe, "spawn-control")
	denied := runExecLimitProbe(t, probe, "spawn-deny")
	if len(control) != 1 || control[0].ForkErrno != 0 || control[0].SpawnErrno != 0 || !control[0].WaitOK {
		t.Fatalf("process-creation positive control failed: %+v", control)
	}
	if len(denied) != 2 || denied[0].Mode != "spawn-deny" || denied[0].Stage != "preexec" || denied[1].Mode != "spawn-attempts" || denied[1].Stage != "attempts" || denied[1].ForkErrno != int(unix.EAGAIN) || denied[1].SpawnErrno != int(unix.EAGAIN) || !denied[1].WaitOK {
		t.Fatalf("direct fork/spawn refusal failed: %+v", denied)
	}
	for _, o := range denied {
		if o.InstallErrno != 0 || o.ReadErrno != 0 || o.NProc != 0 || o.NProcHard != 0 {
			t.Fatalf("process limit did not survive exec: %+v", denied)
		}
	}
	if err := unix.Getrlimit(unix.RLIMIT_NPROC, &after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("parent process limit changed: %v -> %v", before, after)
	}
	t.Logf("direct child creation: control=%+v denied=%+v", control, denied)
	saveExecLimitEvidence(t, "child-creation.json", [][]execLimitObservation{control, denied})
}

func TestMacOSNativeWorkerExecLimits(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to reviewed native worker")
	}
	if !filepath.IsAbs(worker) {
		t.Fatal("DF0_WORKER must be absolute")
	}
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("unprivileged NPROC experiment requires non-root")
	}
	data, err := os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	type observation struct {
		Name    string                 `json:"name"`
		Config  []execLimitObservation `json:"configuration"`
		Outcome string                 `json:"outcome"`
		Rows    uint64                 `json:"rows"`
		Peak    int64                  `json:"peak_rss_bytes"`
		Exit    string                 `json:"worker_exit"`
		Removed bool                   `json:"endpoint_removed"`
	}
	var all []observation
	for _, tc := range []struct {
		name  string
		asMiB int
		deny  bool
	}{{"control", 0, false}, {"nproc", 0, true}, {"as8192", 8192, false}, {"combined", 8192, true}, {"recovery", 0, false}} {
		path := worker
		if tc.name != "control" && tc.name != "recovery" {
			path = compileExecLimitProbe(t, worker, tc.asMiB, tc.deny)
		}
		v := &GenVerifier{Width: 32}
		r := Run(Job{Name: "exec-limits-" + tc.name, Worker: path, Timeout: 10 * time.Second, Verify: v.Check, Execute: &df0pb.Execute{Operation: "generate", Rows: 128, BatchRows: 32, PayloadBytes: 32}})
		o := observation{Name: tc.name, Outcome: r.Outcome, Rows: r.Rows, Peak: r.WorkerPeakRSS, Exit: r.WorkerExit, Removed: r.EndpointRemoved}
		setupRefused := false
		if path != worker {
			// Only the launcher's first line is structured probe evidence;
			// later Rust diagnostics are not exposed in the public record.
			firstLine, _, _ := strings.Cut(strings.TrimSpace(r.WorkerDiag), "\n")
			o.Config = readExecLimitObservations(t, firstLine)
			cfg := o.Config[0]
			if cfg.Mode != "worker" || cfg.ReadErrno != 0 || cfg.RequestedAS != uint64(tc.asMiB)<<20 {
				t.Fatalf("unexpected launcher observation: %+v", cfg)
			}
			if cfg.InstallErrno != 0 {
				if tc.asMiB == 0 || cfg.Stage != "installation" || r.Outcome != "worker_exit_before_stream" || r.WorkerExit != "exit 78" || r.Rows != 0 {
					t.Fatalf("setup refusal misclassified: %+v configuration=%+v", r, cfg)
				}
				setupRefused = true
			} else {
				if cfg.Stage != "preexec" || (tc.asMiB != 0 && (cfg.AS != cfg.RequestedAS || cfg.ASHard != cfg.RequestedAS)) || (tc.deny && (cfg.NProc != 0 || cfg.NProcHard != 0)) {
					t.Fatalf("limits not installed before Rust exec: %+v", cfg)
				}
			}
		}
		if !r.EndpointRemoved || r.Killed {
			t.Fatalf("unbounded or incomplete cleanup: %+v", r)
		}
		if r.Outcome == "completed" {
			if len(r.WorkerEnvKeys) != r.WorkerEnvNames || len(r.WorkerEnvKeys) > 1 || (len(r.WorkerEnvKeys) == 1 && r.WorkerEnvKeys[0] != "__CF_USER_TEXT_ENCODING") {
				t.Fatalf("unexpected worker environment: %v", r.WorkerEnvKeys)
			}
			if r.Rows != 128 || r.DigestMatch == nil || !*r.DigestMatch || r.WorkerExit != "exit 0" || r.WorkerPeakRSS <= 0 {
				t.Fatalf("native result/control invalid: %+v", r)
			}
		} else if !setupRefused {
			t.Fatalf("native positive control failed: %+v", r)
		}
		all = append(all, o)
		t.Logf("%s outcome=%s rows=%d peak=%d exit=%s removed=%v configuration=%+v", tc.name, r.Outcome, r.Rows, r.WorkerPeakRSS, r.WorkerExit, r.EndpointRemoved, o.Config)
	}
	saveExecLimitEvidence(t, "native-worker.json", struct {
		WorkerSHA256 string        `json:"worker_sha256"`
		Results      []observation `json:"results"`
	}{hash, all})
}
