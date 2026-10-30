// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// singlePipeListener prototypes G1 admission rule 6: the endpoint's pipe
// has exactly one instance, and the host holds it for the endpoint's whole
// life.
//
//   - The name allows one instance (nMaxInstances = 1). The host creates it
//     with FILE_FLAG_FIRST_PIPE_INSTANCE, so no other process can create the
//     name first or add an instance after.
//   - A client from any process but the expected one is disconnected from
//     that same instance (DisconnectNamedPipe), and the instance waits again.
//     The slot is never free, so no other process can take it.
//   - The first client from the expected process is handed out, once.
//
// go-winio's ListenPipe cannot do this: it creates every instance with no
// limit, and a refused client's instance is closed rather than reused.
type singlePipeListener struct {
	name    string
	h       windows.Handle
	closeEv windows.Handle // manual-reset; set by Close
	expect  atomic.Int64
	refused atomic.Int64

	mu        sync.Mutex
	accepting bool // an Accept owns h right now
	used      bool // h belongs to the handed-out connection
	closed    bool
}

func listenSinglePipe(name string) (*singlePipeListener, error) {
	sddl, err := userOnlySDDL()
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("parse pipe DACL: %w", err)
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateNamedPipe(p,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		1, 64<<10, 64<<10, 0, sa)
	if err != nil {
		return nil, fmt.Errorf("create the pipe's only instance: %w", err)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = windows.CloseHandle(h)
		return nil, fmt.Errorf("create close event: %w", err)
	}
	return &singlePipeListener{name: name, h: h, closeEv: ev}, nil
}

// setExpected names the only process whose connection is handed out.
func (l *singlePipeListener) setExpected(pid int) { l.expect.Store(int64(pid)) }

// pairStats reports clients refused at this layer, so the scenarios count
// them as they count a pairing listener's refused halves.
func (l *singlePipeListener) pairStats() (refused, timeouts int64) { return l.refused.Load(), 0 }

func (l *singlePipeListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed || l.used || l.accepting {
		l.mu.Unlock()
		// One connection per endpoint: wait for Close, then report it.
		_, _ = windows.WaitForSingleObject(l.closeEv, windows.INFINITE)
		return nil, net.ErrClosed
	}
	l.accepting = true
	l.mu.Unlock()

	conn, err := l.acceptOnce()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.accepting = false
	if conn != nil {
		l.used = true
	} else if l.closed {
		_ = windows.CloseHandle(l.h)
	}
	return conn, err
}

func (l *singlePipeListener) acceptOnce() (net.Conn, error) {
	for {
		if err := l.connect(); err != nil {
			return nil, err
		}
		var pid uint32
		perr := windows.GetNamedPipeClientProcessId(l.h, &pid)
		want := l.expect.Load()
		if perr != nil || want == 0 || int64(pid) != want {
			// Refuse on the same instance; the slot stays the host's.
			l.refused.Add(1)
			if err := windows.DisconnectNamedPipe(l.h); err != nil {
				return nil, fmt.Errorf("disconnect a refused client: %w", err)
			}
			continue
		}
		f, err := winio.NewOpenFile(l.h)
		if err != nil {
			return nil, fmt.Errorf("wrap the pipe: %w", err)
		}
		return &singlePipeConn{ReadWriteCloser: f, h: l.h, name: l.name}, nil
	}
}

// connect waits for a client on the instance, or for Close. It uses an
// event, not a completion port, so a refused client leaves the handle
// unassociated and reusable.
func (l *singlePipeListener) connect() error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("create connect event: %w", err)
	}
	defer windows.CloseHandle(ev)
	ov := &windows.Overlapped{HEvent: ev}
	err = windows.ConnectNamedPipe(l.h, ov)
	switch {
	case err == nil, errors.Is(err, windows.ERROR_PIPE_CONNECTED):
		return nil
	case !errors.Is(err, windows.ERROR_IO_PENDING):
		return fmt.Errorf("connect pipe: %w", err)
	}
	which, err := windows.WaitForMultipleObjects([]windows.Handle{ev, l.closeEv}, false, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("wait for a client: %w", err)
	}
	var n uint32
	if which == windows.WAIT_OBJECT_0+1 {
		_ = windows.CancelIoEx(l.h, ov)
		_ = windows.GetOverlappedResult(l.h, ov, &n, true) // the kernel is done with ov
		return net.ErrClosed
	}
	if err := windows.GetOverlappedResult(l.h, ov, &n, false); err != nil {
		return fmt.Errorf("connect pipe: %w", err)
	}
	return nil
}

// Close stops accepting. An instance never handed out is closed here, or by
// the Accept that still owns it; a handed-out one belongs to its connection.
func (l *singlePipeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	_ = windows.SetEvent(l.closeEv)
	if !l.accepting && !l.used {
		_ = windows.CloseHandle(l.h)
	}
	return nil
}

func (l *singlePipeListener) Addr() net.Addr { return pipeName(l.name) }

// singlePipeConn is the admitted connection. Fd lets the peer listener
// read the client's process ID.
type singlePipeConn struct {
	io.ReadWriteCloser
	h    windows.Handle
	name string
}

type deadliner interface {
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
}

func (c *singlePipeConn) Fd() uintptr          { return uintptr(c.h) }
func (c *singlePipeConn) LocalAddr() net.Addr  { return pipeName(c.name) }
func (c *singlePipeConn) RemoteAddr() net.Addr { return pipeName(c.name) }
func (c *singlePipeConn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *singlePipeConn) SetReadDeadline(t time.Time) error {
	if d, ok := c.ReadWriteCloser.(deadliner); ok {
		return d.SetReadDeadline(t)
	}
	return nil
}

func (c *singlePipeConn) SetWriteDeadline(t time.Time) error {
	if d, ok := c.ReadWriteCloser.(deadliner); ok {
		return d.SetWriteDeadline(t)
	}
	return nil
}

type pipeName string

func (pipeName) Network() string  { return "pipe" }
func (a pipeName) String() string { return string(a) }
