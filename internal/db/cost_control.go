// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopmgr/internal/money"
)

// CostType classifies a cost along independent accounting dimensions.
type CostType struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Attribution string `json:"attribution"`
	Behavior    string `json:"behavior"`
	Treatment   string `json:"treatment"`
	Active      bool   `json:"active"`
}

// CostEntry is an immutable amount-bearing ledger row once recorded. Amounts
// are canonical signed integer minor units; callers use a wire DTO at Wails.
//
// QuantityMilliUnits, Unit, ItemName, SKU, SupplierName, and InvoiceReference
// are optional structured procurement detail (project-cost-ledger-scope.md
// item 3). They are display-value SNAPSHOTS: when a value is chosen from the
// separate user-private catalog (internal/catalog), only its display text is
// copied here, never a foreign key into that database and never a supplier
// address/contact detail. A row with these fields blank remains a fully
// valid ordinary ledger entry, matching existing behavior.
type CostEntry struct {
	ID                 string `json:"id"`
	ProjectID          string `json:"project_id"`
	CostTypeID         string `json:"cost_type_id"`
	Kind               string `json:"kind"`
	CostDate           string `json:"cost_date"`
	Description        string `json:"description"`
	AmountMinorUnits   int64  `json:"amount_minor_units"`
	QuantityMilliUnits int64  `json:"quantity_milli_units"`
	Unit               string `json:"unit"`
	ItemName           string `json:"item_name"`
	SKU                string `json:"sku"`
	SupplierName       string `json:"supplier_name"`
	InvoiceReference   string `json:"invoice_reference"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// CostQuantityAggregate is a same-item/unit rollup of quantity across every
// ledger entry that carries both fields (project-cost-ledger-scope.md item
// 3's "enriched exports" requirement). It is derived/read-only; it is never
// persisted and never feeds baseline or funding totals.
type CostQuantityAggregate struct {
	ItemName                string `json:"item_name"`
	Unit                    string `json:"unit"`
	TotalQuantityMilliUnits int64  `json:"total_quantity_milli_units"`
	EntryCount              int    `json:"entry_count"`
}

const (
	// maxLedgerFieldLength bounds each optional procurement-detail text
	// field. These are short display snapshots (a unit symbol, a SKU, a
	// supplier name, an invoice number), not free-form notes, so this is
	// deliberately tighter than catalog.maxFieldLength (1000).
	maxLedgerFieldLength = 200
	// maxQuantityMajorUnits bounds a single entry's quantity so that
	// summing maxSearchLimit-many entries in AggregateCostEntryQuantities
	// stays far inside int64 milli-unit range without needing a
	// checked-overflow error path on every read.
	maxQuantityMajorUnits = 999_999_999
	maxQuantityMilliUnits = maxQuantityMajorUnits * 1000
	// maxSearchLength/maxSearchResults bound SearchCostEntries, mirroring
	// internal/catalog's identical constants for the same reason: a bounded
	// query length and result count keep search cheap and UI-representable.
	maxSearchLength  = 200
	maxSearchResults = 200
)

// CostReserve is the Phase 1 mutable assessed balance for one reserve kind.
// It is not an authorization or movement history; Phase 2 must introduce its
// own immutable lifecycle before a drawdown feature changes this meaning.
type CostReserve struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Kind             string `json:"kind"`
	AmountMinorUnits int64  `json:"amount_minor_units"`
	Description      string `json:"description"`
}

// CostBaselineSnapshot is an immutable, server-derived approval record. Its
// derived totals are calculated by callers with checked money arithmetic.
type CostBaselineSnapshot struct {
	ID                          string `json:"id"`
	ProjectID                   string `json:"project_id"`
	Version                     int64  `json:"version"`
	CurrencyCode                string `json:"currency_code"`
	PlannedMinorUnits           int64  `json:"planned_minor_units"`
	ContingencyMinorUnits       int64  `json:"contingency_minor_units"`
	ManagementReserveMinorUnits int64  `json:"management_reserve_minor_units"`
	ApprovedBy                  string `json:"approved_by"`
	ApprovalNote                string `json:"approval_note"`
	ApprovedAt                  string `json:"approved_at"`
}

var ErrNoCostType = errors.New("db: cost type not found")

func (db *Database) EnsureCostTypes(projectID string) error {
	seeds := []CostType{
		{Code: "labor", Name: "Labor & human resources", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "materials", Name: "Materials & equipment", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "software", Name: "Software & technology", Attribution: "direct", Behavior: "fixed", Treatment: "opex"},
		{Code: "travel", Name: "Travel & on-site", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "facilities", Name: "Facilities & utilities", Attribution: "indirect", Behavior: "fixed", Treatment: "opex"},
		{Code: "shared_support", Name: "Administrative & shared support", Attribution: "indirect", Behavior: "fixed", Treatment: "opex"},
		{Code: "quality", Name: "Quality, compliance & training", Attribution: "direct", Behavior: "variable", Treatment: "opex"},
		{Code: "capital", Name: "Capital equipment & licensing", Attribution: "direct", Behavior: "fixed", Treatment: "capex"},
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var existing int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM cost_types WHERE project_id = ?`, projectID).Scan(&existing); err != nil {
		return err
	}
	for _, seed := range seeds {
		id, idErr := newID("ctype")
		if idErr != nil {
			err = idErr
			return err
		}
		if _, err = tx.Exec(`INSERT INTO cost_types (id, project_id, code, name, attribution, behavior, treatment, active) VALUES (?, ?, ?, ?, ?, ?, ?, 1) ON CONFLICT(project_id, code) DO NOTHING`, id, projectID, seed.Code, seed.Name, seed.Attribution, seed.Behavior, seed.Treatment); err != nil {
			return err
		}
	}
	if existing == 0 {
		payload, marshalErr := json.Marshal(seeds)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: projectID, EventType: "cost_type.seed", EntityType: "cost_type", AfterJSON: string(payload)}); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (db *Database) ListCostTypes(projectID string) ([]CostType, error) {
	if err := db.EnsureCostTypes(projectID); err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(`SELECT id, project_id, code, name, attribution, behavior, treatment, active FROM cost_types WHERE project_id = ? ORDER BY code`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostType
	for rows.Next() {
		var c CostType
		var active int
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Code, &c.Name, &c.Attribution, &c.Behavior, &c.Treatment, &active); err != nil {
			return nil, err
		}
		c.Active = active != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *Database) SaveCostEntry(entry CostEntry) (CostEntry, error) {
	if entry.ProjectID == "" || entry.CostTypeID == "" || entry.Description == "" || entry.CostDate == "" {
		return CostEntry{}, errors.New("cost entry: project, cost type, date, and description are required")
	}
	if entry.AmountMinorUnits == 0 {
		return CostEntry{}, errors.New("cost entry: amount must not be zero")
	}
	if entry.Kind != "planned" && entry.Kind != "commitment" && entry.Kind != "actual" {
		return CostEntry{}, fmt.Errorf("cost entry: invalid kind %q", entry.Kind)
	}
	if _, err := time.Parse("2006-01-02", entry.CostDate); err != nil {
		return CostEntry{}, fmt.Errorf("cost entry: date must be YYYY-MM-DD: %w", err)
	}
	if err := validateCostEntryProcurementDetail(&entry); err != nil {
		return CostEntry{}, err
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return CostEntry{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var active int
	if err = tx.QueryRow(`SELECT active FROM cost_types WHERE id=? AND project_id=?`, entry.CostTypeID, entry.ProjectID).Scan(&active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CostEntry{}, ErrNoCostType
		}
		return CostEntry{}, err
	}
	if active == 0 {
		return CostEntry{}, errors.New("cost entry: cost type is archived")
	}
	if entry.ID == "" {
		entry.ID, err = newID("cost")
		if err != nil {
			return CostEntry{}, err
		}
	}
	now := captureTimestamp()
	if _, err = tx.Exec(`INSERT INTO cost_entries (id, project_id, cost_type_id, kind, amount_minor_units, cost_date, description, quantity_milli_units, unit, item_name, sku, supplier_name, invoice_reference, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ProjectID, entry.CostTypeID, entry.Kind, entry.AmountMinorUnits, entry.CostDate, entry.Description,
		entry.QuantityMilliUnits, entry.Unit, entry.ItemName, entry.SKU, entry.SupplierName, entry.InvoiceReference,
		now.text, now.text); err != nil {
		return CostEntry{}, err
	}
	entry.CreatedAt, entry.UpdatedAt = now.text, now.text
	after, err := json.Marshal(entry)
	if err != nil {
		return CostEntry{}, err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: entry.ProjectID, EventType: "cost_entry.create", EntityType: "cost_entry", EntityID: entry.ID, AfterJSON: string(after)}); err != nil {
		return CostEntry{}, err
	}
	err = tx.Commit()
	return entry, err
}

const costEntrySelectColumns = `id,project_id,cost_type_id,kind,amount_minor_units,cost_date,description,quantity_milli_units,unit,item_name,sku,supplier_name,invoice_reference,created_at,updated_at`

func scanCostEntry(row interface{ Scan(...any) error }) (CostEntry, error) {
	var e CostEntry
	err := row.Scan(&e.ID, &e.ProjectID, &e.CostTypeID, &e.Kind, &e.AmountMinorUnits, &e.CostDate, &e.Description,
		&e.QuantityMilliUnits, &e.Unit, &e.ItemName, &e.SKU, &e.SupplierName, &e.InvoiceReference,
		&e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (db *Database) ListCostEntries(projectID string) ([]CostEntry, error) {
	rows, err := db.Conn.Query(`SELECT `+costEntrySelectColumns+` FROM cost_entries WHERE project_id=? ORDER BY cost_date DESC, created_at DESC`, projectID) // timestamp-order-guard-exempt: Cost Control tables are new in this schema and write created_at only through captureTimestamp's fixed-width UTC timestampLayout; lexicographic order is chronological. See timestamps.go.
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostEntry
	for rows.Next() {
		e, err := scanCostEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SearchCostEntries returns ledger rows whose description or structured
// procurement detail (item, SKU, supplier, invoice reference) contains query,
// case-insensitively. An empty query delegates to ListCostEntries verbatim
// (no cap) -- maxSearchResults bounds only an actual substring search, where
// a user who over-matches can narrow their own query; it must never silently
// truncate what would otherwise be the complete, uncapped ledger view.
func (db *Database) SearchCostEntries(projectID, query string) ([]CostEntry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return db.ListCostEntries(projectID)
	}
	if len(query) > maxSearchLength {
		return nil, errors.New("cost entry search: query is too long")
	}
	needle := ledgerSearchNeedle(query)
	rows, err := db.Conn.Query(
		`SELECT `+costEntrySelectColumns+` FROM cost_entries
		 WHERE project_id=? AND (
			description LIKE ? ESCAPE '\' OR item_name LIKE ? ESCAPE '\' OR sku LIKE ? ESCAPE '\' OR
			supplier_name LIKE ? ESCAPE '\' OR invoice_reference LIKE ? ESCAPE '\'
		 )
		 ORDER BY cost_date DESC, created_at DESC LIMIT ?`, // timestamp-order-guard-exempt: same as ListCostEntries above -- cost_entries.created_at is written only through captureTimestamp's fixed-width UTC timestampLayout, so lexicographic order is chronological.
		projectID, needle, needle, needle, needle, needle, maxSearchResults,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostEntry
	for rows.Next() {
		e, err := scanCostEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AggregateCostEntryQuantities sums quantity across every ledger entry that
// shares the same (item_name, unit) pair, ignoring entries where either is
// blank -- there is nothing meaningful to group an entry without both into.
// Summation happens in Go via money.Accumulator (an exact int64 big.Int
// accumulator; reused here purely for its overflow-checked arithmetic, not
// for any monetary meaning) rather than SQL SUM(), because SQLite's SUM over
// an INTEGER column silently promotes to floating point on int64 overflow
// instead of erroring -- exactly the failure mode this function must not
// have, given it renders directly into the UI and the financial-report PDF.
func (db *Database) AggregateCostEntryQuantities(projectID string) ([]CostQuantityAggregate, error) {
	rows, err := db.Conn.Query(
		`SELECT item_name, unit, quantity_milli_units
		 FROM cost_entries
		 WHERE project_id=? AND item_name != '' AND unit != '' AND quantity_milli_units > 0
		 ORDER BY item_name, unit`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type bucket struct {
		itemName, unit string
		total          money.Accumulator
		count          int
	}
	var order []string
	byKey := make(map[string]*bucket)
	for rows.Next() {
		var itemName, unit string
		var milliUnits int64
		if err := rows.Scan(&itemName, &unit, &milliUnits); err != nil {
			return nil, err
		}
		key := itemName + "\x00" + unit
		b, ok := byKey[key]
		if !ok {
			b = &bucket{itemName: itemName, unit: unit}
			byKey[key] = b
			order = append(order, key)
		}
		b.total.Add(money.Amount{MinorUnits: milliUnits})
		b.count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]CostQuantityAggregate, 0, len(order))
	for _, key := range order {
		b := byKey[key]
		total, err := b.total.Amount()
		if err != nil {
			return nil, fmt.Errorf("aggregate quantity for %q/%q: %w", b.itemName, b.unit, err)
		}
		out = append(out, CostQuantityAggregate{ItemName: b.itemName, Unit: b.unit, TotalQuantityMilliUnits: total.MinorUnits, EntryCount: b.count})
	}
	return out, nil
}

// ledgerSearchNeedle escapes SQL LIKE metacharacters in query and wraps it
// for a substring match, mirroring internal/catalog's searchNeedle.
func ledgerSearchNeedle(query string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return "%" + escaped + "%"
}

// validateCostEntryProcurementDetail bounds and cross-checks the optional
// structured fields. Quantity without a unit is meaningless for
// AggregateCostEntryQuantities, so it is rejected rather than silently
// accepted and excluded from every rollup.
func validateCostEntryProcurementDetail(entry *CostEntry) error {
	entry.Unit = strings.TrimSpace(entry.Unit)
	entry.ItemName = strings.TrimSpace(entry.ItemName)
	entry.SKU = strings.TrimSpace(entry.SKU)
	entry.SupplierName = strings.TrimSpace(entry.SupplierName)
	entry.InvoiceReference = strings.TrimSpace(entry.InvoiceReference)
	if ledgerFieldTooLong(entry.Unit, entry.ItemName, entry.SKU, entry.SupplierName, entry.InvoiceReference) {
		return errors.New("cost entry: quantity, unit, item, SKU, supplier, and invoice reference fields must each be 200 characters or fewer")
	}
	if entry.QuantityMilliUnits < 0 || entry.QuantityMilliUnits > maxQuantityMilliUnits {
		return errors.New("cost entry: quantity is out of range")
	}
	if entry.QuantityMilliUnits > 0 && entry.Unit == "" {
		return errors.New("cost entry: quantity requires a unit")
	}
	return nil
}

func ledgerFieldTooLong(values ...string) bool {
	for _, v := range values {
		if len(v) > maxLedgerFieldLength {
			return true
		}
	}
	return false
}

func (db *Database) ListCostReserves(projectID string) ([]CostReserve, error) {
	rows, err := db.Conn.Query(`SELECT id,project_id,kind,amount_minor_units,description FROM cost_reserves WHERE project_id=? ORDER BY kind`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostReserve
	for rows.Next() {
		var r CostReserve
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Kind, &r.AmountMinorUnits, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (db *Database) SaveCostReserve(r CostReserve) (CostReserve, error) {
	if r.ProjectID == "" || (r.Kind != "contingency" && r.Kind != "management") || r.AmountMinorUnits < 0 || strings.TrimSpace(r.Description) == "" {
		return CostReserve{}, errors.New("cost reserve: invalid input")
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return CostReserve{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var existingID string
	lookupErr := tx.QueryRow(`SELECT id FROM cost_reserves WHERE project_id = ? AND kind = ?`, r.ProjectID, r.Kind).Scan(&existingID)
	switch lookupErr {
	case nil:
		r.ID = existingID
	case sql.ErrNoRows:
		if r.ID == "" {
			r.ID, err = newID("reserve")
			if err != nil {
				return CostReserve{}, err
			}
		}
	default:
		return CostReserve{}, lookupErr
	}
	now := captureTimestamp()
	_, err = tx.Exec(`INSERT INTO cost_reserves (id,project_id,kind,amount_minor_units,description,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(project_id,kind) DO UPDATE SET amount_minor_units=excluded.amount_minor_units,description=excluded.description,updated_at=excluded.updated_at`, r.ID, r.ProjectID, r.Kind, r.AmountMinorUnits, r.Description, now.text, now.text)
	if err != nil {
		return CostReserve{}, err
	}
	if err := tx.QueryRow(`SELECT id FROM cost_reserves WHERE project_id = ? AND kind = ?`, r.ProjectID, r.Kind).Scan(&r.ID); err != nil {
		return CostReserve{}, err
	}
	after, _ := json.Marshal(r)
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: r.ProjectID, EventType: "cost_reserve.save", EntityType: "cost_reserve", EntityID: r.ID, AfterJSON: string(after)}); err != nil {
		return CostReserve{}, err
	}
	err = tx.Commit()
	return r, err
}

// ApproveCostBaseline snapshots the current Cost Control plan and reserves.
// Project.BudgetMinorUnits is deliberately excluded: it is the legacy Budget
// panel's independent rollup, not Cost Control funding.
func (db *Database) ApproveCostBaseline(projectID, actor, note string) (CostBaselineSnapshot, error) {
	if projectID == "" || strings.TrimSpace(actor) == "" || strings.TrimSpace(note) == "" {
		return CostBaselineSnapshot{}, errors.New("cost baseline approval: project, actor, and note are required")
	}
	tx, err := db.Conn.Begin()
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	defer func() { _ = tx.Rollback() }() // harmless after Commit; releases every early-return path.
	var currency string
	if err = tx.QueryRow(`SELECT currency_code FROM project WHERE id=?`, projectID).Scan(&currency); err != nil {
		return CostBaselineSnapshot{}, err
	}
	var planned money.Accumulator
	rows, err := tx.Query(`SELECT amount_minor_units FROM cost_entries WHERE project_id=? AND kind='planned'`, projectID)
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	for rows.Next() {
		var n int64
		if err = rows.Scan(&n); err != nil {
			_ = rows.Close()
			return CostBaselineSnapshot{}, err
		}
		planned.Add(money.Amount{MinorUnits: n})
	}
	if err = rows.Close(); err != nil {
		return CostBaselineSnapshot{}, err
	}
	pl, err := planned.Amount()
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	var contingency, management int64
	rows, err = tx.Query(`SELECT kind,amount_minor_units FROM cost_reserves WHERE project_id=?`, projectID)
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	for rows.Next() {
		var kind string
		var n int64
		if err = rows.Scan(&kind, &n); err != nil {
			_ = rows.Close()
			return CostBaselineSnapshot{}, err
		}
		switch kind {
		case "contingency":
			contingency = n
		case "management":
			management = n
		}
	}
	if err = rows.Close(); err != nil {
		return CostBaselineSnapshot{}, err
	}
	base, err := pl.Add(money.Amount{MinorUnits: contingency})
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	if base.MinorUnits <= 0 {
		return CostBaselineSnapshot{}, errors.New("cost baseline approval: cost baseline must be positive")
	}
	if _, err = base.Add(money.Amount{MinorUnits: management}); err != nil {
		return CostBaselineSnapshot{}, err
	}
	var version int64
	if err = tx.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM cost_baseline_snapshots WHERE project_id=?`, projectID).Scan(&version); err != nil {
		return CostBaselineSnapshot{}, err
	}
	id, err := newID("costbase")
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	now := captureTimestamp()
	s := CostBaselineSnapshot{ID: id, ProjectID: projectID, Version: version, CurrencyCode: currency, PlannedMinorUnits: pl.MinorUnits, ContingencyMinorUnits: contingency, ManagementReserveMinorUnits: management, ApprovedBy: strings.TrimSpace(actor), ApprovalNote: strings.TrimSpace(note), ApprovedAt: now.text}
	if _, err = tx.Exec(`INSERT INTO cost_baseline_snapshots (id,project_id,version,currency_code,planned_minor_units,contingency_minor_units,management_reserve_minor_units,approved_by,approval_note,approved_at,approved_at_unixnano) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, s.ID, s.ProjectID, s.Version, s.CurrencyCode, s.PlannedMinorUnits, s.ContingencyMinorUnits, s.ManagementReserveMinorUnits, s.ApprovedBy, s.ApprovalNote, s.ApprovedAt, now.unixNano); err != nil {
		return CostBaselineSnapshot{}, err
	}
	payload, err := json.Marshal(s)
	if err != nil {
		return CostBaselineSnapshot{}, err
	}
	if _, err = appendAuditEventTx(tx, AuditEventInput{ProjectID: projectID, EventType: "cost_baseline_snapshot.create", EntityType: "cost_baseline_snapshot", EntityID: s.ID, AfterJSON: string(payload), UserID: s.ApprovedBy}); err != nil {
		return CostBaselineSnapshot{}, err
	}
	if _, err = appendApprovalCheckpointForUserTx(tx, projectID, "cost_baseline_snapshot", s.ID, "cost_baseline_approved", string(payload), s.ApprovedBy); err != nil {
		return CostBaselineSnapshot{}, err
	}
	err = tx.Commit()
	return s, err
}

func (db *Database) ListCostBaselines(projectID string) ([]CostBaselineSnapshot, error) {
	rows, err := db.Conn.Query(`SELECT id,project_id,version,currency_code,planned_minor_units,contingency_minor_units,management_reserve_minor_units,approved_by,approval_note,approved_at FROM cost_baseline_snapshots WHERE project_id=? ORDER BY version DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostBaselineSnapshot
	for rows.Next() {
		var s CostBaselineSnapshot
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Version, &s.CurrencyCode, &s.PlannedMinorUnits, &s.ContingencyMinorUnits, &s.ManagementReserveMinorUnits, &s.ApprovedBy, &s.ApprovalNote, &s.ApprovedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func validCurrencyCode(code string) bool {
	// Phase 1 is intentionally single-currency per project and does not
	// implement FX. Keep the accepted reporting currencies in a deliberately
	// small, UI-representable set rather than claiming every three-letter string
	// is an ISO 4217 code.
	_, ok := supportedProjectCurrencies[code]
	return ok
}

// CanonicalProjectCurrency normalizes a project's reporting currency and
// applies the legacy default. It deliberately accepts JPY because existing
// projects remain readable even though new Cost Control writes are contained.
func CanonicalProjectCurrency(code string) (string, error) {
	code = normaliseCurrencyCode(code)
	if code == "" {
		code = "USD"
	}
	if !validCurrencyCode(code) {
		return "", fmt.Errorf("unsupported project reporting currency %q", code)
	}
	return code, nil
}

var supportedProjectCurrencies = map[string]struct{}{
	"AUD": {},
	"CAD": {},
	"CHF": {},
	"EUR": {},
	"GBP": {},
	"JPY": {},
	"USD": {},
}

func normaliseCurrencyCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }
