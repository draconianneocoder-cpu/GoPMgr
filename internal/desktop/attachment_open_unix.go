// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package desktop

import (
	"os"
	"syscall"
)

// openAttachmentFile opens path for reading without blocking: a plain open
// of a named pipe waits for a writer. O_NONBLOCK has no effect on a regular
// file. The caller requires a regular file and caps the read.
func openAttachmentFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) // #nosec G304 -- path chosen by the user in the native file dialog.
}
