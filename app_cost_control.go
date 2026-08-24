// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gopmgr/internal/db"
	"gopmgr/internal/money"
)

// CostEntryWire is the Wails-safe financial boundary. Amount and Quantity are
// canonical decimal strings such as "12.34" / "2.500"; neither is ever a
// JavaScript number. Quantity, Unit, ItemName, SKU, SupplierName, and
// InvoiceReference are optional structured procurement detail
// (project-cost-ledger-scope.md item 3); an empty Quantity means "not set",
// distinct from a quantity of zero (which SaveCostEntry rejects).
type CostEntryWire struct {
	ID               string `json:"id"`
	CostTypeID       string `json:"cost_type_id"`
	Kind             string `json:"kind"`
	CostDate         string `json:"cost_date"`
	Description      string `json:"description"`
	Amount           string `json:"amount"`
	Quantity         string `json:"quantity"`
	Unit             string `json:"unit"`
	ItemName         string `json:"item_name"`
	SKU              string `json:"sku"`
	SupplierName     string `json:"supplier_name"`
	InvoiceReference string `json:"invoice_reference"`
}

// CostQuantityAggregateWire mirrors db.CostQuantityAggregate at the Wails
// boundary with a canonical decimal quantity string.
type CostQuantityAggregateWire struct {
	ItemName      string `json:"item_name"`
	Unit          string `json:"unit"`
	TotalQuantity string `json:"total_quantity"`
	EntryCount    int    `json:"entry_count"`
}

const quantityMilliUnitsPerMajor = 1000

// parseQuantityDecimal parses an unsigned major-unit decimal quantity (not
// money) with at most three fractional digits into milli-units. Modeled on
// money.ParseDecimal's boundary discipline -- reject whitespace, exponents,
// leading zeroes -- but unsigned (a quantity is never negative) and
// three fractional digits (covers kg/L/hr/ea-style ledger units). An empty
// string means "not set" and returns (0, nil).
func parseQuantityDecimal(v string) (int64, error) {
	if v == "" {
		return 0, nil
	}
	if strings.TrimSpace(v) != v {
		return 0, errors.New("quantity must not contain surrounding whitespace")
	}
	if strings.HasPrefix(v, "-") {
		return 0, errors.New("quantity must not be negative")
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && (parts[1] == "" || len(parts[1]) > 3)) {
		return 0, errors.New("quantity must be a decimal with at most three fractional digits")
	}
	if len(parts[0]) > 1 && parts[0][0] == '0' {
		return 0, errors.New("quantity must not contain leading zeroes")
	}
	whole, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("quantity: %w", err)
	}
	frac := uint64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		for len(fraction) < 3 {
			fraction += "0"
		}
		frac, err = strconv.ParseUint(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("quantity: %w", err)
		}
	}
	const maxUint64 = ^uint64(0)
	if whole > (maxUint64-frac)/quantityMilliUnitsPerMajor {
		return 0, errors.New("quantity is too large")
	}
	milli := whole*quantityMilliUnitsPerMajor + frac
	if milli > math.MaxInt64 {
		return 0, errors.New("quantity is too large")
	}
	return int64(milli), nil
}

// formatQuantityDecimal returns a canonical, fixed-three-decimal major-unit
// representation, or "" for zero/unset -- distinguishing an entry with no
// quantity from one whose quantity happens to be zero (which SaveCostEntry
// never persists; see validateCostEntryProcurementDetail).
func formatQuantityDecimal(milliUnits int64) string {
	if milliUnits <= 0 {
		return ""
	}
	return fmt.Sprintf("%d.%03d", milliUnits/quantityMilliUnitsPerMajor, milliUnits%quantityMilliUnitsPerMajor)
}

type CostSummaryWire struct {
	CurrencyCode           string `json:"currency_code"`
	MutationDisabledReason string `json:"mutation_disabled_reason"`
	LegacyBudget           string `json:"legacy_budget"`
	Planned                string `json:"planned"`
	Contingency            string `json:"contingency"`
	CostBaseline           string `json:"cost_baseline"`
	ManagementReserve      string `json:"management_reserve"`
	AuthorisedFunding      string `json:"authorised_funding"`
	Commitment             string `json:"commitment"`
	Actual                 string `json:"actual"`
}

// CostClassificationRowWire is one bucket in an independent Cost Control
// classification lens. Rows from different lenses must never be summed.
type CostClassificationRowWire struct {
	Value      string `json:"value"`
	Planned    string `json:"planned"`
	Commitment string `json:"commitment"`
	Actual     string `json:"actual"`
}

