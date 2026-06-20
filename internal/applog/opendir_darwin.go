// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package applog

import "os/exec"

// OpenFolder opens path in the macOS Finder. The path is passed as a
// discrete argv value, never interpolated into a shell string. Returns
// any exec error so the caller can surface it to the user.
func OpenFolder(path string) error {
	return exec.Command("/usr/bin/open", path).Run() // #nosec G204 -- fixed binary; path passed as argv.
}
