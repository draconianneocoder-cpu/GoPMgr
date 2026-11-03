// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

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
// and flag changes), then compares the result with the baseline. A container
// that cannot be restored is marked unclean and refuses further jobs.
//
// The sweep never resolves a path below root. It walks directory handles
// opened without following symlinks, and it changes or removes entries only
// through those handles or through a handle on the entry itself. A process
// still running in the container therefore cannot redirect the sweep outside
// root by swapping an entry for a symlink, a hard link or a FIFO, or by
// renaming directories. The one change made by name, restoring permissions on
// a directory too closed to open, succeeds only on a directory: the name ends
// in a slash and no symlink may be resolved. A baseline entry is changed only
// after its inode matches the baseline on root's device, so a hard link to an
// outside file is reported, not modified. An entry the sweep cannot handle
// this way (for example a file the worker made unreadable and immutable)
// fails the sweep.
//
// Every worker signed with the same bundle identifier shares one container,
// so jobs on a container run one at a time, also across host processes: Do
// and Sweep hold an exclusive flock on the container's lock file (one per
// worker slot, in a folder only the host's user can reach) from before the
// job folder is made until the sweep ends. A host never queues on a slot
// another process holds; Do and Sweep refuse with ErrContainerBusy. Under
// the lock, a tree that differs from the baseline (a holder killed before it
// could sweep) is swept before the job runs. Sweep reports carry counts only:
// names and attribute values may hold data the worker wrote.
//
// The lock lasts as long as the host's descriptor. If a host dies while its
// worker still runs, the lock is released and another host may start a job
// beside the orphaned worker until that worker exits.
type Container struct {
	mu       sync.Mutex
	root     string
	lockPath string
	base     map[string]containerEntry
	unclean  bool
	// beforeChange, set only by tests, runs after the sweep has looked at the
	// entry at rel and before it opens, changes or removes it.
	beforeChange func(rel string)
}

// containerEntry records one entry. Device numbers can change across a
// reboot, so identity is the inode on root's current device.
type containerEntry struct {
	Mode   fs.FileMode       `json:"mode"`
	Ino    uint64            `json:"ino"`
	Link   string            `json:"link,omitempty"`
	Data   string            `json:"data,omitempty"`
	Xattrs map[string]string `json:"xattrs,omitempty"`
}

const (
	// maxSweepDepth and maxSweepEntries bound the host's work for a tree the
	// worker shapes. A tree beyond them fails the sweep.
	maxSweepDepth   = 64
	maxSweepEntries = 1 << 17

	dirOpen = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	// entryOpen opens any entry for metadata changes. O_SYMLINK opens a
	// symlink itself, so the last component is never followed either.
	// O_NONBLOCK keeps a FIFO from blocking the open.
	entryOpen = unix.O_EVTONLY | unix.O_SYMLINK | unix.O_NONBLOCK | unix.O_CLOEXEC
	// atSymlinkNofollowAny is AT_SYMLINK_NOFOLLOW_ANY from <sys/fcntl.h>,
	// which x/sys/unix does not define: no symlink anywhere in the path.
	atSymlinkNofollowAny = 0x0800
	userFlags            = unix.UF_IMMUTABLE | unix.UF_APPEND
	sweepLimit           = "container tree exceeds the sweep limits"
)

var (
	errSweepLimit = errors.New(sweepLimit)
	errEntryMoved = errors.New("container entry changed during the sweep")
)

// NewContainer records the baseline of root, which must exist. lockPath is
// the slot's lock file; see checkLockFolder for what its folder must be.
func NewContainer(root, lockPath string) (*Container, error) {
	if err := checkLockFolder(lockPath); err != nil {
		return nil, err
	}
	base, err := snapshotTree(root)
	if err != nil {
		return nil, fmt.Errorf("container baseline: %s", errnoText(err))
	}
	return &Container{root: root, lockPath: lockPath, base: base}, nil
}

