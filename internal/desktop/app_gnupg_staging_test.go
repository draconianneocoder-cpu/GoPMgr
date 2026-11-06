// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopmgr/internal/documents"
)

// The external-command fixture acts as the signing process. Its simulated
// competing writer can plant names only in the export directory, never inside
// a private staging directory. It does not model a compromised signer.
func TestGnuPGExportsUsePrivateStaging(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture; Windows ACL qualification remains separate")
	}
	for _, kind := range []string{"document", "combined"} {
		for _, mode := range []string{"race", "failure", "missing", "empty", "symlink", "directory", "collision", "cleanup"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				if mode == "cleanup" && os.Geteuid() == 0 {
					t.Skip("permission-denial cleanup test requires an unprivileged user")
				}
				app := newEncryptionProjectTestApp(t)
				if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
					t.Fatal(err)
				}
				mustOpenProject(t, app, "GnuPG staging")
				doc, err := app.NewDocument("charter_word", "Staging fixture")
				if err != nil {
					t.Fatal(err)
				}
				fixture := t.TempDir()
				sentinel := filepath.Join(fixture, "sentinel")
				record := filepath.Join(fixture, "record")
				if err := os.WriteFile(sentinel, []byte("preserve me"), 0600); err != nil {
					t.Fatal(err)
				}
				script := `#!/bin/sh
set -eu
out=
for arg do
 if [ "${previous:-}" = --output ]; then out=$arg; fi
 previous=$arg
 input=$arg
done
printf '%s\n%s\n' "$out" "$input" > "$GOPMGR_GPG_RECORD"
stage=$(dirname "$out")
find "$stage" -prune -type d -perm 0700 >> "$GOPMGR_GPG_RECORD"
case "$GOPMGR_GPG_MODE" in
 race)
  if [ "$stage" = "$(dirname "$input")" ]; then ln -s "$GOPMGR_GPG_SENTINEL" "$out"; fi
  printf 'fixture signature' > "$out";;
 failure) printf 'partial signature' > "$out"; printf 'fixture signer failed' >&2; exit 7;;
 missing) :;;
 empty) : > "$out";;
 symlink) ln -s "$GOPMGR_GPG_SENTINEL" "$out";;
 directory) mkdir "$out";;
 cleanup) printf 'fixture signature' > "$out"; chmod 0500 "$stage";;
 collision) printf 'competing artifact' > "$input.asc"; printf 'fixture signature' > "$out";;
esac
`
				if err := os.WriteFile(filepath.Join(fixture, "gpg"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", fixture+string(os.PathListSeparator)+os.Getenv("PATH"))
				t.Setenv("GOPMGR_GPG_RECORD", record)
				t.Setenv("GOPMGR_GPG_SENTINEL", sentinel)
				t.Setenv("GOPMGR_GPG_MODE", mode)
				var result GnuPGExportResult
				if kind == "document" {
					result, err = app.ExportDocumentPDFGnuPG(doc.ID, "  fixture-key  ")
				} else {
					result, err = app.ExportCombinedReportGnuPG("Combined staging", "", []documents.ReportSection{{DocumentID: doc.ID, Title: doc.Title}}, "fixture-key")
				}
				raw, readErr := os.ReadFile(record)
				if readErr != nil {
					t.Fatal(readErr)
				}
				lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
				if len(lines) < 2 {
					t.Fatalf("missing command paths: %q", raw)
				}
				staged, pdf := lines[0], lines[1]
				if got, _ := os.ReadFile(sentinel); !bytes.Equal(got, []byte("preserve me")) {
					t.Errorf("competing export-directory writer changed sentinel: %q", got)
				}
				if filepath.Dir(staged) == filepath.Dir(pdf) {
					t.Error("signer output exposed in export directory")
				} else {
					if len(lines) != 3 || lines[2] != filepath.Dir(staged) {
						t.Errorf("staging directory was not owner-only: %q", raw)
					}
					if mode == "cleanup" {
						t.Cleanup(func() { _ = os.Chmod(filepath.Dir(staged), 0700); _ = os.RemoveAll(filepath.Dir(staged)) })
						if err == nil || !strings.Contains(err.Error(), "remove private GnuPG staging directory") {
							t.Errorf("cleanup failure not reported: %v", err)
						}
					} else if _, statErr := os.Lstat(filepath.Dir(staged)); !os.IsNotExist(statErr) {
						t.Errorf("owned staging remains: %v", statErr)
					}
				}
				if b, e := os.ReadFile(pdf); e != nil || !bytes.HasPrefix(b, []byte("%PDF-")) {
					t.Fatalf("existing PDF retention changed: %v", e)
				}
				var signed int
				if e := app.db.Conn.QueryRow("SELECT COUNT(*) FROM audit_events WHERE signature_status = 'gpg_signed'").Scan(&signed); e != nil {
					t.Fatal(e)
				}
				if (mode == "race" && signed != 1) || (mode != "race" && signed != 0) {
					t.Errorf("signed audit count=%d for %s", signed, mode)
				}
				switch mode {
				case "race":
					if err != nil {
						t.Fatal(err)
					}
					b, e := os.ReadFile(result.SignaturePath)
					if e != nil || string(b) != "fixture signature" {
						t.Fatalf("signature=%q, %v", b, e)
					}
					info, e := os.Stat(result.SignaturePath)
					if e != nil || info.Mode().Perm() != 0600 {
						t.Fatalf("published signature permissions: %v, %v", info, e)
					}
				default:
					if err == nil || result != (GnuPGExportResult{}) {
						t.Errorf("invalid/failed signature reported success: %+v, %v", result, err)
					}
					b, e := os.ReadFile(pdf + ".asc")
					if mode == "collision" {
						if e != nil || string(b) != "competing artifact" {
							t.Errorf("competing sidecar overwritten: %q, %v", b, e)
						}
					} else if !os.IsNotExist(e) {
						t.Errorf("failed signature published: %q, %v", b, e)
					}
					if mode == "failure" && !strings.Contains(err.Error(), "fixture signer failed") {
						t.Errorf("lost command diagnostic: %v", err)
					}
				}
			})
		}
	}
}

