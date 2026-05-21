// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"pmforge/internal/documents"
)

// RenderDocumentODT produces an OpenDocument Text (.odt) file for
// the given document. ODT is a zipped XML package per the
// OpenDocument Format (OASIS standard); we hand-build it because
// no maintained pure-Go ODT generator exists on pkg.go.dev as of
// this writing (kpmy/odf is the closest match but hasn't been
// updated since 2014).
//
// The package layout we emit is the OpenDocument minimum:
//
//   mimetype                  (STORED, first entry, no compression)
//   META-INF/manifest.xml     (manifest listing every member)
//   meta.xml                  (generator + timestamp metadata)
//   styles.xml                (heading + paragraph + table styles)
//   content.xml               (the actual document body)
//
// LibreOffice, Microsoft Word 2007+, Apple Pages, and Google Docs
// all open files with this exact layout. Field-walking matches
// docx.go and the generic PDF renderer.
func RenderDocumentODT(kind documents.Kind, contentJSON, projectName string) ([]byte, error) {
	def, ok := documents.Get(kind)
	if !ok {
		return nil, fmt.Errorf("export: unknown document kind %q", kind)
	}

	var content map[string]interface{}
	if contentJSON != "" {
		if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
			return nil, fmt.Errorf("export: invalid content JSON: %w", err)
		}
	}

	// Build the document body XML.
	body, err := renderODTBody(kind, def, content, projectName)
	if err != nil {
		return nil, err
	}

	// Package as zip.
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)

	// Mimetype MUST be the first entry, STORED (no compression),
	// no extra fields. This is what OpenDocument readers look for
	// to identify the format.
	mimeHeader := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	mimeW, err := z.CreateHeader(mimeHeader)
	if err != nil {
		return nil, err
	}
	if _, err := mimeW.Write([]byte("application/vnd.oasis.opendocument.text")); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	files := []struct {
		name string
		body string
	}{
		{"META-INF/manifest.xml", odtManifest()},
		{"meta.xml", odtMeta(projectName, def.Name, now)},
		{"styles.xml", odtStyles()},
		{"content.xml", body},
	}
	for _, f := range files {
		w, err := z.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- XML templates ----

const odtNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"`

func odtManifest() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2">
  <manifest:file-entry manifest:media-type="application/vnd.oasis.opendocument.text" manifest:full-path="/"/>
  <manifest:file-entry manifest:media-type="text/xml" manifest:full-path="content.xml"/>
  <manifest:file-entry manifest:media-type="text/xml" manifest:full-path="styles.xml"/>
  <manifest:file-entry manifest:media-type="text/xml" manifest:full-path="meta.xml"/>
</manifest:manifest>
`
}

func odtMeta(projectName, kindName, isoNow string) string {
	// The dc: namespace fields are what readers display in
	// File → Properties.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                      xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0"
                      xmlns:dc="http://purl.org/dc/elements/1.1/"
                      office:version="1.2">
  <office:meta>
    <meta:generator>PMForge</meta:generator>
    <dc:title>%s — %s</dc:title>
    <meta:creation-date>%s</meta:creation-date>
    <dc:date>%s</dc:date>
  </office:meta>
</office:document-meta>
`, xmlEscape(projectName), xmlEscape(kindName), isoNow, isoNow)
}

// odtStyles holds the heading + table styles content.xml references.
// Kept minimal: one paragraph style per heading level, one default,
// one table-cell border style. Readers extend this with their own
// defaults — we don't need to enumerate every property.
func odtStyles() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + odtNS + ` office:version="1.2">
  <office:styles>
    <style:style style:name="Title" style:family="paragraph" style:parent-style-name="Standard">
      <style:text-properties fo:font-size="24pt" fo:font-weight="bold"/>
      <style:paragraph-properties fo:margin-bottom="0.25in"/>
    </style:style>
    <style:style style:name="Heading_1" style:family="paragraph" style:parent-style-name="Standard">
      <style:text-properties fo:font-size="18pt" fo:font-weight="bold"/>
      <style:paragraph-properties fo:margin-top="0.20in" fo:margin-bottom="0.10in"/>
    </style:style>
    <style:style style:name="Heading_2" style:family="paragraph" style:parent-style-name="Standard">
      <style:text-properties fo:font-size="13pt" fo:font-weight="bold"/>
      <style:paragraph-properties fo:margin-top="0.15in" fo:margin-bottom="0.05in"/>
    </style:style>
    <style:style style:name="Bullet" style:family="paragraph" style:parent-style-name="Standard">
      <style:paragraph-properties fo:margin-left="0.25in" fo:text-indent="-0.15in"/>
    </style:style>
    <style:style style:name="CellHeader" style:family="table-cell">
      <style:table-cell-properties fo:padding="0.04in" fo:border="0.5pt solid #94a3b8" fo:background-color="#1e293b"/>
    </style:style>
    <style:style style:name="Cell" style:family="table-cell">
      <style:table-cell-properties fo:padding="0.04in" fo:border="0.5pt solid #94a3b8"/>
    </style:style>
  </office:styles>
</office:document-styles>
`
}

