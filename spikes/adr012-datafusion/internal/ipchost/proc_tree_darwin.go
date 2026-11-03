// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// Process states from <sys/proc.h>.
const (
	darwinStopped = 4 // SSTOP
	darwinZombie  = 5 // SZOMB
)

// exitWatch reports the worker's exit before anyone reaps it, and whether
// it created a process (fork or posix_spawn) at any time before that.
type exitWatch struct {
	kq     int
	exited bool // the worker exited before the watch was registered
}

// newExitWatch must run before the worker reads input: a fork made before
// the registration is not reported.
func newExitWatch(pid int) (*exitWatch, error) {
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	unix.CloseOnExec(kq)
	var ev unix.Kevent_t
	unix.SetKevent(&ev, pid, unix.EVFILT_PROC, unix.EV_ADD)
	ev.Fflags = unix.NOTE_FORK | unix.NOTE_EXIT
	if _, err := unix.Kevent(kq, []unix.Kevent_t{ev}, nil, nil); err != nil {
		_ = unix.Close(kq)
		// The caller has not reaped the worker, so ESRCH means it has
		// already exited: a zombie cannot be watched.
		if errors.Is(err, unix.ESRCH) {
			return &exitWatch{kq: -1, exited: true}, nil
		}
		return nil, fmt.Errorf("watch pid %d: %w", pid, err)
	}
	return &exitWatch{kq: kq}, nil
}

// wait blocks until the worker exits. ok is false if the watch failed, in
// which case the worker may still be running.
func (w *exitWatch) wait() (forked, ok bool) {
	if w.exited {
		return false, true
	}
	defer unix.Close(w.kq)
	events := make([]unix.Kevent_t, 1)
	for {
		n, err := unix.Kevent(w.kq, nil, events, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return forked, false
		}
		if n == 0 {
			continue
		}
		forked = forked || events[0].Fflags&unix.NOTE_FORK != 0
		if events[0].Fflags&unix.NOTE_EXIT != 0 {
			return forked, true
		}
	}
}

// processTable lists every process's parent, and whether it is stopped or
// a zombie, from one kernel snapshot.
func processTable() (map[int]procEntry, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	table := make(map[int]procEntry, len(procs))
	for _, p := range procs {
		st := p.Proc.P_stat
		table[int(p.Proc.P_pid)] = procEntry{ppid: int(p.Eproc.Ppid), held: st == darwinStopped || st == darwinZombie}
	}
	return table, nil
}
