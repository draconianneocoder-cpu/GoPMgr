// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// frozenLegacyReferenceSHA256 is the SHA-256 of
// overallocation_legacy_ref_test.go with CRLF line endings normalized to LF.
// Changing that file changes what every faster detector is held to, so the
// change must show up in review as an edit to this constant as well.
const frozenLegacyReferenceSHA256 = "68807af1bde66fe8805827809be5fcab9e723f40825acb01967706776905dc88"

func TestLegacyReferenceIsFrozen(t *testing.T) {
	data, err := os.ReadFile("overallocation_legacy_ref_test.go")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	if got := hex.EncodeToString(sum[:]); got != frozenLegacyReferenceSHA256 {
		t.Errorf("overallocation_legacy_ref_test.go has SHA-256 %s, want %s.\n"+
			"The frozen reference must not follow production. If its behaviour is meant to change, record the decision "+
			"in docs/testing/resource-overallocation-contract.md and update this constant in the same change.",
			got, frozenLegacyReferenceSHA256)
	}
}

// TestLegacyReferenceIsSeparateFromProduction keeps the frozen reference
// independent: if it reached production code, a change there would move
// both sides of the differential test at once. Every call in the file must
// be to a function the file declares, a builtin, or package math or sort,
// and no identifier the file does not declare itself may name a function
// declared in a non-test kernel file.
func TestLegacyReferenceIsSeparateFromProduction(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	production := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				production[fn.Name.Name] = true
			}
		}
	}
	// Names a faster detector keeps; tasksOnDay may legitimately go.
	for _, name := range []string{"ResourceUsage", "DetectOverallocationsWithPlan", "effectiveUnits", "capacityFor"} {
		if !production[name] {
			t.Fatalf("production function %s not found; the guard is not reading internal/kernel", name)
		}
	}

	ref, err := parser.ParseFile(fset, "overallocation_legacy_ref_test.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{} // functions the file declares
	for _, decl := range ref.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			declared[fn.Name.Name] = true
		}
	}
	// Parameters, results and local variables the file declares, so a local
	// such as horizon cannot collide with a future production function.
	local := map[string]bool{}
	ast.Inspect(ref, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			for _, name := range n.Names {
				local[name.Name] = true
			}
		case *ast.ValueSpec:
			for _, name := range n.Names {
				local[name.Name] = true
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for _, lhs := range n.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						local[id.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok {
						local[id.Name] = true
					}
				}
			}
		}
		return true
	})
	allowedPackages := map[string]bool{"math": true, "sort": true}
	allowedBuiltins := map[string]bool{"append": true, "len": true, "make": true, "int": true}
	ast.Inspect(ref, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if production[n.Name] && !local[n.Name] {
				t.Errorf("%s: the frozen reference names production function %s; copy it instead",
					fset.Position(n.Pos()), n.Name)
			}
		case *ast.CallExpr:
			switch fun := n.Fun.(type) {
			case *ast.Ident:
				if !declared[fun.Name] && !allowedBuiltins[fun.Name] {
					t.Errorf("%s: the frozen reference calls %s, which it does not declare",
						fset.Position(fun.Pos()), fun.Name)
				}
			case *ast.SelectorExpr:
				if pkg, ok := fun.X.(*ast.Ident); !ok || !allowedPackages[pkg.Name] {
					t.Errorf("%s: the frozen reference calls a method or another package (%s); copy the code instead",
						fset.Position(fun.Pos()), fun.Sel.Name)
				}
			default:
				t.Errorf("%s: the frozen reference makes an indirect call", fset.Position(n.Pos()))
			}
		}
		return true
	})
}

