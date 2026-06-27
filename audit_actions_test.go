// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"pmforge/internal/agile"
	"pmforge/internal/db"
)

// mustOpenProject is a test helper that creates a project and opens it so
// that a.db is non-nil.  It returns the path of the project file.
func mustOpenProject(t *testing.T, app *App, name string) string {
	t.Helper()
	pf, err := app.CreateProject(name, "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := app.OpenProject(pf.Path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	return pf.Path
}

// auditCount queries the audit_log table in the currently-open project DB and
// returns the number of rows matching the given action and targetID.
func auditCount(t *testing.T, app *App, action, targetID string) int {
	t.Helper()
	app.mu.RLock()
	conn := app.db.Conn
	app.mu.RUnlock()
	var n int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM audit_log WHERE action = ? AND target_id = ?`,
		action, targetID,
	).Scan(&n); err != nil {
		t.Fatalf("audit_log query: %v", err)
	}
	return n
}

func auditEventSignatureStatus(t *testing.T, app *App, projectID, docID string) string {
	t.Helper()
	app.mu.RLock()
	conn := app.db.Conn
	app.mu.RUnlock()
	var status string
	if err := conn.QueryRow(
		`SELECT signature_status
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.signature'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		projectID,
		docID,
	).Scan(&status); err != nil {
		t.Fatalf("signature audit event query: %v", err)
	}
	return status
}

// TestCloneOpenProject_DataSurvivesSnapshot verifies that data committed to
// the open project is present in the clone.  This is the correctness invariant
// of the WAL fix: VACUUM INTO produces a fully-checkpointed snapshot, so any
// chart saved before the clone must be readable in it.  A raw copyFile could
// miss data that was committed to the WAL but not yet checkpointed to the main
// file.
func TestCloneOpenProject_DataSurvivesSnapshot(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	srcPath := mustOpenProject(t, app, "Source Plan")

	// Commit a chart to the open project (this write goes into the WAL).
	chart, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Snapshot WBS"})
	if err != nil {
		t.Fatalf("SaveChart before clone: %v", err)
	}

	// Clone while the project is open.  a.db != nil && samePath(a.dbPath, src)
	// triggers the VACUUM INTO path.
	clone, err := app.CloneProject(srcPath)
	if err != nil {
		t.Fatalf("CloneProject (open project): %v", err)
	}
	if clone.Path == srcPath {
		t.Fatal("clone path equals source path")
	}
	if clone.Name != "Source Plan copy" {
		t.Fatalf("clone name = %q, want %q", clone.Name, "Source Plan copy")
	}

	// Open the clone as the active project and verify the chart is present.
	// If VACUUM INTO missed WAL data, ListCharts would return empty here.
	if _, err := app.OpenProject(clone.Path); err != nil {
		t.Fatalf("OpenProject on clone: %v", err)
	}
	charts, err := app.ListCharts("")
	if err != nil {
		t.Fatalf("ListCharts in clone: %v", err)
	}
	found := false
	for _, c := range charts {
		if c.ID == chart.ID && c.Title == chart.Title {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chart %q (id %s) not found in clone; charts = %v", chart.Title, chart.ID, charts)
	}
}

// TestDeleteChart_WritesAuditLog confirms that DeleteChart records a
// delete_chart entry in the audit_log before removing the chart.
func TestDeleteChart_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	chart, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Audit WBS"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if err := app.DeleteChart(chart.ID); err != nil {
		t.Fatalf("DeleteChart: %v", err)
	}

	if n := auditCount(t, app, "delete_chart", chart.ID); n != 1 {
		t.Fatalf("audit_log delete_chart count = %d, want 1", n)
	}
}

// TestDeleteDocument_WritesAuditLog confirms that DeleteDocument records a
// delete_document entry in the audit_log before removing the document.
func TestDeleteDocument_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	doc, err := app.NewDocument("charter_word", "Audit Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	if err := app.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	if n := auditCount(t, app, "delete_document", doc.ID); n != 1 {
		t.Fatalf("audit_log delete_document count = %d, want 1", n)
	}
}

func TestExportDocumentPDFSignedFailureWritesAuditEvent(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Signature Audit Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	doc, err := app.NewDocument("charter_word", "Signature Audit Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	if _, err := app.ExportDocumentPDFSigned(doc.ID, "/missing/certificate.p12", "bad-password"); err == nil {
		t.Fatal("ExportDocumentPDFSigned unexpectedly succeeded with a missing certificate")
	}

	if got := auditEventSignatureStatus(t, app, project.ID, doc.ID); got != "failed" {
		t.Fatalf("signature_status = %q, want failed", got)
	}
}

