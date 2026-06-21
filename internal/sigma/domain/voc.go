// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain

// VoCEntry represents a Voice of Customer entry that translates
// customer needs into measurable CTQs.
type VoCEntry struct {
	ID             string  `json:"id"`
	CustomerNeed   string  `json:"customer_need"`
	CTQ            string  `json:"ctq"`
	LowerSpec      float64 `json:"lower_spec"`
	UpperSpec      float64 `json:"upper_spec"`
	Measurement    string  `json:"measurement"`
	DataCollection string  `json:"data_collection"`
	Priority       int     `json:"priority"` // 1-5 (1=highest)
	Source         string  `json:"source"`   // survey, interview, complaint, etc.
}

// VoCData holds the complete Voice of Customer dataset for a project.
type VoCData struct {
	ProjectID string    `json:"project_id"`
	Entries   []VoCEntry `json:"entries"`
}
