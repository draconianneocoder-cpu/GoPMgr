// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"gopmgr/internal/kernel"
)

// Every link type and a lead (negative lag) survive export and import
// unchanged. A lead used to round toward zero on export (-0.5 day became
// -2399 tenths of a minute, reading back as -0.4998 day).
func TestMSPDIRoundTripKeepsEveryLinkTypeAndLead(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Survey", Duration: 2,
			Assignments: []kernel.Assignment{{Resource: "alice", Units: 0}, {Resource: "", Units: 1}}},
		"B": {ID: "B", Title: "Design", Duration: 3, Links: []kernel.Link{{Pred: "A", Type: kernel.FinishToFinish}}},
		"C": {ID: "C", Title: "Permit", Duration: 1, Links: []kernel.Link{{Pred: "A", Type: kernel.StartToFinish}}},
		"D": {ID: "D", Title: "Order", Duration: 1, Links: []kernel.Link{{Pred: "B", Type: kernel.StartToStart, Lag: 1.5}}},
		"E": {ID: "E", Title: "Build", Duration: 4, Links: []kernel.Link{{Pred: "D", Type: kernel.FinishToStart, Lag: -0.5}}},
	}
	raw, err := ToMSPDI("Links", tasks)
	if err != nil {
		t.Fatalf("ToMSPDI: %v", err)
	}
	xmlText := string(raw)
	for _, want := range []string{"<Type>0</Type>", "<Type>2</Type>", "<Type>3</Type>", "<LinkLag>7200</LinkLag>", "<LinkLag>-2400</LinkLag>"} {
		if !strings.Contains(xmlText, want) {
			t.Errorf("MSPDI is missing %s", want)
		}
	}
	// An assignment with no units is full time; one with no resource is left out.
	if !strings.Contains(xmlText, "<Units>1</Units>") || strings.Count(xmlText, "<Assignment>") != 1 || strings.Count(xmlText, "<Resource>") != 1 {
		t.Errorf("assignments not exported as one full-time assignment for alice:\n%s", xmlText)
	}

	back, err := fromMSPDI(raw)
	if err != nil {
		t.Fatalf("fromMSPDI: %v", err)
	}
	want := map[string]kernel.Link{
		"B": {Pred: "A", Type: kernel.FinishToFinish},
		"C": {Pred: "A", Type: kernel.StartToFinish},
		"D": {Pred: "B", Type: kernel.StartToStart, Lag: 1.5},
		"E": {Pred: "D", Type: kernel.FinishToStart, Lag: -0.5},
	}
	for _, task := range back.Tasks {
		w, ok := want[task.UID]
		if !ok {
			continue
		}
		if len(task.Links) != 1 || task.Links[0] != w {
			t.Errorf("task %s links = %+v, want [%+v]", task.UID, task.Links, w)
		}
	}
}

// Rows GoPMgr cannot import are skipped and counted, and links or
// assignments pointing at nothing are dropped.
func TestFromMSPDISkipsRowsItCannotImport(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
<Project xmlns="http://schemas.microsoft.com/project">
  <Title>Edge rows</Title>
  <Tasks>
    <Task><UID>1</UID><Name>Blank row</Name><IsNull>1</IsNull></Task>
    <Task><UID>2</UID><Name>Phase</Name><Summary>1</Summary></Task>
    <Task><Name>No UID</Name><Duration>PT8H0M0S</Duration></Task>
    <Task>
      <UID>3</UID><Name>Pour</Name><Duration>PT8H0M0S</Duration>
      <PredecessorLink><PredecessorUID></PredecessorUID></PredecessorLink>
      <PredecessorLink><PredecessorUID>2</PredecessorUID><Type>1</Type></PredecessorLink>
    </Task>
  </Tasks>
  <Resources>
    <Resource><UID>10</UID><Name>alice</Name></Resource>
    <Resource><UID>11</UID></Resource>
  </Resources>
  <Assignments>
    <Assignment><TaskUID>3</TaskUID><ResourceUID>11</ResourceUID><Units>1</Units></Assignment>
    <Assignment><TaskUID>3</TaskUID><ResourceUID>99</ResourceUID><Units>1</Units></Assignment>
    <Assignment><TaskUID>3</TaskUID><ResourceUID>10</ResourceUID><Units>0.5</Units></Assignment>
  </Assignments>
</Project>`
	p, err := fromMSPDI([]byte(doc))
	if err != nil {
		t.Fatalf("fromMSPDI: %v", err)
	}
	if p.Receipt.SkippedNullRows != 1 || p.Receipt.SkippedSummaryRows != 1 {
		t.Errorf("receipt = %+v, want one null row and one summary row skipped", p.Receipt)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].UID != "3" {
		t.Fatalf("tasks = %+v, want only Pour", p.Tasks)
	}
	pour := p.Tasks[0]
	if len(pour.Links) != 0 {
		t.Errorf("links = %+v, want the empty and summary-row links dropped", pour.Links)
	}
	if len(pour.Assignments) != 1 || pour.Assignments[0].Resource != "alice" || pour.Assignments[0].Units != 0.5 {
		t.Errorf("assignments = %+v, want only alice at 0.5", pour.Assignments)
	}
}

func TestISOHoursToDays(t *testing.T) {
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"PT8H0M0S", 1},
		{"pt8h30m0s", 1.0625},
		{"PT1.5H", 0.1875},
		{"PT0H30M0S", 0.0625},
		{"PT0H0M14400S", 0.5},
		{"", 0},
		{"PT", 0},
		{"P1D", 0},
		{"PT..H", 0},
	} {
		if got := isoHoursToDays(c.in); got != c.want {
			t.Errorf("isoHoursToDays(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMSPDINumberHelpers(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 1200: "1200", -42: "-42"} {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d) = %q, want %q", n, got, want)
		}
	}
	// A negative duration cannot be scheduled; it exports as zero.
	if got := isoDurationDays(-2); got != "PT0H0M0S" {
		t.Errorf("isoDurationDays(-2) = %q, want PT0H0M0S", got)
	}
	if got := isoDurationDays(1.25); got != "PT10H0M0S" {
		t.Errorf("isoDurationDays(1.25) = %q, want PT10H0M0S", got)
	}
}
