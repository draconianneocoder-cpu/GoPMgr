// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

// Package ipchost is the ADR-012 DF0-C spike host. For each job it opens a
// private Unix socket, launches one Rust DataFusion worker with a one-time
// credential on stdin, admits only that worker's connection, and drives
// one bidirectional gRPC stream. It is evidence code, not the production
// supervisor (ADR-010 G1).
package ipchost

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime/metrics"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

const (
	// DefaultMaxFrame is the per-message cap in both directions (DF0-C 4).
	DefaultMaxFrame = 4 << 20
	defaultChunk    = 1 << 20
	defaultTimeout  = 2 * time.Minute
	// finishGrace is how long a worker has to exit after Finish.
	finishGrace = 5 * time.Second
	// diagCap bounds the worker's captured stdout and stderr.
	diagCap = 16 << 10
)

// Job is one worker launch.
type Job struct {
	Name    string
	Worker  string
	Execute *df0pb.Execute
	// Source is sent as SourceChunks after Execute.
	Source      []byte
	ChunkBytes  int
	MaxFrame    int
	Timeout     time.Duration
	CancelAfter int // send Cancel after this many batches; 0: never
	// StallSourceAfter stops the upload after this many chunks, never
	// sending the last one: a host that cancels mid-upload (fault test).
	StallSourceAfter int
	SlowPerBatch     time.Duration // host pause per batch, to apply backpressure
	// Verify checks one batch's Arrow IPC bytes and returns its row count.
	Verify func([]byte) (int, error)

	// Negative tests.
	CorruptProof bool // the bootstrap carries a credential the host does not expect
	Expired      bool // the bootstrap has already expired
	Intruder     bool // another process (the host itself) dials the endpoint first
	// OnSpawn, when set, is called with the worker's PID and this job's
	// private endpoint directory after launch.
	OnSpawn func(pid int, endpointDir string)
}

// Result is what the host observed. Durations are nanoseconds; -1 means
// not measured.
type Result struct {
	Name     string `json:"name"`
	Outcome  string `json:"outcome"`
	Detail   string `json:"detail,omitempty"`
	Batches  uint64 `json:"batches"`
	Rows     uint64 `json:"rows"`
	IPCBytes int64  `json:"ipc_bytes"`
	MaxBatch int    `json:"max_batch_bytes"`

	DigestMatch *bool  `json:"digest_match,omitempty"`
	SourceMatch *bool  `json:"source_match,omitempty"`
	ResultJSON  string `json:"result_json,omitempty"`
	FailedCode  string `json:"failed_code,omitempty"`

	WorkerPID      int    `json:"-"`
	WorkerExit     string `json:"worker_exit"`
	Killed         bool   `json:"killed"`
	Engine         string `json:"engine,omitempty"`
	WorkerEnvNames int    `json:"worker_env_names"`
	SendWaits      uint64 `json:"worker_send_waits"`
	WorkerPeakRSS  int64  `json:"worker_peak_rss_bytes"`
	HostHeapPeak   uint64 `json:"host_heap_peak_bytes"`

	WorkerSockets  *Sockets `json:"worker_sockets,omitempty"`
	HostSockets    *Sockets `json:"host_sockets,omitempty"`
	InspectError   string   `json:"inspect_error,omitempty"`
	SecretsVisible *bool    `json:"secrets_in_argv_or_env,omitempty"`

	DirMode         string   `json:"endpoint_dir_mode"`
	SocketMode      string   `json:"endpoint_socket_mode"`
	EndpointRemoved bool     `json:"endpoint_removed"`
	ListenerRefused int64    `json:"listener_refused"`
	SessionRefused  int64    `json:"session_refused"`
	Refusals        []string `json:"refusals,omitempty"`
	IntruderData    *bool    `json:"intruder_received_data,omitempty"`

	SpawnToHelloNS   int64 `json:"spawn_to_hello_ns"`
	CancelToFailedNS int64 `json:"cancel_to_failed_ns"`
	CancelToExitNS   int64 `json:"cancel_to_exit_ns"`
	WallNS           int64 `json:"wall_ns"`

	WorkerDiag string `json:"-"`
}

// EncodeBootstrap frames a bootstrap as a uvarint length and the bytes.
func EncodeBootstrap(b *df0pb.Bootstrap) ([]byte, error) {
	body, err := proto.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("marshal bootstrap: %w", err)
	}
	out := binary.AppendUvarint(nil, uint64(len(body)))
	return append(out, body...), nil
}

