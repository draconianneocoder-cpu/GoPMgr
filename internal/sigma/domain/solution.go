// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain

// Solution represents a potential improvement identified during the Improve phase.
type Solution struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Impact      int     `json:"impact"` // 1-10
	Effort      int     `json:"effort"` // 1-10
	Risk        int     `json:"risk"`   // 1-10
	Cost        float64 `json:"cost"`
	Selected    bool    `json:"selected"`
	Status      string  `json:"status"` // proposed, pilot, implemented
}

// ControlPlanItem represents a row in the Control Plan.
type ControlPlanItem struct {
	ID                string `json:"id"`
	ProcessStep       string `json:"process_step"`
	Metric            string `json:"metric"`
	Specification     string `json:"specification"`
	MeasurementMethod string `json:"measurement_method"`
	Frequency         string `json:"frequency"`
	Owner             string `json:"owner"`
	ResponsePlan      string `json:"response_plan"`
}
