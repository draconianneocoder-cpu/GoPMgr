// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"strings"
	"testing"
)

func TestAuditEventsSchemaHasTamperEvidentColumns(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("audit_events")
	if err != nil {
		t.Fatalf("columnSet(audit_events): %v", err)
	}
	for _, name := range []string{
		"id",
		"project_id",
		"sequence_number",
		"previous_event_hash",
		"event_hash",
		"event_type",
		"entity_type",
		"entity_id",
		"before_canonical_json",
		"after_canonical_json",
		"user_id",
		"session_id",
		"timestamp_utc",
		"signature_status",
		"signature_blob_optional",
	} {
		if _, ok := cols[name]; !ok {
			t.Fatalf("audit_events missing column %q", name)
		}
	}
}

func TestAppendAuditEventChainsCanonicalHashes(t *testing.T) {
	d := newBackupTestDB(t)

	first, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  "project-1",
		EventType:  "create",
		EntityType: "chart",
		EntityID:   "chart-1",
		AfterJSON:  `{"b":2,"a":1}`,
		UserID:     "user-1",
		SessionID:  "session-1",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent first: %v", err)
	}
	second, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  "project-1",
		EventType:  "update",
		EntityType: "chart",
		EntityID:   "chart-1",
		BeforeJSON: `{"a":1,"b":2}`,
		AfterJSON:  `{"a":2,"b":2}`,
		UserID:     "user-1",
		SessionID:  "session-1",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent second: %v", err)
	}

	if first.SequenceNumber != 1 || second.SequenceNumber != 2 {
		t.Fatalf("sequences = %d, %d; want 1, 2", first.SequenceNumber, second.SequenceNumber)
	}
	if first.PreviousEventHash != "" {
		t.Fatalf("first previous hash = %q, want empty", first.PreviousEventHash)
	}
	if second.PreviousEventHash != first.EventHash {
		t.Fatalf("second previous hash = %q, want %q", second.PreviousEventHash, first.EventHash)
	}
	if first.AfterCanonicalJSON != `{"a":1,"b":2}` {
		t.Fatalf("canonical after JSON = %s", first.AfterCanonicalJSON)
	}
	if first.EventHash == "" || second.EventHash == "" || first.EventHash == second.EventHash {
		t.Fatalf("unexpected event hashes: first=%q second=%q", first.EventHash, second.EventHash)
	}

	report, err := d.VerifyAuditChain("project-1")
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 2 {
		t.Fatalf("verification = %+v, want valid with 2 checked events", report)
	}
}

func TestVerifyAuditChainDetectsTampering(t *testing.T) {
	d := newBackupTestDB(t)
	event, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  "project-1",
		EventType:  "create",
		EntityType: "document",
		EntityID:   "doc-1",
		AfterJSON:  `{"title":"Original"}`,
		UserID:     "user-1",
		SessionID:  "session-1",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	if _, err := d.Conn.Exec(
		`UPDATE audit_events SET after_canonical_json = ? WHERE id = ?`,
		`{"title":"Tampered"}`,
		event.ID,
	); err != nil {
		t.Fatalf("tamper event: %v", err)
	}

	report, err := d.VerifyAuditChain("project-1")
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if report.Valid || report.FirstInvalidSequence != 1 {
		t.Fatalf("verification = %+v, want invalid at sequence 1", report)
	}
}

func TestUpsertProjectAppendsCreateAndUpdateAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)

	project, err := d.UpsertProject(Project{Name: "Audited Project"})
	if err != nil {
		t.Fatalf("UpsertProject create: %v", err)
	}
	project.Description = "metadata changed"
	if _, err := d.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject update: %v", err)
	}

	rows, err := d.Conn.Query(
		`SELECT sequence_number, event_type, entity_type, entity_id
		 FROM audit_events
		 WHERE project_id = ?
		 ORDER BY sequence_number ASC`,
		project.ID,
	)
	if err != nil {
		t.Fatalf("query audit_events: %v", err)
	}
	defer rows.Close()

	want := []struct {
		seq       int64
		eventType string
	}{
		{1, "project.create"},
		{2, "project.update"},
	}
	for _, w := range want {
		if !rows.Next() {
			t.Fatalf("missing audit event %+v", w)
		}
		var seq int64
		var eventType, entityType, entityID string
		if err := rows.Scan(&seq, &eventType, &entityType, &entityID); err != nil {
			t.Fatalf("scan audit event: %v", err)
		}
		if seq != w.seq || eventType != w.eventType || entityType != "project" || entityID != project.ID {
			t.Fatalf("audit event = seq:%d type:%q entity:%q id:%q, want seq:%d type:%q entity:project id:%q",
				seq, eventType, entityType, entityID, w.seq, w.eventType, project.ID)
		}
	}
	if rows.Next() {
		t.Fatal("unexpected extra project audit event")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("audit rows: %v", err)
	}

	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 2 {
		t.Fatalf("verification = %+v, want valid with 2 checked events", report)
	}
}

