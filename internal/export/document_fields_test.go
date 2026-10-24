// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"html"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gomutex/godocx"

	"gopmgr/internal/documents"
)

var (
	docxParagraph = regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>`)
	docxRun       = regexp.MustCompile(`(?s)<w:t(?: [^>]*)?>(.*?)</w:t>`)
	odtParagraph  = regexp.MustCompile(`<text:p text:style-name="([^"]+)">(.*?)</text:p>`)
)

// docxParagraphs returns the text of each paragraph in a DOCX's body.
func docxParagraphs(t *testing.T, raw []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open DOCX: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		var out []string
		for _, p := range docxParagraph.FindAll(body, -1) {
			var text strings.Builder
			for _, r := range docxRun.FindAllSubmatch(p, -1) {
				text.WriteString(html.UnescapeString(string(r[1])))
			}
			out = append(out, text.String())
		}
		return out
	}
	t.Fatal("DOCX has no word/document.xml")
	return nil
}

// docxFieldParagraphs renders one field into an otherwise empty DOCX and
// returns the paragraphs it added.
func docxFieldParagraphs(t *testing.T, f documents.Field, v interface{}) []string {
	t.Helper()
	empty, err := godocx.NewDocument()
	if err != nil {
		t.Fatal(err)
	}
	emptyRaw, err := renderDOCXToBytes(empty, "gopmgr-test")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := godocx.NewDocument()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeDOCXField(doc, f, v, "EUR"); err != nil {
		t.Fatalf("writeDOCXField: %v", err)
	}
	raw, err := renderDOCXToBytes(doc, "gopmgr-test")
	if err != nil {
		t.Fatal(err)
	}
	return docxParagraphs(t, raw)[len(docxParagraphs(t, emptyRaw)):]
}

// odtFieldParagraphs renders one field as ODT and returns each paragraph
// as "style|text".
func odtFieldParagraphs(f documents.Field, v interface{}) []string {
	var buf bytes.Buffer
	writeODTField(&buf, f, v, "EUR")
	var out []string
	for _, m := range odtParagraph.FindAllStringSubmatch(buf.String(), -1) {
		out = append(out, m[1]+"|"+html.UnescapeString(m[2]))
	}
	return out
}

// Every field type prints in DOCX and ODT the way its type calls for, and
// an empty or mistyped value prints nothing. No built-in document has a
// yes/no field or an object list without a shape yet, so those are only
// reachable this way.
func TestDocumentExportsWriteEveryFieldType(t *testing.T) {
	list := documents.Field{Key: "risks", Label: "Risks", Type: documents.FieldStringArr}
	text := documents.Field{Key: "summary", Label: "Summary", Type: documents.FieldText}
	budget := documents.Field{Key: "budget", Label: "Budget", Type: documents.FieldNumber, Money: true}
	approved := documents.Field{Key: "approved", Label: "Approved", Type: documents.FieldBool}
	chart := documents.Field{Key: "schedule_ref", Label: "Schedule", Type: documents.FieldChartRef}
	date := documents.Field{Key: "report_date", Label: "Report date", Type: documents.FieldDate}
	owner := documents.Field{Key: "owner", Label: "Owner", Type: documents.FieldString}
	objects := documents.Field{Key: "items", Label: "Items", Type: documents.FieldObjectArr}

	cases := []struct {
		name string
		f    documents.Field
		v    interface{}
		docx []string
		odt  []string
	}{
		{"list", list, []interface{}{"Late steel", 3.0}, []string{"Risks", "• Late steel", "• 3"},
			[]string{"Heading_2|Risks", "Bullet|• Late steel", "Bullet|• 3"}},
		{"empty list", list, []interface{}{}, nil, nil},
		{"list given a string", list, "Late steel", nil, nil},
		{"text", text, "On track & on budget", []string{"Summary", "On track & on budget"},
			[]string{"Heading_2|Summary", "Standard|On track & on budget"}},
		{"empty text", text, "", nil, nil},
		{"amount", budget, 1284500.0, []string{"Budget (EUR): 1,284,500.00"}, []string{"Standard|Budget (EUR): 1,284,500.00"}},
		{"zero amount", budget, 0.0, nil, nil},
		{"amount given a string", budget, "1284500", nil, nil},
		{"yes", approved, true, []string{"Approved: true"}, []string{"Standard|Approved: true"}},
		{"no", approved, false, []string{"Approved: false"}, []string{"Standard|Approved: false"}},
		{"yes/no given a string", approved, "yes", nil, nil},
		{"chart", chart, "chart-7", []string{"Schedule: (chart chart-7)"}, []string{"Standard|Schedule: (chart chart-7)"}},
		{"no chart", chart, "", nil, nil},
		{"date", date, "2026-10-24", []string{"Report date: 2026-10-24"}, []string{"Standard|Report date: 2026-10-24"}},
		{"whole number as text", owner, 3.0, []string{"Owner: 3"}, []string{"Standard|Owner: 3"}},
		{"fraction as text", owner, 12.5, []string{"Owner: 12.50"}, []string{"Standard|Owner: 12.50"}},
		{"null", owner, nil, nil, nil},
		{"objects without a shape", objects, []interface{}{map[string]interface{}{"b": 2.0, "a": "x"}, "not an object"},
			[]string{"Items", "(1) a: x; b: 2; "}, []string{"Heading_2|Items", "Bullet|• a: x; b: 2; "}},
		{"no objects", objects, []interface{}{"not an object"}, nil, nil},
		{"objects given a string", objects, "x", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := docxFieldParagraphs(t, c.f, c.v); !slices.Equal(got, c.docx) {
				t.Errorf("DOCX paragraphs = %q, want %q", got, c.docx)
			}
			if got := odtFieldParagraphs(c.f, c.v); !slices.Equal(got, c.odt) {
				t.Errorf("ODT paragraphs = %q, want %q", got, c.odt)
			}
		})
	}
}

