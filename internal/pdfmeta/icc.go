// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfmeta

import _ "embed"

//go:embed assets/sRGB.icc
var sRGBICC []byte

// DefaultICCProfile returns the embedded compact sRGB ICC profile used for
// PDF/A-3 OutputIntent metadata.
func DefaultICCProfile() []byte {
	if len(sRGBICC) == 0 {
		return nil
	}
	// Return a copy so callers cannot mutate the embedded data.
	out := make([]byte, len(sRGBICC))
	copy(out, sRGBICC)
	return out
}

// HasDefaultICC reports whether the compact sRGB profile was embedded.
func HasDefaultICC() bool {
	return len(sRGBICC) > 0
}
