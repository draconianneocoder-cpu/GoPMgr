// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain

import "time"

type Phase string

const (
	PhaseDefine  Phase = "define"
	PhaseMeasure Phase = "measure"
	PhaseAnalyze Phase = "analyze"
	PhaseImprove Phase = "improve"
	PhaseControl Phase = "control"
)

type ProjectStatus string

const (
	StatusActive   ProjectStatus = "active"
	StatusOnHold   ProjectStatus = "on_hold"
	StatusComplete ProjectStatus = "complete"
)

type BeltLevel string

const (
	BeltGreen  BeltLevel = "green"
	BeltBlack  BeltLevel = "black"
	BeltMaster BeltLevel = "master"
)

// Project represents a Six Sigma improvement project.
type Project struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	BeltLevel    BeltLevel     `json:"belt_level"`
	Phase        Phase         `json:"phase"`
	Status       ProjectStatus `json:"status"`
	Sponsor      string        `json:"sponsor"`
	ProcessOwner string        `json:"process_owner"`
	BeltLead     string        `json:"belt_lead"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Charter represents the Define phase deliverable.
type Charter struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	ProblemStatement string    `json:"problem_statement"`
	BusinessCase     string    `json:"business_case"`
	GoalStatement    string    `json:"goal_statement"`
	ScopeIn          []string  `json:"scope_in"`
	ScopeOut         []string  `json:"scope_out"`
	CTQs             []CTQ     `json:"ctqs"`
	Sponsor          string    `json:"sponsor"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CTQ represents a Critical to Quality requirement.
type CTQ struct {
	CustomerNeed string  `json:"customer_need"`
	CTQ          string  `json:"ctq"`
	LowerSpec    float64 `json:"lower_spec"`
	UpperSpec    float64 `json:"upper_spec"`
}
