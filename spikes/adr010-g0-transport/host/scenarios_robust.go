// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gopmgr-g0spike/host/g0pb"
)

// Robustness scenarios added for the maintainer's G0 Windows transport
// decision (2026-10-25): process-tree cleanup, abnormal JVM termination,
// resource limits, concurrent sessions and connection floods. Each one
// records evidence; none relaxes an existing check.

// spawnSentinel is the record index the worker uses to report a grandchild.
const spawnSentinel = ^uint64(1)

// waitFor checks cond every interval until it holds or the bound passes.
// Test-only: it measures how long the OS took, with a hard upper bound.
func waitFor(bound, interval time.Duration, cond func() bool) (time.Duration, bool) {
	t0 := time.Now()
	for {
		if cond() {
			return time.Since(t0), true
		}
		if time.Since(t0) > bound {
			return time.Since(t0), false
		}
		time.Sleep(interval)
	}
}

// awaitGrandchild reads events until the worker reports its grandchild.
func awaitGrandchild(stream g0pb.WorkerSession_ConnectServer) (int, error) {
	for {
		evt, err := stream.Recv()
		if err != nil {
			return 0, fmt.Errorf("before the grandchild report: %w", err)
		}
		rec := evt.GetRecord()
		if rec == nil || rec.Index != spawnSentinel {
			continue
		}
		report := string(rec.Payload)
		pid, ok := strings.CutPrefix(report, "pid:")
		if !ok {
			return 0, errors.New("worker could not start a grandchild: " + report)
		}
		return strconv.Atoi(pid)
	}
}

// scenarioTreeKill: the worker starts a grandchild; the host ends the
// worker's tree mid-stream; both must be gone (TPT-013).
func scenarioTreeKill(cfg config, transport string) *evidence {
	ev := newEvidence("tree-kill", transport, []string{"TPT-013", "TPT-014"})
	var gpid int
	var aliveBefore bool
	var members int
	var killToExit, grandchildGone time.Duration
	var gone bool
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "spawn", 50_000_000, 64); err != nil {
				return err
			}
			var err error
			if gpid, err = awaitGrandchild(stream); err != nil {
				return err
			}
			// Give the grandchild JVM a moment to be fully started.
			_, aliveBefore = waitFor(5*time.Second, 20*time.Millisecond, func() bool { return processAlive(gpid) })
			members, _ = r.w.tree.members()
			t0 := time.Now()
			r.w.tree.kill()
			select {
			case <-r.w.exited:
				killToExit = time.Since(t0)
			case <-time.After(10 * time.Second):
				return errors.New("worker still running 10 s after the tree kill")
			}
			grandchildGone, gone = waitFor(5*time.Second, 20*time.Millisecond, func() bool { return !processAlive(gpid) })
			for {
				if _, err := stream.Recv(); err != nil {
					return nil
				}
			}
		}})
	ev.absorb(r, err)
	if r == nil || !r.connected {
		return ev
	}
	ev.Metrics["grandchild_alive_before_kill"] = aliveBefore
	ev.Metrics["tree_members_before_kill"] = members
	ev.Metrics["kill_to_worker_exit_ms"] = killToExit.Milliseconds()
	ev.Metrics["kill_to_grandchild_gone_ms"] = (killToExit + grandchildGone).Milliseconds()
	switch {
	case !aliveBefore:
		ev.block("the grandchild never started, so tree cleanup could not be tested")
	case members < 2:
		ev.fail(fmt.Sprintf("the grandchild was not in the worker's tree (members %d)", members))
	case !gone:
		ev.fail("the grandchild survived the tree kill")
	}
	return ev
}

