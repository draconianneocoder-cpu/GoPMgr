// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These are synthetic OS capability probes, not DataFusion quota qualification.
type limitObservation struct {
	Mode            string `json:"mode"`
	VMBefore        uint64 `json:"vm_before_bytes,omitempty"`
	ASLimitBytes    uint64 `json:"address_space_limit_bytes,omitempty"`
	CPUSoftSeconds  uint64 `json:"cpu_soft_seconds,omitempty"`
	CPUHardSeconds  uint64 `json:"cpu_hard_seconds,omitempty"`
	Workload        string `json:"workload,omitempty"`
	Checksum        uint64 `json:"checksum,omitempty"`
	PeakBefore      int64  `json:"peak_before_bytes,omitempty"`
	PeakAfter       int64  `json:"peak_after_bytes,omitempty"`
	AllocationError string `json:"allocation_error,omitempty"`
	CPUTimeNS       int64  `json:"cpu_time_ns,omitempty"`
	ChildCPUTimeNS  int64  `json:"child_cpu_time_ns,omitempty"`
	Signal          string `json:"signal,omitempty"`
	InitialGroup    int    `json:"initial_group,omitempty"`
	EscapedGroup    int    `json:"escaped_group,omitempty"`
}

func limitCPU(t *testing.T) int64 {
	t.Helper()
	var usage unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
		t.Fatal(err)
	}
	return usage.Utime.Nano() + usage.Stime.Nano()
}

func limitPeak(t *testing.T) int64 {
	t.Helper()
	var usage unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
		t.Fatal(err)
	}
	return usage.Maxrss // Darwin bytes, independent of the host's conversion.
}

func rejectCPUSignal(t *testing.T, exceeded <-chan os.Signal) {
	t.Helper()
	select {
	case sig := <-exceeded:
		t.Fatalf("unexpected CPU-limit signal in below-limit process: %v", sig)
	default:
	}
}

func limitChildCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestMacOSLimitChild$")
	cmd.Env = []string{"DF0_LIMIT_CHILD=" + mode}
	cmd.WaitDelay = time.Second
	if mode != "cpu-part" && mode != "escape" {
		// Keep the nested CPU fixture in this owned group so a timeout also
		// stops it. The escape experiment owns and waits both children directly.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	}
	return cmd
}

func runLimitChild(t *testing.T, mode string) limitObservation {
	t.Helper()
	cmd := limitChildCommand(t, mode)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if mode == "cpu-limit" {
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGXCPU {
			t.Fatalf("CPU limit must signal SIGXCPU, not a timeout or other failure: %v: %s", err, output)
		}
	} else if err != nil {
		t.Fatalf("%s: %v: %s", mode, err, output)
	}
	var observed limitObservation
	if err := json.Unmarshal(output, &observed); err != nil || observed.Mode != mode {
		t.Fatalf("%s observation: %v: %s", mode, err, output)
	}
	if exit != nil {
		observed.Signal = exit.Sys().(syscall.WaitStatus).Signal().String()
	}
	return observed
}