// checkLockFolder requires lockPath's folder to be a directory (not a
// symlink) owned by this user with no group or other permissions, so no
// other user can remove or replace the lock file. The lock file is never
// removed: removing it while held would let a second holder lock a new file.
// The check runs once, at construction; a later change to the folder by the
// same user is outside it.
func checkLockFolder(lockPath string) error {
	if !filepath.IsAbs(lockPath) {
		return errors.New("container lock: path is not absolute")
	}
	var st unix.Stat_t
	if err := unix.Lstat(filepath.Dir(lockPath), &st); err != nil {
		return fmt.Errorf("container lock folder: %s", errnoText(err))
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR || int(st.Uid) != os.Geteuid() || st.Mode&0o077 != 0 {
		return errors.New("container lock folder must be a directory owned by this user with no group or other access")
	}
	return nil
}

// lockSlot takes the slot's exclusive lock without waiting. The returned
// function releases it. A lock held by another open file description, in
// this process or another, gives ErrContainerBusy.
func (c *Container) lockSlot() (func(), error) {
	fd, err := unix.Open(c.lockPath, unix.O_RDONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("container lock: %s", errnoText(err))
	}
	st, err := fstat(fd)
	if err == nil && (st.Mode&unix.S_IFMT != unix.S_IFREG || int(st.Uid) != os.Geteuid()) {
		err = errors.New("not a regular file owned by this user")
	}
	if err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if errors.Is(err, unix.EWOULDBLOCK) {
			unix.Close(fd)
			return nil, ErrContainerBusy
		}
	}
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("container lock: %s", errnoText(err))
	}
	return func() { unix.Close(fd) }, nil
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

// LoadContainer reads a baseline written by Save. lockPath is as for
// NewContainer.
func LoadContainer(path, lockPath string) (*Container, error) {
	if err := checkLockFolder(lockPath); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s savedContainer
	if err := json.Unmarshal(b, &s); err != nil || !filepath.IsAbs(s.Root) || s.Entries == nil {
		return nil, errors.New("container baseline is invalid")
	}
	for _, e := range s.Entries {
		if e.Ino == 0 {
			return nil, errors.New("container baseline is invalid")
		}
	}
	return &Container{root: s.Root, lockPath: lockPath, base: s.Entries}, nil
}

// Do runs one job with exclusive use of the container. run receives a fresh,
// empty job folder. The sweep runs after run returns, which for Run means
// after the worker has been reaped, whatever the outcome. The slot lock is
// held throughout; a container left changed by an earlier holder is swept
// first, and the job is refused if that sweep is not clean.
func (c *Container) Do(name string, run func(folder string) Result) Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	refuse := func(detail string) Result {
		res := NewResult(name)
		res.Outcome, res.Detail = "setup_failed", detail
		return res
	}
	if c.unclean {
		return refuse(ErrContainerUnclean.Error())
	}
	unlock, err := c.lockSlot()
	if err != nil {
		return refuse(err.Error())
	}
	defer unlock()
	var pre *SweepReport
	if now, err := snapshotTree(c.root); err != nil || !sameTree(now, c.base) {
		report := c.sweepLocked()
		pre = &report
		if !report.Clean {
			res := refuse(ErrContainerUnclean.Error())
			res.ContainerPreSweep = pre
			return res
		}
	}
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		res := refuse("job folder: entropy unavailable")
		res.ContainerPreSweep = pre
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
	res.ContainerPreSweep = pre
	res.ContainerSweep = &report
	return res
}

// Sweep restores the container without running a job, for example when a
// host starts after a previous host died mid-job. A slot another process
// holds is not swept; the report says so and the container is not marked
// unclean.
func (c *Container) Sweep() SweepReport {
	c.mu.Lock()
	defer c.mu.Unlock()
	unlock, err := c.lockSlot()
	if err != nil {
		return SweepReport{Failure: err.Error()}
	}
	defer unlock()
	return c.sweepLocked()
}

// sweep is the state of one sweepLocked call.
type sweep struct {
	c       *Container
	dev     int32 // root's device
	rep     SweepReport
	seen    map[string]bool
	entries int
}

func (s *sweep) fail(err error) {
	if s.rep.Failure == "" {
		s.rep.Failure = errnoText(err)
	}
}

// visit counts one entry and reports whether the walk may go on.
func (s *sweep) visit(depth int) bool {
	s.entries++
	if depth > maxSweepDepth || s.entries > maxSweepEntries {
		s.fail(errSweepLimit)
		return false
	}
	return true
}

func (s *sweep) hook(rel string) {
	if s.c.beforeChange != nil {
		s.c.beforeChange(rel)
	}
}

