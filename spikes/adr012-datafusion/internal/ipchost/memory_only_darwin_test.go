// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"golang.org/x/sys/unix"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
	"gopmgr/spikes/adr012-datafusion/internal/source"
)

// Memory-only filesystem feasibility for the App Sandbox worker. The policy
// under test is App Sandbox plus child-local RLIMIT_FSIZE=0, an ignored
// SIGXFSZ, RLIMIT_CORE=0 and RLIMIT_NPROC=0, all installed before exec.
// FSEvents is the independent observer; a content and extended-attribute scan
// of the task-owned container is the second. Neither proves memory-only
// processing on its own: see docs/testing/adr-012-df0-memory-only.md.

// FSEvents flags used below (CoreServices FSEvents.h).
const (
	fsevMustScanSubDirs = 0x1
	fsevUserDropped     = 0x2
	fsevKernelDropped   = 0x4
	fsevCreated         = 0x100
	fsevRemoved         = 0x200
	fsevRenamed         = 0x800
	fsevModified        = 0x1000
	fsevXattrMod        = 0x8000
	fsevIsFile          = 0x10000
	fsevIsDir           = 0x20000
)

// eventsDropped reports whether FSEvents says it lost events. Such an
// observation is incomplete and must fail, never pass with gaps.
func eventsDropped(flags uint32) bool {
	return flags&(fsevMustScanSubDirs|fsevUserDropped|fsevKernelDropped) != 0
}

func TestFSEventsDropFlagsFailTheObservation(t *testing.T) {
	for _, tc := range []struct {
		flags   uint32
		dropped bool
	}{
		{fsevCreated | fsevIsFile, false},
		{fsevModified | fsevIsFile | fsevXattrMod, false},
		{fsevMustScanSubDirs, true},
		{fsevUserDropped | fsevCreated, true},
		{fsevKernelDropped | fsevModified | fsevIsFile, true},
	} {
		if got := eventsDropped(tc.flags); got != tc.dropped {
			t.Errorf("eventsDropped(%#x) = %v, want %v", tc.flags, got, tc.dropped)
		}
	}
}

func compileFSEventsProbe(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs("../../tools/macos_fsevents_probe.c")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "fsevents")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "xcrun", "clang", "-std=c11", "-Wall", "-Wextra", "-Werror", "-framework", "CoreServices", src, "-o", out).CombinedOutput()
	if err != nil {
		t.Fatalf("compile FSEvents observer: %v: %s", err, b)
	}
	return out
}

// testContainer creates a unique App Sandbox container by launching a signed
// sandboxed copy of the observer probe with no arguments. It refuses an
// existing container and removes only the one it created.
func testContainer(t *testing.T, observer string) (identifier, root string) {
	t.Helper()
	var entropy [4]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		t.Fatal(err)
	}
	identifier = "d0.mem." + hex.EncodeToString(entropy[:])
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(home, "Library", "Containers", identifier)
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refuse existing container: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove task-owned sandbox container: %v", err)
		}
	})
	guarded := signedProbeApp(t, observer, identifier, true)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, guarded)
	cmd.Env = []string{}
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("container creation launch: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(root, "Data", "tmp")); err != nil || !info.IsDir() {
		t.Fatalf("sandbox container was not created: %v", err)
	}
	return identifier, root
}

type fsEventSummary struct {
	Events        int      `json:"events"`
	FilesCreated  []string `json:"files_created"`
	FilesModified []string `json:"files_modified"`
	XattrChanged  []string `json:"xattr_changed"`
	FilesRenamed  []string `json:"files_renamed"`
	DirsCreated   []string `json:"dirs_created"`
	FilesRemoved  []string `json:"files_removed"`
}

type fsWatch struct {
	t       *testing.T
	root    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan string
	scanErr chan error
	flags   map[string]uint32
	order   []string
	count   int
}

