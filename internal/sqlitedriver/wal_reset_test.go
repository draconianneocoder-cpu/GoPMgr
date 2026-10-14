// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const pristineSQLCipherAmalgamationSHA256 = "f0653cb9b6a67123f1c6059fa746a6c49e23ef67eb5c1c4508b2c543eb2d35d5"

func TestWALResetCheckpointPreservesCommittedData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("upstream WAL-reset fault-injection scenario requires Unix locking; Windows native validation remains separate")
	}
	source := filepath.Join("..", "..", "third_party", "go-sqlcipher")
	core, err := os.ReadFile(filepath.Join(source, "sqlite3.c"))
	if err != nil {
		t.Fatal(err)
	}
	anchor := "  /* Copy data from the log to the database file. */"
	if strings.Count(string(core), anchor) != 1 {
		t.Fatal("checkpoint instrumentation anchor changed")
	}
	patched := string(core)
	pristine := restorePristineWALCheckpoint(t, patched)
	harness, err := os.ReadFile("testdata/wal_reset.c")
	if err != nil {
		t.Fatal(err)
	}
	cc := runNativeCommand(t, "", "go", "env", "CC")
	compiler := strings.Fields(strings.TrimSpace(cc))
	if len(compiler) == 0 {
		t.Fatal("Go C compiler is empty")
	}
	files, err := filepath.Glob(filepath.Join(source, "*.c"))
	if err != nil {
		t.Fatal(err)
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixed := range []bool{false, true} {
		name := "pristine-control"
		if fixed {
			name = "patched"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			text := pristine
			if fixed {
				text = patched
			}
			text = strings.Replace(text, anchor, "  (void)sqlite3FaultSim(660);\n"+anchor, 1)
			for name, data := range map[string][]byte{"sqlite3.c": []byte(text), "harness.c": harness} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			binary := filepath.Join(dir, "harness")
			args := append([]string{}, compiler[1:]...)
			args = append(args, "-std=gnu99", "-O0", "-g", "-DSQLITE_THREADSAFE=1", "-DSQLITE_HAS_CODEC", "-DSQLCIPHER_CRYPTO_LIBTOMCRYPT", "-DLTC_NO_ASM", "-DNDEBUG", "-DSQLITE_TEMP_STORE=2", "-I"+absSource, "harness.c")
			if os.Getenv("GOPMGR_SQLCIPHER_ASAN") == "1" {
				args = append(args, "-fsanitize=address")
			}
			for _, file := range files {
				if filepath.Base(file) == "sqlite3.c" {
					continue
				}
				abs, err := filepath.Abs(file)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, abs)
			}
			args = append(args, "-lpthread", "-lm")
			if runtime.GOOS == "linux" {
				args = append(args, "-ldl")
			}
			args = append(args, "-o", binary)
			runNativeCommand(t, dir, compiler[0], args...)
			for _, keyed := range []string{"0", "1"} {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				cmd := exec.CommandContext(ctx, binary, filepath.Join(dir, "case-"+keyed+".db"), keyed)
				out, err := cmd.CombinedOutput()
				cancel()
				t.Logf("keyed=%s: %s", keyed, out)
				if fixed {
					if err != nil || !strings.Contains(string(out), "WAL_RESET_PASS") {
						t.Fatalf("patched harness: %v\n%s", err, out)
					}
				} else {
					exit, ok := err.(*exec.ExitError)
					if !ok || exit.ExitCode() != 7 || !strings.Contains(string(out), "WAL_RESET_REGRESSION: stale generation backfilled") {
						t.Fatalf("control did not detect intended defect: %v\n%s", err, out)
					}
				}
			}
		})
	}
}

func restorePristineWALCheckpoint(t *testing.T, patched string) string {
	t.Helper()
	startMarker := "      WalIndexHdr *pLive = (WalIndexHdr*)walIndexHdr(pWal);\n"
	guardMarker := "      if( 0==memcmp(pLive->aSalt, pWal->hdr.aSalt, sizeof(pWal->hdr.aSalt)) ){\n"
	releaseMarker := "\n      /* Release the reader lock held while backfilling */"
	start := strings.Index(patched, startMarker)
	guard := strings.Index(patched, guardMarker)
	release := strings.Index(patched, releaseMarker)
	if start < 0 || guard < start || release < guard || strings.Count(patched, startMarker) != 1 || strings.Count(patched, guardMarker) != 1 {
		t.Fatal("WAL-reset backport markers changed")
	}
	bodyStart := guard + len(guardMarker)
	body := patched[bodyStart:release]
	if !strings.HasSuffix(body, "      }\n") {
		t.Fatal("WAL-reset backport closing guard changed")
	}
	body = strings.TrimSuffix(body, "      }\n")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line != "" {
			if !strings.HasPrefix(line, "  ") {
				t.Fatalf("WAL-reset backport line %d cannot be unindented", i+1)
			}
			lines[i] = strings.TrimPrefix(line, "  ")
		}
	}
	pristine := patched[:start] + strings.Join(lines, "\n") + patched[release:]
	sum := sha256.Sum256([]byte(pristine))
	if got := hex.EncodeToString(sum[:]); got != pristineSQLCipherAmalgamationSHA256 {
		t.Fatalf("reconstructed pristine SQLCipher amalgamation hash = %s, want %s", got, pristineSQLCipherAmalgamationSHA256)
	}
	return pristine
}

func runNativeCommand(t *testing.T, dir, command string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", command, err, out)
	}
	return string(out)
}
