// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gopmgr-g0spike/host/g0pb"
)

// Worker exit codes (Worker.java). They avoid the JVM's own codes: 1 for
// an uncaught error, 3 for -XX:+ExitOnOutOfMemoryError, 134 for abort.
const (
	exitOK        = 0
	exitJVMOOM    = 3
	exitBootstrap = 20
	exitCancelled = 21
	exitRejected  = 22
	exitError     = 23
	exitExpired   = 24
	exitCrash     = 70
)

// replaySentinel is the record index the worker uses to report its replay.
const replaySentinel = ^uint64(0)

// transportTestIDs maps a transport to the G0 test it provides evidence for.
func transportTestIDs(transport string) []string {
	switch {
	case runtimeGOOS == "linux":
		return []string{"TPT-001"}
	case runtimeGOOS == "darwin":
		return []string{"TPT-002"}
	case transport == "uds-nio":
		return []string{"TPT-004"}
	default:
		return []string{"TPT-003"}
	}
}

func payload(index uint64, size uint32) []byte {
	p := make([]byte, size)
	for j := range p {
		p[j] = byte((index*31 + uint64(j)) & 0xff)
	}
	return p
}

// send writes one command; recvUntil reads events until fn says stop.
func send(stream g0pb.WorkerSession_ConnectServer, seq *uint64, c *g0pb.HostCommand) error {
	*seq++
	c.Sequence = *seq
	return stream.Send(c)
}

func finish(stream g0pb.WorkerSession_ConnectServer, seq *uint64) error {
	if err := send(stream, seq, &g0pb.HostCommand{Command: &g0pb.HostCommand_Finish{Finish: &g0pb.Finish{}}}); err != nil {
		return fmt.Errorf("send finish: %w", err)
	}
	for {
		if _, err := stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("await close: %w", err)
		}
	}
}

func execute(stream g0pb.WorkerSession_ConnectServer, seq *uint64, scenario string, records uint64, size uint32) error {
	return send(stream, seq, &g0pb.HostCommand{Command: &g0pb.HostCommand_Execute{Execute: &g0pb.Execute{
		Scenario: scenario, Records: records, RecordBytes: size}}})
}

func lastSource(stream g0pb.WorkerSession_ConnectServer, seq *uint64) error {
	return send(stream, seq, &g0pb.HostCommand{Command: &g0pb.HostCommand_Source{Source: &g0pb.SourceBlock{Last: true}}})
}

// commonChecks are the conditions every successful session must meet.
func commonChecks(r *jobRun, ev *evidence) {
	if r.metrics["credential_leaked"] == true {
		ev.fail("the credential appeared in the worker's command line or environment")
	}
	if n, ok := r.metrics["tcp_sockets"].(int); ok && n != 0 {
		ev.fail(fmt.Sprintf("worker owns %d TCP sockets; no TCP is allowed", n))
	}
	if r.hello != nil {
		if slices.Contains(r.hello.EnvNames, "JAVA_TOOL_OPTIONS") || slices.Contains(r.hello.EnvNames, "_JAVA_OPTIONS") || slices.Contains(r.hello.EnvNames, "CLASSPATH") {
			ev.fail("a dangerous environment variable reached the worker")
		}
		ev.Metrics["worker_env_names"] = append([]string{}, r.hello.EnvNames...)
		ev.Metrics["java_version"] = r.hello.JavaVersion
	}
	if s, _ := r.metrics["stderr_head"].(string); strings.Contains(s, "Picked up JAVA_TOOL_OPTIONS") {
		ev.fail("the JVM picked up JAVA_TOOL_OPTIONS")
	}
}

func scenarioHandshake(cfg config, transport string) *evidence {
	ev := newEvidence("handshake", transport, append(transportTestIDs(transport), "TPT-005", "TPT-007", "TPT-017", "TPT-020", "TPT-022"))
	var rtts []time.Duration
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			for i := uint64(1); i <= 200; i++ {
				t0 := time.Now()
				if err := send(stream, &seq, &g0pb.HostCommand{Command: &g0pb.HostCommand_Ping{Ping: &g0pb.Ping{Seq: i}}}); err != nil {
					return err
				}
				evt, err := stream.Recv()
				if err != nil {
					return err
				}
				if evt.GetPong().GetSeq() != i {
					return fmt.Errorf("pong %d for ping %d", evt.GetPong().GetSeq(), i)
				}
				rtts = append(rtts, time.Since(t0))
			}
			r.sampleRSS()
			return finish(stream, &seq)
		}})
	ev.absorb(r, err)
	if ev.Status == statusPass && r.exitCode != exitOK {
		ev.fail(fmt.Sprintf("worker exit code %d, want 0", r.exitCode))
	}
	if r != nil && r.connected {
		ev.Metrics["cold_start_ms"] = r.coldStart.Milliseconds()
		ev.Metrics["ping_rtt_us"] = summarize(rtts, time.Microsecond)
		commonChecks(r, ev)
	}
	return ev
}

