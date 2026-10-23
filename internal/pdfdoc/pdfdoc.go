// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pdfdoc is the drawing surface for GoPMgr's PDF renderers. It wraps
// go-pdf/fpdf so that every string reaches fpdf in a form its embedded
// TrueType fonts can encode.
//
// fpdf writes TrueType text as 16-bit codes, so it cannot show characters
// beyond U+FFFF, such as most emoji. Wrapped text fails the whole export
// with "character outside the supported range", and single-line text
// crashes fpdf while it writes the file (an index out of range in its
// CIDToGIDMap). PDF overrides each fpdf method that draws or measures text
// and passes the text through SafeText first. Renderers take a *PDF, never a
// bare *fpdf.Fpdf, so no text can skip that step; boundary_test.go enforces
// this.
package pdfdoc

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
)

// Replacement is the character drawn in place of text the PDF fonts cannot
// encode: U+FFFD REPLACEMENT CHARACTER. Every bundled IBM Plex face draws
// it, and it copies out of the PDF as itself, so a reader can tell that
// something was there.
const Replacement = '�'

// PDF is an fpdf document whose text methods accept any Unicode text.
// Every other fpdf method is available unchanged through the embedded
// *fpdf.Fpdf.
type PDF struct {
	*fpdf.Fpdf
}

// New returns a document with the same arguments as fpdf.New.
func New(orientationStr, unitStr, sizeStr, fontDirStr string) *PDF {
	return &PDF{Fpdf: fpdf.New(orientationStr, unitStr, sizeStr, fontDirStr)}
}

// SafeText returns s with the characters fpdf cannot encode handled:
//
//   - Each emoji or other character beyond U+FFFF becomes one Replacement,
//     together with the joiners, skin-tone modifiers, tag characters, and
//     second flag letter that belong to it, so a family emoji or a flag
//     yields one mark rather than several.
//   - Variation selectors (U+FE0E, U+FE0F) and the combining keycap
//     (U+20E3) are removed, so "1\uFE0F\u20E3" prints as "1".
//   - U+FEFF and the noncharacters U+FFFE and U+FFFF are removed; they are
//     invisible, and PDF/A forbids mapping text to them.
//
// Invalid UTF-8 becomes Replacement. Everything else passes through
// unchanged, including characters the current font has no glyph for.
func SafeText(s string) string {
	if !needsCleaning(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	inEmoji := false  // the last rune written was a Replacement for an emoji
	joinNext := false // a zero-width joiner continues the current emoji
	openFlag := false // the current emoji is a lone regional indicator
	for _, r := range s {
		switch {
		case r > 0xFFFF:
			switch {
			case inEmoji && (joinNext || isEmojiModifier(r)):
				joinNext = false
				continue
			case inEmoji && openFlag && isRegionalIndicator(r):
				openFlag = false
				continue
			}
			b.WriteRune(Replacement)
			inEmoji, joinNext, openFlag = true, false, isRegionalIndicator(r)
		case r == '\u200D' && inEmoji:
			joinNext = true
		case isInvisibleMark(r):
			// Dropped. These attach to the preceding character, so they
			// neither start nor end an emoji.
		case inEmoji && joinNext:
			// A joiner binds this character into the emoji, as in a
			// transgender flag, which ends in U+26A7.
			joinNext = false
		default:
			b.WriteRune(r)
			inEmoji, joinNext, openFlag = false, false, false
		}
	}
	return b.String()
}

func needsCleaning(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || r > 0xFFFF || isInvisibleMark(r) {
			return true
		}
		i += size
	}
	return false
}

func isInvisibleMark(r rune) bool {
	switch r {
	case '\uFE0E', '\uFE0F', '\u20E3', '\uFEFF', '\uFFFE', '\uFFFF':
		return true
	}
	return false
}

