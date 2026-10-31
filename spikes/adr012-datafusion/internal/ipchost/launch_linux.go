// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"errors"
	"os/exec"
)

func workerCommand(job Job) (*exec.Cmd, error) {
	if job.NoFileWrites {
		return nil, errors.New("DF0 OS write-denial experiment requires native Darwin")
	}
	return exec.Command(job.Worker), nil
}
