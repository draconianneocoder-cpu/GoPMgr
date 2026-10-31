// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResourceChild(t *testing.T) {
	if os.Getenv("DF0_RESOURCE_CHILD") != "1" {
		t.Skip("subprocess fixture")
	}
	data := make([]byte, 64<<20)
	for i := range data {
		data[i] = byte(i)
	}
	var sum uint64
	for n := 0; n < 8; n++ {
		for _, b := range data {
			sum += uint64(b)
		}
	}
	runtime.KeepAlive(data)
	if sum == 0 {
		t.Fatal("fixture did no work")
	}
}

func TestReapedChildResourcesUseNativeUnits(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestResourceChild$")
	cmd.Env = []string{"DF0_RESOURCE_CHILD=1"}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, output)
	}
	res := NewResult("resource-child")
	// A bogus worker report must not become resource-governance evidence.
	res.WorkerPeakRSS = math.MaxInt64
	recordProcessResources(&res, cmd.ProcessState)
	// The fixture touches 64 MiB. A broad ceiling allows instrumentation while
	// detecting the 1024x byte/KiB error independently of the conversion helper.
	if res.WorkerPeakRSS < 64<<20 || res.WorkerPeakRSS >= 4<<30 {
		t.Fatalf("wrong native memory units: %+v", res)
	}
	if res.WorkerUserNS <= 0 || res.WorkerSystemNS < 0 || res.WorkerResourceMethod != "host wait4 child rusage" {
		t.Fatalf("missing CPU/provenance: %+v", res)
	}
	t.Logf("child peak_bytes=%d user_ns=%d system_ns=%d", res.WorkerPeakRSS, res.WorkerUserNS, res.WorkerSystemNS)
}

func TestUnavailableAndInvalidResourcesStayExplicit(t *testing.T) {
	res := NewResult("not-started")
	if res.WorkerPeakRSS != -1 || res.WorkerUserNS != -1 || res.WorkerSystemNS != -1 {
		t.Fatalf("invented measurement: %+v", res)
	}
	notStarted := Run(Job{Name: "not-started", Worker: filepath.Join(t.TempDir(), "missing-worker")})
	if notStarted.Outcome != "setup_failed" || notStarted.WorkerPeakRSS != -1 || notStarted.WorkerUserNS != -1 || notStarted.WorkerSystemNS != -1 || notStarted.WorkerResourceMethod != "" || !notStarted.EndpointRemoved {
		t.Fatalf("invented startup measurement or leaked endpoint: %+v", notStarted)
	}
	res.WorkerPeakRSS = 0
	recordProcessResources(&res, nil)
	if res.WorkerPeakRSS != -1 || res.WorkerUserNS != -1 || res.WorkerSystemNS != -1 || res.WorkerResourceMethod != "" {
		t.Fatalf("unavailable resources accepted: %+v", res)
	}
	for _, raw := range []int64{-1, 0} {
		if rusagePeakBytes(raw) != -1 {
			t.Fatalf("invalid RSS accepted: %d", raw)
		}
	}
	if rusagePeakBytes(1) != rusageRSSScale {
		t.Fatal("wrong unit")
	}
	if rusageRSSScale > 1 {
		largest := int64(math.MaxInt64)
		if rusagePeakBytes(largest/rusageRSSScale+1) != -1 {
			t.Fatal("overflow accepted")
		}
	}
}
