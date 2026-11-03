// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package ipchost

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// slotHolderFlag starts the test binary as a second host process that holds
// a container slot. Usage: -df0-slot-holder MODE BASELINE LOCK. It prints one
// line once it holds the slot, then waits for its stdin to close.
//
//	run:   holds the slot inside a job, after writing a file in the job folder
//	sweep: holds the slot inside the job's sweep, before the first change
const slotHolderFlag = "-df0-slot-holder"

func slotHolderFixture(args []string) (int, bool) {
	if len(args) != 5 || args[1] != slotHolderFlag {
		return 0, false
	}
	mode, baseline, lock := args[2], args[3], args[4]
	c, err := LoadContainer(baseline, lock)
	if err != nil {
		return 30, true
	}
	wait := func(line string) {
		os.Stdout.WriteString(line + "\n")
		_, _ = io.Copy(io.Discard, os.Stdin)
	}
	var once sync.Once
	if mode == "sweep" {
		c.beforeChange = func(string) { once.Do(func() { wait("sweeping") }) }
	}
	res := c.Do("holder", func(folder string) Result {
		if err := os.WriteFile(filepath.Join(folder, "live"), []byte(sweepMarker), 0o600); err != nil {
			return NewResult("holder")
		}
		if mode == "run" {
			wait("holding " + folder)
		}
		return NewResult("holder")
	})
	if res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		return 31, true
	}
	return 0, true
}

type slotHolder struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	line  string
}

// startSlotHolder runs a holder process and returns once it reports that it
// holds the slot.
func startSlotHolder(t *testing.T, mode, baseline, lock string) *slotHolder {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, slotHolderFlag, mode, baseline, lock)
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
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	lines := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(stdout)
		if s.Scan() {
			lines <- s.Text()
		}
		close(lines)
	}()
	select {
	case line, ok := <-lines:
		if !ok {
			t.Fatalf("slot holder %s exited before holding the slot: %v", mode, cmd.Wait())
		}
		return &slotHolder{cmd: cmd, stdin: stdin, line: line}
	case <-time.After(15 * time.Second):
		t.Fatalf("slot holder %s did not take the slot", mode)
	}
	return nil
}

// savedSlot makes a container, saves its baseline the way a host does
// before others load it, and returns the host's own Container.
func savedSlot(t *testing.T) (c *Container, root, baseline, lock string) {
	t.Helper()
	root, _ = fakeContainer(t)
	lock = testLock(t)
	c, err := NewContainer(root, lock)
	if err != nil {
		t.Fatal(err)
	}
	baseline = filepath.Join(t.TempDir(), "baseline.json")
	if err := c.Save(baseline); err != nil {
		t.Fatal(err)
	}
	return c, root, baseline, lock
}

// assertRefusedBusy checks that a job and a sweep on c are refused because
// another process holds the slot, without running the job, sweeping or
// marking c unclean.
func assertRefusedBusy(t *testing.T, c *Container) {
	t.Helper()
	ran := false
	res := c.Do("other", func(string) Result { ran = true; return NewResult("other") })
	if ran || res.Outcome != "setup_failed" || res.Detail != ErrContainerBusy.Error() || res.ContainerSweep != nil || res.ContainerPreSweep != nil {
		t.Fatalf("a job on a held slot: ran=%t %+v", ran, res)
	}
	if rep := c.Sweep(); rep.Clean || rep.Failure != ErrContainerBusy.Error() || rep.EntriesRemoved != 0 {
		t.Fatalf("a sweep of a held slot: %+v", rep)
	}
	if c.unclean {
		t.Fatal("a held slot marked the container unclean")
	}
}

// Two host processes share a container when their workers share a bundle
// identifier. While one holds the slot, the other must neither run a job nor
// sweep away the holder's job folder. When the holder is killed before its
// own sweep, the next job first restores what it left.
func TestContainerSlotRefusesAnotherProcessThenRecoversAfterItsDeath(t *testing.T) {
	c, root, baseline, lock := savedSlot(t)
	h := startSlotHolder(t, "run", baseline, lock)
	folder, ok := strings.CutPrefix(h.line, "holding ")
	if !ok || filepath.Dir(folder) != root {
		t.Fatalf("holder reported %q", h.line)
	}
	assertRefusedBusy(t, c)
	if _, err := os.Lstat(filepath.Join(folder, "live")); err != nil {
		t.Fatalf("the holder's live job file is gone: %v", err)
	}

	if err := h.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = h.cmd.Wait()
	res := c.Do("after", func(own string) Result {
		if _, err := os.Lstat(folder); !os.IsNotExist(err) {
			t.Errorf("the dead holder's job folder was still there when the next job ran: %v", err)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "df0-job-") && filepath.Join(root, e.Name()) != own {
				t.Errorf("another job folder is present beside this job's")
			}
		}
		return NewResult("after")
	})
	pre := res.ContainerPreSweep
	if res.Outcome == "setup_failed" || pre == nil || !pre.Clean || pre.EntriesRemoved < 2 {
		t.Fatalf("the next job did not first restore the dead holder's leftovers: %+v pre=%+v", res, pre)
	}
	if res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		t.Fatalf("sweep after the job: %+v", res.ContainerSweep)
	}
	// A container already at its baseline is not swept first.
	if res := c.Do("again", func(string) Result { return NewResult("again") }); res.ContainerPreSweep != nil {
		t.Fatalf("a clean container was swept before the job: %+v", res.ContainerPreSweep)
	}
}

