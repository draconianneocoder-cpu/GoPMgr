// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package analytics

import (
	"path/filepath"
	"syscall"
	"testing"
)

func makeFIFO(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pipe.csv")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
