// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package db

import "os"

// replaceExistingFile atomically replaces destination with source when both
// files are siblings on a POSIX filesystem. Migration always creates source
// beside destination, so this is a one-filesystem rename.
func replaceExistingFile(source, destination string) error {
	return os.Rename(source, destination)
}
