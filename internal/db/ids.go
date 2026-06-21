// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// newID returns a short, URL-safe identifier prefixed with `prefix`,
// e.g. newID("chart") → "chart_3f2a91b4". The body is 8 hex chars of
// crypto/rand which is enough entropy for a per-user, per-project file
// (2^32 namespace, no collision in practice).
func newID(prefix string) (string, error) {
	var buf [4]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}
