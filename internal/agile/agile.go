// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package agile implements PMForge's Agile / Software-Dev Pack:
// Kanban boards, sprints, work items, and DORA metrics.
//
// The pack is opt-in via the --software-dev-pack CLI flag (and the
// equivalent runtime setting). When disabled, the GUI hides every
// agile entry point but the tables remain so a re-enable is loss-
// less. PackEnabled tracks the in-process toggle.
package agile

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// PackEnabled is the in-memory toggle. main.go flips it true when
// the CLI flag is set or the user enables the pack from settings.
var PackEnabled bool

// ----- Domain types -----

// WorkItemType is one of the canonical agile work-item categories.
type WorkItemType string

const (
	WorkItemStory WorkItemType = "story"
	WorkItemBug   WorkItemType = "bug"
	WorkItemTask  WorkItemType = "task"
	WorkItemEpic  WorkItemType = "epic"
)

// Priority enumerates the priority tiers the GUI offers.
type Priority string

const (
	PrioLow     Priority = "low"
	PrioMedium  Priority = "medium"
	PrioHigh    Priority = "high"
	PrioUrgent  Priority = "urgent"
)

// WorkItem is one story / bug / task / epic on the board or backlog.
type WorkItem struct {
	ID          string       `json:"id"`
	ProjectID   string       `json:"project_id"`
	Type        WorkItemType `json:"type"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	State       string       `json:"state"`     // column ID, or "backlog"
	Points      float64      `json:"points"`    // estimate, story points
	Assignee    string       `json:"assignee"`
	SprintID    string       `json:"sprint_id"` // empty == not in a sprint
	Priority    Priority     `json:"priority"`
	OrderIdx    int          `json:"order_idx"` // position within its column / backlog
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ClosedAt    time.Time    `json:"closed_at,omitempty"`
}

// Column is one Kanban column. The column's ID is also the state
// value stored on work items, so a column's name can change without
// touching every item.
type Column struct {
	ID       string `json:"id"`
	BoardID  string `json:"board_id"`
	Name     string `json:"name"`
	OrderIdx int    `json:"order_idx"`
	WIPLimit int    `json:"wip_limit"` // 0 = unlimited
}

// Board groups columns. Most projects have a single default board;
// future versions may support per-team boards.
type Board struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SprintStatus is the canonical sprint lifecycle.
type SprintStatus string

const (
	SprintPlanning SprintStatus = "planning"
	SprintActive   SprintStatus = "active"
	SprintComplete SprintStatus = "complete"
)

// Sprint is a time-boxed iteration.
type Sprint struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"project_id"`
	Name      string       `json:"name"`
	Goal      string       `json:"goal"`
	Status    SprintStatus `json:"status"`
	StartDate string       `json:"start_date"`
	EndDate   string       `json:"end_date"`
	Capacity  float64      `json:"capacity"` // committed story points
	CreatedAt time.Time    `json:"created_at"`
}

// Deployment is one push to production, used to compute DORA metrics.
type Deployment struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	TS               time.Time `json:"ts"`
	Version          string    `json:"version"`
	Successful       bool      `json:"successful"`
	LeadTimeHours    float64   `json:"lead_time_hours"`    // commit-to-prod
	RestoreTimeHours float64   `json:"restore_time_hours"` // failure-to-restore (0 if not a failure)
	Notes            string    `json:"notes"`
}

// ----- ID generation -----

// newID returns a short, URL-safe identifier prefixed with `prefix`.
// Mirrors db.newID so the agile package can issue IDs without a db
// import cycle.
func newID(prefix string) string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return prefix + "_" + hex.EncodeToString(buf[:])
}

// NewBoardID, NewColumnID, etc. are sugar so call sites read clearly.
func NewBoardID() string      { return newID("board") }
func NewColumnID() string     { return newID("col") }
func NewWorkItemID() string   { return newID("wi") }
func NewSprintID() string     { return newID("sprint") }
func NewDeploymentID() string { return newID("deploy") }

// DefaultColumns returns the columns PMForge seeds a brand-new
// board with. The IDs are stable strings (rather than newID() calls)
// so saved work-item states render correctly even before the user
// has a board open.
func DefaultColumns(boardID string) []Column {
	return []Column{
		{ID: "todo", BoardID: boardID, Name: "To Do", OrderIdx: 0, WIPLimit: 0},
		{ID: "doing", BoardID: boardID, Name: "In Progress", OrderIdx: 1, WIPLimit: 3},
		{ID: "review", BoardID: boardID, Name: "Review", OrderIdx: 2, WIPLimit: 3},
		{ID: "done", BoardID: boardID, Name: "Done", OrderIdx: 3, WIPLimit: 0},
	}
}
