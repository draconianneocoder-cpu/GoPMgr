// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"io/fs"
	"syscall"

	"golang.org/x/sys/unix"
)

// clearUserFlags removes the immutable and append-only flags the owner may
// set, so a host-owned entry can be restored or removed. The caller has
// already established that path is not a symlink.
func clearUserFlags(path string, fi fs.FileInfo) (bool, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Flags&(unix.UF_IMMUTABLE|unix.UF_APPEND) == 0 {
		return false, nil
	}
	return true, unix.Chflags(path, int(st.Flags&^(unix.UF_IMMUTABLE|unix.UF_APPEND)))
}
