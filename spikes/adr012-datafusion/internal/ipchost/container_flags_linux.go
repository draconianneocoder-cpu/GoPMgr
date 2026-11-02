// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import "io/fs"

// clearUserFlags is a no-op on Linux: the App Sandbox container it restores
// exists only on macOS, and Linux file attributes need a separate design.
func clearUserFlags(string, fs.FileInfo) (bool, error) { return false, nil }
