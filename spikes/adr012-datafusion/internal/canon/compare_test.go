// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package canon

import (
	"errors"
	"testing"

	fx "gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name      string
		want, got fx.Cell
		class     Class
	}{
		{"text as string", fx.T("abc"), fx.S("abc"), Exact},
		{"text changed", fx.T("abc"), fx.S("abd"), Lossy},
		{"text integer typed", fx.T("42"), fx.I("42"), TypedExact},
		{"leading zeros dropped", fx.T("007"), fx.I("7"), TypedReformatted},
		{"text decimal to float", fx.T("10.50"), fx.F("10.5"), TypedReformatted},
		{"text fraction rounded", fx.T("0.1"), fx.F("0.1"), FloatRounded},
		{"text integer above 2^53 to float", fx.T("9007199254740993"), fx.F("9007199254740992"), Lossy},
		{"text 2^63 exact as float", fx.T("9223372036854775808"), fx.F("9.223372036854775808e18"), TypedReformatted},
		{"empty field as null", fx.T(""), fx.N(), EmptyAsNull},
		{"word NULL as null", fx.T("NULL"), fx.N(), WordAsNull},
		{"other text as null", fx.T("x"), fx.N(), Lossy},
		{"bool spelling", fx.T("FALSE"), fx.B(false), TypedReformatted},
		{"date typed", fx.T("2026-01-02"), fx.Tm("2026-01-02T00:00:00Z"), TypedReformatted},
		{"int exact", fx.I("5"), fx.I("5"), Exact},
		{"int as dec", fx.I("5"), fx.D("5.00"), ExactValue},
		{"int to float exact", fx.I("9223372036854775808"), fx.F("9.223372036854775808e18"), ExactValue},
		{"int to float changed", fx.I("9007199254740993"), fx.F("9007199254740992"), Lossy},
		{"dec to float rounded", fx.D("0.1"), fx.F("0.1"), FloatRounded},
		{"dec overflow to inf", fx.D("1e400"), fx.F("+Inf"), Lossy},
		{"dec as string", fx.D("1.5"), fx.S("1.5"), TypeChanged},
		{"NaN", fx.F("NaN"), fx.F("NaN"), Exact},
		{"negative zero lost", fx.F("-0"), fx.F("0"), SignOfZeroLost},
		{"time exact", fx.Tm("2026-01-02T03:04:05Z"), fx.Tm("2026-01-02T03:04:05Z"), Exact},
		{"naive time as UTC", fx.Tm("2026-01-02T03:04:05"), fx.Tm("2026-01-02T03:04:05Z"), ExactValue},
		{"time truncated", fx.Tm("2026-01-02T03:04:05.123456789Z"), fx.Tm("2026-01-02T03:04:05.123456Z"), PrecisionLost},
		{"date as midnight", fx.Dt("2026-01-02"), fx.Tm("2026-01-02T00:00:00Z"), ExactValue},
		{"json key order", fx.J(`{"b":1,"a":[2,3]}`), fx.J(`{"a":[2,3],"b":1}`), Exact},
		{"json number spelling", fx.J(`[2]`), fx.J(`[2.0]`), Exact},
		{"json shape changed", fx.J(`{}`), fx.J(`[]`), Lossy},
		{"bytes", fx.Hex("00ff"), fx.Hex("00ff"), Exact},
		{"null", fx.N(), fx.N(), Exact},
		{"null filled", fx.N(), fx.I("0"), Lossy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.want, tt.got); got != tt.class {
				t.Fatalf("Classify(%v, %v) = %s, want %s", tt.want, tt.got, got, tt.class)
			}
		})
	}
}

func oneRow(cs ...fx.Cell) [][]Cell {
	r := make([]Cell, len(cs))
	for i, c := range cs {
		r[i] = Cell{Cell: c}
	}
	return [][]Cell{r}
}

func TestCompareVerdicts(t *testing.T) {
	valid := fx.TabularCase{ID: "v", Accept: fx.AcceptRows, Columns: []string{"a"}, TotalRows: 1, Row: func(int) []fx.Cell { return []fx.Cell{fx.T("1")} }}
	bad := fx.TabularCase{ID: "b", Accept: fx.AcceptError}
	advisory := valid
	advisory.NamesAdvisory = true
	tests := []struct {
		name    string
		c       fx.TabularCase
		cols    []string
		rows    [][]Cell
		err     error
		verdict string
	}{
		{"exact text", valid, []string{"a"}, oneRow(fx.S("1")), nil, VMatch},
		{"typed", valid, []string{"a"}, oneRow(fx.I("1")), nil, VMatch},
		{"leading zero note", fx.TabularCase{ID: "z", Accept: fx.AcceptRows, Columns: []string{"a"}, TotalRows: 1, Row: func(int) []fx.Cell { return []fx.Cell{fx.T("01")} }},
			[]string{"a"}, oneRow(fx.I("1")), nil, VMatchWithNotes},
		{"renamed column", valid, []string{"b"}, oneRow(fx.S("1")), nil, VLoss},
		{"reordered columns", fx.TabularCase{ID: "r", Accept: fx.AcceptRows, Columns: []string{"a", "b"}, TotalRows: 1, Row: func(int) []fx.Cell { return []fx.Cell{fx.T("1"), fx.T("x")} }},
			[]string{"b", "a"}, oneRow(fx.S("x"), fx.S("1")), nil, VMatchWithNotes},
		{"reordered and changed", fx.TabularCase{ID: "r2", Accept: fx.AcceptRows, Columns: []string{"a", "b"}, TotalRows: 1, Row: func(int) []fx.Cell { return []fx.Cell{fx.T("1"), fx.T("x")} }},
			[]string{"b", "a"}, oneRow(fx.S("y"), fx.S("1")), nil, VLoss},
		{"advisory rename", advisory, []string{"a_1"}, oneRow(fx.S("1")), nil, VMatchWithNotes},
		{"missing row", valid, []string{"a"}, nil, nil, VLoss},
		{"valid rejected", valid, nil, nil, errors.New("x"), VRejectedValid},
		{"bad rejected", bad, nil, nil, errors.New("x"), VRejected},
		{"bad accepted", bad, []string{"a"}, oneRow(fx.S("1")), nil, VSilentAccept},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.c, tt.cols, nil, tt.rows, tt.err).Verdict; got != tt.verdict {
				t.Fatalf("verdict %s, want %s", got, tt.verdict)
			}
		})
	}
}
