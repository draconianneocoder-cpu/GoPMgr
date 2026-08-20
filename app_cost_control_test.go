// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseMoneyDecimalRejectsNonCanonicalOrUnsafeInput(t *testing.T) {
	for _, input := range []string{"", "1.234", "1e2", "NaN", " 1.00", "9,000.00", "92233720368547758.08"} {
		if _, err := parseMoneyDecimal(input); err == nil {
			t.Errorf("parseMoneyDecimal(%q) accepted invalid input", input)
		}
	}
	got, err := parseMoneyDecimal("90071992547409.91")
	if err != nil || got.MinorUnits != 9007199254740991 {
		t.Fatalf("large exact amount = %+v, %v", got, err)
	}
	if formatted := formatMoneyDecimal(got); formatted != "90071992547409.91" {
		t.Fatalf("formatted = %q", formatted)
	}
}

func TestCostControlWireUsesSnakeCaseStringMoney(t *testing.T) {
	body, err := json.Marshal(CostSummaryWire{CurrencyCode: "USD", CostBaseline: "90071992547409.91"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	if !strings.Contains(encoded, `"currency_code":"USD"`) || !strings.Contains(encoded, `"cost_baseline":"90071992547409.91"`) {
		t.Fatalf("wire JSON = %s", encoded)
	}
	if strings.Contains(encoded, "CurrencyCode") || strings.Contains(encoded, "CostBaseline") {
		t.Fatalf("wire JSON exposes Go field names: %s", encoded)
	}
}