// TestDetectorMatchesFrozenLegacyReference runs production and the frozen
// legacy detector on the same seeded random schedules and requires
// identical records, flags and usage profiles (oracle OR-02).
//
// Units and caps are multiples of 1/8, so every day's demand is exact in
// binary64 whatever order Go's map iteration adds it in, and exact
// equality is the right comparison. That also means these cases cannot
// tell how demand is summed; the hand oracle's OR-01-S5 covers that. The
// generator produces the awkward legacy inputs: duplicate, negative,
// unsorted and out-of-span split days, empty split lists, zero durations
// with split days, duplicate printed IDs, aliased map keys, blank
// resources, nonpositive units and zero overrides. It sets no assignment
// calendar: since 2026-11-23 (D-CAL) production reads one and the frozen
// copy does not; the hand oracle's F-09 and T3 rows and the eighth-day grid
// property hold that difference.
func TestDetectorMatchesFrozenLegacyReference(t *testing.T) {
	seeds := []uint64{20261027, 1, 2, 3, 0x5eed}
	const casesPerSeed = 400
	var cover differentialCoverage
	for _, seed := range seeds {
		rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
		for i := range casesPerSeed {
			input, plan := generateOverallocationCase(rng)
			prodTasks, legacyTasks := cloneTaskMap(input), cloneTaskMap(input)
			before := detectorInputSnapshot(input, plan)

			got := DetectOverallocationsWithPlan(prodTasks, plan)
			want := legacyDetectOverallocationsWithPlan(legacyTasks, plan)

			fail := func(what, detail string) {
				t.Fatalf("seed %d case %d: %s differs from the frozen legacy reference:%s\ninput:\n%s",
					seed, i, what, detail, describeOverallocationCase(input, plan))
			}
			if diff := overallocationDiff(got, want); diff != "" {
				fail("records", diff)
			}
			for _, key := range sortedTaskKeys(input) {
				if prodTasks[key].Overallocated != legacyTasks[key].Overallocated {
					fail("Task.Overallocated", fmt.Sprintf(" key %q: production %v, legacy %v",
						key, prodTasks[key].Overallocated, legacyTasks[key].Overallocated))
				}
			}
			if after := detectorInputSnapshot(prodTasks, plan); after != before {
				fail("the detector's inputs", fmt.Sprintf(" production changed more than Task.Overallocated:\nafter:\n%s", after))
			}
			if diff := usageDiff(ResourceUsage(prodTasks), legacyResourceUsage(legacyTasks)); diff != "" {
				fail("ResourceUsage", " "+diff)
			}
			cover.observe(input, plan, want)
		}
	}
	cover.require(t)
}

func pickFrom[T any](rng *rand.Rand, choices ...T) T {
	return choices[rng.IntN(len(choices))]
}

// generateOverallocationCase builds one small random schedule and plan.
// It draws from rng only while walking slices and counters, never while
// ranging over a map, so a seed always yields the same case.
func generateOverallocationCase(rng *rand.Rand) (map[string]*Task, ResourceCapacityPlan) {
	resources := []string{"", "r0", "r1", "r2", "r3"}
	n := 1 + rng.IntN(16)
	tasks := make(map[string]*Task, n+1)
	keys := make([]string, 0, n)
	for i := range n {
		// Whole days only: since the 2026-11-21 contract change production
		// counts a part day by the share used, where the frozen copy rounds
		// to whole days; on whole-day schedules the two contracts agree.
		// Part days are held by the hand oracle and the grid property test.
		duration := pickFrom(rng, 0.0, 0, 1, 1, 2, 3, 9)
		es := float64(rng.IntN(12))
		task := &Task{ID: fmt.Sprintf("T%d", rng.IntN(n+2)), Duration: duration, ES: es, EF: es + duration}
		switch rng.IntN(10) {
		case 0, 1, 2:
			for range 1 + rng.IntN(5) {
				task.WorkDays = append(task.WorkDays, rng.IntN(22)-2)
			}
		case 3:
			task.WorkDays = []int{} // empty, not nil: the contiguous span
		}
		for range rng.IntN(4) {
			a := Assignment{
				Resource: pickFrom(rng, resources...),
				Units:    pickFrom(rng, -1, 0, 0.125, 0.25, 0.5, 0.75, 1, 1.5, 2),
			}
			if rng.IntN(4) == 0 {
				a.MaxUnits = pickFrom(rng, 0.25, 0.5, 1.25)
			}
			task.Assignments = append(task.Assignments, a)
		}
		key := fmt.Sprintf("k%02d", i)
		tasks[key] = task
		keys = append(keys, key)
	}
	if rng.IntN(5) == 0 {
		tasks["alias"] = tasks[keys[rng.IntN(len(keys))]]
	}

	plan := ResourceCapacityPlan{
		DefaultCapacity: pickFrom(rng, 0, 1, 1, 1.5, 2, -1),
		Capacities:      map[string]float64{},
		Calendars:       map[string]ResourceCalendar{},
	}
	for _, r := range resources[1:] {
		if rng.IntN(2) == 0 {
			plan.Capacities[r] = pickFrom(rng, -1, 0, 0.5, 1, 1.75, 3)
		}
		if rng.IntN(5) < 2 {
			plan.Calendars[r] = generateCalendar(rng)
		}
	}
	// A calendar no resource or assignment names: never consulted.
	plan.Calendars["winter"] = generateCalendar(rng)
	return tasks, plan
}

