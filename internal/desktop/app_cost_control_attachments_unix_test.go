// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SEC-04: a ledger attachment chosen in the file dialog may be a named pipe.
// Opening one with no writer used to block forever, hanging the bound call.
// The reader must refuse it promptly, as the other import readers do.
func TestReadBoundedFileRefusesANamedPipe(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "invoice.pdf")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("cannot create a named pipe here: %v", err)
	}
	t.Cleanup(func() {
		// Release a reader still blocked on the pipe, so a failing run
		// does not leave a goroutine stuck in open(2).
		if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = w.Close()
		}
	})

	done := make(chan error, 1)
	go func() {
		_, err := readBoundedFile(fifo, 1024)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("readBoundedFile on a named pipe returned %v, want a not-a-regular-file error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readBoundedFile blocked on a named pipe")
	}
}

// A directory is refused for the same reason, before any read.
func TestReadBoundedFileRefusesADirectory(t *testing.T) {
	_, err := readBoundedFile(t.TempDir(), 1024)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("readBoundedFile on a directory returned %v, want a not-a-regular-file error", err)
	}
}
