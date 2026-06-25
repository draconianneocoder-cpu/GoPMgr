// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package fonts manages the TrueType fonts PMForge embeds in generated
// PDFs. It bundles a curated set of professional, modern, open-source
// fonts (all free for commercial AND personal use, all GPL-compatible)
// and lets users add their own .ttf files.
//
// # Why fonts matter for PMForge
//
// fpdf's built-in core fonts (Helvetica, Times, Courier) are NOT
// embedded in the PDF and are NOT permitted by strict PDF/A. To move
// toward PDF/A-3 conformance — and to give users real typographic
// choice — PMForge embeds TrueType fonts via fpdf's
// AddUTF8FontFromBytes. This package owns the catalog of bundled
// fonts, the runtime registration, and user-supplied font import.
//
// Important constraint: fpdf's UTF-8 font path parses TrueType
// tables only. OpenType/CFF fonts (.otf with "OTTO" signature) are
// NOT supported and are rejected at import time with a clear error.
package fonts

import "strings"

// Style is a font style. The zero value is Regular.
type Style int

const (
	Regular Style = iota
	Bold
	Italic
	BoldItalic
)

// FpdfStyle returns the style string fpdf's SetFont /
// AddUTF8FontFromBytes expect: "" / "B" / "I" / "BI".
func (s Style) FpdfStyle() string {
	switch s {
	case Bold:
		return "B"
	case Italic:
		return "I"
	case BoldItalic:
		return "BI"
	default:
		return ""
	}
}

// String returns a human-readable style name.
func (s Style) String() string {
	switch s {
	case Bold:
		return "Bold"
	case Italic:
		return "Italic"
	case BoldItalic:
		return "Bold Italic"
	default:
		return "Regular"
	}
}

// AllStyles is the canonical iteration order for the four faces.
var AllStyles = []Style{Regular, Bold, Italic, BoldItalic}

// FontFile binds one style of a family to its embedded filename.
type FontFile struct {
	Style    Style
	FileName string // base name within the assets directory, e.g. "LiberationSans-Regular.ttf"
}

// FontFamily describes one bundled font family across its styles.
type FontFamily struct {
	// Name is the family name used with SetFont (e.g. "Liberation Sans").
	Name string
	// Category groups families for the UI: "sans", "serif", or "mono".
	Category string
	// Description is a one-line note shown in the font picker.
	Description string
	// License is the SPDX identifier (e.g. "OFL-1.1", "Apache-2.0").
	License string
	// Source is the canonical download URL the fetch script uses.
	Source string
	// Files lists the per-style TTF files. Regular is required; the
	// other styles are optional (the Manager falls back to Regular).
	Files []FontFile
}

// File returns the FontFile for a given style, falling back to Regular
// when the requested style isn't present.
func (f FontFamily) File(style Style) (FontFile, bool) {
	var regular FontFile
	haveRegular := false
	for _, ff := range f.Files {
		if ff.Style == style {
			return ff, true
		}
		if ff.Style == Regular {
			regular = ff
			haveRegular = true
		}
	}
	if haveRegular {
		return regular, true
	}
	return FontFile{}, false
}

