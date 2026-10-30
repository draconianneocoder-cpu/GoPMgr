// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package ipchost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

const fakeFlag = "-df0-fake-worker"

// TestMain lets the test binary act as a misbehaving worker, so host
// rejections are tested without the Rust worker.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == fakeFlag {
		os.Exit(fakeWorker(os.Args[2]))
	}
	os.Exit(m.Run())
}

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "df0t-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestListenAtRefusesOccupiedPaths(t *testing.T) {
	dir := shortDir(t)
	tests := []struct {
		name  string
		place func(path string) error
	}{
		{"regular file", func(p string) error { return os.WriteFile(p, nil, 0o600) }},
		{"symlink", func(p string) error { return os.Symlink(filepath.Join(dir, "elsewhere"), p) }},
		{"stale socket", func(p string) error {
			l, err := net.Listen("unix", p)
			if err != nil {
				return err
			}
			l.(*net.UnixListener).SetUnlinkOnClose(false)
			return l.Close()
		}},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, string(rune('a'+i))+".sock")
			if err := tt.place(path); err != nil {
				t.Fatal(err)
			}
			ep, err := ListenAt(path)
			if err == nil {
				_ = ep.Close()
				t.Fatal("listened on an occupied path")
			}
			if !errors.Is(err, ErrEndpointExists) {
				t.Fatalf("err = %v, want ErrEndpointExists", err)
			}
		})
	}
	t.Run("path too long", func(t *testing.T) {
		if _, err := ListenAt("/tmp/" + strings.Repeat("x", 120)); err == nil {
			t.Fatal("accepted an over-long socket path")
		}
	})
}

func TestCheckPrivateDir(t *testing.T) {
	base := shortDir(t)
	open := filepath.Join(base, "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(base, "private")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		dir string
		ok  bool
	}{{private, true}, {open, false}, {link, false}, {filepath.Join(base, "missing"), false}}
	for _, tt := range tests {
		if err := CheckPrivateDir(tt.dir); (err == nil) != tt.ok {
			t.Errorf("CheckPrivateDir(%s) = %v, want ok=%v", filepath.Base(tt.dir), err, tt.ok)
		}
	}
}

func TestListenIsPrivateAndCleansUp(t *testing.T) {
	ep, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{ep.Dir: 0o700, ep.Path: 0o600} {
		fi, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s mode %v, want %v", path, fi.Mode().Perm(), want)
		}
	}
	if err := ep.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(ep.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("endpoint dir left behind: %v", err)
	}
}

func TestEncodeBootstrapFraming(t *testing.T) {
	in := &df0pb.Bootstrap{Version: 1, OperationId: "op", Endpoint: "/tmp/x/w.sock",
		Credential: bytes.Repeat([]byte{9}, 32), ExpiresUnixMs: 42, MaxFrameBytes: 4 << 20}
	b, err := EncodeBootstrap(in)
	if err != nil {
		t.Fatal(err)
	}
	n, k := binary.Uvarint(b)
	if k <= 0 || int(n) != len(b)-k {
		t.Fatalf("length prefix %d (%d bytes) for %d bytes", n, k, len(b))
	}
	var out df0pb.Bootstrap
	if err := proto.Unmarshal(b[k:], &out); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(in, &out) {
		t.Fatalf("round trip: %v != %v", &out, in)
	}
}

// serveSession runs a session's gRPC server on a plain socket, without the
// peer check, so the proof logic is tested alone.
func serveSession(t *testing.T, s *session) string {
	t.Helper()
	path := filepath.Join(shortDir(t), "s.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(s, DefaultMaxFrame)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		close(s.done)
		srv.Stop()
	})
	go func() {
		for range s.streams {
		}
	}()
	return path
}

func connect(t *testing.T, path, proof string) error {
	t.Helper()
	cc, err := grpc.NewClient("unix:"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if proof != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, ProofHeader, proof)
	}
	st, err := df0pb.NewAnalyticsSessionClient(cc).Connect(ctx)
	if err != nil {
		return err
	}
	_ = st.CloseSend()
	_, err = st.Recv()
	if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled || status.Code(err) == codes.DeadlineExceeded {
		return nil // reached the handler, which holds the stream open
	}
	return err
}