// duplexOutcome is what one full-duplex session measured.
type duplexOutcome struct {
	elapsed  time.Duration
	mismatch []string
}

// duplexDrive sends sourceBytes to the worker while it streams `records`
// records back, at the same time, and checks SHA-256 digests both ways.
func duplexDrive(records uint64, size uint32, sourceBytes int, out *duplexOutcome) func(*jobRun, g0pb.WorkerSession_ConnectServer) error {
	const block = 64 << 10
	return func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
		var seq uint64
		t0 := time.Now()
		if err := execute(stream, &seq, "duplex", records, size); err != nil {
			return err
		}
		srcSum := sha256.New()
		sendErr := make(chan error, 1)
		go func() {
			// Sends source blocks while the main goroutine receives records:
			// both directions run at once (full duplex).
			buf := make([]byte, block)
			for sent := 0; sent < sourceBytes; sent += block {
				for i := range buf {
					buf[i] = byte((sent + i) * 7)
				}
				srcSum.Write(buf)
				chunk := append([]byte(nil), buf...)
				if err := send(stream, &seq, &g0pb.HostCommand{Command: &g0pb.HostCommand_Source{Source: &g0pb.SourceBlock{
					Chunk: chunk, Last: sent+block >= sourceBytes}}}); err != nil {
					sendErr <- err
					return
				}
			}
			sendErr <- nil
		}()
		recSum := sha256.New()
		var got uint64
		var done *g0pb.Completed
		for done == nil {
			evt, err := stream.Recv()
			if err != nil {
				return fmt.Errorf("receive: %w", err)
			}
			if rec := evt.GetRecord(); rec != nil {
				var idx [8]byte
				binary.BigEndian.PutUint64(idx[:], rec.Index)
				recSum.Write(idx[:])
				recSum.Write(rec.Payload)
				if !bytes.Equal(rec.Payload, payload(rec.Index, size)) {
					out.mismatch = append(out.mismatch, fmt.Sprintf("record %d payload differs", rec.Index))
				}
				got++
			}
			done = evt.GetCompleted()
		}
		if err := <-sendErr; err != nil {
			return fmt.Errorf("send source: %w", err)
		}
		out.elapsed = time.Since(t0)
		if got != records || done.RecordsSent != records {
			out.mismatch = append(out.mismatch, fmt.Sprintf("records host %d worker %d want %d", got, done.RecordsSent, records))
		}
		if done.SourceBytes != uint64(sourceBytes) || !bytes.Equal(done.SourceSha256, srcSum.Sum(nil)) {
			out.mismatch = append(out.mismatch, "source digest or size differs")
		}
		if !bytes.Equal(done.RecordsSha256, recSum.Sum(nil)) {
			out.mismatch = append(out.mismatch, "records digest differs")
		}
		r.set("worker_max_heap_used_bytes", done.MaxHeapUsedBytes)
		r.sampleRSS()
		return finish(stream, &seq)
	}
}

func scenarioDuplex(cfg config, transport string) *evidence {
	ev := newEvidence("duplex", transport, append(transportTestIDs(transport), "TPT-012"))
	const records, size = 100_000, 256
	const sourceBytes = 64 << 20
	var out duplexOutcome
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 180 * time.Second,
		drive: duplexDrive(records, size, sourceBytes, &out)})
	ev.absorb(r, err)
	for _, m := range out.mismatch {
		ev.fail(m)
	}
	if r != nil && r.connected && out.elapsed > 0 {
		total := float64(sourceBytes + records*size)
		ev.Metrics["elapsed_ms"] = out.elapsed.Milliseconds()
		ev.Metrics["host_to_worker_bytes"] = sourceBytes
		ev.Metrics["worker_to_host_bytes"] = records * size
		ev.Metrics["combined_mib_per_s"] = round2(total / (1 << 20) / out.elapsed.Seconds())
		commonChecks(r, ev)
	}
	return ev
}

