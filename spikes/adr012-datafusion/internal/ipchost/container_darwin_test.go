// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const sweepMarker = "DF0SWEEPSECRET"

// fakeContainer builds a small tree shaped like an App Sandbox Data
// directory, with a symlink to a file outside it.
// testLock returns a slot lock path in a fresh folder only this user can
// reach.
func testLock(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "slot.lock")
}

func fakeContainer(t *testing.T) (root, outside string) {
	t.Helper()
	dir := t.TempDir()
	root = filepath.Join(dir, "Data")
	outside = filepath.Join(dir, "outside.txt")
	if err := os.WriteFile(outside, []byte("host file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "outside-dir"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outside-dir", "keep"), []byte("host dir file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"tmp", "Library/Caches"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Library", "pref"), []byte("base"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "Downloads")); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lsetxattr(filepath.Join(root, "Library"), "org.gopmgr.base", []byte("kept"), 0); err != nil {
		t.Fatal(err)
	}
	// Clear any flags and modes the test left, so TempDir cleanup works even
	// after a failure mid-test. WalkDir opens a directory up before reading it.
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if d != nil && d.Type()&fs.ModeSymlink == 0 {
				_ = unix.Chflags(p, 0)
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
	})
	return root, outside
}

// dirty does what a worker inside the container could do: names and
// attribute values carrying data, nested directories it makes unreadable, an
// immutable file, a mode change on a baseline directory and a symlink that
// points outside.
func dirty(t *testing.T, root, folder, outside string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{folder, filepath.Join(root, "tmp")} {
		must(os.WriteFile(filepath.Join(dir, "name-"+sweepMarker), nil, 0o600))
	}
	nested := filepath.Join(folder, "a", "b")
	must(os.MkdirAll(nested, 0o700))
	must(os.WriteFile(filepath.Join(nested, "spill"), []byte(sweepMarker), 0o600))
	must(os.Chmod(filepath.Join(folder, "a"), 0))
	locked := filepath.Join(root, "tmp", "locked")
	must(os.WriteFile(locked, nil, 0o600))
	must(unix.Chflags(locked, unix.UF_IMMUTABLE))
	must(unix.Lsetxattr(filepath.Join(root, "tmp"), "org.gopmgr."+sweepMarker, []byte(sweepMarker), 0))
	must(unix.Lsetxattr(filepath.Join(root, "Library", "pref"), "org.gopmgr.added", []byte(sweepMarker), 0))
	must(os.Symlink(outside, filepath.Join(root, "tmp", "escape")))
	// A symlink to an outside directory: following it would chmod and empty
	// a directory the host owns elsewhere.
	outsideDir := filepath.Join(filepath.Dir(outside), "outside-dir")
	must(os.Symlink(outsideDir, filepath.Join(folder, "escape-dir")))
	must(os.Chmod(filepath.Join(root, "tmp"), 0))
}

