// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// plainRenameReplacesEmptyDir calls rename(2) directly, as Go's os.Rename
// does after its Lstat check, to show whether an empty target folder is
// silently replaced.
func plainRenameReplacesEmptyDir(src, dst string) (bool, string) {
	if err := syscall.Rename(src, dst); err != nil {
		return false, "rename(2) refused: " + err.Error()
	}
	if _, err := os.Stat(filepath.Join(dst, "project.gopmgr")); err == nil {
		return true, "rename(2) replaced the empty folder"
	}
	return false, "rename(2) returned success but the file is missing"
}

func lockConflicts(path string) (bool, string) {
	f1, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err.Error()
	}
	defer f1.Close()
	f2, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return false, err.Error()
	}
	defer f2.Close()
	if err := unix.Flock(int(f1.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return false, "first flock: " + err.Error()
	}
	err = unix.Flock(int(f2.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return true, "flock: second exclusive lock refused"
	}
	return false, "flock: second exclusive lock " + errText(err)
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
