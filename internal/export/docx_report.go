// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"time"
	
	"github.com/gomutex/godocx"
	
	"pmforge/internal/kernel"
)

// renderDocumentDOCX produces a Microsoft Word file containing the CPM schedule
// data from the ReportPayload. It follows the same structure as the PDF report
// but uses gomutex/godocx for DOCX generation.
func renderDocumentDOCX(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	doc, err := godocx.NewDocument()
	if err != nil {
		return nil, fmt.Errorf("export: godocx new: %w", err)
	}

	// Title block
	doc.AddHeading(opts.Title, 0)
	doc.AddHeading("CPM Schedule Report", 1)

	// Timestamp
	doc.AddParagraph("Generated " + time.Now().UTC().Format(time.RFC3339Nano))
	doc.AddParagraph("") // empty line

	// Table header
	table := doc.AddTable()
	table.Row().Cells().AddText("ID").AddText("Title").AddText("Duration").AddText("ES").AddText("EF").AddText("LS").AddText("LF").AddText("Float").AddText("Critical")
	table.Rows()[0].Cells()[0].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[1].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[2].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[3].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[4].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[5].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[6].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[7].GetParagraphs()[0].GetRuns()[0].Bold()
	table.Rows()[0].Cells()[8].GetParagraphs()[0].GetRuns()[0].Bold()

	// Stable order.
	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Task rows
	for _, id := range ids {
		t := payload.Tasks[id]
		criticalText := "NO"
		if t.IsCritical {
			criticalText = "YES"
		}
		row := table.AddRow().Cells()
		row.AddText(t.ID)
		row.AddText(t.Title)
		row.AddText(fmt.Sprintf("%.1f", t.Duration))
		row.AddText(fmt.Sprintf("%.1f", t.ES))
		row.AddText(fmt.Sprintf("%.1f", t.EF))
		row.AddText(fmt.Sprintf("%.1f", t.LS))
		row.AddText(fmt.Sprintf("%.1f", t.LF))
		row.AddText(fmt.Sprintf("%.2f", t.Float))
		row.AddText(criticalText)
	}

	// gomutex/godocx writes via a path or io.Writer; we serialise
	// to a temp file and read it back so the export pipeline can
	// hand back bytes (PMForge always returns []byte from
	// renderers).
	tmp, err := os.CreateTemp("", "pmforge-report-*.docx")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := doc.SaveTo(tmpPath); err != nil {
		return nil, fmt.Errorf("export: docx save: %w", err)
	}
	return os.ReadFile(tmpPath)
}