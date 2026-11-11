// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package templates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// rulesSHA256AtRecording is the hash of launchpad_seeds.json when the zen
// engine's answers were recorded. See TestEngineMatchesRecordedZenResults.
const rulesSHA256AtRecording = "853f3d1df100d78b978be92170aaf74d78afc57149301c19162b9ef2cd3dd324"

func mustEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine on the embedded table: %v", err)
	}
	return engine
}

// TestEngineMatchesRecordedZenResults replays every input that was put to
// gorules zen-go v0.20.0 on 2026-11-11, the day that engine was removed:
// all 729 pairs drawn from 27 values (every value in the table, blank,
// unknown names, and case, spacing, quoting and non-ASCII variants).
//
// The recording describes the table as it was then. When
// launchpad_seeds.json is edited on purpose, the recorded answers for the
// rows that changed no longer apply: update those entries by hand and
// update rulesSHA256AtRecording. The hash check exists so that such an
// edit is noticed here rather than passing or failing by accident.
func TestEngineMatchesRecordedZenResults(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256(rulesJSON)
	if got := hex.EncodeToString(sum[:]); got != rulesSHA256AtRecording {
		t.Fatalf("launchpad_seeds.json has changed (sha256 %s). Update the recorded results for the rows you changed, then rulesSHA256AtRecording.", got)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "launchpad_seeds.zen-0.20.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded []struct {
		Industry    string   `json:"industry"`
		Methodology string   `json:"methodology"`
		Seeds       []string `json:"seeds"`
	}
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if len(recorded) != 729 {
		t.Fatalf("recording has %d inputs, want 729", len(recorded))
	}
	engine := mustEngine(t)
	matched := 0
	for _, want := range recorded {
		got, err := engine.Evaluate(context.Background(), SeedRequest{Industry: want.Industry, Methodology: want.Methodology})
		if err != nil {
			t.Fatalf("Evaluate(%q, %q): %v", want.Industry, want.Methodology, err)
		}
		if !reflect.DeepEqual(got.Seeds, want.Seeds) {
			t.Errorf("Evaluate(%q, %q) = %v, zen gave %v", want.Industry, want.Methodology, got.Seeds, want.Seeds)
		}
		if !reflect.DeepEqual(want.Seeds, []string{"charter"}) {
			matched++
		}
	}
	// Guards against a recording that is all fallback, which any engine
	// returning the fallback would satisfy.
	if matched != 11 {
		t.Errorf("recording has %d inputs that match a rule, want 11 (one per non-fallback row)", matched)
	}
}

func TestEvaluateReturnsACopy(t *testing.T) {
	t.Parallel()
	engine := mustEngine(t)
	req := SeedRequest{Industry: "software", Methodology: "scrum"}
	first, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	first.Seeds[0] = "overwritten"
	second, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"kanban", "charter", "backlog", "sprint1"}; !reflect.DeepEqual(second.Seeds, want) {
		t.Errorf("after a caller changed its result, Evaluate = %v, want %v", second.Seeds, want)
	}
}

func TestEvaluateOnNilEngineReturnsError(t *testing.T) {
	t.Parallel()
	var engine *Engine
	if _, err := engine.Evaluate(context.Background(), SeedRequest{}); err == nil {
		t.Fatal("Evaluate on a nil *Engine returned no error")
	}
}

// The evaluator implements a small part of JDM. A table that uses any
// other part must be refused when it is loaded, not misread.
func TestNewEngineRefusesTablesItWouldMisread(t *testing.T) {
	t.Parallel()
	table := func(policy string, rows ...string) []byte {
		return []byte(`{"nodes":[{"type":"inputNode"},{"type":"decisionTableNode","content":{"hitPolicy":"` +
			policy + `","rules":[` + strings.Join(rows, ",") + `]}}]}`)
	}
	row := func(industry, methodology, seeds string) string {
		cells, _ := json.Marshal(map[string]string{"industry": industry, "methodology": methodology, "seeds": seeds})
		return string(cells)
	}
	good := row("'software'", "'scrum'", "['kanban']")
	fallback := row("", "", "['charter']")
	cases := []struct {
		name     string
		document []byte
		wantErr  string
	}{
		{"not JSON", []byte(`{`), "parse Launchpad rules"},
		{"no rows", []byte(`{"nodes":[{"type":"inputNode"}]}`), "contain no rows"},
		{"collect hit policy", table("collect", good, fallback), `hit policy "collect"`},
		{"fallback before the last row", table("first", fallback, good), "is not the last row"},
		{"wildcard industry", table("first", row("", "'scrum'", "['kanban']"), fallback), "wildcard rows are not supported"},
		{"wildcard methodology", table("first", row("'software'", "", "['kanban']"), fallback), "wildcard rows are not supported"},
		{"list of values", table("first", row("'software','business'", "'scrum'", "['kanban']")), "not a plain single-quoted string"},
		{"comparison", table("first", row("> 5", "'scrum'", "['kanban']")), "not a single-quoted string"},
		{"double-quoted value", table("first", row(`"software"`, "'scrum'", "['kanban']")), "not a single-quoted string"},
		{"unquoted value", table("first", row("software", "'scrum'", "['kanban']")), "not a single-quoted string"},
		{"seeds not a list", table("first", row("'software'", "'scrum'", "'kanban'")), "not a bracketed list"},
		{"empty seeds", table("first", row("'software'", "'scrum'", "[]")), "are empty"},
		{"unquoted seed", table("first", row("'software'", "'scrum'", "[kanban]")), "not a single-quoted string"},
		{"empty seed name", table("first", row("'software'", "'scrum'", "['']")), "empty name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			engine, err := newEngine(c.document)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("newEngine = %+v, %v; want an error containing %q", engine, err, c.wantErr)
			}
		})
	}

	// The same shapes, written correctly, load and evaluate.
	engine, err := newEngine(table("first", good, row(" 'business' ", "'lean'", " [ 'pareto' , 'swot' ] "), fallback))
	if err != nil {
		t.Fatalf("a well-formed table was refused: %v", err)
	}
	for _, c := range []struct {
		industry, methodology string
		want                  []string
	}{
		{"software", "scrum", []string{"kanban"}},
		{"business", "lean", []string{"pareto", "swot"}},
		{"business", "scrum", []string{"charter"}},
	} {
		got, err := engine.Evaluate(context.Background(), SeedRequest{Industry: c.industry, Methodology: c.methodology})
		if err != nil || !reflect.DeepEqual(got.Seeds, c.want) {
			t.Errorf("Evaluate(%q, %q) = %v, %v; want %v", c.industry, c.methodology, got.Seeds, err, c.want)
		}
	}

	// A table with no fallback row answers an unknown pair with no seeds.
	bare, err := newEngine(table("first", good))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := bare.Evaluate(context.Background(), SeedRequest{Industry: "x", Methodology: "y"}); err != nil || got.Seeds != nil {
		t.Errorf("no-fallback table: Evaluate = %v, %v; want no seeds", got.Seeds, err)
	}
}
