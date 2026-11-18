// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"fmt"
	"strconv"
	"strings"

	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/schedaudit"
)

// auditSeverityLabel is the printed name of a finding's severity. Severity is
// always written as text, never shown by colour alone.
func auditSeverityLabel(s schedaudit.Severity) string {
	switch s {
	case schedaudit.SeverityError:
		return "Error"
	case schedaudit.SeverityWarning:
		return "Warning"
	default:
		return "Note"
	}
}

// AuditSummaryLines returns the lines that say what an audit covered: its
// source, its scope note, the rules that did not run, and the count of
// findings. A result with no findings is never printed as a clean schedule
// when a rule was not evaluated. The PDF section and its tests share it.
func AuditSummaryLines(r schedaudit.Result) []string {
	reporting := r.ReportingDate
	if reporting == "" {
		reporting = "none"
	}
	lines := []string{
		"Thresholds: " + r.Profile,
		"Chart: " + r.Source.ChartID + " (revision " + r.Source.Revision + ")",
		"Status date: " + reporting,
		"Audit engine: " + r.EngineVersion,
		"This audit reads the schedule and changes nothing. Profile thresholds are guidance; a DCMA-style profile is not a DCMA assessment and certifies nothing.",
	}
	var skipped []string
	for _, rule := range r.Rules {
		if !rule.Evaluated {
			skipped = append(skipped, "Not evaluated: "+string(rule.Rule)+" "+schedaudit.RuleName(rule.Rule)+": "+rule.Reason)
		}
	}
	lines = append(lines, skipped...)

	counts := map[schedaudit.Severity]int{}
	for _, f := range r.Findings {
		counts[f.Severity]++
	}
	switch {
	case len(r.Findings) > 0:
		lines = append(lines, "Findings: "+countOf(counts[schedaudit.SeverityError], "error", "errors")+", "+
			countOf(counts[schedaudit.SeverityWarning], "warning", "warnings")+", "+countOf(counts[schedaudit.SeverityInfo], "note", "notes")+".")
	case len(skipped) > 0:
		verb := "were"
		if len(skipped) == 1 {
			verb = "was"
		}
		lines = append(lines, "No findings from the rules that ran; "+countOf(len(skipped), "rule", "rules")+" "+verb+" not evaluated (listed above).")
	default:
		lines = append(lines, "No findings: every rule ran and found nothing.")
	}
	return lines
}

// countOf writes a count with the singular or plural noun.
func countOf(n int, one, other string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + other
}

// AuditMetricLine prints one metric as a share with its numerator and
// denominator, and its limit and result when the profile sets one.
func AuditMetricLine(m schedaudit.Metric) string {
	line := fmt.Sprintf("%s: %d of %d (%s%%)", m.ID, m.Numerator, m.Denominator, strconv.FormatFloat(m.Percent, 'f', 1, 64))
	if m.Limit != nil {
		op := "at most"
		if m.Limit.AtLeast {
			op = "at least"
		}
		line += fmt.Sprintf("; limit %s %s%%", op, strconv.FormatFloat(m.Limit.Percent, 'f', -1, 64))
	}
	if m.Pass != nil {
		if *m.Pass {
			line += "; passes"
		} else {
			line += "; fails"
		}
	}
	return line
}

// AuditFindingLines returns one finding as printed: a heading naming its
// severity, rule and affected task or link, then the observed value and
// threshold, the explanation and the advice. Empty parts are left out.
func AuditFindingLines(f schedaudit.Finding) []string {
	observed := "Observed: " + f.Observed
	if f.Threshold != "" {
		observed += "; threshold: " + f.Threshold
	}
	lines := []string{auditFindingHeading(f), observed}
	if f.Explanation != "" {
		lines = append(lines, f.Explanation)
	}
	if f.Advice != "" {
		lines = append(lines, "Advice: "+f.Advice)
	}
	return lines
}

// auditFindingHeading names a finding's severity, rule and affected record.
func auditFindingHeading(f schedaudit.Finding) string {
	parts := []string{auditSeverityLabel(f.Severity), string(f.Rule) + " " + schedaudit.RuleName(f.Rule)}
	switch {
	case f.Link != nil:
		parts = append(parts, fmt.Sprintf("Link %s -> %s (%s, lag %s)", f.Link.Pred, f.Link.Succ, f.Link.Type, strconv.FormatFloat(f.Link.Lag, 'f', -1, 64)))
	case f.TaskID != "" && f.TaskTitle != "":
		parts = append(parts, "Task "+f.TaskTitle+" ("+f.TaskID+")")
	case f.TaskID != "":
		parts = append(parts, "Task "+f.TaskID)
	}
	return strings.Join(parts, " - ")
}

// renderAuditSection prints a schedule-audit section: what was audited and
// what was not, the metrics, then every finding in the result's order.
// Nothing is dropped, however many findings there are.
func renderAuditSection(pdf *pdfdoc.PDF, r schedaudit.Result) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(110, 110, 110)
	pdf.CellFormat(0, 5, "Schedule-quality audit (PS-02)", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "", 9)
	for _, line := range AuditSummaryLines(r) {
		pdf.MultiCell(0, 4.5, line, "", "L", false)
	}

	if len(r.Metrics) > 0 {
		writeSubHeading(pdf, "Metrics")
		pdf.SetFont("Helvetica", "", 9)
		for _, m := range r.Metrics {
			pdf.MultiCell(0, 4.5, AuditMetricLine(m), "", "L", false)
		}
	}

	if len(r.Findings) == 0 {
		return
	}
	writeSubHeading(pdf, "Findings")
	for _, f := range r.Findings {
		for i, line := range AuditFindingLines(f) {
			style := ""
			if i == 0 {
				style = "B"
			}
			pdf.SetFont("Helvetica", style, 9)
			pdf.MultiCell(0, 4.5, line, "", "L", false)
		}
		pdf.Ln(1.5)
	}
}
