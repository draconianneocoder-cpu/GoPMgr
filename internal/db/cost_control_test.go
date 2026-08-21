// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newCostControlTestDB(t *testing.T) *Database {
	t.Helper()
	d, err := InitDB(filepath.Join(t.TempDir(), "cost-control.gopmgr"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestCostControlSeedsAndAuditsProjectScopedEntry(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Cost ledger"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) < 8 {
		t.Fatalf("seed types = %d, want at least 8", len(types))
	}
	entry, err := d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "actual", CostDate: "2026-08-20", Description: "Security review", AmountMinorUnits: 125_50})
	if err != nil {
		t.Fatal(err)
	}
	if entry.AmountMinorUnits != 125_50 {
		t.Fatalf("amount = %d", entry.AmountMinorUnits)
	}
	p.CurrencyCode = "EUR"
	if _, err := d.UpsertProject(p); err == nil {
		t.Fatal("currency changed after a cost entry")
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
	verification, err := d.VerifyAuditChain(p.ID)
	if err != nil || !verification.Valid {
		t.Fatalf("audit = %#v, %v", verification, err)
	}
}

func TestProjectReportingCurrencyPolicy(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Currency policy"})
	if err != nil {
		t.Fatal(err)
	}
	if p.CurrencyCode != "USD" {
		t.Fatalf("default currency = %q, want USD", p.CurrencyCode)
	}
	p.CurrencyCode = " eur "
	p, err = d.UpsertProject(p)
	if err != nil {
		t.Fatalf("change empty project currency: %v", err)
	}
	if p.CurrencyCode != "EUR" {
		t.Fatalf("normalised currency = %q, want EUR", p.CurrencyCode)
	}
	p.CurrencyCode = "ZZZ"
	if _, err := d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "unsupported project reporting currency") {
		t.Fatalf("unsupported currency error = %v", err)
	}

	p.CurrencyCode = "EUR"
	p.BudgetMinorUnits = 1
	p, err = d.UpsertProject(p)
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}
	p.CurrencyCode = "USD"
	if _, err := d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "Budget value") {
		t.Fatalf("currency change after budget error = %v", err)
	}
}

