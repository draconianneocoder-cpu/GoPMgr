// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// cleanEnv gives the worker no inherited variables at all, so settings
// such as JAVA_TOOL_OPTIONS, _JAVA_OPTIONS, CLASSPATH or LD_PRELOAD cannot
// reach the JVM (TPT-017). The JVM needs none of them.
func cleanEnv() []string { return []string{} }

// platformProcAttr puts the worker in its own process group so the whole
// tree can be killed (TPT-013).
func platformProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// processTree is the worker's process group. Children inherit it unless
// they deliberately leave (setsid or setpgid); that limit is recorded in
// the G0 report.
type processTree struct {
	pgid int
}

func attachTree(pid int) (*processTree, error) { return &processTree{pgid: pid}, nil }

// kill signals every process left in the group, including grandchildren
// whose parent already died.
func (t *processTree) kill() {
	if t != nil {
		_ = syscall.Kill(-t.pgid, syscall.SIGKILL)
	}
}

// release is kill: a process group needs no handle closed.
func (t *processTree) release() { t.kill() }

// members reports how many live processes remain in the group.
func (t *processTree) members() (int, string) {
	if t == nil {
		return -1, "no process group"
	}
	return groupMembers(t.pgid)
}
