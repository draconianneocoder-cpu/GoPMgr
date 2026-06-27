// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scenario is a project-local what-if branch descriptor. The first
// slice stores metadata and active selection only; later slices attach
// isolated schedule/baseline partitions to this stable ID.
type Scenario struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Name             string `json:"name"`
	SourceBaselineID string `json:"source_baseline_id"`
	Description      string `json:"description"`
	IsActive         bool   `json:"is_active"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// ScenarioChart is an isolated chart/baseline copy owned by a what-if
// scenario. Data and Config start as a copy of the source chart; later
// slices can mutate them without changing the live chart.
type ScenarioChart struct {
	ID               string `json:"id"`
	ScenarioID       string `json:"scenario_id"`
	ProjectID        string `json:"project_id"`
	SourceChartID    string `json:"source_chart_id"`
	SourceBaselineID string `json:"source_baseline_id"`
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Data             string `json:"data"`
	Config           string `json:"config"`
	BaselineData     string `json:"baseline_data"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// ErrNoScenario is returned when GetScenario can't find the requested ID.
var ErrNoScenario = errors.New("db: scenario not found")

// ErrNoScenarioChart is returned when GetScenarioChart can't find the
// requested ID.
var ErrNoScenarioChart = errors.New("db: scenario chart not found")

