// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"regexp"
	"testing"
)

var fixedWidthTimestampRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{9}Z$`)

// TestMigrate_RunsTimestampRetrofit proves the retrofit is actually
// wired into the normal database-open path (Migrate -> migrateLegacyColumns
// -> retrofitTimestampFormat), not just callable in isolation: seeds a
// legacy value, then calls Migrate() -- the same call InitDB makes on
// every open -- rather than retrofitTimestampFormat directly.
func TestMigrate_RunsTimestampRetrofit(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE charts SET updated_at = ? WHERE id = ?`, "2026-01-01T10:00:00Z", chart.ID); err != nil {
		t.Fatalf("seed legacy chart timestamp: %v", err)
	}

	if err := d.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var got string
	if err := d.Conn.QueryRow(`SELECT updated_at FROM charts WHERE id = ?`, chart.ID).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !fixedWidthTimestampRE.MatchString(got) {
		t.Errorf("chart updated_at after Migrate() = %q, want fixed 9-digit-fraction format -- retrofit is not wired into the open path", got)
	}
}

// TestNowTimestamp_AlwaysNineFractionDigits pins the format-contract
// timestampRetrofitTargets and every write call site now depend on:
// nowTimestamp() must never trim or omit the fractional-seconds
// component, unlike time.Now().UTC().Format(time.RFC3339Nano). Run
// several times since a landing on exactly zero nanoseconds -- the
// omitted-fraction case that originally exposed this hazard -- is rare
// under normal wall-clock jitter and this loop increases the odds of
// hitting it if the guard were ever removed.
func TestNowTimestamp_AlwaysNineFractionDigits(t *testing.T) {
	for range 20 {
		got := nowTimestamp()
		if !fixedWidthTimestampRE.MatchString(got) {
			t.Fatalf("nowTimestamp() = %q, want a fixed 9-digit-fraction RFC3339 timestamp", got)
		}
	}
}

// TestSaveChart_UpdatedAtIsFixedWidth and TestSaveDocument_UpdatedAtIsFixedWidth
// are format-contract tests proving the fix reaches real write call
// sites, not just the helper in isolation: a value written through the
// normal Save path must already be fixed-width on disk, immediately,
// without waiting for a future Migrate() retrofit pass.
func TestSaveChart_UpdatedAtIsFixedWidth(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if !fixedWidthTimestampRE.MatchString(chart.CreatedAt) {
		t.Errorf("Chart.CreatedAt = %q, want fixed 9-digit-fraction format", chart.CreatedAt)
	}
	if !fixedWidthTimestampRE.MatchString(chart.UpdatedAt) {
		t.Errorf("Chart.UpdatedAt = %q, want fixed 9-digit-fraction format", chart.UpdatedAt)
	}
}

func TestSaveDocument_UpdatedAtIsFixedWidth(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	doc, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "T"})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}
	if !fixedWidthTimestampRE.MatchString(doc.CreatedAt) {
		t.Errorf("Document.CreatedAt = %q, want fixed 9-digit-fraction format", doc.CreatedAt)
	}
	if !fixedWidthTimestampRE.MatchString(doc.UpdatedAt) {
		t.Errorf("Document.UpdatedAt = %q, want fixed 9-digit-fraction format", doc.UpdatedAt)
	}
}

// TestRetrofitTimestampFormat_NormalizesLegacyShapes seeds every legacy
// shape this retrofit needs to handle directly via SQL (bypassing
// SaveChart/SaveDocument entirely, the only way to reliably produce
// them now that the write path always emits fixed width) and confirms
// retrofitTimestampFormat rewrites all of them to timestampLayout:
// the old trimmed-fraction application format (both the omitted-fraction
// whole-second case and a positive-fraction case with fewer than nine
// digits), and the SQL column DEFAULT's fixed-but-different 3-digit
// format reachable via a row inserted through raw SQL.
func TestRetrofitTimestampFormat_NormalizesLegacyShapes(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	doc, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "T"})
	if err != nil {
		t.Fatalf("SaveDocument: %v", err)
	}

	legacyShapes := map[string]string{
		chart.ID: "2026-01-01T10:00:00Z",     // omitted fraction (whole second)
		doc.ID:   "2026-01-01T10:00:00.123Z", // SQL DEFAULT's fixed 3-digit shape
	}
	if _, err := d.Conn.Exec(`UPDATE charts SET updated_at = ? WHERE id = ?`, legacyShapes[chart.ID], chart.ID); err != nil {
		t.Fatalf("seed legacy chart timestamp: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE documents SET updated_at = ? WHERE id = ?`, legacyShapes[doc.ID], doc.ID); err != nil {
		t.Fatalf("seed legacy document timestamp: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat: %v", err)
	}

	var chartUpdated, docUpdated string
	if err := d.Conn.QueryRow(`SELECT updated_at FROM charts WHERE id = ?`, chart.ID).Scan(&chartUpdated); err != nil {
		t.Fatalf("read back chart updated_at: %v", err)
	}
	if err := d.Conn.QueryRow(`SELECT updated_at FROM documents WHERE id = ?`, doc.ID).Scan(&docUpdated); err != nil {
		t.Fatalf("read back document updated_at: %v", err)
	}

	if !fixedWidthTimestampRE.MatchString(chartUpdated) {
		t.Errorf("chart updated_at after retrofit = %q, want fixed 9-digit-fraction format", chartUpdated)
	}
	if !fixedWidthTimestampRE.MatchString(docUpdated) {
		t.Errorf("document updated_at after retrofit = %q, want fixed 9-digit-fraction format", docUpdated)
	}
	// The retrofit must preserve the underlying instant, not just
	// reformat arbitrarily -- confirm the whole-second value round-trips
	// to the same moment.
	if chartUpdated != "2026-01-01T10:00:00.000000000Z" {
		t.Errorf("chart updated_at after retrofit = %q, want 2026-01-01T10:00:00.000000000Z (same instant, fixed width)", chartUpdated)
	}
	if docUpdated != "2026-01-01T10:00:00.123000000Z" {
		t.Errorf("document updated_at after retrofit = %q, want 2026-01-01T10:00:00.123000000Z (same instant, fixed width)", docUpdated)
	}
}

