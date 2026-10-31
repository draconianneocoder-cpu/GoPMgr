// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

func TestNativeConcurrentResourceAccounting(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the fresh native worker")
	}
	type cohort struct {
		Workers int      `json:"workers"`
		Results []Result `json:"results"`
	}
	var evidence []cohort
	for _, count := range []int{2, 4, 8} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			started := make(chan int, count)
			release := make(chan struct{})
			results := make(chan Result, count)
			// Each launched child waits for bootstrap while its host waits here.
			// Release and collect every Run even if one launch fails or times out.
			for i := 0; i < count; i++ {
				go func(i int) {
					verifier := &GenVerifier{Width: 32}
					results <- Run(Job{Name: fmt.Sprintf("resources-%d-%d", count, i), Worker: worker, Timeout: 30 * time.Second, Verify: verifier.Check,
						Execute: &df0pb.Execute{Operation: "generate", Rows: 128, BatchRows: 32, PayloadBytes: 32},
						OnSpawn: func(pid int, _ string) { started <- pid; <-release },
					})
				}(i)
			}
			timer := time.NewTimer(10 * time.Second)
			launched := 0
		wait:
			for launched < count {
				select {
				case <-started:
					launched++
				case <-timer.C:
					break wait
				}
			}
			timer.Stop()
			close(release)
			var collected []Result
			for i := 0; i < count; i++ {
				collected = append(collected, <-results)
			}
			if launched != count {
				t.Fatalf("only %d/%d children reached barrier: %+v", launched, count, collected)
			}
			evidence = append(evidence, cohort{Workers: count, Results: collected})
			for _, r := range collected {
				if r.Outcome != "completed" || r.Rows != 128 || r.DigestMatch == nil || !*r.DigestMatch || r.WorkerPeakRSS <= 0 || r.WorkerUserNS < 0 || r.WorkerSystemNS < 0 || r.WorkerResourceMethod != "host wait4 child rusage" || !r.EndpointRemoved || r.WorkerExit != "exit 0" {
					t.Fatalf("concurrent child accounting/result/cleanup: %+v", r)
				}
				t.Logf("%s peak_bytes=%d user_ns=%d system_ns=%d", r.Name, r.WorkerPeakRSS, r.WorkerUserNS, r.WorkerSystemNS)
			}
		})
	}
	if path := os.Getenv("DF0_RESOURCE_EVIDENCE"); path != "" {
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