func TestSaveChartAppendsCreateUpdateAndDeleteAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Chart Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	chart, err := d.SaveChart(Chart{
		ProjectID: project.ID,
		Kind:      "bar",
		Title:     "Baseline chart",
		Data:      `{"series":[1]}`,
	})
	if err != nil {
		t.Fatalf("SaveChart create: %v", err)
	}
	chart.Title = "Updated chart"
	if _, err := d.SaveChart(chart); err != nil {
		t.Fatalf("SaveChart update: %v", err)
	}
	if err := d.DeleteChart(chart.ID); err != nil {
		t.Fatalf("DeleteChart: %v", err)
	}

	assertAuditEventTypes(t, d, project.ID, "chart", []string{
		"chart.create",
		"chart.update",
		"chart.delete",
	})
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 4 {
		t.Fatalf("verification = %+v, want valid with 4 checked events", report)
	}
}

func TestSaveDocumentAppendsCreateUpdateAndDeleteAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Document Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	doc, err := d.SaveDocument(Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Original charter",
		Content:   `{"summary":"original"}`,
	})
	if err != nil {
		t.Fatalf("SaveDocument create: %v", err)
	}
	doc.Title = "Updated charter"
	if _, err := d.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument update: %v", err)
	}
	if err := d.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	assertAuditEventTypes(t, d, project.ID, "document", []string{
		"document.create",
		"document.update",
		"document.delete",
	})
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 4 {
		t.Fatalf("verification = %+v, want valid with 4 checked events", report)
	}
}

func TestSaveDocumentApprovedStatusAppendsSignedCheckpoint(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Document Approval Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(Document{
		ProjectID: project.ID,
		Kind:      "charter",
		Title:     "Approval Charter",
		Content:   `{"summary":"ready for approval"}`,
		Status:    "review",
	})
	if err != nil {
		t.Fatalf("SaveDocument create: %v", err)
	}

	doc.Status = "approved"
	approved, err := d.SaveDocument(doc)
	if err != nil {
		t.Fatalf("SaveDocument approve: %v", err)
	}
	approved.Title = "Approved Charter"
	if _, err := d.SaveDocument(approved); err != nil {
		t.Fatalf("SaveDocument approved update: %v", err)
	}

	var checkpointCount int
	if err := d.Conn.QueryRow(
		`SELECT COUNT(*)
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.approval_checkpoint'`,
		project.ID,
		doc.ID,
	).Scan(&checkpointCount); err != nil {
		t.Fatalf("query document approval checkpoint: %v", err)
	}
	if checkpointCount != 1 {
		t.Fatalf("approval checkpoint count = %d, want 1", checkpointCount)
	}
	var signatureStatus, signatureBlob, payload string
	if err := d.Conn.QueryRow(
		`SELECT signature_status, signature_blob_optional, after_canonical_json
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'document' AND entity_id = ? AND event_type = 'document.approval_checkpoint'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		project.ID,
		doc.ID,
	).Scan(&signatureStatus, &signatureBlob, &payload); err != nil {
		t.Fatalf("load document approval checkpoint: %v", err)
	}
	if signatureStatus != "signed" {
		t.Fatalf("signature_status = %q, want signed", signatureStatus)
	}
	if !containsAuditJSON(signatureBlob, `"payload_hash"`) || !containsAuditJSON(payload, `"approval_type":"document_status_approved"`) {
		t.Fatalf("approval checkpoint blob=%s payload=%s, want signed payload hash and approval type", signatureBlob, payload)
	}
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 5 {
		t.Fatalf("verification = %+v, want valid with 5 checked events", report)
	}
}

func TestSaveBaselineAppendsCreateAndDeleteAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)
	projectID, chartID := newBaselineFixture(t, d)

	baseline, err := d.SaveBaseline(Baseline{
		ProjectID: projectID,
		ChartID:   chartID,
		Name:      "Approval baseline",
		Data:      `{"A":{"id":"A","duration":2}}`,
	})
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	if err := d.DeleteBaseline(baseline.ID); err != nil {
		t.Fatalf("DeleteBaseline: %v", err)
	}

	assertAuditEventTypes(t, d, projectID, "baseline", []string{
		"baseline.create",
		"baseline.delete",
	})
	report, err := d.VerifyAuditChain(projectID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 4 {
		t.Fatalf("verification = %+v, want valid with 4 checked events", report)
	}
}

func TestPromoteScenarioChartToBaselineAppendsSignedApprovalCheckpoint(t *testing.T) {
	d := newBackupTestDB(t)
	projectID, chartID := newBaselineFixture(t, d)
	scenario, err := d.SaveScenario(Scenario{
		ProjectID: projectID,
		Name:      "Approved acceleration",
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}
	branched, err := d.BranchScenarioChart(scenario.ID, chartID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}

	promoted, err := d.PromoteScenarioChartToBaseline(branched.ID, "Approved scenario baseline")
	if err != nil {
		t.Fatalf("PromoteScenarioChartToBaseline: %v", err)
	}

	var signatureStatus, signatureBlob, payload string
	if err := d.Conn.QueryRow(
		`SELECT signature_status, signature_blob_optional, after_canonical_json
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = 'baseline' AND entity_id = ? AND event_type = 'baseline.approval_checkpoint'
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		projectID,
		promoted.ID,
	).Scan(&signatureStatus, &signatureBlob, &payload); err != nil {
		t.Fatalf("query baseline approval checkpoint: %v", err)
	}
	if signatureStatus != "signed" {
		t.Fatalf("signature_status = %q, want signed", signatureStatus)
	}
	if !containsAuditJSON(signatureBlob, `"payload_hash"`) || !containsAuditJSON(payload, `"approval_type":"scenario_promoted_to_baseline"`) {
		t.Fatalf("approval checkpoint blob=%s payload=%s, want signed payload hash and promotion approval type", signatureBlob, payload)
	}
	report, err := d.VerifyAuditChain(projectID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 6 {
		t.Fatalf("verification = %+v, want valid with 6 checked events", report)
	}
}

func TestSaveScenarioAppendsCreateUpdateAndDeleteAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Scenario Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	scenario, err := d.SaveScenario(Scenario{
		ProjectID:   project.ID,
		Name:        "Accelerated delivery",
		Description: "Pull procurement earlier.",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("SaveScenario create: %v", err)
	}
	scenario.Description = "Pull procurement and commissioning earlier."
	if _, err := d.SaveScenario(scenario); err != nil {
		t.Fatalf("SaveScenario update: %v", err)
	}
	if err := d.DeleteScenario(scenario.ID); err != nil {
		t.Fatalf("DeleteScenario: %v", err)
	}

	assertAuditEventTypes(t, d, project.ID, "scenario", []string{
		"scenario.create",
		"scenario.update",
		"scenario.delete",
	})
	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 4 {
		t.Fatalf("verification = %+v, want valid with 4 checked events", report)
	}
}

func TestScenarioActiveSelectionAuditsSiblingDeactivation(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Active Scenario Audit"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	first, err := d.SaveScenario(Scenario{
		ProjectID: project.ID,
		Name:      "First active scenario",
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("SaveScenario first: %v", err)
	}
	if _, err := d.SaveScenario(Scenario{
		ProjectID: project.ID,
		Name:      "Second active scenario",
		IsActive:  true,
	}); err != nil {
		t.Fatalf("SaveScenario second: %v", err)
	}

	var deactivationJSON string
	if err := d.Conn.QueryRow(
		`SELECT after_canonical_json
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = ? AND entity_id = ? AND event_type = ?
		 ORDER BY sequence_number DESC
		 LIMIT 1`,
		project.ID, "scenario", first.ID, "scenario.update",
	).Scan(&deactivationJSON); err != nil {
		t.Fatalf("query deactivation audit event: %v", err)
	}
	if want := `"is_active":false`; !containsAuditJSON(deactivationJSON, want) {
		t.Fatalf("deactivation audit JSON = %s, want it to contain %s", deactivationJSON, want)
	}

	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 4 {
		t.Fatalf("verification = %+v, want valid with 4 checked events", report)
	}
}

