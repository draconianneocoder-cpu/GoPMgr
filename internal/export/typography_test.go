// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"gopmgr/internal/documents"
	"gopmgr/internal/fonts"
	"gopmgr/internal/kernel"
	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/sigma/domain"
)

// zipPart returns one part of a zipped office file, failing the test when it
// is missing or is not well-formed XML.
func zipPart(t *testing.T, file []byte, name string) string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		dec := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", name, err)
			}
		}
		return string(b)
	}
	t.Fatalf("zip has no %s part", name)
	return ""
}

// useDocumentFont selects a project document font for one test.
func useDocumentFont(t *testing.T, family string) {
	t.Helper()
	documents.UseFont(nil, family)
	t.Cleanup(func() { documents.UseFont(nil, "") })
}

var themeLatin = regexp.MustCompile(`<a:(major|minor)Font>\s*<a:latin typeface="([^"]*)"`)

func assertDOCXFont(t *testing.T, docx []byte, family, docxFamily string) {
	t.Helper()
	theme := zipPart(t, docx, "word/theme/theme1.xml")
	got := themeLatin.FindAllStringSubmatch(theme, -1)
	if len(got) != 2 {
		t.Fatalf("theme has %d heading/body Latin fonts, want 2", len(got))
	}
	for _, m := range got {
		if m[2] != family {
			t.Errorf("theme %sFont Latin typeface = %q, want %q", m[1], m[2], family)
		}
	}
	table := zipPart(t, docx, "word/fontTable.xml")
	for name, class := range map[string]string{family: docxFamily, fonts.MonoFamily: "modern"} {
		entry := regexp.MustCompile(`<w:font w:name="` + regexp.QuoteMeta(name) + `"><w:family w:val="` + class + `"/>`)
		if !entry.MatchString(table) {
			t.Errorf("font table has no %s entry with w:family %q", name, class)
		}
	}
	styles := zipPart(t, docx, "word/styles.xml")
	if strings.Contains(styles, `w:ascii="Courier"`) {
		t.Error("the template's monospaced style still names Courier")
	}
	if !strings.Contains(styles, `w:ascii="`+fonts.MonoFamily+`"`) {
		t.Errorf("no style names %s for monospaced text", fonts.MonoFamily)
	}
}

// Project documents exported as DOCX use the project's document font for
// headings and body text, IBM Plex Sans by default.
func TestRenderDocumentDOCX_UsesDocumentFont(t *testing.T) {
	content := `{"purpose":"Deliver the new site."}`
	t.Run("default", func(t *testing.T) {
		useDocumentFont(t, "")
		out, err := RenderDocumentDOCX(documents.KindBusinessCase, content, "Plex Project", "USD")
		if err != nil {
			t.Fatalf("RenderDocumentDOCX: %v", err)
		}
		assertDOCXFont(t, out, fonts.DefaultFamily, "swiss")
	})
	t.Run("chosen serif", func(t *testing.T) {
		useDocumentFont(t, "IBM Plex Serif")
		out, err := RenderDocumentDOCX(documents.KindBusinessCase, content, "Plex Project", "USD")
		if err != nil {
			t.Fatalf("RenderDocumentDOCX: %v", err)
		}
		assertDOCXFont(t, out, "IBM Plex Serif", "roman")
	})
}

// Schedule reports exported as DOCX always use IBM Plex Sans, like the PDF
// report, whatever the project's document font is.
func TestRenderReportDOCX_UsesBaselineFont(t *testing.T) {
	useDocumentFont(t, "IBM Plex Serif")
	out, err := renderDocumentDOCX(ReportPayload{Tasks: map[string]*kernel.Task{"t1": {ID: "t1", Title: "Task one"}}}, ExportOptions{Title: "Plex"})
	if err != nil {
		t.Fatalf("renderDocumentDOCX: %v", err)
	}
	assertDOCXFont(t, out, fonts.DefaultFamily, "swiss")
}

var odtFontFace = regexp.MustCompile(`<style:font-face style:name="([^"]+)" svg:font-family="&apos;([^&]+)&apos;"( style:font-family-generic="([a-z]+)")? style:font-pitch="(variable|fixed)"/>`)
var odtDefaultFont = regexp.MustCompile(`<style:default-style style:family="paragraph">\s*<style:text-properties style:font-name="([^"]+)"/>`)

func assertODTFont(t *testing.T, odt []byte, family, generic string) {
	t.Helper()
	styles := zipPart(t, odt, "styles.xml")
	face := odtFontFace.FindStringSubmatch(styles)
	if face == nil {
		t.Fatalf("styles.xml declares no font face:\n%s", styles)
	}
	if face[1] != family || face[2] != family || face[4] != generic || face[5] != "variable" {
		t.Errorf("font face = name %q, family %q, generic %q, pitch %q; want %q, %q, %q, variable", face[1], face[2], face[4], face[5], family, family, generic)
	}
	if m := odtDefaultFont.FindStringSubmatch(styles); m == nil || m[1] != family {
		t.Errorf("default paragraph font = %v, want %q", m, family)
	}
}

