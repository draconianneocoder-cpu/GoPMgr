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

func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// sleeper is a short-lived process whose command line ends with a secret.
var sleeper = []string{"sh", "-c", "sleep 5", "g0-self-check"}