func TestSessionProofIsSingleUseAndExpires(t *testing.T) {
	good := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	t.Run("wrong, missing, then right, then reused", func(t *testing.T) {
		s := newSession([]byte(good), time.Now().Add(time.Minute))
		path := serveSession(t, s)
		for _, tc := range []struct {
			proof string
			code  codes.Code
		}{
			{hex.EncodeToString(bytes.Repeat([]byte{2}, 32)), codes.Unauthenticated},
			{"", codes.Unauthenticated},
		} {
			if err := connect(t, path, tc.proof); status.Code(err) != tc.code {
				t.Fatalf("proof %q: err %v, want %v", tc.proof, err, tc.code)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		cc, _ := grpc.NewClient("unix:"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
		defer cc.Close()
		st, err := df0pb.NewAnalyticsSessionClient(cc).Connect(metadata.AppendToOutgoingContext(ctx, ProofHeader, good))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.Recv(); status.Code(err) == codes.Unauthenticated {
			t.Fatalf("right proof refused: %v", err)
		}
		if err := connect(t, path, good); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("reused proof: err %v, want Unauthenticated", err)
		}
		if got := s.refusals(); len(got) != 3 || got[2] != "proof reused" {
			t.Fatalf("refusals %v", got)
		}
	})
	t.Run("expired", func(t *testing.T) {
		s := newSession([]byte(good), time.Now().Add(-time.Second))
		path := serveSession(t, s)
		if err := connect(t, path, good); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expired proof: err %v, want Unauthenticated", err)
		}
	})
}

func TestPeerListenerAdmitsOnlyTheExpectedProcessOnce(t *testing.T) {
	path := filepath.Join(shortDir(t), "p.sock")
	raw, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	pl := newPeerListener(raw, time.Now().Add(10*time.Second))
	t.Cleanup(func() { _ = pl.Close() })
	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := pl.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	dial := func() net.Conn {
		c, err := net.Dial("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	closedByHost := func(c net.Conn) bool {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err := c.Read(make([]byte, 1))
		return errors.Is(err, io.EOF)
	}

	// PID 1 is never this test: the connection is held until the worker
	// is known, then refused.
	early := dial()
	pl.setExpected(1)
	if !closedByHost(early) {
		t.Fatal("connection from the wrong process was not refused")
	}
	pl.setExpected(os.Getpid())
	dial()
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected process not admitted")
	}
	if second := dial(); !closedByHost(second) {
		t.Fatal("second connection was admitted")
	}
	if got := pl.refusals(); len(got) != 2 || got[0] != "peer is not the launched worker" || got[1] != "endpoint already used" {
		t.Fatalf("refusals %v", got)
	}
}

func genIPC(t *testing.T, schema *arrow.Schema, start int64, n int, width int, corrupt bool) []byte {
	t.Helper()
	b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer b.Release()
	for r := range n {
		i := start + int64(r)
		v := i * 7919 % 1000003
		if corrupt && r == n-1 {
			v++
		}
		p := make([]byte, width)
		for k := range p {
			p[k] = byte('a' + (int(i)+k)%26)
		}
		switch schema.NumFields() {
		case 3:
			b.Field(0).(*array.Int64Builder).Append(i)
			b.Field(1).(*array.Int64Builder).Append(v)
			b.Field(2).(*array.StringBuilder).Append(string(p))
		default:
			b.Field(0).(*array.StringBuilder).Append("x")
		}
	}
	rec := b.NewRecordBatch()
	defer rec.Release()
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema))
	if err := w.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGenVerifier(t *testing.T) {
	other := arrow.NewSchema([]arrow.Field{{Name: "i", Type: arrow.BinaryTypes.String}}, nil)
	tests := []struct {
		name    string
		batches func() [][]byte
		wantErr string
	}{
		{"two good batches in order", func() [][]byte {
			return [][]byte{genIPC(t, GenSchema, 0, 5, 3, false), genIPC(t, GenSchema, 5, 5, 3, false)}
		}, ""},
		{"value differs", func() [][]byte { return [][]byte{genIPC(t, GenSchema, 0, 5, 3, true)} }, "v ="},
		{"out of order", func() [][]byte { return [][]byte{genIPC(t, GenSchema, 5, 5, 3, false)} }, "i ="},
		{"wrong width", func() [][]byte { return [][]byte{genIPC(t, GenSchema, 0, 5, 4, false)} }, "payload"},
		{"other schema", func() [][]byte { return [][]byte{genIPC(t, other, 0, 1, 0, false)} }, "schema"},
		{"garbage", func() [][]byte { return [][]byte{[]byte("not arrow at all")} }, "arrow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GenVerifier{Width: 3}
			var err error
			for _, b := range tt.batches() {
				if _, err = g.Check(b); err != nil {
					break
				}
			}
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// fakeWorker is a deliberately wrong worker in Go: it bootstraps and
// connects like the real one, then breaks one protocol rule.
func fakeWorker(mode string) int {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 64<<10))
	if err != nil {
		return 2
	}
	n, k := binary.Uvarint(raw)
	var boot df0pb.Bootstrap
	if k <= 0 || int(n) != len(raw)-k || proto.Unmarshal(raw[k:], &boot) != nil {
		return 2
	}
	cc, err := grpc.NewClient("unix:"+boot.GetEndpoint(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(64<<20)))
	if err != nil {
		return 3
	}
	defer cc.Close()
	ctx := metadata.AppendToOutgoingContext(context.Background(), ProofHeader, hex.EncodeToString(boot.GetCredential()))
	st, err := df0pb.NewAnalyticsSessionClient(cc).Connect(ctx)
	if err != nil {
		return 3
	}
	hello := &df0pb.Hello{ProtocolMajor: 1, Pid: int64(os.Getpid())}
	var first *df0pb.WorkerEvent
	switch mode {
	case "major2":
		hello.ProtocolMajor = 2
	case "wrong_pid":
		hello.Pid = 1
	case "batch_before_hello":
		first = &df0pb.WorkerEvent{Sequence: 1, Event: &df0pb.WorkerEvent_Batch{Batch: &df0pb.Batch{Rows: 1}}}
	}
	if first == nil {
		first = &df0pb.WorkerEvent{Sequence: 1, Event: &df0pb.WorkerEvent_Hello{Hello: hello}}
	}
	if err := st.Send(first); err != nil {
		return 3
	}
	if _, err := st.Recv(); err != nil { // Execute
		return 3
	}
	switch mode {
	case "garbage_batch":
		_ = st.Send(&df0pb.WorkerEvent{Sequence: 2, Event: &df0pb.WorkerEvent_Batch{Batch: &df0pb.Batch{Rows: 1, ArrowIpc: []byte("garbage")}}})
	case "oversize":
		_ = st.Send(&df0pb.WorkerEvent{Sequence: 2, Event: &df0pb.WorkerEvent_Batch{Batch: &df0pb.Batch{Rows: 1, ArrowIpc: make([]byte, 5<<20)}}})
	}
	_, _ = st.Recv() // wait for the host to end the stream
	return 0
}

func TestHostRejectsProtocolViolations(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The fake worker needs its mode on the command line; Run passes none,
	// so each mode gets a tiny wrapper script.
	tests := []struct {
		mode, outcome string
	}{
		{"major2", "rejected_protocol"},
		{"wrong_pid", "rejected_protocol"},
		{"batch_before_hello", "rejected_protocol"},
		{"garbage_batch", "rejected_batch"},
		{"oversize", "rejected_frame"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			script := filepath.Join(shortDir(t), "w.sh")
			body := "#!/bin/sh\nexec '" + self + "' " + fakeFlag + " " + tt.mode + "\n"
			if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
				t.Fatal(err)
			}
			g := &GenVerifier{}
			res := Run(Job{Name: tt.mode, Worker: script, Timeout: 20 * time.Second,
				Execute: &df0pb.Execute{Operation: "generate"}, Verify: g.Check})
			if res.Outcome != tt.outcome {
				t.Fatalf("outcome %q (%s), want %q; worker: %s", res.Outcome, res.Detail, tt.outcome, res.WorkerDiag)
			}
			if !res.EndpointRemoved {
				t.Error("endpoint left behind")
			}
			if res.WorkerExit == "unknown" {
				t.Error("worker not reaped")
			}
		})
	}
}
