// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "gopmgr/internal/pdfdoc"

// SetFontApplier installs the hook that registers the embedded font on each
// new document PDF. Passing nil leaves documents on fpdf's core Helvetica,
// which is not embedded; only tests that measure core-font output do that.
// It does not change ActiveFamily. Production code selects fonts with
// UseFont.
func SetFontApplier(fn func(*pdfdoc.PDF)) {
	fontMu.Lock()
	fontApplier = fn
	fontMu.Unlock()
}