// watchContainer starts FSEvents on root and returns once the stream is live.
func watchContainer(t *testing.T, probe, root string) *fsWatch {
	t.Helper()
	cmd := exec.Command(probe, root)
	cmd.Env = []string{}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	w := &fsWatch{t: t, root: root, cmd: cmd, stdin: stdin, lines: make(chan string, 4096), scanErr: make(chan error, 1), flags: map[string]uint32{}}
	go func() {
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 64<<10), 1<<20)
		for s.Scan() {
			w.lines <- s.Text()
		}
		w.scanErr <- s.Err()
		close(w.lines)
	}()
	t.Cleanup(func() {
		if w.cmd.ProcessState == nil {
			_ = w.cmd.Process.Kill()
			_ = w.cmd.Wait()
		}
	})
	select {
	case line, ok := <-w.lines:
		if !ok || line != `{"ready":true}` {
			t.Fatalf("FSEvents observer did not start: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FSEvents observer start timed out")
	}
	// A new stream can still deliver events recorded just before it started.
	// Everything up to this barrier belongs to earlier activity; drop it.
	w.barrier()
	w.flags, w.order, w.count = map[string]uint32{}, nil, 0
	return w
}

// record decodes one event line. Events outside the task-owned root are
// dropped without being kept, so no other application's names are recorded.
func (w *fsWatch) record(line string) (string, bool) {
	var e struct {
		PathHex string `json:"path_hex"`
		Flags   uint32 `json:"flags"`
		Done    bool   `json:"done"`
	}
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		w.t.Fatalf("FSEvents record: %v", err)
	}
	if e.Done {
		return "", true
	}
	raw, err := hex.DecodeString(e.PathHex)
	if err != nil {
		w.t.Fatalf("FSEvents path: %v", err)
	}
	if eventsDropped(e.Flags) {
		w.t.Fatalf("FSEvents dropped events (flags %#x): the observation is incomplete", e.Flags)
	}
	path := string(raw)
	rel, err := filepath.Rel(w.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	w.count++
	if _, seen := w.flags[rel]; !seen {
		w.order = append(w.order, rel)
	}
	w.flags[rel] |= e.Flags
	return rel, false
}

// barrierPrefix names the empty files the host writes to order events.
var barrierPrefix = filepath.Join("Data", "tmp", "df0-barrier-")

// barrier writes an empty host-owned file and consumes events until FSEvents
// reports it. FSEvents delivers events in event-ID order, so every event the
// kernel recorded before the barrier has then been read.
func (w *fsWatch) barrier() {
	w.t.Helper()
	var entropy [4]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		w.t.Fatal(err)
	}
	rel := barrierPrefix + hex.EncodeToString(entropy[:])
	path := filepath.Join(w.root, rel)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		w.t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for seen := false; !seen; {
		select {
		case line, ok := <-w.lines:
			if !ok {
				w.t.Fatalf("FSEvents observer ended early: %v", <-w.scanErr)
			}
			got, _ := w.record(line)
			seen = got == rel
		case <-deadline:
			w.t.Fatal("FSEvents barrier was not observed")
		}
	}
	if err := os.Remove(path); err != nil {
		w.t.Fatal(err)
	}
}

// stop writes a final barrier, then flushes and ends the stream.
func (w *fsWatch) stop() fsEventSummary {
	w.t.Helper()
	w.barrier()
	deadline := time.After(10 * time.Second)
	if err := w.stdin.Close(); err != nil {
		w.t.Fatal(err)
	}
	for done := false; !done; {
		select {
		case line, ok := <-w.lines:
			if !ok {
				w.t.Fatalf("FSEvents observer ended without its final record: %v", <-w.scanErr)
			}
			_, done = w.record(line)
		case <-deadline:
			w.t.Fatal("FSEvents flush timed out")
		}
	}
	if err := w.cmd.Wait(); err != nil {
		w.t.Fatalf("FSEvents observer exit: %v", err)
	}
	var s fsEventSummary
	s.Events = w.count
	for _, rel := range w.order {
		f := w.flags[rel]
		if strings.HasPrefix(rel, barrierPrefix) {
			continue
		}
		add := func(list *[]string, mask uint32) {
			if f&mask == mask {
				*list = append(*list, rel)
			}
		}
		add(&s.FilesCreated, fsevCreated|fsevIsFile)
		add(&s.FilesModified, fsevModified|fsevIsFile)
		add(&s.XattrChanged, fsevXattrMod)
		add(&s.FilesRenamed, fsevRenamed|fsevIsFile)
		add(&s.DirsCreated, fsevCreated|fsevIsDir)
		add(&s.FilesRemoved, fsevRemoved|fsevIsFile)
	}
	return s
}

