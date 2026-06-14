// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"testing"

	"pmforge/internal/kernel"
)

const sampleMSPDI = `<?xml version="1.0" encoding="UTF-8"?>
<Project xmlns="http://schemas.microsoft.com/project">
  <Title>Office Move</Title>
  <StartDate>2026-06-01T08:00:00</StartDate>
  <Tasks>
    <Task><UID>0</UID><Name>Summary</Name><Summary>1</Summary><Duration>PT40H0M0S</Duration></Task>
    <Task><UID>1</UID><Name>Pack</Name><Duration>PT16H0M0S</Duration><PercentComplete>50</PercentComplete></Task>
    <Task>
      <UID>2</UID><Name>Move</Name><Duration>PT8H0M0S</Duration>
      <PredecessorLink><PredecessorUID>1</PredecessorUID><Type>1</Type><LinkLag>4800</LinkLag></PredecessorLink>
    </Task>
    <Task>
      <UID>3</UID><Name>Unpack</Name><Duration>PT8H30M0S</Duration>
      <PredecessorLink><PredecessorUID>2</PredecessorUID><Type>3</Type></PredecessorLink>
      <PredecessorLink><PredecessorUID>0</PredecessorUID></PredecessorLink>
    </Task>
    <Task><UID>4</UID><Name>Done</Name><Duration>PT0H0M0S</Duration><Milestone>1</Milestone></Task>
  </Tasks>
  <Resources>
    <Resource><UID>10</UID><Name>alice</Name></Resource>
  </Resources>
  <Assignments>
    <Assignment><TaskUID>1</TaskUID><ResourceUID>10</ResourceUID><Units>0.5</Units></Assignment>
  </Assignments>
</Project>`

func TestFromMSPDI(t *testing.T) {
	p, err := FromMSPDI([]byte(sampleMSPDI))
	if err != nil {
		t.Fatalf("FromMSPDI: %v", err)
	}

	if p.Title != "Office Move" || p.StartDate != "2026-06-01" {
		t.Errorf("header = %q / %q", p.Title, p.StartDate)
	}
	// Summary task skipped: 4 importable tasks.
	if len(p.Tasks) != 4 {
		t.Fatalf("tasks = %d, want 4", len(p.Tasks))
	}

	byUID := map[string]ImportedTask{}
	for _, task := range p.Tasks {
		byUID[task.UID] = task
	}

	if d := byUID["1"].DurationDays; d != 2 {
		t.Errorf("Pack duration = %v days, want 2", d)
	}
	if pc := byUID["1"].PercentComplete; pc != 50 {
		t.Errorf("Pack percent = %v, want 50", pc)
	}
	if a := byUID["1"].Assignments; len(a) != 1 || a[0].Resource != "alice" || a[0].Units != 0.5 {
		t.Errorf("Pack assignments = %+v", a)
	}

	move := byUID["2"]
	if len(move.Links) != 1 || move.Links[0].Type != kernel.FinishToStart || move.Links[0].Lag != 1 {
		t.Errorf("Move link = %+v, want FS lag 1 day (4800 tenths of a minute)", move.Links)
	}

	unpack := byUID["3"]
	// Link to the skipped summary task (UID 0) must be dropped.
	if len(unpack.Links) != 1 || unpack.Links[0].Type != kernel.StartToStart {
		t.Errorf("Unpack links = %+v, want single SS link", unpack.Links)
	}
	if d := unpack.DurationDays; d < 1.06 || d > 1.07 {
		t.Errorf("Unpack duration = %v days, want ~1.0625 (8.5h)", d)
	}

	if !byUID["4"].Milestone || byUID["4"].DurationDays != 0 {
		t.Errorf("Done = %+v, want zero-duration milestone", byUID["4"])
	}
}

func TestFromMSPDIEmptyErrors(t *testing.T) {
	if _, err := FromMSPDI([]byte(`<Project xmlns="x"><Tasks></Tasks></Project>`)); err == nil {
		t.Error("no importable tasks must error")
	}
	if _, err := FromMSPDI([]byte(`not xml`)); err == nil {
		t.Error("malformed XML must error")
	}
}

func TestMSPDIRoundTrip(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 2, PercentComplete: 25,
			Assignments: []kernel.Assignment{{Resource: "alice", Units: 0.5}}},
		"B": {ID: "B", Title: "Build", Duration: 3,
			Links: []kernel.Link{{Pred: "A", Type: kernel.StartToStart, Lag: 1}}},
		"M": {ID: "M", Title: "Ship", Duration: 0, Milestone: true,
			Precedents: []string{"B"}},
	}
	if !kernel.CalculateCPM(tasks) {
		t.Fatal("CalculateCPM cycle")
	}

	xmlBytes, err := ToMSPDI("Round Trip", tasks)
	if err != nil {
		t.Fatalf("ToMSPDI: %v", err)
	}
	back, err := FromMSPDI(xmlBytes)
	if err != nil {
		t.Fatalf("FromMSPDI(ToMSPDI(...)): %v", err)
	}

	if back.Title != "Round Trip" || len(back.Tasks) != 3 {
		t.Fatalf("round trip lost tasks: %+v", back)
	}
	byUID := map[string]ImportedTask{}
	for _, task := range back.Tasks {
		byUID[task.UID] = task
	}

	if byUID["A"].DurationDays != 2 || byUID["A"].PercentComplete != 25 {
		t.Errorf("A = %+v", byUID["A"])
	}
	if a := byUID["A"].Assignments; len(a) != 1 || a[0].Resource != "alice" || a[0].Units != 0.5 {
		t.Errorf("A assignments = %+v", a)
	}
	if l := byUID["B"].Links; len(l) != 1 || l[0].Pred != "A" ||
		l[0].Type != kernel.StartToStart || l[0].Lag != 1 {
		t.Errorf("B links = %+v, want SS+1 from A", l)
	}
	if l := byUID["M"].Links; len(l) != 1 || l[0].Type != kernel.FinishToStart {
		t.Errorf("M links = %+v, want FS from B", l)
	}
	if !byUID["M"].Milestone {
		t.Error("milestone flag lost in round trip")
	}
}
