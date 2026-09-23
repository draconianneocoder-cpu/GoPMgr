// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gopmgr/internal/db"
)

// TestPathTakingIPCMethodsConfineToOwnProjectsDir locks in F-1 from the
// 2026-06-29 security review: every IPC method that opens, mutates, or
// archives a project by a frontend-supplied path must reject paths outside
// the signed-in user's own projects folder, exactly as DeleteProject and
// CloneProject already do via projectPathFor. A regression here would hand a
// logged-in GoPMgr user a filesystem primitive over another user's files
// within the same OS account.
func TestPathTakingIPCMethodsConfineToOwnProjectsDir(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// A plausible target outside the user's projects sandbox: a sibling path
	// under the data root that a path-traversal attempt would aim for.
	outside := filepath.Join(t.TempDir(), "victim.pmforge")
	if d, err := db.InitDB(outside); err != nil {
		t.Fatalf("seed outside file: %v", err)
	} else if err := d.Close(); err != nil {
		t.Fatalf("close outside file: %v", err)
	}

	t.Run("OpenProject", func(t *testing.T) {
		if _, err := app.OpenProject(outside); err == nil {
			t.Fatal("OpenProject accepted a path outside the user's projects dir")
		}
	})
	t.Run("IsProjectEncrypted", func(t *testing.T) {
		if _, err := app.IsProjectEncrypted(outside); err == nil {
			t.Fatal("IsProjectEncrypted accepted a path outside the user's projects dir")
		}
	})
	t.Run("EncryptProjectAtRest", func(t *testing.T) {
		if _, err := app.EncryptProjectAtRest(outside); err == nil {
			t.Fatal("EncryptProjectAtRest accepted a path outside the user's projects dir")
		}
	})
	t.Run("SecureArchive", func(t *testing.T) {
		if _, err := app.SecureArchive(outside); err == nil {
			t.Fatal("SecureArchive accepted a path outside the user's projects dir")
		}
	})
}

// TestEncryptedDSNRejectsAmbiguousPath locks in F-2: a project path containing
// a DSN-significant character ('?' or '#') must be refused rather than folded
// into the SQLCipher DSN, where it could inject or override _pragma_* options
// (including the key).
func TestEncryptedDSNRejectsAmbiguousPath(t *testing.T) {
	dek := make([]byte, 32)
	for _, p := range []string{
		filepath.Join(t.TempDir(), "weird?_pragma_key=x'00'.pmforge"),
		filepath.Join(t.TempDir(), "frag#.pmforge"),
	} {
		if _, err := db.InitEncryptedDB(p, dek); err == nil {
			t.Fatalf("InitEncryptedDB accepted DSN-ambiguous path %q", p)
		}
	}
}

// TestProjectPathForAcceptsCurrentAndLegacyExtension pins the extension
// check in projectPathFor to accept BOTH ".gopmgr" (written by this build)
// and ".pmforge" (written before the 2026-08-04 PMForge -> GoPMgr rename,
// and never rewritten — nothing migrates existing project files). The
// extension is a persistence boundary, not a validation nicety: every
// project file already on a user's disk ends in one of these two, so if
// support for either were ever dropped, DeleteProject/CloneProject/
// OpenProject would reject real existing projects outright.
//
// This test replaces TestProjectPathForRejectsWrongExtension, which
// asserted ".gopmgr" was *rejected* — correct before this rename, inverted
// deliberately here now that ".gopmgr" is the current extension.
//
// This calls projectPathFor directly rather than going through DeleteProject:
// DeleteProject also opens the target as an encrypted SQLite database for
// its audit-log entry, so a fake (non-database) file fails there regardless
// of extension, which would make the assertion pass for the wrong reason
// even if the extension check itself were broken or renamed.
func TestProjectPathForAcceptsCurrentAndLegacyExtension(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	user := app.CurrentUser()
	if user == nil {
		t.Fatal("CurrentUser is nil after CreateAccount")
		return
	}

	// projectPathFor checks the extension before it looks at the filesystem,
	// so the wrong-extension paths need not exist. They live inside the
	// user's own projects directory so the test isolates the extension check
	// from the directory-confinement check covered above. ".pmforg"/".gopmg" are
	// deliberate one-character-off typos of the two valid extensions, not
	// an arbitrary wrong extension like ".txt": they catch a check that was
	// narrowed or corrupted by one character, which a generic case would
	// miss.
	projectsDir := filepath.Join(user.DataDir, "projects")
	for _, ext := range []string{".txt", ".pmforg", ".gopmg"} {
		wrongExt := filepath.Join(projectsDir, "not-a-project"+ext)
		if _, _, err := app.projectPathFor(wrongExt); err == nil {
			t.Fatalf("projectPathFor accepted a path with extension %q, want only .gopmgr/.pmforge accepted", ext)
		}
	}

	// Positive controls: both the current and legacy extension, in the same
	// directory, must be accepted. Without these, a check that rejected
	// every path (not just wrong extensions) would also pass the loop above.
	// projectPathFor requires an existing regular file, so create them.
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".gopmgr", ".pmforge"} {
		rightExt := filepath.Join(projectsDir, "real-project"+ext)
		if err := os.WriteFile(rightExt, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := app.projectPathFor(rightExt); err != nil {
			t.Fatalf("projectPathFor rejected a %s path: %v", ext, err)
		}
	}
}

