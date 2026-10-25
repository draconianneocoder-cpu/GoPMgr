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
	"net"
	"os"
	"os/exec"
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

// describeInterfaces lists the host's network interfaces and whether each
// is up, so a run inside an empty network namespace shows it.
func describeInterfaces() string {
	ifs, err := net.Interfaces()
	if err != nil {
		return "unknown: " + err.Error()
	}
	var parts []string
	for _, it := range ifs {
		state := "down"
		if it.Flags&net.FlagUp != 0 {
			state = "up"
		}
		parts = append(parts, it.Name+"("+state+")")
	}
	return strings.Join(parts, ",")
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

// sleeperFlag makes the probe sleep briefly and exit. The detector
// self-check starts the probe this way with a secret in its arguments.
const sleeperFlag = "-g0-sleeper"

func main() {
	if len(os.Args) > 1 && os.Args[1] == sleeperFlag {
		time.Sleep(5 * time.Second)
		return
	}
	java := flag.String("java", "", "path to the java executable")
	jar := flag.String("jar", "", "path to worker/target/g0-worker.jar")
	out := flag.String("out", "g0-evidence.jsonl", "evidence file to append to")
	head := flag.String("head", "unknown", "source commit the run was built from")
	only := flag.String("transports", "", "comma-separated transports (default: all for this OS)")
	pick := flag.String("scenarios", "", "comma-separated scenarios to run (default: all); handshake always runs")
	runtimeLabel := flag.String("runtime", "jdk", "what -java is, for the record: jdk or jlink")
	cancelRuns := flag.Int("cancel-runs", 30, "cancel scenario repetitions")
	flag.Parse()
	if *java == "" || *jar == "" {
		fmt.Fprintln(os.Stderr, "g0probe: -java and -jar are required")
		os.Exit(2)
	}
	// The worker runs in a private scratch directory, so relative paths
	// would resolve from there; make both absolute and check they exist.
	javaPath, err := exec.LookPath(*java)
	if err == nil {
		javaPath, err = filepath.Abs(javaPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "g0probe: java:", err)
		os.Exit(2)
	}
	jarPath, err := filepath.Abs(*jar)
	if err == nil {
		_, err = os.Stat(jarPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "g0probe: jar:", err)
		os.Exit(2)
	}
	runInfo.head = *head
	runInfo.os = osVersion()
	runInfo.versions = moduleVersions()
	runInfo.versions["java_runtime"] = *runtimeLabel
	runInfo.versions["net_interfaces"] = describeInterfaces()
	cfg := config{java: javaPath, jar: jarPath, sourceHead: *head}

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

	selected := map[string]bool{}
	for _, name := range strings.Split(*pick, ",") {
		if name = strings.TrimSpace(name); name != "" {
			selected[name] = true
		}
	}
	want := func(name string) bool { return len(selected) == 0 || selected[name] }

	type step struct {
		name string
		run  func() *evidence
	}
	transports := transportsForOS()
	if *only != "" {
		transports = strings.Split(*only, ",")
	}
	for i, t := range transports {
		hs := scenarioHandshake(cfg, t)
		emit(hs)
		perTransport := []step{
			{"duplex", func() *evidence { return scenarioDuplex(cfg, t) }},
			{"backpressure", func() *evidence { return scenarioBackpressure(cfg, t) }},
			{"noise", func() *evidence { return scenarioNoise(cfg, t) }},
			{"bad-credential", func() *evidence { return scenarioBadCredential(cfg, t) }},
			{"replay", func() *evidence { return scenarioReplay(cfg, t) }},
			{"impersonate", func() *evidence { return scenarioImpersonate(cfg, t) }},
			{"kill", func() *evidence { return scenarioKill(cfg, t) }},
			{"cancel", func() *evidence { return scenarioCancel(cfg, t, *cancelRuns, "cancel", 0) }},
			{"cancel-backpressure", func() *evidence {
				return scenarioCancel(cfg, t, max(*cancelRuns/3, 5), "cancel-backpressure", time.Second)
			}},
			{"oversize", func() *evidence { return scenarioOversize(cfg, t) }},
			{"concurrent", func() *evidence { return scenarioConcurrent(cfg, t, 4) }},
			{"connect-flood", func() *evidence { return scenarioConnectFlood(cfg, t) }},
		}
		for _, st := range perTransport {
			if !want(st.name) {
				continue
			}
			if hs.Status != statusPass {
				e := newEvidence(st.name, t, transportTestIDs(t))
				e.block("not run: the handshake did not pass on this transport")
				emit(e)
				continue
			}
			emit(st.run())
		}
		if i != 0 {
			continue
		}
		// Once per OS: they test the launch, the process or the detectors,
		// not the transport.
		if want("bootstrap") {
			for _, e := range scenarioBootstrap(cfg, t) {
				emit(e)
			}
		}
		perOS := []step{
			{"handshake-timeout", func() *evidence { return scenarioHandshakeTimeout(cfg, t) }},
			{"missing-jvm", func() *evidence { return scenarioMissingJVM(cfg, t) }},
			{"squat", func() *evidence { return scenarioSquat(t) }},
			{"detector-self-check", func() *evidence { return scenarioSelfCheck(t) }},
			{"bridge-selftest", func() *evidence { return scenarioBridgeSelfTest(cfg) }},
			{"tree-kill", func() *evidence { return scenarioTreeKill(cfg, t) }},
			{"crash", func() *evidence { return scenarioCrash(cfg, t) }},
			{"oom", func() *evidence { return scenarioOOM(cfg, t) }},
		}
		for _, st := range perOS {
			if want(st.name) {
				emit(st.run())
			}
		}
	}
	// Diagnostic controls: the cancel case only, for comparison.
	if *only == "" && want("cancel") {
		for _, t := range diagnosticTransports() {
			e := scenarioCancel(cfg, t, *cancelRuns, "cancel", 0)
			e.Notes = append(e.Notes, "diagnostic control, not a candidate transport")
			emit(e)
		}
	}

	counts := map[string]int{}
	for _, e := range all {
		counts[e.Status]++
	}
	fmt.Printf("\ng0probe: %d PASS, %d FAIL, %d BLOCKED on %s (%s); evidence in %s\n",
		counts[statusPass], counts[statusFail], counts[statusBlocked], runtime.GOOS+"/"+runtime.GOARCH, runInfo.os, *out)
}
