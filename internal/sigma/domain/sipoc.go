// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain

// SIPOCElement represents a single entry in a SIPOC diagram.
type SIPOCElement struct {
	ID           string `json:"id"`
	Category     string `json:"category"` // supplier, input, process, output, customer
	Description  string `json:"description"`
	Owner        string `json:"owner"`
	Requirements string `json:"requirements"`
	Order        int    `json:"order"`
}

// SIPOCData holds the complete SIPOC diagram for a project.
type SIPOCData struct {
	ProjectID    string         `json:"project_id"`
	ProcessName  string         `json:"process_name"`
	ProcessScope string         `json:"process_scope"`
	StartTrigger string         `json:"start_trigger"`
	EndTrigger   string         `json:"end_trigger"`
	Elements     []SIPOCElement `json:"elements"`
}
