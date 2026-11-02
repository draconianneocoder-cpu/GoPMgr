// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"os"
	"syscall"
	"time"
)

// procEntry is one process in a processTable listing. held means the
// process is stopped or a zombie, so it cannot reap its children.
type procEntry struct {
	ppid int
	held bool
}

// killTree stops the worker, every process in its group and every
// descendant it can find, then kills them all (SEC-002). It returns how many
// descendants it killed. The caller must hold the lock that keeps the
// worker from being reaped, so the worker's process ID and group ID cannot
// be reused while they are signalled.
//
// A descendant is signalled only after its parent is seen held, so the
// descendant cannot be reaped and its ID reused before the signal lands.
// A descendant whose parent exited before the walk was reparented to init
// and is not found here.
func killTree(p *os.Process, pid int) int {
	if p.Signal(syscall.SIGSTOP) != nil {
		return 0 // already reaped: nothing of it left to signal
	}
	_ = syscall.Kill(-pid, syscall.SIGSTOP)
	found := stopDescendants(pid)
	for i := len(found) - 1; i >= 0; i-- {
		_ = syscall.Kill(found[i], syscall.SIGKILL)
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = p.Kill()
	return len(found)
}

// stopDescendants walks down from root one generation per listing, stopping
// each process whose parent the same listing shows held. It returns the
// stopped descendants, parents before children. The walk is bounded; a
// process that never stops (Linux uninterruptible sleep) ends it early.
func stopDescendants(root int) []int {
	stopped := map[int]bool{root: true}
	var order []int
	for range 200 {
		procs, err := processTable()
		if err != nil {
			return order
		}
		settled := true
		for pid := range stopped {
			if e, ok := procs[pid]; ok && !e.held {
				settled = false
			}
		}
		added := false
		for pid, e := range procs {
			parent, ok := procs[e.ppid]
			if stopped[pid] || !stopped[e.ppid] || !ok || !parent.held {
				continue
			}
			_ = syscall.Kill(pid, syscall.SIGSTOP)
			stopped[pid] = true
			order = append(order, pid)
			added = true
		}
		if settled && !added {
			return order
		}
		time.Sleep(time.Millisecond)
	}
	return order
}
