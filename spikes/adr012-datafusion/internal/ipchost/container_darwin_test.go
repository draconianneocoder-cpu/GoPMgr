// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"encoding/json"
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
	// after a failure mid-test.
	t.Cleanup(func() {
		_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && fi.Mode().Type() != os.ModeSymlink {
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
	c, err := NewContainer(root)
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
			c, err := NewContainer(root)
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
	c, err := NewContainer(root)
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
	restarted, err := LoadContainer(saved)
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
	c, err := NewContainer(root)
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
	c, err := NewContainer(root)
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