func TestExportAuditVerificationReportWritesPrivateJSONArtifact(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Verification Report Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	if _, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Verification WBS"}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	outPath, err := app.ExportAuditVerificationReport()
	if err != nil {
		t.Fatalf("ExportAuditVerificationReport: %v", err)
	}
	if !strings.Contains(outPath, "exports") || !strings.HasSuffix(outPath, ".json") {
		t.Fatalf("report path = %q, want JSON under exports", outPath)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat report: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("report permissions = %v, want 0600", got)
	}
	var report struct {
		ProjectID         string `json:"project_id"`
		Valid             bool   `json:"valid"`
		CheckedEvents     int    `json:"checked_events"`
		TerminalEventHash string `json:"terminal_event_hash"`
		GeneratedAtUTC    string `json:"generated_at_utc"`
	}
	bytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if err := json.Unmarshal(bytes, &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.ProjectID != project.ID {
		t.Fatalf("project_id = %q, want %q", report.ProjectID, project.ID)
	}
	if !report.Valid {
		t.Fatal("valid = false, want true")
	}
	if report.CheckedEvents < 2 {
		t.Fatalf("checked_events = %d, want at least 2", report.CheckedEvents)
	}
	if report.TerminalEventHash == "" {
		t.Fatal("terminal_event_hash is empty")
	}
	if report.GeneratedAtUTC == "" {
		t.Fatal("generated_at_utc is empty")
	}
}

func TestExportAuditRepairEvidencePreservesInvalidChain(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Repair Evidence Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	if _, err := app.SaveChart(db.Chart{Kind: "wbs", Title: "Evidence WBS"}); err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	app.mu.RLock()
	conn := app.db.Conn
	app.mu.RUnlock()
	if _, err := conn.Exec(
		`UPDATE audit_events
		 SET after_canonical_json = ?
		 WHERE project_id = ? AND sequence_number = (
		   SELECT MAX(sequence_number) FROM audit_events WHERE project_id = ?
		 )`,
		`{"tampered":true}`, project.ID, project.ID,
	); err != nil {
		t.Fatalf("tamper audit event: %v", err)
	}

	outPath, err := app.ExportAuditRepairEvidence()
	if err != nil {
		t.Fatalf("ExportAuditRepairEvidence: %v", err)
	}
	if !strings.Contains(outPath, "exports") || !strings.HasSuffix(outPath, ".json") {
		t.Fatalf("evidence path = %q, want JSON under exports", outPath)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat evidence: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("evidence permissions = %v, want 0600", got)
	}
	var evidence struct {
		ProjectID    string `json:"project_id"`
		GeneratedAt  string `json:"generated_at_utc"`
		Verification struct {
			Valid              bool   `json:"valid"`
			CheckedEvents      int    `json:"checked_events"`
			FirstInvalidReason string `json:"first_invalid_reason"`
		} `json:"verification"`
		Events []struct {
			SequenceNumber      int64  `json:"sequence_number"`
			AfterCanonicalJSON  string `json:"after_canonical_json"`
			SignatureStatus     string `json:"signature_status"`
			SignatureBlobLength int    `json:"signature_blob_length"`
		} `json:"events"`
	}
	bytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if err := json.Unmarshal(bytes, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if evidence.ProjectID != project.ID {
		t.Fatalf("project_id = %q, want %q", evidence.ProjectID, project.ID)
	}
	if evidence.GeneratedAt == "" {
		t.Fatal("generated_at_utc is empty")
	}
	if evidence.Verification.Valid {
		t.Fatal("verification.valid = true, want false")
	}
	if evidence.Verification.FirstInvalidReason != "event hash mismatch" {
		t.Fatalf("first_invalid_reason = %q, want event hash mismatch", evidence.Verification.FirstInvalidReason)
	}
	if len(evidence.Events) < 2 {
		t.Fatalf("events length = %d, want at least 2", len(evidence.Events))
	}
	if got := evidence.Events[len(evidence.Events)-1].AfterCanonicalJSON; got != `{"tampered":true}` {
		t.Fatalf("last after_canonical_json = %q, want tampered JSON", got)
	}
}

// TestDeleteWorkItem_WritesAuditLog confirms that DeleteWorkItem records a
// delete_work_item entry in the audit_log before removing the work item.
func TestDeleteWorkItem_WritesAuditLog(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Audit Plan")

	if err := app.SetAgileEnabled(true); err != nil {
		t.Fatalf("SetAgileEnabled: %v", err)
	}
	bwc, err := app.EnsureDefaultBoard()
	if err != nil {
		t.Fatalf("EnsureDefaultBoard: %v", err)
	}
	defaultState := ""
	if len(bwc.Columns) > 0 {
		defaultState = bwc.Columns[0].ID
	}

	wi, err := app.SaveWorkItem(agile.WorkItem{
		Type:  agile.WorkItemStory,
		Title: "Audit Story",
		State: defaultState,
	})
	if err != nil {
		t.Fatalf("SaveWorkItem: %v", err)
	}

	if err := app.DeleteWorkItem(wi.ID); err != nil {
		t.Fatalf("DeleteWorkItem: %v", err)
	}

	if n := auditCount(t, app, "delete_work_item", wi.ID); n != 1 {
		t.Fatalf("audit_log delete_work_item count = %d, want 1", n)
	}
}
