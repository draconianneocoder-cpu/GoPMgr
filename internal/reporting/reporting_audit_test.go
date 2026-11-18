// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package reporting

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopmgr/internal/documents"
	"gopmgr/internal/schedaudit"
)

// sequenceResolver returns a different audit on each call: the first has one
// error finding and SQ-09 not evaluated, later ones are clean. An export that
// ran the audit more than once could mix them.
func sequenceResolver(calls *int) AuditResolver {
	return func(_ context.Context, section documents.ScheduleAuditSection) (AuditResolution, error) {
		*calls++
		result := schedaudit.Result{
			Profile:       "GoPMgr default",
			Source:        schedaudit.Source{ChartID: section.ChartID, Revision: fmt.Sprintf("rev-%d", *calls)},
			EngineVersion: "schedaudit/1",
			ReportingDate: section.ReportingDate,
			Findings:      []schedaudit.Finding{},
			Rules:         []schedaudit.RuleStatus{{Rule: schedaudit.ConstraintViolated, Evaluated: true}},
		}
		if *calls == 1 {
			result.Findings = []schedaudit.Finding{{Rule: schedaudit.DanglingReference, Severity: schedaudit.SeverityError, TaskID: "A", Observed: "link to Z"}}
			result.Rules = []schedaudit.RuleStatus{{Rule: schedaudit.ConstraintViolated, Evaluated: false, Reason: "no project start date"}}
		}
		return AuditResolution{
			Result:         result,
			ChartTitle:     "Master schedule",
			ChartSHA256:    fmt.Sprintf("chart-digest-%d", *calls),
			ProjectStart:   "2026-06-01",
			CountryCode:    "US",
			CapacitySHA256: "capacity-digest",
			BaselineIDs:    []string{"bl-1"},
		}, nil
	}
}

func auditSection() documents.ReportSection {
	return documents.ReportSection{ScheduleAudit: &documents.ScheduleAuditSection{ChartID: "chart-1", ReportingDate: "2026-06-15", Profile: "default"}}
}

