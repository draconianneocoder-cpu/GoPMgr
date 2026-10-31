// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import "os/exec"

// Experimental DF0 control, not the production macOS containment architecture.
const denyFileWrites = `(version 1)(allow default)(deny file-write*)`

func workerCommand(job Job) (*exec.Cmd, error) {
	if job.NoFileWrites {
		return exec.Command("/usr/bin/sandbox-exec", "-p", denyFileWrites, job.Worker), nil
	}
	return exec.Command(job.Worker), nil
}
