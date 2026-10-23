// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestProjectMetaSaveIncludesClassificationAndRollsBackWithAudit(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "General Save")
	d := app.requireDB()
	auditCount := func() int {
		t.Helper()
		var count int
		if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&count); err != nil {
			t.Fatalf("count audit events: %v", err)
		}
		return count
	}
	before, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	count := auditCount()
	input := before
	input.Name = "Reclassified project"
	input.Description = "All General fields in one write"
	input.Owner = "Project controls"
	input.Industry = "construction"
	input.SubCategory = "bridge"
	input.Methodology = "legacy-method"
	input.CountryCode = "CA"
	input.TimeZone = "America/Toronto"
	input.Status = "active"
	input.Phase = "execution"
	input.StartDate = "2026-10-23"
	input.EndDate = "2027-10-23"
	input.Budget = "92233720368547758.07"
	input.CurrencyCode = "EUR"
	saved, err := app.UpdateProjectMeta(input)
	if err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}
	if saved.UpdatedAt == "" {
		t.Fatal("saved project has no server timestamp")
	}
	input.UpdatedAt = saved.UpdatedAt
	if !reflect.DeepEqual(saved, input) {
		t.Fatalf("saved = %+v, want %+v", saved, input)
	}
	stored, err := app.GetProjectMeta()
	if err != nil || !reflect.DeepEqual(stored, saved) {
		t.Fatalf("persisted project = %+v, err %v; want %+v", stored, err, saved)
	}
	if got := auditCount(); got != count+1 {
		t.Fatalf("audit count = %d, want exactly %d after one save", got, count+1)
	}
	var industry, category, methodology, country, zone string
	var budget int64
	if err := d.Conn.QueryRow(`SELECT industry, sub_category, methodology, country_code, time_zone, budget_minor_units
		FROM project WHERE id = ?`, saved.ID).Scan(&industry, &category, &methodology, &country, &zone, &budget); err != nil {
		t.Fatalf("read stored classification: %v", err)
	}
	if industry != "construction" || category != "bridge" || methodology != "legacy-method" || country != "CA" || zone != "America/Toronto" || budget != 9223372036854775807 {
		t.Fatalf("stored fields = %q %q %q %q %q %d", industry, category, methodology, country, zone, budget)
	}
	if _, err := d.Conn.Exec(`CREATE TRIGGER refuse_general_save BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'project.update' BEGIN SELECT RAISE(ABORT, 'fixture audit failure'); END`); err != nil {
		t.Fatalf("install audit failure: %v", err)
	}
	input.Name = "Must not persist"
	input.Description = ""
	input.Industry = "software"
	input.SubCategory = "internal tools"
	input.Methodology = "scrum"
	input.CountryCode = "US"
	input.TimeZone = "America/Chicago"
	input.Budget = "0.00"
	failed, err := app.UpdateProjectMeta(input)
	if err == nil || !strings.Contains(err.Error(), "fixture audit failure") {
		t.Fatalf("save error = %v, want audit failure", err)
	}
	if failed != (ProjectMetaWire{}) {
		t.Fatalf("failed save returned success data: %+v", failed)
	}
	stored, err = app.GetProjectMeta()
	if err != nil || !reflect.DeepEqual(stored, saved) {
		t.Fatalf("failed save changed project: %+v, err %v; want %+v", stored, err, saved)
	}
	if got := auditCount(); got != count+1 {
		t.Fatalf("failed save changed audit count to %d, want %d", got, count+1)
	}
	if _, err := d.Conn.Exec(`DROP TRIGGER refuse_general_save`); err != nil {
		t.Fatalf("remove audit failure: %v", err)
	}
	retried, err := app.UpdateProjectMeta(input)
	if err != nil {
		t.Fatalf("retry after audit failure: %v", err)
	}
	input.UpdatedAt = retried.UpdatedAt
	stored, err = app.GetProjectMeta()
	if err != nil || !reflect.DeepEqual(retried, input) || !reflect.DeepEqual(stored, input) {
		t.Fatalf("retry = %+v, stored = %+v, err %v; want %+v", retried, stored, err, input)
	}
	report, err := d.VerifyAuditChain(saved.ID)
	if err != nil || !report.Valid || report.CheckedEvents != count+2 {
		t.Fatalf("audit after retry = %+v, err %v; want valid with %d events", report, err, count+2)
	}
}
