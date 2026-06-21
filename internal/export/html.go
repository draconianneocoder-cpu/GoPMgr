// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"html"
	"sort"
	"time"
)

// renderHTML produces a self-contained, printable HTML schedule report:
// a heading, a task table (critical-path rows highlighted), and an
// optional earned-value summary. It has no external assets so it opens
// directly in any browser and is easy to publish. The output mirrors the
// CSV columns plus critical-path styling so the two stay consistent.
func renderHTML(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	title := opts.Title
	if title == "" {
		title = "Project Schedule"
	}

	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b bytes.Buffer
	esc := html.EscapeString
	num := func(v float64) string { return fmt.Sprintf("%.2f", v) }

	b.WriteString(`<!DOCTYPE html>` + "\n")
	b.WriteString(`<html lang="en"><head><meta charset="utf-8">` + "\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">` + "\n")
	b.WriteString(`<title>` + esc(title) + ` — Schedule</title>` + "\n")
	b.WriteString(`<style>
:root { color-scheme: light dark; }
body { font-family: -apple-system, Segoe UI, Roboto, Arial, sans-serif; margin: 2rem; color: #0f172a; }
h1 { font-size: 1.5rem; margin: 0 0 .25rem; }
.meta { color: #64748b; font-size: .8rem; margin-bottom: 1.5rem; }
table { border-collapse: collapse; width: 100%; font-size: .85rem; }
th, td { border: 1px solid #e2e8f0; padding: .4rem .6rem; text-align: left; }
th { background: #f1f5f9; }
td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
tr.critical td { background: #fef2f2; }
tr.critical td:first-child { border-left: 3px solid #dc2626; }
.crit-badge { color: #dc2626; font-weight: 700; }
h2 { font-size: 1.05rem; margin: 1.75rem 0 .5rem; }
.evm { columns: 2; max-width: 720px; }
.evm div { break-inside: avoid; padding: .15rem 0; font-size: .85rem; }
@media print { body { margin: 0; } th { background: #f1f5f9 !important; -webkit-print-color-adjust: exact; print-color-adjust: exact; } }
</style></head><body>` + "\n")

	b.WriteString(`<h1>` + esc(title) + `</h1>` + "\n")
	b.WriteString(`<div class="meta">Project schedule &middot; ` +
		esc(fmtCount(len(ids))) + ` &middot; generated ` +
		esc(time.Now().UTC().Format("2006-01-02 15:04 MST")) + `</div>` + "\n")

	b.WriteString(`<table><thead><tr>` +
		`<th>ID</th><th>Task</th>` +
		`<th class="num">Duration</th><th class="num">ES</th><th class="num">EF</th>` +
		`<th class="num">LS</th><th class="num">LF</th><th class="num">Float</th>` +
		`<th>Critical</th></tr></thead><tbody>` + "\n")

	for _, id := range ids {
		t := payload.Tasks[id]
		cls := ""
		crit := ""
		if t.IsCritical {
			cls = ` class="critical"`
			crit = `<span class="crit-badge">●</span>`
		}
		b.WriteString(`<tr` + cls + `>` +
			`<td>` + esc(t.ID) + `</td>` +
			`<td>` + esc(t.Title) + `</td>` +
			`<td class="num">` + num(t.Duration) + `</td>` +
			`<td class="num">` + num(t.ES) + `</td>` +
			`<td class="num">` + num(t.EF) + `</td>` +
			`<td class="num">` + num(t.LS) + `</td>` +
			`<td class="num">` + num(t.LF) + `</td>` +
			`<td class="num">` + num(t.Float) + `</td>` +
			`<td>` + crit + `</td></tr>` + "\n")
	}
	if len(ids) == 0 {
		b.WriteString(`<tr><td colspan="9" style="color:#64748b">No scheduled tasks.</td></tr>` + "\n")
	}
	b.WriteString(`</tbody></table>` + "\n")

	if lines := evmSummaryLines(payload.EVM); len(lines) > 0 {
		b.WriteString(`<h2>Earned Value (status date: today)</h2>` + "\n")
		b.WriteString(`<div class="evm">` + "\n")
		for _, l := range lines {
			b.WriteString(`<div>` + esc(l) + `</div>` + "\n")
		}
		b.WriteString(`</div>` + "\n")
	}

	b.WriteString(`</body></html>` + "\n")
	return b.Bytes(), nil
}

// fmtCount renders the task-count phrase for the report meta line.
func fmtCount(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}
