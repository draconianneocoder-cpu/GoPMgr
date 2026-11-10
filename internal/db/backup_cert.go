// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// errBackupCertRejected reports a certificate path that may not be copied
// into an archive: not a .p12/.pfx file, not a regular file, or larger than
// maxBackupCertSize.
var errBackupCertRejected = errors.New("certificate cannot be added to the archive")

// readBackupCert returns the bytes of the certificate bundle at path, or
// found=false when no file exists there.
//
// The path comes from the project's cert_path setting, which is stored in
// the project database and so can be set by a restored archive. Only a
// file with the extensions the certificate chooser offers is opened, the
// open does not block on a FIFO, and the read is capped at the size a
// restore accepts. The caller hashes and stores these bytes, so the file
// checked is the file archived.
func readBackupCert(path string) (data []byte, found bool, err error) {
	ext := filepath.Ext(path)
	if !strings.EqualFold(ext, ".p12") && !strings.EqualFold(ext, ".pfx") {
		return nil, false, fmt.Errorf("%w: %s is not a .p12 or .pfx file", errBackupCertRejected, path)
	}
	f, err := openBackupCert(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: %s is not a regular file", errBackupCertRejected, path)
	}
	data, err = io.ReadAll(io.LimitReader(f, maxBackupCertSize+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxBackupCertSize {
		return nil, false, fmt.Errorf("%w: %s is larger than %d MiB", errBackupCertRejected, path, maxBackupCertSize>>20)
	}
	return data, true, nil
}
