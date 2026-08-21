// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"strings"

	"gopmgr/internal/db"
	"gopmgr/internal/money"
)

// CostEntryWire is the Wails-safe financial boundary. Amount is a canonical
// decimal string such as "12.34"; it is never a JavaScript number.
type CostEntryWire struct {
	ID          string `json:"id"`
	CostTypeID  string `json:"cost_type_id"`
	Kind        string `json:"kind"`
	CostDate    string `json:"cost_date"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
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
	saved, err := d.SaveCostEntry(db.CostEntry{ProjectID: p.ID, CostTypeID: input.CostTypeID, Kind: input.Kind, CostDate: input.CostDate, Description: strings.TrimSpace(input.Description), AmountMinorUnits: amount.MinorUnits})
	if err != nil {
		return CostEntryWire{}, err
	}
	return costEntryWire(saved), nil
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
	return CostEntryWire{ID: entry.ID, CostTypeID: entry.CostTypeID, Kind: entry.Kind, CostDate: entry.CostDate, Description: entry.Description, Amount: formatMoneyDecimal(money.Amount{MinorUnits: entry.AmountMinorUnits})}
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
