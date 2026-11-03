// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package analytics

import (
	"fmt"
	"os"
	"syscall"
)

// openRegular opens path for reading without blocking (a FIFO would block a
// plain open), then requires the handle to be a regular file. Reading is
// done through this handle only, so the file checked is the file read.
func openRegular(path string) (*os.File, os.FileInfo, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("analytics: open %s: %w", path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("analytics: stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil, fmt.Errorf("analytics: not a regular file: %s", path)
	}
	// O_NONBLOCK has no effect on reads from a regular file.
	return f, info, nil
}