// An object without a shape lists its keys in sorted order in both
// formats, so the same object reads the same in every export. The ODT
// export used to list them in map order, which changes run to run.
func TestShapelessObjectsListKeysInSortedOrder(t *testing.T) {
	objects := documents.Field{Key: "items", Label: "Items", Type: documents.FieldObjectArr}
	obj := map[string]interface{}{}
	for _, k := range []string{"zeta", "alpha", "mu", "beta", "omega", "gamma", "delta", "kappa"} {
		obj[k] = k
	}
	want := "alpha: alpha; beta: beta; delta: delta; gamma: gamma; kappa: kappa; mu: mu; omega: omega; zeta: zeta; "
	for range 20 {
		if got := odtFieldParagraphs(objects, []interface{}{obj}); !slices.Equal(got, []string{"Heading_2|Items", "Bullet|• " + want}) {
			t.Fatalf("ODT paragraphs = %q, want keys in sorted order", got)
		}
	}
	if got := docxFieldParagraphs(t, objects, []interface{}{obj}); !slices.Equal(got, []string{"Items", "(1) " + want}) {
		t.Fatalf("DOCX paragraphs = %q, want keys in sorted order", got)
	}
}

// A real document runs its fields through the same writers, in the
// template's order, and skips fields the content does not have.
func TestRenderDocumentExportsAStatusReport(t *testing.T) {
	content := `{"report_date":"2026-10-24","overall_status":"Green","accomplishments":["Poured slab"],"schedule_ref":"chart-7","blockers":[]}`
	raw, err := RenderDocumentDOCX(documents.KindStatusReport, content, "Line 3", "USD")
	if err != nil {
		t.Fatalf("RenderDocumentDOCX: %v", err)
	}
	want := []string{"Line 3", "Status Report", "Report Date: 2026-10-24", "Linked Schedule Chart: (chart chart-7)", "Overall Status: Green", "Accomplishments", "• Poured slab"}
	if got := docxParagraphs(t, raw); !slices.Equal(got, want) {
		t.Errorf("DOCX paragraphs = %q, want %q", got, want)
	}

	raw, err = RenderDocumentODT(documents.KindStatusReport, content, "Line 3", "USD")
	if err != nil {
		t.Fatalf("RenderDocumentODT: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open ODT: %v", err)
	}
	var body []byte
	for _, f := range zr.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			body, _ = io.ReadAll(rc)
			_ = rc.Close()
		}
	}
	var got []string
	for _, m := range odtParagraph.FindAllSubmatch(body, -1) {
		got = append(got, html.UnescapeString(string(m[2])))
	}
	if !slices.Equal(got, want) {
		t.Errorf("ODT paragraphs = %q, want %q", got, want)
	}
}

// Both exports refuse an unknown document kind and content that is not
// JSON, as they refuse a malformed currency.
func TestRenderDocumentExportsRefuseBadInput(t *testing.T) {
	for _, render := range []struct {
		name string
		fn   func(documents.Kind, string, string, string) ([]byte, error)
	}{{"DOCX", RenderDocumentDOCX}, {"ODT", RenderDocumentODT}} {
		if _, err := render.fn("no_such_kind", "", "P", "USD"); err == nil || !strings.Contains(err.Error(), "unknown document kind") {
			t.Errorf("%s with an unknown kind: error = %v", render.name, err)
		}
		if _, err := render.fn(documents.KindStatusReport, "{not json", "P", "USD"); err == nil || !strings.Contains(err.Error(), "invalid content JSON") {
			t.Errorf("%s with malformed content: error = %v", render.name, err)
		}
	}
}

// The DOCX library saves through a temporary file; when none can be made
// the export fails with that error instead of returning empty bytes.
func TestRenderDOCXFailsWithoutATempDirectory(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()+"/missing")
	raw, err := RenderDocumentDOCX(documents.KindStatusReport, "", "P", "USD")
	if err == nil || raw != nil {
		t.Fatalf("RenderDocumentDOCX with no temp directory = %d bytes, %v; want an error", len(raw), err)
	}
}

func TestDocumentValueHelpers(t *testing.T) {
	for _, c := range []struct {
		in   interface{}
		want string
	}{
		{nil, ""}, {"text", "text"}, {3.0, "3"}, {-4.0, "-4"}, {12.5, "12.50"}, {true, "true"},
		{map[string]interface{}{"a": 1.0}, "map[a:1]"},
	} {
		if got := toStringLocal(c.in); got != c.want {
			t.Errorf("toStringLocal(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := toStringSliceLocal([]interface{}{"a", 2.0, nil}); !slices.Equal(got, []string{"a", "2", ""}) {
		t.Errorf("toStringSliceLocal = %q", got)
	}
	if got := toStringSliceLocal("a"); got != nil {
		t.Errorf("toStringSliceLocal(non-list) = %q, want nil", got)
	}
	objs := toObjectSliceLocal([]interface{}{map[string]interface{}{"a": 1.0}, "x", 2.0})
	if len(objs) != 1 || objs[0]["a"] != 1.0 {
		t.Errorf("toObjectSliceLocal kept %v, want only the object", objs)
	}
	if toObjectSliceLocal("x") != nil {
		t.Error("toObjectSliceLocal(non-list) is not nil")
	}
}