func scenarioBackpressure(cfg config, transport string) *evidence {
	ev := newEvidence("backpressure", transport, append(transportTestIDs(transport), "TPT-012"))
	const records, size = 1_000_000, 128
	var completed *g0pb.Completed
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 240 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "flood", records, size); err != nil {
				return err
			}
			if err := lastSource(stream, &seq); err != nil {
				return err
			}
			// A slow host: read nothing for 3 s while the worker floods.
			time.Sleep(3 * time.Second)
			r.sampleRSS()
			for completed == nil {
				evt, err := stream.Recv()
				if err != nil {
					return fmt.Errorf("receive: %w", err)
				}
				completed = evt.GetCompleted()
			}
			return finish(stream, &seq)
		}})
	ev.absorb(r, err)
	if completed != nil {
		ev.Metrics["not_ready_waits"] = completed.NotReadyWaits
		ev.Metrics["worker_max_heap_used_bytes"] = completed.MaxHeapUsedBytes
		if completed.NotReadyWaits == 0 {
			ev.fail("the worker never waited for flow control while the host was not reading")
		}
		if completed.MaxHeapUsedBytes > 100<<20 {
			ev.fail("worker heap grew past 100 MiB while the host was not reading")
		}
		if completed.RecordsSent != records {
			ev.fail("not every record arrived")
		}
	} else if ev.Status == statusPass {
		ev.fail("no completion")
	}
	if r != nil && r.connected {
		commonChecks(r, ev)
	}
	return ev
}

// scenarioCancel runs the host-cancel case `runs` times. With a stall, the
// host stops reading for that long first, so the worker's buffers and the
// pipe are full when the cancel arrives (cancel under backpressure).
func scenarioCancel(cfg config, transport string, runs int, name string, stall time.Duration) *evidence {
	ev := newEvidence(name, transport, append(transportTestIDs(transport), "TPT-012", "TPT-013"))
	var latencies, starts, toEOF, eofToExit []time.Duration
	phases := map[string][]time.Duration{}
	nativeRuns := 0
	var example, nativeExample string
	peakPending := int64(0)
	codesSeen := map[int]int{}
	for i := 0; i < runs; i++ {
		var t0 time.Time
		r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
			drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
				var seq uint64
				if err := execute(stream, &seq, "flood", 50_000_000, 64); err != nil {
					return err
				}
				for n := 0; n < 1000; n++ {
					if _, err := stream.Recv(); err != nil {
						return err
					}
				}
				if stall > 0 {
					time.Sleep(stall)
				}
				t0 = time.Now()
				return status.Error(codes.Canceled, "host cancelled the job")
			}})
		if err != nil || r == nil || !r.connected {
			ev.absorb(r, err)
			return ev
		}
		<-r.w.exited
		latencies = append(latencies, r.w.exitedAt.Sub(t0))
		starts = append(starts, r.coldStart)
		if r.pl.conn != nil {
			if eof := r.pl.conn.closedAt(); !eof.IsZero() {
				toEOF = append(toEOF, eof.Sub(t0))
				eofToExit = append(eofToExit, r.w.exitedAt.Sub(eof))
			}
		}
		d := parseWorkerDiag(r.stderr)
		if d.timingLine != "" && example == "" {
			example = d.timingLine
		}
		if base, ok := d.marks["stream_error"]; ok {
			for label, at := range d.marks {
				if at >= base {
					phases[label] = append(phases[label], at-base)
				}
			}
		}
		if len(d.native) > 0 {
			nativeRuns++
			if nativeExample == "" {
				nativeExample = strings.Join(d.native, ";")
			}
		}
		peakPending = max(peakPending, d.stats["bridge_peak_pending_bytes"])
		codesSeen[r.exitCode]++
		if r.exitCode != exitCancelled {
			ev.fail(fmt.Sprintf("run %d: worker exit code %d, want %d", i, r.exitCode, exitCancelled))
		}
		if r.w.alive() {
			ev.fail(fmt.Sprintf("run %d: worker still running after cancel", i))
		}
		if n, _ := r.metrics["tree_left_after_exit"].(int); n > 0 {
			ev.fail(fmt.Sprintf("run %d: %d processes left in the worker's tree after exit", i, n))
		}
	}
	ev.Metrics["runs"] = runs
	if stall > 0 {
		ev.Metrics["host_stall_ms"] = stall.Milliseconds()
	}
	ev.Metrics["cancel_to_exit_ms"] = summarize(latencies, time.Millisecond)
	// Diagnostics: where the time between the host's cancel and the
	// worker's exit goes. Host clock: cancel to the worker closing its end
	// of the connection, then to the process exit. Worker clock: each
	// shutdown step, relative to the worker seeing the stream end.
	ev.Metrics["host_cancel_to_conn_eof_ms"] = summarize(toEOF, time.Millisecond)
	ev.Metrics["host_conn_eof_to_exit_ms"] = summarize(eofToExit, time.Millisecond)
	worker := map[string]any{}
	for label, ds := range phases {
		worker[label] = summarize(ds, time.Millisecond)
	}
	ev.Metrics["worker_phase_ms_after_stream_error"] = worker
	if example != "" {
		ev.Metrics["worker_timing_example"] = example
	}
	// Threads still in a native wait when the worker called System.exit;
	// HotSpot waits for them before the process ends.
	ev.Metrics["runs_with_native_threads_at_exit"] = nativeRuns
	if nativeExample != "" {
		ev.Metrics["native_threads_at_exit_example"] = nativeExample
	}
	if peakPending > 0 {
		ev.Metrics["bridge_peak_pending_bytes"] = peakPending
	}
	ev.Metrics["cold_start_ms"] = summarize(starts, time.Millisecond)
	ev.Metrics["exit_codes"] = codesSeen
	return ev
}

