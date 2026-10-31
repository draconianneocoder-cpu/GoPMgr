// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"errors"
	"sync"
)

// Budget is configured capacity or a snapshot of reserved capacity, not usage
// sampled from the OS. These fixture settings do not define product budgets.
type Budget struct {
	Jobs        uint64 `json:"jobs"`
	MemoryBytes uint64 `json:"memory_bytes"`
	CPUSlots    uint64 `json:"cpu_slots"`
}

// Claim reserves declared capacity for one worker and its host-side job.
type Claim struct {
	MemoryBytes uint64 `json:"memory_bytes"`
	CPUSlots    uint64 `json:"cpu_slots"`
}

// Admission coordinates jobs routed through this shared controller. It must
// not be copied. Raw Run remains available for the spike's baseline experiments.
// Reservations do not enforce actual process RSS, CPU rate or child counts.
type Admission struct {
	mu     sync.Mutex
	limits Budget
	used   Budget
}

// NewAdmission creates a shared controller with positive, caller-chosen limits.
func NewAdmission(limits Budget) (*Admission, error) {
	if limits.Jobs == 0 || limits.MemoryBytes == 0 || limits.CPUSlots == 0 {
		return nil, errors.New("admission capacities must be positive")
	}
	return &Admission{limits: limits}, nil
}

// InUse returns an atomic snapshot of reservations, not measured RSS/CPU.
func (a *Admission) InUse() Budget {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.used
}

func (a *Admission) reserve(c Claim) (code, detail string) {
	if c.MemoryBytes == 0 || c.CPUSlots == 0 {
		return "INPUT_INVALID", "admission claim must be positive"
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.used.Jobs >= a.limits.Jobs || c.MemoryBytes > a.limits.MemoryBytes-a.used.MemoryBytes || c.CPUSlots > a.limits.CPUSlots-a.used.CPUSlots {
		return "RESOURCE_LIMIT", "host reservation capacity exceeded"
	}
	a.used.Jobs++
	a.used.MemoryBytes += c.MemoryBytes
	a.used.CPUSlots += c.CPUSlots
	return "", ""
}

// Run refuses over-capacity work before source grants, endpoints or processes
// are created. The reservation is released only after Run's cleanup completes.
func (a *Admission) Run(job Job, c Claim) Result {
	if code, detail := a.reserve(c); code != "" {
		r := NewResult(job.Name)
		r.Outcome = "admission_rejected"
		r.FailedCode = code
		r.Detail = detail
		return r
	}
	defer func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.used.Jobs--
		a.used.MemoryBytes -= c.MemoryBytes
		a.used.CPUSlots -= c.CPUSlots
	}()
	return Run(job)
}
