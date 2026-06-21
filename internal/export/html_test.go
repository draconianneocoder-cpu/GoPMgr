// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"strings"
	"testing"

	"pmforge/internal/kernel"
)

func TestGenerateHTMLSchedule(t *testing.T) {
	payload := ReportPayload{
		Tasks: map[string]*kernel.Task{
			"t1": {ID: "t1", Title: "Design <phase>", Duration: 5, ES: 0, EF: 5, IsCritical: true},
			"t2": {ID: "t2", Title: "Build", Duration: 8, ES: 5, EF: 13},
		},
	}
	out, err := GenerateArchivalReport(payload, ExportOptions{Format: FormatHTML, Title: "Demo Project"})
	if err != nil {
		t.Fatalf("GenerateArchivalReport(html): %v", err)
	}
	s := string(out)
	for _, want := range []string{"<!DOCTYPE html>", "Demo Project", "Build", "2 tasks", "critical"} {
		if !strings.Contains(s, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	// User text must be HTML-escaped, not raw.
	if strings.Contains(s, "Design <phase>") {
		t.Error("task title was not HTML-escaped")
	}
	if !strings.Contains(s, "Design &lt;phase&gt;") {
		t.Error("expected escaped task title")
	}
}
