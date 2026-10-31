// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestAdmissionLimitsBeforeLaunch(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limits      Budget
		held, extra Claim
	}{
		{"jobs", Budget{1, 100, 100}, Claim{1, 1}, Claim{1, 1}},
		{"memory", Budget{3, 8, 100}, Claim{4, 1}, Claim{5, 1}},
		{"cpu", Budget{3, 100, 4}, Claim{1, 2}, Claim{1, 3}},
		{"memory_overflow", Budget{3, math.MaxUint64, 100}, Claim{math.MaxUint64 - 1, 1}, Claim{2, 1}},
		{"cpu_overflow", Budget{3, 100, math.MaxUint64}, Claim{1, math.MaxUint64 - 1}, Claim{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := NewAdmission(tc.limits)
			if err != nil {
				t.Fatal(err)
			}
			entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			var first Result
			t.Cleanup(func() { once.Do(func() { close(release) }); <-finished })
			go func() {
				defer close(finished)
				first = a.Run(Job{Name: "held", Worker: "/usr/bin/true", Timeout: 5 * time.Second, OnSpawn: func(int, string) { close(entered); <-release }}, tc.held)
			}()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("first job did not launch")
			}
			want := Budget{1, tc.held.MemoryBytes, tc.held.CPUSlots}
			if a.InUse() != want {
				t.Fatalf("reservation not held: %+v", a.InUse())
			}
			spawned := false
			rejected := a.Run(Job{Name: "excess", Worker: filepath.Join(t.TempDir(), "missing"), OnSpawn: func(int, string) { spawned = true }}, tc.extra)
			if rejected.Outcome != "admission_rejected" || rejected.FailedCode != "RESOURCE_LIMIT" || rejected.WorkerPID != 0 || spawned || a.InUse() != want || rejected.WorkerPeakRSS != -1 {
				t.Fatalf("quota did not refuse before launch: %+v usage=%+v", rejected, a.InUse())
			}
			once.Do(func() { close(release) })
			<-finished
			if first.WorkerExit == "" || !first.EndpointRemoved || a.InUse() != (Budget{}) {
				t.Fatalf("reservation/worker cleanup: %+v usage=%+v", first, a.InUse())
			}
			retry := a.Run(Job{Name: "retry", Worker: filepath.Join(t.TempDir(), "missing")}, tc.held)
			if retry.Outcome != "setup_failed" || a.InUse() != (Budget{}) {
				t.Fatalf("failed launch stranded capacity: %+v", retry)
			}
		})
	}
}

func TestAdmissionRejectsInvalidClaimsAndUsesExactCapacity(t *testing.T) {
	for _, limit := range []Budget{{}, {1, 0, 1}, {1, 1, 0}, {0, 1, 1}} {
		if _, err := NewAdmission(limit); err == nil {
			t.Fatalf("invalid capacity accepted: %+v", limit)
		}
	}
	a, err := NewAdmission(Budget{1, 64, 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []Claim{{}, {0, 1}, {1, 0}} {
		r := a.Run(Job{Name: "invalid", Worker: "/no/worker"}, c)
		if r.FailedCode != "INPUT_INVALID" || r.Outcome != "admission_rejected" || r.WorkerPID != 0 || a.InUse() != (Budget{}) {
			t.Fatalf("invalid claim admitted: %+v", r)
		}
	}
	r := a.Run(Job{Name: "exact", Worker: filepath.Join(t.TempDir(), "missing")}, Claim{64, 2})
	if r.Outcome != "setup_failed" || a.InUse() != (Budget{}) {
		t.Fatalf("exact capacity should reach launch: %+v", r)
	}
}

func TestAdmissionConcurrentContendersCannotOversubscribe(t *testing.T) {
	a, _ := NewAdmission(Budget{1, 64, 1})
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); <-finished })
	go func() {
		defer close(finished)
		a.Run(Job{Name: "holder", Worker: "/usr/bin/true", Timeout: 5 * time.Second, OnSpawn: func(int, string) { close(entered); <-release }}, Claim{64, 1})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("holder did not launch")
	}
	var wg sync.WaitGroup
	results := make(chan Result, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- a.Run(Job{Name: fmt.Sprint(i), Worker: "/no/worker"}, Claim{1, 1})
		}(i)
	}
	wg.Wait()
	close(results)
	for r := range results {
		if r.Outcome != "admission_rejected" || r.FailedCode != "RESOURCE_LIMIT" || r.WorkerPID != 0 {
			t.Fatalf("contender bypassed quota: %+v", r)
		}
	}
	if a.InUse() != (Budget{1, 64, 1}) {
		t.Fatalf("concurrent rejection changed reservation: %+v", a.InUse())
	}
	once.Do(func() { close(release) })
	<-finished
	if a.InUse() != (Budget{}) {
		t.Fatalf("stranded capacity: %+v", a.InUse())
	}
}

