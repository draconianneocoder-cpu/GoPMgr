// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package charts

import (
	"fmt"
	"sort"
)

// ParetoItem represents one bar in a Pareto chart.
type ParetoItem struct {
	Category             string  `json:"category"`
	Count                int     `json:"count"`
	Percentage           float64 `json:"percentage"`
	CumulativePercentage float64 `json:"cumulative_percentage"`
}

// CalculatePareto sorts categories by count descending and computes percentages.
func CalculatePareto(categories []string, counts []int) ([]ParetoItem, error) {
	if len(categories) != len(counts) {
		return nil, fmt.Errorf("pareto: categories and counts length mismatch")
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("pareto: empty input")
	}

	type pair struct {
		cat   string
		count int
	}
	pairs := make([]pair, len(categories))
	total := 0
	for i := range categories {
		pairs[i] = pair{categories[i], counts[i]}
		total += counts[i]
	}

	if total == 0 {
		return nil, fmt.Errorf("pareto: total count is zero")
	}

	// Sort descending by count
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	out := make([]ParetoItem, len(pairs))
	cumulative := 0
	for i, p := range pairs {
		cumulative += p.count
		out[i] = ParetoItem{
			Category:             p.cat,
			Count:                p.count,
			Percentage:           float64(p.count) / float64(total) * 100,
			CumulativePercentage: float64(cumulative) / float64(total) * 100,
		}
	}
	return out, nil
}
