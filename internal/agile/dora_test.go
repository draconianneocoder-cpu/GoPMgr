// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"math"
	"testing"
	"time"
)

// ----- Classification threshold tests -----

func TestClassifyDeployFrequency(t *testing.T) {
	tests := []struct {
		name      string
		perDay    float64
		n         int
		wantClass DORAClass
	}{
		{"no deployments", 0, 0, DORAClassUnknown},
		{"elite: multiple per day", 2.0, 60, DORAClassElite},
		{"elite: exactly 1/day", 1.0, 30, DORAClassElite},
		{"high: weekly boundary (1/7)", 1.0 / 7.0, 4, DORAClassHigh},
		{"high: twice a week", 0.3, 9, DORAClassHigh}, // 0.3/day = ~2/wk, < 1/day
		{"medium: monthly boundary (1/30)", 1.0 / 30.0, 1, DORAClassMedium},
		{"medium: bi-weekly (1/14)", 1.0 / 14.0, 2, DORAClassMedium}, // 1/14 < 1/7
		{"low: below monthly", 0.01, 1, DORAClassLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDeployFrequency(tt.perDay, tt.n)
			if got != tt.wantClass {
				t.Errorf("got %q, want %q", got, tt.wantClass)
			}
		})
	}
}

func TestClassifyLeadTime(t *testing.T) {
	tests := []struct {
		name      string
		hours     float64
		n         int
		wantClass DORAClass
	}{
		{"no data", 0, 0, DORAClassUnknown},
		{"elite: 1 hour", 1, 5, DORAClassElite},
		{"elite: exactly 24h", 24, 5, DORAClassElite},
		{"high: 25h", 25, 5, DORAClassHigh},
		{"high: exactly 1 week", 24 * 7, 5, DORAClassHigh},
		{"medium: just over 1 week", 24*7 + 1, 5, DORAClassMedium},
		{"medium: exactly 30 days", 24 * 30, 5, DORAClassMedium},
		{"low: 31 days", 24 * 31, 5, DORAClassLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLeadTime(tt.hours, tt.n)
			if got != tt.wantClass {
				t.Errorf("got %q, want %q", got, tt.wantClass)
			}
		})
	}
}

func TestClassifyCFR(t *testing.T) {
	tests := []struct {
		name      string
		rate      float64
		n         int
		wantClass DORAClass
	}{
		{"no deployments", 0, 0, DORAClassUnknown},
		{"elite: 0%", 0.0, 10, DORAClassElite},
		{"elite: exactly 15%", 0.15, 10, DORAClassElite},
		{"medium: 16%", 0.16, 10, DORAClassMedium},
		{"medium: exactly 30%", 0.30, 10, DORAClassMedium},
		{"low: 31%", 0.31, 10, DORAClassLow},
		{"low: 100%", 1.0, 5, DORAClassLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCFR(tt.rate, tt.n)
			if got != tt.wantClass {
				t.Errorf("got %q, want %q", got, tt.wantClass)
			}
		})
	}
}

func TestClassifyMTTR(t *testing.T) {
	tests := []struct {
		name      string
		hours     float64
		n         int
		wantClass DORAClass
	}{
		{"no data", 0, 0, DORAClassUnknown},
		{"elite: 30 minutes", 0.5, 3, DORAClassElite},
		{"elite: exactly 1h", 1.0, 3, DORAClassElite},
		{"high: 2h", 2, 3, DORAClassHigh},
		{"high: exactly 24h", 24, 3, DORAClassHigh},
		{"medium: 25h", 25, 3, DORAClassMedium},
		{"medium: exactly 1 week", 24 * 7, 3, DORAClassMedium},
		{"low: over 1 week", 24*7 + 1, 3, DORAClassLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMTTR(tt.hours, tt.n)
			if got != tt.wantClass {
				t.Errorf("got %q, want %q", got, tt.wantClass)
			}
		})
	}
}

