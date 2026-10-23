// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

func TestFormatMoneyUsesThousandsSeparators(t *testing.T) {
	tests := map[float64]string{
		0:          "0.00",
		999.99:     "999.99",
		1234.5:     "1,234.50",
		1234567.5:  "1,234,567.50",
		1284500:    "1,284,500.00",
		-1234567.5: "-1,234,567.50",
		-123.45:    "-123.45",
		-123456.78: "-123,456.78",
		-0.001:     "0.00",
	}

	for input, want := range tests {
		if got := formatMoney(input); got != want {
			t.Fatalf("formatMoney(%v) = %q, want %q", input, got, want)
		}
	}
}
