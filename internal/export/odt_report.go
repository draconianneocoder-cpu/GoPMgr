// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"time"
)

// renderDocumentODT produces an OpenDocument Text (.odt) file containing the 
// CPM schedule data from the ReportPayload. It follows the same structure as 
// the PDF report but hand-builds the ODT XML package.
func renderDocumentODT(payload export.ReportPayload, opts export.ExportOptions) ([]byte, error) {
	// Build the document body XML for the CPM report
	body, err := renderODTReportBody(payload, opts.Title)
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
		{"meta.xml", odtMeta("PMForge CPM Report", opts.Title, now)},
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

// renderODTReportBody generates the content.xml for a CPM report ODT file.
func renderODTReportBody(payload export.ReportPayload, title string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + odtNS + ` office:version="1.2">
  <office:body><office:text>
`)

	// Title
	writeODTPara(&buf, "Title", title)

	// Subtitle
	writeODTPara(&buf, "Heading_1", "CPM Schedule Report")

	// Timestamp
	writeODTPara(&buf, "Heading_2", "Generated "+time.Now().UTC().Format(time.RFC3339Nano))
	writeODTPara(&buf, "Standard", "") // empty line

	// Table header
	writeODTPara(&buf, "Heading_2", "Tasks")
	writeODTTableHeaders(&buf, []string{"ID", "Title", "Duration", "ES", "EF", "LS", "LF", "Float", "Critical"})

	// Stable order.
	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Task rows
	tableData := make([][]string, 0, len(ids))
	for _, id := range ids {
		t := payload.Tasks[id]
		criticalText := "NO"
		if t.IsCritical {
			criticalText = "YES"
		}
		tableData = append(tableData, []string{
			t.ID,
			t.Title,
			fmt.Sprintf("%.1f", t.Duration),
			fmt.Sprintf("%.1f", t.ES),
			fmt.Sprintf("%.1f", t.EF),
			fmt.Sprintf("%.1f", t.LS),
			fmt.Sprintf("%.1f", t.LF),
			fmt.Sprintf("%.2f", t.Float),
			criticalText,
		})
	}
	writeODTTableRows(&buf, tableData)

	buf.WriteString(`  </office:text></office:body>
</office:document-content>
`)
	return buf.String(), nil
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

func odtMeta(title, subject, isoNow string) string {
	// The dc: namespace fields are what readers display in
	// File → Properties.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                      xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0"
                      xmlns:dc="http://purl.org/dc/elements/1.1/"
                      office:version="1.2">
  <office:meta>
    <meta:generator>PMForge</meta:generator>
    <dc:title>%s</dc:title>
    <dc:subject>%s</dc:subject>
    <meta:creation-date>%s</meta:creation-date>
    <dc:date>%s</dc:date>
  </office:meta>
</office:document-meta>
`, xmlEscape(title), xmlEscape(subject), isoNow, isoNow)
}

// odtStyles holds the heading + table styles content.xml references.
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
      <style:text-properties fo:font-size="14pt" fo:font-weight="bold"/>
      <style:paragraph-properties fo:margin-top="0.15in" fo:margin-bottom="0.05in"/>
    </style:style>
    <style:style style:name="Standard" style:family="paragraph" style:parent-style-name="Standard"/>
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

// writeODTPara emits a paragraph with the given text style.
func writeODTPara(buf *bytes.Buffer, style, text string) {
	fmt.Fprintf(buf, `    <text:p text:style-name="%s">%s</text:p>`+"\n",
		style, xmlEscape(text))
}

// writeODTTableHeaders emits a table header row.
func writeODTTableHeaders(buf *bytes.Buffer, headers []string) {
	fmt.Fprintf(buf, `    <table:table>`+"\n")
	// Column declarations
	for range headers {
		fmt.Fprintf(buf, `      <table:table-column/>`+"\n")
	}
	// Header row
	fmt.Fprintf(buf, `      <table:table-row>`+"\n")
	for _, h := range headers {
		fmt.Fprintf(buf, `        <table:table-cell table:style-name="CellHeader">`+"\n")
		writeODTPara(buf, "Standard", h)
		fmt.Fprintf(buf, `        </table:table-cell>`+"\n")
	}
	fmt.Fprintf(buf, `      </table:table-row>`+"\n")
}

// writeODTTableRows emits the table body rows.
func writeODTTableRows(buf *bytes.Buffer, rows [][]string) {
	for _, row := range rows {
		fmt.Fprintf(buf, `      <table:table-row>`+"\n")
		for _, cell := range row {
			fmt.Fprintf(buf, `        <table:table-cell table:style-name="Cell">`+"\n")
			writeODTPara(buf, "Standard", cell)
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