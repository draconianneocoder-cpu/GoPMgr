// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pmforge/internal/calendar"
	"pmforge/internal/charts"
	"pmforge/internal/charts/dag"
	chartstats "pmforge/internal/charts/stats"
	"pmforge/internal/db"
	"pmforge/internal/export"
	"pmforge/internal/kernel"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================
// Charts
// =========================================================

func (a *App) ListChartKinds() []charts.Definition { return charts.All() }

func (a *App) ListCharts(kind string) ([]db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListCharts(p.ID, kind)
}

func (a *App) GetChart(id string) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	return d.GetChart(id)
}

func (a *App) SaveChart(c db.Chart) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	if c.ProjectID == "" {
		p, err := d.GetProject()
		if err != nil {
			return db.Chart{}, err
		}
		c.ProjectID = p.ID
	}
	return d.SaveChart(c)
}

func (a *App) DeleteChart(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	actor := "unknown"
	if u := a.requireUser(); u != nil {
		actor = u.Username
	}
	_ = d.LogAction(actor, "delete_chart", id, "")
	return d.DeleteChart(id)
}

// LayoutChart asks the chart engine to produce a frontend-ready
// layout. The Svelte renderer reads `engine` and dispatches.
//
// CPM charts are calendar-anchored when the open project has a start
// date: each node additionally carries StartDate/FinishDate computed
// against the project country's work calendar. Projects without a
// start date keep the plain day-offset layout.
func (a *App) LayoutChart(id string) (charts.LayoutResult, error) {
	d := a.requireDB()
	if d == nil {
		return charts.LayoutResult{}, errors.New("no project open")
	}
	c, err := d.GetChart(id)
	if err != nil {
		return charts.LayoutResult{}, err
	}

	var (
		projectStart time.Time
		isWorkday    kernel.WorkdayFunc
		capacityPlan kernel.ResourceCapacityPlan
	)
	if proj, perr := d.GetProject(); perr == nil {
		if start, ok := parseProjectDate(proj.StartDate); ok {
			projectStart = start
			isWorkday = calendar.For(proj.CountryCode).IsWorkday
			capacityPlan = resourceCapacityPlan(d, proj.ID)
		}
	}

	res, err := charts.LayoutWithSchedulePlan(charts.Kind(c.Kind), c.Data, projectStart, isWorkday, capacityPlan)
	if err != nil && !errors.Is(err, charts.ErrEngineNotImplemented) {
		return charts.LayoutResult{}, err
	}
	res.Title = c.Title
	return res, nil
}

// =========================================================
// Schedule baselines (roadmap item 17)
// =========================================================

// SetScheduleBaseline snapshots the current scheduled state of a CPM
// chart under an optional name. The stored payload is the fully
// scheduled kernel task map (constraints armed, CPM run, dates
// anchored when the project has a start date).
func (a *App) SetScheduleBaseline(chartID, name string) (db.Baseline, error) {
	d := a.requireDB()
	if d == nil {
		return db.Baseline{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Baseline{}, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return db.Baseline{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return db.Baseline{}, err
	}
	if len(tasks) == 0 {
		return db.Baseline{}, errors.New("chart has no tasks to baseline")
	}
	scheduleProjectTasks(proj, tasks)
	blob, err := json.Marshal(tasks)
	if err != nil {
		return db.Baseline{}, err
	}
	return d.SaveBaseline(db.Baseline{
		ProjectID: proj.ID,
		ChartID:   chartID,
		Name:      name,
		Data:      string(blob),
	})
}

// ListScheduleBaselines returns a chart's baselines, newest first.
func (a *App) ListScheduleBaselines(chartID string) ([]db.Baseline, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	return d.ListBaselines(chartID)
}

// DeleteScheduleBaseline removes a baseline snapshot.
func (a *App) DeleteScheduleBaseline(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteBaseline(id)
}

// CompareScheduleBaseline diffs the chart's CURRENT schedule against
// a stored baseline (the newest one when baselineID is empty).
// Returns per-task variances keyed by task ID; an empty map when the
// chart has no baseline yet.
func (a *App) CompareScheduleBaseline(chartID, baselineID string) (map[string]kernel.ScheduleVariance, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}

	var (
		base db.Baseline
		err  error
	)
	if baselineID != "" {
		base, err = d.GetBaseline(baselineID)
		if err != nil {
			return nil, err
		}
	} else {
		list, lerr := d.ListBaselines(chartID)
		if lerr != nil {
			return nil, lerr
		}
		if len(list) == 0 {
			return map[string]kernel.ScheduleVariance{}, nil
		}
		base = list[0]
	}

	baseline := make(map[string]*kernel.Task)
	if err := json.Unmarshal([]byte(base.Data), &baseline); err != nil {
		return nil, fmt.Errorf("baseline %s is corrupt: %w", base.ID, err)
	}

	proj, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return nil, err
	}
	current, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return nil, err
	}
	scheduleProjectTasks(proj, current)

	return kernel.CompareSchedules(current, baseline), nil
}

