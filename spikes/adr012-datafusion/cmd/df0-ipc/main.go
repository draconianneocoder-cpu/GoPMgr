// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

// Command df0-ipc runs the ADR-012 DF0-C scenarios: the Go host serves
// gRPC on a private Unix socket and the Rust DataFusion worker connects as
// the client. Every launch is a fresh process and a fresh endpoint.
//
//	go run ./cmd/df0-ipc -worker PATH -corpus DIR -out evidence/df0-c
package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopmgr/internal/analytics"
	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/ipchost"
	"gopmgr/spikes/adr012-datafusion/internal/oracle"
	"gopmgr/spikes/adr012-datafusion/internal/rollupwire"
)

const cancelCycles = 50

func main() {
	worker := flag.String("worker", "", "df0-worker binary (required)")
	corpus := flag.String("corpus", "", "corpus directory, for the rollup cases (required)")
	out := flag.String("out", "evidence/df0-c", "evidence directory")
	orphanHost := flag.Bool("orphan-host", false, "internal: act as a host that is killed mid-job")
	foreign := flag.String("foreign-server", "", "internal: serve this socket path as a process that did not launch the worker")
	flag.Parse()
	if *orphanHost {
		orphanHostMain(*worker)
		return
	}
	if *foreign != "" {
		foreignServerMain(*foreign)
		return
	}
	if *worker == "" || *corpus == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*worker, *corpus, *out); err != nil {
		fmt.Fprintln(os.Stderr, "df0-ipc:", err)
		os.Exit(1)
	}
}

// record is one scenario's evidence line.
type record struct {
	TestIDs  []string `json:"test_ids"`
	Scenario string   `json:"scenario"`
	Expect   string   `json:"expect"`
	Status   string   `json:"status"` // PASS or FAIL
	Why      []string `json:"why,omitempty"`
	ipchost.Result
}

type runner struct {
	worker string
	corpus string
	f      *os.File
	counts map[string]int
	failed []string
}

func (r *runner) write(rec record) error {
	if len(rec.Why) == 0 {
		rec.Status = "PASS"
	} else {
		rec.Status = "FAIL"
		r.failed = append(r.failed, rec.Scenario)
	}
	r.counts[rec.Status]++
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = r.f.Write(append(b, '\n'))
	return err
}

// invariants hold on every launch that reached the stream: no TCP or UDP
// socket in the worker or the host, no secret in argv or environment, an
// empty environment, private endpoint modes, cleanup and reaping.
func invariants(res ipchost.Result, connected bool) []string {
	var why []string
	if res.DirMode != "-rwx------" || res.SocketMode != "-rw-------" {
		why = append(why, fmt.Sprintf("endpoint modes %s %s", res.DirMode, res.SocketMode))
	}
	if !res.EndpointRemoved {
		why = append(why, "endpoint not removed")
	}
	if res.WorkerExit == "unknown" {
		why = append(why, "worker not reaped")
	}
	if !connected {
		return why
	}
	switch {
	case res.WorkerSockets == nil || res.HostSockets == nil:
		why = append(why, "socket inspection did not run: "+res.InspectError)
	default:
		w, h := res.WorkerSockets, res.HostSockets
		if w.TCP+w.UDP+h.TCP+h.UDP != 0 {
			why = append(why, fmt.Sprintf("IP sockets: worker tcp %d udp %d, host tcp %d udp %d", w.TCP, w.UDP, h.TCP, h.UDP))
		}
		if w.Unix < 1 {
			why = append(why, "worker holds no Unix socket")
		}
	}
	switch {
	case res.SecretsVisible == nil:
		why = append(why, "argv/env leak check did not run")
	case *res.SecretsVisible:
		why = append(why, "credential visible in worker argv or environment")
	}
	if res.WorkerEnvNames != 0 {
		why = append(why, fmt.Sprintf("worker received %d environment variables", res.WorkerEnvNames))
	}
	return why
}

