// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package db

import "os"

func openBackupCert(path string) (*os.File, error) {
	return os.Open(path) // #nosec G304 -- extension-checked certificate path; the caller requires a regular file and caps the read.
}