type containerScan struct {
	// Regular files with content, relative to the container root.
	NonEmptyFiles     []string `json:"non_empty_files"`
	SentinelInContent []string `json:"sentinel_in_content"`
	SentinelInXattr   []string `json:"sentinel_in_xattr"`
	SentinelInName    []string `json:"sentinel_in_name"`
}

// scanContainer reads every regular file and every extended attribute in the
// task-owned container without following symlinks.
func scanContainer(t *testing.T, root string, sentinel []byte) containerScan {
	t.Helper()
	var s containerScan
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if bytes.Contains([]byte(d.Name()), sentinel) {
			s.SentinelInName = append(s.SentinelInName, rel)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		size, err := unix.Llistxattr(path, nil)
		if err != nil {
			return fmt.Errorf("list xattrs of %s: %w", rel, err)
		}
		if size > 0 {
			names := make([]byte, size)
			n, err := unix.Llistxattr(path, names)
			if err != nil {
				return err
			}
			for _, name := range strings.Split(strings.TrimRight(string(names[:n]), "\x00"), "\x00") {
				vsize, err := unix.Lgetxattr(path, name, nil)
				if err != nil {
					return fmt.Errorf("read xattr %s of %s: %w", name, rel, err)
				}
				value := make([]byte, vsize)
				if _, err := unix.Lgetxattr(path, name, value); err != nil {
					return err
				}
				if bytes.Contains(value, sentinel) {
					s.SentinelInXattr = append(s.SentinelInXattr, rel)
				}
			}
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > 0 {
			s.NonEmptyFiles = append(s.NonEmptyFiles, rel)
		}
		if bytes.Contains(data, sentinel) {
			s.SentinelInContent = append(s.SentinelInContent, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan container: %v", err)
	}
	return s
}

// unexpectedFiles returns non-empty files other than the container manager's
// own metadata and CrashReporter's delegated record, which another process
// writes on the worker's behalf after a crash.
func unexpectedFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if f == ".com.apple.containermanagerd.metadata.plist" || strings.HasPrefix(f, filepath.Join("Data", "Library", "Application Support", "CrashReporter")+"/") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// workerWritten returns the paths in list that the sandboxed worker could have
// written: anything under Data/ except CrashReporter's delegated record. The
// container manager's metadata outside Data/ is written by containermanagerd.
func workerWritten(list []string) []string {
	var out []string
	for _, rel := range list {
		if strings.HasPrefix(rel, "Data/") && !strings.HasPrefix(rel, filepath.Join("Data", "Library", "Application Support", "CrashReporter")+"/") {
			out = append(out, rel)
		}
	}
	return out
}

type crashReport struct {
	Observed          bool `json:"observed"`
	SentinelFound     bool `json:"sentinel_found"`
	MentionsContainer bool `json:"mentions_container"`
	Bytes             int  `json:"bytes"`
}

// takeCrashReport waits a bounded time for the crash report ReportCrash writes
// for pid, scans it, and deletes it. Only an <executable>-*.ips report newer
// than since that names pid is touched; absence is recorded, never proof.
func takeCrashReport(t *testing.T, executable string, pid int, since time.Time, container string, sentinel []byte) crashReport {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "Library", "Logs", "DiagnosticReports")
	pidPattern := regexp.MustCompile(fmt.Sprintf(`"pid"\s*:\s*%d\b`, pid))
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, executable+"-") || !strings.HasSuffix(name, ".ips") {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().Before(since.Add(-time.Second)) {
				continue
			}
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil || !pidPattern.Match(data) {
				continue
			}
			r := crashReport{Observed: true, SentinelFound: bytes.Contains(data, sentinel), MentionsContainer: bytes.Contains(data, []byte(container)), Bytes: len(data)}
			if err := os.Remove(path); err != nil {
				t.Errorf("remove task-owned crash report: %v", err)
			}
			return r
		}
		time.Sleep(250 * time.Millisecond)
	}
	return crashReport{}
}

