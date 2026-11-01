// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

func signedProbeApp(t *testing.T, binary, identifier string, sandbox bool) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "Probe.app")
	contents := filepath.Join(app, "Contents")
	macos := filepath.Join(contents, "MacOS")
	if err := os.MkdirAll(macos, 0700); err != nil {
		t.Fatal(err)
	}
	info := fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>%s</string><key>CFBundleExecutable</key><string>probe</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`, identifier)
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(info), 0600); err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	exe := filepath.Join(macos, "probe")
	dst, err := os.OpenFile(exe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy probe: %v, close: %v", copyErr, closeErr)
	}
	ent := `<?xml version="1.0"?><plist version="1.0"><dict></dict></plist>`
	if sandbox {
		ent = `<?xml version="1.0"?><plist version="1.0"><dict><key>com.apple.security.app-sandbox</key><true/></dict></plist>`
	}
	entPath := filepath.Join(filepath.Dir(app), "entitlements.plist")
	if err := os.WriteFile(entPath, []byte(ent), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--force", "--sign", "-", "--identifier", identifier, "--options", "runtime", "--entitlements", entPath, app},
		{"--verify", "--strict", app},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		b, err := exec.CommandContext(ctx, "/usr/bin/codesign", args...).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("sign/verify temporary app: %v: %s", err, b)
		}
	}
	return exe
}

type accessObservation struct {
	Name    string `json:"name"`
	Started bool   `json:"started"`
	Read    int    `json:"read_errno"`
	Byte    byte   `json:"read_byte"`
	Write   int    `json:"write_errno"`
	Unix    int    `json:"unix_errno"`
	TCP     int    `json:"tcp_errno"`
	Fork    int    `json:"fork_errno"`
}

func sandboxArtifactHash(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestMacOSAppSandboxAccess(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("child-creation experiment requires non-root")
	}
	probe := compileObserverProbe(t)
	var entropy [4]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		t.Fatal(err)
	}
	identifier := "d0." + hex.EncodeToString(entropy[:])
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	container := filepath.Join(home, "Library", "Containers", identifier)
	if _, err := os.Lstat(container); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refuse existing container: %v", err)
	}
	// The OS creates this unique test container on the first sandboxed launch.
	// Never reuse/delete an existing application container.
	t.Cleanup(func() {
		if err := os.RemoveAll(container); err != nil {
			t.Errorf("remove task-owned sandbox container: %v", err)
		}
	})
	control := signedProbeApp(t, probe, identifier, false)
	guarded := signedProbeApp(t, probe, identifier, true)
	root, err := os.MkdirTemp("/tmp", "df0-iso-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove private probe directory: %v", err)
		}
	})
	canary := filepath.Join(root, "private")
	if err := os.WriteFile(canary, []byte("Z"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(canary, alias); err != nil {
		t.Fatal(err)
	}
	unixPath := filepath.Join(root, "outside.sock")
	local, err := net.Listen("unix", unixPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = local.Close() })
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tcp.Close() })
	port := strconv.Itoa(tcp.Addr().(*net.TCPAddr).Port)
	run := func(exe, readPath, writePath, socket, children string) accessObservation {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "access", readPath, writePath, socket, port, children)
		cmd.Env = []string{}
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("access probe did not start/complete: %v", err)
		}
		var r accessObservation
		if err := json.Unmarshal(b, &r); err != nil || !r.Started {
			t.Fatalf("access probe record: %v: %s", err, b)
		}
		return r
	}
	permission := func(errno int) bool { return errno == int(syscall.EPERM) || errno == int(syscall.EACCES) }
	var observations []accessObservation
	controlWrite := filepath.Join(root, "control-write")
	r := run(control, canary, controlWrite, unixPath, "allow-children")
	r.Name = "unrestricted_control"
	if r.Read != 0 || r.Byte != 'Z' || r.Write != 0 || r.Unix != 0 || r.TCP != 0 || r.Fork != 0 {
		t.Fatalf("positive controls failed: %+v", r)
	}
	if b, err := os.ReadFile(controlWrite); err != nil || !bytes.Equal(b, []byte("Z")) {
		t.Fatalf("positive write effect missing: %v", err)
	}
	observations = append(observations, r)
	for i, readPath := range []string{canary, alias} {
		writePath := filepath.Join(root, fmt.Sprintf("denied-%d", i))
		r := run(guarded, readPath, writePath, unixPath, "deny-children")
		r.Name = []string{"outside_private", "outside_symlink"}[i]
		if !permission(r.Read) || r.Byte != 0 || !permission(r.Write) || !permission(r.Unix) || !permission(r.TCP) || r.Fork != int(syscall.EAGAIN) {
			t.Fatalf("OS access/child restrictions not enforced: %+v", r)
		}
		if _, err := os.Stat(writePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("denied write persisted: %v", err)
		}
		observations = append(observations, r)
	}
	inside := filepath.Join(container, "Data", "tmp")
	if err := os.MkdirAll(inside, 0700); err != nil {
		t.Fatal(err)
	}
	insideSocket := filepath.Join(inside, "control.sock")
	insideListener, err := net.Listen("unix", insideSocket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = insideListener.Close() })
	insideWrite := filepath.Join(inside, "allowed-write")
	r = run(guarded, canary, insideWrite, insideSocket, "deny-children")
	r.Name = "own_container_write_and_unix"
	if !permission(r.Read) || r.Write != 0 || r.Unix != 0 || !permission(r.TCP) || r.Fork != int(syscall.EAGAIN) {
		t.Fatalf("container/UDS compatibility control failed: %+v", r)
	}
	if b, err := os.ReadFile(insideWrite); err != nil || !bytes.Equal(b, []byte("Z")) {
		t.Fatalf("sandbox container write was not demonstrated: %v", err)
	}
	observations = append(observations, r)
	t.Log("App Sandbox permits own-container writes; this is not memory-only spill enforcement")
	t.Run("native-worker", func(t *testing.T) {
		worker := os.Getenv("DF0_WORKER")
		if worker == "" {
			t.Skip("set DF0_WORKER for native Rust compatibility")
		}
		signedWorker := signedProbeApp(t, worker, identifier, true)
		originalHash := sandboxArtifactHash(t, worker)
		signedHash := sandboxArtifactHash(t, signedWorker)
		launcher := compileExecLimitProbe(t, signedWorker, 0, true)
		before := EndpointBase
		EndpointBase = inside
		t.Cleanup(func() { EndpointBase = before })
		verifier := &GenVerifier{Width: 32}
		var samples []usageSample
		var sampleErr error
		pid := 0
		verifyGeneration := func(data []byte) (int, error) {
			if len(samples) == 1 {
				s, err := observeUsage(probe, []int{pid}, map[int]uint64{pid: samples[0].Start})
				if err != nil {
					return 0, err
				}
				samples = append(samples, s...)
			}
			return verifier.Check(data)
		}
		result := Run(Job{Name: "app-sandbox-generate", Worker: launcher, Timeout: 15 * time.Second,
			Execute: &df0pb.Execute{Operation: "generate", Rows: 128, BatchRows: 32, PayloadBytes: 32},
			Verify:  verifyGeneration, OnSpawn: func(childPID int, _ string) {
				pid = childPID
				samples, sampleErr = observeUsage(probe, []int{pid}, nil)
			}})
		if sampleErr != nil || len(samples) != 2 || result.Outcome != "completed" || result.Rows != 128 || result.DigestMatch == nil || !*result.DigestMatch || result.WorkerExit != "exit 0" || !result.EndpointRemoved {
			t.Fatalf("sandboxed Rust/native UDS compatibility: sample=%v result=%+v", sampleErr, result)
		}
		var data bytes.Buffer
		if err := fixtures.WriteReadAtParquet(&data); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "host-only.parquet")
		if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		readProbe := run(guarded, path, filepath.Join(root, "denied-parquet-write"), unixPath, "deny-children")
		readProbe.Name = "host_only_parquet"
		if !permission(readProbe.Read) || !permission(readProbe.Write) || !permission(readProbe.Unix) || !permission(readProbe.TCP) || readProbe.Fork != int(syscall.EAGAIN) {
			t.Fatalf("host source path was not protected: %+v", readProbe)
		}
		observations = append(observations, readProbe)
		snapshot, err := source.Capture(path)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[int64]bool{}
		verify := func(data []byte) (int, error) {
			rd, err := ipc.NewReader(bytes.NewReader(data))
			if err != nil {
				return 0, err
			}
			defer rd.Release()
			n := 0
			for rd.Next() {
				rec := rd.RecordBatch()
				if rec.NumCols() != 2 {
					return 0, fmt.Errorf("wrong range result shape")
				}
				ids, okID := rec.Column(0).(*array.Int64)
				values, okValue := rec.Column(1).(*array.Int64)
				if !okID || !okValue {
					return 0, fmt.Errorf("wrong range result types")
				}
				for i := range int(rec.NumRows()) {
					id := ids.Value(i)
					index := id - fixtures.ReadAtFirstID
					if ids.IsNull(i) || values.IsNull(i) || index < 0 || index >= fixtures.ReadAtRows || seen[id] || values.Value(i) != index*137-500000 {
						return 0, fmt.Errorf("range oracle mismatch for id %d", id)
					}
					seen[id] = true
					n++
				}
			}
			return n, rd.Err()
		}
		rangeResult := Run(Job{Name: "app-sandbox-parquet", Worker: launcher, Timeout: 20 * time.Second, ReadSource: snapshot, Verify: verify,
			Execute: &df0pb.Execute{Operation: "parquet_range", BatchRows: 128, Projection: []string{"id", "value"}}})
		if rangeResult.Outcome != "completed" || len(seen) != fixtures.ReadAtRows || rangeResult.Rows != fixtures.ReadAtRows || rangeResult.DigestMatch == nil || !*rangeResult.DigestMatch || rangeResult.SourceMatch == nil || !*rangeResult.SourceMatch || rangeResult.WorkerExit != "exit 0" || !rangeResult.EndpointRemoved {
			t.Fatalf("host-mediated source under App Sandbox: %+v", rangeResult)
		}
		type nativeObservation struct {
			Name    string                 `json:"name"`
			Outcome string                 `json:"outcome"`
			Rows    uint64                 `json:"rows"`
			Digest  *bool                  `json:"digest_match"`
			Source  *bool                  `json:"source_match,omitempty"`
			Exit    string                 `json:"worker_exit"`
			Removed bool                   `json:"endpoint_removed"`
			Config  []execLimitObservation `json:"launcher_configuration"`
		}
		var native []nativeObservation
		for _, r := range []Result{result, rangeResult} {
			first, _, _ := strings.Cut(strings.TrimSpace(r.WorkerDiag), "\n")
			cfg := readExecLimitObservations(t, first)
			if len(cfg) != 1 || cfg[0].Stage != "preexec" || cfg[0].ReadErrno != 0 || cfg[0].InstallErrno != 0 || cfg[0].NProc != 0 || cfg[0].NProcHard != 0 {
				t.Fatalf("native child-denial configuration missing: %+v", cfg)
			}
			native = append(native, nativeObservation{r.Name, r.Outcome, r.Rows, r.DigestMatch, r.SourceMatch, r.WorkerExit, r.EndpointRemoved, cfg})
		}
		saveObserverEvidence(t, "sandbox-native.json", struct {
			Jobs         []nativeObservation `json:"jobs"`
			Usage        []usageSample       `json:"usage"`
			InputWorker  string              `json:"input_worker_sha256"`
			SignedWorker string              `json:"signed_copy_sha256"`
		}{native, samples, originalHash, signedHash})
		if sandboxArtifactHash(t, worker) != originalHash {
			t.Fatal("source worker was modified while signing its copy")
		}
	})
	saveObserverEvidence(t, "sandbox-access.json", observations)
}
