// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type usageSample struct {
	PID       int    `json:"pid"`
	Start     uint64 `json:"start_ticks"`
	RSS       uint64 `json:"rss_bytes"`
	Footprint uint64 `json:"footprint_bytes"`
	User      uint64 `json:"user_ns"`
	System    uint64 `json:"system_ns"`
	Before    uint64 `json:"before_ns"`
	After     uint64 `json:"after_ns"`
}

func compileObserverProbe(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs("../../tools/macos_observer_probe.c")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "observer")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "xcrun", "clang", "-std=c11", "-Wall", "-Wextra", "-Werror", src, "-o", out).CombinedOutput()
	if err != nil {
		t.Fatalf("compile observer: %v: %s", err, b)
	}
	return out
}

// One helper reads the requested PIDs sequentially. This is not an atomic
// machine snapshot; timestamps expose the skew. Reject partial observations.
func observeUsage(probe string, pids []int, starts map[int]uint64) ([]usageSample, error) {
	args := []string{"sample"}
	for _, pid := range pids {
		args = append(args, strconv.Itoa(pid))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, probe, args...)
	cmd.Env = []string{}
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("observe usage: %w", err)
	}
	var samples []usageSample
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		var s usageSample
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			return nil, err
		}
		i := len(samples)
		if i >= len(pids) || s.PID != pids[i] || s.Start == 0 || s.RSS == 0 || s.Footprint == 0 || s.Before == 0 || s.After < s.Before {
			return nil, fmt.Errorf("invalid usage observation")
		}
		if start, ok := starts[s.PID]; ok && start != s.Start {
			return nil, fmt.Errorf("process identity changed")
		}
		samples = append(samples, s)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(samples) != len(pids) {
		return nil, fmt.Errorf("incomplete usage observation")
	}
	return samples, nil
}

type observerChild struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *bufio.Scanner
	ctx    context.Context
}

func startObserverChild(t *testing.T, probe string) *observerChild {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(ctx, probe, "payload")
	cmd.Env = []string{}
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = input.Close()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = input.Close()
		_ = output.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = input.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	c := &observerChild{cmd: cmd, input: input, output: bufio.NewScanner(output), ctx: ctx}
	c.expect(t, "ready")
	return c
}

func (c *observerChild) expect(t *testing.T, text string) {
	t.Helper()
	if !c.output.Scan() || c.output.Text() != text {
		t.Fatalf("payload handshake want %q: %q (%v)", text, c.output.Text(), c.output.Err())
	}
}

func (c *observerChild) command(t *testing.T, command, response string) {
	t.Helper()
	if _, err := io.WriteString(c.input, command); err != nil {
		t.Fatal(err)
	}
	c.expect(t, response)
}

func (c *observerChild) stop(t *testing.T, kill bool) *syscall.Rusage {
	t.Helper()
	if kill {
		if err := c.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
	} else if _, err := io.WriteString(c.input, "q"); err != nil {
		t.Fatal(err)
	}
	err := c.cmd.Wait()
	if c.ctx.Err() != nil || (kill && (err == nil || c.cmd.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL)) || (!kill && err != nil) {
		t.Fatalf("payload exit kill=%v: %v, context=%v", kill, err, c.ctx.Err())
	}
	return c.cmd.ProcessState.SysUsage().(*syscall.Rusage)
}

