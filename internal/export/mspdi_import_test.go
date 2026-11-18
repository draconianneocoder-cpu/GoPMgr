// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"gopmgr/internal/kernel"
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

// fromMSPDI parses with every field selected, the app's default choice.
func fromMSPDI(data []byte) (ImportedProject, error) {
	return FromMSPDIWithOptions(data, MSPDIImportOptions{IncludeDependencies: true, IncludeProgress: true, IncludeAssignments: true, IncludeConstraints: true})
}

func TestFromMSPDI(t *testing.T) {
	p, err := fromMSPDI([]byte(sampleMSPDI))
	if err != nil {
		t.Fatalf("fromMSPDI: %v", err)
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
	if _, err := fromMSPDI([]byte(`<Project xmlns="x"><Tasks></Tasks></Project>`)); err == nil {
		t.Error("no importable tasks must error")
	}
	if _, err := fromMSPDI([]byte(`not xml`)); err == nil {
		t.Error("malformed XML must error")
	}
}

func TestFromMSPDIWithOptionsRecordsSelectedFields(t *testing.T) {
	p, err := FromMSPDIWithOptions([]byte(sampleMSPDI), MSPDIImportOptions{
		IncludeDependencies: false,
		IncludeProgress:     false,
		IncludeAssignments:  false,
	})
	if err != nil {
		t.Fatalf("FromMSPDIWithOptions: %v", err)
	}
	if len(p.Tasks) == 0 || len(p.Tasks[0].Links) != 0 || p.Tasks[0].PercentComplete != 0 || len(p.Tasks[0].Assignments) != 0 {
		t.Fatalf("field options were not applied: %+v", p.Tasks[0])
	}
	if len(p.Receipt.ExcludedFields) != 4 {
		t.Fatalf("receipt exclusions = %#v, want 4 selected exclusions", p.Receipt.ExcludedFields)
	}
	if p.Receipt.SkippedSummaryRows != 1 {
		t.Fatalf("summary receipt = %+v, want one skipped summary", p.Receipt)
	}
}

// Every MSPDI ConstraintType code maps to its GoPMgr type (Microsoft's
// published codes: 0 ASAP, 1 ALAP, 2 MSO, 3 MFO, 4 SNET, 5 SNLT, 6 FNET,
// 7 FNLT). ASAP is stored as no constraint. An unknown code, or a dated
// type with no usable date, is left out and counted; with the option off
// nothing is imported.
func TestFromMSPDIConstraints(t *testing.T) {
	task := func(uid, typ, date string) string {
		s := "<Task><UID>" + uid + "</UID><Name>T" + uid + "</Name><Duration>PT8H0M0S</Duration>"
		if typ != "" {
			s += "<ConstraintType>" + typ + "</ConstraintType>"
		}
		if date != "" {
			s += "<ConstraintDate>" + date + "</ConstraintDate>"
		}
		return s + "</Task>"
	}
	xml := `<Project xmlns="http://schemas.microsoft.com/project"><Tasks>` +
		task("1", "0", "2026-06-04T08:00:00") + task("2", "1", "") +
		task("3", "2", "2026-06-04T08:00:00") + task("4", "3", "2026-06-05T17:00:00") +
		task("5", "4", "2026-06-08T08:00:00") + task("6", "5", "2026-06-09T08:00:00") +
		task("7", "6", "2026-06-10T17:00:00") + task("8", "7", "2026-06-11T17:00:00") +
		task("9", "2", "") + task("10", "9", "2026-06-04T08:00:00") + task("11", "5", "NA") +
		task("12", "", "") +
		`</Tasks></Project>`

	p, err := fromMSPDI([]byte(xml))
	if err != nil {
		t.Fatalf("fromMSPDI: %v", err)
	}
	type got struct {
		c    kernel.ConstraintType
		date string
	}
	want := map[string]got{
		"1": {}, "2": {kernel.AsLateAsPossible, ""},
		"3": {kernel.MustStartOn, "2026-06-04"}, "4": {kernel.MustFinishOn, "2026-06-05"},
		"5": {kernel.StartNoEarlierThan, "2026-06-08"}, "6": {kernel.StartNoLaterThan, "2026-06-09"},
		"7": {kernel.FinishNoEarlierThan, "2026-06-10"}, "8": {kernel.FinishNoLaterThan, "2026-06-11"},
		"9": {}, "10": {}, "11": {}, "12": {},
	}
	for _, task := range p.Tasks {
		if g := (got{task.Constraint, task.ConstraintDate}); g != want[task.UID] {
			t.Errorf("task %s: %+v, want %+v", task.UID, g, want[task.UID])
		}
	}
	if p.Receipt.SkippedConstraints != 3 {
		t.Errorf("skipped constraints = %d, want 3 (tasks 9, 10, 11)", p.Receipt.SkippedConstraints)
	}

	off, err := FromMSPDIWithOptions([]byte(xml), MSPDIImportOptions{})
	if err != nil {
		t.Fatalf("FromMSPDIWithOptions (no constraints): %v", err)
	}
	for _, task := range off.Tasks {
		if task.Constraint != "" || task.ConstraintDate != "" {
			t.Errorf("constraints off: task %s has %s %s", task.UID, task.Constraint, task.ConstraintDate)
		}
	}
	if off.Receipt.SkippedConstraints != 0 {
		t.Errorf("constraints off: skipped = %d, want 0", off.Receipt.SkippedConstraints)
	}
}

func TestMSPDIRoundTrip(t *testing.T) {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 2, PercentComplete: 25,
			Assignments: []kernel.Assignment{{Resource: "alice", Units: 0.5}},
			Constraint:  kernel.MustStartOn, ConstraintDate: "2026-06-04"},
		"B": {ID: "B", Title: "Build", Duration: 3,
			Links:      []kernel.Link{{Pred: "A", Type: kernel.StartToStart, Lag: 1}},
			Constraint: kernel.FinishNoLaterThan, ConstraintDate: "2026-06-10"},
		"M": {ID: "M", Title: "Ship", Duration: 0, Milestone: true,
			Precedents: []string{"B"}, Constraint: kernel.AsLateAsPossible},
		// A dated constraint without a valid date is not written.
		"X": {ID: "X", Title: "Loose", Duration: 1, Constraint: kernel.StartNoEarlierThan, ConstraintDate: "soon"},
	}
	if !kernel.CalculateCPM(tasks) {
		t.Fatal("CalculateCPM cycle")
	}

	xmlBytes, err := ToMSPDI("Round Trip", tasks)
	if err != nil {
		t.Fatalf("ToMSPDI: %v", err)
	}
	back, err := fromMSPDI(xmlBytes)
	if err != nil {
		t.Fatalf("fromMSPDI(ToMSPDI(...)): %v", err)
	}

	// Microsoft Project reads the constraint date at the start of the day
	// for a start constraint and its end for a finish constraint, and the
	// schema puts ConstraintType before PredecessorLink.
	text := string(xmlBytes)
	for _, fragment := range []string{
		"<ConstraintType>2</ConstraintType>", "<ConstraintDate>2026-06-04T08:00:00</ConstraintDate>",
		"<ConstraintType>7</ConstraintType>", "<ConstraintDate>2026-06-10T17:00:00</ConstraintDate>",
		"<ConstraintType>1</ConstraintType>",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("exported XML lacks %s", fragment)
		}
	}
	if strings.Contains(text, "soon") || strings.Count(text, "<ConstraintType>") != 3 {
		t.Error("X's constraint has no valid date and must not be written")
	}
	if b := text[strings.Index(text, "<UID>B</UID>"):]; strings.Index(b, "<ConstraintType>") > strings.Index(b, "<PredecessorLink>") {
		t.Error("ConstraintType must come before PredecessorLink")
	}

	if back.Title != "Round Trip" || len(back.Tasks) != 4 {
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
	for id, want := range map[string][2]string{
		"A": {"MSO", "2026-06-04"}, "B": {"FNLT", "2026-06-10"}, "M": {"ALAP", ""}, "X": {"", ""},
	} {
		if got := [2]string{string(byUID[id].Constraint), byUID[id].ConstraintDate}; got != want {
			t.Errorf("%s constraint after round trip = %v, want %v", id, got, want)
		}
	}
}
