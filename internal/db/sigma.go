// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"pmforge/internal/sigma/domain"
)

func decodeSigmaJSON(label string, data []byte, dst any) error {
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("sigma %s decode: %w", label, err)
	}
	return nil
}

func (d *Database) SigmaCreateProject(p domain.Project) error {
	_, err := d.Conn.Exec(
		`INSERT INTO sigma_projects (id, title, description, belt_level, phase, status, sponsor, process_owner, belt_lead)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Title, p.Description, p.BeltLevel, p.Phase, p.Status, p.Sponsor, p.ProcessOwner, p.BeltLead,
	)
	return err
}

func (d *Database) SigmaGetProject(id string) (*domain.Project, error) {
	row := d.Conn.QueryRow(
		`SELECT id, title, description, belt_level, phase, status, sponsor, process_owner, belt_lead, created_at, updated_at
		 FROM sigma_projects WHERE id = ?`,
		id,
	)
	var p domain.Project
	var created, updated string
	err := row.Scan(&p.ID, &p.Title, &p.Description, &p.BeltLevel, &p.Phase, &p.Status, &p.Sponsor, &p.ProcessOwner, &p.BeltLead, &created, &updated)
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &p, nil
}

func (d *Database) SigmaListProjects() ([]domain.Project, error) {
	rows, err := d.Conn.Query(
		`SELECT id, title, description, belt_level, phase, status, sponsor, process_owner, belt_lead, created_at, updated_at
		 FROM sigma_projects ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Project
	for rows.Next() {
		var p domain.Project
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.BeltLevel, &p.Phase, &p.Status, &p.Sponsor, &p.ProcessOwner, &p.BeltLead, &created, &updated); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, created)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, p)
	}
	// A mid-iteration error ends the loop silently; without this check a
	// truncated project list would be returned as if complete.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *Database) SigmaSaveCharter(c domain.Charter) error {
	scopeIn, err := json.Marshal(c.ScopeIn)
	if err != nil {
		return fmt.Errorf("sigma charter scope_in encode: %w", err)
	}
	scopeOut, err := json.Marshal(c.ScopeOut)
	if err != nil {
		return fmt.Errorf("sigma charter scope_out encode: %w", err)
	}
	ctqs, err := json.Marshal(c.CTQs)
	if err != nil {
		return fmt.Errorf("sigma charter ctqs encode: %w", err)
	}

	_, err = d.Conn.Exec(
		`INSERT INTO sigma_charters (id, project_id, problem_statement, business_case, goal_statement, scope_in, scope_out, ctqs, sponsor)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			problem_statement = excluded.problem_statement,
			business_case = excluded.business_case,
			goal_statement = excluded.goal_statement,
			scope_in = excluded.scope_in,
			scope_out = excluded.scope_out,
			ctqs = excluded.ctqs,
			sponsor = excluded.sponsor,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		c.ID, c.ProjectID, c.ProblemStatement, c.BusinessCase, c.GoalStatement, scopeIn, scopeOut, ctqs, c.Sponsor,
	)
	return err
}

func (d *Database) SigmaGetCharter(projectID string) (*domain.Charter, error) {
	row := d.Conn.QueryRow(
		`SELECT id, project_id, problem_statement, business_case, goal_statement, scope_in, scope_out, ctqs, sponsor, updated_at
		 FROM sigma_charters WHERE project_id = ?`,
		projectID,
	)
	var c domain.Charter
	var scopeIn, scopeOut, ctqs []byte
	var updated string
	err := row.Scan(&c.ID, &c.ProjectID, &c.ProblemStatement, &c.BusinessCase, &c.GoalStatement, &scopeIn, &scopeOut, &ctqs, &c.Sponsor, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.Charter{ProjectID: projectID}, nil
		}
		return nil, err
	}
	if err := decodeSigmaJSON("charter scope_in", scopeIn, &c.ScopeIn); err != nil {
		return nil, err
	}
	if err := decodeSigmaJSON("charter scope_out", scopeOut, &c.ScopeOut); err != nil {
		return nil, err
	}
	if err := decodeSigmaJSON("charter ctqs", ctqs, &c.CTQs); err != nil {
		return nil, err
	}
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &c, nil
}

func (d *Database) SigmaAdvancePhase(projectID string, phase domain.Phase) error {
	_, err := d.Conn.Exec(
		`UPDATE sigma_projects SET phase = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		phase, projectID,
	)
	return err
}