// workerDiag is what the worker's diagnostic lines report (Timing.java).
type workerDiag struct {
	timingLine string
	marks      map[string]time.Duration // label -> time since the worker started
	stats      map[string]int64
	native     []string // thread@frame in a native wait at exit
}

// parseWorkerDiag reads the g0-timing, g0-stats and g0-native-at-exit
// lines from the worker's stderr. Unknown or malformed parts are skipped.
func parseWorkerDiag(stderr string) workerDiag {
	d := workerDiag{marks: map[string]time.Duration{}, stats: map[string]int64{}}
	for _, line := range splitLines(stderr) {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "g0-timing: "); ok {
			d.timingLine = line
			for k, v := range keyValues(rest) {
				d.marks[k] = time.Duration(v) * time.Microsecond
			}
		} else if rest, ok := strings.CutPrefix(line, "g0-stats: "); ok {
			for k, v := range keyValues(rest) {
				d.stats[k] = v
			}
		} else if rest, ok := strings.CutPrefix(line, "g0-native-at-exit:"); ok {
			for _, t := range strings.Split(strings.TrimSpace(rest), ";") {
				if t != "" {
					d.native = append(d.native, t)
				}
			}
		}
	}
	return d
}

func keyValues(s string) map[string]int64 {
	out := map[string]int64{}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		var n int64
		if _, err := fmt.Sscan(v, &n); err == nil {
			out[k] = n
		}
	}
	return out
}

func scenarioKill(cfg config, transport string) *evidence {
	ev := newEvidence("kill", transport, append(transportTestIDs(transport), "TPT-013", "TPT-014"))
	var detect time.Duration
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "flood", 50_000_000, 64); err != nil {
				return err
			}
			for n := 0; n < 1000; n++ {
				if _, err := stream.Recv(); err != nil {
					return err
				}
			}
			t0 := time.Now()
			r.w.tree.kill()
			for {
				if _, err := stream.Recv(); err != nil {
					detect = time.Since(t0)
					return nil
				}
			}
		}})
	ev.absorb(r, err)
	if r != nil && r.connected {
		ev.Metrics["kill_to_stream_error_ms"] = detect.Milliseconds()
		if r.w.alive() {
			ev.fail("worker still running after kill")
		}
	}
	return ev
}

func scenarioBadCredential(cfg config, transport string) *evidence {
	ev := newEvidence("bad-credential", transport, []string{"TPT-007", "TPT-018"})
	r, err := runJob(cfg, jobOptions{transport: transport, wrongCredential: true, timeout: 60 * time.Second})
	if err != nil {
		ev.fail(err.Error())
		return ev
	}
	ev.Metrics = r.metrics
	ev.Metrics["worker_exit_code"] = r.exitCode
	if r.sess.handled.Load() != 0 {
		ev.fail("a call with a wrong credential reached the handler")
	}
	if r.sess.rejected.Load() == 0 {
		ev.fail("the host never refused the wrong credential")
	}
	if r.exitCode != exitRejected {
		ev.fail(fmt.Sprintf("worker exit code %d, want %d (rejected)", r.exitCode, exitRejected))
	}
	return ev
}

