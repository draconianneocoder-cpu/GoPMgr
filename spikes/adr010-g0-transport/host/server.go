// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gopmgr-g0spike/host/g0pb"
)

// driver runs one scenario over an accepted, authenticated stream.
type driver func(stream g0pb.WorkerSession_ConnectServer) error

// session is the host side of one job: one listener, one expected child.
type session struct {
	g0pb.UnimplementedWorkerSessionServer

	proof    string
	drive    driver
	mu       sync.Mutex
	used     bool
	rejected atomic.Int64 // calls refused before any handler ran
	handled  atomic.Int64 // calls that reached the handler
	reasons  []string
}

func (s *session) note(reason string) {
	s.mu.Lock()
	s.reasons = append(s.reasons, reason)
	s.mu.Unlock()
}

// authorize checks the per-launch proof before any business message, and
// accepts it once (TPT-007, TPT-018).
func (s *session) authorize(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	got := md.Get("x-g0-proof")
	if len(got) != 1 || subtle.ConstantTimeCompare([]byte(got[0]), []byte(s.proof)) != 1 {
		s.rejected.Add(1)
		s.note("bad proof")
		return status.Error(codes.Unauthenticated, "connection proof did not verify")
	}
	s.mu.Lock()
	if s.used {
		s.mu.Unlock()
		s.rejected.Add(1)
		s.note("proof reused")
		return status.Error(codes.Unauthenticated, "connection proof already used")
	}
	s.used = true
	s.mu.Unlock()
	s.handled.Add(1)
	return handler(srv, ss)
}

func (s *session) Connect(stream g0pb.WorkerSession_ConnectServer) error {
	return s.drive(stream)
}

func newServer(s *session) *grpc.Server {
	srv := grpc.NewServer(
		grpc.StreamInterceptor(s.authorize),
		grpc.MaxRecvMsgSize(4<<20),
		grpc.MaxSendMsgSize(4<<20),
	)
	g0pb.RegisterWorkerSessionServer(srv, s)
	return srv
}

// peerListener admits only connections whose OS-reported peer process is
// the launched child, when the platform can say (TPT-018).
type peerListener struct {
	net.Listener
	mu        sync.Mutex
	expect    int
	peerCheck string // "enforced", or why it is unavailable
	refused   atomic.Int64
	admitted  atomic.Int64
	once      bool // stop accepting after the first admitted connection
	conn      *timedConn
}

// timedConn records when the admitted connection first failed a read,
// which is when the worker closed its end. Diagnostic only.
type timedConn struct {
	net.Conn
	mu      sync.Mutex
	readErr time.Time
}

func (c *timedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil {
		c.mu.Lock()
		if c.readErr.IsZero() {
			c.readErr = time.Now()
		}
		c.mu.Unlock()
	}
	return n, err
}

// closedAt is when the peer's close was first seen, or zero.
func (c *timedConn) closedAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readErr
}

func (l *peerListener) setExpected(pid int) {
	l.mu.Lock()
	l.expect = pid
	l.mu.Unlock()
	// A pairing listener checks each half itself, before pairing.
	if inner, ok := l.Listener.(interface{ setExpected(int) }); ok {
		inner.setExpected(pid)
	}
}

func (l *peerListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		pid, perr := peerPID(c)
		l.mu.Lock()
		expect := l.expect
		l.mu.Unlock()
		switch {
		case perr != nil:
			if errors.Is(perr, errPeerUnavailable) {
				l.peerCheck = "unavailable: " + perr.Error()
			} else {
				l.refused.Add(1)
				_ = c.Close()
				continue
			}
		case expect == 0 || pid != expect:
			// No worker yet, or another process: refuse.
			l.refused.Add(1)
			_ = c.Close()
			continue
		default:
			l.peerCheck = "enforced"
		}
		l.admitted.Add(1)
		if l.once {
			// One authenticated launch per endpoint: refuse anything after it.
			_ = l.Listener.Close()
		}
		tc := &timedConn{Conn: c}
		l.mu.Lock()
		if l.conn == nil {
			l.conn = tc
		}
		l.mu.Unlock()
		return tc, nil
	}
}

var errPeerUnavailable = errors.New("peer process ID not available on this transport")

func describe(err error) string {
	if err == nil {
		return "ok"
	}
	return fmt.Sprintf("%v", err)
}
