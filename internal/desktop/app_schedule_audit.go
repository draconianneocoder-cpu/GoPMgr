// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gopmgr/internal/calendar"
	"gopmgr/internal/charts"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/kernel"
	"gopmgr/internal/reporting"
	"gopmgr/internal/schedaudit"
)

// auditProfiles maps the profile keys AuditSchedule accepts to their
// profiles. The keys are stable identifiers; a profile's Name is display
// text and is not accepted.
var auditProfiles = map[string]func() schedaudit.Profile{
	"":           schedaudit.DefaultProfile,
	"default":    schedaudit.DefaultProfile,
	"dcma-style": schedaudit.DCMAStyleProfile,
}

// AuditSchedule runs the PS-02 schedule-quality audit on one CPM chart of
// the open project and returns its findings. It reads only: nothing is
// saved, repaired or logged.
//
// It audits exactly the chart named by chartID, never the project's newest
// chart, and labels the result with that chart's ID and revision (its
// updated-at stamp). The audit schedules a copy of the chart's tasks the way
// reports do: date constraints armed against the project start date and the
// country work calendar, and resource demand checked against the project's
// capacity plan.
//
// reportingDate is the status date as YYYY-MM-DD, or empty for none.
// profile is "default" (or empty) or "dcma-style". ctx is the frontend
// call's own context, so cancelling the call stops the audit.
func (a *App) AuditSchedule(ctx context.Context, chartID, reportingDate, profile string) (schedaudit.Result, error) {
	d := a.requireDB()
	if d == nil {
		return schedaudit.Result{}, errors.New("no project open")
	}
	r, err := runChartAudit(ctx, d, chartID, reportingDate, profile)
	if err != nil {
		return schedaudit.Result{}, err
	}
	return r.Result, nil
}

// reportAuditResolver runs the schedule-audit sections of a combined report
// against the open project, with the same inputs as AuditSchedule.
func reportAuditResolver(d *db.Database) reporting.AuditResolver {
	return func(ctx context.Context, section documents.ScheduleAuditSection) (reporting.AuditResolution, error) {
		return runChartAudit(ctx, d, section.ChartID, section.ReportingDate, section.Profile)
	}
}

// runChartAudit is AuditSchedule's body. Besides the result it returns the
// inputs it read, so a report can record what its findings were computed
// from. The project, chart, baselines and capacity plan are separate reads,
// not one transaction.
func runChartAudit(ctx context.Context, d *db.Database, chartID, reportingDate, profile string) (reporting.AuditResolution, error) {
	newProfile, ok := auditProfiles[profile]
	if !ok {
		return reporting.AuditResolution{}, fmt.Errorf("unknown audit profile %q: use \"default\" or \"dcma-style\"", profile)
	}
	if reportingDate != "" {
		if _, err := time.Parse(kernel.DateLayout, reportingDate); err != nil {
			return reporting.AuditResolution{}, fmt.Errorf("reporting date %q is not a YYYY-MM-DD date", reportingDate)
		}
	}
	proj, err := d.GetProject()
	if err != nil {
		return reporting.AuditResolution{}, err
	}
	c, err := chartForProject(d, proj.ID, chartID)
	if err != nil {
		return reporting.AuditResolution{}, err
	}
	if c.Kind != string(charts.KindCPM) {
		return reporting.AuditResolution{}, fmt.Errorf("the schedule audit needs a CPM chart, got %q", c.Kind)
	}
	byID, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return reporting.AuditResolution{}, err
	}
	if len(byID) == 0 {
		return reporting.AuditResolution{}, errors.New("chart has no tasks")
	}
	tasks := make([]kernel.Task, 0, len(byID))
	for _, t := range byID {
		tasks = append(tasks, *t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })

	baselines, err := d.ListBaselines(c.ID)
	if err != nil {
		return reporting.AuditResolution{}, err
	}
	start := auditProjectStart(proj.StartDate)
	capacity := resourceCapacityPlan(d, proj.ID)
	capacityJSON, err := json.Marshal(capacity)
	if err != nil {
		return reporting.AuditResolution{}, fmt.Errorf("encode capacity plan: %w", err)
	}
	result, err := schedaudit.Audit(ctx, schedaudit.Input{
		Tasks:             tasks,
		ProjectStart:      start,
		IsWorkday:         calendar.For(proj.CountryCode).IsWorkday,
		ReportingDate:     reportingDate,
		BaselineAvailable: len(baselines) > 0,
		Capacity:          capacity,
		Source:            schedaudit.Source{ChartID: c.ID, Revision: c.UpdatedAt},
		Profile:           newProfile(),
	})
	if err != nil {
		return reporting.AuditResolution{}, fmt.Errorf("audit schedule: %w", err)
	}
	baselineIDs := make([]string, 0, len(baselines))
	for _, b := range baselines {
		baselineIDs = append(baselineIDs, b.ID)
	}
	return reporting.AuditResolution{
		Result:         result,
		ChartTitle:     c.Title,
		ChartSHA256:    fmt.Sprintf("%x", sha256.Sum256([]byte(c.Data+"\n"+c.Config))),
		ProjectStart:   start,
		CountryCode:    proj.CountryCode,
		CapacitySHA256: fmt.Sprintf("%x", sha256.Sum256(capacityJSON)),
		BaselineIDs:    baselineIDs,
	}, nil
}

// auditProjectStart gives the audit the project's start as the calendar
// date the app displays (AnchorSchedule formats dates in the start's own
// zone), or "" when the stored value is not a date. The stored value may be
// RFC3339 with a time and offset, which the audit does not accept. The
// kernel reads it the same way (kernel.DayOffset, KRN-02).
func auditProjectStart(stored string) string {
	day, ok := parseProjectDate(stored)
	if !ok {
		return ""
	}
	return day.Format(kernel.DateLayout)
}
