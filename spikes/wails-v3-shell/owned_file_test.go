// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
)

func TestOwnedFileRejectsAdmissionAndDrainsBeforeClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f, err := os.Create(filepath.Join(t.TempDir(), "resource"))
		if err != nil {
			t.Fatal(err)
		}
		owner := newOwnedFile(f)
		release, err := owner.begin()
		if err != nil {
			t.Fatal(err)
		}
		released := false
		t.Cleanup(func() {
			if !released {
				release()
			}
			_ = owner.Close()
		})
		done := make(chan error, 1)
		go func() { done <- owner.Close() }()
		synctest.Wait()
		<-owner.stopping
		if _, err := owner.begin(); err == nil {
			t.Error("admitted operation during shutdown")
		}
		select {
		case err := <-done:
			t.Fatalf("closed before active operation finished: %v", err)
		default:
		}
		if _, err := f.WriteString("operation finished"); err != nil {
			t.Fatal(err)
		}
		release()
		released = true
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("late"); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("file remains usable: %v", err)
		}
		if err := owner.Close(); err != nil {
			t.Fatalf("second close: %v", err)
		}
	})
}