// scenarioCrash: the worker starts a grandchild, then halts mid-stream.
// The host must see the stream fail promptly and leave no orphan behind
// (TPT-013, TPT-014).
func scenarioCrash(cfg config, transport string) *evidence {
	ev := newEvidence("crash", transport, []string{"TPT-013", "TPT-014"})
	var gpid int
	var reportToStreamError time.Duration
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "crash", 50_000_000, 64); err != nil {
				return err
			}
			var err error
			if gpid, err = awaitGrandchild(stream); err != nil {
				return err
			}
			t0 := time.Now()
			for {
				if _, err := stream.Recv(); err != nil {
					reportToStreamError = time.Since(t0)
					return nil
				}
			}
		}})
	ev.absorb(r, err)
	if r == nil || !r.connected {
		return ev
	}
	ev.Metrics["grandchild_report_to_stream_error_ms"] = reportToStreamError.Milliseconds()
	if r.exitCode != exitCrash {
		ev.fail(fmt.Sprintf("worker exit code %d, want %d (halted)", r.exitCode, exitCrash))
	}
	// runJob ended the tree after the exit; the orphan must be gone.
	gone, ok := waitFor(5*time.Second, 20*time.Millisecond, func() bool { return gpid == 0 || !processAlive(gpid) })
	ev.Metrics["orphan_gone_after_cleanup_ms"] = gone.Milliseconds()
	if gpid == 0 {
		ev.block("the grandchild never started")
	} else if !ok {
		ev.fail("an orphaned grandchild survived the host's cleanup")
	}
	return ev
}

// scenarioOOM: the worker exhausts its heap. With
// -XX:+ExitOnOutOfMemoryError it must exit at once with code 3, and the
// host must see the stream fail (TPT-013, TPT-014).
func scenarioOOM(cfg config, transport string) *evidence {
	ev := newEvidence("oom", transport, []string{"TPT-013", "TPT-014"})
	var detect time.Duration
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			t0 := time.Now()
			if err := execute(stream, &seq, "oom", 0, 0); err != nil {
				return err
			}
			for {
				if _, err := stream.Recv(); err != nil {
					detect = time.Since(t0)
					return nil
				}
			}
		}})
	ev.absorb(r, err)
	if r == nil || !r.connected {
		return ev
	}
	ev.Metrics["execute_to_stream_error_ms"] = detect.Milliseconds()
	if r.exitCode != exitJVMOOM {
		ev.fail(fmt.Sprintf("worker exit code %d, want %d (ExitOnOutOfMemoryError)", r.exitCode, exitJVMOOM))
	}
	// HotSpot prints this line to stdout, not stderr.
	if !strings.Contains(r.stdout+r.stderr, "Terminating due to java.lang.OutOfMemoryError") {
		ev.fail("the JVM did not report the out-of-memory exit")
	}
	return ev
}

// scenarioOversize: the worker sends one message larger than the host's
// 4 MiB limit. The host must refuse it with RESOURCE_EXHAUSTED and both
// sides must end cleanly.
func scenarioOversize(cfg config, transport string) *evidence {
	ev := newEvidence("oversize", transport, []string{"TPT-013", "TPT-014"})
	var code codes.Code
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "oversize", 0, 5<<20); err != nil {
				return err
			}
			for {
				if _, err := stream.Recv(); err != nil {
					code = status.Code(err)
					return nil
				}
			}
		}})
	ev.absorb(r, err)
	if r == nil || !r.connected {
		return ev
	}
	ev.Metrics["host_receive_status"] = code.String()
	if code != codes.ResourceExhausted {
		ev.fail("the host did not refuse the oversized message with RESOURCE_EXHAUSTED: " + code.String())
	}
	if r.exitCode == stillRunning {
		ev.fail("the worker did not exit after the refusal")
	}
	return ev
}

