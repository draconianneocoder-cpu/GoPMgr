// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

// SeedRequest is the input the Launchpad sends to the engine.
type SeedRequest struct {
	Industry    string `json:"industry"`
	Methodology string `json:"methodology"`
}

// SeedResponse is what the engine returns. The order of Seeds follows the
// decision table because some seed actions depend on earlier ones.
type SeedResponse struct {
	Seeds []string `json:"seeds"`
}
