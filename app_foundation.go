// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pmforge/internal/agile"
	"pmforge/internal/analytics"
	"pmforge/internal/budget"
	"pmforge/internal/calendar"
	"pmforge/internal/db"
	"pmforge/internal/kernel"
	"pmforge/internal/templates"
	"pmforge/internal/timeline"
	"pmforge/internal/update"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================
// V2.x Foundation Slice — Launchpad / Stakeholders /
// Timeline / Budget / iCal (rickar+zen-go integrations)
// =========================================================

// LaunchpadEvaluate returns the list of seed actions (chart kinds +
// document kinds + agile flags) for a given (industry, methodology)
// pair. The actual decision lives in internal/templates'
// launchpad_seeds.json and is evaluated by zen-go.
func (a *App) LaunchpadEvaluate(industry, methodology string) ([]string, error) {
	if a.templates == nil {
		return []string{"charter"}, nil
	}
	resp, err := a.templates.Evaluate(a.ctx, templates.SeedRequest{
		Industry:    industry,
		Methodology: methodology,
	})
	if err != nil {
		return nil, err
	}
	return resp.Seeds, nil
}

// CreateProjectFromLaunchpad creates a new .pmforge file just like
// CreateProject, then applies the seed actions returned by the
// templates engine. The receipts slice records what was created so
// the GUI can show the user a "we set up the following for you" toast.
// LaunchpadResult is the single-object result of CreateProjectFromLaunchpad.
// Returned as one struct (not multiple values) so the Wails bridge marshals
// it to a JS object with named fields, which the frontend reads as
// `res.project` / `res.path` instead of destructuring an array (a null
// array result silently broke project creation in the UI).
type LaunchpadResult struct {
	Project db.Project              `json:"project"`
	Seeds   []templates.SeedReceipt `json:"seeds"`
	Path    string                  `json:"path"`
}

func (a *App) CreateProjectFromLaunchpad(
	name, description, industry, subCategory, methodology, countryCode string,
	seeds []string,
) (LaunchpadResult, error) {
	user := a.requireUser()
	if user == nil {
		return LaunchpadResult{}, errors.New("not signed in")
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		return LaunchpadResult{}, errors.New("invalid project name")
	}
	dir := filepath.Join(user.DataDir, "projects")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return LaunchpadResult{}, err
	}
	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return LaunchpadResult{}, err
	}

	// Each project gets its own uniquely-named, time-stamped subfolder
	// (same scheme as CreateProject).
	path, err := newProjectPath(dir, safe)
	if err != nil {
		return LaunchpadResult{}, err
	}

	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		return LaunchpadResult{}, err
	}
	// We close the local handle at the end and rely on OpenProject
	// to install the project as the app's active one, so the flow
	// matches what the user expects after CreateProject.
	proj, err := d.UpsertProject(db.Project{
		Name:        name,
		Description: description,
		Status:      "planning",
		Phase:       "initiation",
		Owner:       user.Username,
		Industry:    industry,
		SubCategory: subCategory,
		Methodology: methodology,
		CountryCode: countryCodeOrDefault(countryCode),
		TimeZone:    calendar.DefaultTimeZone(countryCodeOrDefault(countryCode)),
	})
	if err != nil {
		_ = d.Close()
		return LaunchpadResult{}, err
	}
	a.applyGlobalDefaults(d)

	// Apply seeds via the dedicated seeder.
	seeder := templates.NewSeeder(d, proj.ID)
	receipts, seedErr := seeder.Apply(seeds)
	_ = d.Close()

	// Install as the active project now so that dashboard operations
	// (opening charts, documents, etc.) work immediately after creation
	// without requiring a separate OpenProject call from the frontend.
	if _, openErr := a.OpenProject(path); openErr != nil {
		return LaunchpadResult{Project: proj, Seeds: receipts, Path: path},
			fmt.Errorf("project created but could not activate: %w", openErr)
	}

	// Even on seedErr we keep the project — the user can fix it
	// from the dashboard. Bubble the error up so the GUI shows a
	// notice.
	if seedErr != nil {
		return LaunchpadResult{Project: proj, Seeds: receipts, Path: path},
			fmt.Errorf("project created but seeding partial: %w", seedErr)
	}
	return LaunchpadResult{Project: proj, Seeds: receipts, Path: path}, nil
}