func TestProjectReportingCurrencyRetiresNewJPYButPreservesLegacyJPY(t *testing.T) {
	d := newCostControlTestDB(t)
	if _, err := d.UpsertProject(Project{Name: "New JPY", CurrencyCode: "JPY"}); err == nil || !strings.Contains(err.Error(), "legacy projects") {
		t.Fatalf("new JPY project error = %v", err)
	}
	p, err := d.UpsertProject(Project{Name: "Legacy JPY"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`UPDATE project SET currency_code='JPY' WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	p, err = d.GetProject()
	if err != nil {
		t.Fatal(err)
	}
	p.Description = "Still inspectable"
	p, err = d.UpsertProject(p)
	if err != nil || p.CurrencyCode != "JPY" || p.Description != "Still inspectable" {
		t.Fatalf("preserve legacy JPY = %#v, %v", p, err)
	}
	p.CurrencyCode = "USD"
	p, err = d.UpsertProject(p)
	if err != nil || p.CurrencyCode != "USD" {
		t.Fatalf("empty legacy JPY to USD = %#v, %v", p, err)
	}
	p.CurrencyCode = "JPY"
	if _, err = d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "legacy projects") {
		t.Fatalf("USD to retired JPY error = %v", err)
	}
}

func TestProjectReportingCurrencyLocksReservesAndBaselines(t *testing.T) {
	t.Run("non-zero reserve", func(t *testing.T) {
		d := newCostControlTestDB(t)
		p, err := d.UpsertProject(Project{Name: "Reserve currency policy"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 100, Description: "Known risk"}); err != nil {
			t.Fatal(err)
		}
		p.CurrencyCode = "EUR"
		if _, err = d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "non-zero reserve balance") {
			t.Fatalf("currency change after reserve error = %v", err)
		}
		if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 0, Description: "Known risk retired"}); err != nil {
			t.Fatal(err)
		}
		p, err = d.UpsertProject(p)
		if err != nil || p.CurrencyCode != "EUR" {
			t.Fatalf("currency change after zero reserve = %#v, %v", p, err)
		}
	})

	t.Run("approved baseline", func(t *testing.T) {
		d := newCostControlTestDB(t)
		p, err := d.UpsertProject(Project{Name: "Baseline currency policy"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 100, Description: "Known risk"}); err != nil {
			t.Fatal(err)
		}
		if _, err = d.ApproveCostBaseline(p.ID, "alice", "Approved contingency basis"); err != nil {
			t.Fatal(err)
		}
		if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 0, Description: "Known risk retired"}); err != nil {
			t.Fatal(err)
		}
		p.CurrencyCode = "EUR"
		if _, err = d.UpsertProject(p); err == nil || !strings.Contains(err.Error(), "approved Cost Control baseline") {
			t.Fatalf("currency change after baseline error = %v", err)
		}
	})
}

func TestCostEntryRejectsArchivedOrZeroAmount(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Validation"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`UPDATE cost_types SET active=0 WHERE id=?`, types[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-20", Description: "x", AmountMinorUnits: 1}); err == nil {
		t.Fatal("archived type accepted")
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[1].ID, Kind: "planned", CostDate: "2026-08-20", Description: "x"}); err == nil {
		t.Fatal("zero amount accepted")
	}
}

func TestRejectedCostWritesReleaseTransactionConnection(t *testing.T) {
	t.Run("archived cost type", func(t *testing.T) {
		d := newCostControlTestDB(t)
		p, err := d.UpsertProject(Project{Name: "Archived cost type"})
		if err != nil {
			t.Fatal(err)
		}
		types, err := d.ListCostTypes(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.Conn.Exec(`UPDATE cost_types SET active=0 WHERE id=?`, types[0].ID); err != nil {
			t.Fatal(err)
		}
		d.Conn.SetMaxOpenConns(1)
		if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Archived", AmountMinorUnits: 1}); err == nil || !strings.Contains(err.Error(), "archived") {
			t.Fatalf("SaveCostEntry error = %v", err)
		}
		assertCostControlConnectionAvailable(t, d)
	})

	t.Run("reserve lookup failure", func(t *testing.T) {
		d := newCostControlTestDB(t)
		p, err := d.UpsertProject(Project{Name: "Reserve lookup"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.Conn.Exec(`DROP TABLE cost_reserves`); err != nil {
			t.Fatal(err)
		}
		d.Conn.SetMaxOpenConns(1)
		if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 1, Description: "Risk"}); err == nil {
			t.Fatal("SaveCostReserve accepted missing table")
		}
		assertCostControlConnectionAvailable(t, d)
	})
}

func assertCostControlConnectionAvailable(t *testing.T, d *Database) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.Conn.PingContext(ctx); err != nil {
		t.Fatalf("transaction connection was not released: %v", err)
	}
}

func TestCostReserveUpsertPreservesIdentityAndAudits(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Reserves"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 10_000, Description: "Known risk"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.SaveCostReserve(CostReserve{ID: "conflicting-caller-id", ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 12_500, Description: "Reassessed"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("reserve ID changed: %q -> %q", first.ID, second.ID)
	}
	reserves, err := d.ListCostReserves(p.ID)
	if err != nil || len(reserves) != 1 || reserves[0].AmountMinorUnits != 12_500 {
		t.Fatalf("reserves = %#v, %v", reserves, err)
	}
	if reserves[0].ID != first.ID {
		t.Fatalf("stored reserve ID = %q, want %q", reserves[0].ID, first.ID)
	}
	events, err := d.ListAuditEvents(p.ID)
	if err != nil {
		t.Fatalf("audit events = %#v, %v", events, err)
	}
	var reserveEvents []AuditEvent
	for _, event := range events {
		if event.EventType == "cost_reserve.save" {
			reserveEvents = append(reserveEvents, event)
		}
	}
	if len(reserveEvents) != 2 {
		t.Fatalf("cost reserve audit events = %#v, want two", reserveEvents)
	}
	if reserveEvents[1].EntityID != first.ID {
		t.Fatalf("audit entity ID = %q, want %q", reserveEvents[1].EntityID, first.ID)
	}
	var audited CostReserve
	if err := json.Unmarshal([]byte(reserveEvents[1].AfterCanonicalJSON), &audited); err != nil {
		t.Fatalf("decode audit after payload: %v", err)
	}
	if audited.ID != first.ID {
		t.Fatalf("audit after payload ID = %q, want %q", audited.ID, first.ID)
	}
	verified, err := d.VerifyAuditChain(p.ID)
	if err != nil || !verified.Valid {
		t.Fatalf("audit = %#v, %v", verified, err)
	}
}

func TestCostReserveRollsBackWhenAuditWriteFails(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Reserve rollback"})
	if err != nil {
		t.Fatal(err)
	}
	original, err := d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 10_000, Description: "Known risk"})
	if err != nil {
		t.Fatal(err)
	}
	var beforeEvents int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=?`, p.ID).Scan(&beforeEvents); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn.Exec(`CREATE TRIGGER reject_cost_reserve_audit BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'cost_reserve.save'
		BEGIN SELECT RAISE(ABORT, 'forced cost reserve audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = d.Conn.Exec(`DROP TRIGGER IF EXISTS reject_cost_reserve_audit`) })

	for _, input := range []CostReserve{
		{ID: "conflicting-caller-id", ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 12_500, Description: "Reassessed"},
		{ID: "new-management-id", ProjectID: p.ID, Kind: "management", AmountMinorUnits: 500, Description: "Unknowns"},
	} {
		if _, err := d.SaveCostReserve(input); err == nil || !strings.Contains(err.Error(), "forced cost reserve audit failure") {
			t.Fatalf("SaveCostReserve(%s) error = %v", input.Kind, err)
		}
	}
	if _, err := d.Conn.Exec(`DROP TRIGGER reject_cost_reserve_audit`); err != nil {
		t.Fatal(err)
	}

	reserves, err := d.ListCostReserves(p.ID)
	if err != nil || len(reserves) != 1 || reserves[0] != original {
		t.Fatalf("reserves after rollback = %#v, want %#v, %v", reserves, original, err)
	}
	var afterEvents int
	if err := d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=?`, p.ID).Scan(&afterEvents); err != nil {
		t.Fatal(err)
	}
	if afterEvents != beforeEvents {
		t.Fatalf("audit event count after rollback = %d, want %d", afterEvents, beforeEvents)
	}
	if v, err := d.VerifyAuditChain(p.ID); err != nil || !v.Valid {
		t.Fatalf("audit after rollback = %#v, %v", v, err)
	}

	updated, err := d.SaveCostReserve(CostReserve{ID: "conflicting-caller-id", ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 12_500, Description: "Reassessed"})
	if err != nil || updated.ID != original.ID {
		t.Fatalf("retry update = %#v, %v; want ID %q", updated, err, original.ID)
	}
	management, err := d.SaveCostReserve(CostReserve{ID: "new-management-id", ProjectID: p.ID, Kind: "management", AmountMinorUnits: 500, Description: "Unknowns"})
	if err != nil || management.ID != "new-management-id" {
		t.Fatalf("retry create = %#v, %v", management, err)
	}
}

func TestApproveCostBaselineIsImmutableAndAudited(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Baseline"})
	if err != nil {
		t.Fatal(err)
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-20", Description: "Plan", AmountMinorUnits: 10000}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 1000, Description: "Risk"}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "management", AmountMinorUnits: 500, Description: "Unknowns"}); err != nil {
		t.Fatal(err)
	}
	first, err := d.ApproveCostBaseline(p.ID, "alice", "Initial approval")
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.PlannedMinorUnits != 10000 || first.ContingencyMinorUnits != 1000 || first.ManagementReserveMinorUnits != 500 {
		t.Fatalf("first = %#v", first)
	}
	assertBaselineAuditEvents(t, d, first)
	if _, err = d.SaveCostEntry(CostEntry{ProjectID: p.ID, CostTypeID: types[0].ID, Kind: "planned", CostDate: "2026-08-21", Description: "Later", AmountMinorUnits: 200}); err != nil {
		t.Fatal(err)
	}
	second, err := d.ApproveCostBaseline(p.ID, "alice", "Revised approval")
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 || second.PlannedMinorUnits != 10200 {
		t.Fatalf("second = %#v", second)
	}
	assertBaselineAuditEvents(t, d, second)
	all, err := d.ListCostBaselines(p.ID)
	if err != nil || len(all) != 2 || all[1].PlannedMinorUnits != 10000 {
		t.Fatalf("baselines = %#v, %v", all, err)
	}
	if v, err := d.VerifyAuditChain(p.ID); err != nil || !v.Valid {
		t.Fatalf("audit = %#v, %v", v, err)
	}
}

func TestApproveCostBaselineRollsBackWhenCheckpointAuditFails(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Baseline rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.SaveCostReserve(CostReserve{ProjectID: p.ID, Kind: "contingency", AmountMinorUnits: 100, Description: "Known risk"}); err != nil {
		t.Fatal(err)
	}
	var beforeEvents int
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=?`, p.ID).Scan(&beforeEvents); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Conn.Exec(`CREATE TRIGGER reject_cost_baseline_checkpoint BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'cost_baseline_snapshot.approval_checkpoint'
		BEGIN SELECT RAISE(ABORT, 'forced baseline checkpoint failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = d.ApproveCostBaseline(p.ID, "alice", "This must roll back"); err == nil || !strings.Contains(err.Error(), "forced baseline checkpoint failure") {
		t.Fatalf("approval error = %v", err)
	}
	if _, err = d.Conn.Exec(`DROP TRIGGER reject_cost_baseline_checkpoint`); err != nil {
		t.Fatal(err)
	}
	var snapshots, afterEvents, baselineEvents int
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM cost_baseline_snapshots WHERE project_id=?`, p.ID).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=?`, p.ID).Scan(&afterEvents); err != nil {
		t.Fatal(err)
	}
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=? AND entity_type='cost_baseline_snapshot'`, p.ID).Scan(&baselineEvents); err != nil {
		t.Fatal(err)
	}
	if snapshots != 0 || baselineEvents != 0 || afterEvents != beforeEvents {
		t.Fatalf("rollback snapshots=%d baselineEvents=%d audit events=%d, want 0, 0, %d", snapshots, baselineEvents, afterEvents, beforeEvents)
	}
	if v, err := d.VerifyAuditChain(p.ID); err != nil || !v.Valid {
		t.Fatalf("audit after rollback = %#v, %v", v, err)
	}
	approved, err := d.ApproveCostBaseline(p.ID, "alice", "Retry succeeds")
	if err != nil {
		t.Fatal(err)
	}
	assertBaselineAuditEvents(t, d, approved)
}

func TestApproveCostBaselineRejectsEmptyPlanWithoutLeakingConnection(t *testing.T) {
	d := newCostControlTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Empty baseline"})
	if err != nil {
		t.Fatal(err)
	}
	d.Conn.SetMaxOpenConns(1)
	if _, err = d.ApproveCostBaseline(p.ID, "alice", "No cost basis"); err == nil || !strings.Contains(err.Error(), "cost baseline must be positive") {
		t.Fatalf("approval error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = d.Conn.PingContext(ctx); err != nil {
		t.Fatalf("connection was not released after rejected approval: %v", err)
	}
	var snapshots, baselineEvents int
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM cost_baseline_snapshots WHERE project_id=?`, p.ID).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if err = d.Conn.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE project_id=? AND entity_type='cost_baseline_snapshot'`, p.ID).Scan(&baselineEvents); err != nil {
		t.Fatal(err)
	}
	if snapshots != 0 || baselineEvents != 0 {
		t.Fatalf("rejected approval persisted snapshots=%d baselineEvents=%d", snapshots, baselineEvents)
	}
}

func assertBaselineAuditEvents(t *testing.T, d *Database, snapshot CostBaselineSnapshot) {
	t.Helper()
	rows, err := d.Conn.Query(`SELECT event_type,user_id,after_canonical_json,signature_status,signature_blob_optional
		FROM audit_events WHERE project_id=? AND entity_type='cost_baseline_snapshot' AND entity_id=? ORDER BY sequence_number`, snapshot.ProjectID, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		if err = rows.Scan(&event.EventType, &event.UserID, &event.AfterCanonicalJSON, &event.SignatureStatus, &event.SignatureBlobOptional); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != "cost_baseline_snapshot.create" || events[1].EventType != "cost_baseline_snapshot.approval_checkpoint" {
		t.Fatalf("baseline audit events = %#v", events)
	}
	for _, event := range events {
		if event.UserID != snapshot.ApprovedBy {
			t.Fatalf("audit user = %q, want %q", event.UserID, snapshot.ApprovedBy)
		}
	}
	var recorded CostBaselineSnapshot
	if err = json.Unmarshal([]byte(events[0].AfterCanonicalJSON), &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != snapshot {
		t.Fatalf("create event snapshot = %#v, want %#v", recorded, snapshot)
	}
	var checkpoint struct {
		ApprovalType string `json:"approval_type"`
		EntityType   string `json:"entity_type"`
		EntityID     string `json:"entity_id"`
		PayloadHash  string `json:"payload_hash"`
	}
	if err = json.Unmarshal([]byte(events[1].AfterCanonicalJSON), &checkpoint); err != nil {
		t.Fatal(err)
	}
	wantHashBytes := sha256.Sum256([]byte(events[0].AfterCanonicalJSON))
	wantHash := hex.EncodeToString(wantHashBytes[:])
	if checkpoint.ApprovalType != "cost_baseline_approved" || checkpoint.EntityType != "cost_baseline_snapshot" || checkpoint.EntityID != snapshot.ID || checkpoint.PayloadHash != wantHash {
		t.Fatalf("checkpoint = %#v, want baseline approval hash %q", checkpoint, wantHash)
	}
	if events[1].SignatureStatus != "signed" {
		t.Fatalf("checkpoint signature status = %q, want signed", events[1].SignatureStatus)
	}
	var signature struct {
		PayloadHash string `json:"payload_hash"`
	}
	if err = json.Unmarshal([]byte(events[1].SignatureBlobOptional), &signature); err != nil {
		t.Fatal(err)
	}
	if signature.PayloadHash != wantHash {
		t.Fatalf("checkpoint signature payload hash = %q, want %q", signature.PayloadHash, wantHash)
	}
}
