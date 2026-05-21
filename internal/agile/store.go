// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Store wraps a *sql.DB and provides CRUD over the agile_* tables.
// The *sql.DB itself comes from the caller's *db.Database; this
// package accepts the raw handle so we don't create an import cycle.
type Store struct {
	Conn      *sql.DB
	ProjectID string
}

// NewStore binds an agile Store to a database connection and a
// project. ProjectID is required for every read/write because the
// agile tables are project-scoped.
func NewStore(conn *sql.DB, projectID string) *Store {
	return &Store{Conn: conn, ProjectID: projectID}
}

// ----- Boards -----

// EnsureDefaultBoard returns the project's default board, creating
// one (with the standard 4-column layout) if none exists yet.
func (s *Store) EnsureDefaultBoard() (Board, error) {
	var b Board
	var (
		created, updated string
		isDefault        int
	)
	err := s.Conn.QueryRow(
		`SELECT id, project_id, name, is_default, created_at, updated_at
		 FROM agile_boards WHERE project_id = ? AND is_default = 1 LIMIT 1`,
		s.ProjectID,
	).Scan(&b.ID, &b.ProjectID, &b.Name, &isDefault, &created, &updated)

	if err == nil {
		b.IsDefault = isDefault != 0
		b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		b.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		return b, nil
	}
	if err != sql.ErrNoRows {
		return Board{}, err
	}

	// Seed.
	b = Board{
		ID:        NewBoardID(),
		ProjectID: s.ProjectID,
		Name:      "Main board",
		IsDefault: true,
	}
	if _, err := s.Conn.Exec(
		`INSERT INTO agile_boards (id, project_id, name, is_default)
		 VALUES (?, ?, ?, 1)`,
		b.ID, b.ProjectID, b.Name,
	); err != nil {
		return Board{}, err
	}
	for _, c := range DefaultColumns(b.ID) {
		if _, err := s.Conn.Exec(
			`INSERT INTO agile_columns (id, board_id, name, order_idx, wip_limit)
			 VALUES (?, ?, ?, ?, ?)`,
			c.ID, c.BoardID, c.Name, c.OrderIdx, c.WIPLimit,
		); err != nil {
			return Board{}, err
		}
	}
	return s.EnsureDefaultBoard()
}

