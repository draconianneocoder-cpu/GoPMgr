// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

const (
	workerPID  = 4242
	foreignPID = 6666
)

// fakeListener hands out server ends of in-memory pipes, each tagged with
// the process ID a real OS would report for its client.
type fakeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	pids   map[net.Conn]int
}

func newFakeListener() *fakeListener {
	return &fakeListener{conns: make(chan net.Conn), closed: make(chan struct{}), pids: map[net.Conn]int{}}
}

// connect delivers one connection from pid and returns its client end.
func (f *fakeListener) connect(t *testing.T, pid int) net.Conn {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	f.mu.Lock()
	f.pids[server] = pid
	f.mu.Unlock()
	select {
	case f.conns <- server:
	case <-time.After(2 * time.Second):
		t.Fatalf("listener did not take the connection from pid %d", pid)
	}
	return client
}

func (f *fakeListener) pidOf(c net.Conn) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pid, ok := f.pids[c]
	if !ok {
		return 0, errors.New("unknown connection")
	}
	return pid, nil
}

func (f *fakeListener) Accept() (net.Conn, error) {
	select {
	case c := <-f.conns:
		return c, nil
	case <-f.closed:
		return nil, net.ErrClosed
	}
}

func (f *fakeListener) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeListener) Addr() net.Addr { return &net.UnixAddr{Name: "fake"} }

type acceptResult struct {
	conn net.Conn
	err  error
}

func startAccept(l *pairListener) chan acceptResult {
	out := make(chan acceptResult, 1)
	go func() {
		c, err := l.Accept()
		out <- acceptResult{c, err}
	}()
	return out
}

// isClosedByServer reports whether the server end of client was closed.
func isClosedByServer(client net.Conn) bool {
	_ = client.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	_, err := client.Write([]byte{1})
	return errors.Is(err, io.ErrClosedPipe)
}

func TestPairListener(t *testing.T) {
	tests := []struct {
		name         string
		steps        func(t *testing.T, f *fakeListener) (up, down net.Conn, refusedClients []net.Conn)
		wantRefused  int64
		wantTimeouts int64
	}{
		{
			name: "two halves from the worker pair",
			steps: func(t *testing.T, f *fakeListener) (net.Conn, net.Conn, []net.Conn) {
				up := f.connect(t, workerPID)
				down := f.connect(t, workerPID)
				return up, down, nil
			},
		},
		{
			name: "a foreign first half is refused without taking a slot",
			steps: func(t *testing.T, f *fakeListener) (net.Conn, net.Conn, []net.Conn) {
				bad := f.connect(t, foreignPID)
				up := f.connect(t, workerPID)
				down := f.connect(t, workerPID)
				return up, down, []net.Conn{bad}
			},
			wantRefused: 1,
		},
		{
			name: "a foreign connection between the halves is refused",
			steps: func(t *testing.T, f *fakeListener) (net.Conn, net.Conn, []net.Conn) {
				up := f.connect(t, workerPID)
				bad := f.connect(t, foreignPID)
				down := f.connect(t, workerPID)
				return up, down, []net.Conn{bad}
			},
			wantRefused: 1,
		},
		{
			name: "a stalled first half times out and the next pair succeeds",
			steps: func(t *testing.T, f *fakeListener) (net.Conn, net.Conn, []net.Conn) {
				stalled := f.connect(t, workerPID)
				time.Sleep(150 * time.Millisecond) // longer than the test timeout
				up := f.connect(t, workerPID)
				down := f.connect(t, workerPID)
				return up, down, []net.Conn{stalled}
			},
			wantTimeouts: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeListener()
			l := newPairListener(f, f.pidOf, 50*time.Millisecond)
			l.setExpected(workerPID)
			t.Cleanup(func() { _ = l.Close() })
			got := startAccept(l)
			up, down, refused := tc.steps(t, f)
			var res acceptResult
			select {
			case res = <-got:
			case <-time.After(2 * time.Second):
				t.Fatal("Accept did not return")
			}
			if res.err != nil {
				t.Fatalf("Accept: %v", res.err)
			}
			dc, ok := res.conn.(*dualConn)
			if !ok {
				t.Fatalf("Accept returned %T, want *dualConn", res.conn)
			}
			// The client writes on its first half; the host reads it there.
			go func() { _, _ = up.Write([]byte("u")) }()
			buf := make([]byte, 1)
			if _, err := dc.Read(buf); err != nil || buf[0] != 'u' {
				t.Fatalf("read from first half: %q, %v", buf, err)
			}
			go func() { _, _ = dc.Write([]byte("d")) }()
			if _, err := down.Read(buf); err != nil || buf[0] != 'd' {
				t.Fatalf("write to second half: %q, %v", buf, err)
			}
			for _, c := range refused {
				if !isClosedByServer(c) {
					t.Errorf("a refused or timed-out half was left open")
				}
			}
			r, to := l.pairStats()
			if r != tc.wantRefused || to != tc.wantTimeouts {
				t.Errorf("refused %d timeouts %d, want %d and %d", r, to, tc.wantRefused, tc.wantTimeouts)
			}
		})
	}
}

func TestPairListenerCloseWhileWaitingForPartner(t *testing.T) {
	f := newFakeListener()
	l := newPairListener(f, f.pidOf, time.Minute)
	l.setExpected(workerPID)
	got := startAccept(l)
	up := f.connect(t, workerPID)
	time.Sleep(20 * time.Millisecond)
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case res := <-got:
		if res.err == nil {
			t.Fatal("Accept succeeded after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not return after Close")
	}
	if !isClosedByServer(up) {
		t.Error("the waiting first half was left open after Close")
	}
}

func TestPairListenerRefusesBeforeWorkerStarts(t *testing.T) {
	f := newFakeListener()
	l := newPairListener(f, f.pidOf, 50*time.Millisecond)
	t.Cleanup(func() { _ = l.Close() })
	got := startAccept(l)
	// No worker yet, so no expected process: even a connection whose
	// peer reads as process 0 must be refused.
	early := f.connect(t, 0)
	deadline := time.Now().Add(2 * time.Second)
	for r, _ := l.pairStats(); r != 1; r, _ = l.pairStats() {
		if time.Now().After(deadline) {
			t.Fatalf("refused %d before the worker started, want 1", r)
		}
		time.Sleep(time.Millisecond)
	}
	if !isClosedByServer(early) {
		t.Error("a connection that arrived before the worker started was left open")
	}
	l.setExpected(workerPID)
	f.connect(t, workerPID)
	f.connect(t, workerPID)
	select {
	case res := <-got:
		if res.err != nil {
			t.Fatalf("Accept: %v", res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not return")
	}
}
