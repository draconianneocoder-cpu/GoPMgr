// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package domain_test

import (
	"testing"

	"pmforge/internal/sigma/domain"
)

// TestSigmaEnumStringValuesAreStable pins the on-the-wire string value of
// every Six Sigma domain enum.
//
// These values are persisted to the .pmforge database and serialized to JSON
// across the Wails bridge, so they are a storage contract: changing a
// constant's VALUE (e.g. "define" -> "Define") silently breaks every
// already-stored project. This guard fails the moment a value drifts. If a
// change is intentional, update the expectation here AND ship a data
// migration. (Renaming the Go identifier is fine — only the string matters.)
func TestSigmaEnumStringValuesAreStable(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		// DMAIC phases (domain.Phase)
		{"PhaseDefine", string(domain.PhaseDefine), "define"},
		{"PhaseMeasure", string(domain.PhaseMeasure), "measure"},
		{"PhaseAnalyze", string(domain.PhaseAnalyze), "analyze"},
		{"PhaseImprove", string(domain.PhaseImprove), "improve"},
		{"PhaseControl", string(domain.PhaseControl), "control"},

		// Project status (domain.ProjectStatus)
		{"StatusActive", string(domain.StatusActive), "active"},
		{"StatusOnHold", string(domain.StatusOnHold), "on_hold"},
		{"StatusComplete", string(domain.StatusComplete), "complete"},

		// Belt levels (domain.BeltLevel)
		{"BeltGreen", string(domain.BeltGreen), "green"},
		{"BeltBlack", string(domain.BeltBlack), "black"},
		{"BeltMaster", string(domain.BeltMaster), "master"},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q — this value is persisted; changing it needs a data migration",
				c.name, c.got, c.want)
		}
	}
}