// EnsureProjectSigmaLink creates a sigma_projects row linked to the main project table if it doesn't exist.
func (d *Database) EnsureProjectSigmaLink(p domain.Project) error {
	_, err := d.Conn.Exec(
		`INSERT OR IGNORE INTO sigma_projects (id, title, description, belt_level, phase, status)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Title, p.Description, p.BeltLevel, p.Phase, p.Status,
	)
	return err
}

func (d *Database) SigmaSaveFishbone(fb domain.FishboneData, projectID string) error {
	dataJSON, err := json.Marshal(fb)
	if err != nil {
		return fmt.Errorf("sigma fishbone encode: %w", err)
	}
	_, err = d.Conn.Exec(
		`INSERT INTO sigma_fishbones (id, project_id, problem_statement, data_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			problem_statement = excluded.problem_statement,
			data_json = excluded.data_json,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		"fishbone-"+projectID, projectID, fb.ProblemStatement, dataJSON,
	)
	return err
}

func (d *Database) SigmaGetFishbone(projectID string) (*domain.FishboneData, error) {
	row := d.Conn.QueryRow(
		`SELECT problem_statement, data_json FROM sigma_fishbones WHERE project_id = ?`,
		projectID,
	)
	var fb domain.FishboneData
	var dataJSON []byte
	var ps string
	err := row.Scan(&ps, &dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.FishboneData{
				Branches: []domain.FishboneBranch{
					{Category: "Man"}, {Category: "Machine"}, {Category: "Method"},
					{Category: "Material"}, {Category: "Measurement"}, {Category: "Environment"},
				},
			}, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(dataJSON, &fb); err != nil {
		var branches []domain.FishboneBranch
		if legacyErr := json.Unmarshal(dataJSON, &branches); legacyErr != nil {
			return nil, fmt.Errorf("sigma fishbone decode: %w", err)
		}
		fb.Branches = branches
	}
	if fb.ProblemStatement == "" {
		fb.ProblemStatement = ps
	}
	return &fb, nil
}

func (d *Database) SigmaSaveSolutions(projectID string, solutions []domain.Solution) error {
	dataJSON, err := json.Marshal(solutions)
	if err != nil {
		return fmt.Errorf("sigma solutions encode: %w", err)
	}
	_, err = d.Conn.Exec(
		`INSERT INTO sigma_solutions (id, project_id, data_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			data_json = excluded.data_json,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		"solutions-"+projectID, projectID, dataJSON,
	)
	return err
}

func (d *Database) SigmaGetSolutions(projectID string) ([]domain.Solution, error) {
	row := d.Conn.QueryRow(
		`SELECT data_json FROM sigma_solutions WHERE project_id = ?`,
		projectID,
	)
	var dataJSON []byte
	err := row.Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return []domain.Solution{}, nil
		}
		return nil, err
	}
	var solutions []domain.Solution
	if err := decodeSigmaJSON("solutions", dataJSON, &solutions); err != nil {
		return nil, err
	}
	return solutions, nil
}

func (d *Database) SigmaSaveControlPlan(projectID string, items []domain.ControlPlanItem) error {
	dataJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("sigma control plan encode: %w", err)
	}
	_, err = d.Conn.Exec(
		`INSERT INTO sigma_control_plans (id, project_id, data_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			data_json = excluded.data_json,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		"controlplan-"+projectID, projectID, dataJSON,
	)
	return err
}

func (d *Database) SigmaGetControlPlan(projectID string) ([]domain.ControlPlanItem, error) {
	row := d.Conn.QueryRow(
		`SELECT data_json FROM sigma_control_plans WHERE project_id = ?`,
		projectID,
	)
	var dataJSON []byte
	err := row.Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return []domain.ControlPlanItem{}, nil
		}
		return nil, err
	}
	var items []domain.ControlPlanItem
	if err := decodeSigmaJSON("control plan", dataJSON, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (d *Database) SigmaSaveSIPOC(projectID string, data domain.SIPOCData) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sigma sipoc encode: %w", err)
	}
	_, err = d.Conn.Exec(
		`INSERT INTO sigma_sipocs (id, project_id, data_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			data_json = excluded.data_json,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		"sipoc-"+projectID, projectID, dataJSON,
	)
	return err
}

func (d *Database) SigmaGetSIPOC(projectID string) (*domain.SIPOCData, error) {
	row := d.Conn.QueryRow(
		`SELECT data_json FROM sigma_sipocs WHERE project_id = ?`,
		projectID,
	)
	var dataJSON []byte
	err := row.Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.SIPOCData{ProjectID: projectID}, nil
		}
		return nil, err
	}
	var data domain.SIPOCData
	if err := decodeSigmaJSON("sipoc", dataJSON, &data); err != nil {
		return nil, err
	}
	data.ProjectID = projectID
	return &data, nil
}

func (d *Database) SigmaSaveVoC(projectID string, data domain.VoCData) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sigma voc encode: %w", err)
	}
	_, err = d.Conn.Exec(
		`INSERT INTO sigma_voc (id, project_id, data_json)
		 VALUES (?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
			data_json = excluded.data_json,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		"voc-"+projectID, projectID, dataJSON,
	)
	return err
}

func (d *Database) SigmaGetVoC(projectID string) (*domain.VoCData, error) {
	row := d.Conn.QueryRow(
		`SELECT data_json FROM sigma_voc WHERE project_id = ?`,
		projectID,
	)
	var dataJSON []byte
	err := row.Scan(&dataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.VoCData{ProjectID: projectID}, nil
		}
		return nil, err
	}
	var data domain.VoCData
	if err := decodeSigmaJSON("voc", dataJSON, &data); err != nil {
		return nil, err
	}
	data.ProjectID = projectID
	return &data, nil
}
