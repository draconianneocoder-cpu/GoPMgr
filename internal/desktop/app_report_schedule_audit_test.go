// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

// Both signed entry points include a schedule-audit section as an unsigned
// export does: the report service resolves the audit and records it in the
// provenance manifest. On the PAdES path the signature stays the final PDF
// mutation, so its ByteRange reaches the last byte of the file.
func TestSignedReportsIncludeAScheduleAudit(t *testing.T) {
	auditedChart := func(t *testing.T, pdfPath, chartID string) {
		t.Helper()
		raw, err := os.ReadFile(pdfPath + ".manifest.json")
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		var manifest struct {
			Audits []struct {
				ChartID string `json:"chart_id"`
			} `json:"audits"`
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		if len(manifest.Audits) != 1 || manifest.Audits[0].ChartID != chartID {
			t.Fatalf("manifest audits = %s, want the audit of %s", raw, chartID)
		}
	}

	t.Run("PAdES", func(t *testing.T) {
		app, chartID := openAuditReportProject(t)
		sections := []documents.ReportSection{{ScheduleAudit: &documents.ScheduleAuditSection{ChartID: chartID, ReportingDate: "2026-06-10", Profile: "default"}}}
		project, err := app.GetProjectMeta()
		if err != nil {
			t.Fatalf("GetProjectMeta: %v", err)
		}
		path, err := app.exportCombinedReportSignedWithRuntime("Signed", "", sections, "test-signer.p12", "test-password",
			newAppPAdESTestRuntime(t, rfc3161.TrustVerified, time.Now().UTC().Truncate(time.Second)))
		if err != nil {
			t.Fatalf("signed export with an audit: %v", err)
		}
		assertAppPAdESTPDF(t, path)
		auditedChart(t, path, chartID)

		pdf, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ranges := regexp.MustCompile(`/ByteRange \[\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s*\]`).FindAllSubmatch(pdf, -1)
		if len(ranges) == 0 {
			t.Fatal("no ByteRange in the signed PDF")
		}
		last := ranges[len(ranges)-1]
		offset, _ := strconv.Atoi(string(last[3]))
		length, _ := strconv.Atoi(string(last[4]))
		if offset+length != len(pdf) {
			t.Fatalf("signature covers bytes up to %d of %d: the PDF changed after signing", offset+length, len(pdf))
		}
		reportID := combinedReportCheckpointID(project.ID, "Signed", "", sections)
		if status, _ := combinedReportSignatureEvent(t, app, project.ID, reportID); status != "pades_t_verified" {
			t.Fatalf("signature status = %q, want pades_t_verified", status)
		}
	})

	t.Run("GnuPG", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix executable fixture")
		}
		app, chartID := openAuditReportProject(t)
		sections := []documents.ReportSection{{ScheduleAudit: &documents.ScheduleAuditSection{ChartID: chartID, Profile: "default"}}}
		// A stand-in gpg writes a fixed signature to its --output path.
		fixture := t.TempDir()
		script := "#!/bin/sh\nset -eu\nout=\nfor arg do\n if [ \"${previous:-}\" = --output ]; then out=$arg; fi\n previous=$arg\ndone\nprintf 'fixture signature' > \"$out\"\n"
		if err := os.WriteFile(filepath.Join(fixture, "gpg"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", fixture+string(os.PathListSeparator)+os.Getenv("PATH"))
		result, err := app.ExportCombinedReportGnuPG("Signed", "", sections, "fixture-key")
		if err != nil {
			t.Fatalf("GnuPG export with an audit: %v", err)
		}
		auditedChart(t, result.PDFPath, chartID)
		if sig, err := os.ReadFile(result.SignaturePath); err != nil || string(sig) != "fixture signature" {
			t.Fatalf("signature file = %q, %v", sig, err)
		}
	})
}
