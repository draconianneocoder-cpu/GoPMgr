// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command s0bench measures GoPMgr's current schedule paths at the ADR-010
// workload tiers (S0, P02). It calls the real packages read-only on
// synthetic, seeded schedules in a throwaway encrypted database. It never
// opens a user project and changes no GoPMgr code.
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	randv2 "math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"time"

	"gopmgr/internal/charts/dag"
	"gopmgr/internal/db"
	"gopmgr/internal/export"
	"gopmgr/internal/kernel"
)

// record is one measurement, written as a JSON line.
type record struct {
	ID         string         `json:"id"`
	Tier       string         `json:"tier"`
	Tasks      int            `json:"tasks"`
	Status     string         `json:"status"`
	Metrics    map[string]any `json:"metrics"`
	Notes      []string       `json:"notes,omitempty"`
	SourceHead string         `json:"source_head"`
	Host       map[string]any `json:"host"`
	StartedAt  string         `json:"started_at"`
}

var tiers = []struct {
	name  string
	tasks int
}{
	{"T0", 500},
	{"T1", 10_000},
	{"T2", 50_000},
	{"T3", 100_000},
}

var words = []string{"survey", "design", "procure", "install", "commission", "inspect", "review", "approve", "fabricate", "test", "pour", "frame", "wire", "paint", "hand over"}

// generate builds a seeded schedule: mostly finish-to-start links to
// recent tasks, some start-to-start and finish-to-finish links with lag,
// milestones, and one or two resource assignments per task.
func generate(n int, seed uint64) (map[string]*kernel.Task, dag.LayeredDocument) {
	r := randv2.New(randv2.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	tasks := make(map[string]*kernel.Task, n)
	doc := dag.LayeredDocument{Nodes: make([]dag.LayeredNode, 0, n)}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("T%06d", i)
		dur := float64(1 + r.IntN(20))
		milestone := r.IntN(50) == 0
		if milestone {
			dur = 0
		}
		label := fmt.Sprintf("Activity %d %s %s", i, words[r.IntN(len(words))], words[r.IntN(len(words))])
		var assign []kernel.Assignment
		for k := 0; k < 1+r.IntN(2); k++ {
			assign = append(assign, kernel.Assignment{Resource: fmt.Sprintf("R%03d", r.IntN(300)), Units: 1})
		}
		t := &kernel.Task{ID: id, Title: label, Duration: dur, Milestone: milestone, Assignments: assign}
		node := dag.LayeredNode{ID: id, Label: label, Duration: dur, Milestone: milestone, Assignments: assign, Owner: assign[0].Resource}
		if i > 0 {
			k := 1
			switch x := r.IntN(10); {
			case x >= 9:
				k = 3
			case x >= 6:
				k = 2
			}
			lo := max(0, i-60)
			seen := map[int]bool{}
			for len(seen) < min(k, i-lo) {
				p := lo + r.IntN(i-lo)
				if seen[p] {
					continue
				}
				seen[p] = true
				link := kernel.Link{Pred: fmt.Sprintf("T%06d", p), Type: kernel.FinishToStart}
				label := "FS"
				switch x := r.IntN(20); {
				case x == 0:
					link.Type, label = kernel.FinishToFinish, "FF"
				case x <= 2:
					link.Type, label = kernel.StartToStart, "SS"
				}
				if r.IntN(10) == 0 {
					link.Lag = float64(1 + r.IntN(3))
					label += fmt.Sprintf("+%d", int(link.Lag))
				}
				t.Links = append(t.Links, link)
				doc.Edges = append(doc.Edges, dag.LayeredEdge{From: link.Pred, To: id, Label: label})
			}
		}
		tasks[id] = t
		doc.Nodes = append(doc.Nodes, node)
	}
	return tasks, doc
}

// fillComputed copies CPM results into the document, as a saved chart
// carries them.
func fillComputed(tasks map[string]*kernel.Task, doc *dag.LayeredDocument) {
	for i := range doc.Nodes {
		t := tasks[doc.Nodes[i].ID]
		n := &doc.Nodes[i]
		n.ES, n.EF, n.LS, n.LF, n.Float, n.IsCritical = t.ES, t.EF, t.LS, t.LF, t.Float, t.IsCritical
	}
}

// layoutBudget bounds total Gantt layout time per tier. Layout is
// superlinear at large sizes (see the S0 baseline report).
const layoutBudget = 120 * time.Second

func timeIt(n int, fn func() error) ([]time.Duration, error) {
	var out []time.Duration
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if err := fn(); err != nil {
			return out, err
		}
		out = append(out, time.Since(t0))
	}
	return out, nil
}