// ComputeScheduleEVM derives earned-value metrics for a CPM chart at
// a status date ("" = today, else YYYY-MM-DD). EVM needs the project
// start date to place the status date on the schedule's working-day
// axis, so projects without one get a clear error instead of numbers
// that look right but mean nothing.
func (a *App) ComputeScheduleEVM(chartID, asOfDate string) (kernel.EVMetrics, error) {
	d := a.requireDB()
	if d == nil {
		return kernel.EVMetrics{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		return kernel.EVMetrics{}, errors.New("earned value needs a project start date (Project Settings)")
	}

	c, err := d.GetChart(chartID)
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return kernel.EVMetrics{}, err
	}
	if len(tasks) == 0 {
		return kernel.EVMetrics{}, errors.New("chart has no tasks")
	}
	scheduleProjectTasks(proj, tasks)

	asOf := time.Now().UTC()
	if asOfDate != "" {
		parsed, perr := time.Parse(kernel.DateLayout, asOfDate)
		if perr != nil {
			return kernel.EVMetrics{}, fmt.Errorf("status date %q: want YYYY-MM-DD", asOfDate)
		}
		asOf = parsed
	}
	cal := calendar.For(proj.CountryCode)
	asOfDay, ok := kernel.DayOffset(start, asOf, cal.IsWorkday)
	if !ok {
		return kernel.EVMetrics{}, errors.New("status date is unreachably far from the project start")
	}

	return kernel.ComputeEVM(tasks, asOfDay), nil
}

// RunChartMonteCarlo runs probabilistic scheduling for a CPM chart
// using each task's optional DurationEstimate. Tasks without an
// estimate use their deterministic Duration.
func (a *App) RunChartMonteCarlo(chartID string, iterations int, workers int) (kernel.SimResult, error) {
	d := a.requireDB()
	if d == nil {
		return kernel.SimResult{}, errors.New("no project open")
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return kernel.SimResult{}, err
	}
	if c.Kind != string(charts.KindCPM) {
		return kernel.SimResult{}, fmt.Errorf("monte carlo requires a CPM chart, got %q", c.Kind)
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return kernel.SimResult{}, err
	}
	if len(tasks) == 0 {
		return kernel.SimResult{}, errors.New("chart has no tasks")
	}
	result := kernel.RunMonteCarlo(tasks, iterations, workers)
	if !result.Valid {
		return result, errors.New(result.Error)
	}
	return result, nil
}

