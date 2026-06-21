// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"math"
	"testing"
)

// within checks that got and want are within epsilon of each other.
func within(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// TestAnnotatePERTClassicValues verifies the textbook beta-distribution
// approximation formulas for PERT:
//
//	E = (O + 4M + P) / 6
//	V = ((P - O) / 6)^2
//	σ = sqrt(V)
//
// Using O=2, M=4, P=12:
//
//	E = (2 + 16 + 12) / 6 = 5
//	V = ((12-2)/6)^2 = (10/6)^2 = 100/36 ≈ 2.7778
//	σ = 10/6 ≈ 1.6667
func TestAnnotatePERTClassicValues(t *testing.T) {
	n := &LayeredNode{ID: "A", Optimistic: 2, MostLikely: 4, Pessimistic: 12}
	annotatePERT(n)

	within(t, "Expected", n.Expected, 5.0)
	within(t, "Variance", n.Variance, 100.0/36.0)
	within(t, "StdDev", n.StdDev, 10.0/6.0)
	within(t, "Duration", n.Duration, 5.0) // Duration is set to Expected
}

// TestAnnotatePERTCertainDuration verifies that when O = M = P = 3 the
// variance is zero and expected equals the certain duration.
func TestAnnotatePERTCertainDuration(t *testing.T) {
	n := &LayeredNode{ID: "B", Optimistic: 3, MostLikely: 3, Pessimistic: 3}
	annotatePERT(n)

	within(t, "Expected", n.Expected, 3.0)
	within(t, "Variance", n.Variance, 0.0)
	within(t, "StdDev", n.StdDev, 0.0)
	within(t, "Duration", n.Duration, 3.0)
}

// TestAnnotatePERTSymmetricRange verifies that for a symmetric range
// O=1, M=5, P=9:
//
//	E = (1 + 20 + 9) / 6 = 5
//	V = ((9-1)/6)^2 = (8/6)^2 = 64/36 ≈ 1.7778
func TestAnnotatePERTSymmetricRange(t *testing.T) {
	n := &LayeredNode{ID: "C", Optimistic: 1, MostLikely: 5, Pessimistic: 9}
	annotatePERT(n)

	within(t, "Expected", n.Expected, 5.0)
	within(t, "Variance", n.Variance, 64.0/36.0)
	within(t, "StdDev", n.StdDev, 8.0/6.0)
}

// TestAnnotatePERTAllZeroIsNoop verifies that when O, M, P are all zero
// (user hasn't entered estimates yet) the function makes no changes.
func TestAnnotatePERTAllZeroIsNoop(t *testing.T) {
	n := &LayeredNode{ID: "D"}
	annotatePERT(n)

	within(t, "Expected", n.Expected, 0.0)
	within(t, "Variance", n.Variance, 0.0)
	within(t, "StdDev", n.StdDev, 0.0)
	within(t, "Duration", n.Duration, 0.0)
}

// TestAnnotatePERTStdDevIsSquareRootOfVariance is a structural check:
// for any inputs StdDev must equal sqrt(Variance).
func TestAnnotatePERTStdDevIsSquareRootOfVariance(t *testing.T) {
	cases := []struct{ o, m, p float64 }{
		{1, 3, 7},
		{0, 5, 10},
		{2, 8, 20},
	}
	for _, c := range cases {
		n := &LayeredNode{Optimistic: c.o, MostLikely: c.m, Pessimistic: c.p}
		annotatePERT(n)
		want := math.Sqrt(n.Variance)
		if math.Abs(n.StdDev-want) > 1e-9 {
			t.Errorf("O=%.0f M=%.0f P=%.0f: StdDev=%v, sqrt(Variance)=%v",
				c.o, c.m, c.p, n.StdDev, want)
		}
	}
}

// TestAnnotatePERTDurationMatchesExpected is a structural check: Duration
// must always be set to Expected after annotation.
func TestAnnotatePERTDurationMatchesExpected(t *testing.T) {
	n := &LayeredNode{Optimistic: 3, MostLikely: 6, Pessimistic: 15}
	annotatePERT(n)
	if math.Abs(n.Duration-n.Expected) > 1e-9 {
		t.Errorf("Duration (%v) != Expected (%v)", n.Duration, n.Expected)
	}
}