func (c *Container) sweepLocked() SweepReport {
	s := &sweep{c: c, seen: map[string]bool{}}
	if root, err := unix.Open(c.root, dirOpen, 0); err != nil {
		s.fail(err)
	} else {
		if st, err := fstat(root); err != nil {
			s.fail(err)
		} else {
			s.dev = st.Dev
			s.restoreDir(root, ".", c.base["."], 0)
		}
		unix.Close(root)
	}
	for rel := range c.base {
		if !s.seen[rel] {
			s.rep.BaselineChanged++
		}
	}
	after, err := snapshotTree(c.root)
	if err != nil {
		s.fail(err)
	}
	s.rep.Clean = s.rep.Failure == "" && s.rep.BaselineChanged == 0 && err == nil && sameTree(after, c.base)
	c.unclean = !s.rep.Clean
	return s.rep
}

// restoreDir restores the baseline directory open as fd, then everything
// in it. The caller closes fd.
func (s *sweep) restoreDir(fd int, rel string, b containerEntry, depth int) {
	st, err := fstat(fd)
	if err != nil {
		s.fail(err)
		return
	}
	if !sameIdentity(st, b, s.dev) {
		s.rep.BaselineChanged++
		return
	}
	s.seen[rel] = true
	s.restoreMeta(fd, st, b)
	names, err := dirNames(fd)
	if err != nil {
		s.fail(err)
		return
	}
	for _, name := range names {
		s.entry(fd, name, filepath.Join(rel, name), depth+1)
	}
}

// entry restores or removes the entry name in the directory open as dirfd.
func (s *sweep) entry(dirfd int, name, rel string, depth int) {
	if !s.visit(depth) {
		return
	}
	st, err := lstatAt(dirfd, name)
	if err != nil {
		s.fail(err)
		return
	}
	s.hook(rel)
	b, inBase := s.c.base[rel]
	if !inBase {
		s.remove(dirfd, name, rel, st, depth)
		return
	}
	if fileMode(st).Type() != b.Mode.Type() || !sameIdentity(st, b, s.dev) {
		s.rep.BaselineChanged++
		return
	}
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		if st.Mode&0o500 != 0o500 {
			mode := b.Mode.Perm() | 0o500
			if err := chmodDirAt(dirfd, name, mode); err != nil {
				s.fail(err)
				return
			}
			if mode == b.Mode.Perm() {
				s.rep.ModesRestored++
			}
		}
		fd, err := unix.Openat(dirfd, name, dirOpen, 0)
		if err != nil {
			s.fail(err)
			return
		}
		defer unix.Close(fd)
		s.restoreDir(fd, rel, b, depth)
		return
	}
	s.seen[rel] = true
	fd, err := unix.Openat(dirfd, name, entryOpen, 0)
	if err != nil {
		s.fail(err)
		return
	}
	defer unix.Close(fd)
	cur, err := fstat(fd)
	if err != nil {
		s.fail(err)
		return
	}
	if !sameIdentity(cur, b, s.dev) {
		s.rep.BaselineChanged++
		return
	}
	s.restoreMeta(fd, cur, b)
	switch cur.Mode & unix.S_IFMT {
	case unix.S_IFLNK:
		if link, err := readlinkAt(dirfd, name); err != nil || link != b.Link {
			s.rep.BaselineChanged++
		}
	case unix.S_IFREG:
		if sum, err := digestAt(dirfd, name, b, s.dev); err != nil || sum != b.Data {
			s.rep.BaselineChanged++
		}
	}
}

