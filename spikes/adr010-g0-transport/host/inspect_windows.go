// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func powershell(script string) ([]byte, error) {
	return run(30, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
}

// leakCheck looks for any secret in the worker's command line. Windows
// gives no supported way to read another process's environment, so that
// half is reported as not checked.
func leakCheck(pid int, secrets [][]byte) (string, bool) {
	out, err := powershell(fmt.Sprintf(`(Get-CimInstance Win32_Process -Filter "ProcessId=%d").CommandLine`, pid))
	if err != nil {
		return "command line unreadable: " + err.Error(), false
	}
	for _, s := range secrets {
		if bytes.Contains(out, s) {
			return "command line (environment not readable)", true
		}
	}
	return "command line (environment not readable)", false
}

// tcpSocketCount counts TCP sockets of any state the worker owns.
func tcpSocketCount(pid int) (int, string, error) {
	count := 0
	want := strconv.Itoa(pid)
	for _, proto := range []string{"TCP", "TCPv6"} {
		out, err := run(30, "netstat", "-ano", "-p", proto)
		if err != nil {
			return 0, "", fmt.Errorf("netstat %s: %w", proto, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 4 && strings.HasPrefix(f[0], "TCP") && f[len(f)-1] == want {
				count++
			}
		}
	}
	return count, "netstat -ano TCP and TCPv6", nil
}

func peakRSS(pid int) (int64, string) {
	out, err := powershell(fmt.Sprintf(`(Get-Process -Id %d).PeakWorkingSet64`, pid))
	if err != nil {
		return -1, "PeakWorkingSet64 unavailable"
	}
	v, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return v, "PeakWorkingSet64"
}

func selfPeakRSS() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

func osVersion() string {
	out, _ := run(10, "cmd", "/c", "ver")
	return strings.TrimSpace(string(out))
}
