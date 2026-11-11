// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package db

import (
	"os"
	"syscall"
)

// openBackupCert opens path for reading without blocking: a plain open of a
// FIFO waits for a writer. O_NONBLOCK has no effect on a regular file.
func openBackupCert(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) // #nosec G304 -- extension-checked certificate path; the caller requires a regular file and caps the read.
}