func TestAdmissionLaunchPanicReapsChildBeforeRelease(t *testing.T) {
	a, _ := NewAdmission(Budget{1, 64, 1})
	var pid int
	var endpoint string
	// The child waits on stdin. If the regression returns, clean it up even when
	// testing a deliberately broken disposable copy.
	t.Cleanup(func() {
		if pid > 0 && Alive(pid) {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			var status syscall.WaitStatus
			_, _ = syscall.Wait4(pid, &status, 0, nil)
		}
	})
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		a.Run(Job{Name: "panic", Worker: "/bin/cat", OnSpawn: func(p int, dir string) {
			pid, endpoint = p, dir
			if a.InUse() != (Budget{1, 64, 1}) {
				panic("reservation missing")
			}
			panic("launch hook")
		}}, Claim{64, 1})
	}()
	if recovered != "launch hook" || pid == 0 || Alive(pid) || a.InUse() != (Budget{}) {
		t.Fatalf("panic leaked worker/capacity: panic=%v pid=%d alive=%v usage=%+v", recovered, pid, Alive(pid), a.InUse())
	}
	if _, err := os.Stat(endpoint); !os.IsNotExist(err) {
		t.Fatalf("panic leaked endpoint: %v", err)
	}
}

func TestAdmissionConcurrentAcquisitionsShareOneCapacity(t *testing.T) {
	a, _ := NewAdmission(Budget{4, 4, 4})
	start, release := make(chan struct{}), make(chan struct{})
	entered, results := make(chan int, 32), make(chan Result, 32)
	var wg sync.WaitGroup
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { releaseAll(); wg.Wait() })
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- a.Run(Job{Name: fmt.Sprint(i), Worker: "/usr/bin/true", Timeout: 5 * time.Second, OnSpawn: func(pid int, _ string) { entered <- pid; <-release }}, Claim{1, 1})
		}(i)
	}
	close(start)
	launched, rejected := 0, 0
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for launched+rejected < 32 {
		select {
		case <-entered:
			launched++
		case r := <-results:
			if r.Outcome != "admission_rejected" || r.FailedCode != "RESOURCE_LIMIT" || r.WorkerPID != 0 {
				t.Fatalf("wrong contested rejection: %+v", r)
			}
			rejected++
		case <-timer.C:
			t.Fatal("contested launches did not settle")
		}
	}
	if launched != 4 || rejected != 28 || a.InUse() != (Budget{4, 4, 4}) {
		t.Fatalf("oversubscribed/shared capacity: launched=%d rejected=%d used=%+v", launched, rejected, a.InUse())
	}
	releaseAll()
	wg.Wait()
	for i := 0; i < launched; i++ {
		r := <-results
		if r.WorkerPID == 0 || r.WorkerExit == "" || !r.EndpointRemoved {
			t.Fatalf("admitted cleanup: %+v", r)
		}
	}
	if a.InUse() != (Budget{}) {
		t.Fatalf("contested jobs stranded capacity: %+v", a.InUse())
	}
}