func genJob(name, worker string, rows uint64, batch, width uint32) (ipchost.Job, *ipchost.GenVerifier) {
	g := &ipchost.GenVerifier{Width: int(width)}
	return ipchost.Job{Name: name, Worker: worker, Verify: g.Check,
		Execute: &df0pb.Execute{Operation: "generate", Rows: rows, BatchRows: batch, PayloadBytes: width}}, g
}

func run(worker, corpus, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(out, "ipc-"+runtime.GOOS+"-"+runtime.GOARCH+".jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	r := &runner{worker: worker, corpus: corpus, f: f, counts: map[string]int{}}
	steps := []func() error{r.detectorSelfTest, r.volume, r.duplex, r.rollups, r.backpressure, r.faults, r.auth, r.foreignHost, r.squat, r.orphan, r.cancelCycles}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	if err := environment(worker, filepath.Join(out, "environment.json")); err != nil {
		return err
	}
	fmt.Printf("df0-ipc: %d PASS, %d FAIL\n", r.counts["PASS"], r.counts["FAIL"])
	if len(r.failed) > 0 {
		fmt.Printf("df0-ipc: failed: %s\n", strings.Join(r.failed, ", "))
	}
	return nil
}

// volume: 1, 64 and 1024 batches, frame splitting, and over 1 GiB streamed
// with bounded host and worker memory (DF0-C item 4).
func (r *runner) volume() error {
	type tc struct {
		name           string
		rows           uint64
		batch, width   uint32
		wantBatches    uint64
		minBytes       int64
		maxHostHeap    uint64
		maxWorkerRSS   int64
		splitsExpected bool
	}
	cases := []tc{
		{name: "generate-1-batch", rows: 1000, batch: 1000, width: 16, wantBatches: 1},
		{name: "generate-64-batches", rows: 64 * 512, batch: 512, width: 64, wantBatches: 64},
		{name: "generate-1024-batches", rows: 1024 * 100, batch: 100, width: 16, wantBatches: 1024},
		{name: "generate-frame-split", rows: 2000, batch: 2000, width: 4096, splitsExpected: true},
		{name: "generate-over-1GiB", rows: 104_858, batch: 256, width: 10_240, minBytes: 1 << 30,
			maxHostHeap: 128 << 20, maxWorkerRSS: 512 << 20},
	}
	for _, c := range cases {
		job, g := genJob(c.name, r.worker, c.rows, c.batch, c.width)
		job.Timeout = 5 * time.Minute
		res := ipchost.Run(job)
		rec := record{TestIDs: []string{"DF-021", "DF-027"}, Scenario: c.name, Expect: "completed; every row verified by the host", Result: res}
		rec.Why = invariants(res, true)
		if res.Outcome != "completed" {
			rec.Why = append(rec.Why, "outcome "+res.Outcome+": "+res.Detail)
		}
		if res.Rows != c.rows || uint64(g.Next) != c.rows {
			rec.Why = append(rec.Why, fmt.Sprintf("rows %d (verified %d), want %d", res.Rows, g.Next, c.rows))
		}
		if c.wantBatches > 0 && res.Batches != c.wantBatches {
			rec.Why = append(rec.Why, fmt.Sprintf("batches %d, want %d", res.Batches, c.wantBatches))
		}
		if c.splitsExpected && res.Batches < 2 {
			rec.Why = append(rec.Why, "an over-cap batch was not split")
		}
		if res.MaxBatch+256 > ipchost.DefaultMaxFrame {
			rec.Why = append(rec.Why, fmt.Sprintf("batch of %d bytes over the frame cap", res.MaxBatch))
		}
		if c.minBytes > 0 && res.IPCBytes < c.minBytes {
			rec.Why = append(rec.Why, fmt.Sprintf("streamed %d bytes, want at least %d", res.IPCBytes, c.minBytes))
		}
		if c.maxHostHeap > 0 && res.HostHeapPeak > c.maxHostHeap {
			rec.Why = append(rec.Why, fmt.Sprintf("host heap peak %d over %d", res.HostHeapPeak, c.maxHostHeap))
		}
		if c.maxWorkerRSS > 0 && (res.WorkerPeakRSS <= 0 || res.WorkerPeakRSS > c.maxWorkerRSS) {
			rec.Why = append(rec.Why, fmt.Sprintf("worker peak RSS %d, bound %d", res.WorkerPeakRSS, c.maxWorkerRSS))
		}
		if err := r.write(rec); err != nil {
			return err
		}
	}
	return nil
}

// duplex: the host streams 64 MiB of source while the worker streams rows
// back on the same stream (DF0-C item 5).
func (r *runner) duplex() error {
	src := make([]byte, 64<<20)
	if _, err := io.ReadFull(rand.Reader, src); err != nil {
		return err
	}
	job, g := genJob("duplex-64MiB-source", r.worker, 20_000, 500, 1024)
	job.Execute.Operation = "duplex"
	job.Source = src
	res := ipchost.Run(job)
	rec := record{TestIDs: []string{"DF-021", "DF-028"}, Scenario: job.Name, Expect: "completed; source digest and every row verified", Result: res}
	rec.Why = invariants(res, true)
	if res.Outcome != "completed" || res.SourceMatch == nil || !*res.SourceMatch || g.Next != 20_000 {
		rec.Why = append(rec.Why, fmt.Sprintf("outcome %s %s, source match %v, rows %d", res.Outcome, res.Detail, res.SourceMatch, g.Next))
	}
	return r.write(rec)
}

// rollups: every DF0 portfolio case through the worker over gRPC, scored by
// the same oracle as DF0-B.
func (r *runner) rollups() error {
	pass, total := 0, 0
	for _, c := range fixtures.PortfolioCases() {
		b, err := os.ReadFile(filepath.Join(r.corpus, "portfolio", c.ID+".json")) // #nosec G304 -- corpus file.
		if err != nil {
			return err
		}
		pc, err := fixtures.DecodePortfolio(b)
		if err != nil {
			return err
		}
		want, _ := oracle.Rollup(pc.Projects)
		norm := make([]map[string]any, len(pc.Projects))
		for i, p := range pc.Projects {
			norm[i] = rollupwire.Normalise(p)
		}
		src, err := json.Marshal(norm)
		if err != nil {
			return err
		}
		res := ipchost.Run(ipchost.Job{Name: "rollup-" + c.ID, Worker: r.worker, Source: src,
			Execute: &df0pb.Execute{Operation: "rollup"}})
		rec := record{TestIDs: []string{"DF-021", "DF-002", "DF-054"}, Scenario: res.Name,
			Expect: "completed; result equals the legacy oracle", Result: res}
		rec.Why = invariants(res, true)
		var got analytics.PortfolioSummary
		var gerr error
		var rr rollupwire.Result
		switch {
		case res.Outcome != "completed":
			gerr = errors.New(res.Outcome + ": " + res.Detail)
		case json.Unmarshal([]byte(res.ResultJSON), &rr) != nil:
			gerr = errors.New("result is not JSON")
		case rr.Error != "":
			gerr = errors.New(rr.Error)
		default:
			got, gerr = rollupwire.Summary(rr)
		}
		switch {
		case gerr != nil && want.Err != "":
		case gerr != nil || want.Err != "":
			rec.Why = append(rec.Why, fmt.Sprintf("error %v, oracle error %q", gerr, want.Err))
		default:
			rec.Why = append(rec.Why, oracle.CompareSummary(got, want.Summary)...)
		}
		rec.ResultJSON = "" // the totals are checked above; keep evidence small
		total++
		if len(rec.Why) == 0 {
			pass++
		}
		if err := r.write(rec); err != nil {
			return err
		}
	}
	fmt.Printf("df0-ipc: rollup over gRPC: %d of %d match the legacy oracle\n", pass, total)
	return nil
}

// backpressure: a slow host must hold the worker back through HTTP/2 flow
// control, with bounded memory on both sides (DF-028).
func (r *runner) backpressure() error {
	job, _ := genJob("slow-consumer", r.worker, 4000, 50, 8192)
	job.SlowPerBatch = 2 * time.Millisecond
	res := ipchost.Run(job)
	rec := record{TestIDs: []string{"DF-028"}, Scenario: job.Name,
		Expect: "completed; the worker waited on flow control; memory bounded", Result: res}
	rec.Why = invariants(res, true)
	if res.Outcome != "completed" {
		rec.Why = append(rec.Why, "outcome "+res.Outcome+": "+res.Detail)
	}
	if res.SendWaits == 0 {
		rec.Why = append(rec.Why, "worker never waited: no backpressure observed")
	}
	if res.WorkerPeakRSS <= 0 || res.WorkerPeakRSS > 256<<20 || res.HostHeapPeak > 64<<20 {
		rec.Why = append(rec.Why, fmt.Sprintf("worker RSS %d, host heap %d", res.WorkerPeakRSS, res.HostHeapPeak))
	}
	if err := r.write(rec); err != nil {
		return err
	}

	// Cancel while the worker is blocked on a full stream.
	job, _ = genJob("cancel-while-blocked", r.worker, 1_000_000, 50, 8192)
	job.SlowPerBatch = 5 * time.Millisecond
	job.CancelAfter = 20
	res = ipchost.Run(job)
	rec = record{TestIDs: []string{"DF-028", "DF-039"}, Scenario: job.Name,
		Expect: "failed USER_CANCELLED; clean exit, no kill", Result: res}
	rec.Why = append(invariants(res, true), cancelChecks(res)...)
	return r.write(rec)
}

func cancelChecks(res ipchost.Result) []string {
	var why []string
	if res.Outcome != "failed" || res.FailedCode != "USER_CANCELLED" {
		why = append(why, fmt.Sprintf("outcome %s %s %s", res.Outcome, res.FailedCode, res.Detail))
	}
	if res.Killed || res.WorkerExit != "exit 0" {
		why = append(why, fmt.Sprintf("worker %s, killed %v", res.WorkerExit, res.Killed))
	}
	if res.CancelToExitNS < 0 || res.CancelToExitNS > int64(2*time.Second) {
		why = append(why, fmt.Sprintf("cancel to exit %v", time.Duration(res.CancelToExitNS)))
	}
	return why
}

// faults: the worker breaks one rule; the host must refuse the stream,
// kill the worker and clean up (DF-027, DF-029, DF-040).
func (r *runner) faults() error {
	cases := []struct {
		fault, outcome, exit string
		ids                  []string
	}{
		{"gap", "rejected_sequence_gap", "signal killed", []string{"DF-027"}},
		{"duplicate", "rejected_sequence_repeat", "signal killed", []string{"DF-027"}},
		{"bad_schema", "rejected_batch", "signal killed", []string{"DF-027", "DF-040"}},
		{"bad_hash", "rejected_digest", "signal killed", []string{"DF-027"}},
		{"exit_mid_stream", "stream_ended", "exit 9", []string{"DF-029"}},
		{"hang", "timeout", "signal killed", []string{"DF-029", "DF-039"}},
	}
	for _, c := range cases {
		job, _ := genJob("fault-"+c.fault, r.worker, 20_000, 100, 64)
		job.Execute.Fault = c.fault
		if c.fault == "hang" {
			job.Timeout = 5 * time.Second
		}
		res := ipchost.Run(job)
		rec := record{TestIDs: c.ids, Scenario: job.Name, Expect: c.outcome + "; worker " + c.exit, Result: res}
		rec.Why = invariants(res, true)
		if res.Outcome != c.outcome {
			rec.Why = append(rec.Why, fmt.Sprintf("outcome %s (%s)", res.Outcome, res.Detail))
		}
		if res.WorkerExit != c.exit {
			rec.Why = append(rec.Why, "worker "+res.WorkerExit)
		}
		if err := r.write(rec); err != nil {
			return err
		}
	}
	return nil
}

// auth: a wrong proof, an expired bootstrap, and a foreign process that
// dials first (DF-024, DF-026, DF-037).
func (r *runner) auth() error {
	job, _ := genJob("auth-wrong-proof", r.worker, 100, 100, 8)
	job.CorruptProof = true
	res := ipchost.Run(job)
	rec := record{TestIDs: []string{"DF-024"}, Scenario: job.Name,
		Expect: "refused before any business message; worker exits 3", Result: res}
	rec.Why = invariants(res, false)
	if res.Outcome != "worker_exit_before_stream" || res.SessionRefused != 1 || res.WorkerExit != "exit 3" {
		rec.Why = append(rec.Why, fmt.Sprintf("outcome %s, session refused %d, worker %s", res.Outcome, res.SessionRefused, res.WorkerExit))
	}
	if err := r.write(rec); err != nil {
		return err
	}

	job, _ = genJob("auth-expired-bootstrap", r.worker, 100, 100, 8)
	job.Expired = true
	res = ipchost.Run(job)
	rec = record{TestIDs: []string{"DF-024"}, Scenario: job.Name,
		Expect: "worker refuses the bootstrap and exits 2 without connecting", Result: res}
	rec.Why = invariants(res, false)
	if res.Outcome != "worker_exit_before_stream" || res.WorkerExit != "exit 2" || res.ListenerRefused+res.SessionRefused != 0 {
		rec.Why = append(rec.Why, fmt.Sprintf("outcome %s, worker %s, refused %d/%d", res.Outcome, res.WorkerExit, res.ListenerRefused, res.SessionRefused))
	}
	if err := r.write(rec); err != nil {
		return err
	}

	job, g := genJob("auth-intruder-first", r.worker, 1000, 100, 8)
	job.Intruder = true
	res = ipchost.Run(job)
	rec = record{TestIDs: []string{"DF-026", "DF-037"}, Scenario: job.Name,
		Expect: "the intruder is refused and receives nothing; the worker then completes", Result: res}
	rec.Why = invariants(res, true)
	if res.ListenerRefused < 1 || res.IntruderData == nil || *res.IntruderData || res.Outcome != "completed" || g.Next != 1000 {
		rec.Why = append(rec.Why, fmt.Sprintf("refused %d, intruder data %v, outcome %s", res.ListenerRefused, res.IntruderData, res.Outcome))
	}
	return r.write(rec)
}

// squat: the host refuses an endpoint path someone pre-placed (DF-026).
func (r *runner) squat() error {
	dir, err := os.MkdirTemp(ipchost.EndpointBase, "gopmgr-df0-squat-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	rec := record{TestIDs: []string{"DF-026"}, Scenario: "squat-preplaced-link", Expect: "host refuses to listen", Result: ipchost.NewResult("squat")}
	p := filepath.Join(dir, "w.sock")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), p); err != nil {
		return err
	}
	ep, err := ipchost.ListenAt(p)
	switch {
	case err == nil:
		_ = ep.Close()
		rec.Why = []string{"host listened on a pre-placed link"}
	case !errors.Is(err, ipchost.ErrEndpointExists):
		rec.Why = []string{"refused for another reason: " + err.Error()}
	}
	rec.Outcome, rec.WorkerExit, rec.EndpointRemoved = "refused", "n/a", true
	return r.write(rec)
}

