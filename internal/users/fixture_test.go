// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopmgr/internal/sqlitedriver"
)

// Store templates. Building an account costs a full Argon2id password hash
// and key wrap, and issuing recovery codes costs one of each per code, so
// the setups many tests share are built once per test run and each test
// gets its own copy. Templates are built in TestMain, before any test can
// replace crypto/rand.Reader, and their files are checked again after the
// run so a test that wrote into one fails the run.

// storeTemplate is a closed store on disk plus the secrets its builder
// produced.
type storeTemplate struct {
	leaf  string   // root folder name, kept from the helper it replaces
	root  string   // the template's root folder
	dek   []byte   // alice's data encryption key, if the builder unlocked it
	codes []string // alice's recovery codes, if the builder issued them
	sum   string   // checksum of the root folder after building
}

var templates struct {
	escrow   *storeTemplate // alice: super administrator with an escrow session enrolled
	password *storeTemplate // alice: administrator, key unlocked
	rotation *storeTemplate // the password template plus alice's recovery codes
	status   *storeTemplate // alice: administrator; bob: standard account
	dek      *storeTemplate // alice: standard account, root folder "root"
}

func TestMain(m *testing.M) {
	code, err := runWithTemplates(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "users store templates:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func runWithTemplates(m *testing.M) (code int, err error) {
	dir, err := os.MkdirTemp("", "gopmgr-users-templates-")
	if err != nil {
		return 1, err
	}
	defer func() {
		if removeErr := os.RemoveAll(dir); err == nil && removeErr != nil {
			err = removeErr
		}
	}()
	builders := []struct {
		slot  **storeTemplate
		leaf  string
		build func(*Store, *storeTemplate) error
	}{
		{&templates.escrow, "GoPMgr", buildEscrowTemplate},
		{&templates.password, "GoPMgr", buildPasswordTemplate},
		{&templates.rotation, "GoPMgr", buildRotationTemplate},
		{&templates.status, "GoPMgr", buildStatusTemplate},
		{&templates.dek, "root", buildDEKTemplate},
	}
	for i, b := range builders {
		tmpl, err := buildTemplate(filepath.Join(dir, fmt.Sprint(i), b.leaf), b.leaf, b.build)
		if err != nil {
			return 1, fmt.Errorf("build %s template %d: %w", b.leaf, i, err)
		}
		*b.slot = tmpl
	}
	code = m.Run()
	for _, b := range builders {
		sum, err := treeChecksum((*b.slot).root)
		if err != nil {
			return code, err
		}
		if sum != (*b.slot).sum {
			return code, fmt.Errorf("a test changed the template at %s; tests must use copies", (*b.slot).root)
		}
	}
	return code, nil
}

// buildTemplate creates a store at root, runs build on it, and closes it
// so the database is complete on disk before it is copied.
func buildTemplate(root, leaf string, build func(*Store, *storeTemplate) error) (*storeTemplate, error) {
	store, err := Open(root)
	if err != nil {
		return nil, err
	}
	tmpl := &storeTemplate{leaf: leaf, root: root}
	buildErr := build(store, tmpl)
	if err := errors.Join(buildErr, store.Close()); err != nil {
		return nil, err
	}
	if tmpl.sum, err = treeChecksum(root); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func buildEscrowTemplate(store *Store, tmpl *storeTemplate) error {
	_, dek, err := store.CreateAccountWithKey("", "alice", "Alice", statusPassword, true, nil)
	if err != nil {
		return err
	}
	tmpl.dek = dek
	return store.EnrollAdminSession("alice", dek)
}

func buildPasswordTemplate(store *Store, tmpl *storeTemplate) error {
	if _, err := store.CreateAccount("alice", "Alice", oldPassword, true); err != nil {
		return err
	}
	dek, err := unlockDEK(store, "alice", oldPassword)
	tmpl.dek = dek
	return err
}

func buildRotationTemplate(store *Store, tmpl *storeTemplate) error {
	if err := buildPasswordTemplate(store, tmpl); err != nil {
		return err
	}
	codes, err := store.IssueRecoveryCodes("alice", tmpl.dek)
	tmpl.codes = codes
	return err
}

func buildStatusTemplate(store *Store, _ *storeTemplate) error {
	if _, err := store.CreateAccount("alice", "Alice", statusPassword, true); err != nil {
		return err
	}
	_, err := store.CreateAccount("bob", "Bob", statusPassword, false)
	return err
}

func buildDEKTemplate(store *Store, _ *storeTemplate) error {
	_, err := store.CreateAccount("alice", "Alice", "p4ssw0rd-original", false)
	return err
}

// storeFromTemplate copies tmpl into the test's own folder and opens it.
// The copy is closed when the test ends. Its key and codes are fresh
// slices, because tests clear keys in their cleanup.
func storeFromTemplate(t *testing.T, tmpl *storeTemplate) (*Store, []byte, []string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), tmpl.leaf)
	if err := copyTemplate(tmpl, root); err != nil {
		t.Fatalf("copy %s template: %v", tmpl.leaf, err)
	}
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open copy of %s template: %v", tmpl.leaf, err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	dek := bytes.Clone(tmpl.dek)
	t.Cleanup(func() { clear(dek) })
	return store, dek, slices.Clone(tmpl.codes)
}

