// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"strings"
	"testing"

	"gopmgr/internal/kernel"
)

func TestGenerateMonteCarloRiskReportProducesPDFAReport(t *testing.T) {
	result := kernel.SimResult{
		Valid:      true,
		Iterations: 500,
		Workers:    4,
		P50:        12.4,
		P80:        15.2,
		P90:        18.7,
		FinishCDF: []kernel.ProbabilityPoint{
			{Day: 10, Probability: 0},
			{Day: 14, Probability: 0.5},
			{Day: 19, Probability: 1},
		},
		TornadoDrivers: []kernel.TornadoDriver{
			{
				TaskID:            "Build",
				CriticalFrequency: 0.92,
				P50Duration:       5.1,
				P80Duration:       7.3,
				P90Duration:       9.4,
				DurationSpread:    4.3,
				Score:             3.956,
			},
		},
	}

	out, err := GenerateMonteCarloRiskReport(MonteCarloRiskReportSpec{
		ProjectName: "Demo Project",
		ChartTitle:  "Risk Schedule",
		Result:      result,
	})
	if err != nil {
		t.Fatalf("GenerateMonteCarloRiskReport: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("report is not a PDF, first bytes: %q", out[:min(len(out), 8)])
	}
	if samplePath := os.Getenv("GOPMGR_MONTECARLO_REPORT_SAMPLE"); samplePath != "" {
		if err := os.WriteFile(samplePath, out, 0o600); err != nil {
			t.Fatalf("write report sample: %v", err)
		}
	}
	for _, want := range [][]byte{
		[]byte("Monte Carlo Risk Report"),
		[]byte("Demo Project"),
		[]byte("Risk Schedule"),
		[]byte("P50"),
		[]byte("Finish Probability S-curve"),
		[]byte("Tornado Risk Drivers"),
		[]byte("<pdfaid:part>2</pdfaid:part>"),
		[]byte("<pdfaid:conformance>U</pdfaid:conformance>"),
	} {
		if !bytes.Contains(out, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

// monteCarloText returns the strings a report draws. The report writes its
// content streams uncompressed; a compressed stream is inflated.
func monteCarloText(pdf []byte) string {
	var shown []string
	for _, m := range sigmaPDFStream.FindAllSubmatch(pdf, -1) {
		content := m[1]
		if r, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			if inflated, err := io.ReadAll(r); err == nil {
				content = inflated
			}
		}
		for _, tj := range sigmaPDFTj.FindAllSubmatch(content, -1) {
			shown = append(shown, decodePDFString(tj[1]))
		}
	}
	return strings.Join(shown, " ")
}

// A failed, invalid, or empty simulation is refused rather than drawn.
func TestGenerateMonteCarloRiskReportRefusesUnusableResults(t *testing.T) {
	cases := []struct {
		name   string
		result kernel.SimResult
		want   string
	}{
		{"failed run", kernel.SimResult{Error: "schedule has a cycle"}, "monte carlo risk report: schedule has a cycle"},
		{"invalid run", kernel.SimResult{}, "simulation result is invalid"},
		{"no iterations", kernel.SimResult{Valid: true}, "iterations must be positive"},
	}
	for _, c := range cases {
		out, err := GenerateMonteCarloRiskReport(MonteCarloRiskReportSpec{Result: c.result})
		if err == nil || out != nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %d bytes, error %v; want %q", c.name, len(out), err, c.want)
		}
	}
}

// With no project or chart name the report uses placeholders, and with no
// variable driver it says so instead of drawing an empty chart.
func TestGenerateMonteCarloRiskReportWithoutNamesOrDrivers(t *testing.T) {
	out, err := GenerateMonteCarloRiskReport(MonteCarloRiskReportSpec{Result: kernel.SimResult{
		Valid: true, Iterations: 200, P50: 10, P80: 10, P90: 10,
	}})
	if err != nil {
		t.Fatalf("GenerateMonteCarloRiskReport: %v", err)
	}
	if !bytes.Contains(out, []byte("GoPMgr Project")) || !bytes.Contains(out, []byte("CPM Schedule")) {
		t.Error("report does not name the placeholder project and chart")
	}
	text := monteCarloText(out)
	for _, want := range []string{"No variable risk drivers were detected.", "The leading tornado driver is no variable task."} {
		if !strings.Contains(text, want) {
			t.Errorf("report is missing %q", want)
		}
	}
}

// Drivers that all score zero still draw (each with the minimum bar), and
// a long task ID is cut to 16 characters.
func TestGenerateMonteCarloRiskReportWithZeroScoreDrivers(t *testing.T) {
	out, err := GenerateMonteCarloRiskReport(MonteCarloRiskReportSpec{ProjectName: "P", ChartTitle: "C", Result: kernel.SimResult{
		Valid: true, Iterations: 200, P50: 10, P80: 11, P90: 12,
		TornadoDrivers: []kernel.TornadoDriver{{TaskID: "Commissioning and handover", Score: 0}},
	}})
	if err != nil {
		t.Fatalf("GenerateMonteCarloRiskReport: %v", err)
	}
	text := monteCarloText(out)
	if !strings.Contains(text, "Commissioning a…") || !strings.Contains(text, "score 0.00") {
		t.Errorf("report does not list the zero-score driver, cut to 16 characters: %q", text)
	}
}

func TestMonteCarloScaleHelpers(t *testing.T) {
	// A run whose percentiles and curve sit on one day still gets a
	// one-day axis, so the chart does not divide by zero.
	if lo, hi := monteCarloDayDomain(kernel.SimResult{P50: 10, P90: 10}); lo != 9.5 || hi != 10.5 {
		t.Errorf("single-day domain = %v..%v, want 9.5..10.5", lo, hi)
	}
	if lo, hi := monteCarloDayDomain(kernel.SimResult{P50: 10, P90: 12, FinishCDF: []kernel.ProbabilityPoint{{Day: 8}, {Day: 15}}}); lo != 8 || hi != 15 {
		t.Errorf("domain = %v..%v, want the curve's 8..15", lo, hi)
	}
	for in, want := range map[float64]float64{-0.5: 0, 0: 0, 0.3: 0.3, 1: 1, 1.7: 1} {
		if got := clamp01(in); got != want {
			t.Errorf("clamp01(%v) = %v, want %v", in, got, want)
		}
	}
}