// orphan: a host killed mid-job must not leave its worker running (DF-029).
func (r *runner) orphan() error {
	rec := record{TestIDs: []string{"DF-029"}, Scenario: "host-killed-mid-job",
		Expect: "the kernel kills the worker within 2 s; the worker's endpoint dir is the only residue",
		Result: ipchost.NewResult("orphan")}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "-orphan-host", "-worker", r.worker)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("orphan host: %w", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(line))
	killedAt := time.Now()
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()
	gone := false
	for time.Since(killedAt) < 2*time.Second {
		if !ipchost.Alive(pid) {
			gone = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	took := time.Since(killedAt)
	rec.Outcome, rec.WorkerExit = "host killed", map[bool]string{true: "dead (exited or zombie)", false: "still running"}[gone]
	if !gone {
		rec.Why = []string{"worker outlived its host"}
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	// A killed host cannot run its cleanup: record what it left behind.
	left, _ := filepath.Glob(filepath.Join(ipchost.EndpointBase, "gopmgr-df0-*"))
	rec.Detail = fmt.Sprintf("host SIGKILL to worker dead: %v; endpoint dirs left by the killed host: %d (a supervisor sweeps them at start)",
		took.Round(time.Microsecond), len(left))
	for _, d := range left {
		_ = os.RemoveAll(d)
	}
	rec.EndpointRemoved = true
	return r.write(rec)
}

func orphanHostMain(worker string) {
	job, _ := genJob("orphan", worker, 100_000_000, 100, 64)
	job.SlowPerBatch = 10 * time.Millisecond
	job.OnSpawn = func(pid int) { fmt.Println(pid) }
	ipchost.Run(job)
}

// cancelCycles: 50 fresh workers, each cancelled mid-stream; checks for
// leaked descriptors, threads, goroutines and endpoints (DF0-C item 5).
func (r *runner) cancelCycles() error {
	runtime.GC()
	fds0, thr0, gor0 := ipchost.FDCount(os.Getpid()), ipchost.Threads(os.Getpid()), runtime.NumGoroutine()
	var toFailed, toExit []time.Duration
	for i := range cancelCycles {
		job, _ := genJob(fmt.Sprintf("cancel-cycle-%02d", i+1), r.worker, 10_000_000, 1000, 64)
		job.CancelAfter = 5
		res := ipchost.Run(job)
		rec := record{TestIDs: []string{"DF-039", "DF-029"}, Scenario: job.Name,
			Expect: "failed USER_CANCELLED; clean exit within 2 s", Result: res}
		rec.Why = append(invariants(res, true), cancelChecks(res)...)
		if res.CancelToFailedNS >= 0 {
			toFailed = append(toFailed, time.Duration(res.CancelToFailedNS))
		}
		if res.CancelToExitNS >= 0 {
			toExit = append(toExit, time.Duration(res.CancelToExitNS))
		}
		if err := r.write(rec); err != nil {
			return err
		}
	}
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	fds1, thr1, gor1 := ipchost.FDCount(os.Getpid()), ipchost.Threads(os.Getpid()), runtime.NumGoroutine()
	left, _ := filepath.Glob(filepath.Join(ipchost.EndpointBase, "gopmgr-df0-*"))
	rec := record{TestIDs: []string{"DF-029"}, Scenario: "cancel-cycles-leak-check",
		Expect: fmt.Sprintf("after %d cycles: no growth in host fds, goroutines or endpoint dirs", cancelCycles),
		Result: ipchost.NewResult("leak-check")}
	rec.Outcome = "measured"
	rec.Detail = fmt.Sprintf("fds %d→%d, threads %d→%d, goroutines %d→%d, endpoint dirs left %d; cancel→Failed %s; cancel→exit %s",
		fds0, fds1, thr0, thr1, gor0, gor1, len(left), dist(toFailed), dist(toExit))
	if fds1 > fds0 || gor1 > gor0+2 || len(left) != 0 {
		rec.Why = []string{"resource growth: " + rec.Detail}
	}
	rec.EndpointRemoved, rec.WorkerExit = len(left) == 0, "n/a"
	return r.write(rec)
}

// dist reports n, min, median, p90 and max. With n = 50, p99 is not
// estimable and is not reported.
func dist(d []time.Duration) string {
	if len(d) == 0 {
		return "none"
	}
	s := slices.Clone(d)
	slices.Sort(s)
	q := func(p float64) time.Duration { return s[min(len(s)-1, int(p*float64(len(s))))] }
	return fmt.Sprintf("n=%d min %v p50 %v p90 %v max %v", len(s), s[0].Round(time.Microsecond),
		q(0.5).Round(time.Microsecond), q(0.9).Round(time.Microsecond), s[len(s)-1].Round(time.Microsecond))
}

func environment(worker, path string) error {
	env := map[string]any{"go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"cpus": runtime.NumCPU(), "run_at": time.Now().UTC().Format(time.RFC3339),
		"transport":  "gRPC over a Unix socket; host is the server, worker the client",
		"inspection": ipchost.InspectMethod, "max_frame_bytes": ipchost.DefaultMaxFrame}
	if b, err := exec.Command("rustc", "--version").Output(); err == nil {
		env["rustc"] = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		env["kernel"] = strings.TrimSpace(string(b))
	}
	if head, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		env["source_commit"] = strings.TrimSpace(string(head))
	}
	if st, err := exec.Command("git", "status", "--porcelain", "--", "../..", ":(exclude)evidence").Output(); err == nil {
		env["worktree_dirty"] = len(strings.TrimSpace(string(st))) > 0
	}
	if b, err := os.ReadFile("rust/Cargo.lock"); err == nil {
		var pins []string
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			for _, name := range []string{"datafusion", "arrow", "tonic", "prost", "h2", "hyper", "hyper-util", "tower", "tokio"} {
				if l == fmt.Sprintf("name = %q", name) && i+1 < len(lines) {
					pins = append(pins, name+" "+strings.Trim(strings.TrimPrefix(lines[i+1], "version = "), `"`))
				}
			}
		}
		env["crates"] = pins
	}
	if b, err := os.ReadFile(worker); err == nil { // #nosec G304 -- the worker binary under test.
		sum := sha256.Sum256(b)
		env["worker_bytes"], env["worker_sha256"] = len(b), hex.EncodeToString(sum[:])
	}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644) // #nosec G306 -- evidence file.
}

