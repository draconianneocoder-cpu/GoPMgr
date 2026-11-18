// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopmgr/internal/schedaudit"
)

// A document section must encode exactly as it did before schedule-audit
// sections existed: signed-report checkpoint IDs and audit-log entries hash
// this encoding, so a changed encoding would re-identify every past report.
func TestDocumentSectionEncodingIsUnchanged(t *testing.T) {
	got, err := json.Marshal(ReportSection{DocumentID: "d", Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"document_id":"d","title":"t","description":""}`; string(got) != want {
		t.Fatalf("encoding = %s, want %s", got, want)
	}
}

func TestAuditSummaryNeverReadsAsCleanWhenRulesWereSkipped(t *testing.T) {
	base := schedaudit.Result{
		Profile:       "DCMA-style",
		Source:        schedaudit.Source{ChartID: "chart-1", Revision: "2026-06-15T10:00:00Z"},
		EngineVersion: "schedaudit/1",
	}
	head := []string{
		"Thresholds: DCMA-style",
		"Chart: chart-1 (revision 2026-06-15T10:00:00Z)",
		"Status date: none",
		"Audit engine: schedaudit/1",
		"This audit reads the schedule and changes nothing. Profile thresholds are guidance; a DCMA-style profile is not a DCMA assessment and certifies nothing.",
	}
	skipped := base
	skipped.Rules = []schedaudit.RuleStatus{
		{Rule: schedaudit.MissingPredecessor, Evaluated: true},
		{Rule: schedaudit.ConstraintViolated, Evaluated: false, Reason: "the schedule has a dependency cycle"},
		{Rule: schedaudit.NegativeFloat, Evaluated: false, Reason: "the schedule has a dependency cycle"},
	}
	clean := base
	clean.Rules = []schedaudit.RuleStatus{{Rule: schedaudit.MissingPredecessor, Evaluated: true}}
	findings := base
	findings.ReportingDate = "2026-06-15"
	findings.Findings = []schedaudit.Finding{
		{Rule: schedaudit.DependencyCycle, Severity: schedaudit.SeverityError},
		{Rule: schedaudit.Lead, Severity: schedaudit.SeverityWarning},
		{Rule: schedaudit.LongLag, Severity: schedaudit.SeverityInfo},
		{Rule: schedaudit.LongDuration, Severity: schedaudit.SeverityInfo},
	}

	for name, tc := range map[string]struct {
		result schedaudit.Result
		tail   []string
	}{
		"skipped rules, no findings": {skipped, []string{
			"Not evaluated: SQ-09 Constraint violated: the schedule has a dependency cycle",
			"Not evaluated: SQ-11 Negative float: the schedule has a dependency cycle",
			"No findings from the rules that ran; 2 rules were not evaluated (listed above).",
		}},
		"every rule ran, no findings": {clean, []string{"No findings: every rule ran and found nothing."}},
		"findings":                    {findings, []string{"Findings: 1 error, 1 warning, 2 notes."}},
	} {
		t.Run(name, func(t *testing.T) {
			want := append(append([]string{}, head...), tc.tail...)
			if tc.result.ReportingDate != "" {
				want[2] = "Status date: " + tc.result.ReportingDate
			}
			if got := AuditSummaryLines(tc.result); !reflect.DeepEqual(got, want) {
				t.Fatalf("lines =\n%q\nwant\n%q", got, want)
			}
		})
	}
}

func TestAuditFindingAndMetricLinesAsPrinted(t *testing.T) {
	pass, fail := true, false
	for name, tc := range map[string]struct {
		got, want []string
	}{
		"task finding with every part": {AuditFindingLines(schedaudit.Finding{
			Rule: schedaudit.ConstraintViolated, Severity: schedaudit.SeverityError, TaskID: "B", TaskTitle: "Build",
			Observed: "MFO 2026-06-03 cannot be met", Threshold: "0", Explanation: "The finish is late.", Advice: "Move the date.",
		}), []string{
			"Error - SQ-09 Constraint violated - Task Build (B)",
			"Observed: MFO 2026-06-03 cannot be met; threshold: 0",
			"The finish is late.",
			"Advice: Move the date.",
		}},
		"link finding, no threshold, no text": {AuditFindingLines(schedaudit.Finding{
			Rule: schedaudit.Lead, Severity: schedaudit.SeverityWarning,
			Link: &schedaudit.LinkRef{Pred: "A", Succ: "C", Type: "SS", Lag: -1.5}, Observed: "lag -1.5",
		}), []string{
			"Warning - SQ-05 Lead (negative lag) - Link A -> C (SS, lag -1.5)",
			"Observed: lag -1.5",
		}},
		"task without title, info": {AuditFindingLines(schedaudit.Finding{
			Rule: schedaudit.UnresourcedTask, Severity: schedaudit.SeverityInfo, TaskID: "X", Observed: "no assignment",
		}), []string{"Note - SQ-17 Unresourced task - Task X", "Observed: no assignment"}},
		"metrics": {[]string{
			AuditMetricLine(schedaudit.Metric{ID: "logic", Numerator: 2, Denominator: 3, Percent: 66.66, Limit: &schedaudit.Limit{Percent: 5}, Pass: &fail}),
			AuditMetricLine(schedaudit.Metric{ID: "SQ-07", Numerator: 9, Denominator: 10, Percent: 90, Limit: &schedaudit.Limit{AtLeast: true, Percent: 90}, Pass: &pass}),
			AuditMetricLine(schedaudit.Metric{ID: "SQ-17", Numerator: 0, Denominator: 0, Percent: 0}),
		}, []string{
			"logic: 2 of 3 (66.7%); limit at most 5%; fails",
			"SQ-07: 9 of 10 (90.0%); limit at least 90%; passes",
			"SQ-17: 0 of 0 (0.0%)",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Fatalf("lines =\n%q\nwant\n%q", tc.got, tc.want)
			}
		})
	}
}