// Catalog is the curated set of bundled font families. Every family is
// free for commercial AND personal use and GPL-3.0-compatible.
//
// The actual .ttf binaries are NOT committed to the repository (they
// are large binaries). scripts/fetch-fonts.sh downloads them from the
// canonical sources below into internal/fonts/assets/, where the
// embed directive in manager.go picks them up at build time. If a
// family's files are absent at build time, the Manager simply omits
// it and the renderers fall back to the next available family (and
// ultimately to fpdf's core Helvetica), so the app always works.
var Catalog = []FontFamily{
	{
		Name:        "Liberation Sans",
		Category:    "sans",
		Description: "Metric-compatible with Arial. Safe professional default.",
		License:     "OFL-1.1",
		Source:      "https://github.com/liberationfonts/liberation-fonts",
		Files: []FontFile{
			{Regular, "LiberationSans-Regular.ttf"},
			{Bold, "LiberationSans-Bold.ttf"},
			{Italic, "LiberationSans-Italic.ttf"},
			{BoldItalic, "LiberationSans-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Liberation Serif",
		Category:    "serif",
		Description: "Metric-compatible with Times New Roman.",
		License:     "OFL-1.1",
		Source:      "https://github.com/liberationfonts/liberation-fonts",
		Files: []FontFile{
			{Regular, "LiberationSerif-Regular.ttf"},
			{Bold, "LiberationSerif-Bold.ttf"},
			{Italic, "LiberationSerif-Italic.ttf"},
			{BoldItalic, "LiberationSerif-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Liberation Mono",
		Category:    "mono",
		Description: "Metric-compatible with Courier New. Monospaced.",
		License:     "OFL-1.1",
		Source:      "https://github.com/liberationfonts/liberation-fonts",
		Files: []FontFile{
			{Regular, "LiberationMono-Regular.ttf"},
			{Bold, "LiberationMono-Bold.ttf"},
			{Italic, "LiberationMono-Italic.ttf"},
			{BoldItalic, "LiberationMono-BoldItalic.ttf"},
		},
	},
	{
		Name:        "DejaVu Sans",
		Category:    "sans",
		Description: "Widest glyph coverage. Conventional fpdf pick.",
		License:     "Bitstream-Vera",
		Source:      "https://github.com/dejavu-fonts/dejavu-fonts",
		Files: []FontFile{
			{Regular, "DejaVuSans.ttf"},
			{Bold, "DejaVuSans-Bold.ttf"},
			{Italic, "DejaVuSans-Oblique.ttf"},
			{BoldItalic, "DejaVuSans-BoldOblique.ttf"},
		},
	},
	{
		Name:        "Noto Sans",
		Category:    "sans",
		Description: "Google Noto. Broad international coverage.",
		License:     "OFL-1.1",
		Source:      "https://github.com/notofonts/notofonts.github.io",
		Files: []FontFile{
			{Regular, "NotoSans-Regular.ttf"},
			{Bold, "NotoSans-Bold.ttf"},
			{Italic, "NotoSans-Italic.ttf"},
			{BoldItalic, "NotoSans-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Source Sans 3",
		Category:    "sans",
		Description: "Adobe's modern professional sans-serif.",
		License:     "OFL-1.1",
		Source:      "https://github.com/adobe-fonts/source-sans",
		Files: []FontFile{
			{Regular, "SourceSans3-Regular.ttf"},
			{Bold, "SourceSans3-Bold.ttf"},
			{Italic, "SourceSans3-It.ttf"},
			{BoldItalic, "SourceSans3-BoldIt.ttf"},
		},
	},
	{
		Name:        "JetBrains Mono",
		Category:    "mono",
		Description: "Modern monospaced font for code-heavy documents.",
		License:     "OFL-1.1",
		Source:      "https://github.com/JetBrains/JetBrainsMono",
		Files: []FontFile{
			{Regular, "JetBrainsMono-Regular.ttf"},
			{Bold, "JetBrainsMono-Bold.ttf"},
			{Italic, "JetBrainsMono-Italic.ttf"},
			{BoldItalic, "JetBrainsMono-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Roboto",
		Category:    "sans",
		Description: "Google's screen-optimised neo-grotesque; ideal primary UI font.",
		License:     "Apache-2.0",
		Source:      "https://github.com/google/roboto",
		Files: []FontFile{
			{Regular, "Roboto-Regular.ttf"},
			{Bold, "Roboto-Bold.ttf"},
			{Italic, "Roboto-Italic.ttf"},
			{BoldItalic, "Roboto-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Arimo",
		Category:    "sans",
		Description: "Arial-metric-compatible sans; clean, corporate, native on desktop.",
		License:     "Apache-2.0",
		Source:      "https://github.com/google/fonts/tree/main/apache/arimo",
		Files: []FontFile{
			{Regular, "Arimo-Regular.ttf"},
			{Bold, "Arimo-Bold.ttf"},
			{Italic, "Arimo-Italic.ttf"},
			{BoldItalic, "Arimo-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Cousine",
		Category:    "mono",
		Description: "Monospaced companion to Arimo for code, logs, and commit hashes.",
		License:     "Apache-2.0",
		Source:      "https://github.com/google/fonts/tree/main/apache/cousine",
		Files: []FontFile{
			{Regular, "Cousine-Regular.ttf"},
			{Bold, "Cousine-Bold.ttf"},
			{Italic, "Cousine-Italic.ttf"},
			{BoldItalic, "Cousine-BoldItalic.ttf"},
		},
	},
	{
		Name:        "Ledger",
		Category:    "serif",
		Description: "Modern business serif with strong on-screen legibility.",
		License:     "OFL-1.1",
		Source:      "https://github.com/google/fonts/tree/main/ofl/ledger",
		Files: []FontFile{
			{Regular, "Ledger-Regular.ttf"},
		},
	},
}

// DefaultFamily is the family used when the user hasn't chosen one and
// it is available. Liberation Sans is the safe professional default
// (Arial-metric-compatible).
const DefaultFamily = "Liberation Sans"

// CatalogFamily looks up a family by name (case-insensitive). Returns
// (zero, false) if not found.
func CatalogFamily(name string) (FontFamily, bool) {
	for _, f := range Catalog {
		if strings.EqualFold(f.Name, name) {
			return f, true
		}
	}
	return FontFamily{}, false
}

// FamilyNames returns every bundled family name in catalog order.
func FamilyNames() []string {
	out := make([]string, 0, len(Catalog))
	for _, f := range Catalog {
		out = append(out, f.Name)
	}
	return out
}