func TestMacOSLimitPrimitives(t *testing.T) {
	var observations []limitObservation
	for _, mode := range []string{"allocation-control", "as-growth", "as-resident", "cpu-limit", "cpu-tree"} {
		t.Run(mode, func(t *testing.T) {
			r := runLimitChild(t, mode)
			switch mode {
			case "allocation-control", "as-resident":
				if r.PeakAfter-r.PeakBefore < 64<<20 {
					t.Fatalf("128 MiB touch did not grow resident memory enough to establish counterexample: %+v", r)
				}
			case "as-growth":
				if r.AllocationError != unix.ENOMEM.Error() {
					t.Fatalf("new mapping was not denied with ENOMEM: %+v", r)
				}
			case "cpu-tree":
				if r.CPUTimeNS < 600_000_000 || r.ChildCPUTimeNS < 600_000_000 || r.CPUTimeNS >= 1_000_000_000 || r.ChildCPUTimeNS >= 1_000_000_000 || r.CPUTimeNS+r.ChildCPUTimeNS <= 1_000_000_000 {
					t.Fatalf("did not exceed one second across independently limited processes: %+v", r)
				}
			}
			observations = append(observations, r)
			t.Logf("%+v", r)
		})
	}
	if path := os.Getenv("DF0_LIMIT_EVIDENCE"); path != "" && !t.Failed() {
		data, err := json.MarshalIndent(observations, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMacOSLimitChild(t *testing.T) {
	mode := os.Getenv("DF0_LIMIT_CHILD")
	if mode == "" {
		t.Skip("subprocess fixture")
	}
	// Never allow a probe to create a core dump.
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{}); err != nil {
		t.Fatal(err)
	}
	if mode == "escape" {
		runEscapeChild(t)
		os.Exit(0)
	}
	r := limitObservation{Mode: mode}
	if strings.HasPrefix(mode, "cpu-") {
		exceeded := make(chan os.Signal, 1)
		if mode != "cpu-limit" {
			// Go otherwise ignores SIGXCPU. Detect it rather than allowing a
			// false aggregate-budget counterexample after a delivered signal.
			signal.Notify(exceeded, syscall.SIGXCPU)
			defer signal.Stop(exceeded)
		}
		if mode == "cpu-part" {
			var inherited unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_CPU, &inherited); err != nil || inherited.Cur != 1 || inherited.Max != 2 {
				t.Fatalf("CPU limit was not inherited: %+v: %v", inherited, err)
			}
		} else if err := unix.Setrlimit(unix.RLIMIT_CPU, &unix.Rlimit{Cur: 1, Max: 2}); err != nil {
			t.Fatal(err)
		}
		r.CPUSoftSeconds, r.CPUHardSeconds = 1, 2
		if mode == "cpu-limit" {
			// Go catches SIGXCPU without terminating by default. Exec a shell
			// fixture with default signal dispositions; do not mistake a host
			// timeout for OS enforcement, or assume a worker handles it the same.
			r.Workload = "POSIX shell builtin loop, default signal dispositions"
			if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Exec("/bin/sh", []string{"sh", "-c", "while :; do :; done"}, []string{}); err != nil {
				t.Fatal(err)
			}
		}
		for limitCPU(t) < 600_000_000 {
			rejectCPUSignal(t, exceeded)
			for i := 0; i < 1<<20; i++ {
				r.Checksum = r.Checksum*6364136223846793005 + 1
			}
		}
		if mode == "cpu-tree" {
			r.ChildCPUTimeNS = runLimitChild(t, "cpu-part").CPUTimeNS
		}
		r.CPUTimeNS = limitCPU(t)
		rejectCPUSignal(t, exceeded)
	} else {
		const size = 128 << 20
		var data []byte
		var err error
		if mode != "as-growth" {
			data, err = unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
			if err != nil {
				t.Fatal(err)
			}
		}
		r.PeakBefore = limitPeak(t)
		if mode != "allocation-control" {
			output, err := exec.Command("/bin/ps", "-p", strconv.Itoa(os.Getpid()), "-o", "vsz=").Output()
			if err != nil {
				t.Fatal(err)
			}
			kb, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
			if err != nil || kb == 0 || kb > (^uint64(0)-(8<<20))/1024 {
				t.Fatalf("invalid ps virtual size: %q: %v", output, err)
			}
			r.VMBefore, r.ASLimitBytes = kb*1024, kb*1024+(8<<20)
			if err := unix.Setrlimit(unix.RLIMIT_AS, &unix.Rlimit{Cur: r.ASLimitBytes, Max: r.ASLimitBytes}); err != nil {
				t.Fatal(err)
			}
			var actual unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_AS, &actual); err != nil || actual.Cur != r.ASLimitBytes || actual.Max != r.ASLimitBytes {
				t.Fatalf("address-space limit not installed: %+v: %v", actual, err)
			}
			if mode == "as-resident" {
				// Independently verify enforcement in this same process before
				// touching its pre-existing mapping.
				probe, err := unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
				if err == nil {
					_ = unix.Munmap(probe)
					t.Fatal("new mapping accepted under resident-growth control")
				}
				if !errors.Is(err, unix.ENOMEM) {
					t.Fatal(err)
				}
			}
		}
		if mode == "as-growth" {
			data, err = unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
			if err != nil {
				r.AllocationError = err.Error()
			} else {
				if err := unix.Munmap(data); err != nil {
					t.Fatal(err)
				}
			}
		} else {
			for i := 0; i < len(data); i += os.Getpagesize() {
				data[i] = byte(i/os.Getpagesize()%251 + 1)
			}
		}
		r.PeakAfter = limitPeak(t)
		if mode != "as-growth" {
			if err := unix.Munmap(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestMacOSProcessGroupMemberCanEscape(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	anchor := exec.CommandContext(ctx, "/bin/sleep", "20")
	anchor.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := anchor.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = anchor.Process.Kill(); _ = anchor.Wait() }()
	cmd := limitChildCommand(t, "escape")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: anchor.Process.Pid}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	decoder := json.NewDecoder(stdout)
	var r limitObservation
	if err := decoder.Decode(&r); err != nil || r.InitialGroup != anchor.Process.Pid || r.EscapedGroup != cmd.Process.Pid {
		t.Fatalf("failed escape: %+v: %v", r, err)
	}
	if err := syscall.Kill(-anchor.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	var exit *exec.ExitError
	if err := anchor.Wait(); !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Fatalf("same-group control not terminated: %v", err)
	}
	if _, err := stdin.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	var alive bool
	if err := decoder.Decode(&alive); err != nil || !alive {
		t.Fatalf("escaped member did not answer after group termination: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	t.Logf("same-group control terminated; escaped member responded and reaped: %+v", r)
	if path := os.Getenv("DF0_GROUP_EVIDENCE"); path != "" {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func runEscapeChild(t *testing.T) {
	t.Helper()
	initial, err := unix.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	group, err := unix.Setsid()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(limitObservation{Mode: "escape", InitialGroup: initial, EscapedGroup: group}); err != nil {
		t.Fatal(err)
	}
	var input [1]byte
	if n, err := os.Stdin.Read(input[:]); n != 1 || err != nil {
		t.Fatalf("escape handshake: %d: %v", n, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(true); err != nil {
		t.Fatal(err)
	}
}