// ExportChartMonteCarloRiskReport runs probabilistic scheduling for a CPM
// chart and writes a PDF/A-tagged Monte Carlo risk report to the user's
// private exports folder.
func (a *App) ExportChartMonteCarloRiskReport(chartID string, iterations int, workers int) (string, error) {
	u := a.requireUser()
	d := a.requireDB()
	if u == nil || d == nil {
		return "", errors.New("not signed in or no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return "", err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return "", err
	}
	if c.Kind != string(charts.KindCPM) {
		return "", fmt.Errorf("monte carlo risk report requires a CPM chart, got %q", c.Kind)
	}
	result, err := a.RunChartMonteCarlo(chartID, iterations, workers)
	if err != nil {
		return "", err
	}
	raw, err := export.GenerateMonteCarloRiskReport(export.MonteCarloRiskReportSpec{
		ProjectName: proj.Name,
		ChartTitle:  c.Title,
		Result:      result,
	})
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("Monte-Carlo-Risk-Report-%s-%s.pdf",
		sanitizeFilename(c.Title),
		time.Now().UTC().Format("20060102-150405"),
	))
	if err := os.WriteFile(outPath, raw, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// LevelChartResources runs the kernel's serial resource-levelling
// pass on a CPM chart and PERSISTS the result: every task that
// levelling delayed beyond its precedence-earliest start gets a SNET
// constraint at its levelled start date. Nodes with a user-set
// constraint other than SNET are never touched (links and user intent
// win); previously levelled SNET pins are recomputed. Requires a
// project start date to express levelled offsets as dates.
//
// Returns the number of tasks pinned.
func (a *App) LevelChartResources(chartID string) (int, error) {
	d := a.requireDB()
	if d == nil {
		return 0, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return 0, err
	}
	start, ok := parseProjectDate(proj.StartDate)
	if !ok {
		return 0, errors.New("resource levelling needs a project start date (Project Settings)")
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return 0, err
	}

	var doc dagDoc
	if err := json.Unmarshal([]byte(c.Data), &doc); err != nil {
		return 0, err
	}

	cal := calendar.For(proj.CountryCode)

	// Baseline pass: precedence-only ES per task.
	plain, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return 0, err
	}
	if len(plain) == 0 {
		return 0, errors.New("chart has no tasks")
	}
	kernel.ApplyConstraintDates(plain, start, cal.IsWorkday)
	if !kernel.CalculateCPM(plain) {
		return 0, errors.New("chart contains a dependency cycle")
	}

	// Levelling pass on a fresh copy.
	levelled, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return 0, err
	}
	kernel.ApplyConstraintDates(levelled, start, cal.IsWorkday)
	if !kernel.LevelResourcesWithPlan(levelled, resourceCapacityPlan(d, proj.ID)) {
		return 0, errors.New("chart contains a dependency cycle")
	}
	kernel.AnchorSchedule(levelled, start, cal.IsWorkday)

	pinned := 0
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		lt, lok := levelled[n.ID]
		pt, pok := plain[n.ID]
		if !lok || !pok {
			continue
		}
		existing := strings.ToUpper(strings.TrimSpace(n.Constraint))
		if existing != "" && existing != string(kernel.StartNoEarlierThan) {
			continue // never override a user-set non-SNET constraint
		}
		if lt.ES > pt.ES+1e-9 {
			n.Constraint = string(kernel.StartNoEarlierThan)
			n.ConstraintDate = lt.StartDate
			pinned++
		} else if existing == string(kernel.StartNoEarlierThan) && n.ConstraintDate != "" {
			// A previous levelling pin that's no longer needed.
			n.Constraint = ""
			n.ConstraintDate = ""
		}
	}

	blob, err := json.Marshal(doc)
	if err != nil {
		return 0, err
	}
	c.Data = string(blob)
	if _, err := d.SaveChart(c); err != nil {
		return 0, err
	}
	return pinned, nil
}

