// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopmgr/internal/documents"
	"gopmgr/internal/rfc3161"
	"gopmgr/internal/update"
)

// openAuditReportProject signs in, opens an encrypted project starting on
// Monday 2026-06-01 and stores a CPM chart whose MFO on B cannot be met: A
// takes five working days, so B cannot finish on Wednesday 2026-06-03.
func openAuditReportProject(t *testing.T) (*App, string) {
	t.Helper()
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	acceptNoRecoveryCodes(t, app)
	mustOpenProject(t, app, "Audit Report Plan")
	d := app.requireDB()
	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	proj.StartDate = "2026-06-01"
	if proj, err = d.UpsertProject(proj); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	c := auditChart(t, d, proj.ID, "Master schedule", `{"nodes":[
		{"id":"A","label":"Design","duration":5},
		{"id":"B","label":"Build","duration":3,"constraint":"MFO","constraint_date":"2026-06-03"}],
		"edges":[{"from":"A","to":"B"}]}`)
	return app, c.ID
}

// An unsigned combined report with a schedule-audit section runs the real
// audit on the stored chart and records, in its provenance manifest, the
// chart and inputs that audit read.
func TestCombinedReportRecordsTheScheduleAuditItRan(t *testing.T) {
	app, chartID := openAuditReportProject(t)
	d := app.requireDB()

	path, err := app.ExportCombinedReportWithOptions("Audit Pack", "", []documents.ReportSection{{
		ScheduleAudit: &documents.ScheduleAuditSection{ChartID: chartID, ReportingDate: "2026-06-10", Profile: "default"},
	}}, CombinedReportOptions{ProfileID: "custom"})
	if err != nil {
		t.Fatalf("ExportCombinedReportWithOptions: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat report: %v", err)
	}
	raw, err := os.ReadFile(path + ".manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Audits []struct {
			ChartID       string         `json:"chart_id"`
			ChartTitle    string         `json:"chart_title"`
			Revision      string         `json:"revision"`
			ChartSHA256   string         `json:"chart_sha256"`
			ProjectStart  string         `json:"project_start"`
			BaselineIDs   []string       `json:"baseline_ids"`
			ReportingDate string         `json:"reporting_date"`
			AppVersion    string         `json:"app_version"`
			Counts        map[string]int `json:"counts"`
			NotEvaluated  []string       `json:"not_evaluated"`
		} `json:"audits"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(manifest.Audits) != 1 {
		t.Fatalf("manifest audits = %s", raw)
	}
	got := manifest.Audits[0]
	stored, err := d.GetChart(chartID)
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(stored.Data+"\n"+stored.Config)))
	if got.ChartID != chartID || got.ChartTitle != "Master schedule" || got.Revision != stored.UpdatedAt ||
		got.ChartSHA256 != wantDigest || got.ProjectStart != "2026-06-01" || got.ReportingDate != "2026-06-10" ||
		got.AppVersion != update.CurrentVersion || len(got.BaselineIDs) != 0 || len(got.NotEvaluated) != 0 {
		t.Fatalf("audit provenance = %+v, want chart %s at revision %s, digest %s", got, chartID, stored.UpdatedAt, wantDigest)
	}
	// Hand-computed: B must finish by working day 3, so its latest start is
	// day 0 and A, its predecessor, must finish by day 0: B's MFO is violated
	// (SQ-09) and both A and B have float -5 (SQ-11). No other rule reports an
	// error on this chart.
	if got.Counts["error"] != 3 {
		t.Fatalf("error findings = %d, want 3 (SQ-09 on B, SQ-11 on A and B)", got.Counts["error"])
	}
}

// Both signed entry points refuse a schedule-audit section before they pick
// a destination, sign or write anything.
func TestSignedReportsRefuseAScheduleAudit(t *testing.T) {
	app, chartID := openAuditReportProject(t)
	sections := []documents.ReportSection{{ScheduleAudit: &documents.ScheduleAuditSection{ChartID: chartID, Profile: "default"}}}
	exports := filepath.Join(app.CurrentUser().DataDir, "exports")

	for name, export := range map[string]func() error{
		"PAdES": func() error {
			_, err := app.exportCombinedReportSignedWithRuntime("Signed", "", sections, "test-signer.p12", "test-password",
				newAppPAdESTestRuntime(t, rfc3161.TrustVerified, time.Now().UTC().Truncate(time.Second)))
			return err
		},
		"GnuPG": func() error {
			_, err := app.ExportCombinedReportGnuPG("Signed", "", sections, "0123456789ABCDEF")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := export(); !errors.Is(err, errSignedScheduleAudit) {
				t.Fatalf("error = %v, want errSignedScheduleAudit", err)
			}
			entries, err := os.ReadDir(exports)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("exports written after refusal: %v", entries)
			}
		})
	}
}
