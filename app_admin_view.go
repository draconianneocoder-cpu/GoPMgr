// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"sort"

	"gopmgr/internal/charts"
	"gopmgr/internal/db"
)

// =========================================================
// Read-only viewer for recorded administrator access (ADR-004)
// =========================================================
//
// Every AdminView* method reads the project copy through withAdminView,
// which re-checks the caller's role first. They return what the viewer
// shows (owner decision, 2026-10-02): the schedule, costs, and the list of
// documents, never document contents or cost attachments, and nothing that
// writes or exports.

// withAdminView runs fn with the project copy being viewed and its project
// row, after re-checking that the caller is still an enabled
// administrator. The handle is taken under the lock and used after it is
// released, so fn may call methods that lock a.mu; a concurrent stop
// closes the handle and fn's query fails with "database is closed".
func (a *App) withAdminView(fn func(view *db.Database, proj db.Project) error) error {
	if err := a.checkAccessCaller(); err != nil {
		return err
	}
	a.mu.RLock()
	var view *db.Database
	if a.access != nil {
		view = a.access.view
	}
	a.mu.RUnlock()
	if view == nil {
		return errors.New("no project is open for viewing")
	}
	proj, err := view.GetProject()
	if err != nil {
		return err
	}
	return fn(view, proj)
}

// AdminViewScheduleWire is one schedule (a Gantt or CPM chart) of the
// project being viewed. Note explains a schedule that could not be read.
type AdminViewScheduleWire struct {
	ChartID string              `json:"chart_id"`
	Title   string              `json:"title"`
	Kind    string              `json:"kind"`
	Tasks   []AdminViewTaskWire `json:"tasks"`
	Note    string              `json:"note"`
}

// AdminViewTaskWire is one scheduled task.
type AdminViewTaskWire struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	StartDate       string  `json:"start_date"`
	FinishDate      string  `json:"finish_date"`
	Duration        float64 `json:"duration"`
	PercentComplete float64 `json:"percent_complete"`
	Milestone       bool    `json:"milestone"`
	Critical        bool    `json:"critical"`
}

// AdminViewSchedule returns every schedule in the project being viewed,
// with each task's dates computed as the project's own reports compute
// them. A schedule whose data cannot be read is returned with a note
// rather than failing the rest.
func (a *App) AdminViewSchedule() ([]AdminViewScheduleWire, error) {
	var out []AdminViewScheduleWire
	err := a.withAdminView(func(view *db.Database, proj db.Project) error {
		all, err := view.ListCharts(proj.ID, "")
		if err != nil {
			return err
		}
		for _, c := range all {
			kind := charts.Kind(c.Kind)
			if kind != charts.KindGantt && kind != charts.KindCPM {
				continue
			}
			sched := AdminViewScheduleWire{ChartID: c.ID, Title: c.Title, Kind: c.Kind, Tasks: []AdminViewTaskWire{}}
			tasks, err := cpmChartDataToKernelTasks(c.Data)
			if err != nil {
				sched.Note = "This schedule's data could not be read."
				out = append(out, sched)
				continue
			}
			scheduleProjectTasks(proj, tasks)
			for _, t := range tasks {
				sched.Tasks = append(sched.Tasks, AdminViewTaskWire{
					ID: t.ID, Title: t.Title, StartDate: t.StartDate, FinishDate: t.FinishDate,
					Duration: t.Duration, PercentComplete: t.PercentComplete,
					Milestone: t.Milestone, Critical: t.IsCritical,
				})
			}
			sort.SliceStable(sched.Tasks, func(i, j int) bool {
				ti, tj := tasks[sched.Tasks[i].ID], tasks[sched.Tasks[j].ID]
				if ti.ES != tj.ES {
					return ti.ES < tj.ES
				}
				return sched.Tasks[i].Title < sched.Tasks[j].Title
			})
			out = append(out, sched)
		}
		return nil
	})
	if out == nil && err == nil {
		out = []AdminViewScheduleWire{}
	}
	return out, err
}

// AdminViewCostsWire is the cost record of the project being viewed:
// entries and approved cost baselines. Attachments are not included.
type AdminViewCostsWire struct {
	Entries   []CostEntryWire    `json:"entries"`
	Baselines []CostBaselineWire `json:"baselines"`
}

// AdminViewCosts returns the cost entries and cost baselines of the
// project being viewed.
func (a *App) AdminViewCosts() (AdminViewCostsWire, error) {
	out := AdminViewCostsWire{Entries: []CostEntryWire{}, Baselines: []CostBaselineWire{}}
	err := a.withAdminView(func(view *db.Database, proj db.Project) error {
		entries, err := view.ListCostEntries(proj.ID)
		if err != nil {
			return err
		}
		for _, e := range entries {
			out.Entries = append(out.Entries, costEntryWire(e))
		}
		baselines, err := view.ListCostBaselines(proj.ID)
		if err != nil {
			return err
		}
		for _, b := range baselines {
			wire, err := costBaselineWire(b)
			if err != nil {
				return err
			}
			out.Baselines = append(out.Baselines, wire)
		}
		return nil
	})
	return out, err
}

// AdminViewDocumentWire lists one document of the project being viewed:
// what it is, never its contents.
type AdminViewDocumentWire struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Version   int    `json:"version"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// AdminViewDocuments lists the documents of the project being viewed.
func (a *App) AdminViewDocuments() ([]AdminViewDocumentWire, error) {
	out := []AdminViewDocumentWire{}
	err := a.withAdminView(func(view *db.Database, proj db.Project) error {
		docs, err := view.ListDocuments(proj.ID, "")
		if err != nil {
			return err
		}
		for _, d := range docs {
			out = append(out, AdminViewDocumentWire{
				ID: d.ID, Kind: d.Kind, Title: d.Title, Version: d.Version,
				Status: d.Status, UpdatedAt: d.UpdatedAt,
			})
		}
		return nil
	})
	return out, err
}
