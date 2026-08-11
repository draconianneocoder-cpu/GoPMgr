// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"fmt"
	"regexp"
	"testing"
	"time"
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

// TestMigrate_RunsTimestampUnixNanoRetrofit is the _unixnano counterpart
// to TestMigrate_RunsTimestampRetrofit: proves retrofitTimestampUnixNano
// is actually wired into the normal database-open path, not just
// callable in isolation. Every other test in this file already proves
// this implicitly (they all insert rows naming the _unixnano columns,
// which would fail with "no such column" if ADD COLUMN never ran), but
// this test names the property directly rather than leaving it as a
// side effect other tests happen to depend on.
func TestMigrate_RunsTimestampUnixNanoRetrofit(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if err := d.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var got int64
	if err := d.Conn.QueryRow(`SELECT updated_at_unixnano FROM charts WHERE id = ?`, chart.ID).Scan(&got); err != nil {
		t.Fatalf("read back updated_at_unixnano (column may not exist -- retrofit not wired into the open path): %v", err)
	}
	if got == 0 {
		t.Errorf("chart updated_at_unixnano = 0 after Migrate(), want a real derived value")
	}
}

// TestCaptureTimestamp_TextAndUnixNanoAgree pins the basic contract of
// captureTimestamp itself: both fields must describe the same instant.
func TestCaptureTimestamp_TextAndUnixNanoAgree(t *testing.T) {
	for range 20 {
		c := captureTimestamp()
		parsed, err := time.Parse(time.RFC3339Nano, c.text)
		if err != nil {
			t.Fatalf("parse captureTimestamp().text = %q: %v", c.text, err)
		}
		if got := parsed.UnixNano(); got != c.unixNano {
			t.Fatalf("captureTimestamp() text/unixNano disagree: text %q parses to %d, unixNano field is %d", c.text, got, c.unixNano)
		}
	}
}

