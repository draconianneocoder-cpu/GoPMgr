// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gopmgr/internal/calendar"
	"gopmgr/internal/charts"
	"gopmgr/internal/kernel"
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
	newProfile, ok := auditProfiles[profile]
	if !ok {
		return schedaudit.Result{}, fmt.Errorf("unknown audit profile %q: use \"default\" or \"dcma-style\"", profile)
	}
	if reportingDate != "" {
		if _, err := time.Parse(kernel.DateLayout, reportingDate); err != nil {
			return schedaudit.Result{}, fmt.Errorf("reporting date %q is not a YYYY-MM-DD date", reportingDate)
		}
	}
	proj, err := d.GetProject()
	if err != nil {
		return schedaudit.Result{}, err
	}
	c, err := chartForProject(d, proj.ID, chartID)
	if err != nil {
		return schedaudit.Result{}, err
	}
	if c.Kind != string(charts.KindCPM) {
		return schedaudit.Result{}, fmt.Errorf("the schedule audit needs a CPM chart, got %q", c.Kind)
	}
	byID, err := cpmChartDataToKernelTasks(c.Data)
	if err != nil {
		return schedaudit.Result{}, err
	}
	if len(byID) == 0 {
		return schedaudit.Result{}, errors.New("chart has no tasks")
	}
	tasks := make([]kernel.Task, 0, len(byID))
	for _, t := range byID {
		tasks = append(tasks, *t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })

	baselines, err := d.ListBaselines(c.ID)
	if err != nil {
		return schedaudit.Result{}, err
	}
	result, err := schedaudit.Audit(ctx, schedaudit.Input{
		Tasks:             tasks,
		ProjectStart:      auditProjectStart(proj.StartDate),
		IsWorkday:         calendar.For(proj.CountryCode).IsWorkday,
		ReportingDate:     reportingDate,
		BaselineAvailable: len(baselines) > 0,
		Capacity:          resourceCapacityPlan(d, proj.ID),
		Source:            schedaudit.Source{ChartID: c.ID, Revision: c.UpdatedAt},
		Profile:           newProfile(),
	})
	if err != nil {
		return schedaudit.Result{}, fmt.Errorf("audit schedule: %w", err)
	}
	return result, nil
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
