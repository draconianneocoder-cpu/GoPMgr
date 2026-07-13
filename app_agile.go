// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"pmforge/internal/agile"
	"time"
)

// =========================================================
// Agile Pack (V2.x — Kanban / Sprints / DORA)
// =========================================================
//
// All methods below build an agile.Store on demand, scoped to the
// currently-open project. Callers MUST have a project open;
// otherwise an "agile: no project" error is returned.

func (a *App) agileStore() (*agile.Store, error) {
	d := a.requireDB()
	if d == nil {
		return nil, errors.New("agile: no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return nil, err
	}
	return agile.NewStore(d.Conn, p.ID), nil
}

// AgileEnabled reports whether the Software-Dev Pack is active for the
// open project. The value is read from settings on each project open and
// cached in agile.PackEnabled for cheap in-process checks.
func (a *App) AgileEnabled() (bool, error) {
	d := a.requireDB()
	if d == nil {
		return agile.PackEnabled.Load(), nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return agile.PackEnabled.Load(), fmt.Errorf("AgileEnabled: %w", err)
	}
	agile.PackEnabled.Store(s.AgileEnabled)
	return s.AgileEnabled, nil
}

// SetAgileEnabled persists the Software-Dev Pack toggle to the project
// settings and updates the in-process cache.
func (a *App) SetAgileEnabled(enabled bool) error {
	agile.PackEnabled.Store(enabled)
	d := a.requireDB()
	if d == nil {
		return nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return fmt.Errorf("SetAgileEnabled: %w", err)
	}
	s.AgileEnabled = enabled
	return d.SaveSettings(s)
}

// EnsureDefaultBoard returns (and creates if missing) the default
// Kanban board for the open project, along with its seeded columns.
// BoardWithColumns is the single-object result of EnsureDefaultBoard.
// Returned as one struct (not multiple values) so the Wails bridge marshals
// it to a JS object with named fields, which the frontend reads as
// `res.board` / `res.columns` instead of destructuring an array.
type BoardWithColumns struct {
	Board   agile.Board    `json:"board"`
	Columns []agile.Column `json:"columns"`
}

func (a *App) EnsureDefaultBoard() (BoardWithColumns, error) {
	s, err := a.agileStore()
	if err != nil {
		return BoardWithColumns{}, err
	}
	b, err := s.EnsureDefaultBoard()
	if err != nil {
		return BoardWithColumns{}, err
	}
	cols, err := s.ListColumns(b.ID)
	if err != nil {
		return BoardWithColumns{}, err
	}
	return BoardWithColumns{Board: b, Columns: cols}, nil
}

// SaveColumn upserts a column (rename, change WIP, reorder).
func (a *App) SaveColumn(c agile.Column) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.SaveColumn(c)
}

// DeleteColumn removes a column. The frontend warns about
// re-homing work items before calling this.
func (a *App) DeleteColumn(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteColumn(id)
}

// SaveWorkItem inserts or updates a work item.
func (a *App) SaveWorkItem(wi agile.WorkItem) (agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.WorkItem{}, err
	}
	return s.SaveWorkItem(wi)
}

// GetWorkItem fetches one by ID.
func (a *App) GetWorkItem(id string) (agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.WorkItem{}, err
	}
	return s.GetWorkItem(id)
}

// ListWorkItems returns the project's work items, optionally
// filtered by sprintID, state (column ID), and assignee. Pass
// empty strings to disable a filter.
func (a *App) ListWorkItems(sprintID, state, assignee string) ([]agile.WorkItem, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.ListWorkItems(sprintID, state, assignee)
}

// DeleteWorkItem removes a work item.
func (a *App) DeleteWorkItem(id string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	actor := "unknown"
	if u := a.requireUser(); u != nil {
		actor = u.Username
	}
	_ = d.LogAction(actor, "delete_work_item", id, "")
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteWorkItem(id)
}

// MoveWorkItem is the Kanban drag-and-drop hook: change a work
// item's state (= destination column ID) and its order within that
// column atomically.
func (a *App) MoveWorkItem(id, newState string, newOrder int) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.MoveWorkItem(id, newState, newOrder)
}

// WIPCounts returns the current count of work items per column,
// for the WIP-breach indicators on the Kanban board.
func (a *App) WIPCounts() (map[string]int, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.WIPCountByColumn()
}

// SaveSprint upserts a sprint.
func (a *App) SaveSprint(sp agile.Sprint) (agile.Sprint, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.Sprint{}, err
	}
	return s.SaveSprint(sp)
}

// ListSprints returns every sprint for the open project.
func (a *App) ListSprints() ([]agile.Sprint, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	return s.ListSprints()
}

// DeleteSprint removes a sprint and unlinks its work items
// (transactionally).
func (a *App) DeleteSprint(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteSprint(id)
}

// SaveDeployment upserts a deployment record (feeds DORA metrics).
func (a *App) SaveDeployment(d agile.Deployment) (agile.Deployment, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.Deployment{}, err
	}
	return s.SaveDeployment(d)
}

// ListDeployments returns deployments newer than `sinceISO` (RFC3339
// timestamp). Pass "" for all deployments.
func (a *App) ListDeployments(sinceISO string) ([]agile.Deployment, error) {
	s, err := a.agileStore()
	if err != nil {
		return nil, err
	}
	var since time.Time
	if sinceISO != "" {
		if t, err := time.Parse(time.RFC3339, sinceISO); err == nil {
			since = t
		}
	}
	return s.ListDeployments(since)
}

// DeleteDeployment removes a deployment record.
func (a *App) DeleteDeployment(id string) error {
	s, err := a.agileStore()
	if err != nil {
		return err
	}
	return s.DeleteDeployment(id)
}

// =========================================================
// (back to existing Agile methods)
// =========================================================

// ComputeDORA runs the four DORA metrics over the last `windowDays`
// of deployments. windowDays <= 0 defaults to 30.
func (a *App) ComputeDORA(windowDays int) (agile.DORAResult, error) {
	s, err := a.agileStore()
	if err != nil {
		return agile.DORAResult{}, err
	}
	since := time.Now().AddDate(0, 0, -windowDays)
	deploys, err := s.ListDeployments(since)
	if err != nil {
		return agile.DORAResult{}, err
	}
	return agile.ComputeDORA(deploys, windowDays, time.Now().UTC()), nil
}
