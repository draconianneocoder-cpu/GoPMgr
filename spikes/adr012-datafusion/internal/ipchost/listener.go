// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var errPeerUnavailable = errors.New("peer process ID not available on this transport")

// peerListener admits exactly one connection, and only from the launched
// worker's process ID (DF-026, DF-037). It refuses everything else,
// including any connection after the first.
type peerListener struct {
	net.Listener
	ready    chan struct{} // closed when the worker's PID is known
	once     sync.Once
	expect   atomic.Int64
	deadline time.Time
	admitted atomic.Int64
	refused  atomic.Int64
	mu       sync.Mutex
	reasons  []string
}

func newPeerListener(ln net.Listener, deadline time.Time) *peerListener {
	return &peerListener{Listener: ln, ready: make(chan struct{}), deadline: deadline}
}

// setExpected records the launched worker. Connections that arrive before
// it are held, not refused, so a fast worker cannot lose a race with the
// host's own bookkeeping.
func (l *peerListener) setExpected(pid int) {
	l.expect.Store(int64(pid))
	l.once.Do(func() { close(l.ready) })
}

func (l *peerListener) refuse(c net.Conn, reason string) {
	l.refused.Add(1)
	l.mu.Lock()
	l.reasons = append(l.reasons, reason)
	l.mu.Unlock()
	_ = c.Close()
}

func (l *peerListener) refusals() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.reasons...)
}

func (l *peerListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case <-l.ready:
		case <-time.After(time.Until(l.deadline)):
			l.refuse(c, "no worker launched before the deadline")
			continue
		}
		pid, perr := peerPID(c)
		switch {
		case perr != nil:
			// Fail closed: without a peer identity there is no admission.
			l.refuse(c, "peer identity unavailable: "+perr.Error())
			continue
		case int64(pid) != l.expect.Load():
			l.refuse(c, "peer is not the launched worker")
			continue
		case l.admitted.Load() > 0:
			l.refuse(c, "endpoint already used")
			continue
		}
		l.admitted.Add(1)
		return c, nil
	}
}
