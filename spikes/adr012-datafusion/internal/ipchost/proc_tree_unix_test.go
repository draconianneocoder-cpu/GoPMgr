// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

const (
	treeFixtureFlag = "-df0-tree-fixture"
	treeSleeperFlag = "-df0-tree-sleeper"
)

// treeFixture lets the test binary stand in for a worker that creates a
// process and then either hangs or exits. It writes the child's PID to
// pidFile. Both TestMain functions call it.
func treeFixture(args []string) (int, bool) {
	if len(args) == 2 && args[1] == treeSleeperFlag {
		time.Sleep(time.Minute)
		return 0, true
	}
	if len(args) != 4 || args[1] != treeFixtureFlag {
		return 0, false
	}
	mode, pidFile := args[2], args[3]
	child := exec.Command(args[0], treeSleeperFlag)
	// The child holds none of the fixture's pipes, so only its own life is
	// under test.
	child.Stdin, child.Stdout, child.Stderr = nil, nil, nil
	if strings.Contains(mode, "pipe") {
		child.Stdout = os.Stdout // the worker's diagnostics pipe
	}
	if strings.HasPrefix(mode, "setsid") {
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if child.Start() != nil {
		return 20, true
	}
	if os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600) != nil {
		return 21, true
	}
	if strings.HasSuffix(mode, "hang") {
		time.Sleep(time.Minute)
	}
	return 0, true
}

// runTreeFixture runs one fixture mode as the worker and returns the result
// and the PID of the process the fixture created.
func runTreeFixture(t *testing.T, mode string) (Result, int) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "w.sh")
	body := "#!/bin/sh\nexec '" + self + "' " + treeFixtureFlag + " " + mode + " '" + pidFile + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	res := Run(Job{Name: mode, Worker: script, Timeout: 3 * time.Second,
		Execute: &df0pb.Execute{Operation: "generate"}})
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("fixture wrote no child PID (%s %s): %v", res.Outcome, res.WorkerDiag, err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil || pid <= 1 {
		t.Fatalf("child PID %q", raw)
	}
	// Never leave the sleeper behind, whatever the outcome.
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return res, pid
}

// gone waits for pid to stop existing. A killed child becomes a zombie
// until init reaps it, so this polls.
func gone(pid int) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// SEC-002: a process the worker creates must not outlive the job.
func TestTeardownKillsWhatTheWorkerCreated(t *testing.T) {
	t.Run("child that left the group, worker still running", func(t *testing.T) {
		res, pid := runTreeFixture(t, "setsid_hang")
		if res.Outcome != "worker_forked" || res.DescendantsKilled < 1 {
			t.Fatalf("outcome %q (%s), descendants killed %d", res.Outcome, res.Detail, res.DescendantsKilled)
		}
		if !gone(pid) {
			t.Fatal("the child that left the worker's group survived teardown")
		}
	})

	t.Run("child in the group, worker exits on its own", func(t *testing.T) {
		res, pid := runTreeFixture(t, "group_exit")
		if !gone(pid) {
			t.Fatalf("the child left in the worker's group survived (%s)", res.Outcome)
		}
		if runtime.GOOS == "darwin" && (res.Outcome != "worker_forked" || !res.WorkerForked) {
			t.Fatalf("outcome %q, forked %v: the fork was not reported", res.Outcome, res.WorkerForked)
		}
	})

	t.Run("child that left the group, worker exits on its own", func(t *testing.T) {
		res, _ := runTreeFixture(t, "setsid_exit")
		// The child was reparented to init before teardown, so no walk can
		// find it. macOS still reports the fork; Linux cannot.
		if runtime.GOOS != "darwin" {
			t.Skip("Linux has no unprivileged fork notification; documented residual")
		}
		if res.Outcome != "worker_forked" || !res.WorkerForked {
			t.Fatalf("outcome %q, forked %v: the fork was not reported", res.Outcome, res.WorkerForked)
		}
	})

	t.Run("survivor holding the output pipe does not hold Run", func(t *testing.T) {
		began := time.Now()
		res, _ := runTreeFixture(t, "setsid_pipe_exit")
		if took := time.Since(began); took > 20*time.Second {
			t.Fatalf("Run took %v after the worker exited (%s)", took, res.Outcome)
		}
	})
}

// A worker can exit before the host starts watching it, for example when
// it fails to load. The watch must then report the exit, not fail setup.
func TestExitWatchOnAWorkerThatAlreadyExited(t *testing.T) {
	cmd := exec.Command("/usr/bin/true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		procs, err := processTable()
		if err != nil {
			t.Fatal(err)
		}
		if e, ok := procs[pid]; ok && e.held {
			break // a zombie: exited, not reaped
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not become a zombie")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w, err := newExitWatch(pid)
	if err != nil {
		t.Fatalf("watch on an exited worker: %v", err)
	}
	done := make(chan bool, 1)
	go func() { _, ok := w.wait(); done <- ok }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("wait reported a failed watch")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("wait did not report the exit")
	}
}
