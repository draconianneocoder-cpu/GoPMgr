// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"time"
)

// renderDocumentODT produces an OpenDocument Text (.odt) file containing the
// CPM schedule data from the ReportPayload. It follows the same structure as
// the PDF report but hand-builds the ODT XML package.
func renderDocumentODT(payload ReportPayload, opts ExportOptions) ([]byte, error) {
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
func renderODTReportBody(payload ReportPayload, title string) (string, error) {
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

	// Earned-value summary (suppressed without cost data).
	if lines := evmSummaryLines(payload.EVM); lines != nil {
		writeODTPara(&buf, "Standard", "")
		writeODTPara(&buf, "Heading_2", "Earned Value (status date: today)")
		for _, line := range lines {
			writeODTPara(&buf, "Standard", line)
		}
	}

	buf.WriteString(`  </office:text></office:body>
</office:document-content>
`)
	return buf.String(), nil
}

func writeODTTableHeaders(buf *bytes.Buffer, headers []string) {
	buf.WriteString(`    <table:table>` + "\n")
	for range headers {
		buf.WriteString(`      <table:table-column/>` + "\n")
	}
	buf.WriteString(`      <table:table-row>` + "\n")
	for _, h := range headers {
		buf.WriteString(`        <table:table-cell table:style-name="CellHeader">` + "\n")
		writeODTPara(buf, "Standard", h)
		buf.WriteString(`        </table:table-cell>` + "\n")
	}
	buf.WriteString(`      </table:table-row>` + "\n")
}

func writeODTTableRows(buf *bytes.Buffer, rows [][]string) {
	for _, row := range rows {
		buf.WriteString(`      <table:table-row>` + "\n")
		for _, cell := range row {
			buf.WriteString(`        <table:table-cell table:style-name="Cell">` + "\n")
			writeODTPara(buf, "Standard", cell)
			buf.WriteString(`        </table:table-cell>` + "\n")
		}
		buf.WriteString(`      </table:table-row>` + "\n")
	}
	buf.WriteString(`    </table:table>` + "\n")
}
