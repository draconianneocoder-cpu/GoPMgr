// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InspectMethod names how process state is observed, for the evidence.
const InspectMethod = "/proc: fd links matched against net/{unix,tcp,tcp6,udp,udp6}; cmdline, environ, status"

// Sockets counts a process's sockets by family. Other is any socket whose
// inode is in none of the tables (netlink, packet, ...).
type Sockets struct {
	Unix      int `json:"unix"`
	TCP       int `json:"tcp"`
	TCPListen int `json:"tcp_listen"`
	UDP       int `json:"udp"`
	Other     int `json:"other"`
	// Absent lists IP tables the kernel does not provide in this network
	// namespace (for example tcp6 without IPv6): no such socket can exist.
	Absent []string `json:"absent_tables,omitempty"`
}

// InspectSockets reads the process's open sockets (DF0-C item 6).
func InspectSockets(pid int) (Sockets, error) {
	var s Sockets
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return s, fmt.Errorf("read fds: %w", err)
	}
	inodes := map[string]bool{}
	for _, fd := range fds {
		link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
		if err == nil && strings.HasPrefix(link, "socket:[") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")] = true
		}
	}
	seen := map[string]bool{}
	scan := func(table string, inodeCol int, each func(fields []string)) error {
		f, err := os.Open(fmt.Sprintf("/proc/%d/net/%s", pid, table))
		if errors.Is(err, os.ErrNotExist) && table != "unix" {
			s.Absent = append(s.Absent, table)
			return nil
		}
		if err != nil {
			return fmt.Errorf("open net/%s: %w", table, err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Scan() // header
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) > inodeCol && inodes[fields[inodeCol]] {
				seen[fields[inodeCol]] = true
				each(fields)
			}
		}
		return sc.Err()
	}
	if err := scan("unix", 6, func([]string) { s.Unix++ }); err != nil {
		return s, err
	}
	for _, t := range []string{"tcp", "tcp6"} {
		if err := scan(t, 9, func(f []string) {
			s.TCP++
			if f[3] == "0A" {
				s.TCPListen++
			}
		}); err != nil {
			return s, err
		}
	}
	for _, t := range []string{"udp", "udp6"} {
		if err := scan(t, 9, func([]string) { s.UDP++ }); err != nil {
			return s, err
		}
	}
	for in := range inodes {
		if !seen[in] {
			s.Other++
		}
	}
	return s, nil
}

// SecretsVisible reports whether any secret appears in the process's
// command line or environment.
func SecretsVisible(pid int, secrets ...[]byte) (bool, error) {
	for _, f := range []string{"cmdline", "environ"} {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, f))
		if err != nil {
			return false, fmt.Errorf("read %s: %w", f, err)
		}
		for _, s := range secrets {
			if len(s) > 0 && bytes.Contains(data, s) {
				return true, nil
			}
		}
	}
	return false, nil
}

// FDCount is the number of open descriptors of a process.
func FDCount(pid int) int {
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return -1
	}
	return len(fds)
}

// Threads is the kernel's thread count for a process.
func Threads(pid int) int64 { return statusField(pid, "Threads", 1) }

// PeakRSS is the kernel's resident-memory high-water mark, in bytes.
func PeakRSS(pid int) int64 { return statusField(pid, "VmHWM", 1024) }

func statusField(pid int, key string, scale int64) int64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+":") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				v, err := strconv.ParseInt(f[1], 10, 64)
				if err == nil {
					return v * scale
				}
			}
		}
	}
	return -1
}

// Alive reports whether a process exists and is not a zombie. kill(pid, 0)
// alone is not enough: it succeeds on a zombie that nobody has reaped yet.
func Alive(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	i := bytes.LastIndexByte(data, ')')
	if i < 0 || i+2 >= len(data) {
		return false
	}
	return data[i+2] != 'Z' && data[i+2] != 'X'
}
