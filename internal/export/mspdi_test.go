// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"strings"
	"testing"

	"gopmgr/internal/kernel"
)

func TestToMSPDIUsesAnchoredDates(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {
			ID: "A", Title: "Design", Duration: 2,
			ES: 0, EF: 2,
			StartDate: "2026-06-05", FinishDate: "2026-06-08",
		},
	}

	out, err := ToMSPDI("Demo", tasks, nil)
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

	out, err := ToMSPDI("Demo", tasks, nil)
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

	first, err := ToMSPDI("Demo", mk(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		again, err := ToMSPDI("Demo", mk(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatal("ToMSPDI output is not deterministic across runs")
		}
	}

	// ES order: A before B and C; tie (B, C) broken by ID.
	s := string(first)
	if strings.Index(s, "<Name>A</Name>") >= strings.Index(s, "<Name>B</Name>") ||
		strings.Index(s, "<Name>B</Name>") >= strings.Index(s, "<Name>C</Name>") {
		t.Errorf("tasks not in (ES, ID) order:\n%s", s)
	}
}

// Levelling delays are written in Microsoft's form (owner decision
// 2026-11-23): whole elapsed days as format 8, anything else in elapsed
// minutes as format 4, between ConstraintDate and PredecessorLink. Part-day
// starts and finishes fall at their working minute on the 08:00-12:00,
// 13:00-17:00 day; a part-day milestone finishes when it starts.
func TestToMSPDIWritesLevelingDelaysAndWorkingTimes(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Duration: 1, ES: 2, EF: 3, StartDate: "2026-06-03", FinishDate: "2026-06-03"},
		"B": {ID: "B", Duration: 0.5, ES: 0.5, EF: 1, StartDate: "2026-06-01", FinishDate: "2026-06-01",
			Constraint: kernel.StartNoEarlierThan, ConstraintDate: "2026-06-01",
			Links: []kernel.Link{{Pred: "D", Type: kernel.FinishToStart}}},
		"D": {ID: "D", Duration: 0.5, ES: 0, EF: 0.5, StartDate: "2026-06-01", FinishDate: "2026-06-01"},
		"M": {ID: "M", Milestone: true, ES: 0.5, EF: 0.5, StartDate: "2026-06-01", FinishDate: "2026-06-01"},
	}
	out, err := ToMSPDI("Levelled", tasks, map[string]int{"A": 2880, "B": 300})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"<UID>A</UID><ID>A</ID><Name></Name><Start>2026-06-03T08:00:00</Start><Finish>2026-06-03T17:00:00</Finish>",
		"<UID>D</UID><ID>D</ID><Name></Name><Start>2026-06-01T08:00:00</Start><Finish>2026-06-01T12:00:00</Finish>",
		"<UID>B</UID><ID>B</ID><Name></Name><Start>2026-06-01T13:00:00</Start><Finish>2026-06-01T17:00:00</Finish>",
		"<UID>M</UID><ID>M</ID><Name></Name><Start>2026-06-01T13:00:00</Start><Finish>2026-06-01T13:00:00</Finish>",
		"<LevelingDelay>2</LevelingDelay><LevelingDelayFormat>8</LevelingDelayFormat>",
		"<ConstraintDate>2026-06-01T08:00:00</ConstraintDate><LevelingDelay>300</LevelingDelay><LevelingDelayFormat>4</LevelingDelayFormat><PredecessorLink>",
	} {
		if !strings.Contains(strings.Join(strings.Fields(s), ""), strings.Join(strings.Fields(want), "")) {
			t.Errorf("missing %s in:\n%s", want, s)
		}
	}
	if strings.Count(s, "<LevelingDelay>") != 2 {
		t.Errorf("want exactly two delays (A and B):\n%s", s)
	}
}