// copyTemplate copies tmpl's folder to root and points every account's
// stored folder at root, as creating the accounts there would have. It
// fails if any stored folder still names the template.
func copyTemplate(tmpl *storeTemplate, root string) error {
	if err := copyTree(tmpl.root, root); err != nil {
		return err
	}
	conn, err := sql.Open(sqlitedriver.Name, filepath.Join(root, "system.db"))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	rows, err := conn.Query(`SELECT username FROM users`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for _, name := range names {
		if _, err := conn.Exec(`UPDATE users SET data_dir = ? WHERE username = ?`, filepath.Join(root, name), name); err != nil {
			return err
		}
	}
	return checkNoTemplatePaths(conn, tmpl.root)
}

func checkNoTemplatePaths(conn *sql.DB, templateRoot string) error {
	var stale int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM users WHERE substr(data_dir, 1, ?) = ?`, len(templateRoot), templateRoot,
	).Scan(&stale); err != nil {
		return err
	}
	if stale != 0 {
		return fmt.Errorf("%d account folder(s) still point at the template", stale)
	}
	return nil
}

// treeChecksum hashes every path, permission and file under root.
func treeChecksum(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(h, "%s %v\n", rel, info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path) // #nosec G304 -- a test template under os.MkdirTemp.
			if err != nil {
				return err
			}
			h.Write(data)
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func TestStoreTemplateCopiesAreIndependent(t *testing.T) {
	t.Parallel()
	first, _, _ := storeFromTemplate(t, templates.status)
	second, _, _ := storeFromTemplate(t, templates.status)
	if _, err := first.CreateAccount("carol", "Carol", statusPassword, false); err != nil {
		t.Fatalf("CreateAccount in the first copy: %v", err)
	}
	accounts, err := second.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accounts {
		if a.Username == "carol" {
			t.Fatal("an account created in one copy appeared in another")
		}
	}
	if sum, err := treeChecksum(templates.status.root); err != nil || sum != templates.status.sum {
		t.Fatalf("the template changed while its copies were used (err %v)", err)
	}
	for _, store := range []*Store{first, second} {
		var dirs []string
		rows, err := store.conn.Query(`SELECT data_dir FROM users`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var dir string
			if err := rows.Scan(&dir); err != nil {
				t.Fatal(err)
			}
			dirs = append(dirs, dir)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs {
			if !strings.HasPrefix(dir, store.RootDir()+string(filepath.Separator)) {
				t.Errorf("stored folder %q is not under the copy's root %q", dir, store.RootDir())
			}
		}
	}
}

func TestStoreTemplateCopyKeepsKeysAndCodesPrivate(t *testing.T) {
	t.Parallel()
	_, dek, codes := storeFromTemplate(t, templates.rotation)
	if len(dek) == 0 || len(codes) == 0 {
		t.Fatalf("rotation template copy has key %d bytes and %d codes, want both", len(dek), len(codes))
	}
	clear(dek)
	codes[0] = "changed"
	if bytes.Equal(templates.rotation.dek, dek) || templates.rotation.codes[0] == "changed" {
		t.Fatal("a copy shares its key or codes with the template")
	}
}
