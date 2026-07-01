// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"

	"pmforge/internal/exportsafe"
)

// renderCSV produces a UTF-8 CSV with one row per task. Useful both as
// a debugging aid and as a quick interchange format for spreadsheets
// that don't open XLSX directly.
func renderCSV(payload ReportPayload, _ ExportOptions) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"id", "title", "duration", "es", "ef", "ls", "lf", "float", "critical"}); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		t := payload.Tasks[id]
		// Title is free-text and user-controlled, so neutralize it against
		// formula injection (CWE-1236). The remaining columns are
		// app-generated IDs and formatted numerics/bools, not attacker text.
		err := w.Write([]string{
			t.ID,
			exportsafe.Cell(t.Title),
			fmt.Sprintf("%.4f", t.Duration),
			fmt.Sprintf("%.4f", t.ES),
			fmt.Sprintf("%.4f", t.EF),
			fmt.Sprintf("%.4f", t.LS),
			fmt.Sprintf("%.4f", t.LF),
			fmt.Sprintf("%.4f", t.Float),
			fmt.Sprintf("%t", t.IsCritical),
		})
		if err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
