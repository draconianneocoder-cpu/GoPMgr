// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package templates drives the Project Launchpad's "seed me some
// starter artifacts" behaviour. The rules — which industry +
// methodology combination produces which seed actions — are a decision
// table in launchpad_seeds.json, written in the JDM (JSON Decision
// Model) layout.
//
// Why a table rather than a Go switch
//
//   - Adding a new industry/methodology combination is one row in a
//     table.
//   - The same document can be reviewed by non-Go contributors, and
//     opened in a JDM editor.
//
// The decision input is a small object:
//
//	{ "industry": "software", "methodology": "scrum" }
//
// and the output is:
//
//	{ "seeds": ["kanban", "charter", "backlog", "sprint1"] }
//
// The caller dispatches each seed string to the corresponding action —
// see seeds.go.
//
// The table is evaluated here, in Go. Until 2026-11-11 every platform
// but Windows evaluated it with the gorules zen engine through its
// prebuilt Rust library; Go 1.27.4 refuses to link that library (it
// carries LLVM bitcode), and this evaluator, already used on Windows,
// gave the same answer for all 729 inputs compared before the engine was
// removed (testdata/launchpad_seeds.zen-0.20.0.json). It implements only
// what the table uses: rows that match both inputs exactly, and one
// final blank row as the fallback. NewEngine refuses a table that uses
// anything else, so a rule this evaluator would misread cannot ship.
package templates

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// rulesJSON is the decision table, embedded at build time. Edit
// launchpad_seeds.json (in the same directory) and rebuild.
//
//go:embed launchpad_seeds.json
var rulesJSON []byte

// seedRule is one table row as written: each cell is a JDM expression.
type seedRule struct {
	Industry    string `json:"industry"`
	Methodology string `json:"methodology"`
	Seeds       string `json:"seeds"`
}

type seedRulesDocument struct {
	Nodes []struct {
		Content struct {
			HitPolicy string     `json:"hitPolicy"`
			Rules     []seedRule `json:"rules"`
		} `json:"content"`
	} `json:"nodes"`
}

// seedRow is a validated row: the two values to match and the seeds.
type seedRow struct {
	industry, methodology string
	seeds                 []string
}

// Engine evaluates the embedded Launchpad rule table. It is immutable
// after NewEngine and safe for concurrent use.
type Engine struct {
	rows     []seedRow
	fallback []string
}

// NewEngine parses and validates the embedded table.
func NewEngine() (*Engine, error) {
	return newEngine(rulesJSON)
}

// newEngine builds an Engine from a JDM document. It returns an error for
// a table that uses anything beyond exact-match rows and a final blank
// fallback row, because Evaluate would not give such a table the meaning
// a JDM engine does.
func newEngine(document []byte) (*Engine, error) {
	var doc seedRulesDocument
	if err := json.Unmarshal(document, &doc); err != nil {
		return nil, fmt.Errorf("templates: parse Launchpad rules: %w", err)
	}
	for _, node := range doc.Nodes {
		if len(node.Content.Rules) == 0 {
			continue
		}
		if node.Content.HitPolicy != "first" {
			return nil, fmt.Errorf("templates: Launchpad rules use hit policy %q; only \"first\" is supported", node.Content.HitPolicy)
		}
		return engineFromRules(node.Content.Rules)
	}
	return nil, fmt.Errorf("templates: Launchpad rules contain no rows")
}

func engineFromRules(rules []seedRule) (*Engine, error) {
	engine := &Engine{}
	for i, rule := range rules {
		seeds, err := parseSeedList(rule.Seeds)
		if err != nil {
			return nil, fmt.Errorf("templates: Launchpad rule %d: %w", i+1, err)
		}
		industryBlank := strings.TrimSpace(rule.Industry) == ""
		methodologyBlank := strings.TrimSpace(rule.Methodology) == ""
		if industryBlank && methodologyBlank {
			if i != len(rules)-1 {
				return nil, fmt.Errorf("templates: Launchpad rule %d matches everything but is not the last row", i+1)
			}
			engine.fallback = seeds
			continue
		}
		if industryBlank || methodologyBlank {
			return nil, fmt.Errorf("templates: Launchpad rule %d leaves one input blank; wildcard rows are not supported", i+1)
		}
		industry, err := stringLiteral(rule.Industry)
		if err != nil {
			return nil, fmt.Errorf("templates: Launchpad rule %d industry: %w", i+1, err)
		}
		methodology, err := stringLiteral(rule.Methodology)
		if err != nil {
			return nil, fmt.Errorf("templates: Launchpad rule %d methodology: %w", i+1, err)
		}
		engine.rows = append(engine.rows, seedRow{industry: industry, methodology: methodology, seeds: seeds})
	}
	return engine, nil
}

// Evaluate returns the seeds of the first row matching both inputs
// exactly, or the fallback row's seeds (in the embedded table,
// ["charter"]) when none does. It never fails on an unknown pair. The
// context is unused; it is kept so callers need not change.
func (e *Engine) Evaluate(_ context.Context, req SeedRequest) (SeedResponse, error) {
	if e == nil {
		return SeedResponse{}, fmt.Errorf("templates: engine not initialised")
	}
	for _, row := range e.rows {
		if row.industry == req.Industry && row.methodology == req.Methodology {
			return SeedResponse{Seeds: append([]string(nil), row.seeds...)}, nil
		}
	}
	return SeedResponse{Seeds: append([]string(nil), e.fallback...)}, nil
}

// stringLiteral reads a cell that must be one single-quoted string with
// no quote, comma or bracket inside it, the only input form the table
// uses. Anything else is a JDM expression this package does not evaluate.
func stringLiteral(cell string) (string, error) {
	cell = strings.TrimSpace(cell)
	if len(cell) < 2 || cell[0] != '\'' || cell[len(cell)-1] != '\'' {
		return "", fmt.Errorf("%q is not a single-quoted string", cell)
	}
	value := cell[1 : len(cell)-1]
	if strings.ContainsAny(value, `'",[]\`) {
		return "", fmt.Errorf("%q is not a plain single-quoted string", cell)
	}
	return value, nil
}

// parseSeedList reads an output cell: a bracketed, comma-separated list
// of single-quoted strings, such as ['kanban','charter'].
func parseSeedList(cell string) ([]string, error) {
	cell = strings.TrimSpace(cell)
	if len(cell) < 2 || cell[0] != '[' || cell[len(cell)-1] != ']' {
		return nil, fmt.Errorf("seeds %q are not a bracketed list", cell)
	}
	inner := strings.TrimSpace(cell[1 : len(cell)-1])
	if inner == "" {
		return nil, fmt.Errorf("seeds %q are empty", cell)
	}
	parts := strings.Split(inner, ",")
	seeds := make([]string, 0, len(parts))
	for _, part := range parts {
		seed, err := stringLiteral(part)
		if err != nil {
			return nil, fmt.Errorf("seeds: %w", err)
		}
		if seed == "" {
			return nil, fmt.Errorf("seeds %q contain an empty name", cell)
		}
		seeds = append(seeds, seed)
	}
	return seeds, nil
}
