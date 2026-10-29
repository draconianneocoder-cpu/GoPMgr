// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// renameNoReplace calls MoveFileEx without MOVEFILE_REPLACE_EXISTING.
func renameNoReplace(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, 0)
}

// plainRenameReplacesEmptyDir uses os.Rename, which passes
// MOVEFILE_REPLACE_EXISTING on Windows.
func plainRenameReplacesEmptyDir(src, dst string) (bool, string) {
	if err := os.Rename(src, dst); err != nil {
		return false, "os.Rename refused: " + err.Error()
	}
	if _, err := os.Stat(filepath.Join(dst, "project.gopmgr")); err == nil {
		return true, "os.Rename replaced the empty folder"
	}
	return false, "os.Rename returned success but the file is missing"
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
	ol := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(windows.Handle(f1.Fd()), flags, 0, 1, 0, ol); err != nil {
		return false, "first LockFileEx: " + err.Error()
	}
	err = windows.LockFileEx(windows.Handle(f2.Fd()), flags, 0, 1, 0, new(windows.Overlapped))
	if err != nil {
		return true, "LockFileEx: second exclusive lock refused: " + err.Error()
	}
	return false, "LockFileEx: second exclusive lock granted"
}

// syncDir: Windows has no directory fsync; FlushFileBuffers on a
// directory handle is not supported.
func syncDir(string) error { return errUnsupported }

func fsType(dir string) string {
	root := filepath.VolumeName(dir) + `\`
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "unknown"
	}
	name := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(p, nil, 0, nil, nil, nil, &name[0], uint32(len(name))); err != nil {
		return "unknown: " + err.Error()
	}
	return windows.UTF16ToString(name)
}

// syncHint uses the OneDrive environment variables, by convention only.
// INT-03: this is a hint, never a reliable detector.
func syncHint(path string) string {
	for _, k := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
		if v := os.Getenv(k); v != "" && strings.HasPrefix(strings.ToLower(path), strings.ToLower(v)) {
			return "inside " + k
		}
	}
	return "none recognized"
}
