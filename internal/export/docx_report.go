// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"fmt"
	"sort"
	"time"

	"github.com/gomutex/godocx"
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
	if err := addHeadingDOCX(doc, opts.Title, 0); err != nil {
		return nil, err
	}
	if err := addHeadingDOCX(doc, "CPM Schedule Report", 1); err != nil {
		return nil, err
	}

	// Timestamp
	doc.AddParagraph("Generated " + time.Now().UTC().Format(time.RFC3339Nano))
	doc.AddParagraph("") // empty line

	// Stable order.
	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Task rows as bullet list (matches the proven pattern in docx.go)
	for _, id := range ids {
		t := payload.Tasks[id]
		criticalText := "NO"
		if t.IsCritical {
			criticalText = "YES"
		}
		doc.AddParagraph(fmt.Sprintf("• %s | %s | %.1f | ES:%.1f EF:%.1f LS:%.1f LF:%.1f Float:%.2f Crit:%s",
			id, t.Title, t.Duration, t.ES, t.EF, t.LS, t.LF, t.Float, criticalText))
	}

	// Earned-value summary (suppressed without cost data).
	if lines := evmSummaryLines(payload.EVM); lines != nil {
		doc.AddParagraph("")
		if err := addHeadingDOCX(doc, "Earned Value (status date: today)", 1); err != nil {
			return nil, err
		}
		for _, line := range lines {
			doc.AddParagraph(line)
		}
	}

	return renderDOCXToBytes(doc, "pmforge-report")
}
