// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package templates

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Zen-go distributes its Windows FFI archive for the MSVC ABI, while PMForge's
// SQLCipher driver requires MinGW. Keep the embedded JDM table authoritative,
// but evaluate its exact-match rows in Go on Windows so these toolchains do not
// meet in one linker invocation.
//
//go:embed launchpad_seeds.json
var rulesJSON []byte

type windowsRule struct {
	Industry    string `json:"industry"`
	Methodology string `json:"methodology"`
	Seeds       string `json:"seeds"`
}

type windowsRulesDocument struct {
	Nodes []struct {
		Content struct {
			Rules []windowsRule `json:"rules"`
		} `json:"content"`
	} `json:"nodes"`
}

// Engine evaluates the embedded Launchpad rule table on Windows.
type Engine struct {
	rules []windowsRule
}

// NewEngine parses the same embedded JDM document used on other platforms.
func NewEngine() (*Engine, error) {
	var document windowsRulesDocument
	if err := json.Unmarshal(rulesJSON, &document); err != nil {
		return nil, fmt.Errorf("templates: parse embedded Launchpad rules: %w", err)
	}
	for _, node := range document.Nodes {
		if len(node.Content.Rules) > 0 {
			return &Engine{rules: node.Content.Rules}, nil
		}
	}
	return nil, fmt.Errorf("templates: embedded Launchpad rules contain no rows")
}

// Evaluate preserves the table's exact-match behavior and its final blank-row
// fallback. The context is accepted to keep the platform API identical.
func (e *Engine) Evaluate(_ context.Context, request SeedRequest) (SeedResponse, error) {
	if e == nil {
		return SeedResponse{}, fmt.Errorf("templates: engine not initialised")
	}
	var fallback *windowsRule
	for index := range e.rules {
		rule := &e.rules[index]
		industry := jdmString(rule.Industry)
		methodology := jdmString(rule.Methodology)
		if industry == "" && methodology == "" {
			fallback = rule
			continue
		}
		if industry == request.Industry && methodology == request.Methodology {
			return SeedResponse{Seeds: parseJDMSeeds(rule.Seeds)}, nil
		}
	}
	if fallback == nil {
		return SeedResponse{}, nil
	}
	return SeedResponse{Seeds: parseJDMSeeds(fallback.Seeds)}, nil
}

func jdmString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func parseJDMSeeds(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	seeds := make([]string, 0, len(parts))
	for _, part := range parts {
		if seed := jdmString(part); seed != "" {
			seeds = append(seeds, seed)
		}
	}
	return seeds
}
