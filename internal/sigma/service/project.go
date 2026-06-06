// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"fmt"
	"time"

	"pmforge/internal/db"
	"pmforge/internal/sigma/domain"
)

// ProjectService wraps the DB methods for Sigma projects.
type ProjectService struct {
	DB *db.Database
}

func NewProjectService(d *db.Database) *ProjectService {
	return &ProjectService{DB: d}
}

func (s *ProjectService) CreateProject(input domain.Project) (*domain.Project, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("sigma: title required")
	}
	if input.ID == "" {
		input.ID = fmt.Sprintf("sigma-%d", time.Now().UnixNano())
	}
	input.CreatedAt = time.Now().UTC()
	input.UpdatedAt = time.Now().UTC()
	input.Phase = domain.PhaseDefine
	input.Status = domain.StatusActive

	if err := s.DB.SigmaCreateProject(input); err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *ProjectService) GetProject(id string) (*domain.Project, error) {
	return s.DB.SigmaGetProject(id)
}

func (s *ProjectService) ListProjects() ([]domain.Project, error) {
	return s.DB.SigmaListProjects()
}

func (s *ProjectService) SaveCharter(c domain.Charter) error {
	if c.ProjectID == "" {
		return fmt.Errorf("sigma: project_id required")
	}
	if c.ID == "" {
		c.ID = fmt.Sprintf("charter-%s", c.ProjectID)
	}
	return s.DB.SigmaSaveCharter(c)
}

func (s *ProjectService) GetCharter(projectID string) (*domain.Charter, error) {
	return s.DB.SigmaGetCharter(projectID)
}

func (s *ProjectService) AdvancePhase(projectID string, phase domain.Phase) error {
	return s.DB.SigmaAdvancePhase(projectID, phase)
}

func (s *ProjectService) SaveFishbone(fb domain.FishboneData, projectID string) error {
	return s.DB.SigmaSaveFishbone(fb, projectID)
}

func (s *ProjectService) GetFishbone(projectID string) (*domain.FishboneData, error) {
	return s.DB.SigmaGetFishbone(projectID)
}

func (s *ProjectService) SaveSolutions(projectID string, solutions []domain.Solution) error {
	if projectID == "" {
		return fmt.Errorf("sigma: project_id required")
	}
	for i := range solutions {
		if solutions[i].ID == "" {
			solutions[i].ID = fmt.Sprintf("sol-%s-%d", projectID, i)
		}
	}
	return s.DB.SigmaSaveSolutions(projectID, solutions)
}

func (s *ProjectService) GetSolutions(projectID string) ([]domain.Solution, error) {
	return s.DB.SigmaGetSolutions(projectID)
}

func (s *ProjectService) SaveControlPlan(projectID string, items []domain.ControlPlanItem) error {
	if projectID == "" {
		return fmt.Errorf("sigma: project_id required")
	}
	for i := range items {
		if items[i].ID == "" {
			items[i].ID = fmt.Sprintf("cp-%s-%d", projectID, i)
		}
	}
	return s.DB.SigmaSaveControlPlan(projectID, items)
}

func (s *ProjectService) GetControlPlan(projectID string) ([]domain.ControlPlanItem, error) {
	return s.DB.SigmaGetControlPlan(projectID)
}

func (s *ProjectService) SaveSIPOC(projectID string, data domain.SIPOCData) error {
	if projectID == "" {
		return fmt.Errorf("sigma: project_id required")
	}
	for i := range data.Elements {
		if data.Elements[i].ID == "" {
			data.Elements[i].ID = fmt.Sprintf("sipoc-%s-%d", projectID, i)
		}
	}
	return s.DB.SigmaSaveSIPOC(projectID, data)
}

func (s *ProjectService) GetSIPOC(projectID string) (*domain.SIPOCData, error) {
	return s.DB.SigmaGetSIPOC(projectID)
}

func (s *ProjectService) SaveVoC(projectID string, data domain.VoCData) error {
	if projectID == "" {
		return fmt.Errorf("sigma: project_id required")
	}
	for i := range data.Entries {
		if data.Entries[i].ID == "" {
			data.Entries[i].ID = fmt.Sprintf("voc-%s-%d", projectID, i)
		}
	}
	return s.DB.SigmaSaveVoC(projectID, data)
}

func (s *ProjectService) GetVoC(projectID string) (*domain.VoCData, error) {
	return s.DB.SigmaGetVoC(projectID)
}
