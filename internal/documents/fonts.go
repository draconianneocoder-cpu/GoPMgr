// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"sync"

	"github.com/go-pdf/fpdf"

	"pmforge/internal/fonts"
	"pmforge/internal/pdfmeta"
)

// Font integration.
//
// Every document renderer draws text with SetFont("Helvetica", ...).
// fpdf's AddUTF8FontFromBytes overrides a core-font family name when
// you register an embedded TrueType font under that name, so PMForge
// can swap the document font for ALL renderers at once by registering
// the user's chosen family under "Helvetica" on each new PDF — no
// per-renderer changes beyond using newDocPDF instead of fpdf.New.
//
// The App configures the applier at startup / login / font-change via
// SetFontApplier. When no applier is set (or the chosen font isn't
// available), renderers fall back to fpdf's built-in Helvetica, so
// document export always works.

var (
	fontMu      sync.RWMutex
	fontApplier func(*fpdf.Fpdf)
)

// SetFontApplier installs the hook that registers the active embedded
// font on each new document PDF. Pass nil to revert to fpdf's core
// Helvetica. Safe for concurrent use (the Wails runtime may call this
// from a different goroutine than a render in flight).
func SetFontApplier(fn func(*fpdf.Fpdf)) {
	fontMu.Lock()
	fontApplier = fn
	fontMu.Unlock()
}

// UseFont configures every renderer to draw document text with the
// named font family, by registering that family under "Helvetica" on
// each new PDF (which overrides fpdf's core font of the same name).
// Pass an empty family or a nil manager to revert to the built-in
// Helvetica. This is the single entry point the App calls when a
// project opens or the user changes the default font.
func UseFont(mgr *fonts.Manager, family string) {
	if mgr == nil || family == "" {
		SetFontApplier(nil)
		return
	}
	SetFontApplier(func(pdf *fpdf.Fpdf) {
		// Best-effort: if the family can't be registered (e.g. its
		// .ttf files weren't fetched), the renderer keeps fpdf's
		// core Helvetica and still produces a valid PDF.
		_ = mgr.RegisterAs(pdf, family, "Helvetica")
	})
}

// newDocPDF creates a portrait/landscape A4 PDF and applies the active
// embedded font, if one is configured. Renderers use this in place of
// fpdf.New so the user's font choice takes effect everywhere.
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

	// Apply PDF/A-3 metadata setters early. Individual renderers can
	// still override Title/Author/Subject with more specific values.
	pdfmeta.ApplyPDFAMetadata(pdf, pdfmeta.XMPSpec{
		Author:      "PMForge",
		CreatorTool: "PMForge",
	})

	return pdf
}
