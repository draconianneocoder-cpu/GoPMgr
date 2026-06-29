// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"os"
	"testing"

	"pmforge/internal/kernel"
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
	if samplePath := os.Getenv("PMFORGE_MONTECARLO_REPORT_SAMPLE"); samplePath != "" {
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
		[]byte("<pdfaid:part>3</pdfaid:part>"),
		[]byte("<pdfaid:conformance>B</pdfaid:conformance>"),
	} {
		if !bytes.Contains(out, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}
