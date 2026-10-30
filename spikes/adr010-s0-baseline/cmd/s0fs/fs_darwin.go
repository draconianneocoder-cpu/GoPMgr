// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func renameNoReplace(src, dst string) error {
	return unix.RenamexNp(src, dst, unix.RENAME_EXCL)
}

func fsType(dir string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return "unknown: " + err.Error()
	}
	b := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

// syncHint names a sync location the path is inside, by convention only.
// INT-03: this is a hint, never a reliable detector.
func syncHint(path string) string {
	home, _ := os.UserHomeDir()
	for _, d := range []string{"Library/Mobile Documents", "Library/CloudStorage", "Dropbox"} {
		if strings.HasPrefix(path, filepath.Join(home, d)) {
			return "inside ~/" + d
		}
	}
	return "none recognized"
}