// Project documents exported as ODT use the project's document font, and
// reports use IBM Plex Sans.
func TestRenderODT_UsesDocumentAndReportFonts(t *testing.T) {
	useDocumentFont(t, "IBM Plex Serif")
	doc, err := RenderDocumentODT(documents.KindBusinessCase, `{"purpose":"Deliver the new site."}`, "Plex Project", "USD")
	if err != nil {
		t.Fatalf("RenderDocumentODT: %v", err)
	}
	assertODTFont(t, doc, "IBM Plex Serif", "roman")

	report, err := renderDocumentODT(ReportPayload{Tasks: map[string]*kernel.Task{"t1": {ID: "t1", Title: "Task one"}}}, ExportOptions{Title: "Plex"})
	if err != nil {
		t.Fatalf("renderDocumentODT: %v", err)
	}
	assertODTFont(t, report, fonts.DefaultFamily, "swiss")
}

// A user-imported font has no known kind, so office formats name it without
// a generic family rather than claiming one.
func TestFontClass_UnknownFamilyHasNoGeneric(t *testing.T) {
	c := fontClass("Acme Display")
	if c.docxFamily != "auto" || c.odtGeneric != "" || c.fixedPitch {
		t.Errorf("fontClass(user font) = %+v, want docx auto, no ODT generic, variable pitch", c)
	}
	if decls := odtFontFaceDecls("Acme Display"); strings.Contains(decls, "font-family-generic") {
		t.Errorf("ODT declaration claims a generic family for a user font: %s", decls)
	}
}

func TestRenderXLSX_DefaultFontIsIBMPlexSans(t *testing.T) {
	out, err := renderXLSX(ReportPayload{Tasks: map[string]*kernel.Task{"t1": {ID: "t1", Title: "Task one"}}}, ExportOptions{})
	if err != nil {
		t.Fatalf("renderXLSX: %v", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = book.Close() })
	if got, err := book.GetDefaultFont(); err != nil || got != fonts.DefaultFamily {
		t.Errorf("default font = %q, %v; want %q", got, err, fonts.DefaultFamily)
	}
}

func TestRenderHTML_UsesIBMPlexSans(t *testing.T) {
	out, err := renderHTML(ReportPayload{Tasks: map[string]*kernel.Task{"t1": {ID: "t1", Title: "Task one"}}}, ExportOptions{Title: "Plex"})
	if err != nil {
		t.Fatalf("renderHTML: %v", err)
	}
	if !bytes.Contains(out, []byte(`body { font-family: "IBM Plex Sans", `)) {
		t.Error("HTML report does not set IBM Plex Sans first in its font stack")
	}
	if !bytes.Contains(out, []byte(`th { background: #f1f5f9; font-weight: 600; }`)) {
		t.Error("HTML table headers are not SemiBold, the weight the PDF report uses")
	}
}

// Report PDFs register IBM Plex Sans (the PDF/A baseline) under the
// "Helvetica" key the renderers use, and the Six Sigma report embeds it
// instead of naming core fonts.
func TestReportPDFs_EmbedIBMPlexSans(t *testing.T) {
	const sample = "Earned value 1,284,500.00 — Phase 2 (WBS 1.2.3)"
	measure := func(register func(*pdfdoc.PDF), family string) float64 {
		pdf := pdfdoc.New("P", "mm", "A4", "")
		register(pdf)
		pdf.AddPage()
		pdf.SetFont(family, "B", 10)
		return pdf.GetStringWidth(sample)
	}
	baseline := measure(registerPDFABaseline, "Helvetica")
	plex := measure(func(pdf *pdfdoc.PDF) {
		if err := fonts.NewManager("").RegisterAs(pdf, fonts.DefaultFamily, "Probe"); err != nil {
			t.Fatalf("register %s: %v", fonts.DefaultFamily, err)
		}
	}, "Probe")
	if baseline != plex {
		t.Errorf("report bold text measures %.3f mm, want IBM Plex Sans SemiBold %.3f mm", baseline, plex)
	}

	sigma, _, err := GenerateSigmaReportPDF(domain.Project{Title: "Line 3", BeltLevel: domain.BeltGreen, Phase: domain.PhaseDefine, Status: domain.StatusActive}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateSigmaReportPDF: %v", err)
	}
	if !bytes.Contains(sigma, []byte("/FontFile2")) {
		t.Error("Six Sigma report embeds no TrueType font program")
	}
	if regexp.MustCompile(`/BaseFont /(Helvetica|Courier|Times)`).Match(sigma) {
		t.Error("Six Sigma report references an unembedded core font")
	}
}
