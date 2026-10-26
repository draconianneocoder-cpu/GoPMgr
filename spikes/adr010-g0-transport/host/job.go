// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"gopmgr-g0spike/host/g0pb"
)

// config is fixed for a whole run.
type config struct {
	java, jar  string
	sourceHead string
}

// jobOptions shape one worker launch.
type jobOptions struct {
	transport       string
	drive           func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error
	wrongCredential bool          // give the worker a credential the host won't accept
	rawBootstrap    []byte        // write these bytes instead of a valid bootstrap
	expires         time.Time     // zero means 60 s from launch
	workerEndpoint  string        // tell the worker this endpoint instead of the real one
	acceptMany      bool          // keep the listener open after the first connection
	timeout         time.Duration // whole job; zero means 120 s
	javaPath        string        // override cfg.java
	beforeConnect   func(r *jobRun)
}

// jobRun is what a scenario can see and record about one job.
type jobRun struct {
	ep        *endpoint
	sess      *session
	pl        *peerListener
	w         *worker
	hello     *g0pb.WorkerHello
	coldStart time.Duration
	secrets   [][]byte
	mu        sync.Mutex
	metrics   map[string]any
	exitCode  int
	driveErr  error
	connected bool
	stderr    string // the worker's captured stderr, for diagnostics
	stdout    string // the worker's captured stdout, for diagnostics
	startedAt time.Time
	endedAt   time.Time
	// live is the worker as the session goroutine sees it. The worker can
	// connect before start returns and the main goroutine sets w, so code
	// that runs in the session (drive functions) must use worker(), not w.
	// The race detector found the unsynchronized reads.
	live *worker
}

func (r *jobRun) setWorker(w *worker) {
	r.mu.Lock()
	r.live = w
	r.mu.Unlock()
}

// worker returns the launched worker for code running in the session.
func (r *jobRun) worker() *worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.live
}

func (r *jobRun) set(k string, v any) {
	r.mu.Lock()
	r.metrics[k] = v
	r.mu.Unlock()
}

// inspect records what the OS shows about the live worker: secrets in its
// command line or environment, TCP sockets it owns, and its memory.
func (r *jobRun) inspect(pid int) {
	how, leaked, err := leakCheck(pid, r.secrets)
	r.set("credential_leak_check", how)
	if err != nil {
		r.set("credential_leak_check_error", err.Error())
	} else {
		r.set("credential_leaked", leaked)
	}
	n, method, err := tcpSocketCount(pid)
	r.set("tcp_socket_method", method)
	if err != nil {
		r.set("tcp_socket_check_error", err.Error())
	} else {
		r.set("tcp_sockets", n)
	}
}

func (r *jobRun) sampleRSS() {
	v, how := peakRSS(r.worker().pid)
	r.set("worker_rss_bytes", v)
	r.set("worker_rss_method", how)
}

var errNoConnection = errors.New("worker never completed a session")