func saveObserverEvidence(t *testing.T, name string, value any) {
	t.Helper()
	if dir := os.Getenv("DF0_OBSERVER_EVIDENCE"); dir != "" && !t.Failed() {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMacOSCurrentUsageCohorts(t *testing.T) {
	probe := compileObserverProbe(t)
	type cohort struct {
		Workers int           `json:"workers"`
		Before  []usageSample `json:"before"`
		Held    []usageSample `json:"held"`
	}
	var all []cohort
	for _, count := range []int{2, 4, 8} {
		var children []*observerChild
		var pids []int
		for range count {
			c := startObserverChild(t, probe)
			children = append(children, c)
			pids = append(pids, c.cmd.Process.Pid)
		}
		before, err := observeUsage(probe, pids, nil)
		if err != nil {
			t.Fatal(err)
		}
		starts := map[int]uint64{}
		for i, c := range children {
			starts[pids[i]] = before[i].Start
			c.command(t, "a", "held")
		}
		held, err := observeUsage(probe, pids, starts)
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range children {
			if held[i].RSS < before[i].RSS+28<<20 || held[i].User+held[i].System < before[i].User+before[i].System {
				t.Fatalf("current usage did not observe held resident growth: before=%+v held=%+v", before[i], held[i])
			}
			if usage := c.stop(t, false); usage.Maxrss < 32<<20 {
				t.Fatalf("independent peak control did not see allocation: %+v", usage)
			}
		}
		all = append(all, cohort{count, before, held})
	}
	saveObserverEvidence(t, "cohorts.json", all)
}

func TestMacOSObservedTermination(t *testing.T) {
	probe := compileObserverProbe(t)
	type reaction struct {
		Mode       string       `json:"mode"`
		Before     usageSample  `json:"before"`
		Observed   *usageSample `json:"observed,omitempty"`
		DelayNS    int64        `json:"ready_to_observed_ns"`
		StopNS     int64        `json:"observation_to_reaped_ns"`
		PeakRSS    int64        `json:"wait4_peak_rss_bytes"`
		CPUTimeNS  int64        `json:"wait4_cpu_ns"`
		CPUCores   float64      `json:"interval_cpu_cores"`
		ReadFailed bool         `json:"identity_mismatch_refused"`
	}
	var all []reaction
	lastPID := 0
	for _, mode := range []string{"rss", "cpu", "identity_failure"} {
		c := startObserverChild(t, probe)
		pids := []int{c.cmd.Process.Pid}
		lastPID = pids[0]
		b, err := observeUsage(probe, pids, nil)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "cpu" {
			c.command(t, "c", "cpu")
		} else {
			c.command(t, "a", "held")
		}
		ready := time.Now()
		// Deliberately postpone the next observation after the fixture's
		// handshake. This measures a delay; it does not promise a scheduler bound.
		time.Sleep(25 * time.Millisecond)
		starts := map[int]uint64{pids[0]: b[0].Start}
		if mode == "identity_failure" {
			starts[pids[0]]++
		}
		s, err := observeUsage(probe, pids, starts)
		delay := time.Since(ready).Nanoseconds()
		r := reaction{Mode: mode, Before: b[0], DelayNS: delay}
		if mode == "identity_failure" {
			if err == nil || err.Error() != "process identity changed" || s != nil {
				t.Fatal("identity mismatch accepted as a usable sample")
			}
			r.ReadFailed = true
		} else {
			if err != nil {
				t.Fatal(err)
			}
			r.Observed = &s[0]
			r.CPUCores = float64(s[0].User+s[0].System-b[0].User-b[0].System) / float64(s[0].Before-b[0].Before)
			if (mode == "rss" && s[0].RSS <= b[0].RSS+16<<20) || (mode == "cpu" && s[0].User+s[0].System <= b[0].User+b[0].System+10_000_000) {
				t.Fatalf("fixture did not exceed observation trigger: %+v", r)
			}
		}
		stop := time.Now()
		usage := c.stop(t, true)
		r.StopNS, r.PeakRSS = time.Since(stop).Nanoseconds(), usage.Maxrss
		r.CPUTimeNS = usage.Utime.Nano() + usage.Stime.Nano()
		if (mode != "cpu" && r.PeakRSS < 32<<20) || (mode == "cpu" && r.CPUTimeNS < 40_000_000) {
			t.Fatalf("independent rusage did not confirm effect: %+v", r)
		}
		if mode == "cpu" && (s[0].User+s[0].System-b[0].User-b[0].System < 40_000_000 || s[0].User+s[0].System > uint64(r.CPUTimeNS)+1_000_000) {
			t.Fatalf("sample CPU units not calibrated to independent rusage: %+v", r)
		}
		if s, err := observeUsage(probe, pids, nil); err == nil || s != nil {
			t.Fatal("reaped process returned a usable observation")
		}
		all = append(all, r)
	}
	recovery := startObserverChild(t, probe)
	// The helper emits a valid first record before failing on the reaped PID.
	// Never accept that partial cohort as a usable resource observation.
	partial := exec.CommandContext(recovery.ctx, probe, "sample", strconv.Itoa(recovery.cmd.Process.Pid), strconv.Itoa(lastPID))
	partial.Env = []string{}
	b, partialErr := partial.Output()
	var first usageSample
	if partialErr == nil || json.Unmarshal(bytes.TrimSpace(b), &first) != nil || first.PID != recovery.cmd.Process.Pid || first.Start == 0 || first.RSS == 0 {
		t.Fatalf("partial-cohort precondition not demonstrated: %v: %s", partialErr, b)
	}
	if s, err := observeUsage(probe, []int{recovery.cmd.Process.Pid, lastPID}, nil); err == nil || s != nil {
		t.Fatal("partial cohort returned a usable observation")
	}
	recovery.command(t, "a", "held")
	if recovery.stop(t, false).Maxrss < 32<<20 {
		t.Fatal("post-termination recovery allocation missing")
	}
	saveObserverEvidence(t, "termination.json", all)
}

func TestMacOSObserverCanMissResidentBurst(t *testing.T) {
	probe := compileObserverProbe(t)
	c := startObserverChild(t, probe)
	pids := []int{c.cmd.Process.Pid}
	before, err := observeUsage(probe, pids, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.command(t, "b", "burst")
	after, err := observeUsage(probe, pids, map[int]uint64{pids[0]: before[0].Start})
	if err != nil {
		t.Fatal(err)
	}
	peak := c.stop(t, false).Maxrss
	trigger := before[0].RSS + 16<<20
	if before[0].RSS >= trigger || after[0].RSS >= trigger || uint64(peak) <= trigger {
		t.Fatalf("missing inter-sample counterexample: before=%+v after=%+v peak=%d", before, after, peak)
	}
	saveObserverEvidence(t, "missed-burst.json", struct {
		Before  usageSample `json:"before"`
		After   usageSample `json:"after"`
		Trigger uint64      `json:"fixture_trigger_bytes"`
		Peak    int64       `json:"wait4_peak_rss_bytes"`
	}{before[0], after[0], trigger, peak})
}
