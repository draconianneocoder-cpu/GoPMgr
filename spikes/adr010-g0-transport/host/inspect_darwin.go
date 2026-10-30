// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// The process checks below ask the kernel through sysctl rather than
// running ps. ps is setuid root on macOS, and a sandboxed process (the G0
// offline run uses sandbox-exec) may not run setuid programs.

// szomb is a zombie's p_stat value (sys/proc.h).
const szomb = 5

// leakCheck looks for any secret in the worker's arguments and
// environment, read through sysctl kern.procargs2, the same source ps -E
// uses (TPT-007). An error means the check did not run.
func leakCheck(pid int, secrets [][]byte) (string, bool, error) {
	const how = "sysctl kern.procargs2 (arguments and environment)"
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return how, false, fmt.Errorf("read the arguments of pid %d: %w", pid, err)
	}
	if len(buf) < 4 {
		return how, false, fmt.Errorf("kern.procargs2 returned %d bytes for pid %d", len(buf), pid)
	}
	for _, s := range secrets {
		if bytes.Contains(buf, s) {
			return how, true, nil
		}
	}
	return how, false, nil
}

// tcpSocketCount counts TCP sockets of any state the worker owns. lsof
// exits 1 with no output when nothing matches; any other failure means the
// check did not run.
func tcpSocketCount(pid int) (int, string, error) {
	const how = "lsof -iTCP for the worker pid"
	out, err := run(10, "lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP")
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(bytes.TrimSpace(out)) != 0 {
			return 0, how, fmt.Errorf("lsof: %w: %s", err, bytes.TrimSpace(out))
		}
	}
	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" && !strings.HasPrefix(l, "COMMAND") {
			lines++
		}
	}
	return lines, how, nil
}

// peakRSS samples current resident memory; macOS ps has no peak.
func peakRSS(pid int) (int64, string) {
	out, err := run(10, "ps", "-o", "rss=", "-p", strconv.Itoa(pid))
	if err != nil {
		return -1, "ps rss unavailable"
	}
	kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb * 1024, "ps rss (sampled, not peak)"
}

func selfPeakRSS() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

func osVersion() string {
	v, _ := run(10, "sw_vers", "-productVersion")
	return "macOS " + strings.TrimSpace(string(v))
}

// groupMembers counts live (non-zombie) processes in a process group.
func groupMembers(pgid int) (int, string) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pgid)
	if err != nil {
		return -1, "sysctl kern.proc.pgrp failed: " + err.Error()
	}
	n := 0
	for _, p := range procs {
		if p.Proc.P_stat != szomb {
			n++
		}
	}
	return n, "sysctl kern.proc.pgrp, zombies excluded"
}

// processAlive reports whether pid is a live (non-zombie) process. When
// the process exists but its state cannot be read, it counts as alive, so
// a "the process is gone" check fails rather than passing by default.
func processAlive(pid int) bool {
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return false
	}
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return true
	}
	return p.Proc.P_pid != int32(pid) || p.Proc.P_stat != szomb
}
