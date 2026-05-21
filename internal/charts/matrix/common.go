// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package matrix implements PMForge's Matrix-family chart engine.
//
// The family covers four kinds — RACI, SWOT, Stakeholder Analysis, and
// Generic Matrix Diagram — that share a grid layout but have very
// different content models:
//
//   - RACI is a Roles × Tasks grid whose cells take values
//     R/A/C/I. The backend validates "exactly one A per task".
//   - SWOT is a fixed 2×2 with four string lists.
//   - Stakeholder is a 2×2 Power × Interest plot, each stakeholder
//     placed in one of four canonical engagement quadrants.
//   - Generic Matrix is an m×n grid with arbitrary cell text — used
//     for traceability matrices, prioritization, etc.
//
// Because the four kinds are structurally different, each gets its
// own *Layout type. The frontend dispatches on `engine == "matrix"`
// and then on `kind` to pick the right renderer.
package matrix

// Validation surfaces backend-detected issues to the GUI so it can
// render badges. ErrorCount is the field the UI reads to decide
// whether to show a "schema invalid" indicator.
type Validation struct {
	Issues     []string `json:"issues,omitempty"`
	ErrorCount int      `json:"error_count"`
}

// AddIssue appends a human-readable problem to v.Issues. The frontend
// renders these verbatim in a validation tray; keep them short.
func (v *Validation) AddIssue(s string) {
	v.Issues = append(v.Issues, s)
	v.ErrorCount++
}