// detectorSelfTest proves the socket and leak detectors can see what they
// look for, before any "no TCP" or "no leak" result is trusted. If the
// detector cannot be shown to work, the dependent rows are not PASS.
func (r *runner) detectorSelfTest() error {
	rec := record{TestIDs: []string{"DF-021", "DF-036"}, Scenario: "detector-self-test",
		Expect: "the socket detector sees a planted TCP listener and UDP socket; the leak detector sees planted argv and env sentinels",
		Result: ipchost.NewResult("detector-self-test")}
	rec.Outcome, rec.WorkerExit, rec.EndpointRemoved = "measured", "n/a", true
	tcp, terr := net.Listen("tcp", "127.0.0.1:0")
	udp, uerr := net.ListenPacket("udp", "127.0.0.1:0")
	switch {
	case terr != nil || uerr != nil:
		rec.Why = append(rec.Why, fmt.Sprintf("cannot plant IP sockets (%v, %v): the no-TCP result is BLOCKED", terr, uerr))
	default:
		s, err := ipchost.InspectSockets(os.Getpid())
		if err != nil || s.TCPListen < 1 || s.UDP < 1 {
			rec.Why = append(rec.Why, fmt.Sprintf("socket detector missed planted sockets: %+v %v", s, err))
		}
		rec.HostSockets = &s
	}
	if tcp != nil {
		_ = tcp.Close()
	}
	if udp != nil {
		_ = udp.Close()
	}
	argv, envs := "df0-sentinel-argv-7f3a", "df0-sentinel-env-91c2"
	// "; :" keeps the shell alive (no exec into sleep); argv becomes $0.
	cmd := exec.Command("/bin/sh", "-c", "sleep 5; :", argv)
	cmd.Env = []string{"X=" + envs}
	if err := cmd.Start(); err != nil {
		rec.Why = append(rec.Why, "cannot start the leak probe: "+err.Error())
		return r.write(rec)
	}
	time.Sleep(50 * time.Millisecond)
	inArgv, err1 := ipchost.SecretsVisible(cmd.Process.Pid, []byte(argv))
	inEnv, err2 := ipchost.SecretsVisible(cmd.Process.Pid, []byte(envs))
	clean, err3 := ipchost.SecretsVisible(cmd.Process.Pid, []byte("df0-not-present"))
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if err1 != nil || err2 != nil || err3 != nil || !inArgv || !inEnv || clean {
		rec.Why = append(rec.Why, fmt.Sprintf("leak detector: argv %v env %v absent-sentinel %v (%v %v %v)", inArgv, inEnv, clean, err1, err2, err3))
	}
	return r.write(rec)
}

