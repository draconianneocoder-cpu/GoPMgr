// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func renameNoReplace(src, dst string) error {
	return unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
}

var fsMagic = map[int64]string{
	0xEF53: "ext2/3/4", 0x794c7630: "overlayfs", 0x01021994: "tmpfs", 0x58465342: "xfs",
	0x9123683E: "btrfs", 0x6969: "nfs", 0x65735546: "fuse", 0xFF534D42: "cifs", 0x2FC12FC1: "zfs",
}

func fsType(dir string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return "unknown: " + err.Error()
	}
	name, ok := fsMagic[int64(st.Type)]
	if !ok {
		name = "other"
	}
	return fmt.Sprintf("%s (magic 0x%x)", name, st.Type)
}

// syncHint names a sync client the path is inside, by convention only.
// INT-03: this is a hint, never a reliable detector.
func syncHint(path string) string {
	home, _ := os.UserHomeDir()
	for _, d := range []string{"Dropbox", "OneDrive", "Nextcloud", "pCloudDrive", "MEGA"} {
		if strings.HasPrefix(path, home+"/"+d) {
			return "inside ~/" + d
		}
	}
	return "none recognized"
}
