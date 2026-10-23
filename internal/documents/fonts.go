// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"strings"
	"sync"

	"github.com/go-pdf/fpdf"

	"gopmgr/internal/fonts"
	"gopmgr/internal/pdfmeta"
)

// Font integration.
//
// Every document renderer draws text with SetFont("Helvetica", ...).
// fpdf's AddUTF8FontFromBytes overrides a core-font family name when
// you register an embedded TrueType font under that name, so GoPMgr
// can swap the document font for ALL renderers at once by registering
// the chosen family under "Helvetica" on each new PDF — no per-renderer
// changes beyond using newDocPDF instead of fpdf.New.
//
// By default every document embeds fonts.DefaultFamily (IBM Plex Sans),
// which every build bundles. The App selects the project's font with
// UseFont when a project opens or the user changes it; a family that is
// not available falls back to the default, so documents never depend on
// fpdf's unembedded core Helvetica.

var (
	fontMu       sync.RWMutex
	fontApplier  = defaultFontApplier
	activeFamily = fonts.DefaultFamily
)

// defaultFontApplier embeds the bundled default family.
func defaultFontApplier(pdf *fpdf.Fpdf) {
	registerDocumentFont(pdf, fonts.NewManager(""), fonts.DefaultFamily)
}

// registerDocumentFont registers family under "Helvetica", falling back to
// the bundled default family if none of its faces can be registered.
func registerDocumentFont(pdf *fpdf.Fpdf, mgr *fonts.Manager, family string) {
	if err := mgr.RegisterAs(pdf, family, "Helvetica"); err == nil || family == fonts.DefaultFamily {
		return
	}
	_ = fonts.NewManager("").RegisterAs(pdf, fonts.DefaultFamily, "Helvetica")
}

// SetFontApplier installs the hook that registers the embedded font on
// each new document PDF. Passing nil leaves documents on fpdf's core
// Helvetica, which is not embedded; only tests that measure core-font
// output do that. It does not change ActiveFamily. Safe for concurrent use
// (the Wails runtime may call this from a different goroutine than a
// render in flight).
func SetFontApplier(fn func(*fpdf.Fpdf)) {
	fontMu.Lock()
	fontApplier = fn
	fontMu.Unlock()
}

// UseFont configures every renderer to draw document text with the named
// font family, by registering that family under "Helvetica" on each new PDF
// (which overrides fpdf's core font of the same name). An empty family, or
// one that mgr does not offer, selects fonts.DefaultFamily; a nil manager
// offers only the bundled families. This is the single entry point the App
// calls when a project opens or closes, or the user changes the font.
func UseFont(mgr *fonts.Manager, family string) {
	if mgr == nil {
		mgr = fonts.NewManager("")
	}
	family, ok := availableFamily(mgr, family)
	if !ok {
		family = fonts.DefaultFamily
	}
	fontMu.Lock()
	activeFamily = family
	fontApplier = func(pdf *fpdf.Fpdf) { registerDocumentFont(pdf, mgr, family) }
	fontMu.Unlock()
}

// ActiveFamily is the family that UseFont selected for document exports
// (fonts.DefaultFamily until UseFont chooses another). The DOCX and ODT
// exports name it so they match the project's PDFs.
func ActiveFamily() string {
	fontMu.RLock()
	defer fontMu.RUnlock()
	return activeFamily
}

// availableFamily returns mgr's spelling of family when mgr offers it.
func availableFamily(mgr *fonts.Manager, family string) (string, bool) {
	if family == "" {
		return "", false
	}
	for _, info := range mgr.Available() {
		if strings.EqualFold(info.Name, family) {
			return info.Name, true
		}
	}
	return "", false
}

// newDocPDF creates a portrait/landscape A4 PDF and applies the active
// embedded font. Renderers use this in place of fpdf.New so the font
// choice takes effect everywhere.
//
// orientation is "P" (portrait) or "L" (landscape), matching the
// fpdf.New first argument the renderers previously passed.
func newDocPDF(orientation string) *fpdf.Fpdf {
	pdf := fpdf.New(orientation, "mm", "A4", "")
	fontMu.RLock()
	fn := fontApplier
	fontMu.RUnlock()
	if fn != nil {
		fn(pdf)
	}

	// Apply PDF/A metadata setters early. Individual renderers can
	// still override Title/Author/Subject with more specific values.
	pdfmeta.ApplyPDFAMetadata(pdf, pdfmeta.XMPSpec{
		Author:      "GoPMgr",
		CreatorTool: "GoPMgr",
	})

	return pdf
}
