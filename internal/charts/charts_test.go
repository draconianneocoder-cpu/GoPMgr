// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"encoding/json"
	"testing"
)

// --- registry ---

func TestAll_Returns20Definitions(t *testing.T) {
	all := All()
	if len(all) != 20 {
		t.Errorf("All() = %d definitions, want 20", len(all))
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
		if got.Engine != d.Engine {
			t.Errorf("Get(%q).Engine = %q, want %q", d.Kind, got.Engine, d.Engine)
		}
	}
}

func TestGet_KnownKind_ReturnsDefinition(t *testing.T) {
	def, ok := Get(KindWBS)
	if !ok {
		t.Fatal("Get(KindWBS) returned ok=false, want true")
	}
	if def.Kind != KindWBS {
		t.Errorf("def.Kind = %q, want %q", def.Kind, KindWBS)
	}
	if def.Engine != EngineDAG {
		t.Errorf("def.Engine = %q, want %q", def.Engine, EngineDAG)
	}
	if def.Name == "" {
		t.Error("def.Name is empty")
	}
}

func TestGet_UnknownKind_ReturnsFalse(t *testing.T) {
	_, ok := Get("does_not_exist")
	if ok {
		t.Error("Get(\"does_not_exist\") returned ok=true, want false")
	}
}

func TestByEngine_DAG_Returns6Kinds(t *testing.T) {
	defs := ByEngine(EngineDAG)
	if len(defs) != 6 {
		t.Errorf("ByEngine(EngineDAG) = %d, want 6", len(defs))
	}
}

func TestByEngine_Stats_Returns8Kinds(t *testing.T) {
	defs := ByEngine(EngineStats)
	if len(defs) != 8 {
		t.Errorf("ByEngine(EngineStats) = %d, want 8", len(defs))
	}
}

func TestByEngine_Matrix_Returns4Kinds(t *testing.T) {
	defs := ByEngine(EngineMatrix)
	if len(defs) != 4 {
		t.Errorf("ByEngine(EngineMatrix) = %d, want 4", len(defs))
	}
}

func TestByEngine_Flow_Returns2Kinds(t *testing.T) {
	defs := ByEngine(EngineFlow)
	if len(defs) != 2 {
		t.Errorf("ByEngine(EngineFlow) = %d, want 2", len(defs))
	}
}

func TestByEngine_UnknownEngine_ReturnsEmpty(t *testing.T) {
	defs := ByEngine("nonexistent")
	if len(defs) != 0 {
		t.Errorf("ByEngine(\"nonexistent\") = %d, want 0", len(defs))
	}
}

func TestByEngine_SumEqualsAll(t *testing.T) {
	total := len(ByEngine(EngineDAG)) + len(ByEngine(EngineStats)) +
		len(ByEngine(EngineMatrix)) + len(ByEngine(EngineFlow))
	all := len(All())
	if total != all {
		t.Errorf("sum of ByEngine counts = %d, want %d (len(All()))", total, all)
	}
}

// --- dispatcher (Layout) ---

func TestLayout_UnknownKind_ReturnsError(t *testing.T) {
	_, err := Layout("bogus_kind", "{}")
	if err == nil {
		t.Error("Layout(\"bogus_kind\") returned nil error, want error")
	}
}

func TestLayout_WBS_ReturnsLayoutResult(t *testing.T) {
	raw := `{"root":{"id":"1","title":"Project"}}`
	result, err := Layout(KindWBS, raw)
	if err != nil {
		t.Fatalf("Layout(KindWBS) error: %v", err)
	}
	if result.Engine != EngineDAG {
		t.Errorf("Engine = %q, want %q", result.Engine, EngineDAG)
	}
	if result.Kind != KindWBS {
		t.Errorf("Kind = %q, want %q", result.Kind, KindWBS)
	}
	if !json.Valid(result.Body) {
		t.Error("Body is not valid JSON")
	}
}

func TestLayout_Control_ReturnsLayoutResult(t *testing.T) {
	result, err := Layout(KindControl, "{}")
	if err != nil {
		t.Fatalf("Layout(KindControl) error: %v", err)
	}
	if result.Engine != EngineStats {
		t.Errorf("Engine = %q, want %q", result.Engine, EngineStats)
	}
	if result.Kind != KindControl {
		t.Errorf("Kind = %q, want %q", result.Kind, KindControl)
	}
	if !json.Valid(result.Body) {
		t.Error("Body is not valid JSON")
	}
}

func TestLayout_RACI_ReturnsLayoutResult(t *testing.T) {
	result, err := Layout(KindRACI, "{}")
	if err != nil {
		t.Fatalf("Layout(KindRACI) error: %v", err)
	}
	if result.Engine != EngineMatrix {
		t.Errorf("Engine = %q, want %q", result.Engine, EngineMatrix)
	}
	if result.Kind != KindRACI {
		t.Errorf("Kind = %q, want %q", result.Kind, KindRACI)
	}
	if !json.Valid(result.Body) {
		t.Error("Body is not valid JSON")
	}
}

func TestLayout_Workflow_ReturnsLayoutResult(t *testing.T) {
	result, err := Layout(KindWorkflow, "{}")
	if err != nil {
		t.Fatalf("Layout(KindWorkflow) error: %v", err)
	}
	if result.Engine != EngineFlow {
		t.Errorf("Engine = %q, want %q", result.Engine, EngineFlow)
	}
	if result.Kind != KindWorkflow {
		t.Errorf("Kind = %q, want %q", result.Kind, KindWorkflow)
	}
	if !json.Valid(result.Body) {
		t.Error("Body is not valid JSON")
	}
}

func TestLayout_Title_MatchesDefinitionName(t *testing.T) {
	raw := `{"root":{"id":"1","title":"Project"}}`
	result, err := Layout(KindWBS, raw)
	if err != nil {
		t.Fatalf("Layout(KindWBS) error: %v", err)
	}
	def, _ := Get(KindWBS)
	if result.Title != def.Name {
		t.Errorf("Title = %q, want %q", result.Title, def.Name)
	}
}

// TestLayout_AllKindsHaveDataExample smoke-tests that each kind's
// DataExample parses and lays out without error. This catches format
// drift between the registry documentation and the actual parsers.
func TestLayout_AllKindsHaveDataExample(t *testing.T) {
	for _, d := range All() {
		t.Run(string(d.Kind), func(t *testing.T) {
			_, err := Layout(d.Kind, d.DataExample)
			if err != nil {
				t.Errorf("Layout(%q, DataExample) error: %v", d.Kind, err)
			}
		})
	}
}
