// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopmgr/internal/cli"
	"gopmgr/internal/db"
	"gopmgr/internal/export"
)

const sampleImportXML = `<?xml version="1.0" encoding="UTF-8"?>
<Project xmlns="http://schemas.microsoft.com/project">
  <Title>Imported Plan</Title>
  <StartDate>2026-07-06T08:00:00</StartDate>
  <Tasks>
    <Task><UID>1</UID><Name>Pack</Name><Duration>PT16H0M0S</Duration></Task>
    <Task>
      <UID>2</UID><Name>Move</Name><Duration>PT8H0M0S</Duration>
      <ConstraintType>5</ConstraintType><ConstraintDate>2026-07-10T08:00:00</ConstraintDate>
      <PredecessorLink><PredecessorUID>1</PredecessorUID><Type>3</Type><LinkLag>4800</LinkLag></PredecessorLink>
    </Task>
  </Tasks>
</Project>`

// allMSPDIFields imports every schedule field, the import dialog's default.
var allMSPDIFields = export.MSPDIImportOptions{IncludeDependencies: true, IncludeProgress: true, IncludeAssignments: true, IncludeConstraints: true}

func TestImportMSPDIFromBytes(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	// Project WITHOUT a start date: the import should adopt the file's.
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Import Target"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}

	c, err := app.importMSPDIFromBytesWithOptions([]byte(sampleImportXML), allMSPDIFields)
	if err != nil {
		t.Fatalf("importMSPDIFromBytesWithOptions: %v", err)
	}
	if c.Kind != "cpm" || c.Title != "Imported Plan" {
		t.Errorf("chart = %s %q", c.Kind, c.Title)
	}

	var doc struct {
		Nodes []struct {
			ID             string  `json:"id"`
			Label          string  `json:"label"`
			Duration       float64 `json:"duration"`
			Constraint     string  `json:"constraint"`
			ConstraintDate string  `json:"constraint_date"`
		} `json:"nodes"`
		Edges []struct {
			From, To, Label string
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(c.Data), &doc); err != nil {
		t.Fatalf("unmarshal chart data: %v", err)
	}
	if len(doc.Nodes) != 2 || len(doc.Edges) != 1 {
		t.Fatalf("doc shape: %d nodes %d edges", len(doc.Nodes), len(doc.Edges))
	}
	if doc.Edges[0].Label != "SS+1" {
		t.Errorf("edge label = %q, want SS+1", doc.Edges[0].Label)
	}
	// Move's SNLT (type 5) reaches the chart node.
	for _, n := range doc.Nodes {
		want := [2]string{}
		if n.ID == "2" {
			want = [2]string{"SNLT", "2026-07-10"}
		}
		if got := [2]string{n.Constraint, n.ConstraintDate}; got != want {
			t.Errorf("node %s constraint = %v, want %v", n.ID, got, want)
		}
	}

	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.StartDate != "2026-07-06" {
		t.Errorf("project start date = %q, want adopted 2026-07-06", proj.StartDate)
	}

	// The imported chart must schedule end-to-end.
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("loader: %v (%d tasks)", err, len(tasks))
	}
	scheduleProjectTasks(proj, tasks)
	if tasks["2"].ES != 1 { // SS+1 off task 1's start (day 0)
		t.Errorf("Move ES = %v, want 1", tasks["2"].ES)
	}
	if tasks["2"].StartDate != "2026-07-07" {
		t.Errorf("Move StartDate = %q, want 2026-07-07", tasks["2"].StartDate)
	}
	// SNLT Friday 07-10 (index 4) is armed and met by a Tuesday start.
	if m := tasks["2"]; !m.ConstraintArmed || m.ConstraintDay != 4 || m.ConstraintViolated {
		t.Errorf("Move constraint armed %v day %v violated %v, want armed day 4 met", m.ConstraintArmed, m.ConstraintDay, m.ConstraintViolated)
	}
}

func TestImportMSPDIWithOptionsPersistsMappingReceipt(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import-options.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Import Target"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}
	c, err := app.importMSPDIFromBytesWithOptions([]byte(sampleImportXML), export.MSPDIImportOptions{
		IncludeDependencies: false, IncludeProgress: false, IncludeAssignments: false,
	})
	if err != nil {
		t.Fatalf("importMSPDIFromBytesWithOptions: %v", err)
	}
	var config struct {
		Receipt export.MSPDIImportReceipt `json:"mspdi_import_receipt"`
	}
	if err := json.Unmarshal([]byte(c.Config), &config); err != nil {
		t.Fatalf("decode import receipt: %v", err)
	}
	// All five options off: links, progress, assignments, constraints and
	// levelling delays.
	if len(config.Receipt.ExcludedFields) != 5 {
		t.Fatalf("receipt exclusions = %#v, want five", config.Receipt.ExcludedFields)
	}
}

func TestImportMSPDIKeepsExistingStartDate(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-import2.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Has Date", StartDate: "2026-01-01"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}

	if _, err := app.importMSPDIFromBytesWithOptions([]byte(sampleImportXML), allMSPDIFields); err != nil {
		t.Fatalf("importMSPDIFromBytesWithOptions: %v", err)
	}
	proj, _ := d.GetProject()
	if proj.StartDate != "2026-01-01" {
		t.Errorf("existing start date must be preserved, got %q", proj.StartDate)
	}
}

