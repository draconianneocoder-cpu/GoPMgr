// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
// returns the list of seed action strings. An unknown
// industry/methodology pair returns an empty Seeds slice rather than
// an error — the GUI treats that as "no auto-seed, user starts
// blank".
//
// The zen (Go binding) Evaluate takes the decision key and an input map. We
// build the map by marshalling SeedRequest to JSON and back into a
// map[string]any — slower than constructing the map directly but
// keeps SeedRequest as the single source of truth for the input
// schema. The cost is negligible at one call per project creation.
func (e *Engine) Evaluate(ctx context.Context, req SeedRequest) (SeedResponse, error) {
	if e == nil {
		return SeedResponse{}, fmt.Errorf("templates: engine not initialised")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return SeedResponse{}, err
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return SeedResponse{}, err
	}

	result, err := e.z.Evaluate(decisionKey, input)
	if err != nil {
		return SeedResponse{}, fmt.Errorf("templates: evaluate: %w", err)
	}

	// The zen (Go binding) EvaluationResult.Result is a JSON-encoded map; marshal
	// back into our typed shape.
	resultRaw, err := json.Marshal(result.Result)
	if err != nil {
		return SeedResponse{Seeds: nil}, nil
	}
	var resp SeedResponse
	if err := json.Unmarshal(resultRaw, &resp); err != nil {
		return SeedResponse{Seeds: nil}, nil
	}
	return resp, nil
}