// restoreMeta clears user flags, restores the permission bits and removes
// added extended attributes on the baseline entry open as fd, whose identity
// the caller has checked.
func (s *sweep) restoreMeta(fd int, st unix.Stat_t, b containerEntry) {
	if st.Flags&userFlags != 0 {
		if err := unix.Fchflags(fd, int(st.Flags&^userFlags)); err != nil {
			s.fail(err)
		} else {
			s.rep.FlagsCleared++
		}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFLNK && fs.FileMode(st.Mode&0o777) != b.Mode.Perm() {
		if err := unix.Fchmod(fd, uint32(b.Mode.Perm())); err != nil {
			s.fail(err)
		} else {
			s.rep.ModesRestored++
		}
	}
	current, err := xattrDigests(fd)
	if err != nil {
		s.fail(err)
	}
	for name, sum := range current {
		want, ok := b.Xattrs[name]
		switch {
		case !ok:
			if err := unix.Fremovexattr(fd, name); err != nil {
				s.fail(err)
			} else {
				s.rep.XattrsRemoved++
			}
		case want != sum:
			s.rep.BaselineChanged++
		}
	}
}

// remove deletes the entry name, which the worker added, and everything
// under it. Directories the worker made unreadable are opened up and flags
// cleared first; a regular file is never chmod-ed, since it may be a hard
// link to a file outside root.
func (s *sweep) remove(dirfd int, name, rel string, st unix.Stat_t, depth int) {
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		s.removeDir(dirfd, name, rel, st, depth)
		return
	}
	if st.Flags&userFlags != 0 {
		fd, err := unix.Openat(dirfd, name, entryOpen, 0)
		if err != nil {
			s.fail(err)
			return
		}
		cur, err := fstat(fd)
		if err == nil && (cur.Dev != st.Dev || cur.Ino != st.Ino) {
			err = errEntryMoved
		}
		if err == nil {
			err = unix.Fchflags(fd, int(cur.Flags&^userFlags))
		}
		unix.Close(fd)
		if err != nil {
			s.fail(err)
			return
		}
		s.rep.FlagsCleared++
	}
	if err := unix.Unlinkat(dirfd, name, 0); err != nil {
		s.fail(err)
		return
	}
	s.rep.EntriesRemoved++
}

func (s *sweep) removeDir(dirfd int, name, rel string, st unix.Stat_t, depth int) {
	if st.Mode&0o500 != 0o500 {
		if err := chmodDirAt(dirfd, name, 0o700); err != nil {
			s.fail(err)
			return
		}
	}
	fd, err := unix.Openat(dirfd, name, dirOpen, 0)
	if err != nil {
		s.fail(err)
		return
	}
	cur, err := fstat(fd)
	if err == nil && (cur.Dev != st.Dev || cur.Ino != st.Ino) {
		err = errEntryMoved
	}
	if err == nil && cur.Flags&userFlags != 0 {
		if err = unix.Fchflags(fd, int(cur.Flags&^userFlags)); err == nil {
			s.rep.FlagsCleared++
		}
	}
	if err == nil && cur.Mode&0o700 != 0o700 {
		err = unix.Fchmod(fd, 0o700)
	}
	var names []string
	if err == nil {
		names, err = dirNames(fd)
	}
	if err != nil {
		unix.Close(fd)
		s.fail(err)
		return
	}
	for _, child := range names {
		if !s.visit(depth + 1) {
			break
		}
		cst, err := lstatAt(fd, child)
		if err != nil {
			s.fail(err)
			continue
		}
		crel := filepath.Join(rel, child)
		s.hook(crel)
		s.remove(fd, child, crel, cst, depth+1)
	}
	unix.Close(fd)
	if err := unix.Unlinkat(dirfd, name, unix.AT_REMOVEDIR); err != nil {
		s.fail(err)
		return
	}
	s.rep.EntriesRemoved++
}