func TestExportRunsEachAuditOnceAndRecordsThatRun(t *testing.T) {
	database, _, _ := newReportFixture(t, "approved")
	exportsDir := filepath.Join(t.TempDir(), "exports")
	calls := 0
	service := Service{Database: database, ResolveAudit: sequenceResolver(&calls), AppVersion: "1.1.0-test"}

	path, err := service.Export(ExportRequest{
		ReportTitle: "Audit Pack",
		Sections:    []documents.ReportSection{auditSection()},
		Options:     Options{ProfileID: "custom"},
		ExportsDir:  exportsDir,
		FileStem:    "Audit Pack",
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if calls != 1 {
		t.Fatalf("audit ran %d times in one export, want 1", calls)
	}
	raw, err := os.ReadFile(path + ".manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest provenanceManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(manifest.Audits) != 1 {
		t.Fatalf("manifest audits = %+v, want one", manifest.Audits)
	}
	got := manifest.Audits[0]
	// The issues printed in the report and the manifest both come from the
	// first (and only) run: one error, SQ-09 not evaluated, revision rev-1.
	if got.Revision != "rev-1" || got.ChartSHA256 != "chart-digest-1" || got.ChartID != "chart-1" ||
		got.ChartTitle != "Master schedule" || got.ProjectStart != "2026-06-01" || got.CountryCode != "US" ||
		got.CapacitySHA256 != "capacity-digest" || got.Profile != "GoPMgr default" ||
		got.ReportingDate != "2026-06-15" || got.EngineVersion != "schedaudit/1" || got.AppVersion != "1.1.0-test" {
		t.Fatalf("audit provenance = %+v", got)
	}
	if !reflect.DeepEqual(got.BaselineIDs, []string{"bl-1"}) || !reflect.DeepEqual(got.NotEvaluated, []string{"SQ-09"}) ||
		!reflect.DeepEqual(got.Counts, map[string]int{"error": 1, "warning": 0, "info": 0}) {
		t.Fatalf("audit provenance lists = %+v", got)
	}
	var codes []string
	for _, issue := range manifest.Issues {
		codes = append(codes, issue.Code+":"+issue.Message)
	}
	if !reflect.DeepEqual(codes, []string{"schedule_audit_rules_not_evaluated:Schedule audit rules not evaluated: SQ-09"}) {
		t.Fatalf("manifest issues = %v", codes)
	}
	// result_sha256 pins the printed result: recompute it from the first
	// run's result, which a second run would not reproduce.
	again := 0
	first, _ := sequenceResolver(&again)(context.Background(), *auditSection().ScheduleAudit)
	encoded, err := json.Marshal(first.Result)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%x", sha256.Sum256(encoded)); got.ResultSHA256 != want {
		t.Fatalf("result digest = %s, want %s", got.ResultSHA256, want)
	}
}

func TestExportWritesNothingWhenAnAuditCannotRun(t *testing.T) {
	database, _, _ := newReportFixture(t, "approved")
	exportsDir := filepath.Join(t.TempDir(), "exports")
	auditErr := errors.New("chart has no tasks")
	service := Service{Database: database, ResolveAudit: func(context.Context, documents.ScheduleAuditSection) (AuditResolution, error) {
		return AuditResolution{}, auditErr
	}}
	sections := []documents.ReportSection{auditSection()}

	preflight, err := service.Preflight(sections, Options{ProfileID: "custom"})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(preflight.Issues) != 1 || preflight.Issues[0].Code != "schedule_audit_unavailable" || preflight.Issues[0].Severity != "error" ||
		!strings.Contains(preflight.Issues[0].Message, "chart has no tasks") {
		t.Fatalf("Preflight issues = %+v, want one schedule_audit_unavailable error", preflight.Issues)
	}
	_, err = service.Export(ExportRequest{ReportTitle: "Broken", Sections: sections, Options: Options{ProfileID: "custom"}, ExportsDir: exportsDir, FileStem: "Broken"})
	if !errors.Is(err, auditErr) {
		t.Fatalf("Export error = %v, want the audit error", err)
	}
	if _, err := os.Stat(exportsDir); !os.IsNotExist(err) {
		t.Fatalf("exports directory exists after a failed audit: %v", err)
	}
}

func TestReportSectionNeedsExactlyOneSource(t *testing.T) {
	database, _, document := newReportFixture(t, "approved")
	calls := 0
	service := Service{Database: database, ResolveAudit: sequenceResolver(&calls)}
	both := auditSection()
	both.DocumentID = document.ID
	for name, section := range map[string]documents.ReportSection{
		"both":    both,
		"neither": {Title: "Empty"},
	} {
		t.Run(name, func(t *testing.T) {
			sections := []documents.ReportSection{section}
			preflight, err := service.Preflight(sections, Options{ProfileID: "custom"})
			if err != nil {
				t.Fatalf("Preflight: %v", err)
			}
			if len(preflight.Issues) != 1 || preflight.Issues[0].Code != "section_source_invalid" || preflight.Issues[0].Severity != "error" {
				t.Fatalf("Preflight issues = %+v, want one section_source_invalid error", preflight.Issues)
			}
			exportsDir := filepath.Join(t.TempDir(), "exports")
			if _, err := service.Export(ExportRequest{ReportTitle: "Bad", Sections: sections, Options: Options{ProfileID: "custom"}, ExportsDir: exportsDir, FileStem: "Bad"}); err == nil {
				t.Fatal("Export accepted a section without exactly one source")
			}
			if _, err := os.Stat(exportsDir); !os.IsNotExist(err) {
				t.Fatalf("exports directory exists after refusal: %v", err)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("audit ran %d times for invalid sections, want 0", calls)
	}
}

func TestCertifiedReportRefusesAScheduleAudit(t *testing.T) {
	database, _, _ := newReportFixture(t, "approved")
	calls := 0
	service := Service{Database: database, ResolveAudit: sequenceResolver(&calls)}
	sections := []documents.ReportSection{auditSection()}

	preflight, err := service.Preflight(sections, Options{ProfileID: "custom", Mode: documents.ReportModeCertified})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	found := false
	for _, issue := range preflight.Issues {
		found = found || (issue.Code == "schedule_audit_not_certifiable" && issue.Severity == "error")
	}
	if preflight.Ready || !found {
		t.Fatalf("certified preflight = %+v, want not ready with schedule_audit_not_certifiable", preflight)
	}
	exportsDir := filepath.Join(t.TempDir(), "exports")
	if _, err := service.Export(ExportRequest{ReportTitle: "Cert", Sections: sections, Options: Options{ProfileID: "custom", Mode: documents.ReportModeCertified}, ExportsDir: exportsDir, FileStem: "Cert"}); err == nil {
		t.Fatal("certified Export accepted a schedule audit")
	}
	if _, err := os.Stat(exportsDir); !os.IsNotExist(err) {
		t.Fatalf("exports directory exists after refusal: %v", err)
	}
}
