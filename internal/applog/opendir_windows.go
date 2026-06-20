// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package applog

import "os/exec"

// OpenFolder opens path in Windows Explorer. The path is passed as a
// discrete argv value, never interpolated into a command string.
func OpenFolder(path string) error {
	return exec.Command("explorer", path).Run() // #nosec G204 -- fixed binary; path passed as argv.
}
