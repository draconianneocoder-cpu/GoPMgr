// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package analytics

import (
	"fmt"
	"os"
)

// openRegular opens path and requires the handle to be a regular file.
func openRegular(path string) (*os.File, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("analytics: open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("analytics: stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, nil, fmt.Errorf("analytics: not a regular file: %s", path)
	}
	return f, info, nil
}
