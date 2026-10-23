// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"gopmgr/internal/documents"
)

// DOCX and ODT documents label amounts in the project's reporting
// currency, in plain fields and in object-array tables, with thousands
// separators.
func TestRenderDocument_LabelsAmountsInTheProjectCurrency(t *testing.T) {
	charter := `{"high_level_budget":1284500}`
	plan := `{"tasks":[{"name":"Build","cost":1284500}]}`
	cases := []struct {
		name, part string
		render     func(documents.Kind, string, string, string) ([]byte, error)
	}{
		{"DOCX", "word/document.xml", RenderDocumentDOCX},
		{"ODT", "content.xml", RenderDocumentODT},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := c.render(documents.KindProjectCharterWord, charter, "Atlas", "EUR")
			if err != nil {
				t.Fatalf("render charter: %v", err)
			}
			body := zipPart(t, out, c.part)
			for _, want := range []string{"High-Level Budget (EUR)", "1,284,500.00"} {
				if !strings.Contains(body, want) {
					t.Errorf("charter %s lacks %q", c.part, want)
				}
			}
			if strings.Contains(body, "USD") {
				t.Errorf("a EUR project's charter %s still says USD", c.part)
			}

			out, err = c.render(documents.KindExecutionPlan, plan, "Atlas", "GBP")
			if err != nil {
				t.Fatalf("render execution plan: %v", err)
			}
			body = zipPart(t, out, c.part)
			for _, want := range []string{"Cost (GBP)", "1,284,500.00"} {
				if !strings.Contains(body, want) {
					t.Errorf("execution plan %s lacks %q", c.part, want)
				}
			}

			out, err = c.render(documents.KindProjectCharterWord, charter, "Atlas", "JPY")
			if err != nil {
				t.Fatalf("render yen charter: %v", err)
			}
			body = zipPart(t, out, c.part)
			if !strings.Contains(body, "High-Level Budget (JPY)") || !strings.Contains(body, "1,284,500") || strings.Contains(body, "1,284,500.00") {
				t.Errorf("yen charter %s should print High-Level Budget (JPY) 1,284,500 with no decimals", c.part)
			}

			if _, err := c.render(documents.KindProjectCharterWord, charter, "Atlas", "eur"); err == nil {
				t.Errorf("render accepted a malformed currency")
			}
		})
	}
}