// The slot stays held until the holder's sweep has finished, not only while
// its job runs.
func TestContainerSlotIsHeldThroughTheHoldersSweep(t *testing.T) {
	c, _, baseline, lock := savedSlot(t)
	h := startSlotHolder(t, "sweep", baseline, lock)
	if h.line != "sweeping" {
		t.Fatalf("holder reported %q", h.line)
	}
	assertRefusedBusy(t, c)
	if err := h.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.cmd.Wait(); err != nil {
		t.Fatalf("holder: %v", err)
	}
	res := c.Do("after", func(string) Result { return NewResult("after") })
	if res.Outcome == "setup_failed" || res.ContainerPreSweep != nil || res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		t.Fatalf("job after the holder swept: %+v pre=%+v", res, res.ContainerPreSweep)
	}
}

// Two Container values in one process lock through separate open files, so
// they exclude each other as two processes do. A per-process lock (fcntl)
// would let both in.
func TestContainerSlotExcludesASecondContainerInTheSameProcess(t *testing.T) {
	first, root, _, lock := savedSlot(t)
	second, err := NewContainer(root, lock)
	if err != nil {
		t.Fatal(err)
	}
	holding, release, done := make(chan struct{}), make(chan struct{}), make(chan Result)
	go func() {
		done <- first.Do("first", func(string) Result {
			close(holding)
			<-release
			return NewResult("first")
		})
	}()
	<-holding
	assertRefusedBusy(t, second)
	close(release)
	if res := <-done; res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		t.Fatalf("first job: %+v", res.ContainerSweep)
	}
	if res := second.Do("second", func(string) Result { return NewResult("second") }); res.Outcome == "setup_failed" {
		t.Fatalf("the slot was not released: %+v", res)
	}
}

// The lock folder must keep other users from replacing the lock file, and
// the lock file must be a regular file reached without a symlink. Anything
// else fails closed without running, sweeping, hanging or marking unclean.
func TestContainerSlotLockMustBeAPrivateRegularFile(t *testing.T) {
	root, _ := fakeContainer(t)
	t.Run("folder_open_to_the_group", func(t *testing.T) {
		lock := testLock(t)
		if err := os.Chmod(filepath.Dir(lock), 0o750); err != nil {
			t.Fatal(err)
		}
		if _, err := NewContainer(root, lock); err == nil {
			t.Fatal("a group-readable lock folder was accepted")
		}
	})
	t.Run("folder_is_a_symlink", func(t *testing.T) {
		real := filepath.Dir(testLock(t))
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		if _, err := NewContainer(root, filepath.Join(link, "slot.lock")); err == nil {
			t.Fatal("a symlinked lock folder was accepted")
		}
	})
	t.Run("relative_path", func(t *testing.T) {
		if _, err := NewContainer(root, "slot.lock"); err == nil {
			t.Fatal("a relative lock path was accepted")
		}
	})
	for _, kind := range []string{"symlink", "fifo", "directory"} {
		t.Run("lock_file_is_a_"+kind, func(t *testing.T) {
			lock := testLock(t)
			c, err := NewContainer(root, lock)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				target := filepath.Join(filepath.Dir(lock), "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(target, lock)
			case "fifo":
				err = unix.Mkfifo(lock, 0o600)
			case "directory":
				err = os.Mkdir(lock, 0o700)
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan Result, 1)
			ran := false
			go func() { done <- c.Do("bad-lock", func(string) Result { ran = true; return NewResult("bad-lock") }) }()
			var res Result
			select {
			case res = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("opening the lock hung")
			}
			if ran || res.Outcome != "setup_failed" || !strings.HasPrefix(res.Detail, "container lock: ") || res.ContainerSweep != nil {
				t.Fatalf("job with a %s lock: ran=%t %+v", kind, ran, res)
			}
			if rep := c.Sweep(); rep.Clean || !strings.HasPrefix(rep.Failure, "container lock: ") {
				t.Fatalf("sweep with a %s lock: %+v", kind, rep)
			}
			if c.unclean {
				t.Fatal("a bad lock marked the container unclean")
			}
		})
	}
}

// The lock's descriptor must not pass to processes the job starts, such as
// the worker: a child that outlives the job would otherwise keep the slot
// locked, and would hold a descriptor from outside its container.
func TestContainerSlotLockIsNotInheritedByTheJobsChildren(t *testing.T) {
	first, root, _, lock := savedSlot(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	res := first.Do("spawns", func(string) Result {
		child := exec.Command(self, treeSleeperFlag)
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = child.Process.Kill()
			_ = child.Wait()
		})
		return NewResult("spawns")
	})
	if res.ContainerSweep == nil || !res.ContainerSweep.Clean {
		t.Fatalf("job that started a child: %+v", res.ContainerSweep)
	}
	second, err := NewContainer(root, lock)
	if err != nil {
		t.Fatal(err)
	}
	if res := second.Do("after", func(string) Result { return NewResult("after") }); res.Outcome == "setup_failed" {
		t.Fatalf("a child the job started kept the slot locked: %+v", res)
	}
}