func saveMemoryOnlyEvidence(t *testing.T, name string, value any) {
	t.Helper()
	if dir := os.Getenv("DF0_MEMORY_ONLY_EVIDENCE"); dir != "" && !t.Failed() {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

type growthObservation struct {
	Name         string         `json:"name"`
	Started      bool           `json:"started"`
	FSize        uint64         `json:"fsize_soft_bytes"`
	XFSZIgnored  bool           `json:"sigxfsz_ignored"`
	Create       int            `json:"create_errno"`
	Write        int            `json:"write_errno"`
	Truncate     int            `json:"ftruncate_errno"`
	Xattr        int            `json:"setxattr_errno"`
	Rename       int            `json:"rename_errno"`
	Mkdir        int            `json:"mkdir_errno"`
	Stat         int            `json:"stat_errno"`
	FinalSize    int64          `json:"final_size"`
	ExitSignal   string         `json:"exit_signal,omitempty"`
	Scan         containerScan  `json:"container_scan"`
	FSEvents     fsEventSummary `json:"fsevents"`
	CrashReports *crashReport   `json:"crash_report,omitempty"`
}

// TestMacOSFileGrowthLimitChannels characterizes what the zero file-size
// limit does and does not stop inside the worker's own App Sandbox
// container, and shows that the deprecated write-denial profile cannot be
// combined with App Sandbox.
func TestMacOSFileGrowthLimitChannels(t *testing.T) {
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("App Sandbox experiment requires non-root")
	}
	observer := compileObserverProbe(t)
	fsevents := compileFSEventsProbe(t)
	identifier, root := testContainer(t, observer)
	guarded := signedProbeApp(t, observer, identifier, true)
	var entropy [6]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		t.Fatal(err)
	}
	marker := "DF0GROWTH" + strings.ToUpper(hex.EncodeToString(entropy[:]))
	workDir := filepath.Join(root, "Data", "tmp")
	clean := func() {
		entries, err := os.ReadDir(workDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "growth-") {
				if err := os.RemoveAll(filepath.Join(workDir, e.Name())); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	run := func(name string, args ...string) growthObservation {
		clean()
		w := watchContainer(t, fsevents, root)
		since := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = []string{}
		out, err := cmd.Output()
		var o growthObservation
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				o.ExitSignal = ws.Signal().String()
				r := takeCrashReport(t, appExecutable(identifier), cmd.Process.Pid, since, root, []byte(marker))
				o.CrashReports = &r
			} else {
				t.Fatalf("%s exited %d", name, exit.ExitCode())
			}
		} else if err != nil {
			t.Fatalf("%s: %v", name, err)
		} else if err := json.Unmarshal(out, &o); err != nil || !o.Started {
			t.Fatalf("%s record: %v: %s", name, err, out)
		}
		o.Name = name
		o.FSEvents = w.stop()
		o.Scan = scanContainer(t, root, []byte(marker))
		return o
	}
	var observations []growthObservation

	plain := run("app_sandbox_only", guarded, "growth", workDir, marker)
	if plain.FSize == 0 || plain.Create != 0 || plain.Write != 0 || plain.Truncate != 0 || plain.FinalSize != 4096 || len(plain.Scan.SentinelInContent) != 1 || len(workerWritten(plain.FSEvents.FilesModified)) == 0 {
		t.Fatalf("positive control: App Sandbox should allow own-container content: %+v", plain)
	}
	observations = append(observations, plain)

	limited := run("app_sandbox_fsize_zero", observer, "growth-launch", guarded, workDir, marker)
	efbig := int(syscall.EFBIG)
	if limited.FSize != 0 || !limited.XFSZIgnored || limited.Create != 0 || limited.Write != efbig || limited.Truncate != efbig || limited.FinalSize != 0 || len(limited.Scan.SentinelInContent) != 0 || len(workerWritten(limited.FSEvents.FilesModified)) != 0 {
		t.Fatalf("zero file-size limit did not stop file content: %+v", limited)
	}
	// These channels stay open under the limit. The test records them so a
	// change in OS behavior is noticed rather than assumed.
	if limited.Xattr != 0 || limited.Rename != 0 || limited.Mkdir != 0 || len(limited.Scan.SentinelInXattr) != 1 || len(limited.Scan.SentinelInName) != 1 || len(limited.FSEvents.DirsCreated) == 0 || len(limited.FSEvents.FilesCreated) == 0 {
		t.Fatalf("metadata channels behaved differently than recorded: %+v", limited)
	}
	observations = append(observations, limited)

	stacked := run("sandbox_exec_then_app_sandbox", "/usr/bin/sandbox-exec", "-p", denyFileWrites, guarded, "growth", workDir, marker)
	if stacked.ExitSignal != syscall.SIGTRAP.String() || stacked.Started || stacked.CrashReports == nil || !stacked.CrashReports.Observed || stacked.CrashReports.SentinelFound || len(stacked.Scan.SentinelInContent)+len(stacked.Scan.SentinelInXattr)+len(stacked.Scan.SentinelInName) != 0 {
		t.Fatalf("expected App Sandbox to refuse an already-sandboxed process: %+v", stacked)
	}
	observations = append(observations, stacked)
	clean()
	saveMemoryOnlyEvidence(t, "growth-channels.json", observations)
}

