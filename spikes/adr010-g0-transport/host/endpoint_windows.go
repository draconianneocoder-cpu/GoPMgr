// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var errEndpointExists = errors.New("endpoint path already exists")

type endpoint struct {
	addr    string
	ln      net.Listener
	dir     string
	cleanup func() error
}

func transportsForOS() []string {
	return []string{"uds-nio", "npipe", "npipe-dual", "npipe-async", "npipe-async-single"}
}

// diagnosticTransports run only the cancel scenario, as controls. The
// default-pool overlapped pipe reproduces G0 finding W2 for comparison.
func diagnosticTransports() []string { return []string{"npipe-async-default"} }

// listen creates the endpoint for one transport. Named pipes get an
// explicit DACL for the current user only, first-instance creation
// (refusing an existing name) and remote-client rejection.
//
// npipe-async-single is npipe-async with a singlePipeListener: one
// instance, held by the host for the endpoint's life (G1 rule 6
// prototype). The worker side is the same overlapped pipe. The other pipe
// transports keep go-winio's listener, whose instances are unlimited (G0
// report §14.3), so the two can be compared in one run.
func listen(transport string) (*endpoint, error) {
	switch transport {
	case "uds-nio":
		dir, err := os.MkdirTemp("", "gopmgr-g0-")
		if err != nil {
			return nil, fmt.Errorf("create endpoint dir: %w", err)
		}
		ep, err := listenUnixAt(filepath.Join(dir, "w.sock"))
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		ep.dir = dir
		inner := ep.cleanup
		ep.cleanup = func() error {
			err := inner()
			if rmErr := os.RemoveAll(dir); err == nil {
				err = rmErr
			}
			return err
		}
		return ep, nil
	case "npipe", "npipe-dual", "npipe-async", "npipe-async-default", "npipe-async-single":
		id, err := randomID()
		if err != nil {
			return nil, err
		}
		name := `\\.\pipe\gopmgr-g0-` + id
		if transport == "npipe-async-single" {
			ln, err := listenSinglePipe(name)
			if err != nil {
				return nil, err
			}
			return &endpoint{addr: name, ln: ln, cleanup: ln.Close}, nil
		}
		ln, err := listenPipe(name)
		if err != nil {
			return nil, err
		}
		if transport == "npipe-dual" {
			ln = newPairListener(ln, peerPID, pairTimeout)
		}
		return &endpoint{addr: name, ln: ln, cleanup: ln.Close}, nil
	}
	return nil, fmt.Errorf("unknown transport %q", transport)
}

func userOnlySDDL() (string, error) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("read token user: %w", err)
	}
	// Protected DACL: generic access for this user's SID, nobody else.
	return "D:P(A;;GA;;;" + tu.User.Sid.String() + ")", nil
}

func listenPipe(name string) (net.Listener, error) {
	sddl, err := userOnlySDDL()
	if err != nil {
		return nil, err
	}
	ln, err := winio.ListenPipe(name, &winio.PipeConfig{
		SecurityDescriptor: sddl,
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	})
	if err != nil {
		return nil, fmt.Errorf("listen on pipe: %w", err)
	}
	return ln, nil
}

func listenUnixAt(path string) (*endpoint, error) {
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", errEndpointExists, path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on AF_UNIX socket: %w", err)
	}
	return &endpoint{addr: path, ln: ln, cleanup: ln.Close}, nil
}