func scenarioReplay(cfg config, transport string) *evidence {
	ev := newEvidence("replay", transport, []string{"TPT-007"})
	var report string
	r, err := runJob(cfg, jobOptions{transport: transport, acceptMany: true, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "reconnect", 0, 0); err != nil {
				return err
			}
			for report == "" {
				evt, err := stream.Recv()
				if err != nil {
					return err
				}
				if rec := evt.GetRecord(); rec != nil && rec.Index == replaySentinel {
					report = string(rec.Payload)
				}
			}
			if err := lastSource(stream, &seq); err != nil {
				return err
			}
			for {
				evt, err := stream.Recv()
				if err != nil {
					return err
				}
				if evt.GetCompleted() != nil {
					break
				}
			}
			return finish(stream, &seq)
		}})
	ev.absorb(r, err)
	if r != nil && r.connected {
		ev.Metrics["worker_second_call"] = report
		if report != "replay:UNAUTHENTICATED" {
			ev.fail("the reused proof was not refused as UNAUTHENTICATED: " + report)
		}
	}
	return ev
}

func scenarioImpersonate(cfg config, transport string) *evidence {
	ev := newEvidence("impersonate", transport, []string{"TPT-018"})
	var stolen string
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		beforeConnect: func(r *jobRun) {
			// The host process plays a local attacker that stole the
			// credential and connects before the real worker does.
			conn, err := grpc.NewClient("passthrough:///g0",
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return dialEndpoint(r.ep.addr, transport)
				}))
			if err != nil {
				stolen = "client: " + err.Error()
				return
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ctx = metadata.AppendToOutgoingContext(ctx, "x-g0-proof", r.sess.proof)
			st, err := g0pb.NewWorkerSessionClient(conn).Connect(ctx)
			if err == nil {
				_, err = st.Recv()
			}
			stolen = status.Code(err).String()
		},
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			return finish(stream, &seq)
		}})
	ev.absorb(r, err)
	if r == nil {
		return ev
	}
	ev.Metrics["impersonator_result"] = stolen
	if r.sess.handled.Load() > 1 {
		ev.fail("an impersonating process reached the handler")
	}
	if pc, _ := r.metrics["peer_check"].(string); strings.HasPrefix(pc, "unavailable") {
		ev.block("the OS gave no peer process ID, so a process holding a stolen credential cannot be told apart: " + pc)
	} else if r.pl.refused.Load() == 0 {
		ev.fail("the listener admitted the impersonator")
	}
	return ev
}

func scenarioNoise(cfg config, transport string) *evidence {
	ev := newEvidence("noise", transport, []string{"TPT-021"})
	r, err := runJob(cfg, jobOptions{transport: transport, timeout: 60 * time.Second,
		drive: func(r *jobRun, stream g0pb.WorkerSession_ConnectServer) error {
			var seq uint64
			if err := execute(stream, &seq, "noise", 1000, 64); err != nil {
				return err
			}
			if err := lastSource(stream, &seq); err != nil {
				return err
			}
			for {
				evt, err := stream.Recv()
				if err != nil {
					return err
				}
				if evt.GetCompleted() != nil {
					return finish(stream, &seq)
				}
			}
		}})
	ev.absorb(r, err)
	if r != nil && r.connected {
		if kept, _ := r.metrics["stdout_kept_bytes"].(int); kept > captureCap {
			ev.fail("stdout capture exceeded its cap")
		}
		if d, _ := r.metrics["stdout_dropped_bytes"].(int64); d == 0 {
			ev.fail("the noisy worker's stdout was never capped")
		}
	}
	return ev
}

