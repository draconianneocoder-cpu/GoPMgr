// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// hostInfo records the machine a measurement came from. A run on a cloud
// container or CI runner is not an approved hardware profile (OWN-10).
func hostInfo() map[string]any {
	h := map[string]any{
		"goos":   runtime.GOOS,
		"goarch": runtime.GOARCH,
		"cpus":   runtime.NumCPU(),
		"go":     runtime.Version(),
	}
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "model name") {
					h["cpu"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					break
				}
			}
		}
		h["mem_bytes"] = meminfoKB("MemTotal") * 1024
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			h["cpu"] = strings.TrimSpace(string(out))
		}
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			v, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			h["mem_bytes"] = v
		}
	}
	return h
}

func meminfoKB(key string) int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+":") {
			f := strings.Fields(line)
			v, _ := strconv.ParseInt(f[1], 10, 64)
			return v
		}
	}
	return -1
}

// peakRSS is the process's resident high-water mark where the OS reports
// one; elsewhere it is the Go runtime's total reserved memory.
func peakRSS() int64 {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/self/status")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "VmHWM:") {
					v, _ := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
					return v * 1024
				}
			}
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}