// GenerateResourceHistogram builds (or refreshes) a Bar chart showing
// each resource's per-day demand for a CPM chart's schedule. The
// histogram is a snapshot: regenerate it after editing the schedule.
// The bar chart's config carries {"source_chart_id": ...} so repeated
// generation updates the same chart instead of accumulating copies.
func (a *App) GenerateResourceHistogram(chartID string) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Chart{}, err
	}
	c, err := d.GetChart(chartID)
	if err != nil {
		return db.Chart{}, err
	}
	tasks, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return db.Chart{}, err
	}
	if len(tasks) == 0 {
		return db.Chart{}, errors.New("chart has no tasks")
	}
	scheduleProjectTasks(proj, tasks)

	usage := kernel.ResourceUsage(tasks)
	if len(usage) == 0 {
		return db.Chart{}, errors.New("no resource assignments on this chart")
	}

	// Shared horizon across resources; day labels are real dates when
	// the project is anchored, plain offsets otherwise.
	horizon := 0
	for _, profile := range usage {
		if len(profile) > horizon {
			horizon = len(profile)
		}
	}
	categories := make([]string, horizon)
	if start, ok := parseProjectDate(proj.StartDate); ok {
		cal := calendar.For(proj.CountryCode)
		dayTasks := make(map[string]*kernel.Task, horizon)
		for i := 0; i < horizon; i++ {
			id := fmt.Sprintf("d%d", i)
			dayTasks[id] = &kernel.Task{ID: id, Duration: 1, ES: float64(i), EF: float64(i + 1)}
		}
		kernel.AnchorSchedule(dayTasks, start, cal.IsWorkday)
		for i := 0; i < horizon; i++ {
			categories[i] = dayTasks[fmt.Sprintf("d%d", i)].StartDate
		}
	} else {
		for i := 0; i < horizon; i++ {
			categories[i] = fmt.Sprintf("Day %d", i+1)
		}
	}

	resources := make([]string, 0, len(usage))
	for r := range usage {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	capacityProfiles := kernel.ResourceCapacityProfiles(resourceCapacityPlan(d, proj.ID), resources, horizon)
	barDoc := struct {
		Title      string                 `json:"title"`
		XLabel     string                 `json:"x_label"`
		YLabel     string                 `json:"y_label"`
		Categories []string               `json:"categories"`
		Series     []chartstats.BarSeries `json:"series"`
	}{
		Title:      "Resource usage — " + c.Title,
		XLabel:     "Day",
		YLabel:     "Units",
		Categories: categories,
	}
	for _, r := range resources {
		values := make([]float64, horizon)
		copy(values, usage[r])
		barDoc.Series = append(barDoc.Series, chartstats.BarSeries{Name: r, Values: values})
	}
	for _, r := range resources {
		values := make([]float64, horizon)
		copy(values, capacityProfiles[r])
		barDoc.Series = append(barDoc.Series, chartstats.BarSeries{
			Name:   r + " capacity",
			Values: values,
			Type:   "line",
			Color:  "#f59e0b",
			Dashed: true,
		})
	}

	blob, err := json.Marshal(barDoc)
	if err != nil {
		return db.Chart{}, err
	}

	// Reuse the previous histogram for this source chart if present.
	configMarker := fmt.Sprintf(`{"source_chart_id":%q}`, chartID)
	target := db.Chart{
		ProjectID: proj.ID,
		Kind:      string(charts.KindBar),
		Title:     "Resource Histogram — " + c.Title,
		Config:    configMarker,
	}
	if existing, lerr := d.ListCharts(proj.ID, string(charts.KindBar)); lerr == nil {
		for _, e := range existing {
			if e.Config == configMarker {
				target.ID = e.ID
				break
			}
		}
	}
	target.Data = string(blob)
	return d.SaveChart(target)
}

// dagDoc is the minimal layered-document shape LevelChartResources
// round-trips. It must list every persisted node field so the
// re-marshal does not drop data.
type dagDoc struct {
	Nodes []dag.LayeredNode `json:"nodes"`
	Edges []dag.LayeredEdge `json:"edges"`
}

