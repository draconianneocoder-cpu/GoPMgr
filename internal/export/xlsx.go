// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"sort"
	"time"

	"github.com/xuri/excelize/v2"
)

// renderXLSX produces an .xlsx workbook with two sheets:
//
//	"Schedule"  -- one row per task with full CPM fields
//	"Meta"      -- a key/value sheet with project title, timestamp,
//	               and app version. Useful for auditors.
//
// Excelize writes a real XLSX (zipped XML), not a CSV with an .xlsx
// extension, so opens cleanly in Excel, LibreOffice Calc, and Numbers.
func renderXLSX(payload ReportPayload, opts ExportOptions) (out []byte, err error) {
	f := excelize.NewFile()
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	const sheet = "Schedule"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	header := []interface{}{"ID", "Title", "Duration", "ES", "EF", "LS", "LF", "Float", "Critical"}
	if err := f.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, err
	}

	// Stable order.
	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for i, id := range ids {
		t := payload.Tasks[id]
		row := []interface{}{
			t.ID, t.Title, t.Duration, t.ES, t.EF, t.LS, t.LF, t.Float, t.IsCritical,
		}
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
	}

	// Meta sheet.
	const meta = "Meta"
	if _, err := f.NewSheet(meta); err != nil {
		return nil, err
	}
	metaRows := [][]interface{}{
		{"Title", opts.Title},
		{"GeneratedAt", time.Now().UTC().Format(time.RFC3339Nano)},
		{"AppVersion", exportVersion()},
	}
	for i, row := range metaRows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(meta, cell, &row); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
