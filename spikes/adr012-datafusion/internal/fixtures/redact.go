// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import "strings"

// minRedactPrefix is the shortest truncated path prefix RedactDir removes.
// Shorter prefixes ("/", "/tmp") are too common to redact safely.
const minRedactPrefix = 8

// RedactDir replaces the corpus directory in an engine message with
// "<corpus>" so evidence carries no machine-specific path. Engines print
// the path whole, without its leading slash (DataFusion), or cut short
// with "..." (DuckDB's "LINE 1:" context); all three forms are removed.
func RedactDir(msg, dir string) string {
	for _, d := range []string{dir, strings.TrimPrefix(dir, "/")} {
		if d == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, d, "<corpus>")
		for n := len(d) - 1; n >= minRedactPrefix; n-- {
			msg = strings.ReplaceAll(msg, d[:n]+"...", "<corpus>...")
		}
	}
	return msg
}