// CostClassificationSummaryWire reconciles the ledger through independent
// attribution, behavior, and accounting-treatment lenses.
type CostClassificationSummaryWire struct {
	Attribution []CostClassificationRowWire `json:"attribution"`
	Behavior    []CostClassificationRowWire `json:"behavior"`
	Treatment   []CostClassificationRowWire `json:"treatment"`
}
type CostReserveWire struct {
	Kind        string `json:"kind"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
}

type CostBaselineWire struct {
	Version           int64  `json:"version"`
	CurrencyCode      string `json:"currency_code"`
	Planned           string `json:"planned"`
	Contingency       string `json:"contingency"`
	CostBaseline      string `json:"cost_baseline"`
	ManagementReserve string `json:"management_reserve"`
	AuthorisedFunding string `json:"authorised_funding"`
	ApprovedBy        string `json:"approved_by"`
	ApprovalNote      string `json:"approval_note"`
	ApprovedAt        string `json:"approved_at"`
}

func (a *App) ListCostTypes() ([]db.CostType, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListCostTypes(p.ID)
}

func (a *App) ListCostEntries() ([]CostEntryWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]CostEntryWire, 0, len(entries))
	for _, entry := range entries {
		out = append(out, costEntryWire(entry))
	}
	return out, nil
}

func (a *App) ComputeCostClassificationSummary() (CostClassificationSummaryWire, error) {
	d := a.requireDB()
	if d == nil {
		return CostClassificationSummaryWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	types, err := d.ListCostTypes(p.ID)
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	typesByID := make(map[string]db.CostType, len(types))
	for _, costType := range types {
		typesByID[costType.ID] = costType
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	return classifyCostEntries(entries, typesByID)
}
func (a *App) ListCostReserves() ([]CostReserveWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	items, err := d.ListCostReserves(p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]CostReserveWire, 0, len(items))
	for _, r := range items {
		out = append(out, CostReserveWire{Kind: r.Kind, Amount: formatMoneyDecimal(money.Amount{MinorUnits: r.AmountMinorUnits}), Description: r.Description})
	}
	return out, nil
}
func (a *App) SaveCostReserve(input CostReserveWire) (CostReserveWire, error) {
	d := a.requireDB()
	if d == nil {
		return CostReserveWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostReserveWire{}, err
	}
	if err := costControlMutationAllowed(p); err != nil {
		return CostReserveWire{}, err
	}
	amount, err := parseMoneyDecimal(input.Amount)
	if err != nil {
		return CostReserveWire{}, err
	}
	saved, err := d.SaveCostReserve(db.CostReserve{ProjectID: p.ID, Kind: input.Kind, AmountMinorUnits: amount.MinorUnits, Description: strings.TrimSpace(input.Description)})
	if err != nil {
		return CostReserveWire{}, err
	}
	return CostReserveWire{Kind: saved.Kind, Amount: formatMoneyDecimal(money.Amount{MinorUnits: saved.AmountMinorUnits}), Description: saved.Description}, nil
}

func (a *App) SaveCostEntry(input CostEntryWire) (CostEntryWire, error) {
	d := a.requireDB()
	if d == nil {
		return CostEntryWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostEntryWire{}, err
	}
	if err := costControlMutationAllowed(p); err != nil {
		return CostEntryWire{}, err
	}
	amount, err := parseMoneyDecimal(input.Amount)
	if err != nil {
		return CostEntryWire{}, err
	}
	quantity, err := parseQuantityDecimal(input.Quantity)
	if err != nil {
		return CostEntryWire{}, err
	}
	saved, err := d.SaveCostEntry(db.CostEntry{
		ProjectID: p.ID, CostTypeID: input.CostTypeID, Kind: input.Kind, CostDate: input.CostDate,
		Description: strings.TrimSpace(input.Description), AmountMinorUnits: amount.MinorUnits,
		QuantityMilliUnits: quantity, Unit: input.Unit, ItemName: input.ItemName, SKU: input.SKU,
		SupplierName: input.SupplierName, InvoiceReference: input.InvoiceReference,
	})
	if err != nil {
		return CostEntryWire{}, err
	}
	return costEntryWire(saved), nil
}

// SearchCostEntries returns ledger rows whose description or structured
// procurement detail matches query (case-insensitive substring); an empty
// query returns the same rows as ListCostEntries.
func (a *App) SearchCostEntries(query string) ([]CostEntryWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	entries, err := d.SearchCostEntries(p.ID, query)
	if err != nil {
		return nil, err
	}
	out := make([]CostEntryWire, 0, len(entries))
	for _, entry := range entries {
		out = append(out, costEntryWire(entry))
	}
	return out, nil
}

// AggregateCostEntryQuantities sums quantity across every ledger entry that
// shares the same item and unit (project-cost-ledger-scope.md item 3's
// "enriched exports" requirement).
func (a *App) AggregateCostEntryQuantities() ([]CostQuantityAggregateWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	rows, err := d.AggregateCostEntryQuantities(p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]CostQuantityAggregateWire, 0, len(rows))
	for _, r := range rows {
		out = append(out, CostQuantityAggregateWire{ItemName: r.ItemName, Unit: r.Unit, TotalQuantity: formatQuantityDecimal(r.TotalQuantityMilliUnits), EntryCount: r.EntryCount})
	}
	return out, nil
}

func (a *App) ApproveCostBaseline(note string) (CostBaselineWire, error) {
	d := a.requireDB()
	user := a.requireUser()
	if d == nil || user == nil {
		return CostBaselineWire{}, errors.New("no signed-in project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostBaselineWire{}, err
	}
	if err := costControlMutationAllowed(p); err != nil {
		return CostBaselineWire{}, err
	}
	s, err := d.ApproveCostBaseline(p.ID, user.Username, strings.TrimSpace(note))
	if err != nil {
		return CostBaselineWire{}, err
	}
	return costBaselineWire(s)
}

func (a *App) ListCostBaselines() ([]CostBaselineWire, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	items, err := d.ListCostBaselines(p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]CostBaselineWire, 0, len(items))
	for _, item := range items {
		wire, err := costBaselineWire(item)
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return out, nil
}

func (a *App) ComputeCostSummary() (CostSummaryWire, error) {
	d := a.requireDB()
	if d == nil {
		return CostSummaryWire{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return CostSummaryWire{}, err
	}
	entries, err := d.ListCostEntries(p.ID)
	if err != nil {
		return CostSummaryWire{}, err
	}
	var planned, commitment, actual money.Accumulator
	for _, entry := range entries {
		switch entry.Kind {
		case "planned":
			planned.Add(money.Amount{MinorUnits: entry.AmountMinorUnits})
		case "commitment":
			commitment.Add(money.Amount{MinorUnits: entry.AmountMinorUnits})
		case "actual":
			actual.Add(money.Amount{MinorUnits: entry.AmountMinorUnits})
		}
	}
	pl, err := planned.Amount()
	if err != nil {
		return CostSummaryWire{}, fmt.Errorf("cost planned total: %w", err)
	}
	co, err := commitment.Amount()
	if err != nil {
		return CostSummaryWire{}, fmt.Errorf("cost commitment total: %w", err)
	}
	ac, err := actual.Amount()
	if err != nil {
		return CostSummaryWire{}, fmt.Errorf("cost actual total: %w", err)
	}
	reserves, err := d.ListCostReserves(p.ID)
	if err != nil {
		return CostSummaryWire{}, err
	}
	var cont, mgmt money.Accumulator
	for _, r := range reserves {
		if r.Kind == "contingency" {
			cont.Add(money.Amount{MinorUnits: r.AmountMinorUnits})
		} else {
			mgmt.Add(money.Amount{MinorUnits: r.AmountMinorUnits})
		}
	}
	c, err := cont.Amount()
	if err != nil {
		return CostSummaryWire{}, err
	}
	m, err := mgmt.Amount()
	if err != nil {
		return CostSummaryWire{}, err
	}
	legacyBudget := money.Amount{MinorUnits: p.BudgetMinorUnits}
	baseline, err := pl.Add(c)
	if err != nil {
		return CostSummaryWire{}, err
	}
	authority, err := baseline.Add(m)
	if err != nil {
		return CostSummaryWire{}, err
	}
	return CostSummaryWire{CurrencyCode: p.CurrencyCode, MutationDisabledReason: costControlMutationDisabledReason(p), LegacyBudget: formatMoneyDecimal(legacyBudget), Planned: formatMoneyDecimal(pl), Contingency: formatMoneyDecimal(c), CostBaseline: formatMoneyDecimal(baseline), ManagementReserve: formatMoneyDecimal(m), AuthorisedFunding: formatMoneyDecimal(authority), Commitment: formatMoneyDecimal(co), Actual: formatMoneyDecimal(ac)}, nil
}

func costControlMutationAllowed(project db.Project) error {
	if reason := costControlMutationDisabledReason(project); reason != "" {
		return errors.New(reason)
	}
	return nil
}

func costControlMutationDisabledReason(project db.Project) string {
	if project.CurrencyCode == "JPY" {
		return "Cost Control is read-only for this legacy JPY project: existing amounts retain their original fixed two-decimal convention and are not being converted."
	}
	return ""
}

func costEntryWire(entry db.CostEntry) CostEntryWire {
	return CostEntryWire{
		ID: entry.ID, CostTypeID: entry.CostTypeID, Kind: entry.Kind, CostDate: entry.CostDate, Description: entry.Description,
		Amount: formatMoneyDecimal(money.Amount{MinorUnits: entry.AmountMinorUnits}), Quantity: formatQuantityDecimal(entry.QuantityMilliUnits),
		Unit: entry.Unit, ItemName: entry.ItemName, SKU: entry.SKU, SupplierName: entry.SupplierName, InvoiceReference: entry.InvoiceReference,
	}
}

func costBaselineWire(s db.CostBaselineSnapshot) (CostBaselineWire, error) {
	pl := money.Amount{MinorUnits: s.PlannedMinorUnits}
	cont := money.Amount{MinorUnits: s.ContingencyMinorUnits}
	mgmt := money.Amount{MinorUnits: s.ManagementReserveMinorUnits}
	base, err := pl.Add(cont)
	if err != nil {
		return CostBaselineWire{}, fmt.Errorf("cost baseline snapshot %q: %w", s.ID, err)
	}
	authority, err := base.Add(mgmt)
	if err != nil {
		return CostBaselineWire{}, fmt.Errorf("cost baseline snapshot %q: %w", s.ID, err)
	}
	return CostBaselineWire{Version: s.Version, CurrencyCode: s.CurrencyCode, Planned: formatMoneyDecimal(pl), Contingency: formatMoneyDecimal(cont), CostBaseline: formatMoneyDecimal(base), ManagementReserve: formatMoneyDecimal(mgmt), AuthorisedFunding: formatMoneyDecimal(authority), ApprovedBy: s.ApprovedBy, ApprovalNote: s.ApprovalNote, ApprovedAt: s.ApprovedAt}, nil
}

func classifyCostEntries(entries []db.CostEntry, typesByID map[string]db.CostType) (CostClassificationSummaryWire, error) {
	type totals struct {
		planned    money.Accumulator
		commitment money.Accumulator
		actual     money.Accumulator
	}
	newLens := func(values []string) map[string]*totals {
		out := make(map[string]*totals, len(values))
		for _, value := range values {
			out[value] = &totals{}
		}
		return out
	}
	attribution := newLens([]string{"direct", "indirect"})
	behavior := newLens([]string{"fixed", "variable"})
	treatment := newLens([]string{"capex", "opex", "not_applicable"})
	for _, entry := range entries {
		costType, ok := typesByID[entry.CostTypeID]
		if !ok {
			return CostClassificationSummaryWire{}, fmt.Errorf("cost entry %q: cost type is outside the open project", entry.ID)
		}
		for _, bucket := range []struct {
			name   string
			value  string
			values map[string]*totals
		}{
			{name: "attribution", value: costType.Attribution, values: attribution},
			{name: "behavior", value: costType.Behavior, values: behavior},
			{name: "treatment", value: costType.Treatment, values: treatment},
		} {
			total, ok := bucket.values[bucket.value]
			if !ok {
				return CostClassificationSummaryWire{}, fmt.Errorf("cost type %q: unsupported %s %q", costType.ID, bucket.name, bucket.value)
			}
			amount := money.Amount{MinorUnits: entry.AmountMinorUnits}
			switch entry.Kind {
			case "planned":
				total.planned.Add(amount)
			case "commitment":
				total.commitment.Add(amount)
			case "actual":
				total.actual.Add(amount)
			default:
				return CostClassificationSummaryWire{}, fmt.Errorf("cost entry %q: unsupported kind %q", entry.ID, entry.Kind)
			}
		}
	}
	toRows := func(values []string, source map[string]*totals) ([]CostClassificationRowWire, error) {
		out := make([]CostClassificationRowWire, 0, len(values))
		for _, value := range values {
			total := source[value]
			planned, err := total.planned.Amount()
			if err != nil {
				return nil, fmt.Errorf("cost classification %s planned: %w", value, err)
			}
			commitment, err := total.commitment.Amount()
			if err != nil {
				return nil, fmt.Errorf("cost classification %s commitment: %w", value, err)
			}
			actual, err := total.actual.Amount()
			if err != nil {
				return nil, fmt.Errorf("cost classification %s actual: %w", value, err)
			}
			out = append(out, CostClassificationRowWire{Value: value, Planned: formatMoneyDecimal(planned), Commitment: formatMoneyDecimal(commitment), Actual: formatMoneyDecimal(actual)})
		}
		return out, nil
	}
	attributionRows, err := toRows([]string{"direct", "indirect"}, attribution)
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	behaviorRows, err := toRows([]string{"fixed", "variable"}, behavior)
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	treatmentRows, err := toRows([]string{"capex", "opex", "not_applicable"}, treatment)
	if err != nil {
		return CostClassificationSummaryWire{}, err
	}
	return CostClassificationSummaryWire{Attribution: attributionRows, Behavior: behaviorRows, Treatment: treatmentRows}, nil
}

func parseMoneyDecimal(v string) (money.Amount, error) {
	return money.ParseDecimal(v)
}
func formatMoneyDecimal(amount money.Amount) string {
	return amount.Decimal()
}
