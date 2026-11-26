// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"testing"

	"gopmgr/internal/charts/matrix"
)

func TestRiskRegisterExposesRiskMatrixChartReference(t *testing.T) {
	def, ok := Get(KindRiskRegister)
	if !ok {
		t.Fatal("Risk Register definition is missing")
	}
	for _, field := range def.Fields {
		if field.Key == "risk_matrix_ref" {
			if field.Type != FieldChartRef || field.ChartKind != "risk_matrix" {
				t.Fatalf("risk_matrix_ref = %+v, want risk_matrix chart reference", field)
			}
			return
		}
	}
	t.Fatal("Risk Register has no risk_matrix_ref field")
}

// The register's choices and bounds must match what the Risk Matrix
// accepts when the register is synchronized to it: the editor offers only
// these, so a type the matrix rejects, or a bound wider than its 1–5
// scale, would let users enter rows that fail the sync. The oracle is the
// matrix's own validation (matrix.LayoutRiskMatrix), not the definition.
func TestRiskRegisterChoicesMatchRiskMatrixValidation(t *testing.T) {
	def, ok := Get(KindRiskRegister)
	if !ok {
		t.Fatal("risk register definition missing")
	}
	fields := map[string]Field{}
	for _, f := range def.Fields {
		if f.Key == "risks" {
			for _, sub := range f.ObjectShape {
				fields[sub.Key] = sub
			}
		}
	}
	errorsFor := func(item matrix.RiskItem) int {
		item.ID, item.Title = "r1", "Supplier delay"
		return matrix.LayoutRiskMatrix(matrix.RiskMatrixDocument{Items: []matrix.RiskItem{item}}).Validation.ErrorCount
	}

	kind := fields["kind"]
	if len(kind.Options) == 0 {
		t.Fatal("kind offers no options")
	}
	for _, option := range kind.Options {
		if n := errorsFor(matrix.RiskItem{Kind: option, Probability: 3, Impact: 3}); n != 0 {
			t.Errorf("kind option %q is rejected by the Risk Matrix", option)
		}
	}
	for _, key := range []string{"probability", "impact"} {
		f := fields[key]
		if f.Min == nil || f.Max == nil {
			t.Fatalf("%s has no bounds", key)
		}
		at := func(v float64) matrix.RiskItem {
			item := matrix.RiskItem{Kind: "risk", Probability: 3, Impact: 3}
			if key == "probability" {
				item.Probability = int(v)
			} else {
				item.Impact = int(v)
			}
			return item
		}
		if errorsFor(at(*f.Min)) != 0 || errorsFor(at(*f.Max)) != 0 {
			t.Errorf("%s bounds %v..%v are not accepted by the Risk Matrix", key, *f.Min, *f.Max)
		}
		if errorsFor(at(*f.Min-1)) == 0 || errorsFor(at(*f.Max+1)) == 0 {
			t.Errorf("%s bounds %v..%v are narrower than the Risk Matrix scale", key, *f.Min, *f.Max)
		}
	}
}
