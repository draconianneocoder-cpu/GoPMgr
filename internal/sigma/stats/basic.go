// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package stats

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/stat"
)

// DescriptiveResult holds basic statistics for a dataset.
type DescriptiveResult struct {
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	StdDev  float64 `json:"std_dev"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Count   int     `json:"count"`
}

// CalculateDescriptive computes mean, median, std dev, min, max.
func CalculateDescriptive(values []float64) (DescriptiveResult, error) {
	if len(values) == 0 {
		return DescriptiveResult{}, fmt.Errorf("stats: empty dataset")
	}

	mean := stat.Mean(values, nil)
	stdDev := stat.StdDev(values, nil)

	// Simple median
	sorted := make([]float64, len(values))
	copy(sorted, values)
	// In a real app, use a proper sort or gonum sort, but for MVP slice sort is fine
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	var median float64
	n := len(sorted)
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	min, max := sorted[0], sorted[n-1]

	return DescriptiveResult{
		Mean:   mean,
		Median: median,
		StdDev: stdDev,
		Min:    min,
		Max:    max,
		Count:  n,
	}, nil
}

// CapabilityResult holds process capability indices.
type CapabilityResult struct {
	Cp   float64 `json:"cp"`
	Cpk  float64 `json:"cpk"`
	Pp   float64 `json:"pp"`
	Ppk  float64 `json:"ppk"`
	SigmaLevel float64 `json:"sigma_level"`
	DPMO float64 `json:"dpmo"`
}

// CalculateCapability computes Cp, Cpk, Pp, Ppk, Sigma Level, and DPMO.
func CalculateCapability(values []float64, usl, lsl float64) (CapabilityResult, error) {
	desc, err := CalculateDescriptive(values)
	if err != nil {
		return CapabilityResult{}, err
	}

	if desc.StdDev == 0 {
		return CapabilityResult{}, fmt.Errorf("stats: zero standard deviation")
	}

	// Cp = (USL - LSL) / 6σ
	cp := (usl - lsl) / (6 * desc.StdDev)

	// Cpk = min((USL - μ)/3σ, (μ - LSL)/3σ)
	cpkUSL := (usl - desc.Mean) / (3 * desc.StdDev)
	cpkLSL := (desc.Mean - lsl) / (3 * desc.StdDev)
	cpk := math.Min(cpkUSL, cpkLSL)

	// Pp / Ppk use overall std dev (here we approximate with sample std dev for MVP)
	pp := cp
	ppk := cpk

	// DPMO approximation (assuming centered process for MVP)
	// Z = 3 * Cpk
	z := 3 * cpk
	// Simple lookup approximation for Sigma Level
	// In a full app, use math.Erfc or a lookup table
	sigmaLevel := z + 1.5 // Shift assumption

	// DPMO = (1 - Yield) * 1,000,000
	// Approximation based on sigma level
	dpmo := 0.0
	if sigmaLevel >= 6 {
		dpmo = 3.4
	} else if sigmaLevel >= 5 {
		dpmo = 233
	} else if sigmaLevel >= 4 {
		dpmo = 6210
	} else if sigmaLevel >= 3 {
		dpmo = 66807
	} else if sigmaLevel >= 2 {
		dpmo = 308537
	} else {
		dpmo = 691462
	}

	return CapabilityResult{
		Cp:   cp,
		Cpk:  cpk,
		Pp:   pp,
		Ppk:  ppk,
		SigmaLevel: sigmaLevel,
		DPMO: dpmo,
	}, nil
}
