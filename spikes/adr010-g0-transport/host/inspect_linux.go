// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// leakCheck looks for any secret in the worker's command line and
// environment (TPT-007).
func leakCheck(pid int, secrets [][]byte) (string, bool) {
	var leaked bool
	for _, f := range []string{"cmdline", "environ"} {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, f))
		if err != nil {
			return "unreadable: " + err.Error(), false
		}
		for _, s := range secrets {
			if bytes.Contains(data, s) {
				leaked = true
			}
		}
	}
	return "/proc cmdline and environ", leaked
}

// tcpSocketCount counts TCP sockets of any state the worker owns
// (TPT-005, TPT-020).
func tcpSocketCount(pid int) (int, string, error) {
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return 0, "", fmt.Errorf("read fds: %w", err)
	}
	inodes := map[string]bool{}
	for _, fd := range fds {
		link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
		if err == nil && strings.HasPrefix(link, "socket:[") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")] = true
		}
	}
	count := 0
	for _, table := range []string{"tcp", "tcp6"} {
		f, err := os.Open(fmt.Sprintf("/proc/%d/net/%s", pid, table))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Scan() // header
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) > 9 && inodes[fields[9]] {
				count++
			}
		}
		_ = f.Close()
	}
	return count, "/proc fd inodes against net/tcp and net/tcp6", nil
}

// peakRSS reads the kernel's high-water mark for resident memory.
func peakRSS(pid int) (int64, string) {
	return statusKB(fmt.Sprintf("/proc/%d/status", pid), "VmHWM"), "VmHWM (kernel peak)"
}

func selfPeakRSS() int64 { return statusKB("/proc/self/status", "VmHWM") }

func statusKB(path, key string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+":") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				kb, _ := strconv.ParseInt(f[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return -1
}

func osVersion() string {
	rel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	name := "linux"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	return name + ", kernel " + strings.TrimSpace(string(rel))
}