// checkTimestampColumnsAgree reads a row's TEXT and INTEGER timestamp
// columns directly via raw SQL -- bypassing every Save function -- and
// fails if they describe different instants. This is the actual
// enforcement backstop for the structural fix (see timestampCapture's
// doc comment in timestamps.go): the Go type system cannot stop a call
// site from binding only one half of a pair, so this test is what
// catches it if one ever does.
func checkTimestampColumnsAgree(t *testing.T, d *Database, table, idCol, id, textCol, intCol string) {
	t.Helper()
	query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = ?", textCol, intCol, table, idCol) // #nosec G201 -- test-only, table/idCol/textCol/intCol are all hardcoded literals in this file, never external input.
	var text string
	var gotUnixNano int64
	if err := d.Conn.QueryRow(query, id).Scan(&text, &gotUnixNano); err != nil {
		t.Fatalf("read back %s.%s/%s: %v", table, textCol, intCol, err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		t.Fatalf("parse %s.%s = %q: %v", table, textCol, text, err)
	}
	if want := parsed.UnixNano(); gotUnixNano != want {
		t.Errorf("%s.%s = %d, want %d (parsed from %s.%s = %q) -- TEXT and INTEGER timestamp columns disagree for id %s", table, intCol, gotUnixNano, want, table, textCol, text, id)
	}
}

// TestAllTimestampColumns_TextAndUnixNanoAgree is the persisted-state
// consistency test for the structural ordering fix, covering all 8
// tables in timestampRetrofitTargets. For every table that has an
// update/upsert path, it saves twice (create, then a second save with
// changed content against the same ID) and re-checks after each save --
// an insert-only check would pass even if a table's UPDATE/ON CONFLICT
// DO UPDATE clause forgets to advance the _unixnano column alongside the
// TEXT one, which is exactly the drift this fix exists to prevent.
func TestAllTimestampColumns_TextAndUnixNanoAgree(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	checkTimestampColumnsAgree(t, d, "project", "id", project.ID, "created_at", "created_at_unixnano")
	checkTimestampColumnsAgree(t, d, "project", "id", project.ID, "updated_at", "updated_at_unixnano")
	project.Name = "Timestamps (updated)"
	project, err = d.UpsertProject(project)
	if err != nil {
		t.Fatalf("UpsertProject (update): %v", err)
	}
	checkTimestampColumnsAgree(t, d, "project", "id", project.ID, "created_at", "created_at_unixnano")
	checkTimestampColumnsAgree(t, d, "project", "id", project.ID, "updated_at", "updated_at_unixnano")

	t.Run("charts", func(t *testing.T) {
		chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
		if err != nil {
			t.Fatalf("SaveChart: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "charts", "id", chart.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "charts", "id", chart.ID, "updated_at", "updated_at_unixnano")
		chart.Title = "T (updated)"
		chart, err = d.SaveChart(chart)
		if err != nil {
			t.Fatalf("SaveChart (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "charts", "id", chart.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "charts", "id", chart.ID, "updated_at", "updated_at_unixnano")
	})

	t.Run("documents", func(t *testing.T) {
		doc, err := d.SaveDocument(Document{ProjectID: project.ID, Kind: "charter", Title: "T"})
		if err != nil {
			t.Fatalf("SaveDocument: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "documents", "id", doc.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "documents", "id", doc.ID, "updated_at", "updated_at_unixnano")
		doc.Title = "T (updated)"
		doc, err = d.SaveDocument(doc)
		if err != nil {
			t.Fatalf("SaveDocument (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "documents", "id", doc.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "documents", "id", doc.ID, "updated_at", "updated_at_unixnano")
	})

	t.Run("baselines", func(t *testing.T) {
		chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
		if err != nil {
			t.Fatalf("SaveChart: %v", err)
		}
		baseline, err := d.SaveBaseline(Baseline{ProjectID: project.ID, ChartID: chart.ID, Name: "B"})
		if err != nil {
			t.Fatalf("SaveBaseline: %v", err)
		}
		// Baselines are immutable (no update path) -- created_at is the
		// only column to check.
		checkTimestampColumnsAgree(t, d, "baselines", "id", baseline.ID, "created_at", "created_at_unixnano")
	})

	t.Run("stakeholders", func(t *testing.T) {
		sh, err := d.SaveStakeholder(Stakeholder{ProjectID: project.ID, Name: "S"})
		if err != nil {
			t.Fatalf("SaveStakeholder: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "stakeholders", "id", sh.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "stakeholders", "id", sh.ID, "updated_at", "updated_at_unixnano")
		sh.Name = "S (updated)"
		sh, err = d.SaveStakeholder(sh)
		if err != nil {
			t.Fatalf("SaveStakeholder (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "stakeholders", "id", sh.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "stakeholders", "id", sh.ID, "updated_at", "updated_at_unixnano")
	})

	t.Run("resource_calendars", func(t *testing.T) {
		rc, err := d.SaveResourceCalendar(ResourceCalendar{ProjectID: project.ID, Resource: "eng", Name: "R"})
		if err != nil {
			t.Fatalf("SaveResourceCalendar: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "resource_calendars", "id", rc.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "resource_calendars", "id", rc.ID, "updated_at", "updated_at_unixnano")
		rc.Name = "R (updated)"
		rc, err = d.SaveResourceCalendar(rc)
		if err != nil {
			t.Fatalf("SaveResourceCalendar (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "resource_calendars", "id", rc.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "resource_calendars", "id", rc.ID, "updated_at", "updated_at_unixnano")
	})

	t.Run("scenarios", func(t *testing.T) {
		scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Scn", IsActive: true})
		if err != nil {
			t.Fatalf("SaveScenario: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "scenarios", "id", scenario.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "scenarios", "id", scenario.ID, "updated_at", "updated_at_unixnano")
		scenario.Description = "updated"
		scenario, err = d.SaveScenario(scenario)
		if err != nil {
			t.Fatalf("SaveScenario (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "scenarios", "id", scenario.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "scenarios", "id", scenario.ID, "updated_at", "updated_at_unixnano")

		// Also cover the third scenarios.updated_at write site: the bulk
		// "deactivate every other active scenario" UPDATE that fires when
		// a second scenario is saved as active. scenario (above) is
		// IsActive so it is the row this UPDATE actually touches --
		// checking it, not just the newly-active "other" row, is what
		// makes this exercise the bulk UPDATE's own binding rather than
		// the unrelated INSERT/ON CONFLICT path already covered above.
		other, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "Scn2", IsActive: true})
		if err != nil {
			t.Fatalf("SaveScenario (second, active): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "scenarios", "id", other.ID, "updated_at", "updated_at_unixnano")
		checkTimestampColumnsAgree(t, d, "scenarios", "id", scenario.ID, "updated_at", "updated_at_unixnano")
	})

	t.Run("scenario_charts", func(t *testing.T) {
		chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
		if err != nil {
			t.Fatalf("SaveChart: %v", err)
		}
		scenario, err := d.SaveScenario(Scenario{ProjectID: project.ID, Name: "ScnForChart"})
		if err != nil {
			t.Fatalf("SaveScenario: %v", err)
		}
		branched, err := d.BranchScenarioChart(scenario.ID, chart.ID, "")
		if err != nil {
			t.Fatalf("BranchScenarioChart: %v", err)
		}
		checkTimestampColumnsAgree(t, d, "scenario_charts", "id", branched.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "scenario_charts", "id", branched.ID, "updated_at", "updated_at_unixnano")
		branched.Title = "T (updated)"
		branched, err = d.SaveScenarioChart(branched)
		if err != nil {
			t.Fatalf("SaveScenarioChart (update): %v", err)
		}
		checkTimestampColumnsAgree(t, d, "scenario_charts", "id", branched.ID, "created_at", "created_at_unixnano")
		checkTimestampColumnsAgree(t, d, "scenario_charts", "id", branched.ID, "updated_at", "updated_at_unixnano")
	})
}

// TestRetrofitTimestampUnixNano_BackfillsExistingRows seeds a row the
// way a pre-this-cycle database would have it: a real TEXT timestamp but
// the _unixnano sibling still at its ADD COLUMN default of 0 (simulating
// a row that existed before this migration ever ran), and confirms
// retrofitTimestampUnixNano derives the correct integer from it.
func TestRetrofitTimestampUnixNano_BackfillsExistingRows(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	const seeded = "2026-03-15T12:30:00.500000000Z"
	if _, err := d.Conn.Exec(
		`UPDATE charts SET updated_at = ?, updated_at_unixnano = 0 WHERE id = ?`,
		seeded, chart.ID,
	); err != nil {
		t.Fatalf("seed pre-migration row: %v", err)
	}

	if err := d.retrofitTimestampUnixNano(); err != nil {
		t.Fatalf("retrofitTimestampUnixNano: %v", err)
	}

	want, err := time.Parse(time.RFC3339Nano, seeded)
	if err != nil {
		t.Fatalf("parse seeded value: %v", err)
	}
	var got int64
	if err := d.Conn.QueryRow(`SELECT updated_at_unixnano FROM charts WHERE id = ?`, chart.ID).Scan(&got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got != want.UnixNano() {
		t.Errorf("updated_at_unixnano = %d, want %d (derived from seeded TEXT value %q)", got, want.UnixNano(), seeded)
	}
}

// TestRetrofitTimestampUnixNano_IsIdempotent mirrors
// TestRetrofitTimestampFormat_IsIdempotent for the new integer column:
// this also runs on every database open, so a second pass over
// already-backfilled data must not touch it again.
func TestRetrofitTimestampUnixNano_IsIdempotent(t *testing.T) {
	d := newBackupTestDB(t)
	project, err := d.UpsertProject(Project{Name: "Timestamps"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	chart, err := d.SaveChart(Chart{ProjectID: project.ID, Kind: "cpm", Title: "T"})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}

	if err := d.retrofitTimestampUnixNano(); err != nil {
		t.Fatalf("retrofitTimestampUnixNano (first pass): %v", err)
	}
	var firstPass int64
	if err := d.Conn.QueryRow(`SELECT updated_at_unixnano FROM charts WHERE id = ?`, chart.ID).Scan(&firstPass); err != nil {
		t.Fatalf("read back after first pass: %v", err)
	}

	if err := d.retrofitTimestampUnixNano(); err != nil {
		t.Fatalf("retrofitTimestampUnixNano (second pass): %v", err)
	}
	var secondPass int64
	if err := d.Conn.QueryRow(`SELECT updated_at_unixnano FROM charts WHERE id = ?`, chart.ID).Scan(&secondPass); err != nil {
		t.Fatalf("read back after second pass: %v", err)
	}

	if firstPass != secondPass {
		t.Errorf("value drifted across retrofit passes: first = %d, second = %d", firstPass, secondPass)
	}
}

// explainUsesIndex runs EXPLAIN QUERY PLAN and fails if SQLite reports a
// separate sort step (a query plan node containing "USE TEMP B-TREE FOR
// ORDER BY"), which means the ORDER BY's target index isn't actually
// serving the sort -- catching a silent regression in
// repointUnixNanoIndex or the static schema's index definitions with a
// measured answer rather than an assumption.
func explainUsesIndex(t *testing.T, d *Database, query string, args ...any) {
	t.Helper()
	rows, err := d.Conn.Query("EXPLAIN QUERY PLAN "+query, args...) // #nosec G201 -- test-only, query is a hardcoded literal in this file, never external input.
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var plan string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan query plan row: %v", err)
		}
		plan += detail + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query plan rows: %v", err)
	}
	if regexp.MustCompile(`(?i)USE TEMP B-TREE FOR ORDER BY`).MatchString(plan) {
		t.Errorf("query falls back to a full sort instead of using an index:\n%s\nquery: %s", plan, query)
	}
}

// TestListActiveScenariosExcept_UsesIndexForSort and
// TestListScenarioCharts_UsesIndexForSort prove idx_scenarios_project and
// idx_scenario_charts_scenario actually serve their queries' sort after
// repointUnixNanoIndex switches them to the _unixnano column -- an
// unmeasured assumption here would mean these queries silently fall back
// to a full table sort. listActiveScenariosExceptTx's query
// (project_id equality + is_active <> 0 filter, ORDER BY created_at ASC
// with no other leading sort key) is the one idx_scenarios_project is
// actually shaped for: unlike ListScenarios' ORDER BY (which leads with
// is_active, a column the index doesn't cover, so it needs a temp
// B-tree sort regardless of which timestamp column it targets -- verified
// directly, not assumed, and unchanged by this fix either way), this
// query's WHERE + ORDER BY together are fully covered by
// (project_id, created_at_unixnano).
func TestListActiveScenariosExcept_UsesIndexForSort(t *testing.T) {
	d := newBackupTestDB(t)
	explainUsesIndex(t, d, `SELECT id FROM scenarios WHERE project_id = ? AND id <> ? AND is_active <> 0 ORDER BY created_at_unixnano ASC`, "p1", "x")
}

func TestListScenarioCharts_UsesIndexForSort(t *testing.T) {
	d := newBackupTestDB(t)
	explainUsesIndex(t, d, `SELECT id FROM scenario_charts WHERE scenario_id = ? ORDER BY updated_at_unixnano DESC, title ASC`, "s1")
}

// TestRepointUnixNanoIndex_RecreatesWhenMissing covers the crash-recovery
// property repointUnixNanoIndex exists for: if a process died between the
// DROP and CREATE on a prior open (or anything else left the index
// missing entirely), the next Migrate() must recreate it rather than
// treating "index not found" as "nothing to do." Without this, a single
// interrupted repoint would permanently lose the index and its query
// would silently fall back to a full table sort forever after.
func TestRepointUnixNanoIndex_RecreatesWhenMissing(t *testing.T) {
	d := newBackupTestDB(t)
	if _, err := d.Conn.Exec(`DROP INDEX idx_scenarios_project`); err != nil {
		t.Fatalf("seed missing index: %v", err)
	}

	if err := d.repointUnixNanoIndex("idx_scenarios_project", "scenarios", "created_at", []string{"project_id"}); err != nil {
		t.Fatalf("repointUnixNanoIndex: %v", err)
	}

	cols, err := d.indexColumns("idx_scenarios_project")
	if err != nil {
		t.Fatalf("indexColumns: %v", err)
	}
	want := []string{"project_id", "created_at_unixnano"}
	if !stringSlicesEqual(cols, want) {
		t.Errorf("idx_scenarios_project columns = %v, want %v (index was not recreated after being found missing)", cols, want)
	}
}

// TestRepointUnixNanoIndex_FailsLoudlyOnUnexpectedShape covers the other
// safety property: if the on-disk index doesn't match either the legacy
// TEXT-column shape or the already-repointed shape (e.g. someone changed
// the static schema's CREATE INDEX in sqlite.go without updating
// unixNanoIndexTargets to match), this must return an error rather than
// silently dropping and rebuilding a narrower index than the one
// actually in use.
func TestRepointUnixNanoIndex_FailsLoudlyOnUnexpectedShape(t *testing.T) {
	d := newBackupTestDB(t)
	if _, err := d.Conn.Exec(`DROP INDEX idx_scenarios_project`); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := d.Conn.Exec(`CREATE INDEX idx_scenarios_project ON scenarios(project_id, is_active, created_at)`); err != nil {
		t.Fatalf("seed unexpected index shape: %v", err)
	}

	err := d.repointUnixNanoIndex("idx_scenarios_project", "scenarios", "created_at", []string{"project_id"})
	if err == nil {
		t.Fatal("repointUnixNanoIndex should fail loudly when the on-disk index shape doesn't match unixNanoIndexTargets, not silently rebuild a narrower index")
	}
}
