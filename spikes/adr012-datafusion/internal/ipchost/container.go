// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// Container keeps a worker's writable directory (the App Sandbox container's
// Data directory) in the state recorded by its baseline. Each job runs with
// exclusive use of the directory in a fresh job folder. Afterwards Sweep
// removes everything the job added (entries, extended attributes, permission
// and flag changes) without following symlinks, then compares the result with
// the baseline. A container that cannot be restored is marked unclean and
// refuses further jobs.
//
// Every worker signed with the same bundle identifier shares one container,
// so jobs on a container run one at a time. Sweep reports carry counts only:
// names and attribute values may hold data the worker wrote.
type Container struct {
	mu      sync.Mutex
	root    string
	base    map[string]containerEntry
	unclean bool
}

type containerEntry struct {
	Mode   fs.FileMode       `json:"mode"`
	Link   string            `json:"link,omitempty"`
	Data   string            `json:"data,omitempty"`
	Xattrs map[string]string `json:"xattrs,omitempty"`
}

// SweepReport says what a sweep undid. It never names an entry.
type SweepReport struct {
	EntriesRemoved  int    `json:"entries_removed"`
	XattrsRemoved   int    `json:"xattrs_removed"`
	ModesRestored   int    `json:"modes_restored"`
	FlagsCleared    int    `json:"flags_cleared"`
	BaselineChanged int    `json:"baseline_entries_changed"`
	Clean           bool   `json:"clean"`
	Failure         string `json:"failure,omitempty"`
}

// ErrContainerUnclean is the setup failure for a job refused because an
// earlier sweep could not restore the container.
var ErrContainerUnclean = errors.New("worker container is not clean")

// NewContainer records the baseline of root, which must exist.
func NewContainer(root string) (*Container, error) {
	base, err := snapshotTree(root)
	if err != nil {
		return nil, fmt.Errorf("container baseline: %s", errnoText(err))
	}
	return &Container{root: root, base: base}, nil
}

type savedContainer struct {
	Root    string                    `json:"root"`
	Entries map[string]containerEntry `json:"entries"`
}

// Save writes the baseline to path (outside the container), so a host that
// restarts after dying mid-job can sweep what the job left.
func (c *Container) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.Marshal(savedContainer{c.root, c.base})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadContainer reads a baseline written by Save.
func LoadContainer(path string) (*Container, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s savedContainer
	if err := json.Unmarshal(b, &s); err != nil || s.Root == "" || s.Entries == nil {
		return nil, errors.New("container baseline is invalid")
	}
	return &Container{root: s.Root, base: s.Entries}, nil
}

// Do runs one job with exclusive use of the container. run receives a fresh,
// empty job folder. The sweep runs after run returns, which for Run means
// after the worker has been reaped, whatever the outcome.
func (c *Container) Do(name string, run func(folder string) Result) Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unclean {
		res := NewResult(name)
		res.Outcome, res.Detail = "setup_failed", ErrContainerUnclean.Error()
		return res
	}
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		res := NewResult(name)
		res.Outcome, res.Detail = "setup_failed", "job folder: entropy unavailable"
		return res
	}
	folder := filepath.Join(c.root, "df0-job-"+hex.EncodeToString(entropy[:]))
	var res Result
	if err := os.Mkdir(folder, 0o700); err != nil {
		res = NewResult(name)
		res.Outcome, res.Detail = "setup_failed", "job folder: "+errnoText(err)
	} else {
		res = run(folder)
	}
	report := c.sweepLocked()
	res.ContainerSweep = &report
	return res
}

// Sweep restores the container without running a job, for example when a
// host starts after a previous host died mid-job.
func (c *Container) Sweep() SweepReport {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sweepLocked()
}

func (c *Container) sweepLocked() SweepReport {
	var rep SweepReport
	fail := func(err error) {
		if rep.Failure == "" {
			rep.Failure = errnoText(err)
		}
	}
	seen := map[string]bool{}
	c.restore(".", seen, &rep, fail)
	for rel := range c.base {
		if !seen[rel] {
			rep.BaselineChanged++
		}
	}
	after, err := snapshotTree(c.root)
	if err != nil {
		fail(err)
	}
	rep.Clean = rep.Failure == "" && rep.BaselineChanged == 0 && err == nil && sameTree(after, c.base)
	c.unclean = !rep.Clean
	return rep
}

