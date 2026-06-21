// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"strings"
	"testing"

	"pmforge/internal/kernel"
)

func TestToMSPDIUsesAnchoredDates(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {
			ID: "A", Title: "Design", Duration: 2,
			ES: 0, EF: 2,
			StartDate: "2026-06-05", FinishDate: "2026-06-08",
		},
	}

	out, err := ToMSPDI("Demo", tasks)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	if !strings.Contains(s, "<Start>2026-06-05T08:00:00</Start>") {
		t.Errorf("anchored Start not emitted:\n%s", s)
	}
	if !strings.Contains(s, "<Finish>2026-06-08T17:00:00</Finish>") {
		t.Errorf("anchored Finish not emitted:\n%s", s)
	}
	if !strings.Contains(s, "PT16H0M0S") {
		t.Errorf("duration PT16H0M0S not emitted:\n%s", s)
	}
}

func TestToMSPDIFallsBackWithoutAnchor(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 2, ES: 0, EF: 2},
	}

	out, err := ToMSPDI("Demo", tasks)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy path still emits a Start element (anchored at "today");
	// just assert structure, not the moving date.
	if !strings.Contains(string(out), "<Start>") {
		t.Errorf("fallback Start missing:\n%s", out)
	}
}

func TestToMSPDIDeterministicOrder(t *testing.T) {
	mk := func() map[string]*kernel.Task {
		return map[string]*kernel.Task{
			"B": {ID: "B", Title: "B", Duration: 1, ES: 2, EF: 3},
			"A": {ID: "A", Title: "A", Duration: 2, ES: 0, EF: 2},
			"C": {ID: "C", Title: "C", Duration: 1, ES: 2, EF: 3},
		}
	}

	first, err := ToMSPDI("Demo", mk())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		again, err := ToMSPDI("Demo", mk())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatal("ToMSPDI output is not deterministic across runs")
		}
	}

	// ES order: A before B and C; tie (B, C) broken by ID.
	s := string(first)
	if !(strings.Index(s, "<Name>A</Name>") < strings.Index(s, "<Name>B</Name>") &&
		strings.Index(s, "<Name>B</Name>") < strings.Index(s, "<Name>C</Name>")) {
		t.Errorf("tasks not in (ES, ID) order:\n%s", s)
	}
}
