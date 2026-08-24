// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"gopmgr/internal/agile"
	"gopmgr/internal/budget"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/money"
)

// ExportFinancialReportPDF writes a printable, project-scoped snapshot of the
// legacy Budget context and Cost Control. It intentionally does not introduce
// Cost Control forecasts, allocations, drawdowns, or authority workflows.
func (a *App) ExportFinancialReportPDF() (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	report, err := buildFinancialReportSnapshot(d, time.Now().UTC())
	if err != nil {
		return "", err
	}
	bytes, err := documents.RenderFinancialReportPDF(report)
	if err != nil {
		return "", fmt.Errorf("render financial report: %w", err)
	}
	path, err := a.selectExportDestination(filepath.Join(u.DataDir, "exports"), fmt.Sprintf("%s-Financial-Report-%s.pdf", sanitizeFilename(report.ProjectName), report.GeneratedAt.Format("20060102-150405")), ".pdf", "Export project financial report")
	if err != nil {
		return "", err
	}
	if err := writeNewPrivateExport(path, bytes); err != nil {
		return "", err
	}
	return path, nil
}

// buildFinancialReportSnapshot holds a single SQLite read transaction open
// across every input to the printable report. That prevents an interleaved
// Wails mutation from producing a half-old Budget and half-new Cost Control
// document.
func buildFinancialReportSnapshot(d *db.Database, asOf time.Time) (documents.FinancialReport, error) {
	tx, err := d.Conn.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return documents.FinancialReport{}, fmt.Errorf("begin financial report snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var project db.Project
	if err := tx.QueryRow(`SELECT id,name,budget_minor_units,currency_code FROM project LIMIT 1`).Scan(&project.ID, &project.Name, &project.BudgetMinorUnits, &project.CurrencyCode); err != nil {
		return documents.FinancialReport{}, fmt.Errorf("read financial report project: %w", err)
	}
	stakeholders, err := snapshotStakeholders(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	workItems, err := snapshotWorkItems(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	types, err := snapshotCostTypes(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	entries, err := snapshotCostEntries(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	reserves, err := snapshotCostReserves(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	baselines, err := snapshotCostBaselines(tx, project.ID)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return documents.FinancialReport{}, fmt.Errorf("finish financial report snapshot: %w", err)
	}

	legacy, err := budget.Compute(project, stakeholders, workItems)
	if err != nil {
		return documents.FinancialReport{}, fmt.Errorf("compute financial report legacy Budget: %w", err)
	}
	legacyWire := budgetSummaryWire(legacy)
	control, err := financialCostControlSnapshot(types, entries, reserves, baselines)
	if err != nil {
		return documents.FinancialReport{}, err
	}
	return documents.FinancialReport{ProjectName: project.Name, CurrencyCode: project.CurrencyCode, GeneratedAt: asOf, Legacy: documents.FinancialLegacyBudget{Budget: legacyWire.Budget, ContractValue: legacyWire.ContractValue, LabourEstimate: legacyWire.LabourEstimate, Committed: legacyWire.Committed, Remaining: legacyWire.Remaining}, CostControl: control}, nil
}

func snapshotStakeholders(tx *sql.Tx, projectID string) ([]db.Stakeholder, error) {
	rows, err := tx.Query(`SELECT name,category,hourly_rate_minor_units,contract_value_minor_units FROM stakeholders WHERE project_id=? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []db.Stakeholder
	for rows.Next() {
		var s db.Stakeholder
		var category string
		if err := rows.Scan(&s.Name, &category, &s.HourlyRateMinorUnits, &s.ContractValueMinorUnits); err != nil {
			return nil, err
		}
		s.Category = db.StakeholderCategory(category)
		out = append(out, s)
	}
	return out, rows.Err()
}
func snapshotWorkItems(tx *sql.Tx, projectID string) ([]agile.WorkItem, error) {
	rows, err := tx.Query(`SELECT assignee,points FROM agile_work_items WHERE project_id=? ORDER BY order_idx,created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []agile.WorkItem
	for rows.Next() {
		var w agile.WorkItem
		if err := rows.Scan(&w.Assignee, &w.Points); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func snapshotCostTypes(tx *sql.Tx, projectID string) ([]db.CostType, error) {
	rows, err := tx.Query(`SELECT id,code,name,attribution,behavior,treatment,active FROM cost_types WHERE project_id=? ORDER BY code`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []db.CostType
	for rows.Next() {
		var c db.CostType
		var active int
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Attribution, &c.Behavior, &c.Treatment, &active); err != nil {
			return nil, err
		}
		c.ProjectID = projectID
		c.Active = active != 0
		out = append(out, c)
	}
	return out, rows.Err()
}
func snapshotCostEntries(tx *sql.Tx, projectID string) ([]db.CostEntry, error) {
	rows, err := tx.Query(`SELECT id,cost_type_id,kind,amount_minor_units,cost_date,description,quantity_milli_units,unit,item_name,sku,supplier_name,invoice_reference FROM cost_entries WHERE project_id=? ORDER BY cost_date DESC,created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []db.CostEntry
	for rows.Next() {
		var e db.CostEntry
		if err := rows.Scan(&e.ID, &e.CostTypeID, &e.Kind, &e.AmountMinorUnits, &e.CostDate, &e.Description, &e.QuantityMilliUnits, &e.Unit, &e.ItemName, &e.SKU, &e.SupplierName, &e.InvoiceReference); err != nil {
			return nil, err
		}
		e.ProjectID = projectID
		out = append(out, e)
	}
	return out, rows.Err()
}
func snapshotCostReserves(tx *sql.Tx, projectID string) ([]db.CostReserve, error) {
	rows, err := tx.Query(`SELECT id,kind,amount_minor_units,description FROM cost_reserves WHERE project_id=? ORDER BY kind`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []db.CostReserve
	for rows.Next() {
		var r db.CostReserve
		if err := rows.Scan(&r.ID, &r.Kind, &r.AmountMinorUnits, &r.Description); err != nil {
			return nil, err
		}
		r.ProjectID = projectID
		out = append(out, r)
	}
	return out, rows.Err()
}
func snapshotCostBaselines(tx *sql.Tx, projectID string) ([]db.CostBaselineSnapshot, error) {
	rows, err := tx.Query(`SELECT id,version,currency_code,planned_minor_units,contingency_minor_units,management_reserve_minor_units,approved_by,approval_note,approved_at FROM cost_baseline_snapshots WHERE project_id=? ORDER BY version DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []db.CostBaselineSnapshot
	for rows.Next() {
		var s db.CostBaselineSnapshot
		if err := rows.Scan(&s.ID, &s.Version, &s.CurrencyCode, &s.PlannedMinorUnits, &s.ContingencyMinorUnits, &s.ManagementReserveMinorUnits, &s.ApprovedBy, &s.ApprovalNote, &s.ApprovedAt); err != nil {
			return nil, err
		}
		s.ProjectID = projectID
		out = append(out, s)
	}
	return out, rows.Err()
}

func financialCostControlSnapshot(types []db.CostType, entries []db.CostEntry, reserves []db.CostReserve, baselines []db.CostBaselineSnapshot) (documents.FinancialCostControl, error) {
	typesByID := make(map[string]db.CostType, len(types))
	for _, t := range types {
		typesByID[t.ID] = t
	}
	var planned, commitment, actual, contingency, management money.Accumulator
	rows := make([]documents.FinancialLedgerEntry, 0, len(entries))
	for _, e := range entries {
		t, ok := typesByID[e.CostTypeID]
		if !ok {
			return documents.FinancialCostControl{}, fmt.Errorf("financial report cost entry %q has no owning-project cost type", e.ID)
		}
		amount := money.Amount{MinorUnits: e.AmountMinorUnits}
		switch e.Kind {
		case "planned":
			planned.Add(amount)
		case "commitment":
			commitment.Add(amount)
		case "actual":
			actual.Add(amount)
		default:
			return documents.FinancialCostControl{}, fmt.Errorf("financial report cost entry %q has unsupported kind %q", e.ID, e.Kind)
		}
		rows = append(rows, documents.FinancialLedgerEntry{Date: e.CostDate, State: e.Kind, Type: t.Name, Attribution: t.Attribution, Behavior: t.Behavior, Treatment: t.Treatment, Description: e.Description, Amount: amount.Decimal(), ItemName: e.ItemName, SKU: e.SKU, SupplierName: e.SupplierName, InvoiceReference: e.InvoiceReference, Quantity: formatQuantityDecimal(e.QuantityMilliUnits), Unit: e.Unit})
	}
	quantityAggregates, err := aggregateEntryQuantities(entries)
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	reserveRows := make([]documents.FinancialReserve, 0, len(reserves))
	for _, r := range reserves {
		amount := money.Amount{MinorUnits: r.AmountMinorUnits}
		switch r.Kind {
		case "contingency":
			contingency.Add(amount)
		case "management":
			management.Add(amount)
		default:
			return documents.FinancialCostControl{}, fmt.Errorf("financial report reserve has unsupported kind %q", r.Kind)
		}
		reserveRows = append(reserveRows, documents.FinancialReserve{Kind: r.Kind, Amount: amount.Decimal(), Description: r.Description})
	}
	pl, err := planned.Amount()
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	co, err := commitment.Amount()
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	ac, err := actual.Amount()
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	cont, err := contingency.Amount()
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	mgmt, err := management.Amount()
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	baseline, err := pl.Add(cont)
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	authority, err := baseline.Add(mgmt)
	if err != nil {
		return documents.FinancialCostControl{}, err
	}
	baselineRows := make([]documents.FinancialBaseline, 0, len(baselines))
	for _, s := range baselines {
		wire, err := costBaselineWire(s)
		if err != nil {
			return documents.FinancialCostControl{}, err
		}
		baselineRows = append(baselineRows, documents.FinancialBaseline{Version: wire.Version, Planned: wire.Planned, Contingency: wire.Contingency, CostBaseline: wire.CostBaseline, ManagementReserve: wire.ManagementReserve, AuthorisedFunding: wire.AuthorisedFunding, ApprovedBy: wire.ApprovedBy, ApprovalNote: wire.ApprovalNote, ApprovedAt: wire.ApprovedAt})
	}
	return documents.FinancialCostControl{Planned: pl.Decimal(), Contingency: cont.Decimal(), CostBaseline: baseline.Decimal(), ManagementReserve: mgmt.Decimal(), AuthorisedFunding: authority.Decimal(), Commitment: co.Decimal(), Actual: ac.Decimal(), Entries: rows, Reserves: reserveRows, Baselines: baselineRows, QuantityAggregates: quantityAggregates}, nil
}

// aggregateEntryQuantities mirrors db.(*Database).AggregateCostEntryQuantities
// but groups the entries already fetched inside buildFinancialReportSnapshot's
// single read transaction, rather than issuing a second query outside it --
// preserving that function's documented guarantee that the whole report is a
// single consistent snapshot.
func aggregateEntryQuantities(entries []db.CostEntry) ([]documents.FinancialQuantityAggregate, error) {
	type bucket struct {
		itemName, unit string
		total          money.Accumulator
		count          int
	}
	var order []string
	byKey := make(map[string]*bucket)
	for _, e := range entries {
		if e.ItemName == "" || e.Unit == "" || e.QuantityMilliUnits <= 0 {
			continue
		}
		key := e.ItemName + "\x00" + e.Unit
		b, ok := byKey[key]
		if !ok {
			b = &bucket{itemName: e.ItemName, unit: e.Unit}
			byKey[key] = b
			order = append(order, key)
		}
		b.total.Add(money.Amount{MinorUnits: e.QuantityMilliUnits})
		b.count++
	}
	sort.Strings(order)
	out := make([]documents.FinancialQuantityAggregate, 0, len(order))
	for _, key := range order {
		b := byKey[key]
		total, err := b.total.Amount()
		if err != nil {
			return nil, fmt.Errorf("aggregate quantity for %q/%q: %w", b.itemName, b.unit, err)
		}
		out = append(out, documents.FinancialQuantityAggregate{ItemName: b.itemName, Unit: b.unit, TotalQuantity: formatQuantityDecimal(total.MinorUnits), EntryCount: b.count})
	}
	return out, nil
}