// foreignHost: the endpoint is served by a process other than the one
// that launched the worker. The worker must refuse it before sending a
// byte (mutual authentication, DF-024).
func (r *runner) foreignHost() error {
	rec := record{TestIDs: []string{"DF-024", "DF-037"}, Scenario: "worker-refuses-foreign-host",
		Expect: "worker exits 3; the foreign server receives 0 bytes", Result: ipchost.NewResult("foreign-host")}
	rec.Outcome = "measured"
	dir, err := os.MkdirTemp(ipchost.EndpointBase, "gopmgr-df0-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "w.sock")
	self, err := os.Executable()
	if err != nil {
		return err
	}
	srv := exec.Command(self, "-foreign-server", path)
	out, err := srv.StdoutPipe()
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	rd := bufio.NewReader(out)
	if l, err := rd.ReadString('\n'); err != nil || strings.TrimSpace(l) != "ready" {
		_ = srv.Process.Kill()
		_ = srv.Wait()
		return fmt.Errorf("foreign server: %q %v", l, err)
	}
	cred := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, cred); err != nil {
		return err
	}
	boot, err := ipchost.EncodeBootstrap(&df0pb.Bootstrap{Version: 1, OperationId: "foreign", Endpoint: path,
		Credential: cred, ExpiresUnixMs: time.Now().Add(time.Minute).UnixMilli(), MaxFrameBytes: ipchost.DefaultMaxFrame})
	if err != nil {
		return err
	}
	var stderr strings.Builder
	w := exec.Command(r.worker)
	w.Env = []string{}
	w.Stdin = strings.NewReader(string(boot))
	w.Stderr = &stderr
	werr := w.Run()
	rec.WorkerExit = "exit 0"
	if ee := (*exec.ExitError)(nil); errors.As(werr, &ee) {
		rec.WorkerExit = fmt.Sprintf("exit %d", ee.ExitCode())
	}
	l, _ := rd.ReadString('\n')
	_ = srv.Wait()
	reason := strings.TrimSpace(stderr.String())
	rec.Detail = "foreign server: " + strings.TrimSpace(l) + "; worker: " + fixtures.RedactDir(reason, dir)
	rec.EndpointRemoved = true
	if rec.WorkerExit != "exit 3" || strings.TrimSpace(l) != "received 0" || !strings.Contains(reason, "not the launching host") {
		rec.Why = []string{fmt.Sprintf("worker %s, %s", rec.WorkerExit, rec.Detail)}
	}
	return r.write(rec)
}

func foreignServerMain(path string) {
	ln, err := net.Listen("unix", path)
	if err != nil {
		fmt.Println("listen:", err)
		os.Exit(1)
	}
	fmt.Println("ready")
	c, err := ln.Accept()
	if err != nil {
		fmt.Println("accept:", err)
		os.Exit(1)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _ := io.Copy(io.Discard, c)
	fmt.Printf("received %d\n", n)
}
