// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import "syscall"

// wait4 reports ru_maxrss in KiB on Linux.
const rusageRSSScale int64 = 1024

// sysProcAttr gives the host a worker process group to signal. Members can
// leave it; this is not process-tree containment. Pdeathsig covers the direct worker.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
