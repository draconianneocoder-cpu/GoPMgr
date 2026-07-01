// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package exportsafe

import "testing"

func TestCellNeutralizesFormulaTriggers(t *testing.T) {
	dangerous := []string{
		"=1+1",
		"+1+1",
		"-1+1",
		"@SUM(A1)",
		"=cmd|'/c calc'!A1",
		"\t=1+1",
		"\r=1+1",
		"\n=1+1",
	}
	for _, in := range dangerous {
		got := Cell(in)
		if got == in {
			t.Errorf("Cell(%q) = %q, want a leading-quote escape", in, got)
		}
		if got[0] != '\'' {
			t.Errorf("Cell(%q) = %q, want it to start with a single quote", in, got)
		}
	}
}

func TestCellLeavesSafeValuesUnchanged(t *testing.T) {
	safe := []string{
		"",
		"Project Alpha",
		"Q3 Roadmap",
		"123 Main St",
		"already 'quoted",
		"a=b later", // '=' not in leading position is harmless
	}
	for _, in := range safe {
		if got := Cell(in); got != in {
			t.Errorf("Cell(%q) = %q, want unchanged", in, got)
		}
	}
}