// timeBudget is timeIt that stops early once the samples taken so far
// add up to more than budget. It always takes at least one sample.
func timeBudget(n int, budget time.Duration, fn func() error) ([]time.Duration, error) {
	var out []time.Duration
	var total time.Duration
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if err := fn(); err != nil {
			return out, err
		}
		d := time.Since(t0)
		out = append(out, d)
		total += d
		if total > budget {
			break
		}
	}
	return out, nil
}

func dist(ds []time.Duration) map[string]any {
	if len(ds) == 0 {
		return map[string]any{"n": 0}
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	pick := func(q float64) float64 {
		i := int(math.Ceil(q*float64(len(s)))) - 1
		return math.Round(float64(s[max(i, 0)])/float64(time.Millisecond)*100) / 100
	}
	return map[string]any{"n": len(s), "p50_ms": pick(0.5), "p95_ms": pick(0.95), "max_ms": pick(1)}
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func main() {
	out := flag.String("out", "s0-bench.jsonl", "evidence file to append to")
	head := flag.String("head", "unknown", "source commit the run was built from")
	maxTasks := flag.Int("max-tasks", 100_000, "skip tiers larger than this")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile of the run here")
	flag.Parse()
	if *cpuProfile != "" {
		pf, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "s0bench:", err)
			os.Exit(1)
		}
		defer pf.Close()
		if err := pprof.StartCPUProfile(pf); err != nil {
			fmt.Fprintln(os.Stderr, "s0bench:", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "s0bench:", err)
		os.Exit(1)
	}
	defer f.Close()
	host := hostInfo()
	emit := func(rec *record) {
		rec.SourceHead, rec.Host = *head, host
		line, _ := json.Marshal(rec)
		_, _ = f.Write(append(line, '\n'))
		fmt.Printf("%-6s %-26s %s\n", rec.Status, rec.ID, compact(rec.Metrics))
	}

	for _, tier := range tiers {
		if tier.tasks > *maxTasks {
			continue
		}
		tasks, doc := generate(tier.tasks, 20261024)
		reps := 30
		if tier.tasks >= 50_000 {
			reps = 10
		}
		newRec := func(name string) *record {
			return &record{ID: tier.name + "/" + name, Tier: tier.name, Tasks: tier.tasks, Status: "PASS",
				Metrics: map[string]any{}, StartedAt: time.Now().UTC().Format(time.RFC3339)}
		}

		// Scheduling: the Go kernel's full critical-path pass.
		rec := newRec("cpm")
		ds, err := timeIt(30, func() error {
			if !kernel.CalculateCPM(tasks) {
				return fmt.Errorf("dependency cycle")
			}
			return nil
		})
		rec.Metrics["calculate_cpm"] = dist(ds)
		rec.Metrics["links"] = len(doc.Edges)
		if err != nil {
			rec.Status, rec.Notes = "FAIL", []string{err.Error()}
		}
		emit(rec)
		fillComputed(tasks, &doc)

		// What the interface does to draw a schedule: lay out every row.
		rec = newRec("gantt-layout")
		var layout dag.GanttLayout
		ds, err = timeBudget(reps, layoutBudget, func() error { var err error; layout, err = dag.LayoutGantt(doc); return err })
		if len(ds) < reps {
			rec.Notes = append(rec.Notes, fmt.Sprintf("stopped after %d of %d samples: samples exceeded the %s budget", len(ds), reps, layoutBudget))
		}
		rec.Metrics["layout_gantt"] = dist(ds)
		// Overallocation detection rescans every task per breached
		// resource-day, so record how many breaches this plan produces.
		over := 0
		for _, row := range layout.Rows {
			if row.Overallocated {
				over++
			}
		}
		rec.Metrics["overallocated_rows"] = over
		rec.Metrics["horizon_days"] = layout.Horizon
		if err != nil {
			rec.Status, rec.Notes = "FAIL", []string{err.Error()}
		}
		emit(rec)

		// The whole schedule as one chart JSON document.
		rec = newRec("chart-json")
		var blob []byte
		ds, _ = timeIt(reps, func() error { blob, err = json.Marshal(doc); return err })
		rec.Metrics["marshal"] = dist(ds)
		rec.Metrics["chart_json_bytes"] = len(blob)
		emit(rec)

		// MSPDI round trip through the current importer.
		rec = newRec("mspdi")
		var xml []byte
		ds, err = timeIt(3, func() error { xml, err = export.ToMSPDI("S0", tasks); return err })
		rec.Metrics["export"] = dist(ds)
		rec.Metrics["mspdi_bytes"] = len(xml)
		rec.Metrics["over_64MiB_import_cap"] = len(xml) > 64<<20
		var imported export.ImportedProject
		ds, err = timeIt(3, func() error {
			imported, err = export.FromMSPDIWithOptions(xml, export.MSPDIImportOptions{IncludeDependencies: true, IncludeProgress: true, IncludeAssignments: true})
			return err
		})
		rec.Metrics["import"] = dist(ds)
		rec.Metrics["imported_tasks"] = len(imported.Tasks)
		if err != nil {
			rec.Status, rec.Notes = "FAIL", []string{err.Error()}
		}
		emit(rec)

		emit(benchStorage(newRec("storage"), doc, blob, reps))
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		rec = newRec("memory")
		rec.Metrics["go_sys_bytes"] = m.Sys
		rec.Metrics["process_peak_rss_bytes"] = peakRSS()
		emit(rec)
	}
}

// benchStorage stores the schedule as one chart in a throwaway encrypted
// project, then makes one-field edits the way the app does today: the
// whole chart JSON is rewritten and audited before and after.
func benchStorage(rec *record, doc dag.LayeredDocument, blob []byte, edits int) *record {
	dir, err := os.MkdirTemp("", "gopmgr-s0-")
	if err != nil {
		rec.Status, rec.Notes = "FAIL", []string{err.Error()}
		return rec
	}
	defer os.RemoveAll(dir)
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		rec.Status, rec.Notes = "FAIL", []string{err.Error()}
		return rec
	}
	path := filepath.Join(dir, "project.gopmgr")
	d, err := db.InitEncryptedDB(path, dek)
	if err != nil {
		rec.Status, rec.Notes = "FAIL", []string{"open: " + err.Error()}
		return rec
	}
	defer d.Close()
	p, err := d.UpsertProject(db.Project{Name: "S0 baseline"})
	if err != nil {
		rec.Status, rec.Notes = "FAIL", []string{"project: " + err.Error()}
		return rec
	}
	t0 := time.Now()
	c, err := d.SaveChart(db.Chart{ProjectID: p.ID, Kind: "cpm", Title: "S0", Data: string(blob)})
	if err != nil {
		rec.Status, rec.Notes = "FAIL", []string{"create chart: " + err.Error()}
		return rec
	}
	rec.Metrics["create_chart_ms"] = time.Since(t0).Milliseconds()

	ds, err := timeIt(edits, func() error {
		doc.Nodes[len(doc.Nodes)/2].Duration++
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		c.Data = string(data)
		c, err = d.SaveChart(c)
		return err
	})
	rec.Metrics["one_field_edit"] = dist(ds)
	if err != nil {
		rec.Status, rec.Notes = "FAIL", []string{"edit: " + err.Error()}
		return rec
	}
	var auditBytes, updates int64
	if err := d.Conn.QueryRow(`SELECT COALESCE(SUM(LENGTH(before_canonical_json)+LENGTH(after_canonical_json)),0), COUNT(*)
		FROM audit_events WHERE event_type='chart.update'`).Scan(&auditBytes, &updates); err == nil && updates > 0 {
		rec.Metrics["audit_bytes_per_edit"] = auditBytes / updates
	}
	ds, err = timeIt(min(edits, 10), func() error {
		got, err := d.GetChart(c.ID)
		if err != nil {
			return err
		}
		_, err = dag.ParseLayered(got.Data)
		return err
	})
	rec.Metrics["open_chart_and_parse"] = dist(ds)
	t0 = time.Now()
	v, err := d.VerifyAuditChain(p.ID)
	rec.Metrics["verify_audit_chain_ms"] = time.Since(t0).Milliseconds()
	if err != nil {
		rec.Notes = append(rec.Notes, "verify: "+err.Error())
	} else {
		rec.Metrics["audit_chain"] = fmt.Sprintf("%+v", v)
	}
	rec.Metrics["db_bytes"] = fileSize(path)
	rec.Metrics["wal_bytes"] = fileSize(path + "-wal")
	var sqliteVersion, cipherVersion string
	_ = d.Conn.QueryRow(`SELECT sqlite_version()`).Scan(&sqliteVersion)
	_ = d.Conn.QueryRow(`PRAGMA cipher_version`).Scan(&cipherVersion)
	rec.Metrics["sqlite_version"] = sqliteVersion
	rec.Metrics["cipher_version"] = cipherVersion
	return rec
}

func compact(m map[string]any) string {
	b, _ := json.Marshal(m)
	s := string(b)
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return strings.ReplaceAll(s, `"`, "")
}