// UpdateProjectIndustry persists changes to industry / sub-category /
// methodology / country code on an already-open project. Used by the
// project Settings view if the user reclassifies later.
func (a *App) UpdateProjectIndustry(industry, subCategory, methodology, countryCode string) (db.Project, error) {
	d := a.requireDB()
	if d == nil {
		return db.Project{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Project{}, err
	}
	p.Industry = industry
	p.SubCategory = subCategory
	p.Methodology = methodology
	p.CountryCode = countryCodeOrDefault(countryCode)
	if !calendar.ValidTimeZone(p.CountryCode, p.TimeZone) {
		p.TimeZone = calendar.DefaultTimeZone(p.CountryCode)
	}
	return d.UpsertProject(p)
}

// ----- Stakeholders -----

// ListStakeholders returns every stakeholder for the open project,
// optionally filtered by category ("team" / "vendor" / "sponsor" /
// "external").
func (a *App) ListStakeholders(category string) ([]db.Stakeholder, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListStakeholders(p.ID, category)
}

// SaveStakeholder upserts a stakeholder. Empty ID creates one.
func (a *App) SaveStakeholder(s db.Stakeholder) (db.Stakeholder, error) {
	d := a.requireDB()
	if d == nil {
		return db.Stakeholder{}, errors.New("no project open")
	}
	if s.ProjectID == "" {
		p, err := d.GetProject()
		if err != nil {
			return db.Stakeholder{}, err
		}
		s.ProjectID = p.ID
	}
	return d.SaveStakeholder(s)
}

// DeleteStakeholder removes a stakeholder.
func (a *App) DeleteStakeholder(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteStakeholder(id)
}

// ----- Resource calendars -----

// ListResourceCalendars returns every named resource-capacity calendar
// for the open project.
func (a *App) ListResourceCalendars() ([]db.ResourceCalendar, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListResourceCalendars(p.ID)
}

// SaveResourceCalendar upserts a resource-capacity calendar for the
// open project. The current project ID always wins over frontend input.
func (a *App) SaveResourceCalendar(c db.ResourceCalendar) (db.ResourceCalendar, error) {
	d := a.requireDB()
	if d == nil {
		return db.ResourceCalendar{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.ResourceCalendar{}, err
	}
	c.ProjectID = p.ID
	return d.SaveResourceCalendar(c)
}

// DeleteResourceCalendar removes a named resource-capacity calendar.
func (a *App) DeleteResourceCalendar(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	return d.DeleteResourceCalendar(id)
}

// ----- Scenarios / what-if analysis -----

// ListScenarios returns what-if scenario metadata for the open project.
func (a *App) ListScenarios() ([]db.Scenario, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return d.ListScenarios(p.ID)
}

// GetScenario fetches one scenario by ID for the open project.
func (a *App) GetScenario(id string) (db.Scenario, error) {
	d := a.requireDB()
	if d == nil {
		return db.Scenario{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Scenario{}, err
	}
	s, err := d.GetScenario(id)
	if err != nil {
		return db.Scenario{}, err
	}
	if s.ProjectID != p.ID {
		return db.Scenario{}, db.ErrNoScenario
	}
	return s, nil
}

// SaveScenario upserts scenario metadata for the open project. The
// frontend-supplied project ID is ignored so scenarios cannot be
// written across project boundaries.
func (a *App) SaveScenario(s db.Scenario) (db.Scenario, error) {
	d := a.requireDB()
	if d == nil {
		return db.Scenario{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Scenario{}, err
	}
	s.ProjectID = p.ID
	return d.SaveScenario(s)
}

// DeleteScenario removes one scenario from the open project.
func (a *App) DeleteScenario(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return err
	}
	s, err := d.GetScenario(id)
	if err != nil {
		return err
	}
	if s.ProjectID != p.ID {
		return db.ErrNoScenario
	}
	return d.DeleteScenario(id)
}

// BranchScenarioChart copies a live chart and optional baseline into an
// isolated scenario partition for the open project.
func (a *App) BranchScenarioChart(scenarioID, chartID, baselineID string) (db.ScenarioChart, error) {
	d := a.requireDB()
	if d == nil {
		return db.ScenarioChart{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.ScenarioChart{}, err
	}
	s, err := d.GetScenario(scenarioID)
	if err != nil {
		return db.ScenarioChart{}, err
	}
	if s.ProjectID != p.ID {
		return db.ScenarioChart{}, db.ErrNoScenario
	}
	return d.BranchScenarioChart(scenarioID, chartID, baselineID)
}

// ListScenarioCharts returns isolated chart copies for a scenario in the
// open project.
func (a *App) ListScenarioCharts(scenarioID string) ([]db.ScenarioChart, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	s, err := d.GetScenario(scenarioID)
	if err != nil {
		return nil, err
	}
	if s.ProjectID != p.ID {
		return nil, db.ErrNoScenario
	}
	return d.ListScenarioCharts(scenarioID)
}

// GetScenarioChart fetches one isolated scenario chart copy in the open
// project.
func (a *App) GetScenarioChart(id string) (db.ScenarioChart, error) {
	d := a.requireDB()
	if d == nil {
		return db.ScenarioChart{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.ScenarioChart{}, err
	}
	c, err := d.GetScenarioChart(id)
	if err != nil {
		return db.ScenarioChart{}, err
	}
	if c.ProjectID != p.ID {
		return db.ScenarioChart{}, db.ErrNoScenarioChart
	}
	return c, nil
}

// SaveScenarioChart updates the editable fields of an isolated scenario
// chart copy in the open project.
func (a *App) SaveScenarioChart(c db.ScenarioChart) (db.ScenarioChart, error) {
	d := a.requireDB()
	if d == nil {
		return db.ScenarioChart{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.ScenarioChart{}, err
	}
	existing, err := d.GetScenarioChart(c.ID)
	if err != nil {
		return db.ScenarioChart{}, err
	}
	if existing.ProjectID != p.ID {
		return db.ScenarioChart{}, db.ErrNoScenarioChart
	}
	return d.SaveScenarioChart(c)
}

// PromoteScenarioChartToBaseline approves an isolated scenario chart copy
// by saving its current data as a named baseline on the source chart.
func (a *App) PromoteScenarioChartToBaseline(scenarioChartID, name string) (db.Baseline, error) {
	d := a.requireDB()
	if d == nil {
		return db.Baseline{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return db.Baseline{}, err
	}
	scenarioChart, err := d.GetScenarioChart(scenarioChartID)
	if err != nil {
		return db.Baseline{}, err
	}
	if scenarioChart.ProjectID != p.ID {
		return db.Baseline{}, db.ErrNoScenarioChart
	}
	return d.PromoteScenarioChartToBaseline(scenarioChartID, name)
}

// CompareScenarioChart diffs an isolated scenario chart copy against the
// baseline snapshot captured with that copy.
func (a *App) CompareScenarioChart(scenarioChartID string) (map[string]kernel.ScheduleVariance, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	scenarioChart, err := d.GetScenarioChart(scenarioChartID)
	if err != nil {
		return nil, err
	}
	if scenarioChart.ProjectID != p.ID {
		return nil, db.ErrNoScenarioChart
	}
	if scenarioChart.BaselineData == "" || scenarioChart.BaselineData == "{}" {
		return map[string]kernel.ScheduleVariance{}, nil
	}
	baseline := make(map[string]*kernel.Task)
	if err := json.Unmarshal([]byte(scenarioChart.BaselineData), &baseline); err != nil {
		return nil, fmt.Errorf("scenario baseline %s is corrupt: %w", scenarioChart.ID, err)
	}
	current, err := cpmChartDataToKernelTasks(scenarioChart.Data)
	if err != nil {
		return nil, err
	}
	scheduleProjectTasks(p, current)
	return kernel.CompareSchedules(current, baseline), nil
}

// ----- Timeline + Budget -----

var errTimelineSourceMismatch = errors.New("timeline: source id does not match open project")

// BuildTimeline returns the project's chronological event stream
// (project start/end, sprint start/end, deployments).
func (a *App) BuildTimeline() ([]timeline.Entry, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	return buildTimelineFromDB(d)
}

// MoveTimelineEntry updates an editable timeline boundary and returns
// the rebuilt event stream. Only project and sprint date boundaries are
// editable from the Timeline view; deployments remain immutable DORA
// history and must be edited through the deployment log.
func (a *App) MoveTimelineEntry(kind, sourceID, dateISO string) ([]timeline.Entry, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	date, err := normaliseTimelineMoveDate(dateISO)
	if err != nil {
		return nil, err
	}

	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	store := agile.NewStore(d.Conn, p.ID)

	switch timeline.EntryKind(kind) {
	case timeline.KindProjectStart:
		if sourceID != "" && sourceID != p.ID {
			return nil, errTimelineSourceMismatch
		}
		p.StartDate = date
		if err := validateTimelineDateRange(p.StartDate, p.EndDate, "project"); err != nil {
			return nil, err
		}
		if _, err := d.UpsertProject(p); err != nil {
			return nil, err
		}
	case timeline.KindProjectEnd:
		if sourceID != "" && sourceID != p.ID {
			return nil, errTimelineSourceMismatch
		}
		p.EndDate = date
		if err := validateTimelineDateRange(p.StartDate, p.EndDate, "project"); err != nil {
			return nil, err
		}
		if _, err := d.UpsertProject(p); err != nil {
			return nil, err
		}
	case timeline.KindSprintStart, timeline.KindSprintEnd:
		if sourceID == "" {
			return nil, errors.New("timeline: sprint move requires a source id")
		}
		sp, err := store.GetSprint(sourceID)
		if err != nil {
			return nil, err
		}
		if timeline.EntryKind(kind) == timeline.KindSprintStart {
			sp.StartDate = date
		} else {
			sp.EndDate = date
		}
		if err := validateTimelineDateRange(sp.StartDate, sp.EndDate, "sprint"); err != nil {
			return nil, err
		}
		if _, err := store.SaveSprint(sp); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("timeline: %s entries are read-only", kind)
	}

	return buildTimelineFromDB(d)
}

func buildTimelineFromDB(d *db.Database) ([]timeline.Entry, error) {
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	store := agile.NewStore(d.Conn, p.ID)
	sprints, err := store.ListSprints()
	if err != nil {
		return nil, err
	}
	deploys, err := store.ListDeployments(time.Time{})
	if err != nil {
		return nil, err
	}
	return timeline.Build(p, sprints, deploys), nil
}

func normaliseTimelineMoveDate(dateISO string) (string, error) {
	t, err := time.Parse("2006-01-02", dateISO)
	if err != nil {
		return "", fmt.Errorf("timeline: date must be YYYY-MM-DD: %w", err)
	}
	return t.Format("2006-01-02"), nil
}

func validateTimelineDateRange(start, end, label string) error {
	if start == "" || end == "" {
		return nil
	}
	startDate, err := time.Parse("2006-01-02", start)
	if err != nil {
		return fmt.Errorf("timeline: %s start date is invalid: %w", label, err)
	}
	endDate, err := time.Parse("2006-01-02", end)
	if err != nil {
		return fmt.Errorf("timeline: %s end date is invalid: %w", label, err)
	}
	if endDate.Before(startDate) {
		return fmt.Errorf("timeline: %s end date cannot be before start date", label)
	}
	return nil
}

// ListHolidays returns the holidays between `fromISO` and `toISO`
// for the open project's country.
func (a *App) ListHolidays(fromISO, toISO string) ([]calendar.HolidayEvent, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	from, err := time.Parse("2006-01-02", fromISO)
	if err != nil {
		return nil, fmt.Errorf("from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", toISO)
	if err != nil {
		return nil, fmt.Errorf("to date: %w", err)
	}
	c := calendar.For(p.CountryCode)
	return c.HolidaysIn(from, to), nil
}

// ComputeBudget rolls up the project's cost picture from stakeholder
// rates × work-item points + vendor contract values.
func (a *App) ComputeBudget() (budget.Summary, error) {
	d := a.requireDB()
	if d == nil {
		return budget.Summary{}, errors.New("no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return budget.Summary{}, err
	}
	stakeholders, err := d.ListStakeholders(p.ID, "")
	if err != nil {
		return budget.Summary{}, err
	}
	store := agile.NewStore(d.Conn, p.ID)
	workItems, err := store.ListWorkItems("", "", "")
	if err != nil {
		return budget.Summary{}, err
	}
	return budget.Compute(p, stakeholders, workItems), nil
}

// RunPortfolioAnalytics aggregates a cross-project cost rollup over every
// readable project in the signed-in user's folder using the embedded
// DuckDB analytics engine (ADR-002 Option B). The engine is in-memory and
// ephemeral and never opens the encrypted files: this method reads each
// project with the session DEK, builds the per-project figures in Go, and
// passes them in. Production/package builds include the `duckdb` tag; an
// untagged developer build still returns analytics.ErrAnalyticsUnavailable.
//
// Per-project actual cost is the budget "committed" total (vendor
// contracts + labour estimate); earned/planned value (EVM) aggregation is
// a later enhancement, so SPI/CPI report 0 ("n/a") for now.
func (a *App) RunPortfolioAnalytics() (analytics.PortfolioSummary, error) {
	user := a.requireUser()
	if user == nil {
		return analytics.PortfolioSummary{}, errors.New("not signed in")
	}

	eng := analytics.New()
	defer func() { _ = eng.Close() }()
	if !eng.Available() {
		// Default build: skip the (expensive) project scan entirely.
		return analytics.PortfolioSummary{}, analytics.ErrAnalyticsUnavailable
	}

	a.mu.RLock()
	dek, err := a.requireDEKLocked()
	a.mu.RUnlock()
	if err != nil {
		return analytics.PortfolioSummary{}, err
	}

	dir := filepath.Join(user.DataDir, "projects")
	entries, err := enumerateProjects(dir)
	if err != nil {
		return analytics.PortfolioSummary{}, err
	}

	metrics := make([]analytics.ProjectMetrics, 0, len(entries))
	for _, e := range entries {
		d, derr := db.InitEncryptedDB(e.Path, dek)
		if derr != nil {
			continue // unreadable project: skip, matching ProjectsOverview
		}
		p, perr := d.GetProject()
		if perr != nil {
			_ = d.Close()
			continue
		}
		var committed float64
		var committedMinorUnits int64
		if sks, serr := d.ListStakeholders(p.ID, ""); serr == nil {
			wis, _ := agile.NewStore(d.Conn, p.ID).ListWorkItems("", "", "")
			summary := budget.Compute(p, sks, wis)
			committed = summary.Committed
			committedMinorUnits = summary.CommittedMinorUnits
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = e.Name
		}
		metrics = append(metrics, analytics.ProjectMetrics{
			ProjectID:              p.ID,
			Name:                   name,
			BudgetedCost:           p.Budget,
			ActualCost:             committed,
			BudgetedCostMinorUnits: p.BudgetMinorUnits,
			ActualCostMinorUnits:   committedMinorUnits,
		})
		_ = d.Close()
	}

	return eng.PortfolioRollup(a.ctx, metrics)
}

// ImportDatasetForAnalysis opens a native file picker for a CSV/Parquet/JSON
// file and reads it into an in-memory Dataset via the DuckDB analytics engine
// (ADR-002 Option B). Returns an empty Dataset (no error) when the user
// cancels. Production/package builds include DuckDB; untagged developer builds
// return analytics.ErrAnalyticsUnavailable. `.xlsx` is not handled here — the
// Sigma import uses the frontend read-excel-file reader.
func (a *App) ImportDatasetForAnalysis() (analytics.Dataset, error) {
	if a.requireUser() == nil {
		return analytics.Dataset{}, errors.New("not signed in")
	}
	if a.ctx == nil {
		return analytics.Dataset{}, errors.New("no context (Wails not started)")
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Select a data file to analyze",
		DefaultDirectory: a.userDir(),
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Data files (*.csv, *.tsv, *.parquet, *.json)",
				Pattern:     "*.csv;*.tsv;*.parquet;*.json",
			},
		},
	})
	if err != nil {
		return analytics.Dataset{}, err
	}
	if path == "" {
		return analytics.Dataset{}, nil // user cancelled the picker
	}

	eng := analytics.New()
	defer func() { _ = eng.Close() }()
	if !eng.Available() {
		return analytics.Dataset{}, analytics.ErrAnalyticsUnavailable
	}
	return eng.ImportTabular(a.ctx, path)
}

// ----- iCal export -----

// CheckLatestVersion runs the signed-manifest update check. The
// frontend calls this from the Settings panel; the result is a
// Status struct describing whether an update is available.
func (a *App) CheckLatestVersion() (update.Status, error) {
	return update.CheckLatest(a.ctx)
}

// ChooseCertFile opens a native file-picker for X.509 certificate
// bundles (.p12 / .pfx). Returns the absolute path the user picked
// or empty string on cancel. Used by SignatureSettings.svelte.
func (a *App) ChooseCertFile() (string, error) {
	if a.ctx == nil {
		return "", errors.New("no context (Wails not started)")
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Select signing certificate",
		DefaultDirectory: a.userDir(),
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "PKCS#12 bundles (*.p12, *.pfx)",
				Pattern:     "*.p12;*.pfx",
			},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}
