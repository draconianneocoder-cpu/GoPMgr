// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

// insertBareProjectRow inserts a project row directly via raw SQL,
// bypassing UpsertProject -- which itself appends a "project.create"
// audit event as a side effect (confirmed by direct experimentation:
// every project created through the normal path already has one audit
// event before a test appends any of its own). Tests that need a
// project_id with a known, exact audit-event count, or a second
// project_id purely to exercise a WHERE clause, use this instead.
func insertBareProjectRow(t *testing.T, d *Database, id string) {
	t.Helper()
	if _, err := d.Conn.Exec(
		`INSERT INTO project (id, name, created_at, updated_at) VALUES (?, 'Bare', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		id,
	); err != nil {
		t.Fatalf("insert bare project row %s: %v", id, err)
	}
}

func TestListAuditEvents_ReturnsEventsInSequenceOrder(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := "prj-seq-order"
	insertBareProjectRow(t, d, projectID)

	for i, evtType := range []string{"a.first", "b.second", "c.third"} {
		if _, err := d.AppendAuditEvent(AuditEventInput{
			ProjectID:  projectID,
			EventType:  evtType,
			EntityType: "test",
			EntityID:   "e",
		}); err != nil {
			t.Fatalf("AppendAuditEvent[%d]: %v", i, err)
		}
	}

	got, err := d.ListAuditEvents(projectID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListAuditEvents returned %d events, want 3", len(got))
	}
	wantSeq := []int64{1, 2, 3}
	wantType := []string{"a.first", "b.second", "c.third"}
	for i := range got {
		if got[i].SequenceNumber != wantSeq[i] || got[i].EventType != wantType[i] {
			t.Errorf("event[%d] = {seq:%d type:%q}, want {seq:%d type:%q}",
				i, got[i].SequenceNumber, got[i].EventType, wantSeq[i], wantType[i])
		}
	}
}

// TestListAuditEvents_ReturnsEmptyForProjectWithNoEvents pins that the
// zero-events case returns a nil slice (Go's zero value for `var events
// []AuditEvent` with no appends), not an allocated empty slice -- worth
// pinning since a compliance-evidence exporter reading this result might
// distinguish "nil" from "[]" on the wire. Uses insertBareProjectRow
// rather than UpsertProject specifically because UpsertProject's own
// "project.create" audit event would make a true zero-events project
// unreachable through the normal path.
func TestListAuditEvents_ReturnsEmptyForProjectWithNoEvents(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := "prj-no-events"
	insertBareProjectRow(t, d, projectID)

	got, err := d.ListAuditEvents(projectID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListAuditEvents = %v, want empty", got)
	}
	if got != nil {
		t.Errorf("ListAuditEvents returned a non-nil empty slice; if this changes intentionally, update this test's expectation deliberately")
	}
}

func TestListAuditEvents_WhereClauseFiltersOnProjectID(t *testing.T) {
	d := newBackupTestDB(t)
	mineID := "prj-mine"
	otherID := "prj-other"
	insertBareProjectRow(t, d, mineID)
	insertBareProjectRow(t, d, otherID)

	if _, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  mineID,
		EventType:  "mine",
		EntityType: "test",
		EntityID:   "e",
	}); err != nil {
		t.Fatalf("AppendAuditEvent(mine): %v", err)
	}
	if _, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  otherID,
		EventType:  "not-mine",
		EntityType: "test",
		EntityID:   "e",
	}); err != nil {
		t.Fatalf("AppendAuditEvent(other): %v", err)
	}

	got, err := d.ListAuditEvents(mineID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(got) != 1 || got[0].EventType != "mine" {
		t.Errorf("ListAuditEvents(mineID) = %v, want exactly one \"mine\" event -- WHERE project_id clause did not filter out the other project's event", got)
	}
}

// TestListAuditEvents_DoesNotRepairDamagedRows is this file's headline
// test: ListAuditEvents' doc comment explicitly states it "must not
// attempt to repair or normalize damaged rows" -- the actual compliance
// contract (this is used for "compliance evidence exports"). Corrupting
// event_hash itself would be the wrong tamper: ListAuditEvents never
// recomputes hashes, so returning a corrupted hash verbatim would be
// trivially true even if the function normalized OTHER fields. Instead
// this tampers before_canonical_json into a non-canonical shape
// (different key order and spacing than canonicalJSON would ever
// produce) and signature_status into an empty string (which
// appendAuditEventTx would have defaulted to "unsigned", so an empty
// result on read can only mean the row was returned raw, not
// re-processed). Paired with VerifyAuditChain on the same corrupted
// row to confirm the *chain* verification (a different code path) does
// flag it -- the contrast is the actual point: ListAuditEvents is a
// raw forensic read, VerifyAuditChain is the integrity check, and they
// must not be conflated.
func TestListAuditEvents_DoesNotRepairDamagedRows(t *testing.T) {
	d := newBackupTestDB(t)
	projectID := "prj-tamper"
	insertBareProjectRow(t, d, projectID)

	event, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  projectID,
		EventType:  "test.tamper",
		EntityType: "test",
		EntityID:   "e",
		BeforeJSON: `{"a":1,"b":2}`,
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	const tamperedBefore = `{ "b": 2 ,"a":1}` // non-canonical: reordered keys, extra spacing
	if _, err := d.Conn.Exec(
		`UPDATE audit_events SET before_canonical_json = ?, signature_status = '' WHERE id = ?`,
		tamperedBefore, event.ID,
	); err != nil {
		t.Fatalf("tamper audit row: %v", err)
	}

	got, err := d.ListAuditEvents(projectID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListAuditEvents returned %d events, want 1", len(got))
	}
	if got[0].BeforeCanonicalJSON != tamperedBefore {
		t.Errorf("BeforeCanonicalJSON = %q, want the raw tampered value %q unchanged -- ListAuditEvents must not re-canonicalize on read",
			got[0].BeforeCanonicalJSON, tamperedBefore)
	}
	if got[0].SignatureStatus != "" {
		t.Errorf("SignatureStatus = %q, want empty string unchanged -- ListAuditEvents must not re-apply appendAuditEventTx's default", got[0].SignatureStatus)
	}

	verification, err := d.VerifyAuditChain(projectID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if verification.Valid {
		t.Error("VerifyAuditChain reported the tampered chain as valid -- the tamper in this test must be detectable by the integrity check, or this test isn't proving what it claims")
	}
}
