// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"strings"
	"testing"
	"time"
)

const cpmRaw = `{"nodes":[{"id":"A","label":"A","duration":2}],"edges":[]}`

func TestLayoutWithSchedule_CPMEmitsAnchoredDates(t *testing.T) {
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	res, err := LayoutWithSchedule(KindCPM, cpmRaw, start, nil, nil)
	if err != nil {
		t.Fatalf("LayoutWithSchedule: %v", err)
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
	res, err := LayoutWithSchedule(KindCPM, cpmRaw, time.Time{}, nil, nil)
	if err != nil {
		t.Fatalf("LayoutWithSchedule: %v", err)
	}
	if strings.Contains(string(res.Body), "start_date") {
		t.Errorf("un-anchored layout must not carry start_date:\n%s", res.Body)
	}
}

func TestLayoutWithSchedule_NonCPMDelegates(t *testing.T) {
	raw := `{"root":{"id":"1","title":"Project"}}`
	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	res, err := LayoutWithSchedule(KindWBS, raw, start, nil, nil)
	if err != nil {
		t.Fatalf("LayoutWithSchedule(WBS): %v", err)
	}
	plain, err := Layout(KindWBS, raw)
	if err != nil {
		t.Fatalf("Layout(WBS): %v", err)
	}
	if string(res.Body) != string(plain.Body) {
		t.Error("non-CPM kinds must produce identical output via either entry point")
	}
}