// renderODTBody walks the document's Field definitions and emits
// content.xml.
func renderODTBody(_ documents.Kind, def documents.Definition, content map[string]interface{}, projectName string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + odtNS + ` office:version="1.2">
  <office:body><office:text>
`)

	writeODTPara(&buf, "Title", projectName)
	writeODTPara(&buf, "Heading_1", def.Name)

	for _, f := range documents.EffectiveFields(def.Kind) {
		v, present := content[f.Key]
		if !present {
			continue
		}
		switch f.Type {
		case documents.FieldStringArr:
			arr := toStringSliceLocal(v)
			if len(arr) == 0 {
				continue
			}
			writeODTPara(&buf, "Heading_2", f.Label)
			for _, item := range arr {
				writeODTPara(&buf, "Bullet", "• "+item)
			}
		case documents.FieldObjectArr:
			objs := toObjectSliceLocal(v)
			if len(objs) == 0 {
				continue
			}
			writeODTPara(&buf, "Heading_2", f.Label)
			writeODTTable(&buf, f, objs)
		case documents.FieldText:
			body := toStringLocal(v)
			if body == "" {
				continue
			}
			writeODTPara(&buf, "Heading_2", f.Label)
			writeODTPara(&buf, "Standard", body)
		case documents.FieldNumber:
			if n, ok := v.(float64); ok && n != 0 {
				writeODTPara(&buf, "Standard", f.Label+": "+fmt.Sprintf("%.2f", n))
			}
		case documents.FieldBool:
			if b, ok := v.(bool); ok {
				writeODTPara(&buf, "Standard", f.Label+": "+fmt.Sprintf("%t", b))
			}
		case documents.FieldChartRef:
			if id := toStringLocal(v); id != "" {
				writeODTPara(&buf, "Standard", f.Label+": (chart "+id+")")
			}
		default:
			if s := toStringLocal(v); s != "" {
				writeODTPara(&buf, "Standard", f.Label+": "+s)
			}
		}
	}

	buf.WriteString(`  </office:text></office:body>
</office:document-content>
`)
	return buf.String(), nil
}

func writeODTPara(buf *bytes.Buffer, style, text string) {
	fmt.Fprintf(buf, `    <text:p text:style-name="%s">%s</text:p>`+"\n",
		style, xmlEscape(text))
}

// writeODTTable emits an object-array field as a 1-header + N-row
// table. Column widths are left to the reader.
func writeODTTable(buf *bytes.Buffer, f documents.Field, objs []map[string]interface{}) {
	if len(f.ObjectShape) == 0 {
		// Fall back to bullet rows.
		for _, obj := range objs {
			line := ""
			for k, v := range obj {
				line += fmt.Sprintf("%s: %v; ", k, v)
			}
			writeODTPara(buf, "Bullet", "• "+line)
		}
		return
	}
	fmt.Fprintf(buf, `    <table:table>`+"\n")
	// Column declarations
	for range f.ObjectShape {
		fmt.Fprintf(buf, `      <table:table-column/>`+"\n")
	}
	// Header row
	fmt.Fprintf(buf, `      <table:table-row>`+"\n")
	for _, sub := range f.ObjectShape {
		fmt.Fprintf(buf, `        <table:table-cell table:style-name="CellHeader">`+"\n")
		writeODTPara(buf, "Standard", sub.Label)
		fmt.Fprintf(buf, `        </table:table-cell>`+"\n")
	}
	fmt.Fprintf(buf, `      </table:table-row>`+"\n")
	// Body rows
	for _, obj := range objs {
		fmt.Fprintf(buf, `      <table:table-row>`+"\n")
		for _, sub := range f.ObjectShape {
			val := fmt.Sprintf("%v", obj[sub.Key])
			if val == "<nil>" {
				val = ""
			}
			fmt.Fprintf(buf, `        <table:table-cell table:style-name="Cell">`+"\n")
			writeODTPara(buf, "Standard", val)
			fmt.Fprintf(buf, `        </table:table-cell>`+"\n")
		}
		fmt.Fprintf(buf, `      </table:table-row>`+"\n")
	}
	fmt.Fprintf(buf, `    </table:table>`+"\n")
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
