// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"encoding/json"
	"testing"
)

// --- registry ---

func TestAll_Returns25Definitions(t *testing.T) {
	all := All()
	if len(all) != 25 {
		t.Errorf("All() = %d definitions, want 25", len(all))
	}
}

func TestAll_ReturnsCopy_NotMutable(t *testing.T) {
	all := All()
	original := all[0].Kind
	all[0] = Definition{}
	all2 := All()
	if all2[0].Kind != original {
		t.Error("All() returned a reference to the registry instead of a copy")
	}
}

func TestAll_KindsMatchGetLookup(t *testing.T) {
	for _, d := range All() {
		got, ok := Get(d.Kind)
		if !ok {
			t.Errorf("Get(%q) = false, but kind is in All()", d.Kind)
			continue
		}
		if got.Name != d.Name {
			t.Errorf("Get(%q).Name = %q, want %q", d.Kind, got.Name, d.Name)
		}
	}
}

func TestGet_KnownKind_ReturnsDefinition(t *testing.T) {
	d, ok := Get(KindProjectCharterWord)
	if !ok {
		t.Fatal("Get(KindProjectCharterWord) = false")
	}
	if d.Kind != KindProjectCharterWord {
		t.Errorf("Kind = %q, want %q", d.Kind, KindProjectCharterWord)
	}
	if d.Name == "" {
		t.Error("Name is empty")
	}
}

func TestGet_UnknownKind_ReturnsFalse(t *testing.T) {
	_, ok := Get("no_such_kind")
	if ok {
		t.Error(`Get("no_such_kind") = true, want false`)
	}
}

func TestByPhase_SumEqualsAll(t *testing.T) {
	phases := []Phase{PhaseInitiation, PhasePlanning, PhaseExecution, PhaseMonitoring, PhaseClosing}
	total := 0
	for _, p := range phases {
		total += len(ByPhase(p))
	}
	if total != len(All()) {
		t.Errorf("ByPhase sum across all phases = %d, want %d", total, len(All()))
	}
}

// --- DefaultContent ---

// TestDefaultContent_AllKindsProduceValidJSON verifies that DefaultContent
// returns parseable JSON for every registered kind, including the two
// Word/Excel alias pairs whose Fields lists are resolved at runtime.
func TestDefaultContent_AllKindsProduceValidJSON(t *testing.T) {
	for _, d := range All() {
		t.Run(string(d.Kind), func(t *testing.T) {
			content := DefaultContent(d.Kind)
			if content == "" {
				t.Fatal("DefaultContent returned empty string")
			}
			var v map[string]any
			if err := json.Unmarshal([]byte(content), &v); err != nil {
				t.Fatalf("DefaultContent returned invalid JSON: %v\ncontent: %s", err, content)
			}
		})
	}
}

func TestDefaultContent_UnknownKind_ReturnsBraces(t *testing.T) {
	got := DefaultContent("no_such_kind")
	if got != "{}" {
		t.Errorf("DefaultContent(unknown) = %q, want {}", got)
	}
}

// --- Render smoke test ---

// TestRender_AllKindsProduceValidPDF mirrors TestLayout_AllKindsHaveDataExample
// in the charts package. It uses DefaultContent as the seed so coverage expands
// automatically when new kinds are added, and asserts that every registered kind
// produces a non-empty PDF with a valid %%PDF- header without panicking.
//
// This is the primary regression guard against renderer dispatch gaps and
// nil-pointer panics on zero-value content.
func TestRender_AllKindsProduceValidPDF(t *testing.T) {
	for _, d := range All() {
		t.Run(string(d.Kind), func(t *testing.T) {
			content := DefaultContent(d.Kind)
			pdf, err := Render(d.Kind, content, "Smoke Test Project")
			if err != nil {
				t.Fatalf("Render(%s): %v", d.Kind, err)
			}
			if len(pdf) < 5 {
				t.Fatalf("Render(%s): PDF too short (%d bytes)", d.Kind, len(pdf))
			}
			if string(pdf[:5]) != "%PDF-" {
				t.Fatalf("Render(%s): missing %%PDF- header (got %q)", d.Kind, string(pdf[:5]))
			}
		})
	}
}
