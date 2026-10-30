// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// intrude tries, as a separate same-user process, to add a server instance
// to an existing pipe name. It uses no FILE_FLAG_FIRST_PIPE_INSTANCE, as an
// intruder would not. It prints CREATED or REFUSED and the reason.
func intrude(name string) int {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		fmt.Println("ERROR:", err)
		return 2
	}
	h, err := windows.CreateNamedPipe(p, windows.PIPE_ACCESS_DUPLEX, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES, 4096, 4096, 0, nil)
	if err != nil {
		fmt.Println("REFUSED:", err)
		return 1
	}
	fmt.Println("CREATED")
	_ = windows.CloseHandle(h)
	return 0
}
