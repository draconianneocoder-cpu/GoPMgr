// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package db

import "golang.org/x/sys/unix"

// RENAME_EXCL leaves a concurrently created destination untouched.
func publishRestoredFile(source, destination string) error {
	return unix.RenamexNp(source, destination, unix.RENAME_EXCL)
}
