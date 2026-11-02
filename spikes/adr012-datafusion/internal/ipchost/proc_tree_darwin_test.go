// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"os"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

// SEC-002 on the real worker: it gives up the right to create processes
// before reading input, so a spawn attempt (posix_spawn, as Rust's Command
// uses on macOS) is refused and the job completes.
func TestNativeWorkerCannotCreateProcesses(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the fresh native worker")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores RLIMIT_NPROC")
	}
	v := &GenVerifier{Width: 32}
	res := Run(Job{Name: "spawn_child", Worker: worker, Timeout: 30 * time.Second, Verify: v.Check,
		Execute: &df0pb.Execute{Operation: "generate", Rows: 128, BatchRows: 32, PayloadBytes: 32, Fault: "spawn_child"}})
	// Kill a child the worker should never have been able to start.
	if m := regexp.MustCompile(`spawned pid (\d+)`).FindStringSubmatch(res.WorkerDiag); m != nil {
		if pid, err := strconv.Atoi(m[1]); err == nil && pid > 1 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	if !regexp.MustCompile(`spawn fault: refused: .*\(os error 35\)`).MatchString(res.WorkerDiag) {
		t.Fatalf("spawn was not refused with EAGAIN: %q (outcome %s, forked %v)", res.WorkerDiag, res.Outcome, res.WorkerForked)
	}
	if res.Outcome != "completed" || res.WorkerForked || res.Rows != 128 {
		t.Fatalf("outcome %q (%s), forked %v, rows %d", res.Outcome, res.Detail, res.WorkerForked, res.Rows)
	}
}
