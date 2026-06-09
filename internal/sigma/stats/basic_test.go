// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"math"
	"testing"
)

// tol is the absolute tolerance used for floating-point comparisons.
const tol = 1e-6

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) <= tol
}

// ----- CalculateDescriptive -----

func TestCalculateDescriptive_Empty(t *testing.T) {
	_, err := CalculateDescriptive(nil)
	if err == nil {
		t.Fatal("expected error for empty slice")
	}
}

func TestCalculateDescriptive_SingleValue(t *testing.T) {
	res, err := CalculateDescriptive([]float64{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEqual(res.Mean, 42) {
		t.Errorf("Mean: got %v, want 42", res.Mean)
	}
	if !approxEqual(res.Median, 42) {
		t.Errorf("Median: got %v, want 42", res.Median)
	}
	if !approxEqual(res.Min, 42) {
		t.Errorf("Min: got %v, want 42", res.Min)
	}
	if !approxEqual(res.Max, 42) {
		t.Errorf("Max: got %v, want 42", res.Max)
	}
	if res.Count != 1 {
		t.Errorf("Count: got %d, want 1", res.Count)
	}
}

func TestCalculateDescriptive_OddCount(t *testing.T) {
	// [1, 3, 5] → mean=3, median=3, min=1, max=5
	res, err := CalculateDescriptive([]float64{5, 1, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEqual(res.Mean, 3) {
		t.Errorf("Mean: got %v, want 3", res.Mean)
	}
	if !approxEqual(res.Median, 3) {
		t.Errorf("Median: got %v, want 3", res.Median)
	}
	if !approxEqual(res.Min, 1) {
		t.Errorf("Min: got %v, want 1", res.Min)
	}
	if !approxEqual(res.Max, 5) {
		t.Errorf("Max: got %v, want 5", res.Max)
	}
	if res.Count != 3 {
		t.Errorf("Count: got %d, want 3", res.Count)
	}
}

func TestCalculateDescriptive_EvenCount(t *testing.T) {
	// [1, 2, 3, 4] → mean=2.5, median=(2+3)/2=2.5
	res, err := CalculateDescriptive([]float64{4, 1, 3, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEqual(res.Mean, 2.5) {
		t.Errorf("Mean: got %v, want 2.5", res.Mean)
	}
	if !approxEqual(res.Median, 2.5) {
		t.Errorf("Median: got %v, want 2.5", res.Median)
	}
	if res.Count != 4 {
		t.Errorf("Count: got %d, want 4", res.Count)
	}
}

func TestCalculateDescriptive_StdDevPositive(t *testing.T) {
	// Any varied dataset must produce StdDev > 0.
	res, err := CalculateDescriptive([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StdDev <= 0 {
		t.Errorf("StdDev should be > 0 for varied data, got %v", res.StdDev)
	}
	// Mean of that dataset = (2+4+4+4+5+5+7+9)/8 = 40/8 = 5
	if !approxEqual(res.Mean, 5) {
		t.Errorf("Mean: got %v, want 5", res.Mean)
	}
}

// ----- CalculateCapability -----

func TestCalculateCapability_Empty(t *testing.T) {
	_, err := CalculateCapability(nil, 10, 0)
	if err == nil {
		t.Fatal("expected error for empty slice")
	}
}

func TestCalculateCapability_ZeroStdDev(t *testing.T) {
	// All identical values → std dev is zero → must error.
	_, err := CalculateCapability([]float64{5, 5, 5, 5}, 10, 0)
	if err == nil {
		t.Fatal("expected error for zero std dev")
	}
}

func TestCalculateCapability_CpFormula(t *testing.T) {
	// Cp = (USL - LSL) / (6 * σ).
	// Use values [4, 6] repeated to get σ = 1.0 (population) or ~1.414 (sample, n=2).
	// Use a dataset where we can control the std dev through volume.
	// Simpler: use [0,10] × 50 to get σ≈5 approximately, then check Cp ~ 20/30 = 0.667.
	// Actually, let's use a large uniform symmetric distribution.
	// Easier: use 100 uniformly spaced values from 5 to 15 (mean=10, σ≈2.906 sample).
	// USL=20, LSL=0 → Cp = 20 / (6*σ). Verify Cp > 0 and Cpk <= Cp.
	var values []float64
	for i := range 100 {
		values = append(values, float64(5)+float64(i)*10.0/99.0)
	}
	res, err := CalculateCapability(values, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Cp <= 0 {
		t.Errorf("Cp should be positive, got %v", res.Cp)
	}
	if res.Cpk > res.Cp {
		t.Errorf("Cpk (%v) should be <= Cp (%v)", res.Cpk, res.Cp)
	}
}

func TestCalculateCapability_CpkLessThanCpWhenOffCenter(t *testing.T) {
	// Skew the process mean toward USL to produce Cpk < Cp.
	// Values [8,9,10,11,12] clustered near USL=14, LSL=0 → Cpk < Cp.
	values := []float64{9, 9.5, 10, 10.5, 11, 11, 11.5, 12}
	res, err := CalculateCapability(values, 14, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mean ≈ 10.6, LSL=0, USL=14 → (14-10.6) = 3.4, (10.6-0) = 10.6
	// Cpk = min(3.4/(3σ), 10.6/(3σ)) = 3.4/(3σ) — smaller upper side
	if res.Cpk >= res.Cp {
		t.Errorf("off-center process: expected Cpk (%v) < Cp (%v)", res.Cpk, res.Cp)
	}
}

func TestCalculateCapability_DPMOBands(t *testing.T) {
	// Verify every sigma-level DPMO band, not just the top one.
	//
	// The dataset {-1, 1} has sample mean 0 and sample StdDev exactly √2
	// (variance = ((-1)^2 + 1^2)/(2-1) = 2). With a centered spec
	// USL = H, LSL = -H the code computes:
	//   cpk        = H / (3σ)
	//   sigmaLevel = 3*cpk + 1.5 = H/σ + 1.5
	// Choosing H = √2 * k makes sigmaLevel = k + 1.5 exactly. Each k below
	// lands its sigmaLevel at least 0.3 inside the target band, so float
	// rounding cannot flip a band boundary.
	values := []float64{-1, 1}
	tests := []struct {
		name       string
		k          float64 // sigmaLevel = k + 1.5
		wantSigma  float64
		wantDPMO   float64
	}{
		{"sigma>=6", 5.5, 7.0, 3.4},
		{"sigma>=5", 4.0, 5.5, 233},
		{"sigma>=4", 3.0, 4.5, 6210},
		{"sigma>=3", 2.0, 3.5, 66807},
		{"sigma>=2", 1.0, 2.5, 308537},
		{"sigma<2", 0.3, 1.8, 691462},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := math.Sqrt2 * tt.k
			res, err := CalculateCapability(values, h, -h)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(res.SigmaLevel-tt.wantSigma) > 1e-9 {
				t.Errorf("sigma_level: got %v, want %v", res.SigmaLevel, tt.wantSigma)
			}
			if res.DPMO != tt.wantDPMO {
				t.Errorf("DPMO: got %v, want %v (sigma=%v)", res.DPMO, tt.wantDPMO, res.SigmaLevel)
			}
		})
	}
}