func TestGnuPGStagingCancellationCleansOwnedDirectory(t *testing.T) {
	bin := t.TempDir()
	name := "gpg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("not executed"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app := &App{ctx: ctx}
	got, err := app.signFileWithGnuPG(filepath.Join(root, "input.pdf"), "")
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled signature=%x, %v", got, err)
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != 0 {
		t.Fatalf("cancelled staging remains: %v, %v", entries, e)
	}
}

func TestGnuPGStagingNativeSigner(t *testing.T) {
	home := os.Getenv("GOPMGR_SYNTHETIC_GPG_HOME")
	if home == "" {
		t.Skip("explicit isolated synthetic GnuPG keyring required")
	}
	if b, e := os.ReadFile(filepath.Join(home, ".gopmgr-synthetic-keyring")); e != nil || string(b) != "test-only" {
		t.Fatal("refusing unmarked GnuPG keyring")
	}
	t.Setenv("GNUPGHOME", home)

	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatal(err)
	}
	mustOpenProject(t, app, "Native GnuPG fixture")
	doc, err := app.NewDocument("charter_word", "Native signing fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"document", "combined"} {
		t.Run(kind, func(t *testing.T) {
			var result GnuPGExportResult
			if kind == "document" {
				result, err = app.ExportDocumentPDFGnuPG(doc.ID, "")
			} else {
				result, err = app.ExportCombinedReportGnuPG("Native combined", "", []documents.ReportSection{{DocumentID: doc.ID, Title: doc.Title}}, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			got, e := os.ReadFile(result.SignaturePath)
			if e != nil || !bytes.HasPrefix(got, []byte("-----BEGIN PGP SIGNATURE-----")) {
				t.Fatalf("not armored signature: %v", e)
			}
			if output, e := exec.Command("gpg", "--batch", "--verify", result.SignaturePath, result.PDFPath).CombinedOutput(); e != nil {
				t.Fatalf("native signature does not verify exported PDF: %v, %s", e, output)
			}
			if e := os.WriteFile(result.PDFPath, []byte("changed input"), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := exec.Command("gpg", "--batch", "--verify", result.SignaturePath, result.PDFPath).CombinedOutput(); e == nil {
				t.Fatal("native verifier accepted changed PDF")
			}
		})
	}
}