func TestContainerSweepRestoresTheBaseline(t *testing.T) {
	root, outside := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	var folder string
	res := c.Do("dirty", func(f string) Result {
		folder = f
		if entries, err := os.ReadDir(f); err != nil || len(entries) != 0 {
			t.Fatalf("job folder is not fresh and empty: %v %d", err, len(entries))
		}
		dirty(t, root, f, outside)
		return NewResult("dirty")
	})
	rep := res.ContainerSweep
	if rep == nil || !rep.Clean || rep.Failure != "" || rep.BaselineChanged != 0 {
		t.Fatalf("sweep did not restore the baseline: %+v", rep)
	}
	if rep.EntriesRemoved == 0 || rep.XattrsRemoved != 2 || rep.ModesRestored != 1 || rep.FlagsCleared != 1 {
		t.Fatalf("sweep counts: %+v", rep)
	}
	after, err := snapshotTree(root)
	if err != nil || !sameTree(after, c.base) {
		t.Fatalf("container differs from the baseline after the sweep: %v", err)
	}
	if _, err := os.Lstat(folder); !os.IsNotExist(err) {
		t.Fatalf("job folder survived: %v", err)
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "host file" {
		t.Fatalf("sweep followed a symlink out of the container: %v", err)
	}
	outsideDir := filepath.Join(filepath.Dir(outside), "outside-dir")
	if fi, err := os.Stat(outsideDir); err != nil || fi.Mode().Perm() != 0o750 {
		t.Fatalf("sweep changed a directory outside the container: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(outsideDir, "keep")); err != nil || string(b) != "host dir file" {
		t.Fatalf("sweep deleted through a symlinked directory: %v", err)
	}
	b, err := json.Marshal(res)
	if err != nil || strings.Contains(string(b), sweepMarker) {
		t.Fatalf("the result names what was swept: %v", err)
	}
	// The next job runs normally.
	if res := c.Do("next", func(string) Result { return NewResult("next") }); res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		t.Fatalf("next job: %+v", res.ContainerSweep)
	}
}

func TestContainerRefusesJobsAfterAChangedBaseline(t *testing.T) {
	for _, change := range []struct {
		name string
		do   func(root, outside string) error
	}{
		{"retargeted symlink", func(root, outside string) error {
			link := filepath.Join(root, "Downloads")
			if err := os.Remove(link); err != nil {
				return err
			}
			return os.Symlink(outside+".other", link)
		}},
		{"changed attribute value", func(root, _ string) error {
			return unix.Lsetxattr(filepath.Join(root, "Library"), "org.gopmgr.base", []byte("changed"), 0)
		}},
		{"changed file content", func(root, _ string) error {
			return os.WriteFile(filepath.Join(root, "Library", "pref"), []byte("other"), 0o600)
		}},
		{"removed baseline directory", func(root, _ string) error {
			return os.Remove(filepath.Join(root, "Library", "Caches"))
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			root, outside := fakeContainer(t)
			c, err := NewContainer(root, testLock(t))
			if err != nil {
				t.Fatal(err)
			}
			res := c.Do("change", func(string) Result {
				if err := change.do(root, outside); err != nil {
					t.Fatal(err)
				}
				return NewResult("change")
			})
			if rep := res.ContainerSweep; rep == nil || rep.Clean || rep.BaselineChanged == 0 {
				t.Fatalf("a changed baseline entry was not reported: %+v", rep)
			}
			ran := false
			next := c.Do("refused", func(string) Result { ran = true; return NewResult("refused") })
			if ran || next.Outcome != "setup_failed" || next.Detail != ErrContainerUnclean.Error() {
				t.Fatalf("an unclean container accepted a job: ran=%v %+v", ran, next)
			}
		})
	}
}

// A host that dies mid-job leaves the job's files behind. The next host loads
// the saved baseline and sweeps before running anything.
func TestContainerSweepAfterHostDeath(t *testing.T) {
	root, outside := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(t.TempDir(), "baseline.json")
	if err := c.Save(saved); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "df0-job-left-by-dead-host")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	dirty(t, root, folder, outside)
	restarted, err := LoadContainer(saved, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	if rep := restarted.Sweep(); !rep.Clean || rep.EntriesRemoved == 0 {
		t.Fatalf("startup sweep: %+v", rep)
	}
	if after, err := snapshotTree(root); err != nil || !sameTree(after, restarted.base) {
		t.Fatalf("container differs from the saved baseline: %v", err)
	}
}

// Workers share one container, so a sweep must never run while another job
// on it is in flight.
func TestContainerRunsJobsOneAtATime(t *testing.T) {
	root, _ := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	var inFlight, most atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := c.Do("concurrent", func(folder string) Result {
				n := inFlight.Add(1)
				for {
					m := most.Load()
					if n <= m || most.CompareAndSwap(m, n) {
						break
					}
				}
				if err := os.WriteFile(filepath.Join(folder, "live"), nil, 0o600); err != nil {
					t.Error(err)
				}
				time.Sleep(20 * time.Millisecond)
				if _, err := os.Stat(filepath.Join(folder, "live")); err != nil {
					t.Errorf("a sweep removed a live job's file: %v", err)
				}
				inFlight.Add(-1)
				return NewResult("concurrent")
			})
			if res.ContainerSweep == nil || !res.ContainerSweep.Clean {
				t.Errorf("sweep: %+v", res.ContainerSweep)
			}
		}()
	}
	wg.Wait()
	if most.Load() != 1 {
		t.Fatalf("%d jobs used the container at once", most.Load())
	}
}