// A stale path (the project was deleted or moved outside the app) must fail
// as not-found. SQLite creates a missing database file on open, so before
// projectPathFor required an existing file, opening a missing legacy-layout
// path left an empty, uninitialised database in the projects folder that
// then showed up as an unreadable project and could not be deleted.
func TestPathTakingIPCMethodsRefuseMissingProjectFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	projects := filepath.Join(app.requireUser().DataDir, "projects")
	if err := os.MkdirAll(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []string{
		filepath.Join(projects, "gone.pmforge"),                           // legacy flat layout
		filepath.Join(projects, "20260101-000000-gone", "project.gopmgr"), // current layout
	} {
		if _, err := app.OpenProject(missing); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("OpenProject(%s) error = %v, want ErrProjectNotFound", missing, err)
		}
		if err := app.DeleteProject(missing); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("DeleteProject(%s) error = %v, want ErrProjectNotFound", missing, err)
		}
		if _, err := os.Lstat(missing); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s exists after refused calls (err %v); a missing project must not be created", missing, err)
		}
	}
}

// The containment check is lexical, so a symlink inside the projects folder
// pointing outside it would pass it. Only a regular file is a project.
func TestPathTakingIPCMethodsRefuseSymlinkedProjectFile(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	// A real project encrypted under this user's key, so the only thing
	// that can refuse it is the regular-file check, not a failed decrypt.
	created, err := app.CreateProject("Moved outside", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "victim.gopmgr")
	if err := os.Rename(created.Path, outside); err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(app.requireUser().DataDir, "projects")
	link := filepath.Join(projects, "linked.pmforge")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := app.OpenProject(link); err == nil {
		t.Fatal("OpenProject followed a symlink out of the projects folder")
	}
	if err := app.DeleteProject(link); err == nil {
		t.Fatal("DeleteProject accepted a symlinked project file")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("symlink target was touched: %v", err)
	}
}

type fakeDirEntry struct {
	name string
	typ  fs.FileMode
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return e.typ.IsDir() }
func (e fakeDirEntry) Type() fs.FileMode          { return e.typ }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("not used") }

