// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package exportsafe neutralizes user-controlled values written into
// delimited text exports (CSV/TSV) so spreadsheet applications (Excel,
// LibreOffice Calc, Numbers) cannot interpret them as formulas when the
// file is opened — CWE-1236, "CSV / formula injection".
//
// It is intentionally a tiny leaf package: both internal/export and
// internal/db emit CSV and need the same rule, and internal/db cannot
// import internal/export (that would be an import cycle).
//
// Note: this is only needed for delimited *text* formats. XLSX written via
// excelize stores Go strings as string-typed cells (`<c t="str">`), which
// Excel never evaluates as formulas — only an explicit SetCellFormula does —
// so XLSX cells must NOT be passed through Cell (it would corrupt legitimate
// values like "-5%" with a spurious quote for no security benefit).
package exportsafe

// Cell returns s unchanged unless it begins with a byte a spreadsheet would
// treat as the start of a formula, in which case it prepends a single quote
// to force the value to be read as text. The leading quote is the standard
// spreadsheet text-escape and is hidden by Excel on display.
//
// The trigger set is the four formula-introducer characters plus the
// leading whitespace bytes (tab/CR/LF) that some parsers strip before
// re-checking, which would otherwise smuggle a formula past a naive filter.
func Cell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return "'" + s
	}
	return s
}
