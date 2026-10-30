// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"errors"
	"syscall"
)

// InspectMethod says process inspection is not implemented on macOS, so
// every row that needs it is BLOCKED, never PASS.
const InspectMethod = "unavailable on darwin in this spike"

var errInspect = errors.New("process inspection not implemented on darwin")

type Sockets struct {
	Unix      int      `json:"unix"`
	TCP       int      `json:"tcp"`
	TCPListen int      `json:"tcp_listen"`
	UDP       int      `json:"udp"`
	Other     int      `json:"other"`
	Absent    []string `json:"absent_tables,omitempty"`
}

func InspectSockets(int) (Sockets, error)         { return Sockets{}, errInspect }
func SecretsVisible(int, ...[]byte) (bool, error) { return false, errInspect }
func FDCount(int) int                             { return -1 }
func Threads(int) int64                           { return -1 }
func PeakRSS(int) int64                           { return -1 }

// Alive cannot tell a zombie from a live process on darwin here.
func Alive(pid int) bool { return syscall.Kill(pid, 0) == nil }
