// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command g0probe gathers ADR-010 G0 evidence: real gRPC/HTTP2 between this
// Go host and the Java worker over local IPC, on whatever OS it runs on.
// It is a spike, not a GoPMgr component. It records PASS, FAIL or BLOCKED
// for each scenario and never falls back to another transport.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"
)

const (
	statusPass    = "PASS"
	statusFail    = "FAIL"
	statusBlocked = "BLOCKED"
)

// javaStack is pinned in worker/pom.xml.
const javaStack = "grpc-java 1.85.1, Netty 4.2.17.Final, protobuf-java 3.25.9"

type evidence struct {
	ID         string            `json:"id"`
	Scenario   string            `json:"scenario"`
	TestIDs    []string          `json:"test_ids"`
	Status     string            `json:"status"`
	Platform   string            `json:"platform"`
	OS         string            `json:"os"`
	Transport  string            `json:"transport"`
	SourceHead string            `json:"source_head"`
	Versions   map[string]string `json:"versions"`
	Metrics    map[string]any    `json:"metrics"`
	Notes      []string          `json:"notes,omitempty"`
	StartedAt  string            `json:"started_at"`
	DurationMS int64             `json:"duration_ms"`
	began      time.Time
}

var runInfo struct {
	head     string
	os       string
	versions map[string]string
}

func newEvidence(scenario, transport string, testIDs []string) *evidence {
	return &evidence{
		ID:         runtime.GOOS + "-" + runtime.GOARCH + "/" + transport + "/" + scenario,
		Scenario:   scenario,
		TestIDs:    testIDs,
		Status:     statusPass,
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
		OS:         runInfo.os,
		Transport:  transport,
		SourceHead: runInfo.head,
		Versions:   runInfo.versions,
		Metrics:    map[string]any{},
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		began:      time.Now(),
	}
}

func (e *evidence) fail(note string) {
	e.Status = statusFail
	e.Notes = append(e.Notes, note)
}

// block records a prerequisite that is missing, unless the scenario has
// already failed outright.
func (e *evidence) block(note string) {
	if e.Status != statusFail {
		e.Status = statusBlocked
	}
	e.Notes = append(e.Notes, note)
}

// absorb copies a job's measurements and turns launch or session errors
// into failures.
func (e *evidence) absorb(r *jobRun, err error) {
	if err != nil {
		e.fail(err.Error())
	}
	if r == nil {
		return
	}
	for k, v := range r.metrics {
		e.Metrics[k] = v
	}
	e.Metrics["worker_exit_code"] = r.exitCode
	if !r.connected {
		e.fail("the worker never completed the handshake")
	} else if r.driveErr != nil {
		e.fail("session: " + r.driveErr.Error())
	}
}

// summarize reports a latency distribution in the given unit.
func summarize(ds []time.Duration, unit time.Duration) map[string]any {
	if len(ds) == 0 {
		return map[string]any{"n": 0}
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	pick := func(q float64) float64 {
		i := int(math.Ceil(q*float64(len(s)))) - 1
		return round2(float64(s[max(i, 0)]) / float64(unit))
	}
	return map[string]any{"n": len(s), "p50": pick(0.50), "p95": pick(0.95), "p99": pick(0.99), "max": pick(1)}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func missingEndpoint(transport string) string {
	id, _ := randomID()
	if strings.HasPrefix(transport, "npipe") {
		return `\\.\pipe\gopmgr-g0-missing-` + id
	}
	return filepath.Join(os.TempDir(), "gopmgr-g0-missing-"+id, "w.sock")
}

func missingJava() string {
	return filepath.Join(os.TempDir(), "gopmgr-g0-no-such-dir", "java")
}

func moduleVersions() map[string]string {
	v := map[string]string{"go": runtime.Version(), "java_stack": javaStack}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			switch d.Path {
			case "google.golang.org/grpc", "google.golang.org/protobuf", "golang.org/x/sys", "github.com/Microsoft/go-winio":
				v[d.Path] = d.Version
			}
		}
	}
	return v
}

func main() {
	java := flag.String("java", "", "path to the java executable")
	jar := flag.String("jar", "", "path to worker/target/g0-worker.jar")
	out := flag.String("out", "g0-evidence.jsonl", "evidence file to append to")
	head := flag.String("head", "unknown", "source commit the run was built from")
	only := flag.String("transports", "", "comma-separated transports (default: all for this OS)")
	cancelRuns := flag.Int("cancel-runs", 30, "cancel scenario repetitions")
	flag.Parse()
	if *java == "" || *jar == "" {
		fmt.Fprintln(os.Stderr, "g0probe: -java and -jar are required")
		os.Exit(2)
	}
	runInfo.head = *head
	runInfo.os = osVersion()
	runInfo.versions = moduleVersions()
	cfg := config{java: *java, jar: *jar, sourceHead: *head}

	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "g0probe:", err)
		os.Exit(1)
	}
	defer f.Close()
	var all []*evidence
	emit := func(e *evidence) {
		e.DurationMS = time.Since(e.began).Milliseconds()
		line, _ := json.Marshal(e)
		_, _ = f.Write(append(line, '\n'))
		all = append(all, e)
		fmt.Printf("%-8s %-44s %s\n", e.Status, e.ID, strings.Join(e.Notes, "; "))
	}

	transports := transportsForOS()
	if *only != "" {
		transports = strings.Split(*only, ",")
	}
	for i, t := range transports {
		hs := scenarioHandshake(cfg, t)
		emit(hs)
		rest := []func() *evidence{
			func() *evidence { return scenarioDuplex(cfg, t) },
			func() *evidence { return scenarioBackpressure(cfg, t) },
			func() *evidence { return scenarioNoise(cfg, t) },
			func() *evidence { return scenarioBadCredential(cfg, t) },
			func() *evidence { return scenarioReplay(cfg, t) },
			func() *evidence { return scenarioImpersonate(cfg, t) },
			func() *evidence { return scenarioKill(cfg, t) },
			func() *evidence { return scenarioCancel(cfg, t, *cancelRuns) },
		}
		names := []string{"duplex", "backpressure", "noise", "bad-credential", "replay", "impersonate", "kill", "cancel"}
		for j, run := range rest {
			if hs.Status != statusPass {
				e := newEvidence(names[j], t, transportTestIDs(t))
				e.block("not run: the handshake did not pass on this transport")
				emit(e)
				continue
			}
			emit(run())
		}
		if i == 0 {
			for _, e := range scenarioBootstrap(cfg, t) {
				emit(e)
			}
			emit(scenarioHandshakeTimeout(cfg, t))
			emit(scenarioMissingJVM(cfg, t))
			emit(scenarioSquat(t))
			emit(scenarioSelfCheck(t))
		}
	}

	counts := map[string]int{}
	for _, e := range all {
		counts[e.Status]++
	}
	fmt.Printf("\ng0probe: %d PASS, %d FAIL, %d BLOCKED on %s (%s); evidence in %s\n",
		counts[statusPass], counts[statusFail], counts[statusBlocked], runtime.GOOS+"/"+runtime.GOARCH, runInfo.os, *out)
}