// runJob launches one worker for one job against a fresh endpoint.
func runJob(cfg config, o jobOptions) (*jobRun, error) {
	if o.timeout == 0 {
		o.timeout = 120 * time.Second
	}
	ep, err := listen(o.transport)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer ep.cleanup()

	cred, err := newCredential()
	if err != nil {
		return nil, err
	}
	opID, err := randomID()
	if err != nil {
		return nil, err
	}
	workerID := "org.gopmgr.g0spike"
	r := &jobRun{ep: ep, metrics: map[string]any{}, exitCode: stillRunning, startedAt: time.Now()}
	// The credential in every encoding a careless worker might print.
	r.secrets = [][]byte{cred, []byte(fmt.Sprintf("%x", cred)), []byte(fmt.Sprintf("%X", cred)),
		[]byte(base64.StdEncoding.EncodeToString(cred)), []byte(base64.RawURLEncoding.EncodeToString(cred))}

	done := make(chan error, 1)
	r.sess = &session{proof: expectedProof(cred, opID, workerID)}
	r.sess.drive = func(stream g0pb.WorkerSession_ConnectServer) error {
		ev, err := stream.Recv()
		if err != nil {
			done <- fmt.Errorf("receive hello: %w", err)
			return err
		}
		if ev.GetHello() == nil {
			err := fmt.Errorf("first event was %T, want hello", ev.GetEvent())
			done <- err
			return err
		}
		w := r.worker()
		r.hello = ev.GetHello()
		r.coldStart = time.Since(w.started)
		r.connected = true
		r.inspect(w.pid)
		var derr error
		if o.drive != nil {
			derr = o.drive(r, stream)
		}
		done <- derr
		return derr
	}
	r.pl = &peerListener{Listener: ep.ln, once: !o.acceptMany}
	srv := newServer(r.sess)
	go func() { _ = srv.Serve(r.pl) }()
	defer srv.Stop()

	scratch, err := os.MkdirTemp("", "gopmgr-g0-scratch-")
	if err != nil {
		return nil, fmt.Errorf("scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	given := cred
	if o.wrongCredential {
		if given, err = newCredential(); err != nil {
			return nil, err
		}
	}
	expires := o.expires
	if expires.IsZero() {
		expires = time.Now().Add(60 * time.Second)
	}
	workerEP := ep.addr
	if o.workerEndpoint != "" {
		workerEP = o.workerEndpoint
	}
	java := cfg.java
	if o.javaPath != "" {
		java = o.javaPath
	}
	r.w, err = start(launchSpec{
		java: java, jar: cfg.jar, transport: o.transport, endpoint: workerEP,
		credential: given, expires: expires, bootstrap: o.rawBootstrap, scratch: scratch,
		onStarted: func(w *worker) {
			r.setWorker(w)
			r.pl.setExpected(w.pid)
		},
	}, opID, workerID)
	if err != nil {
		return r, err
	}
	if o.beforeConnect != nil {
		o.beforeConnect(r)
	}

	select {
	case r.driveErr = <-done:
	case <-r.w.exited:
		r.driveErr = errNoConnection
		// A session may still be finishing; give it a moment to report.
		select {
		case derr := <-done:
			r.driveErr = derr
		case <-time.After(2 * time.Second):
		}
	case <-time.After(o.timeout):
		r.driveErr = fmt.Errorf("job timed out after %v", o.timeout)
	}

	r.exitCode = r.w.exitCode(15 * time.Second)
	if r.exitCode == stillRunning {
		r.set("killed_after_timeout", true)
		r.w.kill()
		r.exitCode = r.w.exitCode(5 * time.Second)
	}
	// No orphans (TPT-013): count what the worker left behind, then end
	// the whole tree whatever the exit path was.
	left, how := r.w.tree.members()
	r.set("tree_left_after_exit", left)
	r.set("tree_method", how)
	r.w.tree.release()
	r.set("worker_cpu_ms", r.w.cpuTime().Milliseconds())
	out, outDropped := r.w.stdout.snapshot()
	r.stdout = out
	errText, errDropped := r.w.stderr.snapshot()
	r.stderr = errText
	r.set("stdout_kept_bytes", len(out))
	r.set("stdout_dropped_bytes", outDropped)
	r.set("stderr_kept_bytes", len(errText))
	r.set("stderr_dropped_bytes", errDropped)
	r.set("stderr_head", firstLines(errText, 6))
	if ps, ok := r.ep.ln.(interface{ pairStats() (int64, int64) }); ok {
		refused, timeouts := ps.pairStats()
		r.set("pair_refused_halves", refused)
		r.set("pair_timeouts", timeouts)
	}
	r.set("listener_refused", r.pl.refused.Load())
	r.set("listener_admitted", r.pl.admitted.Load())
	r.set("peer_check", r.pl.peerCheck)
	r.set("proof_rejected", r.sess.rejected.Load())
	r.set("proof_accepted", r.sess.handled.Load())
	r.endedAt = time.Now()
	return r, nil
}

func firstLines(s string, n int) string {
	out := ""
	for i, line := range splitLines(s) {
		if i == n {
			break
		}
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		out += line + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
