// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close %s: %v", dst, err)
	}
}

// A copy opened for reading cannot be written on any pooled connection.
func TestEncryptedCopyOpensReadOnlyOnEveryConnection(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "project.gopmgr")
	dek := testDEK(t, 0x5a)
	d, err := InitEncryptedDB(original, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if _, err := d.Conn.Exec(`CREATE TABLE probe (x INTEGER); INSERT INTO probe VALUES (1);`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	copyPath := filepath.Join(dir, "copy.gopmgr")
	copyTestFile(t, original, copyPath)

	ro, err := OpenEncryptedCopyReadOnly(copyPath, dek)
	if err != nil {
		t.Fatalf("OpenEncryptedCopyReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	var n int
	if err := ro.Conn.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("read the copy: %d rows, %v; want 1", n, err)
	}

	ctx := context.Background()
	first, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	defer func() { _ = first.Close() }()
	second, err := ro.Conn.Conn(ctx)
	if err != nil {
		t.Fatalf("second connection: %v", err)
	}
	defer func() { _ = second.Close() }()
	if _, err := first.ExecContext(ctx, `INSERT INTO probe VALUES (2)`); err == nil {
		t.Error("the first pooled connection wrote to the copy")
	}
	if _, err := second.ExecContext(ctx, `UPDATE probe SET x = 3`); err == nil {
		t.Error("the second pooled connection wrote to the copy")
	}
	if _, err := ro.Conn.Exec(`CREATE TABLE other (y)`); err == nil {
		t.Error("the copy's schema changed")
	}
}

func TestDamagedEncryptedCopyIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copy.gopmgr")
	dek := testDEK(t, 0x5b)
	d, err := InitEncryptedDB(path, dek)
	if err != nil {
		t.Fatalf("InitEncryptedDB: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteAt(make([]byte, 64), 5000); err != nil {
		t.Fatalf("damage: %v", err)
	}
	_ = f.Close()

	if ro, err := OpenEncryptedCopyReadOnly(path, dek); err == nil {
		_ = ro.Close()
		t.Fatal("a damaged copy was opened")
	}
}
