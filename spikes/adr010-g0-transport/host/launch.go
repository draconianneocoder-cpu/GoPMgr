// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"gopmgr-g0spike/host/g0pb"
)

// proofLabel must match Worker.PROOF_LABEL.
const proofLabel = "gopmgr-g0spike/connect/v1"

// captureCap bounds what the host keeps of a worker's stdout or stderr
// (TPT-021). The rest is counted and dropped, never persisted.
const captureCap = 64 << 10

// cappedBuffer keeps the first captureCap bytes and counts the rest.
type cappedBuffer struct {
	mu      sync.Mutex
	buf     []byte
	dropped int64
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	room := captureCap - len(c.buf)
	if room > 0 {
		n := min(room, len(p))
		c.buf = append(c.buf, p[:n]...)
		c.dropped += int64(len(p) - n)
	} else {
		c.dropped += int64(len(p))
	}
	return len(p), nil
}

func (c *cappedBuffer) snapshot() (string, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.buf), c.dropped
}

// launchSpec says how to start one worker process for one job (D-06).
type launchSpec struct {
	java, jar  string
	transport  string
	endpoint   string
	credential []byte // what the worker is given; may differ from the host's for negative tests
	expires    time.Time
	bootstrap  []byte // when set, written instead of a well-formed bootstrap
	scratch    string
	onStarted  func(pid int) // runs after the process starts, before the bootstrap
}

// worker is a running child process.
type worker struct {
	cmd      *exec.Cmd
	pid      int
	stdout   *cappedBuffer
	stderr   *cappedBuffer
	exited   chan struct{}
	exitErr  error
	started  time.Time
	exitedAt time.Time
}

// stillRunning is what exitCode returns when the worker has not exited.
const stillRunning = -1000

func newCredential() ([]byte, error) {
	c := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, c); err != nil {
		return nil, fmt.Errorf("read credential entropy: %w", err)
	}
	return c, nil
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("read id entropy: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// expectedProof is the HMAC the host accepts for this launch.
func expectedProof(credential []byte, operationID, workerID string) string {
	m := hmac.New(sha256.New, credential)
	m.Write([]byte(proofLabel))
	m.Write([]byte{0})
	m.Write([]byte(operationID))
	m.Write([]byte{0})
	m.Write([]byte(workerID))
	return hex.EncodeToString(m.Sum(nil))
}

// encodeBootstrap returns the uvarint-length-prefixed bootstrap (INT-02).
func encodeBootstrap(b *g0pb.Bootstrap) ([]byte, error) {
	body, err := proto.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("marshal bootstrap: %w", err)
	}
	return append(binary.AppendUvarint(nil, uint64(len(body))), body...), nil
}

// start launches the worker with a fixed argument list, a clean
// environment, a private working directory and only stdio inherited.
func start(spec launchSpec, operationID, workerID string) (*worker, error) {
	cmd := exec.Command(spec.java,
		"-Xmx128m",
		"-XX:+UseSerialGC",
		"-XX:+DisableAttachMechanism",
		"-jar", spec.jar)
	cmd.Env = cleanEnv()
	cmd.Dir = spec.scratch
	platformProcAttr(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	w := &worker{cmd: cmd, stdout: &cappedBuffer{}, stderr: &cappedBuffer{}, exited: make(chan struct{})}
	cmd.Stdout = w.stdout
	cmd.Stderr = w.stderr
	w.started = time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start worker: %w", err)
	}
	w.pid = cmd.Process.Pid
	if spec.onStarted != nil {
		spec.onStarted(w.pid)
	}
	go func() {
		w.exitErr = cmd.Wait()
		w.exitedAt = time.Now()
		close(w.exited)
	}()

	payload := spec.bootstrap
	if payload == nil {
		payload, err = encodeBootstrap(&g0pb.Bootstrap{
			Version:       1,
			OperationId:   operationID,
			WorkerId:      workerID,
			Transport:     spec.transport,
			Endpoint:      spec.endpoint,
			Credential:    spec.credential,
			ExpiresUnixMs: spec.expires.UnixMilli(),
		})
		if err != nil {
			w.kill()
			return nil, err
		}
	}
	if _, err := stdin.Write(payload); err != nil && !errors.Is(err, os.ErrClosed) {
		w.kill()
		return nil, fmt.Errorf("write bootstrap: %w", err)
	}
	if err := stdin.Close(); err != nil {
		w.kill()
		return nil, fmt.Errorf("close bootstrap pipe: %w", err)
	}
	return w, nil
}

// exitCode waits up to d for the worker to exit and returns its code
// (-1 if a signal ended it), or stillRunning.
func (w *worker) exitCode(d time.Duration) int {
	select {
	case <-w.exited:
	case <-time.After(d):
		return stillRunning
	}
	var ee *exec.ExitError
	if errors.As(w.exitErr, &ee) {
		return ee.ExitCode()
	}
	if w.exitErr != nil {
		return -2
	}
	return 0
}

// kill ends the worker's process tree and reaps it.
func (w *worker) kill() {
	killTree(w.cmd)
	<-w.exited
}

func (w *worker) alive() bool {
	select {
	case <-w.exited:
		return false
	default:
		return true
	}
}