// ----- median helper tests -----

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		xs   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"two elements", []float64{3, 7}, 5},
		{"odd count", []float64{1, 3, 5}, 3},
		{"even count", []float64{1, 2, 3, 4}, 2.5},
		{"unsorted input", []float64{9, 1, 5}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := median(tt.xs)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ----- formatFloat1 helper tests -----

func TestFormatFloat1(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{1.5, "1.5"},
		{2.05, "2.1"}, // rounds up
		{-3.2, "-3.2"},
		{10, "10"},
	}
	for _, tt := range tests {
		got := formatFloat1(tt.in)
		if got != tt.want {
			t.Errorf("formatFloat1(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ----- ComputeDORA integration tests -----

func TestComputeDORAEmpty(t *testing.T) {
	res := ComputeDORA(nil, 30, time.Now())
	if res.TotalDeploys != 0 {
		t.Errorf("expected 0 deploys, got %d", res.TotalDeploys)
	}
	if res.DeployFrequency.Class != DORAClassUnknown {
		t.Errorf("empty: deploy frequency class should be unknown")
	}
	if res.LeadTime.Class != DORAClassUnknown {
		t.Errorf("empty: lead time class should be unknown")
	}
	if res.ChangeFailureRate.Class != DORAClassUnknown {
		t.Errorf("empty: CFR class should be unknown")
	}
	if res.MTTR.Class != DORAClassUnknown {
		t.Errorf("empty: MTTR class should be unknown")
	}
}

func TestComputeDORAWindowFiltering(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deploys := []Deployment{
		{ID: "d1", TS: now.AddDate(0, 0, -31), Successful: true}, // outside window
		{ID: "d2", TS: now.AddDate(0, 0, -15), Successful: true}, // inside
		{ID: "d3", TS: now.AddDate(0, 0, -1), Successful: true},  // inside
	}
	res := ComputeDORA(deploys, 30, now)
	if res.TotalDeploys != 2 {
		t.Errorf("expected 2 deploys in window, got %d", res.TotalDeploys)
	}
}

func TestComputeDORADefaultWindowDays(t *testing.T) {
	// windowDays <= 0 falls back to 30.
	res := ComputeDORA(nil, 0, time.Now())
	if res.WindowDays != 30 {
		t.Errorf("expected window 30, got %d", res.WindowDays)
	}
}

func TestComputeDORAEliteTeam(t *testing.T) {
	// Simulate an elite DORA team: daily deploys, <1h lead time, <15% CFR, <1h MTTR.
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	var deploys []Deployment
	for i := range 30 {
		deploys = append(deploys, Deployment{
			ID:            "ok" + itoaW(int64(i)),
			TS:            now.AddDate(0, 0, -i),
			Successful:    true,
			LeadTimeHours: 0.5, // 30 minutes
		})
	}
	// Two failures with fast restore.
	deploys = append(deploys,
		Deployment{ID: "f1", TS: now.AddDate(0, 0, -5), Successful: false, RestoreTimeHours: 0.5},
		Deployment{ID: "f2", TS: now.AddDate(0, 0, -10), Successful: false, RestoreTimeHours: 0.5},
	)

	res := ComputeDORA(deploys, 30, now)

	if res.DeployFrequency.Class != DORAClassElite {
		t.Errorf("deploy frequency: got %q, want elite", res.DeployFrequency.Class)
	}
	if res.LeadTime.Class != DORAClassElite {
		t.Errorf("lead time: got %q, want elite", res.LeadTime.Class)
	}
	if res.MTTR.Class != DORAClassElite {
		t.Errorf("MTTR: got %q, want elite", res.MTTR.Class)
	}
}

func TestComputeDORADailyTrendLength(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	res := ComputeDORA(nil, 7, now)
	// Trend should span 8 points: day -7 through day 0 inclusive.
	if len(res.DailyDeployTrend) != 8 {
		t.Errorf("expected 8 trend points for 7-day window, got %d", len(res.DailyDeployTrend))
	}
}

func TestFormatHours(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{0, "—"},
		{-1, "—"},
		{0.5, "30 min"},
		{2, "2 h"},
		{72, "3 d"},
		{800, "4.8 wk"},
	}
	for _, tt := range tests {
		got := formatHours(tt.v)
		if got != tt.want {
			t.Errorf("formatHours(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestComputeDORAZeroNowFallsBack(t *testing.T) {
	res := ComputeDORA(nil, 7, time.Time{})
	if res.From.IsZero() {
		t.Error("ComputeDORA with zero now should use time.Now(), giving non-zero From")
	}
}

func TestComputeDORAChangeFailureRateMedium(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deploys := []Deployment{
		{ID: "ok1", TS: now.AddDate(0, 0, -1), Successful: true},
		{ID: "ok2", TS: now.AddDate(0, 0, -2), Successful: true},
		{ID: "ok3", TS: now.AddDate(0, 0, -3), Successful: true},
		{ID: "ok4", TS: now.AddDate(0, 0, -4), Successful: true},
		{ID: "ok5", TS: now.AddDate(0, 0, -5), Successful: true},
		{ID: "f1", TS: now.AddDate(0, 0, -6), Successful: false}, // 1/6 ≈ 16.7% > 15%
	}
	res := ComputeDORA(deploys, 30, now)
	if res.ChangeFailureRate.Class != DORAClassMedium {
		t.Errorf("CFR: got %q, want medium (rate ~16.7%%)", res.ChangeFailureRate.Class)
	}
}
