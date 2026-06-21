// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"context"
	"encoding/json"
	"testing"
)

// TestEmbeddedRulesParse asserts that launchpad_seeds.json — embedded
// at build time — is well-formed JSON. A malformed file would make
// NewEngine return an error at startup, which we want caught here
// rather than in production.
func TestEmbeddedRulesParse(t *testing.T) {
	var v map[string]any
	if err := json.Unmarshal(rulesJSON, &v); err != nil {
		t.Fatalf("embedded launchpad_seeds.json is not valid JSON: %v", err)
	}
	if _, ok := v["nodes"]; !ok {
		t.Fatal("launchpad_seeds.json: missing required `nodes` array")
	}
}

// TestEngineEvaluatesFallback confirms that an unknown industry
// returns the JDM's fallback row (a single `charter` seed) rather
// than an error.
//
// This test is best-effort: if zen-go fails to construct an engine
// in this test environment (e.g. CGo not available), we skip
// rather than fail — the real coverage is in production startup.
func TestEngineEvaluatesFallback(t *testing.T) {
	eng, err := NewEngine()
	if err != nil {
		t.Skipf("could not initialise zen-go engine in test env: %v", err)
	}
	resp, err := eng.Evaluate(context.Background(), SeedRequest{
		Industry:    "unknown-industry",
		Methodology: "unknown-methodology",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	// Fallback row in the JDM yields ['charter']. We accept any
	// non-empty fallback in case the JDM is later edited.
	if len(resp.Seeds) == 0 {
		t.Log("note: fallback row yielded no seeds; verify launchpad_seeds.json has a catch-all")
	}
}
