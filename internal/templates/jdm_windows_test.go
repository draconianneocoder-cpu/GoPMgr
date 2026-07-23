// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package templates

import (
	"context"
	"reflect"
	"testing"
)

func TestWindowsEngineEvaluatesTableAndFallback(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	tests := []struct {
		name    string
		request SeedRequest
		want    []string
	}{
		{
			name: "exact match",
			request: SeedRequest{
				Industry:    "software",
				Methodology: "scrum",
			},
			want: []string{"kanban", "charter", "backlog", "sprint1"},
		},
		{
			name: "fallback",
			request: SeedRequest{
				Industry:    "unrecognised",
				Methodology: "unrecognised",
			},
			want: []string{"charter"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := engine.Evaluate(context.Background(), test.request)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if !reflect.DeepEqual(response.Seeds, test.want) {
				t.Errorf("seeds = %#v, want %#v", response.Seeds, test.want)
			}
		})
	}
}
