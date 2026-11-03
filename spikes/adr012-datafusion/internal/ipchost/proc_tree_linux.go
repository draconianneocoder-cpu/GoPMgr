// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// exitWatch reports the worker's exit before anyone reaps it. Linux offers
// no unprivileged fork notification, so forked is always false.
type exitWatch struct{ pid int }

func newExitWatch(pid int) (*exitWatch, error) { return &exitWatch{pid: pid}, nil }

// wait blocks until the worker exits, leaving it unreaped. ok is false if
// the wait failed, in which case the worker may still be running.
func (w *exitWatch) wait() (forked, ok bool) {
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, w.pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return false, err == nil
	}
}

// processTable lists every process's parent, and whether it is stopped or
// a zombie, from /proc. The listing is not one atomic snapshot.
func processTable() (map[int]procEntry, error) {
	dir, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	table := make(map[int]procEntry, len(dir))
	for _, d := range dir {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + d.Name() + "/stat")
		if err != nil {
			continue // exited since the listing
		}
		// The command name may contain spaces and parentheses; the fields
		// after the last ')' are state then parent ID.
		end := bytes.LastIndexByte(stat, ')')
		if end < 0 {
			continue
		}
		f := bytes.Fields(stat[end+1:])
		if len(f) < 2 {
			continue
		}
		ppid, err := strconv.Atoi(string(f[1]))
		if err != nil {
			continue
		}
		table[pid] = procEntry{ppid: ppid, held: bytes.ContainsAny(f[0], "TtZX")}
	}
	return table, nil
}
