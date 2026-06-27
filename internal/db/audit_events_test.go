// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

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