func generateCalendar(rng *rand.Rand) ResourceCalendar {
	cal := ResourceCalendar{DefaultCapacity: pickFrom(rng, 0, 0, -1, 1.25, 2.5)}
	if rng.IntN(2) == 0 {
		cal.WeeklyCapacity = map[int]float64{}
		for phase := range 7 {
			if rng.IntN(3) == 0 {
				cal.WeeklyCapacity[phase] = pickFrom(rng, 0, 0.5, 1, -1)
			}
		}
	}
	if rng.IntN(2) == 0 {
		cal.Overrides = map[int]float64{}
		for range 1 + rng.IntN(4) {
			cal.Overrides[rng.IntN(24)-1] = pickFrom(rng, 0, 0.75, -1, 2)
		}
	}
	return cal
}

// cloneTaskMap deep-copies a task map, keeping aliased keys aliased.
func cloneTaskMap(in map[string]*Task) map[string]*Task {
	copies := make(map[*Task]*Task, len(in))
	out := make(map[string]*Task, len(in))
	for key, task := range in {
		c, ok := copies[task]
		if !ok {
			dup := *task
			dup.Assignments = slices.Clone(task.Assignments)
			dup.WorkDays = slices.Clone(task.WorkDays)
			c = &dup
			copies[task] = c
		}
		out[key] = c
	}
	return out
}

