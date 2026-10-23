// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"encoding/binary"
	"testing"
)

// cmapFormat4 returns the BMP character-to-glyph map of a TrueType font's
// Windows Unicode (platform 3, encoding 1) format 4 subtable, the only
// character map go-pdf/fpdf reads.
func cmapFormat4(t *testing.T, font []byte) map[rune]uint16 {
	t.Helper()
	be16 := func(b []byte, off int) int { return int(binary.BigEndian.Uint16(b[off:])) }
	numTables := be16(font, 4)
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if string(font[rec:rec+4]) != "cmap" {
			continue
		}
		cmap := font[binary.BigEndian.Uint32(font[rec+8:]):]
		for j := 0; j < be16(cmap, 2); j++ {
			enc := cmap[4+8*j:]
			if be16(enc, 0) != 3 || be16(enc, 2) != 1 {
				continue
			}
			sub := cmap[binary.BigEndian.Uint32(enc[4:]):]
			if be16(sub, 0) != 4 {
				continue
			}
			segs := be16(sub, 6) / 2
			ends, starts := 14, 16+2*segs
			deltas, offsets := starts+2*segs, starts+4*segs
			out := map[rune]uint16{}
			for k := 0; k < segs; k++ {
				end, start := be16(sub, ends+2*k), be16(sub, starts+2*k)
				delta, ro := be16(sub, deltas+2*k), be16(sub, offsets+2*k)
				for c := start; c <= end && c != 0xFFFF; c++ {
					var g int
					if ro == 0 {
						g = (c + delta) & 0xFFFF
					} else {
						at := offsets + 2*k + ro + 2*(c-start)
						if g = be16(sub, at); g != 0 {
							g = (g + delta) & 0xFFFF
						}
					}
					if g != 0 {
						out[rune(c)] = uint16(g)
					}
				}
			}
			return out
		}
	}
	return nil
}

// TestCommittedFonts_MapTheirScriptsForThePDFEngine reads each committed
// face the way fpdf does. IBM Plex Sans TC is the reason: its TrueType
// files leave this map empty, so fpdf drew every character, Latin
// included, as a missing glyph. A family joins the catalog only if fpdf
// can map the text it exists for.
func TestCommittedFonts_MapTheirScriptsForThePDFEngine(t *testing.T) {
	latin := "Project charter 2026 — €1,250"
	samples := map[string]string{
		DefaultFamily:             latin,
		MonoFamily:                latin,
		"IBM Plex Serif":          latin,
		"IBM Plex Sans Condensed": latin,
		"IBM Plex Sans JP":        latin + "プロジェクト憲章、日本語のテキスト",
		"IBM Plex Sans KR":        latin + "프로젝트 헌장, 한국어 텍스트",
		"IBM Plex Sans SC":        latin + "项目章程，简体中文文本",
		"Source Sans 3":           latin,
	}
	for family, text := range samples {
		fam, ok := CatalogFamily(family)
		if !ok {
			t.Fatalf("%s is not in the catalog", family)
		}
		for _, ff := range fam.Files {
			b, err := assetsFS.ReadFile(bundledAsset(ff.FileName))
			if err != nil {
				t.Fatalf("read %s: %v", ff.FileName, err)
			}
			cmap := cmapFormat4(t, b)
			if len(cmap) == 0 {
				t.Errorf("%s has no cmap format 4 entries; fpdf would draw no text with it", ff.FileName)
				continue
			}
			for _, r := range text {
				if r == ' ' {
					continue
				}
				if _, ok := cmap[r]; !ok {
					t.Errorf("%s does not map %q (U+%04X) for the PDF engine", ff.FileName, r, r)
				}
			}
		}
	}
}
