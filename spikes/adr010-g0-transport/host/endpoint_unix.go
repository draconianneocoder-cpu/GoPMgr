// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// endpointBase is short on purpose: socket paths are limited to about 104
// bytes on macOS and 108 on Linux, and the data root is far longer.
const endpointBase = "/tmp"

// maxSocketPath leaves headroom under the smallest platform limit.
const maxSocketPath = 100

var errEndpointExists = errors.New("endpoint path already exists")

type endpoint struct {
	addr    string
	ln      net.Listener
	dir     string
	cleanup func() error
}

// diagnosticTransports run only the cancel scenario, as controls.
func diagnosticTransports() []string { return nil }

// transportsForOS lists the worker transports this OS can attempt.
func transportsForOS() []string {
	if runtimeGOOS == "darwin" {
		return []string{"uds-kqueue", "uds-nio"}
	}
	return []string{"uds-epoll", "uds-nio"}
}

// listen creates a fresh private directory and a socket inside it
// (TPT-006, TPT-008). Every worker transport on this OS uses the same Go
// listener; only the Java side differs.
func listen(_ string) (*endpoint, error) {
	dir, err := os.MkdirTemp(endpointBase, "gopmgr-g0-")
	if err != nil {
		return nil, fmt.Errorf("create endpoint dir: %w", err)
	}
	if err := checkPrivateDir(dir); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	ep, err := listenAt(filepath.Join(dir, "w.sock"))
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
}

// checkPrivateDir refuses a directory that is a link, is not owned by this
// user, or is readable by anyone else.
func checkPrivateDir(dir string) error {
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

// listenAt refuses any existing file, link or stale socket at path, so a
// squatter cannot pre-place the endpoint.
func listenAt(path string) (*endpoint, error) {
	if len(path) > maxSocketPath {
		return nil, fmt.Errorf("socket path is %d bytes, over %d", len(path), maxSocketPath)
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", errEndpointExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat socket path: %w", err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on socket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("restrict socket mode: %w", err)
	}
	return &endpoint{addr: path, ln: ln, cleanup: ln.Close}, nil
}

// squatEndpoint pre-places a link where the endpoint would go and checks
// that the host refuses it.
func squatEndpoint() (refused bool, detail string) {
	dir, err := os.MkdirTemp(endpointBase, "gopmgr-g0-squat-")
	if err != nil {
		return false, "mkdir: " + err.Error()
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "w.sock")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), path); err != nil {
		return false, "symlink: " + err.Error()
	}
	ep, err := listenAt(path)
	if err == nil {
		_ = ep.cleanup()
		return false, "host listened on a pre-placed link"
	}
	return errors.Is(err, errEndpointExists), err.Error()
}

// dialEndpoint connects the way a foreign local process would (TPT-018).
func dialEndpoint(addr, _ string) (net.Conn, error) {
	return net.Dial("unix", addr)
}
