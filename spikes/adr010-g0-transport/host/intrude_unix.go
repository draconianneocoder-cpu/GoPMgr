// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package main

import "fmt"

// intrude is Windows-only: a Unix-domain socket path that exists cannot be
// bound a second time.
func intrude(string) int {
	fmt.Println("ERROR: not supported on this OS")
	return 2
}
