// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gopmgr/internal/charts/dag"
	"gopmgr/internal/kernel"
)

const cpmRaw = `{"nodes":[{"id":"A","label":"A","duration":2}],"edges":[]}`

func TestLayoutWithSchedule_CPMEmitsAnchoredDates(t *testing.T) {
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	res, err := LayoutWithSchedulePlan(KindCPM, cpmRaw, start, nil, kernel.ResourceCapacityPlan{DefaultCapacity: 1}, "")
	if err != nil {
		t.Fatalf("LayoutWithSchedulePlan: %v", err)
	}
	body := string(res.Body)
	if !strings.Contains(body, `"start_date":"2026-06-05"`) {
		t.Errorf("start_date missing from body:\n%s", body)
	}
	if !strings.Contains(body, `"finish_date":"2026-06-06"`) {
		t.Errorf("finish_date missing from body:\n%s", body)
	}
}

func TestLayoutWithSchedule_ZeroStartFallsBack(t *testing.T) {
	res, err := LayoutWithSchedulePlan(KindCPM, cpmRaw, time.Time{}, nil, kernel.ResourceCapacityPlan{DefaultCapacity: 1}, "")
	if err != nil {
		t.Fatalf("LayoutWithSchedulePlan: %v", err)
	}
	if strings.Contains(string(res.Body), "start_date") {
		t.Errorf("un-anchored layout must not carry start_date:\n%s", res.Body)
	}
}

func TestLayoutWithSchedule_NonCPMDelegates(t *testing.T) {
	raw := `{"root":{"id":"1","title":"Project"}}`
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	res, err := LayoutWithSchedulePlan(KindWBS, raw, start, nil, kernel.ResourceCapacityPlan{DefaultCapacity: 1}, "")
	if err != nil {
		t.Fatalf("LayoutWithSchedulePlan(WBS): %v", err)
	}
	plain, err := Layout(KindWBS, raw)
	if err != nil {
		t.Fatalf("Layout(WBS): %v", err)
	}
	if string(res.Body) != string(plain.Body) {
		t.Error("non-CPM kinds must produce identical output via either entry point")
	}
}

// With project context the CPM and Gantt layouts report in their annotated
// document whether the chart's scheduling inputs changed since levelling
// (owner decision 2026-11-21); the layouts take the document by value, so
// the engine sets it. Plain Layout has no project context and never
// reports it (unknown, not stale).
func TestLayoutsReportStaleLevelling(t *testing.T) {
	const context = `{"start":"2026-06-01"}`
	levelled := dag.LayeredDocument{
		Nodes: []dag.LayeredNode{{ID: "A", Label: "A", Duration: 2, LevelStart: "2026-06-03"}},
	}
	digest := dag.LevellingInputsDigest(levelled, context)
	raw := func(duration float64) string {
		return fmt.Sprintf(`{"nodes":[{"id":"A","label":"A","duration":%v,"level_start":"2026-06-03"}],"edges":[],"levelled_inputs":%q}`, duration, digest)
	}
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, kind := range []Kind{KindCPM, KindGantt} {
		for _, anchored := range []bool{false, true} {
			for duration, changed := range map[float64]bool{2: false, 3: true} {
				var res LayoutResult
				var err error
				if anchored {
					res, err = LayoutWithSchedulePlan(kind, raw(duration), start, nil, kernel.ResourceCapacityPlan{DefaultCapacity: 1}, context)
				} else {
					res, err = Layout(kind, raw(duration))
				}
				if err != nil {
					t.Fatalf("%s anchored=%v: %v", kind, anchored, err)
				}
				var body struct {
					Doc struct {
						LevelledInputs string `json:"levelled_inputs"`
						LevellingStale bool   `json:"levelling_stale"`
					} `json:"doc"`
				}
				if err := json.Unmarshal(res.Body, &body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				want := anchored && changed
				if body.Doc.LevellingStale != want || body.Doc.LevelledInputs != digest {
					t.Errorf("%s anchored=%v duration %v: stale %v (stored %q), want stale %v with the stored digest kept",
						kind, anchored, duration, body.Doc.LevellingStale, body.Doc.LevelledInputs, want)
				}
			}
		}
	}
}