// ListColumns returns every column on the given board, ordered by
// order_idx.
func (s *Store) ListColumns(boardID string) ([]Column, error) {
	rows, err := s.Conn.Query(
		`SELECT id, board_id, name, order_idx, wip_limit
		 FROM agile_columns WHERE board_id = ? ORDER BY order_idx ASC`,
		boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.OrderIdx, &c.WIPLimit); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveColumn upserts one column. Used to rename a column or change
// its WIP limit.
func (s *Store) SaveColumn(c Column) error {
	if c.ID == "" {
		c.ID = NewColumnID()
	}
	_, err := s.Conn.Exec(`
		INSERT INTO agile_columns (id, board_id, name, order_idx, wip_limit)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name      = excluded.name,
			order_idx = excluded.order_idx,
			wip_limit = excluded.wip_limit
	`, c.ID, c.BoardID, c.Name, c.OrderIdx, c.WIPLimit)
	return err
}

// DeleteColumn removes a column. The caller MUST first re-home or
// delete any work items whose state references this column.
func (s *Store) DeleteColumn(id string) error {
	_, err := s.Conn.Exec(`DELETE FROM agile_columns WHERE id = ?`, id)
	return err
}

// ----- Work items -----

// ErrNoWorkItem indicates a missing work item.
var ErrNoWorkItem = errors.New("agile: work item not found")

// SaveWorkItem inserts or updates a work item. If `wi.State` is
// "done" and `wi.ClosedAt` is zero, the current time is stamped on
// closed_at so DORA can compute lead times.
func (s *Store) SaveWorkItem(wi WorkItem) (WorkItem, error) {
	if wi.ID == "" {
		wi.ID = NewWorkItemID()
	}
	if wi.ProjectID == "" {
		wi.ProjectID = s.ProjectID
	}
	if wi.Type == "" {
		wi.Type = WorkItemStory
	}
	if wi.Priority == "" {
		wi.Priority = PrioMedium
	}
	if wi.State == "" {
		wi.State = "backlog"
	}

	closedAt := ""
	if wi.State == "done" {
		if wi.ClosedAt.IsZero() {
			wi.ClosedAt = time.Now().UTC()
		}
		closedAt = wi.ClosedAt.Format(time.RFC3339Nano)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := s.Conn.Exec(`
		INSERT INTO agile_work_items
			(id, project_id, type, title, description, state, points,
			 assignee, sprint_id, priority, order_idx,
			 created_at, updated_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type        = excluded.type,
			title       = excluded.title,
			description = excluded.description,
			state       = excluded.state,
			points      = excluded.points,
			assignee    = excluded.assignee,
			sprint_id   = excluded.sprint_id,
			priority    = excluded.priority,
			order_idx   = excluded.order_idx,
			updated_at  = excluded.updated_at,
			closed_at   = excluded.closed_at
	`,
		wi.ID, wi.ProjectID, string(wi.Type), wi.Title, wi.Description,
		wi.State, wi.Points, wi.Assignee, wi.SprintID, string(wi.Priority),
		wi.OrderIdx, now, now, closedAt,
	)
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWorkItem(wi.ID)
}

// GetWorkItem fetches by ID.
func (s *Store) GetWorkItem(id string) (WorkItem, error) {
	row := s.Conn.QueryRow(`
		SELECT id, project_id, type, title, description, state, points,
		       assignee, sprint_id, priority, order_idx,
		       created_at, updated_at, closed_at
		FROM agile_work_items WHERE id = ?`, id)
	return scanWorkItem(row)
}

// ListWorkItems returns every item for the configured project. Pass
// non-empty filters to constrain by sprint, state, or assignee.
func (s *Store) ListWorkItems(sprintID, state, assignee string) ([]WorkItem, error) {
	q := `SELECT id, project_id, type, title, description, state, points,
	             assignee, sprint_id, priority, order_idx,
	             created_at, updated_at, closed_at
	      FROM agile_work_items WHERE project_id = ?`
	args := []interface{}{s.ProjectID}
	if sprintID != "" {
		q += ` AND sprint_id = ?`
		args = append(args, sprintID)
	}
	if state != "" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	if assignee != "" {
		q += ` AND assignee = ?`
		args = append(args, assignee)
	}
	q += ` ORDER BY order_idx ASC, created_at ASC`

	rows, err := s.Conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkItem
	for rows.Next() {
		wi, err := scanWorkItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wi)
	}
	return out, rows.Err()
}

// DeleteWorkItem removes a work item.
func (s *Store) DeleteWorkItem(id string) error {
	_, err := s.Conn.Exec(`DELETE FROM agile_work_items WHERE id = ?`, id)
	return err
}

// MoveWorkItem updates state + order in one statement. Used by the
// Kanban drag-and-drop handler — sets `closed_at` if the destination
// is the "done" column.
func (s *Store) MoveWorkItem(id, newState string, newOrder int) error {
	closedAt := ""
	if newState == "done" {
		closedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := s.Conn.Exec(`
		UPDATE agile_work_items
		SET state = ?, order_idx = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
		    closed_at = CASE WHEN ? = 'done' THEN ? ELSE closed_at END
		WHERE id = ?
	`, newState, newOrder, newState, closedAt, id)
	return err
}

// WIPCountByColumn returns the number of items currently in each
// column, keyed by column ID. The Kanban GUI uses this to show WIP
// breach indicators on columns whose count exceeds their limit.
func (s *Store) WIPCountByColumn() (map[string]int, error) {
	rows, err := s.Conn.Query(`
		SELECT state, COUNT(*) FROM agile_work_items
		WHERE project_id = ? GROUP BY state`, s.ProjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}
		out[state] = n
	}
	return out, rows.Err()
}

func scanWorkItem(row interface {
	Scan(...interface{}) error
}) (WorkItem, error) {
	var (
		wi                                   WorkItem
		typeStr, prio                        string
		created, updated, closed             string
	)
	err := row.Scan(
		&wi.ID, &wi.ProjectID, &typeStr, &wi.Title, &wi.Description,
		&wi.State, &wi.Points, &wi.Assignee, &wi.SprintID, &prio, &wi.OrderIdx,
		&created, &updated, &closed,
	)
	if err == sql.ErrNoRows {
		return WorkItem{}, ErrNoWorkItem
	}
	if err != nil {
		return WorkItem{}, err
	}
	wi.Type = WorkItemType(typeStr)
	wi.Priority = Priority(prio)
	wi.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	wi.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if closed != "" {
		wi.ClosedAt, _ = time.Parse(time.RFC3339Nano, closed)
	}
	return wi, nil
}

// ----- Sprints -----

// ErrNoSprint indicates a missing sprint.
var ErrNoSprint = errors.New("agile: sprint not found")

// SaveSprint upserts a sprint. Activating a sprint (status →
// "active") is a normal save; the caller is responsible for ensuring
// only one sprint is active at a time (the GUI enforces this).
func (s *Store) SaveSprint(sp Sprint) (Sprint, error) {
	if sp.ID == "" {
		sp.ID = NewSprintID()
	}
	if sp.ProjectID == "" {
		sp.ProjectID = s.ProjectID
	}
	if sp.Status == "" {
		sp.Status = SprintPlanning
	}
	_, err := s.Conn.Exec(`
		INSERT INTO agile_sprints
			(id, project_id, name, goal, status, start_date, end_date, capacity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name       = excluded.name,
			goal       = excluded.goal,
			status     = excluded.status,
			start_date = excluded.start_date,
			end_date   = excluded.end_date,
			capacity   = excluded.capacity
	`,
		sp.ID, sp.ProjectID, sp.Name, sp.Goal, string(sp.Status),
		sp.StartDate, sp.EndDate, sp.Capacity,
	)
	if err != nil {
		return Sprint{}, err
	}
	return s.GetSprint(sp.ID)
}

// GetSprint fetches by ID.
func (s *Store) GetSprint(id string) (Sprint, error) {
	row := s.Conn.QueryRow(`
		SELECT id, project_id, name, goal, status,
		       start_date, end_date, capacity, created_at
		FROM agile_sprints WHERE id = ?`, id)
	return scanSprint(row)
}

// ListSprints returns every sprint for the project, newest first.
func (s *Store) ListSprints() ([]Sprint, error) {
	rows, err := s.Conn.Query(`
		SELECT id, project_id, name, goal, status,
		       start_date, end_date, capacity, created_at
		FROM agile_sprints WHERE project_id = ? ORDER BY created_at DESC`,
		s.ProjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Sprint
	for rows.Next() {
		sp, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// DeleteSprint removes a sprint and clears the sprint_id on any
// work items that referenced it (those items return to the backlog
// conceptually; their state column is preserved).
func (s *Store) DeleteSprint(id string) error {
	tx, err := s.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op if Commit succeeds

	if _, err := tx.Exec(
		`UPDATE agile_work_items SET sprint_id = '' WHERE sprint_id = ?`, id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM agile_sprints WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanSprint(row interface {
	Scan(...interface{}) error
}) (Sprint, error) {
	var (
		sp                Sprint
		status            string
		created           string
	)
	err := row.Scan(
		&sp.ID, &sp.ProjectID, &sp.Name, &sp.Goal, &status,
		&sp.StartDate, &sp.EndDate, &sp.Capacity, &created,
	)
	if err == sql.ErrNoRows {
		return Sprint{}, ErrNoSprint
	}
	if err != nil {
		return Sprint{}, err
	}
	sp.Status = SprintStatus(status)
	sp.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return sp, nil
}

// ----- Deployments -----

// SaveDeployment upserts a deployment record. DORA metrics derive
// from this table — every push to production should produce one row.
func (s *Store) SaveDeployment(d Deployment) (Deployment, error) {
	if d.ID == "" {
		d.ID = NewDeploymentID()
	}
	if d.ProjectID == "" {
		d.ProjectID = s.ProjectID
	}
	if d.TS.IsZero() {
		d.TS = time.Now().UTC()
	}
	successful := 0
	if d.Successful {
		successful = 1
	}
	_, err := s.Conn.Exec(`
		INSERT INTO agile_deployments
			(id, project_id, ts, version, successful,
			 lead_time_hours, restore_time_hours, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			ts                 = excluded.ts,
			version            = excluded.version,
			successful         = excluded.successful,
			lead_time_hours    = excluded.lead_time_hours,
			restore_time_hours = excluded.restore_time_hours,
			notes              = excluded.notes
	`,
		d.ID, d.ProjectID, d.TS.Format(time.RFC3339Nano), d.Version, successful,
		d.LeadTimeHours, d.RestoreTimeHours, d.Notes,
	)
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

// ListDeployments returns deployments in a time window, newest first.
// Pass zero values for `since` to get all deployments.
func (s *Store) ListDeployments(since time.Time) ([]Deployment, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if since.IsZero() {
		rows, err = s.Conn.Query(`
			SELECT id, project_id, ts, version, successful,
			       lead_time_hours, restore_time_hours, notes
			FROM agile_deployments WHERE project_id = ?
			ORDER BY ts DESC`, s.ProjectID)
	} else {
		rows, err = s.Conn.Query(`
			SELECT id, project_id, ts, version, successful,
			       lead_time_hours, restore_time_hours, notes
			FROM agile_deployments
			WHERE project_id = ? AND ts >= ?
			ORDER BY ts DESC`,
			s.ProjectID, since.Format(time.RFC3339Nano),
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deployment
	for rows.Next() {
		var (
			d          Deployment
			ts         string
			successful int
		)
		if err := rows.Scan(
			&d.ID, &d.ProjectID, &ts, &d.Version, &successful,
			&d.LeadTimeHours, &d.RestoreTimeHours, &d.Notes,
		); err != nil {
			return nil, err
		}
		d.TS, _ = time.Parse(time.RFC3339Nano, ts)
		d.Successful = successful != 0
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDeployment removes a deployment record.
func (s *Store) DeleteDeployment(id string) error {
	_, err := s.Conn.Exec(`DELETE FROM agile_deployments WHERE id = ?`, id)
	return err
}

// ensureProject is a small guard the public methods could call when
// ProjectID looks empty. Kept private so the contract is "construct
// with NewStore(_, projectID) or pay the consequences".
func (s *Store) ensureProject() error {
	if s.ProjectID == "" {
		return fmt.Errorf("agile: store has no project id")
	}
	return nil
}