// The frontend sends back whatever Wails' encoding/json produced, which
// replaces EACH invalid UTF-8 byte with U+FFFD. The matcher must use that
// exact mapping (strings.ToValidUTF8 collapses a run into one U+FFFD), only
// consider the requested kind of entry, never follow a symlink, and refuse
// to guess when two entries encode to the same name.
func TestMatchWireProjectName(t *testing.T) {
	invalid := "20260922-150405-工程\xe5\xb7"  // pre-fix cut: 2 stray bytes
	wire := "20260922-150405-工程\ufffd\ufffd" // what the frontend returns
	literal := wire                          // a real folder whose name contains U+FFFD
	dir := fakeDirEntry{name: invalid, typ: fs.ModeDir}
	cases := []struct {
		name      string
		requested string
		entries   []fs.DirEntry
		wantDir   bool
		want      string
		wantErr   error
	}{
		{"invalid folder found by its wire name", wire, []fs.DirEntry{fakeDirEntry{name: "other", typ: fs.ModeDir}, dir}, true, invalid, nil},
		{"one U+FFFD per invalid byte, not per run", "20260922-150405-工程\ufffd", []fs.DirEntry{dir}, true, "", ErrProjectNotFound},
		{"literal U+FFFD folder alone", literal, []fs.DirEntry{fakeDirEntry{name: literal, typ: fs.ModeDir}}, true, literal, nil},
		{"ambiguous: invalid and literal both match", wire, []fs.DirEntry{dir, fakeDirEntry{name: literal, typ: fs.ModeDir}}, true, "", ErrProjectPathAmbiguous},
		{"symlink with a matching name is ignored", wire, []fs.DirEntry{fakeDirEntry{name: invalid, typ: fs.ModeSymlink}}, true, "", ErrProjectNotFound},
		{"a file does not match a folder request", wire, []fs.DirEntry{fakeDirEntry{name: invalid}}, true, "", ErrProjectNotFound},
		{"legacy file found by its wire name", wire + ".pmforge", []fs.DirEntry{fakeDirEntry{name: invalid + ".pmforge"}}, false, invalid + ".pmforge", nil},
		{"a folder does not match a file request", wire + ".pmforge", []fs.DirEntry{fakeDirEntry{name: invalid + ".pmforge", typ: fs.ModeDir}}, false, "", ErrProjectNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := matchWireProjectName(tc.requested, tc.entries, tc.wantDir)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("matchWireProjectName(%q) = %q, %v; want %q, %v", tc.requested, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// The same escape one level up: in the current layout the project folder
// itself can be a symlink to a folder outside the projects directory.
func TestPathTakingIPCMethodsRefuseSymlinkedProjectFolder(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	created, err := app.CreateProject("Folder moved outside", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	folder := filepath.Dir(created.Path)
	outside := filepath.Join(t.TempDir(), "outside-folder")
	if err := os.Rename(folder, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, folder); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := app.OpenProject(created.Path); err == nil {
		t.Fatal("OpenProject followed a symlinked project folder out of the projects folder")
	}
	if err := app.DeleteProject(created.Path); err == nil {
		t.Fatal("DeleteProject accepted a symlinked project folder")
	}
	if _, err := os.Stat(filepath.Join(outside, "project.gopmgr")); err != nil {
		t.Fatalf("symlinked folder's target was touched: %v", err)
	}
}

// A folder whose real name contains U+FFFD is valid UTF-8, so every
// filesystem accepts it, and its path goes through the same resolver as a
// wire path. This keeps resolveWireProjectPath exercised on macOS, where the
// invalid-UTF-8 test below has to skip.
func TestProjectFolderNamedWithReplacementCharacterOpens(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	created, err := app.CreateProject("Literal replacement", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	folder := filepath.Join(filepath.Dir(filepath.Dir(created.Path)), "20260922-150405-工程�")
	if err := os.Rename(filepath.Dir(created.Path), folder); err != nil {
		t.Fatal(err)
	}
	proj, err := app.OpenProject(filepath.Join(folder, "project.gopmgr"))
	if err != nil || proj.Name != "Literal replacement" {
		t.Fatalf("OpenProject(literal U+FFFD folder) = %+v, %v", proj, err)
	}
	if _, err := app.OpenProject(filepath.Join(folder+"�", "project.gopmgr")); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("OpenProject(no matching folder) error = %v, want ErrProjectNotFound", err)
	}
}

// Before rune-boundary truncation, a long non-ASCII project name could
// produce an invalid-UTF-8 folder name. Linux accepts such names (APFS does
// not, so this skips on macOS). The project list reaches the frontend as
// JSON, which replaces the invalid bytes, and the path that comes back must
// still reach the project on disk: open, clone, and delete it, and refuse
// rather than guess when a second folder encodes to the same name.
func TestProjectWithInvalidUTF8FolderIsReachableFromWirePath(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "alice-strong-password", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	created, err := app.CreateProject("Pre-fix project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	projects := filepath.Join(app.requireUser().DataDir, "projects")
	realFolder := filepath.Join(projects, "20260922-150405-工程\xe5\xb7")
	if err := os.Rename(filepath.Dir(created.Path), realFolder); err != nil {
		if errors.Is(err, syscall.EILSEQ) {
			t.Skip("filesystem rejects invalid UTF-8 names (APFS); Linux CI runs this")
		}
		t.Fatalf("rename to invalid-UTF-8 folder: %v", err)
	}
	wirePath := func() string {
		t.Helper()
		files, err := app.ListProjects()
		if err != nil || len(files) != 1 {
			t.Fatalf("ListProjects() = %v, %v; want the one project", files, err)
		}
		raw, err := json.Marshal(files[0]) // what Wails sends to the frontend
		if err != nil {
			t.Fatal(err)
		}
		var back ProjectFile
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatal(err)
		}
		if back.Path == files[0].Path || !strings.ContainsRune(back.Path, '\uFFFD') {
			t.Fatalf("wire path %q should differ from the on-disk path by U+FFFD substitution", back.Path)
		}
		return back.Path
	}()

	proj, err := app.OpenProject(wirePath)
	if err != nil || proj.Name != "Pre-fix project" {
		t.Fatalf("OpenProject(wire path) = %+v, %v; want the pre-fix project", proj, err)
	}
	if err := app.CloseProject(); err != nil {
		t.Fatal(err)
	}
	clone, err := app.CloneProject(wirePath)
	if err != nil {
		t.Fatalf("CloneProject(wire path): %v", err)
	}

	// A folder whose real name is the wire name makes the request ambiguous:
	// refuse, and delete neither.
	decoy := filepath.Join(projects, filepath.Base(filepath.Dir(wirePath)))
	if err := os.Mkdir(decoy, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := app.OpenProject(wirePath); !errors.Is(err, ErrProjectPathAmbiguous) {
		t.Fatalf("OpenProject(ambiguous) error = %v, want ErrProjectPathAmbiguous", err)
	}
	if err := app.DeleteProject(wirePath); !errors.Is(err, ErrProjectPathAmbiguous) {
		t.Fatalf("DeleteProject(ambiguous) error = %v, want ErrProjectPathAmbiguous", err)
	}
	for _, p := range []string{realFolder, decoy} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%q was touched by a refused call: %v", p, err)
		}
	}
	if err := os.Remove(decoy); err != nil {
		t.Fatal(err)
	}

	if err := app.DeleteProject(wirePath); err != nil {
		t.Fatalf("DeleteProject(wire path): %v", err)
	}
	if _, err := os.Stat(realFolder); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("real folder still present after delete (err %v)", err)
	}
	if _, err := os.Stat(clone.Path); err != nil {
		t.Fatalf("delete removed the clone too: %v", err)
	}
}
