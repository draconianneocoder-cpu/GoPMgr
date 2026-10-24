// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package main

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
)

// leakCheck looks for any secret in the worker's command line and
// environment, as ps reports them (TPT-007).
func leakCheck(pid int, secrets [][]byte) (string, bool) {
	out, err := run(10, "ps", "-E", "-ww", "-o", "command=", "-p", strconv.Itoa(pid))
	if err != nil {
		return "ps failed: " + err.Error(), false
	}
	for _, s := range secrets {
		if bytes.Contains(out, s) {
			return "ps -E command and environment", true
		}
	}
	return "ps -E command and environment", false
}

// tcpSocketCount counts TCP sockets of any state the worker owns.
func tcpSocketCount(pid int) (int, string, error) {
	out, _ := run(10, "lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP")
	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" && !strings.HasPrefix(l, "COMMAND") {
			lines++
		}
	}
	return lines, "lsof -iTCP for the worker pid", nil
}

// peakRSS samples current resident memory; macOS ps has no peak.
func peakRSS(pid int) (int64, string) {
	out, err := run(10, "ps", "-o", "rss=", "-p", strconv.Itoa(pid))
	if err != nil {
		return -1, "ps rss unavailable"
	}
	kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb * 1024, "ps rss (sampled, not peak)"
}

func selfPeakRSS() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

func osVersion() string {
	v, _ := run(10, "sw_vers", "-productVersion")
	return "macOS " + strings.TrimSpace(string(v))
}
