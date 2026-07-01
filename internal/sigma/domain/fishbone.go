// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain

// FishboneData represents the full Cause & Effect diagram for a project.
type FishboneData struct {
	ProblemStatement string           `json:"problem_statement"`
	Branches         []FishboneBranch `json:"branches"`
}

// FishboneBranch represents one of the 6Ms (or custom categories).
type FishboneBranch struct {
	Category string  `json:"category"`
	Causes   []Cause `json:"causes"`
}

// Cause represents a specific factor contributing to the problem.
type Cause struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	IsRootCause bool     `json:"is_root_cause"`
	FiveWhys    []string `json:"five_whys"` // Ordered from 1st Why to 5th
	Evidence    string   `json:"evidence"`
}
