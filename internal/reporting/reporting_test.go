// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package reporting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/kernel"
)

func TestExportWritesPrivatePDFAndProvenanceManifest(t *testing.T) {
	database, project, document := newReportFixture(t, "approved")
	exportsDir := filepath.Join(t.TempDir(), "exports")
	now := time.Date(2026, 8, 8, 12, 34, 56, 0, time.UTC)
	service := Service{Database: database, Now: func() time.Time { return now }}

	path, err := service.Export(ExportRequest{
		ReportTitle: "Governance Pack",
		Sections:    []documents.ReportSection{{DocumentID: document.ID}},
		Options:     Options{ProfileID: "custom", Mode: documents.ReportModeCertified},
		Author:      "Alice",
		ExportsDir:  exportsDir,
		FileStem:    "Governance Pack",
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if want := filepath.Join(exportsDir, "Governance Pack-20260808-123456.pdf"); path != want {
		t.Fatalf("output path = %q, want %q", path, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat PDF: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("PDF permissions = %v, want 0600", got)
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PDF: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatal("combined report does not start with a PDF header")
	}

	manifestBytes, err := os.ReadFile(path + ".manifest.json")
	if err != nil {
		t.Fatalf("read provenance manifest: %v", err)
	}
	manifestInfo, err := os.Stat(path + ".manifest.json")
	if err != nil {
		t.Fatalf("stat provenance manifest: %v", err)
	}
	if got := manifestInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("manifest permissions = %v, want 0600", got)
	}
	var manifest provenanceManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode provenance manifest: %v", err)
	}
	if manifest.Format != "gopmgr-report-provenance/v1" || manifest.ReportTitle != "Governance Pack" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if len(manifest.Documents) != 1 || manifest.Documents[0].ID != document.ID {
		t.Fatalf("manifest documents = %+v", manifest.Documents)
	}
	digest := sha256.Sum256([]byte(document.Content))
	if got, want := manifest.Documents[0].SHA256, hex.EncodeToString(digest[:]); got != want {
		t.Fatalf("document digest = %q, want %q", got, want)
	}
	if project.ID == "" {
		t.Fatal("fixture project was not persisted")
	}
}

func TestPreflightAndExportRejectMissingDocumentForCertifiedReport(t *testing.T) {
	database, _, _ := newReportFixture(t, "approved")
	service := Service{Database: database}
	sections := []documents.ReportSection{{DocumentID: "missing-document"}}

	preflight, err := service.Preflight(sections, Options{ProfileID: "custom", Mode: documents.ReportModeCertified})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if preflight.Ready || len(preflight.Issues) != 1 || preflight.Issues[0].Code != "document_missing" {
		t.Fatalf("preflight = %+v, want one blocking missing-document issue", preflight)
	}
	if _, err := service.Export(ExportRequest{
		ReportTitle: "Missing",
		Sections:    sections,
		Options:     Options{ProfileID: "custom", Mode: documents.ReportModeCertified},
		ExportsDir:  t.TempDir(),
		FileStem:    "Missing",
	}); err == nil || !strings.Contains(err.Error(), "certified report preflight failed") {
		t.Fatalf("Export error = %v, want certified preflight failure", err)
	}
}

func TestExportRejectsPathSeparatorsInFileStem(t *testing.T) {
	database, _, document := newReportFixture(t, "approved")
	service := Service{Database: database}

	if _, err := service.Export(ExportRequest{
		ReportTitle: "Unsafe",
		Sections:    []documents.ReportSection{{DocumentID: document.ID}},
		Options:     Options{ProfileID: "custom"},
		ExportsDir:  t.TempDir(),
		FileStem:    "../unsafe",
	}); err == nil || !strings.Contains(err.Error(), "path separators") {
		t.Fatalf("Export error = %v, want path-separator rejection", err)
	}
}

func TestExportDoesNotWriteWhenEVMResolutionFails(t *testing.T) {
	database, _, document := newReportFixture(t, "approved")
	exportsDir := filepath.Join(t.TempDir(), "exports")
	resolveErr := errors.New("EVM total exceeds range")
	service := Service{
		Database: database,
		ResolveEVM: func(db.Project, map[string]documents.ResolvedChart, time.Time) (map[string]*kernel.EVMetrics, error) {
			return nil, resolveErr
		},
	}

	_, err := service.Export(ExportRequest{
		ReportTitle: "Overflow",
		Sections:    []documents.ReportSection{{DocumentID: document.ID}},
		Options:     Options{ProfileID: "custom"},
		ExportsDir:  exportsDir,
		FileStem:    "Overflow",
	})
	if !errors.Is(err, resolveErr) {
		t.Fatalf("Export error = %v, want resolver error", err)
	}
	if _, err := os.Stat(exportsDir); !os.IsNotExist(err) {
		t.Fatalf("exports directory exists after failed EVM resolution: %v", err)
	}
}

func TestChartReferences(t *testing.T) {
	fields := []documents.Field{
		{Key: "chart_id", Type: documents.FieldChartRef},
		{Key: "title", Type: documents.FieldText},
	}
	if got, want := ChartReferences(`{"chart_id":"chart-1","title":"Report"}`, fields), []string{"chart-1"}; !equalStrings(got, want) {
		t.Fatalf("ChartReferences() = %v, want %v", got, want)
	}
	if got := ChartReferences(`{`, fields); got != nil {
		t.Fatalf("invalid JSON references = %v, want nil", got)
	}
}

func newReportFixture(t *testing.T, status string) (*db.Database, db.Project, db.Document) {
	t.Helper()
	database, err := db.InitDB(filepath.Join(t.TempDir(), "project.gopmgr"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	project, err := database.UpsertProject(db.Project{Name: "Atlas", Industry: "custom", Status: "planning", Phase: "planning", StartDate: "2026-08-01"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	document, err := database.SaveDocument(db.Document{
		ProjectID: project.ID,
		Kind:      string(documents.KindProjectCharterWord),
		Title:     "Project Charter",
		Content:   `{"project_name":"Atlas"}`,
		Status:    status,
	})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	return database, project, document
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