// SaveScenario inserts or updates scenario metadata. When s.IsActive is
// true, any other active scenario for the same project is deactivated in
// the same transaction.
func (db *Database) SaveScenario(s Scenario) (Scenario, error) {
	if s.ProjectID == "" {
		return Scenario{}, errors.New("scenario: project_id is required")
	}
	if s.Name == "" {
		return Scenario{}, errors.New("scenario: name is required")
	}
	if s.ID == "" {
		id, err := newID("scn")
		if err != nil {
			return Scenario{}, fmt.Errorf("generate scenario id: %w", err)
		}
		s.ID = id
	}
	active := 0
	if s.IsActive {
		active = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	tx, err := db.Conn.Begin()
	if err != nil {
		return Scenario{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if s.IsActive {
		if _, err = tx.Exec(
			`UPDATE scenarios SET is_active = 0, updated_at = ? WHERE project_id = ? AND id <> ?`,
			now, s.ProjectID, s.ID,
		); err != nil {
			return Scenario{}, err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO scenarios (
			id, project_id, name, source_baseline_id, description,
			is_active, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id         = excluded.project_id,
			name               = excluded.name,
			source_baseline_id = excluded.source_baseline_id,
			description        = excluded.description,
			is_active          = excluded.is_active,
			updated_at         = excluded.updated_at
	`, s.ID, s.ProjectID, s.Name, s.SourceBaselineID, s.Description, active, now, now)
	if err != nil {
		return Scenario{}, err
	}
	if err = tx.Commit(); err != nil {
		return Scenario{}, err
	}
	return db.GetScenario(s.ID)
}

// GetScenario fetches one scenario by ID.
func (db *Database) GetScenario(id string) (Scenario, error) {
	row := db.Conn.QueryRow(`
		SELECT id, project_id, name, source_baseline_id, description,
		       is_active, created_at, updated_at
		FROM scenarios
		WHERE id = ?
	`, id)
	return scanScenario(row)
}

// ListScenarios returns project scenarios in active-first, newest-first
// order so the current branch is easy to surface in UI layers.
func (db *Database) ListScenarios(projectID string) ([]Scenario, error) {
	rows, err := db.Conn.Query(`
		SELECT id, project_id, name, source_baseline_id, description,
		       is_active, created_at, updated_at
		FROM scenarios
		WHERE project_id = ?
		ORDER BY is_active DESC, created_at DESC, name ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Scenario
	for rows.Next() {
		s, err := scanScenario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteScenario removes scenario metadata. Later scenario-partitioned
// rows should reference scenarios with ON DELETE CASCADE.
func (db *Database) DeleteScenario(id string) error {
	_, err := db.Conn.Exec(`DELETE FROM scenarios WHERE id = ?`, id)
	return err
}

// BranchScenarioChart copies a live chart and optional baseline into an
// isolated scenario partition. If baselineID is empty, the scenario's
// SourceBaselineID is used.
func (db *Database) BranchScenarioChart(scenarioID, chartID, baselineID string) (ScenarioChart, error) {
	scenario, err := db.GetScenario(scenarioID)
	if err != nil {
		return ScenarioChart{}, err
	}
	chart, err := db.GetChart(chartID)
	if err != nil {
		return ScenarioChart{}, err
	}
	if chart.ProjectID != scenario.ProjectID {
		return ScenarioChart{}, errors.New("scenario chart: source chart is outside scenario project")
	}
	if baselineID == "" {
		baselineID = scenario.SourceBaselineID
	}
	baselineData := "{}"
	if baselineID != "" {
		baseline, err := db.GetBaseline(baselineID)
		if err != nil {
			return ScenarioChart{}, err
		}
		if baseline.ProjectID != scenario.ProjectID {
			return ScenarioChart{}, errors.New("scenario chart: source baseline is outside scenario project")
		}
		if baseline.ChartID != chart.ID {
			return ScenarioChart{}, errors.New("scenario chart: source baseline does not match chart")
		}
		baselineData = baseline.Data
	}
	id, err := newID("schart")
	if err != nil {
		return ScenarioChart{}, fmt.Errorf("generate scenario chart id: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Conn.Exec(`
		INSERT INTO scenario_charts (
			id, scenario_id, project_id, source_chart_id, source_baseline_id,
			kind, title, data, config, baseline_data, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, scenario.ID, scenario.ProjectID, chart.ID, baselineID,
		chart.Kind, chart.Title, chart.Data, chart.Config, baselineData, now, now)
	if err != nil {
		return ScenarioChart{}, err
	}
	return db.GetScenarioChart(id)
}

// GetScenarioChart fetches one isolated scenario chart by ID.
func (db *Database) GetScenarioChart(id string) (ScenarioChart, error) {
	row := db.Conn.QueryRow(`
		SELECT id, scenario_id, project_id, source_chart_id, source_baseline_id,
		       kind, title, data, config, baseline_data, created_at, updated_at
		FROM scenario_charts
		WHERE id = ?
	`, id)
	return scanScenarioChart(row)
}

// ListScenarioCharts returns isolated chart copies for a scenario.
func (db *Database) ListScenarioCharts(scenarioID string) ([]ScenarioChart, error) {
	rows, err := db.Conn.Query(`
		SELECT id, scenario_id, project_id, source_chart_id, source_baseline_id,
		       kind, title, data, config, baseline_data, created_at, updated_at
		FROM scenario_charts
		WHERE scenario_id = ?
		ORDER BY updated_at DESC, title ASC
	`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ScenarioChart
	for rows.Next() {
		c, err := scanScenarioChart(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveScenarioChart updates the editable fields of an isolated scenario
// chart copy. Project/source/baseline/kind fields remain immutable.
func (db *Database) SaveScenarioChart(c ScenarioChart) (ScenarioChart, error) {
	if c.ID == "" {
		return ScenarioChart{}, errors.New("scenario chart: id is required")
	}
	existing, err := db.GetScenarioChart(c.ID)
	if err != nil {
		return ScenarioChart{}, err
	}
	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = existing.Title
	}
	data := strings.TrimSpace(c.Data)
	if data == "" {
		data = "{}"
	}
	config := strings.TrimSpace(c.Config)
	if config == "" {
		config = "{}"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Conn.Exec(`
		UPDATE scenario_charts
		SET title = ?, data = ?, config = ?, updated_at = ?
		WHERE id = ?
	`, title, data, config, now, c.ID); err != nil {
		return ScenarioChart{}, err
	}
	return db.GetScenarioChart(c.ID)
}

// PromoteScenarioChartToBaseline writes a scenario chart's current data
// back as a named immutable baseline for its source chart.
func (db *Database) PromoteScenarioChartToBaseline(scenarioChartID, name string) (Baseline, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Baseline{}, errors.New("scenario promotion: baseline name is required")
	}
	scenarioChart, err := db.GetScenarioChart(scenarioChartID)
	if err != nil {
		return Baseline{}, err
	}
	sourceChart, err := db.GetChart(scenarioChart.SourceChartID)
	if err != nil {
		return Baseline{}, err
	}
	if sourceChart.ProjectID != scenarioChart.ProjectID {
		return Baseline{}, errors.New("scenario promotion: source chart is outside scenario project")
	}
	return db.SaveBaseline(Baseline{
		ProjectID: scenarioChart.ProjectID,
		ChartID:   scenarioChart.SourceChartID,
		Name:      name,
		Data:      scenarioChart.Data,
	})
}

func scanScenario(row interface {
	Scan(...interface{}) error
}) (Scenario, error) {
	var (
		s      Scenario
		active int
	)
	err := row.Scan(
		&s.ID, &s.ProjectID, &s.Name, &s.SourceBaselineID, &s.Description,
		&active, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Scenario{}, ErrNoScenario
	}
	if err != nil {
		return Scenario{}, err
	}
	s.IsActive = active != 0
	return s, nil
}

func scanScenarioChart(row interface {
	Scan(...interface{}) error
}) (ScenarioChart, error) {
	var c ScenarioChart
	err := row.Scan(
		&c.ID, &c.ScenarioID, &c.ProjectID, &c.SourceChartID, &c.SourceBaselineID,
		&c.Kind, &c.Title, &c.Data, &c.Config, &c.BaselineData, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return ScenarioChart{}, ErrNoScenarioChart
	}
	if err != nil {
		return ScenarioChart{}, err
	}
	return c, nil
}
