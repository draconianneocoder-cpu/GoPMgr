// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package desktop

import "os"

// openAttachmentFile opens path for reading. The caller requires a regular
// file and caps the read.
func openAttachmentFile(path string) (*os.File, error) {
	return os.Open(path) // #nosec G304 -- path chosen by the user in the native file dialog.
}
