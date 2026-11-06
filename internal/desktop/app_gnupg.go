// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopmgr/internal/signing"
)

// signFileWithGnuPG keeps the signer's output out of the export directory.
// The OS temporary parent and local signer are trusted; owner-only staging
// is not an isolation boundary against another process with the same UID.
func (a *App) signFileWithGnuPG(inputPath, keyID string) (signature []byte, err error) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 2*time.Minute)
	defer cancel()
	directory, err := os.MkdirTemp("", "gopmgr-signature-*")
	if err != nil {
		return nil, fmt.Errorf("create private GnuPG staging directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			signature = nil
			err = errors.Join(err, fmt.Errorf("remove private GnuPG staging directory: %w", cleanupErr))
		}
	}()
	path := filepath.Join(directory, "signature.asc")
	if err := signing.SignDetachedASCIIArmored(ctx, signing.ExecCommandRunner, inputPath, path, strings.TrimSpace(keyID)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect temporary GnuPG signature: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, errors.New("temporary GnuPG signature must be a nonempty regular file")
	}
	signature, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read temporary GnuPG signature: %w", err)
	}
	return signature, nil
}
