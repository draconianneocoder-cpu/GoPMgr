// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

func TestNativeAdmissionCohortsRejectExcessAndRecover(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the fresh native worker")
	}
	type cohort struct {
		Limits   Budget   `json:"limits"`
		Claim    Claim    `json:"claim"`
		Held     Budget   `json:"held_at_barrier"`
		Rejected Result   `json:"rejected"`
		Admitted []Result `json:"admitted"`
		Recovery Result   `json:"recovery"`
		Final    Budget   `json:"final_reservations"`
	}
	var evidence []cohort
	for _, n := range []int{2, 4, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			limits := Budget{Jobs: uint64(n), MemoryBytes: uint64(n) * 64 << 20, CPUSlots: uint64(n)}
			claim := Claim{MemoryBytes: 64 << 20, CPUSlots: 1}
			a, err := NewAdmission(limits)
			if err != nil {
				t.Fatal(err)
			}
			started, release, results := make(chan int, n), make(chan struct{}), make(chan Result, n)
			var once sync.Once
			var admitted []Result
			releaseAll := func() { once.Do(func() { close(release) }) }
			collect := func() {
				for len(admitted) < n {
					admitted = append(admitted, <-results)
				}
			}
			t.Cleanup(func() { releaseAll(); collect() })
			job := func(name string) Job {
				v := &GenVerifier{Width: 32}
				return Job{Name: name, Worker: worker, Timeout: 30 * time.Second, Verify: v.Check, Execute: &df0pb.Execute{Operation: "generate", Rows: 128, BatchRows: 32, PayloadBytes: 32}}
			}
			for i := 0; i < n; i++ {
				go func(i int) {
					j := job(fmt.Sprintf("admitted-%d-%d", n, i))
					j.OnSpawn = func(pid int, _ string) { started <- pid; <-release }
					results <- a.Run(j, claim)
				}(i)
			}
			timer := time.NewTimer(10 * time.Second)
			launched := 0
		wait:
			for launched < n {
				select {
				case <-started:
					launched++
				case <-timer.C:
					break wait
				}
			}
			timer.Stop()
			held := a.InUse()
			// A missing executable makes a quota bypass fail differently from rejection.
			rejected := a.Run(Job{Name: "over-capacity", Worker: filepath.Join(t.TempDir(), "missing")}, claim)
			releaseAll()
			collect()
			if launched != n || held != limits || rejected.Outcome != "admission_rejected" || rejected.FailedCode != "RESOURCE_LIMIT" || rejected.WorkerPID != 0 {
				t.Fatalf("quota/launch barrier: launched=%d held=%+v rejected=%+v", launched, held, rejected)
			}
			verify := func(r Result) {
				t.Helper()
				if r.Outcome != "completed" || r.Rows != 128 || r.DigestMatch == nil || !*r.DigestMatch || !r.EndpointRemoved || r.WorkerExit != "exit 0" || r.WorkerPeakRSS <= 0 || r.WorkerResourceMethod != "host wait4 child rusage" {
					t.Fatalf("admitted worker failed: %+v", r)
				}
			}
			for _, r := range admitted {
				verify(r)
			}
			if a.InUse() != (Budget{}) {
				t.Fatalf("cohort stranded reservations: %+v", a.InUse())
			}
			recovery := a.Run(job("recovery"), claim)
			verify(recovery)
			if a.InUse() != (Budget{}) {
				t.Fatalf("retry stranded reservations: %+v", a.InUse())
			}
			evidence = append(evidence, cohort{Limits: limits, Claim: claim, Held: held, Rejected: rejected, Admitted: admitted, Recovery: recovery, Final: a.InUse()})
		})
	}
	if out := os.Getenv("DF0_ADMISSION_EVIDENCE"); out != "" {
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