type memoryOnlyObservation struct {
	Name           string                 `json:"name"`
	Policy         string                 `json:"policy"`
	Outcome        string                 `json:"outcome"`
	FailedCode     string                 `json:"failed_code,omitempty"`
	Detail         string                 `json:"detail,omitempty"`
	SourceReads    int                    `json:"source_reads"`
	Rows           uint64                 `json:"rows"`
	WorkerExit     string                 `json:"worker_exit"`
	Killed         bool                   `json:"killed"`
	EndpointGone   bool                   `json:"endpoint_removed"`
	SpillCount     uint64                 `json:"spill_count"`
	SpilledBytes   uint64                 `json:"spilled_bytes"`
	SpillSentinel  bool                   `json:"sentinel_seen_in_spill_during_run"`
	SentinelInDiag bool                   `json:"sentinel_in_diagnostics"`
	Launcher       []execLimitObservation `json:"launcher_configuration"`
	FSEvents       fsEventSummary         `json:"fsevents"`
	ScanBefore     containerScan          `json:"container_scan_before"`
	ScanAfter      containerScan          `json:"container_scan_after"`
	CrashReport    *crashReport           `json:"crash_report,omitempty"`
}

// TestMacOSMemoryOnlyWorker runs the real Rust worker inside its App Sandbox
// container, with and without the zero file-size limit, and checks spill,
// pressure, cancellation, kill, crash and recovery against FSEvents and a
// content scan of the container.
func TestMacOSMemoryOnlyWorker(t *testing.T) {
	worker := os.Getenv("DF0_WORKER")
	if worker == "" {
		t.Skip("set DF0_WORKER to the reviewed native worker")
	}
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		t.Skip("App Sandbox experiment requires non-root")
	}
	observer := compileObserverProbe(t)
	fsevents := compileFSEventsProbe(t)
	identifier, root := testContainer(t, observer)
	signedWorker := signedProbeApp(t, worker, identifier, true)
	originalHash := sandboxArtifactHash(t, worker)
	signedHash := sandboxArtifactHash(t, signedWorker)
	launchers := map[string]string{
		"app_sandbox":            compileExecLimitProbe(t, signedWorker, launcherLimits{DenyChildren: true}),
		"app_sandbox_fsize_zero": compileExecLimitProbe(t, signedWorker, launcherLimits{DenyChildren: true, DenyFileGrowth: true}),
	}
	inside := filepath.Join(root, "Data", "tmp")
	before := EndpointBase
	EndpointBase = inside
	t.Cleanup(func() { EndpointBase = before })

	var data bytes.Buffer
	if err := fixtures.WriteNoSpillParquet(&data); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "selected.parquet")
	if err := os.WriteFile(sourcePath, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte(fixtures.NoSpillSentinel)

	type scenario struct {
		name, policy string
		spill        bool
		pool         uint64
		cancel       int
		killRead     uint64
		crashRead    uint64
	}
	scenarios := []scenario{
		{name: "spill_control", policy: "app_sandbox", spill: true, pool: 1 << 20},
		{name: "spill_under_policy", policy: "app_sandbox_fsize_zero", spill: true, pool: 1 << 20},
		{name: "memory_only_pressure", policy: "app_sandbox_fsize_zero", pool: 1 << 20},
		{name: "normal", policy: "app_sandbox_fsize_zero", pool: 64 << 20},
		{name: "cancel", policy: "app_sandbox_fsize_zero", pool: 64 << 20, cancel: 1},
		// The fixture takes 17 source reads. Stopping at read 16 means 15 have
		// been served. Which column chunks each read carried is not checked.
		{name: "kill", policy: "app_sandbox_fsize_zero", pool: 64 << 20, killRead: 16},
		{name: "crash", policy: "app_sandbox_fsize_zero", pool: 64 << 20, crashRead: 16},
		{name: "recovery", policy: "app_sandbox_fsize_zero", pool: 64 << 20},
	}
	var evidence []memoryOnlyObservation
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			o := memoryOnlyObservation{Name: sc.name, Policy: sc.policy}
			o.ScanBefore = scanContainer(t, root, sentinel)
			// What an earlier job left behind is checked before this job runs.
			if extra := unexpectedFiles(o.ScanBefore.NonEmptyFiles); len(extra) != 0 || len(o.ScanBefore.SentinelInContent)+len(o.ScanBefore.SentinelInXattr) != 0 {
				t.Fatalf("earlier job left content in the container: %+v", o.ScanBefore)
			}
			snap, err := source.Capture(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			ex := &df0pb.Execute{Operation: "parquet_range", BatchRows: 64, SortSource: true, QueryMemoryBytes: sc.pool}
			spillDir := filepath.Join(inside, "spill-"+sc.name)
			if sc.spill {
				if err := os.Mkdir(spillDir, 0700); err != nil {
					t.Fatal(err)
				}
				ex.SpillControlDirectory = spillDir
				ex.Fault = "source_spill_control"
			}
			job := Job{Name: sc.name, Worker: launchers[sc.policy], ReadSource: snap, Execute: ex, Timeout: 30 * time.Second,
				CancelAfter: sc.cancel, KillAfterRead: sc.killRead, CrashAfterRead: sc.crashRead}
			next := fixtures.NoSpillRows - 1
			job.Verify = func(data []byte) (int, error) {
				if sc.spill && !o.SpillSentinel {
					found, _, err := sentinelInFiles(spillDir)
					if err != nil {
						return 0, err
					}
					o.SpillSentinel = found
				}
				rd, err := ipc.NewReader(bytes.NewReader(data))
				if err != nil {
					return 0, err
				}
				defer rd.Release()
				rows := 0
				for rd.Next() {
					rec := rd.RecordBatch()
					if rec.NumCols() != 2 {
						return 0, errors.New("wrong schema")
					}
					ids, ok := rec.Column(0).(*array.Int64)
					if !ok {
						return 0, errors.New("id lost int64 type")
					}
					payload, ok := rec.Column(1).(array.StringLike)
					if !ok {
						return 0, errors.New("payload lost string type")
					}
					for i := 0; i < ids.Len(); i++ {
						if ids.IsNull(i) || payload.IsNull(i) || ids.Value(i) != fixtures.ReadAtFirstID+int64(next) || !strings.Contains(payload.Value(i), fixtures.NoSpillSentinel) {
							return 0, errors.New("source sort oracle mismatch")
						}
						next--
						rows++
					}
				}
				return rows, rd.Err()
			}
			w := watchContainer(t, fsevents, root)
			started := time.Now()
			r := Run(job)
			o.FSEvents = w.stop()
			o.Outcome, o.FailedCode, o.Rows, o.WorkerExit, o.Killed, o.EndpointGone = r.Outcome, r.FailedCode, r.Rows, r.WorkerExit, r.Killed, r.EndpointRemoved
			o.SourceReads = len(r.SourceReads)
			o.SentinelInDiag = bytes.Contains([]byte(r.Detail+r.WorkerDiag), sentinel)
			if !o.SentinelInDiag {
				o.Detail = r.Detail
			}
			first, _, _ := strings.Cut(strings.TrimSpace(r.WorkerDiag), "\n")
			o.Launcher = readExecLimitObservations(t, first)
			if r.ResultJSON != "" {
				var m struct {
					SpillCount   uint64 `json:"spill_count"`
					SpilledBytes uint64 `json:"spilled_bytes"`
				}
				if err := json.Unmarshal([]byte(r.ResultJSON), &m); err == nil {
					o.SpillCount, o.SpilledBytes = m.SpillCount, m.SpilledBytes
				}
			}
			if sc.crashRead > 0 {
				report := takeCrashReport(t, appExecutable(identifier), r.WorkerPID, started, root, sentinel)
				o.CrashReport = &report
			}
			o.ScanAfter = scanContainer(t, root, sentinel)
			if sc.spill {
				if err := os.RemoveAll(spillDir); err != nil {
					t.Fatal(err)
				}
			}
			evidence = append(evidence, o)

			cfg := o.Launcher
			if len(cfg) != 1 || cfg[0].Stage != "preexec" || cfg[0].InstallErrno != 0 || cfg[0].NProc != 0 || cfg[0].NProcHard != 0 || cfg[0].Core != 0 || cfg[0].CoreHard != 0 {
				t.Fatalf("launcher configuration: %+v", cfg)
			}
			policy := sc.policy == "app_sandbox_fsize_zero"
			if policy != (cfg[0].FSize == 0 && cfg[0].FSizeHard == 0 && cfg[0].XFSZIgnored) {
				t.Fatalf("file-growth limit readback does not match the policy: %+v", cfg[0])
			}
			if o.SentinelInDiag || !o.EndpointGone {
				t.Fatalf("diagnostics or endpoint cleanup: %+v", o)
			}
			if len(o.ScanAfter.SentinelInContent)+len(o.ScanAfter.SentinelInXattr)+len(o.ScanAfter.SentinelInName) != 0 {
				t.Fatalf("sentinel persisted in the container: %+v", o.ScanAfter)
			}
			if extra := unexpectedFiles(o.ScanAfter.NonEmptyFiles); len(extra) != 0 {
				t.Fatalf("content persisted in the container: %v", extra)
			}
			if !policy {
				// Positive control: the observer must see the spill content
				// that the policy runs below must not produce.
				if r.Outcome != "completed" || r.Rows != fixtures.NoSpillRows || o.SpillCount == 0 || o.SpilledBytes == 0 || !o.SpillSentinel || len(workerWritten(o.FSEvents.FilesModified)) == 0 {
					t.Fatalf("spill positive control did not write plaintext: %+v diag=%s", o, r.WorkerDiag)
				}
				return
			}
			if len(workerWritten(o.FSEvents.FilesModified)) != 0 || o.SpillSentinel {
				t.Fatalf("file content was written under the policy: %+v", o)
			}
			switch sc.name {
			case "spill_under_policy":
				// A refused spill write is a resource limit with a constant
				// detail; no OS error text reaches the host.
				if r.Outcome != "failed" || r.FailedCode != "RESOURCE_LIMIT" || r.Detail != "source query file writes are not permitted" || r.Rows != 0 || strings.HasPrefix(r.WorkerExit, "signal") || len(o.FSEvents.FilesCreated) == 0 {
					t.Fatalf("spill under the policy should fail as a typed error, not kill the worker: %+v diag=%s", o, r.WorkerDiag)
				}
			case "memory_only_pressure":
				if r.Outcome != "failed" || r.FailedCode != "RESOURCE_LIMIT" || r.Rows != 0 {
					t.Fatalf("pressure did not fail closed: %+v", o)
				}
			case "cancel":
				if r.Outcome != "failed" || r.FailedCode != "USER_CANCELLED" {
					t.Fatalf("cancel: %+v", o)
				}
			case "kill":
				if r.Outcome != "fault_killed" || !r.Killed || r.Rows != 0 || o.SourceReads != 16 {
					t.Fatalf("kill: %+v", o)
				}
			case "crash":
				if r.Outcome == "completed" || r.WorkerExit != "signal "+syscall.SIGABRT.String() || o.SourceReads != 16 {
					t.Fatalf("crash: %+v", o)
				}
				// A missing report is an incomplete observation, not a pass.
				if !o.CrashReport.Observed || o.CrashReport.SentinelFound {
					t.Fatalf("crash report: %+v", o.CrashReport)
				}
			default:
				if r.Outcome != "completed" || r.Rows != fixtures.NoSpillRows || next != -1 {
					t.Fatalf("query: %+v diag=%s", o, r.WorkerDiag)
				}
			}
		})
	}
	if t.Failed() {
		return
	}
	if sandboxArtifactHash(t, worker) != originalHash {
		t.Fatal("source worker was modified while signing its copy")
	}
	saveMemoryOnlyEvidence(t, "memory-only-worker.json", struct {
		Scenarios    []memoryOnlyObservation `json:"scenarios"`
		InputWorker  string                  `json:"input_worker_sha256"`
		SignedWorker string                  `json:"signed_copy_sha256"`
	}{evidence, originalHash, signedHash})
}
