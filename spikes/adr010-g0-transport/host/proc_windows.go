// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"os"
	"os/exec"
)

// cleanEnv keeps only what the JVM needs to start on Windows. SystemRoot
// is required by Winsock; TEMP and TMP by the JVM's temporary files.
// JAVA_TOOL_OPTIONS, _JAVA_OPTIONS and CLASSPATH never pass (TPT-017).
func cleanEnv() []string {
	var env []string
	for _, k := range []string{"SystemRoot", "TEMP", "TMP"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// platformProcAttr is empty in the spike. A production supervisor would
// assign the worker to a Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// so the whole tree dies with the host; the JVM here starts no children.
func platformProcAttr(*exec.Cmd) {}

func killTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
