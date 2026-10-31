// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build duckdb && (darwin || linux)

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gopmgr/internal/analytics"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/ipchost"
)

func TestDistribution(t *testing.T) {
	values := make([]int64, 30)
	for i := range values {
		values[i] = int64(30 - i)
	}
	d := distribution(values)
	if d.N != 30 || d.MinNS != 1 || d.P50NS != 15 || d.P95NS != 29 || d.MaxNS != 30 || d.MeanNS != 15.5 || d.VarianceNS2 != 899.0/12 || values[0] != 30 {
		t.Fatalf("nearest-rank statistics or input preservation: %+v, %v", d, values)
	}
	b, err := json.Marshal(d)
	if err != nil || !bytes.Contains(b, []byte(`"p99_ns":null`)) {
		t.Fatalf("p99 must remain unestimated: %s, %v", b, err)
	}
}

func TestWorkerCompletion(t *testing.T) {
	yes, no := true, false
	good := ipchost.Result{Outcome: "completed", WorkerExit: "exit 0", EndpointRemoved: true,
		DigestMatch: &yes, SourceMatch: &yes, WorkerPeakRSS: 1, WorkerUserNS: 1, WorkerSystemNS: 0}
	for _, field := range []string{"outcome", "exit", "cleanup", "digest", "source", "killed", "resources"} {
		t.Run(field, func(t *testing.T) {
			r := good
			switch field {
			case "outcome":
				r.Outcome = "timeout"
			case "exit":
				r.WorkerExit = "exit 1"
			case "cleanup":
				r.EndpointRemoved = false
			case "digest":
				r.DigestMatch = &no
			case "source":
				r.SourceMatch = nil
			case "killed":
				r.Killed = true
			case "resources":
				r.WorkerPeakRSS = -1
			}
			if err := checkWorker(r); err == nil {
				t.Fatalf("accepted %s failure", field)
			}
		})
	}
	if err := checkWorker(good); err != nil {
		t.Fatal(err)
	}
}

func TestOracleRejectsWrongSummary(t *testing.T) {
	c := fixtures.PortfolioCase{Projects: []fixtures.Project{{BudgetedCostMinorUnits: 101, EVMAvailable: true}}}
	got := analytics.PortfolioSummary{ProjectCount: 1, EVMProjectCount: 1, TotalBudgetedCostMinorUnits: 102, TotalBudgetedCost: 1.02}
	if err := checkSummary(got, c); err == nil {
		t.Fatal("wrong exact total passed")
	}
}

func TestNativePerformanceProviders(t *testing.T) {
	cases := selectedCases()
	if len(cases) != 3 || len(cases[0].Projects) != 0 || len(cases[1].Projects) != 100 || len(cases[2].Projects) != 1000 {
		t.Fatal("expected exactly the 0/100/1000-project fixtures")
	}
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("requires explicit DF0_WORKER")
	}
	for _, c := range cases {
		for _, provider := range providers {
			t.Run(c.ID+"/"+provider, func(t *testing.T) {
				s, err := measure(provider, worker, c)
				if err != nil {
					t.Fatal(err)
				}
				if s.WallNS <= 0 || s.HostCPUNS < 0 || s.OutputBytes == 0 {
					t.Fatalf("incomplete observation: %+v", s)
				}
			})
		}
	}
	if _, err := measure("rust", "/nonexistent/df0-worker", selectedCases()[0]); err == nil {
		t.Fatal("missing worker counted as a timing success")
	}
}

func TestRunRejectsIncompleteEvidence(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out, "/nonexistent/df0-worker", 29); err == nil || !strings.Contains(err.Error(), "30") || out.Len() != 0 {
		t.Fatalf("sample floor must fail before evidence: %v %s", err, out.Bytes())
	}
	if err := run(&out, "/nonexistent/df0-worker", 30); err == nil || bytes.Contains(out.Bytes(), []byte(`"type":"summary"`)) {
		t.Fatalf("provider failure must not produce a completion summary: %v", err)
	}
}
