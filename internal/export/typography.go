// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gomutex/godocx/docx"

	"gopmgr/internal/fonts"
)

// Typography for the editable and web export formats. These formats name a
// font but do not embed it, so each one also says what kind of font it is
// (sans-serif, serif, or monospaced), which Word and LibreOffice use to pick
// a close substitute when the recipient does not have it installed. See
// docs/design/typography-ibm-plex.md.

// officeFontClass is a font's generic family for office formats: the DOCX
// w:family value, the ODT style:font-family-generic value (empty when it
// has none), and whether the font is monospaced.
type officeFontClass struct {
	docxFamily string
	odtGeneric string
	fixedPitch bool
}

func fontClass(family string) officeFontClass {
	fam, ok := fonts.CatalogFamily(family)
	if !ok {
		// A user-imported font: its kind is unknown.
		return officeFontClass{docxFamily: "auto"}
	}
	switch fam.Category {
	case "serif":
		return officeFontClass{docxFamily: "roman", odtGeneric: "roman"}
	case "mono":
		return officeFontClass{docxFamily: "modern", odtGeneric: "modern", fixedPitch: true}
	default:
		return officeFontClass{docxFamily: "swiss", odtGeneric: "swiss"}
	}
}

func (c officeFontClass) pitch() string {
	if c.fixedPitch {
		return "fixed"
	}
	return "variable"
}

const docxThemePart = "word/theme/theme1.xml"
const docxFontTablePart = "word/fontTable.xml"

// themeLatinFont matches the Latin typeface of the theme's heading (major)
// and body (minor) fonts, which the template's text, heading, and table
// styles all reference.
var themeLatinFont = regexp.MustCompile(`(<a:(?:major|minor)Font>\s*<a:latin typeface=")[^"]*"`)

// applyDOCXFont sets a godocx document's heading and body fonts to family,
// moves the template's monospaced style to fonts.MonoFamily, and lists both
// in the font table with their generic family for substitution.
func applyDOCXFont(doc *docx.RootDoc, family string) error {
	raw, ok := doc.FileMap.Load(docxThemePart)
	if !ok {
		return errors.New("export: DOCX template has no theme part")
	}
	theme, ok := raw.([]byte)
	if !ok {
		return errors.New("export: DOCX theme part is not XML bytes")
	}
	if n := len(themeLatinFont.FindAllIndex(theme, -1)); n != 2 {
		return fmt.Errorf("export: DOCX theme has %d Latin heading/body fonts, want 2", n)
	}
	doc.FileMap.Store(docxThemePart, themeLatinFont.ReplaceAll(theme, []byte("${1}"+xmlEscape(family)+`"`)))

	if doc.DocStyles != nil {
		for i := range doc.DocStyles.StyleList {
			rp := doc.DocStyles.StyleList[i].RunProp
			if rp == nil || rp.Fonts == nil || !isCourier(rp.Fonts.Ascii) {
				continue
			}
			rp.Fonts.Ascii, rp.Fonts.HAnsi = fonts.MonoFamily, fonts.MonoFamily
		}
	}

	raw, ok = doc.FileMap.Load(docxFontTablePart)
	if !ok {
		return errors.New("export: DOCX template has no font table")
	}
	table, ok := raw.([]byte)
	if !ok {
		return errors.New("export: DOCX font table is not XML bytes")
	}
	for _, name := range []string{family, fonts.MonoFamily} {
		table, ok = addDOCXFontTableEntry(table, name)
		if !ok {
			return errors.New("export: DOCX font table has no closing </w:fonts>")
		}
	}
	doc.FileMap.Store(docxFontTablePart, table)
	return nil
}

func isCourier(name string) bool {
	return strings.EqualFold(name, "Courier") || strings.EqualFold(name, "Courier New")
}

// addDOCXFontTableEntry adds a w:font entry for name unless one exists.
func addDOCXFontTableEntry(table []byte, name string) ([]byte, bool) {
	escaped := xmlEscape(name)
	if bytes.Contains(table, []byte(`<w:font w:name="`+escaped+`"`)) {
		return table, true
	}
	end := bytes.LastIndex(table, []byte("</w:fonts>"))
	if end < 0 {
		return table, false
	}
	class := fontClass(name)
	entry := fmt.Sprintf(`<w:font w:name="%s"><w:family w:val="%s"/><w:pitch w:val="%s"/></w:font>`,
		escaped, class.docxFamily, class.pitch())
	out := make([]byte, 0, len(table)+len(entry))
	out = append(out, table[:end]...)
	out = append(out, entry...)
	return append(out, table[end:]...), true
}

// odtFontFaceDecls declares family for an ODT styles.xml.
func odtFontFaceDecls(family string) string {
	class := fontClass(family)
	generic := ""
	if class.odtGeneric != "" {
		generic = ` style:font-family-generic="` + class.odtGeneric + `"`
	}
	escaped := xmlEscape(family)
	return `  <office:font-face-decls>
    <style:font-face style:name="` + escaped + `" svg:font-family="&apos;` + escaped + `&apos;"` + generic + ` style:font-pitch="` + class.pitch() + `"/>
  </office:font-face-decls>
`
}

// htmlFontStack is the CSS font-family list for HTML exports.
const htmlFontStack = `"IBM Plex Sans", ui-sans-serif, system-ui, sans-serif`