// squatEndpoint checks both Windows squat defences: an existing pipe name
// and a pre-placed AF_UNIX path.
func squatEndpoint() (bool, string) {
	id, err := randomID()
	if err != nil {
		return false, err.Error()
	}
	name := `\\.\pipe\gopmgr-g0-squat-` + id
	squatter, err := listenPipe(name)
	if err != nil {
		return false, "squatter pipe: " + err.Error()
	}
	defer squatter.Close()
	pipeRefused := false
	// Both host listeners must refuse the squatted name.
	pipeDetail := "host created a second instance of a squatted pipe name"
	winioErr := "accepted"
	if ln, err := listenPipe(name); err != nil {
		winioErr = err.Error()
	} else {
		_ = ln.Close()
	}
	singleErr := "accepted"
	if ln, err := listenSinglePipe(name); err != nil {
		singleErr = err.Error()
	} else {
		_ = ln.Close()
	}
	if winioErr != "accepted" && singleErr != "accepted" {
		pipeRefused = true
		pipeDetail = "pipe: go-winio: " + winioErr + "; single-instance: " + singleErr
	} else {
		pipeDetail += " (go-winio: " + winioErr + "; single-instance: " + singleErr + ")"
	}

	dir, err := os.MkdirTemp("", "gopmgr-g0-squat-")
	if err != nil {
		return false, "mkdir: " + err.Error()
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "w.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return false, "pre-place: " + err.Error()
	}
	unixRefused := false
	unixDetail := "host listened on a pre-placed path"
	if ep, err := listenUnixAt(path); err != nil {
		unixRefused = errors.Is(err, errEndpointExists)
		unixDetail = "af_unix: " + err.Error()
	} else {
		_ = ep.cleanup()
	}
	return pipeRefused && unixRefused, pipeDetail + "; " + unixDetail
}

// peerPID asks Windows which process opened the pipe, or the AF_UNIX
// socket's peer through SIO_AF_UNIX_GETPEERPID.
func peerPID(c net.Conn) (int, error) {
	switch v := c.(type) {
	case *dualConn:
		a, err := peerPID(v.up)
		if err != nil {
			return 0, err
		}
		b, err := peerPID(v.down)
		if err != nil {
			return 0, err
		}
		if a != b {
			return 0, fmt.Errorf("dual pipe halves from different processes: %d and %d", a, b)
		}
		return a, nil
	case *net.UnixConn:
		return afUnixPeerPID(v)
	}
	if f, ok := c.(interface{ Fd() uintptr }); ok {
		var pid uint32
		if err := windows.GetNamedPipeClientProcessId(windows.Handle(f.Fd()), &pid); err != nil {
			return 0, fmt.Errorf("GetNamedPipeClientProcessId: %w", err)
		}
		return int(pid), nil
	}
	return 0, errPeerUnavailable
}

// sioAFUnixGetPeerPID is _WSAIOR(IOC_VENDOR, 256) from afunix.h; x/sys
// does not define it.
const sioAFUnixGetPeerPID = 0x58000100

func afUnixPeerPID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("syscall conn: %w", err)
	}
	var pid uint32
	var ioErr error
	if err := raw.Control(func(fd uintptr) {
		var got uint32
		ioErr = windows.WSAIoctl(windows.Handle(fd), sioAFUnixGetPeerPID, nil, 0,
			(*byte)(unsafe.Pointer(&pid)), uint32(unsafe.Sizeof(pid)), &got, nil, 0)
	}); err != nil {
		return 0, fmt.Errorf("control: %w", err)
	}
	if ioErr != nil {
		return 0, fmt.Errorf("%w: SIO_AF_UNIX_GETPEERPID: %v", errPeerUnavailable, ioErr)
	}
	return int(pid), nil
}

// dialEndpoint connects the way a foreign local process would (TPT-018).
func dialEndpoint(addr, transport string) (net.Conn, error) {
	timeout := 5 * time.Second
	switch transport {
	case "uds-nio":
		return net.Dial("unix", addr)
	case "npipe", "npipe-async", "npipe-async-default", "npipe-async-single":
		return winio.DialPipe(addr, &timeout)
	case "npipe-dual":
		up, err := winio.DialPipe(addr, &timeout)
		if err != nil {
			return nil, err
		}
		down, err := winio.DialPipe(addr, &timeout)
		if err != nil {
			_ = up.Close()
			return nil, err
		}
		// The client writes on the first connection and reads the second.
		return &dualConn{up: down, down: up}, nil
	}
	return nil, fmt.Errorf("unknown transport %q", transport)
}
