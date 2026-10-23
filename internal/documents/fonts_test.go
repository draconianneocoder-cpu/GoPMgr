// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"

	"gopmgr/internal/fonts"
)

// fontSample mixes letters, tabular figures, and punctuation, so faces with
// different widths cannot measure the same by coincidence.
const fontSample = "Earned value 1,284,500.00 — Phase 2 (WBS 1.2.3)"

// withDefaultDocumentFont starts a test from the production font state
// (TestMain selects core Helvetica for this package) and restores the
// previous state afterwards.
func withDefaultDocumentFont(t *testing.T) {
	t.Helper()
	fontMu.Lock()
	prevApplier, prevFamily := fontApplier, activeFamily
	fontApplier, activeFamily = defaultFontApplier, fonts.DefaultFamily
	fontMu.Unlock()
	t.Cleanup(func() {
		fontMu.Lock()
		fontApplier, activeFamily = prevApplier, prevFamily
		fontMu.Unlock()
	})
}

// documentFontWidth measures s in the "Helvetica" family that newDocPDF
// registers, which every renderer draws with.
func documentFontWidth(t *testing.T, style, s string) float64 {
	t.Helper()
	pdf := newDocPDF("P")
	pdf.AddPage()
	pdf.SetFont("Helvetica", style, 10)
	if err := pdf.Error(); err != nil {
		t.Fatalf("set document font: %v", err)
	}
	return pdf.GetStringWidth(s)
}

// bundledFamilyWidth measures s in a bundled family registered directly.
func bundledFamilyWidth(t *testing.T, family, style, s string) float64 {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	if err := fonts.NewManager("").RegisterAs(pdf, family, "Probe"); err != nil {
		t.Fatalf("register %s: %v", family, err)
	}
	pdf.AddPage()
	pdf.SetFont("Probe", style, 10)
	return pdf.GetStringWidth(s)
}

func coreHelveticaWidth(style, s string) float64 {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", style, 10)
	return pdf.GetStringWidth(s)
}

func assertDocumentFontIs(t *testing.T, family string) {
	t.Helper()
	for _, style := range []string{"", "B", "I", "BI"} {
		got, want := documentFontWidth(t, style, fontSample), bundledFamilyWidth(t, family, style, fontSample)
		if got != want {
			t.Errorf("style %q: document text measures %.3f mm, want %.3f mm (%s)", style, got, want, family)
		}
	}
}

// Documents embed IBM Plex Sans before any project chooses a font, in every
// style, and never name fpdf's unembedded core Helvetica.
func TestNewDocPDF_EmbedsIBMPlexSansByDefault(t *testing.T) {
	withDefaultDocumentFont(t)

	assertDocumentFontIs(t, fonts.DefaultFamily)
	if got := ActiveFamily(); got != fonts.DefaultFamily {
		t.Errorf("ActiveFamily() = %q, want %q", got, fonts.DefaultFamily)
	}
	if documentFontWidth(t, "", fontSample) == coreHelveticaWidth("", fontSample) {
		t.Fatal("document text measures like core Helvetica; the comparison cannot tell the fonts apart")
	}

	pdf, err := RenderCharterPDF(map[string]interface{}{"purpose": "Deliver the new site."}, "Plex Project")
	if err != nil {
		t.Fatalf("RenderCharterPDF: %v", err)
	}
	if !bytes.Contains(pdf, []byte("/FontFile2")) {
		t.Error("charter PDF embeds no TrueType font program")
	}
	if bytes.Contains(pdf, []byte("/BaseFont /Helvetica")) {
		t.Error("charter PDF references the unembedded core Helvetica")
	}
}

// An empty or unknown family selects IBM Plex Sans rather than leaving
// documents on core Helvetica.
func TestUseFont_UnavailableFamilySelectsDefault(t *testing.T) {
	cases := []struct {
		name   string
		mgr    *fonts.Manager
		family string
	}{
		{"no project font", nil, ""},
		{"unknown family", fonts.NewManager(t.TempDir()), "No Such Font"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withDefaultDocumentFont(t)
			UseFont(tc.mgr, tc.family)
			if got := ActiveFamily(); got != fonts.DefaultFamily {
				t.Errorf("ActiveFamily() = %q, want %q", got, fonts.DefaultFamily)
			}
			assertDocumentFontIs(t, fonts.DefaultFamily)
		})
	}
}

// A bundled family is selected by name in any letter case, and ActiveFamily
// reports the catalog spelling that DOCX and ODT exports write.
func TestUseFont_SelectsBundledFamilyByCanonicalName(t *testing.T) {
	withDefaultDocumentFont(t)
	UseFont(nil, "ibm plex serif")
	if got := ActiveFamily(); got != "IBM Plex Serif" {
		t.Errorf("ActiveFamily() = %q, want %q", got, "IBM Plex Serif")
	}
	assertDocumentFontIs(t, "IBM Plex Serif")
}