// ImportMSPDIChart opens a file dialog for a Microsoft Project Data
// Interchange XML file and imports it as a new CPM chart in the open
// project. If the project has no start date yet and the file carries
// one, the project start date is adopted so the imported schedule
// anchors immediately.
func (a *App) ImportMSPDIChart() (db.Chart, error) {
	if a.ctx == nil {
		return db.Chart{}, errors.New("no context (Wails not started)")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Import project schedule",
		DefaultDirectory: a.userDir(),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Project schedules (*.xml, *.mpp, *.mpx, *.pod)", Pattern: "*.xml;*.mpp;*.mpx;*.pod"},
			{DisplayName: "MS Project XML (*.xml)", Pattern: "*.xml"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return db.Chart{}, err
	}
	if path == "" {
		return db.Chart{}, errors.New("import cancelled")
	}
	return a.importScheduleFile(path)
}

// importScheduleFile routes an imported project file by extension. MS Project
// XML (MSPDI, *.xml) is parsed directly. Binary/serialized formats (.mpp,
// .pod) and the legacy .mpx text format cannot be read in pure Go, so we
// return a precise, actionable message pointing at the universally-supported
// MS Project XML interchange path rather than failing opaquely.
func (a *App) importScheduleFile(path string) (db.Chart, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mpp":
		return db.Chart{}, errors.New(
			"Microsoft Project .mpp is a binary format PMForge cannot read directly. " +
				"In Microsoft Project choose File → Save As → \"Microsoft Project XML (*.xml)\", " +
				"then import that .xml here.")
	case ".mpx":
		return db.Chart{}, errors.New(
			"The legacy .mpx format is not supported directly. Re-save it as " +
				"\"Microsoft Project XML (*.xml)\" from Microsoft Project and import the .xml here.")
	case ".pod":
		return db.Chart{}, errors.New(
			"ProjectLibre .pod is a native binary format PMForge cannot read directly. " +
				"In ProjectLibre choose File → Save As / Export → \"Microsoft Project XML (*.xml)\", " +
				"then import that .xml here.")
	default:
		// .xml or any other extension: attempt the MSPDI parser (it fails
		// clearly if the content is not MSPDI XML).
		data, err := os.ReadFile(path) // #nosec G304 -- user-selected import file.
		if err != nil {
			return db.Chart{}, err
		}
		return a.importMSPDIFromBytes(data)
	}
}

// importMSPDIFromBytes is ImportMSPDIChart minus the file dialog so
// the conversion is unit-testable.
func (a *App) importMSPDIFromBytes(data []byte) (db.Chart, error) {
	d := a.requireDB()
	if d == nil {
		return db.Chart{}, errors.New("no project open")
	}
	proj, err := d.GetProject()
	if err != nil {
		return db.Chart{}, err
	}

	imported, err := export.FromMSPDI(data)
	if err != nil {
		return db.Chart{}, err
	}

	var doc dagDoc
	for _, t := range imported.Tasks {
		doc.Nodes = append(doc.Nodes, dag.LayeredNode{
			ID:              t.UID,
			Label:           t.Name,
			Duration:        t.DurationDays,
			Milestone:       t.Milestone,
			PercentComplete: t.PercentComplete,
			Assignments:     t.Assignments,
		})
		for _, l := range t.Links {
			doc.Edges = append(doc.Edges, dag.LayeredEdge{
				From:  l.Pred,
				To:    t.UID,
				Label: dag.FormatLinkLabel(l.Type, l.Lag),
			})
		}
	}

	blob, err := json.Marshal(doc)
	if err != nil {
		return db.Chart{}, err
	}

	title := imported.Title
	if title == "" {
		title = "Imported Schedule"
	}

	// Adopt the file's start date when the project lacks one.
	if _, ok := parseProjectDate(proj.StartDate); !ok && imported.StartDate != "" {
		proj.StartDate = imported.StartDate
		if _, err := d.UpsertProject(proj); err != nil {
			return db.Chart{}, err
		}
	}

	return d.SaveChart(db.Chart{
		ProjectID: proj.ID,
		Kind:      string(charts.KindCPM),
		Title:     title,
		Data:      string(blob),
	})
}
