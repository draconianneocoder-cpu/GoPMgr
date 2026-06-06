// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

func TestFormatMoneyUsesThousandsSeparators(t *testing.T) {
	tests := map[float64]string{
		0:          "0.00",
		999.99:     "999.99",
		1234.5:     "1,234.50",
		1234567.5:  "1,234,567.50",
		-1234567.5: "-1,234,567.50",
	}

	for input, want := range tests {
		if got := formatMoney(input); got != want {
			t.Fatalf("formatMoney(%v) = %q, want %q", input, got, want)
		}
	}
}
