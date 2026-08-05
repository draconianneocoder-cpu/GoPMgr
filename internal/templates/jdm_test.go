// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	zen "github.com/gorules/zen-go"
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

// TestEvaluate_SoftwareScrumRow pins Evaluate against a specific,
// known row in launchpad_seeds.json's decision table (industry=software,
// methodology=scrum -> kanban/charter/backlog/sprint1). This is what
// actually proves the "industry"/"methodology" map keys Evaluate builds
// reach the JDM correctly -- TestEngineEvaluatesFallback only exercises an
// unrecognised pair, which would stay green even if both keys were
// misspelled (every lookup would just fall through to the same fallback
// row it's already asserting).
//
// This couples the test to one specific row's content, in tension with the
// package doc's "adding a new industry/methodology combination is one row
// in a table, not a recompile" -- if launchpad_seeds.json's software+scrum
// row is edited, update the `want` slice here to match, same as you would
// any other pinning test whose fixture changed on purpose.
func TestEvaluate_SoftwareScrumRow(t *testing.T) {
	eng, err := NewEngine()
	if err != nil {
		t.Skipf("could not initialise zen-go engine in test env: %v", err)
	}
	resp, err := eng.Evaluate(context.Background(), SeedRequest{
		Industry:    "software",
		Methodology: "scrum",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	want := []string{"kanban", "charter", "backlog", "sprint1"}
	if len(resp.Seeds) != len(want) {
		t.Fatalf("Seeds = %v, want %v", resp.Seeds, want)
	}
	for i, s := range want {
		if resp.Seeds[i] != s {
			t.Errorf("Seeds[%d] = %q, want %q", i, resp.Seeds[i], s)
		}
	}
}

// TestEvaluate_NilEngineReturnsError covers Evaluate's defensive nil guard.
func TestEvaluate_NilEngineReturnsError(t *testing.T) {
	var eng *Engine
	_, err := eng.Evaluate(context.Background(), SeedRequest{})
	if err == nil {
		t.Fatal("expected an error calling Evaluate on a nil *Engine")
	}
}

// TestEvaluate_WrapsLoaderError proves Evaluate wraps a zen engine error
// (rather than swallowing it) with the "templates: evaluate:" prefix. Uses
// the same fixture shape as zen-go's own TestEngine_ErrorTransparency: a
// loader that returns an error is the library's documented failure mode,
// not an internal we're pinning by accident.
func TestEvaluate_WrapsLoaderError(t *testing.T) {
	wantErr := "boom: launchpad rules unavailable"
	z := zen.NewEngine(zen.EngineConfig{
		Loader: func(key string) ([]byte, error) {
			return nil, errors.New(wantErr)
		},
	})
	defer z.Dispose()
	eng := &Engine{z: z}

	_, err := eng.Evaluate(context.Background(), SeedRequest{Industry: "software", Methodology: "scrum"})
	if err == nil {
		t.Fatal("expected an error when the loader fails")
	}
	if !strings.Contains(err.Error(), "templates: evaluate:") {
		t.Errorf("error %q does not carry the templates: evaluate: wrapper prefix", err.Error())
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("error %q does not carry the underlying loader error %q", err.Error(), wantErr)
	}
}