// A levelled schedule exported as MSPDI and imported again keeps its
// levelled starts (owner decision 2026-11-23: LevelingDelay, following
// Microsoft's documentation), whether or not links are imported: the
// delays are applied on the file's own network. X (4.75 days on alice,
// critical through X2) holds Y back to Friday 06-05 15:00; Z follows Y by
// its link and needs no start of its own. Y's delay, Monday 08:00 to
// Friday 15:00, is 6180 elapsed minutes, written in tenths: 61800.
func TestMSPDIRoundTripKeepsLevelledStarts(t *testing.T) {
	app, d, _ := newResourceTestApp(t)
	c, err := d.SaveChart(db.Chart{ProjectID: "project-1", Kind: "cpm", Title: "Levelled", Data: `{"nodes":[
		{"id":"X","label":"X","duration":4.75,"assignments":[{"resource":"alice"}]},
		{"id":"X2","label":"X2","duration":3},
		{"id":"Y","label":"Y","duration":1,"assignments":[{"resource":"alice"}]},
		{"id":"Z","label":"Z","duration":1}],
		"edges":[{"from":"X","to":"X2"},{"from":"Y","to":"Z"}]}`})
	if err != nil {
		t.Fatalf("SaveChart: %v", err)
	}
	if _, err := app.LevelChartResources(c.ID, "", false, false); err != nil {
		t.Fatalf("LevelChartResources: %v", err)
	}
	levelled := storedLevelling(t, d, c.ID)
	if want := (levelledNode{LevelStart: "2026-06-05", LevelStartFraction: 0.75}); levelled["Y"] != want || levelled["Z"] != (levelledNode{}) {
		t.Fatalf("levelled Y %+v Z %+v, want Y %+v and nothing on Z", levelled["Y"], levelled["Z"], want)
	}
	// The export reads the project's current CPM chart: keep only this one.
	charts, err := d.ListCharts("project-1", "cpm")
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	for _, other := range charts {
		if other.ID != c.ID {
			if err := d.DeleteChart(other.ID); err != nil {
				t.Fatalf("DeleteChart: %v", err)
			}
		}
	}

	out := filepath.Join(t.TempDir(), "levelled.xml")
	if err := runHeadlessExport(&cli.Config{ExportPath: out, ExportFormat: "mspdi"}, d); err != nil {
		t.Fatalf("export: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<LevelingDelay>61800</LevelingDelay>") || strings.Count(string(data), "<LevelingDelay>") != 1 {
		t.Fatalf("export should carry one delay, Y's 61800 tenths of elapsed minutes:\n%s", data)
	}

	for _, withLinks := range []bool{true, false} {
		options := allMSPDIFields
		options.IncludeDependencies, options.IncludeLevelingDelay = withLinks, true
		imported, err := app.importMSPDIFromBytesWithOptions(data, options)
		if err != nil {
			t.Fatalf("import (links %v): %v", withLinks, err)
		}
		got := storedLevelling(t, d, imported.ID)
		for _, id := range []string{"X", "X2", "Y", "Z"} {
			if got[id] != levelled[id] {
				t.Errorf("links %v: imported %s = %+v, want %+v", withLinks, id, got[id], levelled[id])
			}
		}
		if !strings.HasPrefix(storedLevelledInputs(t, d, imported.ID), "v2:") {
			t.Errorf("links %v: the imported levelled chart has no levelling digest", withLinks)
		}
		if withLinks && layoutReportsStale(t, app, imported.ID) {
			t.Error("a just-imported levelled chart reports stale levelling")
		}
	}
}

// Without a project start date the delays cannot be placed on dates: they
// are counted as skipped, and nothing is stored.
func TestMSPDIImportSkipsLevelingDelaysWithoutAStartDate(t *testing.T) {
	d, err := db.InitDB(filepath.Join(t.TempDir(), "mspdi-delay.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.UpsertProject(db.Project{ID: "project-1", Name: "No start"}); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	app := &App{db: d}
	options := allMSPDIFields
	options.IncludeLevelingDelay = true
	c, err := app.importMSPDIFromBytesWithOptions([]byte(`<Project xmlns="http://schemas.microsoft.com/project"><Tasks>
		<Task><UID>1</UID><Name>A</Name><Duration>PT8H0M0S</Duration><LevelingDelay>9600</LevelingDelay><LevelingDelayFormat>7</LevelingDelayFormat></Task>
		</Tasks></Project>`), options)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := storedLevelling(t, d, c.ID)["1"]; got != (levelledNode{}) {
		t.Errorf("stored %+v, want nothing", got)
	}
	var config struct {
		Receipt export.MSPDIImportReceipt `json:"mspdi_import_receipt"`
	}
	if err := json.Unmarshal([]byte(c.Config), &config); err != nil {
		t.Fatal(err)
	}
	if config.Receipt.SkippedLevelingDelays != 1 {
		t.Errorf("skipped %d, want 1", config.Receipt.SkippedLevelingDelays)
	}
}
