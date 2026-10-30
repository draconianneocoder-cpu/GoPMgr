// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// EndpointBase is short on purpose: Unix socket paths are limited to about
// 104 bytes on macOS and 108 on Linux.
var EndpointBase = "/tmp"

// maxSocketPath leaves headroom under the smallest platform limit.
const maxSocketPath = 100

// ErrEndpointExists means something already occupies the socket path.
var ErrEndpointExists = errors.New("endpoint path already exists")

// Endpoint is one job's private socket.
type Endpoint struct {
	Path string
	Dir  string
	ln   net.Listener
}

// Close stops listening and removes the private directory.
func (e *Endpoint) Close() error {
	err := e.ln.Close()
	if e.Dir != "" {
		if rmErr := os.RemoveAll(e.Dir); err == nil {
			err = rmErr
		}
	}
	return err
}

// Listen creates a fresh 0700 directory with a 0600 socket inside it, one
// per job (DF-026). It refuses a directory it does not privately own.
func Listen() (*Endpoint, error) {
	dir, err := os.MkdirTemp(EndpointBase, "gopmgr-df0-")
	if err != nil {
		return nil, fmt.Errorf("create endpoint dir: %w", err)
	}
	if err := CheckPrivateDir(dir); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	ep, err := ListenAt(filepath.Join(dir, "w.sock"))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	ep.Dir = dir
	return ep, nil
}

// CheckPrivateDir refuses a directory that is a link, is not owned by this
// user, or is open to anyone else.
func CheckPrivateDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("stat endpoint dir: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return fmt.Errorf("endpoint dir %s is not a plain directory", dir)
	}
	if fi.Mode().Perm() != 0o700 {
		return fmt.Errorf("endpoint dir mode is %v, want 0700", fi.Mode().Perm())
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("endpoint dir is not owned by this user")
	}
	return nil
}

// ListenAt refuses any existing file, link or stale socket at path, so a
// squatter cannot pre-place the endpoint (DF-026).
func ListenAt(path string) (*Endpoint, error) {
	if len(path) > maxSocketPath {
		return nil, fmt.Errorf("socket path is %d bytes, over %d", len(path), maxSocketPath)
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrEndpointExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat socket path: %w", err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on socket: %w", err)
	}
	// Go removes the socket file when the listener closes.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("restrict socket mode: %w", err)
	}
	return &Endpoint{Path: path, ln: ln}, nil
}
