// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopmgr/internal/db"
	"gopmgr/internal/schedaudit"
)

// auditChart is a CPM chart in the stored JSON shape.
func auditChart(t *testing.T, d *db.Database, projectID, title, data string) db.Chart {
	t.Helper()
	c, err := d.SaveChart(db.Chart{ProjectID: projectID, Kind: "cpm", Title: title, Data: data})
	if err != nil {
		t.Fatalf("SaveChart %s: %v", title, err)
	}
	return c
}

// newAuditTestApp opens a plaintext project starting on Monday 2026-06-01.
func newAuditTestApp(t *testing.T, start string) (*App, *db.Database, db.Project) {
	t.Helper()
	d, err := db.InitDB(filepath.Join(t.TempDir(), "audit.pmforge"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := d.UpsertProject(db.Project{ID: "project-1", Name: "Audit", StartDate: start})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	return &App{db: d}, d, p
}

func subjects(r schedaudit.Result, rule schedaudit.RuleID) []string {
	var out []string
	for _, f := range r.Findings {
		if f.Rule == rule {
			out = append(out, f.TaskID)
		}
	}
	sort.Strings(out)
	return out
}

func ruleStatus(t *testing.T, r schedaudit.Result, rule schedaudit.RuleID) schedaudit.RuleStatus {
	t.Helper()
	for _, s := range r.Rules {
		if s.Rule == rule {
			return s
		}
	}
	t.Fatalf("no status for %s", rule)
	return schedaudit.RuleStatus{}
}

// SQA-020, stored half: an audit of a real encrypted project writes nothing.
// The chart row, its revision, the project's charts and baselines and the
// tamper-evident audit log are all unchanged afterwards.
func TestAuditScheduleLeavesTheEncryptedProjectUnchanged(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	acceptNoRecoveryCodes(t, app)
	file, err := app.CreateProject("Audit Plan", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if encrypted, err := db.IsEncryptedFile(file.Path); err != nil || !encrypted {
		t.Fatalf("project %s encrypted = %v, %v; the test needs an encrypted project", file.Path, encrypted, err)
	}
	if _, err := app.OpenProject(file.Path); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	d := app.requireDB()
	proj, err := d.GetProject()
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	proj.StartDate = "2026-06-01"
	if proj, err = d.UpsertProject(proj); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	c := auditChart(t, d, proj.ID, "Plan", `{"nodes":[
		{"id":"A","label":"Design","duration":5},
		{"id":"B","label":"Build","duration":3,"constraint":"MFO","constraint_date":"2026-06-03"}],
		"edges":[{"from":"A","to":"B"}]}`)

	snapshot := func() []any {
		chart, err := d.GetChart(c.ID)
		if err != nil {
			t.Fatalf("GetChart: %v", err)
		}
		charts, err := d.ListCharts(proj.ID, "")
		if err != nil {
			t.Fatalf("ListCharts: %v", err)
		}
		baselines, err := d.ListBaselines(c.ID)
		if err != nil {
			t.Fatalf("ListBaselines: %v", err)
		}
		events, err := d.ListAuditEvents(proj.ID)
		if err != nil {
			t.Fatalf("ListAuditEvents: %v", err)
		}
		project, err := d.GetProject()
		if err != nil {
			t.Fatalf("GetProject: %v", err)
		}
		return []any{chart, charts, baselines, events, project}
	}
	before := snapshot()

	r, err := app.AuditSchedule(context.Background(), c.ID, "2026-06-10", "")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	if got := subjects(r, schedaudit.ConstraintViolated); !reflect.DeepEqual(got, []string{"B"}) {
		t.Fatalf("SQ-09 = %v, want [B]: the audit must have run on the stored chart", got)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("the audit changed the stored project:\nbefore %+v\nafter  %+v", before, after)
	}
}

// SQA-029: with two CPM charts, auditing the older one audits that chart and
// labels the result with its ID and revision, not the newest chart's.
func TestAuditScheduleAuditsTheNamedChart(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	older := auditChart(t, d, p.ID, "Older", `{"nodes":[
		{"id":"O1","label":"Old one","duration":2},{"id":"O2","label":"Old two","duration":2}],"edges":[]}`)
	newer := auditChart(t, d, p.ID, "Newer", `{"nodes":[{"id":"N1","label":"New","duration":1}],"edges":[]}`)
	if older.UpdatedAt == newer.UpdatedAt {
		t.Fatalf("both charts have revision %q; the revision check below would prove nothing", older.UpdatedAt)
	}

	r, err := app.AuditSchedule(context.Background(), older.ID, "", "default")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	want := schedaudit.Source{ChartID: older.ID, Revision: older.UpdatedAt}
	if r.Source != want {
		t.Fatalf("result source = %+v, want %+v", r.Source, want)
	}
	if len(r.Findings) == 0 {
		t.Fatal("the older chart has unlinked tasks, so SQ-01 and SQ-02 must report them")
	}
	for _, f := range r.Findings {
		if f.Source != want {
			t.Fatalf("finding %s source = %+v, want %+v", f.Rule, f.Source, want)
		}
		if f.TaskID != "" && f.TaskID != "O1" && f.TaskID != "O2" {
			t.Fatalf("finding %s names task %q, which is not in the audited chart", f.Rule, f.TaskID)
		}
	}
}

func TestAuditScheduleRefusesWhatItCannotAudit(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	good := auditChart(t, d, p.ID, "Plan", `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
	empty := auditChart(t, d, p.ID, "Empty", `{"nodes":[],"edges":[]}`)
	gantt, err := d.SaveChart(db.Chart{ProjectID: p.ID, Kind: "gantt", Title: "Bars", Data: `{"nodes":[]}`})
	if err != nil {
		t.Fatalf("SaveChart gantt: %v", err)
	}

	cases := []struct {
		name, chart, date, profile, want string
	}{
		{"unknown chart", "no-such-chart", "", "", "not found"},
		{"not a CPM chart", gantt.ID, "", "", "CPM"},
		{"no tasks", empty.ID, "", "", "no tasks"},
		{"unknown profile", good.ID, "", "strict", "profile"},
		{"display name is not a profile key", good.ID, "", "DCMA-style", "profile"},
		{"impossible date", good.ID, "2026-13-01", "", "reporting date"},
		{"timestamp instead of a date", good.ID, "2026-06-10T00:00:00Z", "", "reporting date"},
		{"day-first date", good.ID, "10/06/2026", "", "reporting date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.AuditSchedule(context.Background(), tc.chart, tc.date, tc.profile)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AuditSchedule error = %v, want one mentioning %q", err, tc.want)
			}
		})
	}

	if _, err := (&App{}).AuditSchedule(context.Background(), good.ID, "", ""); err == nil {
		t.Fatal("AuditSchedule with no project open must fail")
	}
}

func TestAuditScheduleSelectsTheProfileByKey(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	c := auditChart(t, d, p.ID, "Plan", `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
	for key, name := range map[string]string{
		"":           schedaudit.DefaultProfile().Name,
		"default":    schedaudit.DefaultProfile().Name,
		"dcma-style": schedaudit.DCMAStyleProfile().Name,
	} {
		r, err := app.AuditSchedule(context.Background(), c.ID, "", key)
		if err != nil {
			t.Fatalf("AuditSchedule(%q): %v", key, err)
		}
		if r.Profile != name {
			t.Fatalf("profile key %q ran %q, want %q", key, r.Profile, name)
		}
	}
}

// The audit must judge the schedule the app reports. The expected
// violations and every task's float come from scheduleProjectTasks, the
// pipeline reports use, not from the audit's own code. The project start is
// stored as RFC3339, which the audit does not accept, so the method must
// convert it. It is midnight UTC because any other time of day shifts the
// app's constraint offsets (KRN-02); TestAuditProjectStartIsTheDisplayedDate
// covers those values.
//
// Start Mon 2026-06-01, a work week of Monday to Friday: L (60 days) runs in
// parallel with A (5) -> B (3). B must finish on Wed 2026-06-03 but cannot
// finish before day 8, so SQ-09 reports B, and its finish pulls A back too:
// both have negative float. S (1 day) is free, so its float is large. M
// carries MSO, which the scheduler does not support.
func TestAuditScheduleJudgesTheScheduleTheAppReports(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01T00:00:00Z")
	data := `{"nodes":[
		{"id":"A","label":"A","duration":5},
		{"id":"B","label":"B","duration":3,"constraint":"MFO","constraint_date":"2026-06-03"},
		{"id":"L","label":"L","duration":60},
		{"id":"S","label":"S","duration":1},
		{"id":"M","label":"M","duration":2,"constraint":"mso","constraint_date":"2026-06-02"}],
		"edges":[{"from":"A","to":"B"}]}`
	c := auditChart(t, d, p.ID, "Plan", data)

	tasks, err := cpmChartDataToKernelTasks(data)
	if err != nil {
		t.Fatalf("cpmChartDataToKernelTasks: %v", err)
	}
	if err := scheduleProjectTasks(p, tasks); err != nil {
		t.Fatalf("scheduleProjectTasks: %v", err)
	}
	violated := []string{"M"} // the unsupported type, reported by SQ-09 too
	wantFloat := map[schedaudit.RuleID]map[string]string{schedaudit.HighFloat: {}, schedaudit.NegativeFloat: {}}
	for id, task := range tasks {
		if task.ConstraintViolated {
			violated = append(violated, id)
		}
		observed := "float " + strconv.FormatFloat(task.Float, 'f', -1, 64) + " working days"
		switch {
		case task.Float < -1e-9:
			wantFloat[schedaudit.NegativeFloat][id] = observed
		case task.Float > schedaudit.DefaultProfile().HighFloatDays:
			wantFloat[schedaudit.HighFloat][id] = observed
		}
	}
	sort.Strings(violated)
	if len(wantFloat[schedaudit.NegativeFloat]) != 2 || len(wantFloat[schedaudit.HighFloat]) == 0 {
		t.Fatalf("fixture check: scheduleProjectTasks gives %v; the test needs two negative and some high floats", wantFloat)
	}

	r, err := app.AuditSchedule(context.Background(), c.ID, "", "")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	if s := ruleStatus(t, r, schedaudit.ConstraintViolated); !s.Evaluated {
		t.Fatalf("SQ-09 not evaluated (%s): the RFC3339 project start was not converted", s.Reason)
	}
	if got := subjects(r, schedaudit.ConstraintViolated); !reflect.DeepEqual(got, violated) {
		t.Fatalf("SQ-09 = %v, want %v", got, violated)
	}
	for rule, want := range wantFloat {
		got := map[string]string{}
		for _, f := range r.Findings {
			if f.Rule == rule {
				got[f.TaskID] = f.Observed
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want the app's floats %v", rule, got, want)
		}
	}
}

// The audit takes the project start as the calendar date the app displays.
// The oracle is the start date scheduleProjectTasks gives an unconstrained
// first task, so it does not reuse the conversion under test.
func TestAuditProjectStartIsTheDisplayedDate(t *testing.T) {
	for _, stored := range []string{
		"2026-06-01",
		"2026-06-01T00:00:00Z",
		"2026-06-01T23:00:00-05:00",
		"2026-06-01T00:30:00+02:00",
	} {
		tasks, err := cpmChartDataToKernelTasks(`{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
		if err != nil {
			t.Fatalf("cpmChartDataToKernelTasks: %v", err)
		}
		if err := scheduleProjectTasks(db.Project{StartDate: stored}, tasks); err != nil {
			t.Fatalf("scheduleProjectTasks(%q): %v", stored, err)
		}
		if got, shown := auditProjectStart(stored), tasks["A"].StartDate; got != shown {
			t.Fatalf("auditProjectStart(%q) = %q, but the app shows the project starting %q", stored, got, shown)
		}
	}
	for _, stored := range []string{"", "June 1", "2026-06-31"} {
		if got := auditProjectStart(stored); got != "" {
			t.Fatalf("auditProjectStart(%q) = %q, want no date", stored, got)
		}
	}
}

// SQ-16 must check demand against the plan the chart itself uses: the
// stakeholder's availability, not a default of one.
func TestAuditScheduleUsesTheProjectsCapacityPlan(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	c := auditChart(t, d, p.ID, "Plan", `{"nodes":[
		{"id":"A","label":"A","duration":2,"assignments":[{"resource":"alice"}]},
		{"id":"B","label":"B","duration":2,"assignments":[{"resource":"alice"}]}],"edges":[]}`)

	r, err := app.AuditSchedule(context.Background(), c.ID, "", "")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	if len(subjects(r, schedaudit.Overallocation)) == 0 {
		t.Fatal("alice works on A and B at once with capacity 1, so SQ-16 must report it")
	}

	if _, err := d.SaveStakeholder(db.Stakeholder{ProjectID: p.ID, Name: "alice", Availability: 2}); err != nil {
		t.Fatalf("SaveStakeholder: %v", err)
	}
	r, err = app.AuditSchedule(context.Background(), c.ID, "", "")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	if got := subjects(r, schedaudit.Overallocation); len(got) != 0 {
		t.Fatalf("SQ-16 = %v; alice's availability is 2, so nothing is overallocated", got)
	}
}

// SQ-18 notes a missing baseline only while the audited chart has none.
func TestAuditScheduleReadsTheChartsBaselines(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	c := auditChart(t, d, p.ID, "Plan", `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
	other := auditChart(t, d, p.ID, "Other", `{"nodes":[{"id":"X","label":"X","duration":1}],"edges":[]}`)
	noBaseline := func() bool {
		t.Helper()
		r, err := app.AuditSchedule(context.Background(), c.ID, "", "")
		if err != nil {
			t.Fatalf("AuditSchedule: %v", err)
		}
		for _, f := range r.Findings {
			if f.Rule == schedaudit.ReportingContext && strings.Contains(f.Observed, "no baseline") {
				return true
			}
		}
		return false
	}
	if !noBaseline() {
		t.Fatal("SQ-18 must note that the chart has no baseline")
	}
	if _, err := d.SaveBaseline(db.Baseline{ProjectID: p.ID, ChartID: other.ID, Name: "Other's", Data: other.Data}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	if !noBaseline() {
		t.Fatal("another chart's baseline is not this chart's baseline")
	}
	if _, err := d.SaveBaseline(db.Baseline{ProjectID: p.ID, ChartID: c.ID, Name: "Approved", Data: c.Data}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	if noBaseline() {
		t.Fatal("SQ-18 still notes a missing baseline after one was saved for the chart")
	}
}

// The frontend reads findings, metrics and rule statuses as arrays, never
// null, including when there is nothing to report.
func TestAuditScheduleWireShape(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	c := auditChart(t, d, p.ID, "Plan", `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
	r, err := app.AuditSchedule(context.Background(), c.ID, "2026-06-10", "")
	if err != nil {
		t.Fatalf("AuditSchedule: %v", err)
	}
	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(blob, &wire); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"findings", "metrics", "rules"} {
		if v := wire[key]; len(v) == 0 || v[0] != '[' {
			t.Fatalf("%s = %s, want a JSON array", key, v)
		}
	}
	for _, key := range []string{"profile", "source", "engine_version", "reporting_date"} {
		if _, ok := wire[key]; !ok {
			t.Fatalf("wire result lacks %q: %s", key, blob)
		}
	}
}

// A cancelled call stops: the context the frontend's call carries reaches
// the audit.
func TestAuditScheduleStopsWhenCancelled(t *testing.T) {
	app, d, p := newAuditTestApp(t, "2026-06-01")
	c := auditChart(t, d, p.ID, "Plan", `{"nodes":[{"id":"A","label":"A","duration":1}],"edges":[]}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.AuditSchedule(ctx, c.ID, "", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("AuditSchedule on a cancelled call returned %v, want context.Canceled", err)
	}
}
