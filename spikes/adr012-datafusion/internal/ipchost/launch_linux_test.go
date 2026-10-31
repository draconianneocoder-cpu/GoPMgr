// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package ipchost

import (
	"strings"
	"testing"
)

func TestUnsupportedWriteDenialNeverLaunchesWorker(t *testing.T) {
	r := Run(Job{Name: "unsupported-denial", Worker: "/not/a/worker", NoFileWrites: true})
	if r.Outcome != "setup_failed" || !strings.Contains(r.Detail, "requires native Darwin") || r.WorkerPID != 0 || !r.EndpointRemoved {
		t.Fatalf("unsupported guard did not fail closed: %+v", r)
	}
}
