// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

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