func (c *Container) restore(rel string, seen map[string]bool, rep *SweepReport, fail func(error)) {
	path := filepath.Join(c.root, rel)
	fi, err := os.Lstat(path)
	if err != nil {
		fail(err)
		return
	}
	b, inBase := c.base[rel]
	if !inBase {
		removeEntry(path, rep, fail)
		return
	}
	seen[rel] = true
	if fi.Mode().Type() != b.Mode.Type() {
		rep.BaselineChanged++
		return
	}
	if fi.Mode().Type() == fs.ModeSymlink {
		if link, err := os.Readlink(path); err != nil || link != b.Link {
			rep.BaselineChanged++
		}
	} else {
		if cleared, err := clearUserFlags(path, fi); err != nil {
			fail(err)
		} else if cleared {
			rep.FlagsCleared++
		}
		if fi.Mode().Perm() != b.Mode.Perm() {
			// Lstat shows a non-symlink and no worker is running, so chmod
			// cannot be redirected through a link.
			if err := os.Chmod(path, b.Mode.Perm()); err != nil {
				fail(err)
			} else {
				rep.ModesRestored++
			}
		}
		if fi.Mode().IsRegular() {
			if sum, err := fileDigest(path); err != nil || sum != b.Data {
				rep.BaselineChanged++
			}
		}
	}
	current, err := xattrDigests(path)
	if err != nil {
		fail(err)
	}
	for name, sum := range current {
		want, ok := b.Xattrs[name]
		switch {
		case !ok:
			if err := unix.Lremovexattr(path, name); err != nil {
				fail(err)
			} else {
				rep.XattrsRemoved++
			}
		case want != sum:
			rep.BaselineChanged++
		}
	}
	if !fi.IsDir() {
		return
	}
	children, err := os.ReadDir(path)
	if err != nil {
		fail(err)
		return
	}
	for _, child := range children {
		c.restore(filepath.Join(rel, child.Name()), seen, rep, fail)
	}
}

// removeEntry deletes path and everything under it. Symlinks are removed,
// never followed. Directories the worker made unreadable and files it made
// immutable are opened up first, since the host owns them.
func removeEntry(path string, rep *SweepReport, fail func(error)) {
	fi, err := os.Lstat(path)
	if err != nil {
		fail(err)
		return
	}
	if fi.Mode().Type() != fs.ModeSymlink {
		if cleared, err := clearUserFlags(path, fi); err != nil {
			fail(err)
		} else if cleared {
			rep.FlagsCleared++
		}
	}
	if fi.IsDir() {
		if fi.Mode().Perm()&0o700 != 0o700 {
			if err := os.Chmod(path, 0o700); err != nil {
				fail(err)
			}
		}
		children, err := os.ReadDir(path)
		if err != nil {
			fail(err)
		}
		for _, child := range children {
			removeEntry(filepath.Join(path, child.Name()), rep, fail)
		}
	}
	if err := os.Remove(path); err != nil {
		fail(err)
		return
	}
	rep.EntriesRemoved++
}

func snapshotTree(root string) (map[string]containerEntry, error) {
	out := map[string]containerEntry{}
	var walk func(rel string) error
	walk = func(rel string) error {
		path := filepath.Join(root, rel)
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		e := containerEntry{Mode: fi.Mode().Type() | fi.Mode().Perm()}
		switch {
		case fi.Mode().Type() == fs.ModeSymlink:
			if e.Link, err = os.Readlink(path); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if e.Data, err = fileDigest(path); err != nil {
				return err
			}
		}
		if e.Xattrs, err = xattrDigests(path); err != nil {
			return err
		}
		out[rel] = e
		if !fi.IsDir() {
			return nil
		}
		children, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(filepath.Join(rel, child.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, err
	}
	return out, nil
}

func sameTree(a, b map[string]containerEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for rel, x := range a {
		y, ok := b[rel]
		if !ok || x.Mode != y.Mode || x.Link != y.Link || x.Data != y.Data || len(x.Xattrs) != len(y.Xattrs) {
			return false
		}
		for name, sum := range x.Xattrs {
			if y.Xattrs[name] != sum {
				return false
			}
		}
	}
	return true
}

func fileDigest(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// xattrDigests returns each extended attribute name with a digest of its
// value, without following a symlink.
func xattrDigests(path string) (map[string]string, error) {
	size, err := unix.Llistxattr(path, nil)
	if err != nil || size == 0 {
		return nil, err
	}
	buf := make([]byte, size)
	n, err := unix.Llistxattr(path, buf)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, name := range strings.Split(strings.TrimRight(string(buf[:n]), "\x00"), "\x00") {
		vsize, err := unix.Lgetxattr(path, name, nil)
		if err != nil {
			return nil, err
		}
		value := make([]byte, vsize)
		if _, err := unix.Lgetxattr(path, name, value); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(value)
		out[name] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

// errnoText reports only the system error, never the path it was about:
// paths inside the container are names the worker chose.
func errnoText(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno.Error()
	}
	return "container sweep failed"
}
