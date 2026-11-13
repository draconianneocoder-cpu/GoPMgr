// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import "sort"

// cycles returns the strongly connected components of the link graph that
// are cycles: every component of two or more tasks, and a single task that
// links to itself. Each component's members are sorted, and the components
// are ordered by their first member.
//
// It is Tarjan's algorithm with an explicit stack, so a 50,000-task chain
// needs no deep recursion.
func (a *auditor) cycles() ([][]string, error) {
	index := make(map[string]int, len(a.ids))
	for i, id := range a.ids {
		index[id] = i
	}
	n := len(a.ids)
	succ := make([][]int, n)
	self := make([]bool, n)
	for _, e := range a.links {
		p, s := index[e.pred], index[e.succ]
		succ[p] = append(succ[p], s)
		if p == s {
			self[p] = true
		}
	}

	const unvisited = -1
	order := make([]int, n) // discovery index
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range order {
		order[i] = unvisited
	}
	var stack []int
	type frame struct{ node, next int }
	var out [][]string
	counter := 0

	for root := 0; root < n; root++ {
		if order[root] != unvisited {
			continue
		}
		if err := a.tick(); err != nil {
			return nil, err
		}
		call := []frame{{node: root}}
		order[root], low[root] = counter, counter
		counter++
		stack = append(stack, root)
		onStack[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			v := f.node
			if f.next < len(succ[v]) {
				w := succ[v][f.next]
				f.next++
				switch {
				case order[w] == unvisited:
					order[w], low[w] = counter, counter
					counter++
					stack = append(stack, w)
					onStack[w] = true
					call = append(call, frame{node: w})
				case onStack[w] && order[w] < low[v]:
					low[v] = order[w]
				}
				continue
			}
			// All of v's successors are done.
			call = call[:len(call)-1]
			if len(call) > 0 {
				parent := call[len(call)-1].node
				if low[v] < low[parent] {
					low[parent] = low[v]
				}
			}
			if low[v] != order[v] {
				continue
			}
			var members []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				members = append(members, a.ids[w])
				if w == v {
					break
				}
			}
			if len(members) > 1 || self[v] {
				sort.Strings(members)
				out = append(out, members)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out, nil
}