func scenarioBootstrap(cfg config, transport string) []*evidence {
	var out []*evidence
	cases := []struct {
		name    string
		raw     []byte
		expires time.Time
		want    int
	}{
		{"bootstrap-oversize", append(binary.AppendUvarint(nil, 5000), make([]byte, 5000)...), time.Time{}, exitBootstrap},
		{"bootstrap-garbage", append(binary.AppendUvarint(nil, 10), []byte("not-proto!")...), time.Time{}, exitBootstrap},
		{"bootstrap-empty", []byte{}, time.Time{}, exitBootstrap},
		{"bootstrap-expired", nil, time.Now().Add(-time.Second), exitExpired},
	}
	for _, c := range cases {
		ev := newEvidence(c.name, transport, []string{"TPT-022"})
		raw := c.raw
		if raw == nil && c.name != "bootstrap-expired" {
			raw = []byte{}
		}
		r, err := runJob(cfg, jobOptions{transport: transport, rawBootstrap: raw, expires: c.expires, timeout: 30 * time.Second})
		if err != nil {
			ev.fail(err.Error())
		} else {
			ev.Metrics = r.metrics
			ev.Metrics["worker_exit_code"] = r.exitCode
			if r.exitCode != c.want {
				ev.fail(fmt.Sprintf("worker exit code %d, want %d", r.exitCode, c.want))
			}
			if r.sess.handled.Load() != 0 || r.pl.admitted.Load() != 0 {
				ev.fail("a worker with a rejected bootstrap still connected")
			}
		}
		out = append(out, ev)
	}
	return out
}

func scenarioHandshakeTimeout(cfg config, transport string) *evidence {
	ev := newEvidence("handshake-timeout", transport, []string{"TPT-014"})
	t0 := time.Now()
	r, err := runJob(cfg, jobOptions{transport: transport, workerEndpoint: missingEndpoint(transport), timeout: 20 * time.Second})
	if err != nil {
		ev.fail(err.Error())
		return ev
	}
	ev.Metrics = r.metrics
	ev.Metrics["worker_exit_code"] = r.exitCode
	ev.Metrics["host_wait_ms"] = time.Since(t0).Milliseconds()
	if r.connected {
		ev.fail("a worker given the wrong endpoint still connected")
	}
	if r.exitCode == exitOK {
		ev.fail("the worker reported success without connecting")
	}
	return ev
}

func scenarioMissingJVM(cfg config, transport string) *evidence {
	ev := newEvidence("missing-jvm", transport, []string{"TPT-014"})
	_, err := runJob(cfg, jobOptions{transport: transport, javaPath: missingJava(), timeout: 10 * time.Second})
	if err == nil {
		ev.fail("launching a missing JVM did not fail")
	} else {
		ev.Metrics["error"] = err.Error()
	}
	return ev
}

func scenarioSquat(transport string) *evidence {
	ids := []string{"TPT-006", "TPT-008"}
	if runtimeGOOS == "windows" {
		ids = []string{"TPT-006", "TPT-009"}
	}
	ev := newEvidence("squat", transport, ids)
	refused, detail := squatEndpoint()
	ev.Metrics["detail"] = detail
	if !refused {
		ev.fail("a pre-placed endpoint was not refused")
	}
	return ev
}

// scenarioSelfCheck proves the leak and TCP detectors find real positives,
// so their "none found" results elsewhere mean something.
func scenarioSelfCheck(transport string) *evidence {
	ev := newEvidence("detector-self-check", transport, []string{"TPT-007", "TPT-020"})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ev.block("cannot open a test TCP listener: " + err.Error())
	} else {
		n, method, cerr := tcpSocketCount(os.Getpid())
		_ = ln.Close()
		ev.Metrics["tcp_detector_method"] = method
		ev.Metrics["tcp_detector_found"] = n
		if cerr != nil || n < 1 {
			ev.fail(fmt.Sprintf("the TCP detector missed a real listener (found %d, err %v)", n, cerr))
		}
	}
	secret, err := randomID()
	if err != nil {
		ev.fail(err.Error())
		return ev
	}
	// The probe re-runs itself as the planted process. A shell would be
	// fragile here: macOS bash replaces itself with the last command,
	// dropping the secret, and Windows cmd hands it to the child.
	self, err := os.Executable()
	if err != nil {
		ev.block("cannot find the probe executable: " + err.Error())
		return ev
	}
	cmd := exec.Command(self, sleeperFlag, secret)
	if err := cmd.Start(); err != nil {
		ev.block("cannot start the sleeper process: " + err.Error())
		return ev
	}
	time.Sleep(500 * time.Millisecond)
	how, leaked := leakCheck(cmd.Process.Pid, [][]byte{[]byte(secret)})
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	ev.Metrics["leak_detector_method"] = how
	ev.Metrics["leak_detector_found"] = leaked
	if !leaked {
		ev.fail("the leak detector missed a secret placed in a command line")
	}
	return ev
}
