// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfdoc

import (
	"bytes"
	"testing"
	"unicode/utf16"

	"gopmgr/internal/fonts"
)

func TestSafeText(t *testing.T) {
	const r = "�"
	for _, tc := range []struct {
		name, in, want string
	}{
		{"plain text", "Phase 2 — “Approved” • €1,250", "Phase 2 — “Approved” • €1,250"},
		{"empty", "", ""},
		{"scripts and CJK pass through", "Привет Ω 東京 नमस\u094Dत\u0947", "Привет Ω 東京 नमस\u094Dत\u0947"},
		{"joiner outside an emoji is kept", "क\u094D\u200Dष", "क\u094D\u200Dष"},
		{"single emoji", "Launch 🚀 day", "Launch " + r + " day"},
		{"adjacent emoji", "🚀🎉", r + r},
		{"skin tone", "👍🏽 done", r + " done"},
		{"joiner sequence", "👩\u200D💻 dev", r + " dev"},
		{"joiner sequence ending in BMP", "🏳\uFE0F\u200D⚧\uFE0F flag", r + " flag"},
		{"flag", "🇺🇸 office", r + " office"},
		{"two flags", "🇺🇸🇩🇪", r + r},
		{"subdivision flag", "🏴\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F", r},
		{"BMP emoji keeps its glyph", "❤\uFE0F", "❤"},
		{"keycap", "1\uFE0F\u20E3 Kickoff", "1 Kickoff"},
		{"BOM and noncharacters", "a\uFEFFb\uFFFEc\uFFFFd", "abcd"},
		{"invalid UTF-8 becomes a replacement", "a\xffb", "a" + r + "b"},
	} {
		if got := SafeText(tc.in); got != tc.want {
			t.Errorf("%s: SafeText(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSafeText_DoesNotAllocateForCleanText(t *testing.T) {
	s := "Critical path: Design → Build → Test"
	if n := testing.AllocsPerRun(100, func() { _ = SafeText(s) }); n != 0 {
		t.Fatalf("SafeText allocated %v times for text that needs no change", n)
	}
}

// newPlexPDF returns an uncompressed document using IBM Plex Sans, so tests
// can read its content streams.
func newPlexPDF(t *testing.T) *PDF {
	t.Helper()
	pdf := New("P", "mm", "A4", "")
	if err := fonts.NewManager("").RegisterAs(pdf.Fpdf, fonts.DefaultFamily, "Helvetica"); err != nil {
		t.Fatalf("RegisterAs: %v", err)
	}
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 11)
	return pdf
}

func utf16BE(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u>>8), byte(u))
	}
	return b
}

// TestPDF_TextMethodsAcceptEmoji drives each text method with an emoji.
// Through bare fpdf, Cell panics while the file is written and MultiCell
// fails the export; through PDF both succeed and draw U+FFFD.
func TestPDF_TextMethodsAcceptEmoji(t *testing.T) {
	const text = "Go-live 🚀 approved"
	for name, draw := range map[string]func(*PDF){
		"Cell":            func(p *PDF) { p.Cell(0, 8, text) },
		"Cellf":           func(p *PDF) { p.Cellf(0, 8, "%s", text) },
		"CellFormat":      func(p *PDF) { p.CellFormat(0, 8, text, "1", 1, "L", false, 0, "") },
		"MultiCell":       func(p *PDF) { p.MultiCell(60, 6, text+" "+text, "", "L", false) },
		"Text":            func(p *PDF) { p.Text(20, 20, text) },
		"Write":           func(p *PDF) { p.Write(6, text) },
		"Writef":          func(p *PDF) { p.Writef(6, "%s", text) },
		"WriteAligned":    func(p *PDF) { p.WriteAligned(0, 6, text, "C") },
		"WriteLinkString": func(p *PDF) { p.WriteLinkString(6, text, "https://example.com") },
		"WriteLinkID":     func(p *PDF) { p.WriteLinkID(6, text, p.AddLink()) },
		"SubWrite":        func(p *PDF) { p.SubWrite(6, text, 8, 2, 0, "") },
		"ClipText":        func(p *PDF) { p.ClipText(20, 40, text, false); p.ClipEnd() },
		"SplitText": func(p *PDF) {
			for _, line := range p.SplitText(text+" "+text, 40) {
				p.Cell(0, 6, line)
			}
		},
		"SplitLines": func(p *PDF) {
			for _, line := range p.SplitLines([]byte(text+" "+text), 40) {
				p.Cell(0, 6, string(line))
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			pdf := newPlexPDF(t)
			draw(pdf)
			var buf bytes.Buffer
			if err := pdf.Output(&buf); err != nil {
				t.Fatalf("Output: %v", err)
			}
			if name != "ClipText" && !bytes.Contains(buf.Bytes(), utf16BE("Go-live �")) {
				t.Fatal("content does not show the emoji as U+FFFD")
			}
		})
	}
}

func TestPDF_MeasuresTextAsDrawn(t *testing.T) {
	pdf := newPlexPDF(t)
	if got, want := pdf.GetStringWidth("A 🚀"), pdf.GetStringWidth("A �"); got != want {
		t.Fatalf("GetStringWidth(emoji) = %v, want the drawn width %v", got, want)
	}
	if got, want := pdf.GetStringSymbolWidth("A 🚀"), pdf.GetStringSymbolWidth("A �"); got != want {
		t.Fatalf("GetStringSymbolWidth(emoji) = %v, want %v", got, want)
	}
}
