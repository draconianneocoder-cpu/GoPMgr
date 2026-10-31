// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package source

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Capture opens the host-selected file once. Nonblocking open avoids waiting on
// a FIFO before regular-file validation. Selected aliases are resolved only here.
func Capture(path string) (*Snapshot, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	f := os.NewFile(uintptr(fd), "DF0 source")
	s, readErr := captureFile(f)
	closeErr := f.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close source: %w", closeErr)
	}
	return s, nil
}

func captureFile(f *os.File) (*Snapshot, error) {
	before, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("source is not a regular file")
	}
	if before.Size() > MaxSourceBytes {
		return nil, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("capture source: %w", err)
	}
	if len(data) > MaxSourceBytes {
		return nil, ErrLimit
	}
	after, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("restat source: %w", err)
	}
	if before.Size() != after.Size() || before.Size() != int64(len(data)) || !before.ModTime().Equal(after.ModTime()) {
		return nil, ErrChanged
	}
	return snapshotBytes(data), nil
}
