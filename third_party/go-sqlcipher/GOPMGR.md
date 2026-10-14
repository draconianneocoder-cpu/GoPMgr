<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr SQLCipher source provenance

GoPMgr builds this local module instead of the published module to carry a
WAL-reset correctness backport and failed-COMMIT cleanup. It retains the published
`github.com/mutecomm/go-sqlcipher/v4` v4.4.2 API, SQLite 3.33.0 core,
SQLCipher 4.4.2 format and codec, and LibTomCrypt provider.

## Source and change

- Published module: `github.com/mutecomm/go-sqlcipher/v4@v4.4.2`
- Tag commit: `25f68adb9fb3bc95229bee7e3498a6ca2f3e752d`
- Go module checksum:
  `h1:eM10bFtI4UvibIsKr10/QT7Yfz+NADfjZYh0GKrXUNc=`
- Unmodified `sqlite3.c` SHA-256:
  `f0653cb9b6a67123f1c6059fa746a6c49e23ef67eb5c1c4508b2c543eb2d35d5`
- Patched `sqlite3.c` SHA-256:
  `054c25f4d2637247452338f8277884eeaf67731697a18c7a46b5581c7fae8b78`
- Upstream SQLite correction:
  `fe57e14b49f9189b56da8233ab3415ce5ff6b1ff`
- Upstream testability follow-up:
  `bb80d688acfdd33706f6f7a773fbf8960aeb112b`

The native runtime-source difference from v4.4.2 is in `sqlite3.c`. After a
PASSIVE checkpoint obtains WAL read-lock slot zero, it compares the live WAL
salt with the checkpoint's cached header. If a writer reset the WAL generation,
the checkpoint skips all stale backfill I/O and bookkeeping. Lock and iterator
cleanup remain unconditional.

The Go-side difference is in `sqlite3.go`: every failed COMMIT attempts ROLLBACK
without replacing the original error. Failed cleanup with an active transaction
marks the connection unusable; the additive `IsValid` method implements
`driver.Validator`. Preparation, statement execution and cursor stepping refuse
that connection before native execution. Manual transactions are not invalidated
merely for being open. Driver-owned statements keep callback handles alive across
`sqlite3_close_v2` until the last statement finalizes, allowing deferred native
rollback and lock release. No native pointer is retained after destruction.

- Original `sqlite3.go` SHA-256:
  `8e1c03179bf1259c8fbbb983e6932aa9e847692c509c3cfd4cc3fcb6f6bb4071`
- Patched `sqlite3.go` SHA-256:
  `ce54fd87a573f3e3097cd75873448b6f7fd2f724905c66a6ccbc1827c643d890`
- Pristine 92-file runtime-tree SHA-256:
  `53b55af239a633b04a6fee66f0e5a9a76fdf2f7dc24c57b78e378d4beda6a73b`
- Patched 92-file runtime-tree SHA-256:
  `53faf187f7ff6d4306280b06f48f8e9801e7aaaddce345eeb41769bd3675c430`

The cleanup fix covers repository `database/sql` callers and their driver-owned
statements. It does not establish safe closure with an externally retained
`SQLiteConn.Backup`, BLOB handle, or native statement created outside this driver.
Such resources must finish before connection close. Explicit ROLLBACK rejection
is a separate existing gap; the new invalidation applies to failed COMMIT cleanup.

This directory contains the 92 root files required to compile the module. The
upstream tests, examples, scripts, and maintenance utilities are omitted because
they are not part of the runtime build. `TestVendoredSQLCipherProvenance`
verifies the file count, the patched amalgamation hash, and deterministic
pristine and patched runtime-tree hashes. The same test pins the complete
binary-notice artifact. It also reconstructs the exact published amalgamation
before running the causal negative control. The compatibility helper reverses
both corrections and verifies the entire pristine runtime tree before building.

## Maintenance

When replacing or refreshing this copy:

1. Verify the published tag, module checksum, and source commit.
2. Recreate the same runtime-only inventory from the module source.
3. Prefer an upstream release that contains both corrections and is
   proven compatible with GoPMgr's keying and file format. Remove the local
   patch and `replace` directive when that is available.
4. Update the hashes and provenance test intentionally.
5. Run `go test -count=1 ./internal/sqlitedriver`,
   `scripts/validate-encrypted-db.sh`, `reuse lint`, and the broader release
   gates. Exercise both plaintext and keyed WAL-reset cases and old/new file
   interchange, failed-COMMIT reuse/disposal, and retained-statement rollback hooks.

Rolling back to the unmodified v4.4.2 module restores exposure to the WAL-reset
and failed-COMMIT defects and is not a safe operational recovery. Existing
database files require no migration because these corrections do not alter their format.

The original `LICENSE` is retained unchanged. Repository-level SPDX annotations
and the standard texts in `LICENSES/` cover the MIT, BSD-3-Clause, SQLite
blessing, and Unlicense portions of this mixed-source module. Every binary
packaging path installs `THIRD_PARTY_NOTICES.txt`, which combines the applicable
source-specific copyright notices and license texts so they remain with
distributed applications.
