// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"
)

// projectCurrencies are the reporting currencies a new project can use
// (db.supportedProjectCurrencies).
var projectCurrencies = []string{"AUD", "CAD", "CHF", "EUR", "GBP", "JPY", "USD"}

var pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)

// streamText returns every stream in pdf, inflated when compressed. This
// package's tests draw in the core fonts (see main_test.go), which write
// text as literal strings, so a label is searchable in the result.
func streamText(t *testing.T, pdf []byte) string {
	t.Helper()
	var out strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(pdf, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			out.Write(m[1])
			continue
		}
		inflated, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			out.Write(m[1])
			continue
		}
		out.Write(inflated)
	}
	return out.String()
}

// Every document that prints an amount names the project's reporting
// currency, and no longer says USD for a project in another currency.
func TestRender_LabelsAmountsInTheProjectCurrency(t *testing.T) {
	const amount = 1284500.0
	task := map[string]interface{}{"name": "Build", "start_date": "2026-01-05", "end_date": "2026-01-20", "cost": amount}
	docs := []struct {
		kind    Kind
		content map[string]interface{}
	}{
		{KindProjectCharterWord, map[string]interface{}{"project_name": "Atlas", "high_level_budget": amount}},
		{KindBusinessCase, map[string]interface{}{"costs_summary": amount}},
		{KindProjectProposal, map[string]interface{}{"budget_summary": amount}},
		{KindExecutionPlan, map[string]interface{}{"tasks": []interface{}{task}}},
		{KindProcurementPlan, map[string]interface{}{"items": []interface{}{map[string]interface{}{"item": "Servers", "budget": amount}}}},
		{KindProjectBrief, map[string]interface{}{"summary": "A short brief.", "budget": amount}},
		{KindChangeRequest, map[string]interface{}{"description": "Add a site.", "cost_impact": amount}},
		{KindProjectClosure, map[string]interface{}{"final_budget_actual": amount}},
		{KindProjectBudget, map[string]interface{}{"categories": []interface{}{map[string]interface{}{"category": "Labor", "amount": amount}}}},
	}
	for _, code := range projectCurrencies {
		for _, d := range docs {
			t.Run(code+"/"+string(d.kind), func(t *testing.T) {
				body, err := json.Marshal(d.content)
				if err != nil {
					t.Fatal(err)
				}
				pdf, err := Render(d.kind, string(body), "Atlas", code)
				if err != nil {
					t.Fatalf("Render: %v", err)
				}
				text := streamText(t, pdf)
				if !strings.Contains(text, code) {
					t.Errorf("the PDF never names %s", code)
				}
				if code != "USD" && strings.Contains(text, "USD") {
					t.Errorf("a %s project's PDF still says USD", code)
				}
				want := "1,284,500.00"
				if code == "JPY" {
					// Yen prints as a whole number (ADR-009).
					want = "1,284,500"
					if strings.Contains(text, "1,284,500.00") {
						t.Errorf("a yen amount printed decimals")
					}
				}
				if !strings.Contains(text, want) {
					t.Errorf("the amount %s is missing or lacks thousands separators", want)
				}
			})
		}
	}
}

// A budget document's own Currency field still wins over the project's.
func TestRender_ProjectBudgetKeepsItsOwnCurrency(t *testing.T) {
	pdf, err := Render(KindProjectBudget, `{"currency":"GBP","categories":[{"category":"Labor","amount":10}]}`, "Atlas", "EUR")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if text := streamText(t, pdf); !strings.Contains(text, "GBP") || strings.Contains(text, "EUR") {
		t.Errorf("budget currency: want GBP from the document, not the project's EUR")
	}
}

func TestRender_RejectsMalformedCurrency(t *testing.T) {
	for _, code := range []string{"", "usd", "EURO", "E1R", "€"} {
		if _, err := Render(KindProjectCharterWord, `{}`, "Atlas", code); err == nil || !strings.Contains(err.Error(), "reporting currency") {
			t.Errorf("Render(currency %q) error = %v, want a reporting currency error", code, err)
		}
	}
	if _, err := Render(KindProjectCharterWord, `null`, "Atlas", "EUR"); err != nil {
		t.Errorf("Render(null content): %v", err)
	}
}

// Labels name no currency, so one project's code is never printed in
// another's documents; LabelIn adds it. No field may use the key Render
// reserves for the reporting currency.
func TestTemplates_LabelsNameNoCurrency(t *testing.T) {
	code := regexp.MustCompile(`\b(USD|EUR|GBP|CAD|AUD|CHF|JPY)\b`)
	var check func(kind Kind, fields []Field)
	check = func(kind Kind, fields []Field) {
		for _, f := range fields {
			if code.MatchString(f.Label) {
				t.Errorf("%s field %q label %q names a currency; mark it Money instead", kind, f.Key, f.Label)
			}
			if f.Key == reportingCurrencyKey {
				t.Errorf("%s field uses the reserved key %q", kind, reportingCurrencyKey)
			}
			check(kind, f.ObjectShape)
		}
	}
	for _, def := range All() {
		check(def.Kind, def.Fields)
	}
}

func TestField_LabelIn(t *testing.T) {
	cases := []struct {
		field    Field
		currency string
		want     string
	}{
		{Field{Label: "High-Level Budget", Money: true}, "EUR", "High-Level Budget (EUR)"},
		{Field{Label: "High-Level Budget", Money: true}, "", "High-Level Budget"},
		{Field{Label: "Probability (1-5)"}, "EUR", "Probability (1-5)"},
	}
	for _, c := range cases {
		if got := c.field.LabelIn(c.currency); got != c.want {
			t.Errorf("LabelIn(%q) = %q, want %q", c.currency, got, c.want)
		}
	}
}

// Combined reports walk each section's fields generically; amounts take
// the report's currency.
func TestBuildCombinedReport_LabelsAmountsInTheReportCurrency(t *testing.T) {
	pdf, err := BuildCombinedReport(ReportSpec{
		ReportTitle: "Quarterly report",
		ProjectName: "Atlas",
		Currency:    "CHF",
		Sections:    []ReportSection{{DocumentID: "doc-1", Title: "Charter"}},
	}, []ResolvedSection{{
		Section: ReportSection{DocumentID: "doc-1", Title: "Charter"},
		Kind:    KindProjectCharterWord,
		Content: `{"high_level_budget":1284500}`,
		Version: 1,
		Status:  "approved",
	}})
	if err != nil {
		t.Fatalf("BuildCombinedReport: %v", err)
	}
	text := streamText(t, pdf)
	if !strings.Contains(text, "High-Level Budget \\(CHF\\)") || !strings.Contains(text, "1,284,500.00") {
		t.Errorf("report section should print High-Level Budget (CHF) 1,284,500.00")
	}
}
