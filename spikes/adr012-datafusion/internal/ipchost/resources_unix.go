// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"math"
	"os"
	"syscall"
)

func rusagePeakBytes(raw int64) int64 {
	if raw <= 0 || raw > math.MaxInt64/rusageRSSScale {
		return -1
	}
	return raw * rusageRSSScale
}

// Called only after Wait completes. Per-child kernel accounting survives early
// failure and SIGKILL, and avoids trusting the worker or looking up a reused PID.
// This is lifetime child usage, not simultaneous host-wide or process-tree RSS.
func recordProcessResources(res *Result, state *os.ProcessState) {
	res.WorkerPeakRSS, res.WorkerUserNS, res.WorkerSystemNS = -1, -1, -1
	res.WorkerResourceMethod = ""
	if state == nil {
		return
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage == nil {
		return
	}
	res.WorkerPeakRSS = rusagePeakBytes(usage.Maxrss)
	res.WorkerUserNS = state.UserTime().Nanoseconds()
	res.WorkerSystemNS = state.SystemTime().Nanoseconds()
	res.WorkerResourceMethod = "host wait4 child rusage"
}
