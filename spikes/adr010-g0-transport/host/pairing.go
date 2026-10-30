// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// pairTimeout bounds how long the first half of a dual connection waits
// for its partner. The worker opens both halves back to back, so a
// healthy pair arrives in milliseconds.
const pairTimeout = 3 * time.Second

var errPairTimeout = errors.New("second half of a dual connection did not arrive in time")

// pairListener turns two connections from one process into one duplex
// connection: the client writes on the first and reads from the second
// (`npipe-dual`). It checks every half against the expected peer as soon
// as it is accepted, so a foreign connection is refused without taking a
// slot, and it closes a first half whose partner does not arrive within
// the timeout. One goroutine owns the inner Accept; it ends when the
// listener is closed.
type pairListener struct {
	inner   net.Listener
	pidOf   func(net.Conn) (int, error)
	timeout time.Duration
	expect  atomic.Int64

	startOnce sync.Once
	closeOnce sync.Once
	conns     chan net.Conn
	done      chan struct{} // closed by Close
	failed    chan struct{} // closed when the inner Accept fails
	failErr   error         // set before failed is closed

	refused  atomic.Int64 // halves from the wrong process, or whose peer could not be read
	timeouts atomic.Int64 // first halves closed because no partner arrived
}

func newPairListener(inner net.Listener, pidOf func(net.Conn) (int, error), timeout time.Duration) *pairListener {
	return &pairListener{
		inner:   inner,
		pidOf:   pidOf,
		timeout: timeout,
		conns:   make(chan net.Conn),
		done:    make(chan struct{}),
		failed:  make(chan struct{}),
	}
}

// setExpected names the only process whose halves are accepted.
func (l *pairListener) setExpected(pid int) { l.expect.Store(int64(pid)) }

// pairStats reports refused halves and pairing timeouts.
func (l *pairListener) pairStats() (refused, timeouts int64) {
	return l.refused.Load(), l.timeouts.Load()
}

func (l *pairListener) run() {
	for {
		c, err := l.inner.Accept()
		if err != nil {
			l.failErr = err
			close(l.failed)
			return
		}
		select {
		case l.conns <- c:
		case <-l.done:
			_ = c.Close()
			return
		}
	}
}

// next returns the next accepted connection, or errPairTimeout when the
// timeout channel fires first. A nil timeout waits until Close.
func (l *pairListener) next(timeout <-chan time.Time) (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.failed:
		return nil, l.failErr
	case <-l.done:
		return nil, net.ErrClosed
	case <-timeout:
		return nil, errPairTimeout
	}
}

// fromPeer keeps c only when it comes from the expected process. Until
// the worker has started there is no expected process, so every
// connection is refused.
func (l *pairListener) fromPeer(c net.Conn) bool {
	want := l.expect.Load()
	pid, err := l.pidOf(c)
	if err != nil || want == 0 || int64(pid) != want {
		l.refused.Add(1)
		_ = c.Close()
		return false
	}
	return true
}

func (l *pairListener) Accept() (net.Conn, error) {
	l.startOnce.Do(func() { go l.run() })
	for {
		up, err := l.next(nil)
		if err != nil {
			return nil, err
		}
		if !l.fromPeer(up) {
			continue
		}
		down, err := l.partner()
		if errors.Is(err, errPairTimeout) {
			l.timeouts.Add(1)
			_ = up.Close()
			continue
		}
		if err != nil {
			_ = up.Close()
			return nil, err
		}
		return &dualConn{up: up, down: down}, nil
	}
}

// partner waits, up to the timeout, for a second half from the expected
// process, refusing any foreign connection that arrives meanwhile.
func (l *pairListener) partner() (net.Conn, error) {
	deadline := time.NewTimer(l.timeout)
	defer deadline.Stop()
	for {
		c, err := l.next(deadline.C)
		if err != nil {
			return nil, err
		}
		if l.fromPeer(c) {
			return c, nil
		}
	}
}

func (l *pairListener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return l.inner.Close()
}

func (l *pairListener) Addr() net.Addr { return l.inner.Addr() }

// dualConn is one duplex connection made of two halves: reads come from
// up (the client's first connection), writes go to down.
type dualConn struct{ up, down net.Conn }

func (c *dualConn) Read(p []byte) (int, error)  { return c.up.Read(p) }
func (c *dualConn) Write(p []byte) (int, error) { return c.down.Write(p) }
func (c *dualConn) Close() error {
	e1, e2 := c.up.Close(), c.down.Close()
	return errors.Join(e1, e2)
}
func (c *dualConn) LocalAddr() net.Addr  { return c.up.LocalAddr() }
func (c *dualConn) RemoteAddr() net.Addr { return c.up.RemoteAddr() }
func (c *dualConn) SetDeadline(t time.Time) error {
	return errors.Join(c.up.SetDeadline(t), c.down.SetDeadline(t))
}
func (c *dualConn) SetReadDeadline(t time.Time) error  { return c.up.SetReadDeadline(t) }
func (c *dualConn) SetWriteDeadline(t time.Time) error { return c.down.SetWriteDeadline(t) }