func sortedTaskKeys(tasks map[string]*Task) []string {
	keys := make([]string, 0, len(tasks))
	for k := range tasks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func describeOverallocationCase(tasks map[string]*Task, plan ResourceCapacityPlan) string {
	var b strings.Builder
	for _, key := range sortedTaskKeys(tasks) {
		task := tasks[key]
		fmt.Fprintf(&b, "  %s: ID=%q Duration=%v ES=%v EF=%v WorkDays=%v Assignments=%+v\n",
			key, task.ID, task.Duration, task.ES, task.EF, task.WorkDays, task.Assignments)
	}
	fmt.Fprintf(&b, "  plan: %+v", plan)
	return b.String()
}

// differentialCoverage counts how often the random cases reach the inputs
// the differential test exists for, so a generator change cannot quietly
// turn it into a test of easy schedules.
type differentialCoverage struct {
	cases, breaches, duplicateIDBreaches, aliasBreaches, splitGapBreaches,
	lateWeeklyBreaches, zeroCapacityBreaches, emptyWorkDaysBreaches int
}

func (c *differentialCoverage) observe(tasks map[string]*Task, plan ResourceCapacityPlan, records []Overallocation) {
	c.cases++
	for _, rec := range records {
		c.breaches++
		for i := 1; i < len(rec.TaskIDs); i++ {
			if rec.TaskIDs[i] == rec.TaskIDs[i-1] {
				c.duplicateIDBreaches++
				break
			}
		}
		if alias, ok := tasks["alias"]; ok && legacyOccupiesDay(alias, rec.Day) && assignedTo(alias, rec.Resource) {
			c.aliasBreaches++
		}
		if cal, ok := plan.Calendars[rec.Resource]; ok && cal.WeeklyCapacity != nil && rec.Day >= 7 {
			c.lateWeeklyBreaches++
		}
		if rec.Capacity == 0 {
			c.zeroCapacityBreaches++
		}
		for _, task := range tasks {
			if !assignedTo(task, rec.Resource) {
				continue
			}
			first, last, ok := legacyTaskSpan(task)
			if len(task.WorkDays) > 0 && ok && rec.Day >= first && rec.Day <= last && !legacyOccupiesDay(task, rec.Day) {
				c.splitGapBreaches++
				break
			}
		}
		for _, task := range tasks {
			if task.WorkDays != nil && len(task.WorkDays) == 0 && assignedTo(task, rec.Resource) && legacyOccupiesDay(task, rec.Day) {
				c.emptyWorkDaysBreaches++
				break
			}
		}
	}
}

func assignedTo(task *Task, resource string) bool {
	return slices.ContainsFunc(task.Assignments, func(a Assignment) bool { return a.Resource == resource })
}

func (c *differentialCoverage) require(t *testing.T) {
	t.Helper()
	t.Logf("differential coverage: %+v", *c)
	floors := []struct {
		name       string
		got, floor int
	}{
		// A quarter to a third of what TestDetectorMatchesFrozenLegacyReference's
		// seeds produced with Go 1.27.3, leaving room for a toolchain to
		// change the random stream. The generator without assignment
		// calendars (2026-11-23) measured breaches 9973, duplicate IDs 830,
		// aliases 465, split gaps 928, late weekly 1527, zero capacity 644,
		// empty WorkDays 1558: every floor still holds with room.
		{"breaches", c.breaches, 4000},
		{"breaches listing a printed ID twice", c.duplicateIDBreaches, 300},
		{"breaches involving an aliased key", c.aliasBreaches, 200},
		{"breaches beside a split task's idle day", c.splitGapBreaches, 300},
		{"breaches on day 7 or later under a weekly calendar", c.lateWeeklyBreaches, 400},
		{"breaches on a zero-capacity day", c.zeroCapacityBreaches, 200},
		{"breaches with an empty, non-nil WorkDays participant", c.emptyWorkDaysBreaches, 400},
	}
	for _, f := range floors {
		if f.got < f.floor {
			t.Errorf("generator produced %d %s, want at least %d", f.got, f.name, f.floor)
		}
	}
}

// Part-day demand (OR-01 contract change, 2026-11-21): on schedules whose
// starts and durations are multiples of an eighth of a day, each day's
// demand must equal an independent count of the eighth-day cells the work
// covers on that day, times the assignment's effective units. Every value
// is a multiple of 1/64, so the comparison is exact. The oracle mirrors
// production's profile rules: a resource gets a profile when a task with
// work assigns it, every profile is as long as the last day any task's
// work reaches (+1), and days before 0 are dropped.
func TestPartDayDemandMatchesAnEighthDayGrid(t *testing.T) {
	floorDiv := func(a, b int) int {
		q := a / b
		if a%b != 0 && (a < 0) != (b < 0) {
			q--
		}
		return q
	}
	effective := func(a Assignment) float64 {
		u := a.Units
		if u <= 0 {
			u = 1
		}
		if a.MaxUnits > 0 && u > a.MaxUnits {
			u = a.MaxUnits
		}
		return u
	}
	rng := rand.New(rand.NewPCG(20261121, 0x9e3779b97f4a7c15))
	for i := range 2000 {
		tasks := map[string]*Task{}
		for k := range 1 + rng.IntN(12) {
			startCell := rng.IntN(105) - 8 // -1 to 12 days, in eighths
			cells := rng.IntN(41)          // 0 to 5 days
			task := &Task{ID: fmt.Sprintf("T%d", k), Duration: float64(cells) / 8,
				ES: float64(startCell) / 8, EF: float64(startCell+cells) / 8}
			for range rng.IntN(4) {
				a := Assignment{Resource: pickFrom(rng, "", "r0", "r1"), Units: pickFrom(rng, -1, 0, 0.125, 0.5, 1, 1.5)}
				if rng.IntN(4) == 0 {
					a.MaxUnits = pickFrom(rng, 0.25, 1.25)
				}
				task.Assignments = append(task.Assignments, a)
			}
			tasks[task.ID] = task
		}

		horizon := 0
		for _, task := range tasks {
			if task.Duration > 0 {
				lastCell := int(task.EF*8) - 1
				horizon = max(horizon, floorDiv(lastCell, 8)+1)
			}
		}
		want := map[string][]float64{}
		for _, task := range tasks {
			if task.Duration <= 0 {
				continue
			}
			for _, a := range task.Assignments {
				if a.Resource == "" {
					continue
				}
				if want[a.Resource] == nil {
					want[a.Resource] = make([]float64, horizon)
				}
				for cell := int(task.ES * 8); cell < int(task.EF*8); cell++ {
					if day := floorDiv(cell, 8); day >= 0 && day < horizon {
						want[a.Resource][day] += effective(a) / 8
					}
				}
			}
		}
		if diff := usageDiff(ResourceUsage(tasks), want); diff != "" {
			var b strings.Builder
			for _, id := range sortedTaskKeys(tasks) {
				task := tasks[id]
				fmt.Fprintf(&b, "  %s ES %v EF %v %+v\n", id, task.ES, task.EF, task.Assignments)
			}
			t.Fatalf("case %d: %s\n%s", i, diff, b.String())
		}
	}
}

// Time intervals (D-TIME) and assignment calendars (D-CAL), 2026-11-23: on
// schedules whose starts and durations are multiples of an eighth of a
// day, a day is a breach exactly when some eighth-day cell of it has more
// units in use than the smallest capacity among the assignments in use in
// that cell; the participants are the tasks in use in such a cell, and the
// record's capacity is the smallest among the participants' assignments to
// the resource that day. The oracle counts cells and states capacities
// itself (it never asks the plan); units are multiples of 1/8 and
// capacities of 1/2, so every comparison is exact.
func TestDetectionMatchesAnEighthDayGrid(t *testing.T) {
	plan := ResourceCapacityPlan{
		DefaultCapacity: 1,
		Capacities:      map[string]float64{"r1": 1.5},
		Calendars: map[string]ResourceCalendar{
			"half": {DefaultCapacity: 0.5},
			"r1":   {Overrides: map[int]float64{2: 0}},
		},
	}
	// capacity is the oracle's own statement of the plan above.
	capacity := func(a Assignment, day int) float64 {
		switch {
		case a.CalendarID == "half":
			return 0.5
		case a.Resource == "r1" && day == 2:
			return 0
		case a.Resource == "r1":
			return 1.5
		}
		return 1
	}
	effective := func(a Assignment) float64 {
		u := a.Units
		if u <= 0 {
			u = 1
		}
		if a.MaxUnits > 0 && u > a.MaxUnits {
			u = a.MaxUnits
		}
		return u
	}
	floorDiv := func(a int) int { return int(math.Floor(float64(a) / 8)) }

	rng := rand.New(rand.NewPCG(20261123, 0x9e3779b97f4a7c15))
	breaches, partial := 0, 0
	for i := range 2000 {
		tasks := map[string]*Task{}
		for k := range 1 + rng.IntN(10) {
			startCell := rng.IntN(49) - 8 // -1 to 5 days, in eighths
			cells := rng.IntN(25)         // 0 to 3 days
			task := &Task{ID: fmt.Sprintf("T%d", k), Duration: float64(cells) / 8,
				ES: float64(startCell) / 8, EF: float64(startCell+cells) / 8}
			for range rng.IntN(3) {
				a := Assignment{Resource: pickFrom(rng, "", "r0", "r1"), Units: pickFrom(rng, 0, 0.125, 0.25, 0.5, 1, 1.5)}
				if rng.IntN(3) == 0 {
					a.CalendarID = pickFrom(rng, "half", "nope")
				}
				if rng.IntN(5) == 0 {
					a.MaxUnits = 0.25
				}
				task.Assignments = append(task.Assignments, a)
			}
			tasks[task.ID] = task
		}

		horizon := 0
		for _, task := range tasks {
			if task.Duration > 0 {
				horizon = max(horizon, floorDiv(int(task.EF*8)-1)+1)
			}
		}
		type cellUse struct {
			units  float64
			minCap float64
			tasks  []string
		}
		demand := map[string][]float64{}
		cellsOf := map[string]map[int]*cellUse{}
		for _, id := range sortedTaskKeys(tasks) {
			task := tasks[id]
			if task.Duration <= 0 {
				continue
			}
			for _, a := range task.Assignments {
				if a.Resource == "" {
					continue
				}
				if demand[a.Resource] == nil {
					demand[a.Resource] = make([]float64, horizon)
					cellsOf[a.Resource] = map[int]*cellUse{}
				}
				for cell := int(task.ES * 8); cell < int(task.EF*8); cell++ {
					day := floorDiv(cell)
					if day < 0 || day >= horizon {
						continue
					}
					demand[a.Resource][day] += effective(a) / 8
					c := cellsOf[a.Resource][cell]
					if c == nil {
						c = &cellUse{minCap: math.Inf(1)}
						cellsOf[a.Resource][cell] = c
					}
					c.units += effective(a)
					c.minCap = min(c.minCap, capacity(a, day))
					if !slices.Contains(c.tasks, id) {
						c.tasks = append(c.tasks, id)
					}
				}
			}
		}
		var want []Overallocation
		flags := map[string]bool{}
		resources := slices.Sorted(maps.Keys(demand))
		for _, r := range resources {
			for day := range horizon {
				var ids []string
				for cell := day * 8; cell < day*8+8; cell++ {
					if c := cellsOf[r][cell]; c != nil && c.units > c.minCap {
						ids = append(ids, c.tasks...)
					}
				}
				if ids == nil {
					continue
				}
				slices.Sort(ids)
				ids = slices.Compact(ids)
				capMin := math.Inf(1)
				for _, id := range ids {
					flags[id] = true
					for _, a := range tasks[id].Assignments {
						if a.Resource == r {
							capMin = min(capMin, capacity(a, day))
						}
					}
				}
				if demand[r][day] <= capMin {
					partial++
				}
				want = append(want, Overallocation{Resource: r, Day: day, Demand: demand[r][day], Capacity: capMin, TaskIDs: ids})
			}
		}
		breaches += len(want)

		got := DetectOverallocationsWithPlan(tasks, plan)
		if diff := overallocationDiff(got, want); diff != "" {
			var b strings.Builder
			for _, id := range sortedTaskKeys(tasks) {
				task := tasks[id]
				fmt.Fprintf(&b, "  %s ES %v EF %v %+v\n", id, task.ES, task.EF, task.Assignments)
			}
			t.Fatalf("case %d: records differ:%s\n%s", i, diff, b.String())
		}
		for id, task := range tasks {
			if task.Overallocated != flags[id] {
				t.Fatalf("case %d: %s.Overallocated = %v, want %v", i, id, task.Overallocated, flags[id])
			}
		}
	}
	// The cases must include breaches, and overlaps whose day total fits.
	t.Logf("grid detection: %d breaches, %d with the day's total within capacity", breaches, partial)
	if breaches < 1000 || partial < 100 {
		t.Errorf("generator produced %d breaches (%d with the total within capacity), want at least 1000 (100)", breaches, partial)
	}
}