func TestScenarioChartWorkflowAppendsCreateAndUpdateAuditEvents(t *testing.T) {
	d := newBackupTestDB(t)
	projectID, chartID := newBaselineFixture(t, d)
	scenario, err := d.SaveScenario(Scenario{
		ProjectID: projectID,
		Name:      "Scenario chart audit",
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("SaveScenario: %v", err)
	}

	branched, err := d.BranchScenarioChart(scenario.ID, chartID, "")
	if err != nil {
		t.Fatalf("BranchScenarioChart: %v", err)
	}
	branched.Title = "Edited scenario copy"
	branched.Data = `{"edited":true}`
	if _, err := d.SaveScenarioChart(branched); err != nil {
		t.Fatalf("SaveScenarioChart: %v", err)
	}

	assertAuditEventTypes(t, d, projectID, "scenario_chart", []string{
		"scenario_chart.create",
		"scenario_chart.update",
	})
	report, err := d.VerifyAuditChain(projectID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid || report.CheckedEvents != 5 {
		t.Fatalf("verification = %+v, want valid with 5 checked events", report)
	}
}

func assertAuditEventTypes(t *testing.T, d *Database, projectID, entityType string, want []string) {
	t.Helper()
	rows, err := d.Conn.Query(
		`SELECT event_type
		 FROM audit_events
		 WHERE project_id = ? AND entity_type = ?
		 ORDER BY sequence_number ASC`,
		projectID,
		entityType,
	)
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatalf("scan audit event: %v", err)
		}
		got = append(got, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("audit rows: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s audit event types = %#v, want %#v", entityType, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s audit event types = %#v, want %#v", entityType, got, want)
		}
	}
}

func containsAuditJSON(raw, needle string) bool {
	return strings.Contains(raw, needle)
}
