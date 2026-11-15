// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// newID returns a URL-safe identifier: prefix, an underscore, and 128 random
// bits encoded as 32 lowercase hexadecimal characters. Existing stored IDs
// remain opaque strings; this changes only newly generated identifiers.
func newID(prefix string) (string, error) {
	var buf [16]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}