// A failed sweep reports the system error only. Paths inside the container
// are names the worker chose, so they must not reach the result.
func TestContainerSweepFailureNamesNothing(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "Data-"+sweepMarker)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(dir, "moved")); err != nil {
		t.Fatal(err)
	}
	rep := c.Sweep()
	if rep.Clean || rep.Failure == "" {
		t.Fatalf("a sweep of a missing container passed: %+v", rep)
	}
	if strings.Contains(rep.Failure, sweepMarker) || strings.Contains(rep.Failure, dir) {
		t.Fatalf("sweep failure text names a path: %q", rep.Failure)
	}
}

// outsideUntouched checks the host-owned directory next to the container that
// fakeContainer made: same mode, same file, same content.
func outsideUntouched(t *testing.T, outside string) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(outside), "outside-dir")
	if fi, err := os.Lstat(dir); err != nil || fi.Mode().Perm() != 0o750 {
		t.Fatalf("the sweep changed a directory outside the container: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "keep")); err != nil || string(b) != "host dir file" {
		t.Fatalf("the sweep deleted outside the container: %v", err)
	}
}

// SEC-001: a process still running in the container swaps a directory for a
// symlink to an outside directory between the sweep's look at the entry and
// its next step. The sweep must not follow it, on either the removal path (a
// directory the job added) or the restore path (a baseline directory).
func TestContainerSweepIgnoresASwappedDirectory(t *testing.T) {
	for _, tc := range []struct {
		name, rel string
		closed    bool // the worker made the directory unreadable first
	}{
		{"added directory", "job/a", false},
		{"baseline directory", "Library/Caches", false},
		{"unreadable added directory", "job/a", true},
		{"unreadable baseline directory", "Library/Caches", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, outside := fakeContainer(t)
			c, err := NewContainer(root, testLock(t))
			if err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(root, "job", "a", "b")
			if err := os.MkdirAll(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(nested, "spill"), []byte(sweepMarker), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.closed {
				if err := os.Chmod(filepath.Join(root, tc.rel), 0); err != nil {
					t.Fatal(err)
				}
			}
			outsideDir := filepath.Join(filepath.Dir(outside), "outside-dir")
			swapped := false
			c.beforeChange = func(rel string) {
				if rel != tc.rel || swapped {
					return
				}
				swapped = true
				p := filepath.Join(root, rel)
				if err := os.Rename(p, p+"-moved"); err != nil {
					t.Error(err)
				}
				if err := os.Symlink(outsideDir, p); err != nil {
					t.Error(err)
				}
			}
			rep := c.Sweep()
			if !swapped {
				t.Fatal("the swap never ran")
			}
			outsideUntouched(t, outside)
			if rep.Clean {
				t.Fatalf("a swapped entry left the container clean: %+v", rep)
			}
			if strings.Contains(rep.Failure, sweepMarker) || strings.Contains(rep.Failure, root) {
				t.Fatalf("failure text names a path: %q", rep.Failure)
			}
		})
	}
}

// SEC-001: the worker replaces a baseline file with a hard link to a file
// outside the container, with another mode and an extra attribute. The sweep
// must report the change and leave the outside file as it was.
func TestContainerSweepLeavesAHardLinkedOutsideFile(t *testing.T) {
	root, outside := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outside, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lsetxattr(outside, "org.gopmgr.host", []byte("host"), 0); err != nil {
		t.Fatal(err)
	}
	pref := filepath.Join(root, "Library", "pref")
	if err := os.Remove(pref); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, pref); err != nil {
		t.Fatal(err)
	}
	rep := c.Sweep()
	if rep.Clean || rep.BaselineChanged == 0 {
		t.Fatalf("a replaced baseline file was not reported: %+v", rep)
	}
	fi, err := os.Lstat(outside)
	if err != nil || fi.Mode().Perm() != 0o640 {
		t.Fatalf("the sweep changed the outside file's mode: %v %v", fi.Mode(), err)
	}
	if size, err := unix.Lgetxattr(outside, "org.gopmgr.host", nil); err != nil || size != 4 {
		t.Fatalf("the sweep removed the outside file's attribute: %v", err)
	}
}

