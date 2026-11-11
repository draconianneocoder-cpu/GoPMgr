// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package db

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Opening a FIFO for reading blocks until a writer appears, so a plain open
// would hang Secure Archive forever.
func TestCreateArchivalBundleRefusesAFIFOWithoutBlocking(t *testing.T) {
	d := newBackupTestDB(t)
	dir := t.TempDir()
	fifo := filepath.Join(dir, "signer.p12")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	destPath := filepath.Join(dir, "fifo.pmba")

	done := make(chan error, 1)
	go func() { done <- d.CreateArchivalBundle(destPath, []string{fifo}) }()

	select {
	case err := <-done:
		requireCertRefusal(t, err, "is not a regular file")
	case <-time.After(10 * time.Second):
		// Release every blocked open, however many there are, so the
		// goroutine and the database can be cleaned up, then fail.
		for released := false; !released; {
			if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				_ = w.Close()
			}
			select {
			case <-done:
				released = true
			case <-time.After(50 * time.Millisecond):
			}
		}
		t.Fatal("CreateArchivalBundle blocked on a FIFO")
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("archive published after a refused certificate: stat err=%v", statErr)
	}
}

// A device that never ends, such as /dev/zero, must not be streamed into the
// archive until the disk fills.
func TestCreateArchivalBundleRefusesADeviceFile(t *testing.T) {
	link := filepath.Join(t.TempDir(), "signer.p12")
	if err := os.Symlink("/dev/zero", link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	requireCertRefused(t, link, "is not a regular file")
}
