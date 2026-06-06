// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"math"
	"strings"
	"testing"
)

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// ----- error paths -----

func TestCalculatePareto_EmptyInput(t *testing.T) {
	_, err := CalculatePareto(nil, nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error %q should mention 'empty'", err.Error())
	}
}

func TestCalculatePareto_LengthMismatch(t *testing.T) {
	_, err := CalculatePareto([]string{"A"}, []int{1, 2})
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("error %q should mention 'mismatch'", err.Error())
	}
}

func TestCalculatePareto_ZeroTotal(t *testing.T) {
	_, err := CalculatePareto([]string{"A", "B"}, []int{0, 0})
	if err == nil {
		t.Fatal("expected error for zero total")
	}
	if !strings.Contains(err.Error(), "zero") {
		t.Errorf("error %q should mention 'zero'", err.Error())
	}
}

// ----- single item -----

func TestCalculatePareto_SingleItem(t *testing.T) {
	items, err := CalculatePareto([]string{"X"}, []int{7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Category != "X" {
		t.Errorf("Category: got %q, want %q", items[0].Category, "X")
	}
	if items[0].Count != 7 {
		t.Errorf("Count: got %d, want 7", items[0].Count)
	}
	approx(t, "Percentage", items[0].Percentage, 100.0)
	approx(t, "CumulativePercentage", items[0].CumulativePercentage, 100.0)
}

// ----- sort descending -----

func TestCalculatePareto_SortDescendingByCount(t *testing.T) {
	items, err := CalculatePareto(
		[]string{"Low", "High", "Mid"},
		[]int{10, 50, 30},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Category != "High" || items[1].Category != "Mid" || items[2].Category != "Low" {
		t.Errorf("wrong sort order: %v %v %v",
			items[0].Category, items[1].Category, items[2].Category)
	}
}

// ----- percentage values -----

func TestCalculatePareto_PercentageValues(t *testing.T) {
	// A=50, B=30, C=20; total=100 — percentages are exact integers.
	items, err := CalculatePareto([]string{"A", "B", "C"}, []int{50, 30, 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approx(t, "items[0].Percentage", items[0].Percentage, 50.0)
	approx(t, "items[1].Percentage", items[1].Percentage, 30.0)
	approx(t, "items[2].Percentage", items[2].Percentage, 20.0)
}

// ----- cumulative percentage values -----

func TestCalculatePareto_CumulativePercentageValues(t *testing.T) {
	items, err := CalculatePareto([]string{"A", "B", "C"}, []int{50, 30, 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approx(t, "cum[0]", items[0].CumulativePercentage, 50.0)
	approx(t, "cum[1]", items[1].CumulativePercentage, 80.0)
	approx(t, "cum[2]", items[2].CumulativePercentage, 100.0)
}

// ----- last cumulative is always 100 -----

func TestCalculatePareto_LastCumulativeIs100(t *testing.T) {
	items, err := CalculatePareto(
		[]string{"A", "B", "C", "D"},
		[]int{3, 7, 5, 1},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	approx(t, "last CumulativePercentage", items[len(items)-1].CumulativePercentage, 100.0)
}

// ----- stable sort preserves input order for equal counts -----

func TestCalculatePareto_StableSort_EqualCounts(t *testing.T) {
	items, err := CalculatePareto([]string{"Alpha", "Beta"}, []int{5, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Category != "Alpha" || items[1].Category != "Beta" {
		t.Errorf("stable sort violated: got %q %q, want Alpha Beta",
			items[0].Category, items[1].Category)
	}
}

// ----- output length matches input -----

func TestCalculatePareto_ResultLength(t *testing.T) {
	items, err := CalculatePareto(
		[]string{"A", "B", "C", "D", "E"},
		[]int{10, 20, 5, 15, 50},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("expected 5 items, got %d", len(items))
	}
}