// A file the worker made unreadable and immutable cannot be opened to clear
// its flag without resolving a path, so the sweep fails closed.
func TestContainerSweepFailsClosedOnAnUnreadableImmutableFile(t *testing.T) {
	root, _ := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "tmp", "locked-"+sweepMarker)
	if err := os.WriteFile(locked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	if err := unix.Chflags(locked, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	rep := c.Sweep()
	if rep.Clean || rep.Failure == "" || strings.Contains(rep.Failure, sweepMarker) {
		t.Fatalf("an unreadable immutable file did not fail the sweep closed: %+v", rep)
	}
	ran := false
	if res := c.Do("refused", func(string) Result { ran = true; return NewResult("refused") }); ran || res.Detail != ErrContainerUnclean.Error() {
		t.Fatalf("an unclean container accepted a job: %+v", res)
	}
}

// The worker shapes the tree, so the sweep bounds its depth and fails closed
// past the bound instead of recursing without limit.
func TestContainerSweepBoundsTreeDepth(t *testing.T) {
	root, _ := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "tmp")
	for range maxSweepDepth + 2 {
		deep = filepath.Join(deep, "d")
	}
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	rep := c.Sweep()
	if rep.Clean || rep.Failure != sweepLimit {
		t.Fatalf("a tree past the depth bound did not fail the sweep: %+v", rep)
	}
}

// SEC-001: the sweep must restore permissions on a directory the worker made
// unreadable before it can open it. A process still running in the container
// swaps that directory for a hard link to an outside file in between; the
// outside file's mode must not change.
func TestContainerSweepUnreadableDirectorySwappedForAHardLink(t *testing.T) {
	for _, tc := range []struct{ name, rel string }{
		{"added directory", "job/a"},
		{"baseline directory", "Library/Caches"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, outside := fakeContainer(t)
			c, err := NewContainer(root, testLock(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, "job", "a"), 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, tc.rel)
			if err := os.Chmod(target, 0); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(outside, 0o640); err != nil {
				t.Fatal(err)
			}
			swapped := false
			c.beforeChange = func(rel string) {
				if rel != tc.rel || swapped {
					return
				}
				swapped = true
				if err := os.Rename(target, target+"-moved"); err != nil {
					t.Error(err)
				}
				if err := os.Link(outside, target); err != nil {
					t.Error(err)
				}
			}
			rep := c.Sweep()
			if !swapped {
				t.Fatal("the swap never ran")
			}
			if fi, err := os.Lstat(outside); err != nil || fi.Mode().Perm() != 0o640 {
				t.Fatalf("the sweep changed the outside file's mode through a hard link: %v %v", fi.Mode(), err)
			}
			if rep.Clean {
				t.Fatalf("a swapped entry left the container clean: %+v", rep)
			}
		})
	}
}

// A FIFO the worker made and flagged immutable must not hang the sweep while
// it holds the container lock.
func TestContainerSweepDoesNotBlockOnAFIFO(t *testing.T) {
	root, _ := fakeContainer(t)
	c, err := NewContainer(root, testLock(t))
	if err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "tmp", "pipe")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := unix.Chflags(fifo, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	done := make(chan SweepReport, 1)
	go func() { done <- c.Sweep() }()
	select {
	case rep := <-done:
		if !rep.Clean || rep.FlagsCleared != 1 {
			t.Fatalf("FIFO sweep: %+v", rep)
		}
	case <-time.After(10 * time.Second):
		// Unblock the sweep so the test binary can finish.
		if w, err := os.OpenFile(fifo, os.O_WRONLY, 0); err == nil {
			w.Close()
		}
		t.Fatal("the sweep blocked opening a FIFO")
	}
}