// isEmojiModifier reports the supplementary-plane characters that extend
// the emoji before them: skin-tone modifiers and tag characters (used in
// subdivision flags).
func isEmojiModifier(r rune) bool {
	return (r >= 0x1F3FB && r <= 0x1F3FF) || (r >= 0xE0020 && r <= 0xE007F)
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

// Text-drawing and text-measuring methods. Each passes its text through
// SafeText and then calls the fpdf method of the same name.

// Cell draws a single-line cell; see fpdf.Fpdf.Cell.
func (p *PDF) Cell(w, h float64, txtStr string) {
	p.Fpdf.Cell(w, h, SafeText(txtStr))
}

// Cellf draws a single-line cell with formatted text; see fpdf.Fpdf.Cellf.
func (p *PDF) Cellf(w, h float64, fmtStr string, args ...interface{}) {
	p.Fpdf.Cell(w, h, SafeText(fmt.Sprintf(fmtStr, args...)))
}

// CellFormat draws a single-line cell with borders, alignment, fill, and a
// link; see fpdf.Fpdf.CellFormat.
func (p *PDF) CellFormat(w, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) {
	p.Fpdf.CellFormat(w, h, SafeText(txtStr), borderStr, ln, alignStr, fill, link, linkStr)
}

// MultiCell draws wrapped text; see fpdf.Fpdf.MultiCell.
func (p *PDF) MultiCell(w, h float64, txtStr, borderStr, alignStr string, fill bool) {
	p.Fpdf.MultiCell(w, h, SafeText(txtStr), borderStr, alignStr, fill)
}

// Text draws text at a position; see fpdf.Fpdf.Text.
func (p *PDF) Text(x, y float64, txtStr string) {
	p.Fpdf.Text(x, y, SafeText(txtStr))
}

// ClipText starts a clipping region shaped like text; see fpdf.Fpdf.ClipText.
func (p *PDF) ClipText(x, y float64, txtStr string, outline bool) {
	p.Fpdf.ClipText(x, y, SafeText(txtStr), outline)
}

// Write flows text from the current position; see fpdf.Fpdf.Write.
func (p *PDF) Write(h float64, txtStr string) {
	p.Fpdf.Write(h, SafeText(txtStr))
}

// Writef flows formatted text from the current position; see fpdf.Fpdf.Writef.
func (p *PDF) Writef(h float64, fmtStr string, args ...interface{}) {
	p.Fpdf.Write(h, SafeText(fmt.Sprintf(fmtStr, args...)))
}

// WriteAligned flows aligned text; see fpdf.Fpdf.WriteAligned.
func (p *PDF) WriteAligned(width, lineHeight float64, textStr, alignStr string) {
	p.Fpdf.WriteAligned(width, lineHeight, SafeText(textStr), alignStr)
}

// WriteLinkString flows text linked to a URL; see fpdf.Fpdf.WriteLinkString.
func (p *PDF) WriteLinkString(h float64, displayStr, targetStr string) {
	p.Fpdf.WriteLinkString(h, SafeText(displayStr), targetStr)
}

// WriteLinkID flows text linked to an internal link; see fpdf.Fpdf.WriteLinkID.
func (p *PDF) WriteLinkID(h float64, displayStr string, linkID int) {
	p.Fpdf.WriteLinkID(h, SafeText(displayStr), linkID)
}

// SubWrite flows subscript or superscript text; see fpdf.Fpdf.SubWrite.
func (p *PDF) SubWrite(ht float64, str string, subFontSize, subOffset float64, link int, linkStr string) {
	p.Fpdf.SubWrite(ht, SafeText(str), subFontSize, subOffset, link, linkStr)
}

// GetStringWidth measures text in the current font; see
// fpdf.Fpdf.GetStringWidth. It measures the text as it will be drawn.
func (p *PDF) GetStringWidth(s string) float64 {
	return p.Fpdf.GetStringWidth(SafeText(s))
}

// GetStringSymbolWidth measures text in font units; see
// fpdf.Fpdf.GetStringSymbolWidth.
func (p *PDF) GetStringSymbolWidth(s string) int {
	return p.Fpdf.GetStringSymbolWidth(SafeText(s))
}

// SplitText splits text into lines that fit width w; see
// fpdf.Fpdf.SplitText. The lines hold the text as it will be drawn.
func (p *PDF) SplitText(txt string, w float64) []string {
	return p.Fpdf.SplitText(SafeText(txt), w)
}

// SplitLines splits text into lines that fit width w; see
// fpdf.Fpdf.SplitLines. The lines hold the text as it will be drawn.
func (p *PDF) SplitLines(txt []byte, w float64) [][]byte {
	return p.Fpdf.SplitLines([]byte(SafeText(string(txt))), w)
}
