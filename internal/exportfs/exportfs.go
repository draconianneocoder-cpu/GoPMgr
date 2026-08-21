// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package exportfs publishes user-requested exports without replacing an
// existing artifact. It is shared by Wails handlers and report services so
// primary PDFs and generated sidecars follow the same file-safety policy.
package exportfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrDestinationExists = errors.New("export destination already exists")

func EnsureAvailable(paths ...string) error {
	for _, path := range paths {
		if path == "" {
			return errors.New("export destination is required")
		}
		_, err := os.Lstat(path)
		switch {
		case err == nil:
			return fmt.Errorf("%w: %s", ErrDestinationExists, filepath.Base(path))
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect export destination %q: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// WriteNewPrivate writes complete bytes to a 0600 temporary file and then
// publishes it as a new hard link. The no-replacement policy is deliberate:
// a save dialog can confirm only a primary artifact, not its sidecars.
func WriteNewPrivate(path string, data []byte) (err error) {
	if err := EnsureAvailable(path); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gopmgr-export-*")
	if err != nil {
		return fmt.Errorf("create export temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set export temporary permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write export temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync export temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close export temporary file: %w", err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrDestinationExists, filepath.Base(path))
		}
		return fmt.Errorf("publish export to %q: %w", filepath.Base(path), err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove published export temporary file: %w", err)
	}
	return nil
}
