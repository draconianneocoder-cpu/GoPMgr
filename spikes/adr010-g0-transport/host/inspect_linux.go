// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// leakCheck looks for any secret in the worker's command line and
// environment (TPT-007). An error means the check did not run.
func leakCheck(pid int, secrets [][]byte) (string, bool, error) {
	const how = "/proc cmdline and environ"
	var leaked bool
	for _, f := range []string{"cmdline", "environ"} {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, f))
		if err != nil {
			return how, false, fmt.Errorf("read %s: %w", f, err)
		}
		for _, s := range secrets {
			if bytes.Contains(data, s) {
				leaked = true
			}
		}
	}
	return how, leaked, nil
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
		n, err := countTableSockets(fmt.Sprintf("/proc/%d/net/%s", pid, table), inodes)
		if errors.Is(err, os.ErrNotExist) {
			continue // no such table (for example tcp6 without IPv6): no such socket
		}
		if err != nil {
			// A table we could not read would undercount: report, do not pass.
			return 0, "", fmt.Errorf("read net/%s: %w", table, err)
		}
		count += n
	}
	return count, "/proc fd inodes against net/tcp and net/tcp6", nil
}

// countTableSockets counts rows of one /proc/net table whose inode is in
// inodes.
func countTableSockets(path string, inodes map[string]bool) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	count := 0
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) > 9 && inodes[fields[9]] {
			count++
		}
	}
	return count, sc.Err()
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

// procState reads a process's state letter and process group from
// /proc/<pid>/stat. The command name is in parentheses and may contain
// spaces, so parsing starts after the last ')'.
func procState(pid string) (state byte, pgrp int, ok bool) {
	data, err := os.ReadFile("/proc/" + pid + "/stat")
	if err != nil {
		return 0, 0, false
	}
	i := bytes.LastIndexByte(data, ')')
	if i < 0 {
		return 0, 0, false
	}
	f := strings.Fields(string(data[i+1:]))
	if len(f) < 3 {
		return 0, 0, false
	}
	g, err := strconv.Atoi(f[2])
	if err != nil {
		return 0, 0, false
	}
	return f[0][0], g, true
}

// groupMembers counts live (non-zombie) processes in a process group.
func groupMembers(pgid int) (int, string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return -1, "unreadable /proc: " + err.Error()
	}
	n := 0
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		if st, g, ok := procState(e.Name()); ok && g == pgid && st != 'Z' {
			n++
		}
	}
	return n, "/proc process groups, zombies excluded"
}

// processAlive reports whether pid is a live (non-zombie) process.
func processAlive(pid int) bool {
	st, _, ok := procState(strconv.Itoa(pid))
	return ok && st != 'Z'
}
