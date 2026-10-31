// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import "syscall"

// wait4 reports ru_maxrss in bytes on Darwin.
const rusageRSSScale int64 = 1

// sysProcAttr gives the host a worker process group to signal. Members can
// leave it; this is not process-tree containment. macOS has no parent-death signal.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
