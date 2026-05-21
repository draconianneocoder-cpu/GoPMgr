// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a short, URL-safe identifier prefixed with `prefix`,
// e.g. newID("chart") → "chart_3f2a91b4". The body is 8 hex chars of
// crypto/rand which is enough entropy for a per-user, per-project file
// (2^32 namespace, no collision in practice).
func newID(prefix string) string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return prefix + "_" + hex.EncodeToString(buf[:])
}
