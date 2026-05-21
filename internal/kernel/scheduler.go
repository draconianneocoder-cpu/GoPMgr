// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package kernel implements PMForge's scheduling math: Critical Path
// Method (CPM) and the data shapes used by Earned Value Management
// (EVM) and the MSPDI exporter.
//
// The kernel is intentionally pure: no I/O, no globals, no database
// access. All inputs come in and all results come out via plain maps
// and structs so the algorithm can be unit-tested in isolation.
package kernel

import "sort"

// Task is the scheduling-side view of an activity. The persistence
// layer (db.tasks) is a near-mirror, but kernel.Task carries the
// computed CPM fields (ES/EF/LS/LF/Float and IsCritical) that the GUI
// renders directly.
type Task struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Duration   float64  `json:"duration"`
	Precedents []string `json:"precedents"`

	// CPM outputs — populated by CalculateCPM.
	ES         float64 `json:"es"`
	EF         float64 `json:"ef"`
	LS         float64 `json:"ls"`
	LF         float64 `json:"lf"`
	Float      float64 `json:"float"`
	IsCritical bool    `json:"is_critical"`
}

// CalculateCPM runs the full Critical Path Method on the given task
// graph in place. The input map is keyed by Task.ID; every Precedents
// entry MUST refer to a key in the map.
//
// Algorithm:
//
//  1. Topologically sort the graph (predecessors before successors).
//  2. Forward pass: ES = max(EF of all precedents), EF = ES + Duration.
//  3. Backward pass: LF = min(LS of all successors), LS = LF - Duration.
//     For the terminal node(s), LF defaults to EF (project finish).
//  4. Float = LS - ES. Tasks with Float == 0 lie on the critical path.
//
// If the graph contains a cycle, CalculateCPM returns false; the task
// map is left in an undefined intermediate state.
func CalculateCPM(tasks map[string]*Task) bool {
	order, ok := topoSort(tasks)
	if !ok {
		return false
	}

	// --- Forward pass: ES / EF ---
	for _, id := range order {
		t := tasks[id]
		maxEF := 0.0
		for _, pID := range t.Precedents {
			p, exists := tasks[pID]
			if !exists {
				continue
			}
			if p.EF > maxEF {
				maxEF = p.EF
			}
		}
		t.ES = maxEF
		t.EF = t.ES + t.Duration
	}

	// Project finish = max EF across all tasks.
	projectEF := 0.0
	for _, t := range tasks {
		if t.EF > projectEF {
			projectEF = t.EF
		}
	}

	// Reverse adjacency: successors[id] = list of task IDs that name id
	// as a precedent. Used in the backward pass.
	successors := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		for _, pID := range t.Precedents {
			successors[pID] = append(successors[pID], t.ID)
		}
	}

	// --- Backward pass: LF / LS ---
	// Walk the topo order in reverse so every task is visited AFTER its
	// successors.
	for i := len(order) - 1; i >= 0; i-- {
		t := tasks[order[i]]
		succs := successors[t.ID]
		if len(succs) == 0 {
			// Terminal node — its late finish is the project finish.
			t.LF = projectEF
		} else {
			minLS := -1.0
			for _, sID := range succs {
				s := tasks[sID]
				if minLS < 0 || s.LS < minLS {
					minLS = s.LS
				}
			}
			t.LF = minLS
		}
		t.LS = t.LF - t.Duration
		t.Float = t.LS - t.ES
		// Use a tiny epsilon to absorb floating-point noise from float
		// arithmetic. Strictly, Float == 0 is what defines critical.
		t.IsCritical = t.Float < 1e-9 && t.Float > -1e-9
	}

	return true
}

// topoSort returns task IDs in dependency order (predecessors first).
// Returns (nil, false) if the graph contains a cycle.
//
// Implementation: Kahn's algorithm with deterministic ordering. The
// stable sort on `ready` keeps output reproducible across runs, which
// matters for snapshot-style tests.
func topoSort(tasks map[string]*Task) ([]string, bool) {
	indegree := make(map[string]int, len(tasks))
	for id := range tasks {
		indegree[id] = 0
	}
	for _, t := range tasks {
		for _, pID := range t.Precedents {
			if _, ok := tasks[pID]; ok {
				indegree[t.ID]++
			}
		}
	}

	// Reverse adjacency for the queue step.
	out := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		for _, pID := range t.Precedents {
			if _, ok := tasks[pID]; ok {
				out[pID] = append(out[pID], t.ID)
			}
		}
	}

	var ready []string
	for id, n := range indegree {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(tasks))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)

		successors := out[id]
		sort.Strings(successors)
		for _, sID := range successors {
			indegree[sID]--
			if indegree[sID] == 0 {
				ready = append(ready, sID)
				sort.Strings(ready)
			}
		}
	}

	if len(order) != len(tasks) {
		return nil, false // cycle
	}
	return order, true
}