// TestRetrofitTimestampFormat_IsIdempotent confirms a second retrofit
// pass over already-normalized data is a true no-op: this runs on every
// database open (from migrateLegacyColumns), so it must not keep
// rewriting rows -- or worse, drift a value -- on every subsequent open.
func TestRetrofitTimestampFormat_IsIdempotent(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := d.Conn.Exec(`UPDATE charts SET updated_at = ? WHERE id = ?`, "2026-01-01T10:00:00Z", chart.ID); err != nil {
		t.Fatalf("seed legacy chart timestamp: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat (first pass): %v", err)
	}
	var firstPass string
	if err := d.Conn.QueryRow(`SELECT updated_at FROM charts WHERE id = ?`, chart.ID).Scan(&firstPass); err != nil {
		t.Fatalf("read back after first pass: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat (second pass): %v", err)
	}
	var secondPass string
	if err := d.Conn.QueryRow(`SELECT updated_at FROM charts WHERE id = ?`, chart.ID).Scan(&secondPass); err != nil {
		t.Fatalf("read back after second pass: %v", err)
	}

	if firstPass != secondPass {
		t.Errorf("value drifted across retrofit passes: first = %q, second = %q", firstPass, secondPass)
	}
}

// TestRetrofitTimestampFormat_LeavesMalformedValuesUntouched covers the
// safety property this migration must uphold: it runs inside Migrate(),
// so a single corrupted or garbage timestamp value must not fail the
// migration (and so prevent the database from opening at all) or get
// silently mangled -- it is left exactly as found.
func TestRetrofitTimestampFormat_LeavesMalformedValuesUntouched(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	const garbage = "not-a-timestamp-at-all"
	if _, err := d.Conn.Exec(`UPDATE charts SET updated_at = ? WHERE id = ?`, garbage, chart.ID); err != nil {
		t.Fatalf("seed garbage timestamp: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat should not fail on a malformed value: %v", err)
	}

	var got string
	if err := d.Conn.QueryRow(`SELECT updated_at FROM charts WHERE id = ?`, chart.ID).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != garbage {
		t.Errorf("updated_at = %q, want untouched garbage value %q", got, garbage)
	}

	// The safety property that actually matters: a real database with
	// this corruption must still open successfully via the normal path
	// (Migrate() runs retrofitTimestampFormat as its last step).
	if err := d.Migrate(); err != nil {
		t.Fatalf("Migrate() failed on a database with a malformed timestamp value: %v", err)
	}
}

// TestRetrofitTimestampFormat_DoesNotTouchAuditEventTimestamps is the
// highest-consequence case: audit_events.timestamp_utc is one of the
// fields eventHash hashes (see eventHash in audit.go), so rewriting an
// existing row's value would silently break VerifyAuditChain's
// tamper-evident hash chain for every database that already has audit
// history. timestampRetrofitTargets deliberately excludes this column;
// this test proves that exclusion holds by seeding an old-format value
// directly (bypassing AppendAuditEvent, which now writes fixed-width via
// nowTimestamp() and would mask the exclusion) and confirming
// retrofitTimestampFormat leaves it byte-for-byte unchanged.
func TestRetrofitTimestampFormat_DoesNotTouchAuditEventTimestamps(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	event, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  project.ID,
		EventType:  "test.event",
		EntityType: "test",
		EntityID:   "e1",
	})
	if err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	const legacyShape = "2026-01-01T10:00:00Z"
	if _, err := d.Conn.Exec(`UPDATE audit_events SET timestamp_utc = ? WHERE id = ?`, legacyShape, event.ID); err != nil {
		t.Fatalf("seed legacy audit event timestamp: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat: %v", err)
	}

	var got string
	if err := d.Conn.QueryRow(`SELECT timestamp_utc FROM audit_events WHERE id = ?`, event.ID).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != legacyShape {
		t.Errorf("audit_events.timestamp_utc = %q, want untouched %q -- retrofit must never rewrite this column", got, legacyShape)
	}
}

// TestVerifyAuditChain_SurvivesRetrofit is the end-to-end companion to
// the test above: a real audit event created through the normal path
// (fixed-width timestamp_utc, valid hash) must still verify after
// retrofitTimestampFormat runs, proving the retrofit doesn't disturb an
// already-valid audit chain even though every other timestamp column in
// the same database is being rewritten around it.
func TestVerifyAuditChain_SurvivesRetrofit(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if _, err := d.AppendAuditEvent(AuditEventInput{
		ProjectID:  project.ID,
		EventType:  "test.event",
		EntityType: "test",
		EntityID:   "e1",
	}); err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	if err := d.retrofitTimestampFormat(); err != nil {
		t.Fatalf("retrofitTimestampFormat: %v", err)
	}

	report, err := d.VerifyAuditChain(project.ID)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !report.Valid {
		t.Errorf("VerifyAuditChain.Valid = false after retrofit, want true: %+v", report)
	}
}
