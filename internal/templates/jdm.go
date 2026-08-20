// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

// Package templates drives the Project Launchpad's "seed me some
// starter artifacts" behaviour. The rules — which industry +
// methodology combination produces which seed actions — are
// expressed as a JDM (JSON Decision Model) document evaluated by
// github.com/gorules/zen (Go binding at zen-go).
//
// Why JDM rather than a Go switch
//
//   - Adding a new industry/methodology combination is one row in a
//     table, not a recompile.
//   - The same JDM document can be reviewed by non-Go contributors.
//   - Future versions can ship organisation-specific overlay rules
//     in a sibling JDM file without forking the project.
//
// The decision input is a small object:
//
//	{ "industry": "software", "methodology": "scrum" }
//
// and the output is:
//
//	{ "seeds": ["kanban", "charter", "backlog", "sprint"] }
//
// The caller (root main.go) dispatches each seed string to
// the corresponding action — see seeds.go.
package templates

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	zen "github.com/gorules/zen-go"
)

// rulesJSON is the JDM decision document, embedded at build time.
// Edit launchpad_seeds.json (in the same directory) and rebuild;
// the binary picks up the change automatically.
//
//go:embed launchpad_seeds.json
var rulesJSON []byte

// decisionKey is the loader-key the engine uses to fetch our JDM.
// It's referenced both by the loader callback and by every
// Evaluate() call so the spelling is centralised here.
const decisionKey = "launchpad_seeds"

// Engine is a small wrapper around the zen (github.com/gorules/zen) decision engine (Go binding) that
// remembers the parsed Launchpad rules. Construct one per process;
// the underlying engine is safe for concurrent Evaluate calls.
type Engine struct {
	z zen.Engine
}

// NewEngine wires the zen (github.com/gorules/zen) Go binding to the embedded JDM document.
//
// The loader is a `func(key string) ([]byte, error)` — zen's (Go binding)
// pluggable file-source interface. We close over the embedded bytes
// rather than reading from disk so the running binary is
// self-contained.
//
// Returns an error if the engine can't be constructed; the embedded
// JSON itself is validated by TestEmbeddedRulesParse in
// jdm_test.go so a corrupt file fails the test, not production.
func NewEngine() (*Engine, error) {
	loader := func(key string) ([]byte, error) {
		if key == decisionKey {
			return rulesJSON, nil
		}
		return nil, fmt.Errorf("templates: no decision named %q", key)
	}
	z := zen.NewEngine(zen.EngineConfig{Loader: loader})
	return &Engine{z: z}, nil
}

// Evaluate runs the Launchpad decision against the given request and
// returns the list of seed action strings. An unknown industry/
// methodology pair does not error — it hits launchpad_seeds.json's
// catch-all rule (empty industry/methodology, first-hit policy) and
// returns Seeds == []string{"charter"}, not an empty slice.
//
// The zen (Go binding) Evaluate takes the decision key and an input map;
// SeedRequest's two fields are built into the map directly (their JSON
// tags in jdm_types.go are the schema, so there is nothing a
// marshal/unmarshal round-trip would add).
//
// result.Result is a json.RawMessage, whose MarshalJSON always succeeds
// (it returns its own bytes, or "null" if nil), so re-marshalling it can't
// fail. Only e.z.Evaluate (crossing into the zen engine) and the final
// Unmarshal into SeedResponse (if the JDM's output shape doesn't match)
// can actually fail.
func (e *Engine) Evaluate(ctx context.Context, req SeedRequest) (SeedResponse, error) {
	if e == nil {
		return SeedResponse{}, fmt.Errorf("templates: engine not initialised")
	}
	input := map[string]any{
		"industry":    req.Industry,
		"methodology": req.Methodology,
	}

	result, err := e.z.Evaluate(decisionKey, input)
	if err != nil {
		return SeedResponse{}, fmt.Errorf("templates: evaluate: %w", err)
	}

	// The zen (Go binding) EvaluationResult.Result is a JSON-encoded map; marshal
	// back into our typed shape.
	resultRaw, _ := json.Marshal(result.Result)
	var resp SeedResponse
	if err := json.Unmarshal(resultRaw, &resp); err != nil {
		return SeedResponse{Seeds: nil}, nil
	}
	return resp, nil
}