// snapshotTree records root's tree through directory handles, without
// following symlinks.
func snapshotTree(root string) (map[string]containerEntry, error) {
	fd, err := unix.Open(root, dirOpen, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	st, err := fstat(fd)
	if err != nil {
		return nil, err
	}
	out := map[string]containerEntry{}
	count := 0
	return out, snapshotDir(fd, ".", st, st.Dev, out, 0, &count)
}

func snapshotDir(fd int, rel string, st unix.Stat_t, dev int32, out map[string]containerEntry, depth int, count *int) error {
	e := entryFor(st)
	var err error
	if e.Xattrs, err = xattrDigests(fd); err != nil {
		return err
	}
	out[rel] = e
	names, err := dirNames(fd)
	if err != nil {
		return err
	}
	for _, name := range names {
		*count++
		if depth+1 > maxSweepDepth || *count > maxSweepEntries {
			return errSweepLimit
		}
		if err := snapshotEntry(fd, name, filepath.Join(rel, name), dev, out, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func snapshotEntry(dirfd int, name, rel string, dev int32, out map[string]containerEntry, depth int, count *int) error {
	st, err := lstatAt(dirfd, name)
	if err != nil {
		return err
	}
	if st.Dev != dev {
		return errEntryMoved
	}
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		fd, err := unix.Openat(dirfd, name, dirOpen, 0)
		if err != nil {
			return err
		}
		defer unix.Close(fd)
		cur, err := fstat(fd)
		if err != nil {
			return err
		}
		if cur.Dev != st.Dev || cur.Ino != st.Ino {
			return errEntryMoved
		}
		return snapshotDir(fd, rel, cur, dev, out, depth, count)
	}
	e := entryFor(st)
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFLNK:
		if e.Link, err = readlinkAt(dirfd, name); err != nil {
			return err
		}
	case unix.S_IFREG:
		if e.Data, err = digestAt(dirfd, name, e, dev); err != nil {
			return err
		}
	}
	fd, err := unix.Openat(dirfd, name, entryOpen, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	cur, err := fstat(fd)
	if err != nil {
		return err
	}
	if !sameIdentity(cur, e, dev) {
		return errEntryMoved
	}
	if e.Xattrs, err = xattrDigests(fd); err != nil {
		return err
	}
	out[rel] = e
	return nil
}

func sameTree(a, b map[string]containerEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for rel, x := range a {
		y, ok := b[rel]
		if !ok || x.Mode != y.Mode || x.Ino != y.Ino || x.Link != y.Link || x.Data != y.Data || len(x.Xattrs) != len(y.Xattrs) {
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

func entryFor(st unix.Stat_t) containerEntry {
	return containerEntry{Mode: fileMode(st), Ino: st.Ino}
}

// sameIdentity reports whether st is the recorded entry e on root's device.
func sameIdentity(st unix.Stat_t, e containerEntry, dev int32) bool {
	return st.Dev == dev && st.Ino == e.Ino
}

// chmodDirAt sets the permissions of the directory name. The trailing slash
// makes anything but a directory fail (a hard link to a file is refused with
// ENOTDIR), and atSymlinkNofollowAny refuses a symlink, which the slash would
// otherwise resolve.
func chmodDirAt(dirfd int, name string, mode fs.FileMode) error {
	return unix.Fchmodat(dirfd, name+"/", uint32(mode), atSymlinkNofollowAny)
}

func fileMode(st unix.Stat_t) fs.FileMode {
	m := fs.FileMode(st.Mode & 0o777)
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		m |= fs.ModeDir
	case unix.S_IFLNK:
		m |= fs.ModeSymlink
	case unix.S_IFIFO:
		m |= fs.ModeNamedPipe
	case unix.S_IFSOCK:
		m |= fs.ModeSocket
	case unix.S_IFCHR:
		m |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFBLK:
		m |= fs.ModeDevice
	}
	return m
}

func fstat(fd int) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := unix.Fstat(fd, &st)
	return st, err
}

func lstatAt(dirfd int, name string) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	return st, err
}

// dirNames lists the directory open as fd without consuming fd.
func dirNames(fd int) ([]string, error) {
	dup, err := unix.Dup(fd)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(dup), "")
	defer f.Close()
	return f.Readdirnames(-1)
}

func readlinkAt(dirfd int, name string) (string, error) {
	buf := make([]byte, unix.PathMax)
	n, err := unix.Readlinkat(dirfd, name, buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

// digestAt hashes the regular file name, which must still be the entry e.
// O_NONBLOCK keeps a FIFO swapped in from blocking the open.
func digestAt(dirfd int, name string, e containerEntry, dev int32) (string, error) {
	fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	f := os.NewFile(uintptr(fd), "")
	defer f.Close()
	st, err := fstat(fd)
	if err != nil {
		return "", err
	}
	if !sameIdentity(st, e, dev) {
		return "", errEntryMoved
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// xattrDigests returns each extended attribute name of the entry open as fd
// with a digest of its value.
func xattrDigests(fd int) (map[string]string, error) {
	size, err := unix.Flistxattr(fd, nil)
	if err != nil || size == 0 {
		return nil, err
	}
	buf := make([]byte, size)
	n, err := unix.Flistxattr(fd, buf)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, name := range strings.Split(strings.TrimRight(string(buf[:n]), "\x00"), "\x00") {
		vsize, err := unix.Fgetxattr(fd, name, nil)
		if err != nil {
			return nil, err
		}
		value := make([]byte, vsize)
		if _, err := unix.Fgetxattr(fd, name, value); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(value)
		out[name] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

// errnoText reports only the system error or a constant sweep error, never
// the path it was about: paths inside the container are names the worker
// chose.
func errnoText(err error) string {
	var errno syscall.Errno
	switch {
	case errors.As(err, &errno):
		return errno.Error()
	case errors.Is(err, errSweepLimit), errors.Is(err, errEntryMoved):
		return err.Error()
	}
	return "container sweep failed"
}
