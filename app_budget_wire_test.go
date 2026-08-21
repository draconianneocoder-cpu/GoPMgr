// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"gopmgr/internal/db"
)

func TestProjectMetaWireBudgetEditsAreExactAndCanClear(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Budget wire")

	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	for _, want := range []string{"90071992547409.91", "92233720368547758.07", "0.00"} {
		meta.Budget = want
		meta, err = app.UpdateProjectMeta(meta)
		if err != nil {
			t.Fatalf("UpdateProjectMeta(%q): %v", want, err)
		}
		if meta.Budget != want {
			t.Fatalf("saved budget = %q, want %q", meta.Budget, want)
		}
	}
	stored, err := app.requireDB().GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if stored.BudgetMinorUnits != 0 {
		t.Fatalf("cleared budget minor units = %d, want 0", stored.BudgetMinorUnits)
	}
}

func TestProjectMetaWireRejectsInvalidBudgetWithoutMutation(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Budget validation")
	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	meta.Budget = "100.00"
	if _, err := app.UpdateProjectMeta(meta); err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	before, err := app.requireDB().GetProject()
	if err != nil {
		t.Fatalf("GetProject before invalid update: %v", err)
	}
	meta.Budget = "92233720368547758.08"
	if _, err := app.UpdateProjectMeta(meta); err == nil {
		t.Fatal("UpdateProjectMeta accepted overflowing budget")
	}
	after, err := app.requireDB().GetProject()
	if err != nil {
		t.Fatalf("GetProject after invalid update: %v", err)
	}
	if after.BudgetMinorUnits != before.BudgetMinorUnits || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("invalid budget mutated project: before=%+v after=%+v", before, after)
	}
}

func TestBudgetWiresUseDecimalStrings(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Budget summary wire")
	meta, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	meta.Budget = "92233720368547758.07"
	if _, err := app.UpdateProjectMeta(meta); err != nil {
		t.Fatalf("UpdateProjectMeta: %v", err)
	}
	if _, err := app.SaveStakeholder(db.Stakeholder{Name: "Vendor", Category: db.StakeholderVendor, ContractValueMinorUnits: math.MaxInt64}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	summary, err := app.ComputeBudget()
	if err != nil {
		t.Fatalf("ComputeBudget: %v", err)
	}
	if summary.Budget != "92233720368547758.07" || summary.Committed != "92233720368547758.07" || summary.Remaining != "0.00" || summary.ByCategory[string(db.StakeholderVendor)] != "92233720368547758.07" {
		t.Fatalf("unexpected exact summary: %+v", summary)
	}
	for _, wire := range []any{meta, summary} {
		data, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("marshal wire: %v", err)
		}
		text := string(data)
		if strings.Contains(text, "minor_units") || strings.Contains(text, `"budget":9`) || strings.Contains(text, `"committed":9`) {
			t.Fatalf("wire exposed numeric money: %s", text)
		}
	}
}