// A user font that disappears after it was selected (its file deleted while
// the project is open) falls back to IBM Plex Sans instead of core
// Helvetica.
func TestUseFont_MissingUserFontFallsBackToDefault(t *testing.T) {
	withDefaultDocumentFont(t)
	ttf, err := os.ReadFile(filepath.Join("..", "fonts", "assets", "IBMPlexMono-Regular.ttf"))
	if err != nil {
		t.Fatalf("read a committed TrueType file to import: %v", err)
	}
	userDir := t.TempDir()
	userFont := filepath.Join(userDir, "Acme-Regular.ttf")
	if err := os.WriteFile(userFont, ttf, 0o600); err != nil {
		t.Fatalf("write user font: %v", err)
	}

	UseFont(fonts.NewManager(userDir), "acme")
	if got := ActiveFamily(); got != "Acme" {
		t.Fatalf("ActiveFamily() = %q, want %q", got, "Acme")
	}
	if got, want := documentFontWidth(t, "", fontSample), bundledFamilyWidth(t, fonts.MonoFamily, "", fontSample); got != want {
		t.Fatalf("selected user font measures %.3f mm, want %.3f mm", got, want)
	}

	if err := os.Remove(userFont); err != nil {
		t.Fatalf("remove user font: %v", err)
	}
	if got, want := documentFontWidth(t, "", fontSample), bundledFamilyWidth(t, fonts.DefaultFamily, "", fontSample); got != want {
		t.Errorf("after the user font disappeared, text measures %.3f mm, want IBM Plex Sans %.3f mm", got, want)
	}
}

var pdfShownText = regexp.MustCompile(`BT ([0-9.]+) [0-9.]+ Td \(((?:\\.|[^\\)])*)\)\s*Tj`)

// textX returns the x position, in mm, where s is drawn in an uncompressed
// PDF.
func textX(t *testing.T, pdf []byte, s string) float64 {
	t.Helper()
	unescape := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`, `\r`, "\r")
	for _, m := range pdfShownText.FindAllSubmatch(pdf, -1) {
		if decodePDFText(unescape.Replace(string(m[2]))) == s {
			x, err := strconv.ParseFloat(string(m[1]), 64)
			if err != nil {
				t.Fatalf("parse x %q: %v", m[1], err)
			}
			return x * 25.4 / 72
		}
	}
	t.Fatalf("text %q not drawn", s)
	return 0
}

// A key/value label never runs into its value, in any bundled family. The
// label column is a fixed width that Helvetica Bold happened to fit, and
// IBM Plex Serif SemiBold overflowed it, so the budget label covered its
// first digit.
func TestKeyValueLabels_NeverOverlapTheirValues(t *testing.T) {
	const label, value = "High-level budget (USD)", "1284500.00"
	writers := map[string]struct {
		write func(*fpdf.Fpdf, string, string)
		size  float64
	}{
		"writeKV":        {writeKV, 10},
		"writeKVCompact": {writeKVCompact, 9},
	}
	for _, family := range []string{fonts.DefaultFamily, "IBM Plex Serif", "IBM Plex Sans Condensed", fonts.MonoFamily, "Source Sans 3"} {
		for name, w := range writers {
			t.Run(family+"/"+name, func(t *testing.T) {
				withDefaultDocumentFont(t)
				UseFont(nil, family)
				pdf := newDocPDF("P")
				pdf.SetCompression(false)
				pdf.AddPage()
				w.write(pdf, label, value)
				pdf.SetFont("Helvetica", "B", w.size)
				labelWidth := pdf.GetStringWidth(label + ":")
				var out bytes.Buffer
				if err := pdf.Output(&out); err != nil {
					t.Fatalf("output: %v", err)
				}
				labelEnd := textX(t, out.Bytes(), label+":") + labelWidth
				if gap := textX(t, out.Bytes(), value) - labelEnd; gap < 1 {
					t.Errorf("value starts %.2f mm after the label ends, want at least 1 mm", gap)
				}
			})
		}
	}
}

// Labels that fit keep the aligned column, so short labels still line up.
func TestKVLabelWidth_KeepsTheColumnForShortLabels(t *testing.T) {
	withDefaultDocumentFont(t)
	pdf := newDocPDF("P")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 10)
	if got := kvLabelWidth(pdf, "Sponsor:", 40); got != 40 {
		t.Errorf("kvLabelWidth(short label) = %v, want the 40 mm column", got)
	}
}