// capWriter keeps the first n bytes written to it.
type capWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
	n   int
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if room := w.n - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (w *capWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("random: %w", err)
	}
	return b, nil
}

func heapBytes() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return s[0].Value.Uint64()
}

func boolp(b bool) *bool { return &b }

type recvItem struct {
	ev  *df0pb.WorkerEvent
	err error
	at  time.Time
}

// NewResult returns a Result with every measurement marked not measured.
func NewResult(name string) Result {
	return Result{Name: name, SpawnToHelloNS: -1, CancelToFailedNS: -1, CancelToExitNS: -1, WorkerPeakRSS: -1, WallNS: -1}
}

// Run launches the worker and drives one job. It always cleans up: the
// worker is reaped and the endpoint removed before it returns.
func Run(job Job) (res Result) {
	res = NewResult(job.Name)
	if job.MaxFrame == 0 {
		job.MaxFrame = DefaultMaxFrame
	}
	if job.ChunkBytes == 0 {
		job.ChunkBytes = defaultChunk
	}
	if job.Timeout == 0 {
		job.Timeout = defaultTimeout
	}
	start := time.Now()
	deadline := start.Add(job.Timeout)
	defer func() { res.WallNS = time.Since(start).Nanoseconds() }()

	ep, err := Listen()
	if err != nil {
		res.Outcome, res.Detail = "setup_failed", err.Error()
		return res
	}
	if fi, err := os.Lstat(ep.Dir); err == nil {
		res.DirMode = fi.Mode().Perm().String()
	}
	if fi, err := os.Lstat(ep.Path); err == nil {
		res.SocketMode = fi.Mode().Perm().String()
	}
	defer func() {
		_ = ep.Close()
		_, err := os.Lstat(ep.Dir)
		res.EndpointRemoved = errors.Is(err, os.ErrNotExist)
	}()

	cred, err := randomBytes(32)
	if err != nil {
		res.Outcome, res.Detail = "setup_failed", err.Error()
		return res
	}
	proof := []byte(hex.EncodeToString(cred))
	sess := newSession(proof, deadline)
	pl := newPeerListener(ep.ln, deadline)
	srv := newServer(sess, job.MaxFrame)
	served := make(chan struct{})
	go func() {
		_ = srv.Serve(pl)
		close(served)
	}()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(sess.done) }) }
	defer func() {
		release()
		srv.Stop()
		<-served
		res.ListenerRefused = pl.refused.Load()
		res.SessionRefused = sess.refused.Load()
		res.Refusals = append(pl.refusals(), sess.refusals()...)
	}()

	var intruder net.Conn
	if job.Intruder {
		if intruder, err = net.Dial("unix", ep.Path); err != nil {
			res.Outcome, res.Detail = "setup_failed", "intruder dial: "+err.Error()
			return res
		}
		defer intruder.Close()
	}

	bootCred := cred
	if job.CorruptProof {
		if bootCred, err = randomBytes(32); err != nil {
			res.Outcome, res.Detail = "setup_failed", err.Error()
			return res
		}
	}
	expires := deadline
	if job.Expired {
		expires = time.Now().Add(-time.Second)
	}
	framed, err := EncodeBootstrap(&df0pb.Bootstrap{
		Version: 1, OperationId: job.Name, Endpoint: ep.Path, Credential: bootCred,
		ExpiresUnixMs: expires.UnixMilli(), MaxFrameBytes: uint32(job.MaxFrame),
	})
	if err != nil {
		res.Outcome, res.Detail = "setup_failed", err.Error()
		return res
	}

	diag := &capWriter{n: diagCap}
	cmd := exec.Command(job.Worker)
	cmd.Env = []string{} // the worker inherits nothing (DF-036)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = diag, diag
	cmd.SysProcAttr = sysProcAttr()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		res.Outcome, res.Detail = "setup_failed", err.Error()
		return res
	}
	spawned := time.Now()
	if err := cmd.Start(); err != nil {
		res.Outcome, res.Detail = "setup_failed", "start worker: "+err.Error()
		return res
	}
	pid := cmd.Process.Pid
	res.WorkerPID = pid
	pl.setExpected(pid)
	if job.OnSpawn != nil {
		job.OnSpawn(pid, ep.Dir)
	}
	_, werr := stdin.Write(framed)
	cerr := stdin.Close()
	exited := make(chan struct{})
	var exitedAt time.Time
	go func() {
		_ = cmd.Wait()
		exitedAt = time.Now()
		close(exited)
	}()
	kill := func() {
		res.Killed = true
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	defer func() {
		select {
		case <-exited:
		default:
			kill()
			<-exited
		}
		res.WorkerExit = exitText(cmd.ProcessState)
		res.WorkerDiag = diag.String()
	}()
	if werr != nil || cerr != nil {
		res.Outcome, res.Detail = "setup_failed", fmt.Sprintf("bootstrap write: %v %v", werr, cerr)
		return res
	}
	if intruder != nil {
		// The listener must refuse the intruder: it reads EOF, never data.
		_ = intruder.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = intruder.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"))
		n, _ := intruder.Read(make([]byte, 64))
		res.IntruderData = boolp(n > 0)
	}

	timeout := time.NewTimer(time.Until(deadline))
	defer timeout.Stop()
	var stream df0pb.AnalyticsSession_ConnectServer
	select {
	case stream = <-sess.streams:
	case <-exited:
		res.Outcome = "worker_exit_before_stream"
		return res
	case <-timeout.C:
		res.Outcome = "timeout_before_stream"
		return res
	}

	// One goroutine reads and one writes: a gRPC stream allows one of each.
	// stop ends both, and every producer, when Run returns.
	stop := make(chan struct{})
	defer close(stop)
	recv := make(chan recvItem, 4)
	go func() {
		for {
			ev, err := stream.Recv()
			select {
			case recv <- recvItem{ev: ev, err: err, at: time.Now()}:
			case <-stop:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	sendQ := make(chan *df0pb.HostCommand, 16)
	sendErr := make(chan error, 1)
	go func() {
		var seq uint64
		for {
			var cmd *df0pb.HostCommand
			select {
			case cmd = <-sendQ:
			case <-stop:
				return
			}
			seq++
			cmd.Sequence = seq
			if err := stream.Send(cmd); err != nil {
				sendErr <- err
				return
			}
		}
	}()
	// send queues a command; it gives up when Run ends or the deadline
	// passes, so a frozen worker cannot block the host.
	send := func(c *df0pb.HostCommand) bool {
		select {
		case sendQ <- c:
			return true
		case <-stop:
			return false
		case <-time.After(time.Until(deadline)):
			return false
		}
	}

	var (
		want     uint64 = 1
		sum             = sha256.New()
		cancelAt time.Time
		peak     uint64
	)
	fail := func(outcome, detail string) {
		res.Outcome, res.Detail = outcome, detail
		kill() // fail closed: a misbehaving worker is not trusted to stop
	}
	for res.Outcome == "" {
		var it recvItem
		select {
		case it = <-recv:
		case err := <-sendErr:
			fail("send_failed", err.Error())
			continue
		case <-timeout.C:
			fail("timeout", fmt.Sprintf("no completion within %v", job.Timeout))
			continue
		}
		if it.err != nil {
			if status.Code(it.err) == codes.ResourceExhausted {
				fail("rejected_frame", it.err.Error())
				continue
			}
			select {
			case <-exited:
			case <-time.After(time.Second):
			}
			res.Outcome, res.Detail = "stream_ended", it.err.Error()
			continue
		}
		ev := it.ev
		if ev.GetSequence() != want {
			kind := "sequence_gap"
			if ev.GetSequence() < want {
				kind = "sequence_repeat"
			}
			fail("rejected_"+kind, fmt.Sprintf("worker sequence %d where %d was due", ev.GetSequence(), want))
			continue
		}
		want++
		switch e := ev.GetEvent().(type) {
		case *df0pb.WorkerEvent_Hello:
			if want != 2 {
				fail("rejected_protocol", "second Hello")
				continue
			}
			res.SpawnToHelloNS = it.at.Sub(spawned).Nanoseconds()
			res.Engine = e.Hello.GetEngine()
			res.WorkerEnvNames = len(e.Hello.GetEnvNames())
			if e.Hello.GetProtocolMajor() != 1 || e.Hello.GetPid() != int64(pid) {
				fail("rejected_protocol", fmt.Sprintf("protocol %d, pid %d", e.Hello.GetProtocolMajor(), e.Hello.GetPid()))
				continue
			}
			if s, err := InspectSockets(pid); err == nil {
				res.WorkerSockets = &s
			} else {
				res.InspectError = err.Error()
			}
			if s, err := InspectSockets(os.Getpid()); err == nil {
				res.HostSockets = &s
			}
			if v, err := SecretsVisible(pid, cred, proof); err == nil {
				res.SecretsVisible = &v
			}
			if !send(&df0pb.HostCommand{Command: &df0pb.HostCommand_Execute{Execute: job.Execute}}) {
				fail("send_failed", "could not queue Execute")
				continue
			}
			if job.Source != nil {
				go func() {
					src := job.Source
					for sent := 0; ; sent++ {
						if job.StallSourceAfter > 0 && sent == job.StallSourceAfter {
							return
						}
						n := min(job.ChunkBytes, len(src))
						last := n == len(src)
						if !send(&df0pb.HostCommand{Command: &df0pb.HostCommand_Source{Source: &df0pb.SourceChunk{Data: src[:n], Last: last}}}) || last {
							return
						}
						src = src[n:]
					}
				}()
			}
		case *df0pb.WorkerEvent_Batch:
			if want == 2 {
				fail("rejected_protocol", "batch before Hello")
				continue
			}
			b := e.Batch.GetArrowIpc()
			res.IPCBytes += int64(len(b))
			res.MaxBatch = max(res.MaxBatch, len(b))
			if job.Verify != nil {
				n, err := job.Verify(b)
				if err != nil {
					fail("rejected_batch", err.Error())
					continue
				}
				if uint64(n) != e.Batch.GetRows() {
					fail("rejected_batch", fmt.Sprintf("batch says %d rows, holds %d", e.Batch.GetRows(), n))
					continue
				}
			}
			sum.Write(b)
			res.Batches++
			res.Rows += e.Batch.GetRows()
			if res.Batches%16 == 0 {
				peak = max(peak, heapBytes())
			}
			if job.SlowPerBatch > 0 {
				time.Sleep(job.SlowPerBatch)
			}
			if job.CancelAfter > 0 && res.Batches == uint64(job.CancelAfter) && cancelAt.IsZero() {
				cancelAt = time.Now()
				if !send(&df0pb.HostCommand{Command: &df0pb.HostCommand_Cancel{Cancel: &df0pb.Cancel{}}}) {
					fail("send_failed", "could not queue Cancel")
				}
			}
		case *df0pb.WorkerEvent_Completed:
			c := e.Completed
			res.SendWaits = c.GetSendWaits()
			res.WorkerPeakRSS = int64(c.GetPeakRssBytes())
			res.ResultJSON = c.GetResultJson()
			res.DigestMatch = boolp(bytes.Equal(c.GetBatchesSha256(), sum.Sum(nil)) || c.GetBatches() == 0 && len(c.GetBatchesSha256()) == 0)
			if job.Source != nil {
				srcSum := sha256.Sum256(job.Source)
				res.SourceMatch = boolp(bytes.Equal(c.GetSourceSha256(), srcSum[:]) && c.GetSourceBytes() == uint64(len(job.Source)))
			}
			switch {
			case c.GetBatches() != res.Batches || (c.GetBatches() > 0 && c.GetRows() != res.Rows):
				fail("rejected_counts", fmt.Sprintf("worker says %d batches %d rows, host saw %d and %d", c.GetBatches(), c.GetRows(), res.Batches, res.Rows))
			case !*res.DigestMatch:
				fail("rejected_digest", "batch digest differs from the bytes received")
			case res.SourceMatch != nil && !*res.SourceMatch:
				fail("rejected_digest", "source digest differs from the bytes sent")
			default:
				res.Outcome = "completed"
			}
		case *df0pb.WorkerEvent_Failed:
			res.Outcome = "failed"
			res.FailedCode = e.Failed.GetCode()
			res.Detail = e.Failed.GetMessage()
			if !cancelAt.IsZero() {
				res.CancelToFailedNS = it.at.Sub(cancelAt).Nanoseconds()
			}
		default:
			fail("rejected_protocol", "empty event")
		}
	}
	res.HostHeapPeak = max(peak, heapBytes())
	if res.Outcome == "completed" || res.Outcome == "failed" {
		res.WorkerPeakRSS = max(res.WorkerPeakRSS, PeakRSS(pid))
		send(&df0pb.HostCommand{Command: &df0pb.HostCommand_Finish{Finish: &df0pb.Finish{}}})
		select {
		case <-exited:
		case <-time.After(finishGrace):
			kill()
			<-exited
		}
	}
	release()
	select {
	case <-exited:
	case <-time.After(finishGrace):
		kill()
		<-exited
	}
	if !cancelAt.IsZero() {
		res.CancelToExitNS = exitedAt.Sub(cancelAt).Nanoseconds()
	}
	return res
}

func exitText(ps *os.ProcessState) string {
	if ps == nil {
		return "unknown"
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return "signal " + ws.Signal().String()
	}
	return fmt.Sprintf("exit %d", ps.ExitCode())
}
