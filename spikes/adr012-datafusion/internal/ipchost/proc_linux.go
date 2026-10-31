// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import "syscall"

// wait4 reports ru_maxrss in KiB on Linux.
const rusageRSSScale int64 = 1024

// sysProcAttr puts the worker in its own process group, so the host can
// kill the whole tree, and has the kernel kill it if the host dies.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