// scenarioConcurrent runs several sessions at once, each on its own
// endpoint, with full-duplex traffic. No session may see another's data
// or connection.
func scenarioConcurrent(cfg config, transport string, n int) *evidence {
	ev := newEvidence("concurrent", transport, []string{"TPT-012", "TPT-013", "TPT-018"})
	const records, size, source = 20_000, 256, 8 << 20
	type result struct {
		r   *jobRun
		err error
		out duplexOutcome
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	t0 := time.Now()
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := &results[i]
			res.r, res.err = runJob(cfg, jobOptions{transport: transport, timeout: 180 * time.Second,
				drive: duplexDrive(records, size, source, &res.out)})
		}()
	}
	wg.Wait()
	ev.Metrics["sessions"] = n
	ev.Metrics["wall_ms"] = time.Since(t0).Milliseconds()
	seen := map[string]bool{}
	var rss int64
	for i, res := range results {
		if res.err != nil {
			ev.fail(fmt.Sprintf("session %d: %v", i, res.err))
			continue
		}
		r := res.r
		if !r.connected || r.driveErr != nil {
			ev.fail(fmt.Sprintf("session %d did not complete: %v", i, r.driveErr))
		}
		for _, m := range res.out.mismatch {
			ev.fail(fmt.Sprintf("session %d: %s", i, m))
		}
		if seen[r.ep.addr] {
			ev.fail("two sessions shared an endpoint")
		}
		seen[r.ep.addr] = true
		if v, _ := r.metrics["listener_refused"].(int64); v != 0 {
			ev.fail(fmt.Sprintf("session %d's listener refused %d connections: cross-talk", i, v))
		}
		if v, _ := r.metrics["worker_rss_bytes"].(int64); v > rss {
			rss = v
		}
	}
	ev.Metrics["max_worker_rss_bytes"] = rss
	return ev
}

// scenarioConnectFlood: while the worker starts, another local process
// opens many connections to the endpoint. Every one must be refused
// without taking the worker's slot, and the session must still succeed
// (TPT-018).
func scenarioConnectFlood(cfg config, transport string) *evidence {
	ev := newEvidence("connect-flood", transport, []string{"TPT-006", "TPT-018"})
	const dials = 50
	var connected, failed int
	var mu sync.Mutex
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		beforeConnect: func(r *jobRun) {
			var wg sync.WaitGroup
			for range dials {
				wg.Add(1)
				go func() {
					defer wg.Done()
					c, err := dialEndpoint(r.ep.addr, transport)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						failed++
						return
					}
					connected++
					go func() {
						time.Sleep(300 * time.Millisecond)
						_ = c.Close()
					}()
				}()
			}
			wg.Wait()
		},
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			return finish(stream, &seq)
		}})
	ev.absorb(r, err)
	if r == nil {
		return ev
	}
	refused, _ := r.metrics["listener_refused"].(int64)
	halves, _ := r.metrics["pair_refused_halves"].(int64)
	ev.Metrics["flood_dials"] = dials
	ev.Metrics["flood_connected"] = connected
	ev.Metrics["flood_dial_failed"] = failed
	ev.Metrics["refused_total"] = refused + halves
	if r.sess.handled.Load() != 1 {
		ev.fail(fmt.Sprintf("%d calls reached the handler, want exactly the worker's", r.sess.handled.Load()))
	}
	if connected > 0 && refused+halves == 0 {
		ev.fail("connections from another process were admitted")
	}
	return ev
}

// scenarioBridgeSelfTest runs the worker's in-memory PipeBridge self-test
// (event-loop blocking, bounded buffering, close with pending I/O, second
// connection refused). It needs no pipe, so it runs on every OS.
func scenarioBridgeSelfTest(cfg config) *evidence {
	ev := newEvidence("bridge-selftest", "in-memory", []string{"TPT-012", "TPT-013"})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.java, "-Xmx128m", "-cp", cfg.jar, "org.gopmgr.g0spike.BridgeSelfTest")
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	results := map[string]string{}
	for _, line := range splitLines(string(out)) {
		rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "g0-selftest: ")
		if !ok {
			continue
		}
		name, detail, _ := strings.Cut(rest, "=")
		results[name] = detail
		if name != "summary" && !strings.HasPrefix(detail, "PASS") {
			ev.fail(name + ": " + detail)
		}
	}
	ev.Metrics["results"] = results
	if err != nil && ev.Status == statusPass {
		ev.fail("self-test exited with an error: " + err.Error())
	}
	if len(results) == 0 {
		ev.fail("the self-test printed no results")
	}
	return ev
}
