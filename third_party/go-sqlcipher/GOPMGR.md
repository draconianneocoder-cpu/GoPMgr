<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr SQLCipher source provenance

GoPMgr builds this local module instead of the published
`github.com/mutecomm/go-sqlcipher/v4`, whose last release (v4.4.2, SQLCipher
4.4.2 on SQLite 3.33.0) is from 2020. Since 2026-11-10 this copy is:

- the **Go bindings and LibTomCrypt files of go-sqlcipher v4.4.2**, with
  GoPMgr's transaction-cleanup changes to `sqlite3.go`;
- the **SQLCipher 4.20.0 engine** (SQLite 3.54.1): `sqlite3.c` and
  `sqlite3.h`, built unmodified from the upstream tag;
- `flags.go` with two added build flags, and `gopmgr_rng.c`.

The encrypted file format (SQLCipher 4), raw-key handling, LibTomCrypt
provider and public driver API are unchanged. Existing database files need
no migration; see
[the upgrade evidence](../../docs/testing/sqlcipher-4.20-upgrade.md).

## Engine source

- Upstream: `https://github.com/sqlcipher/sqlcipher`, tag `v4.20.0`
- Tag commit: `355d4db92d3404e71faa8088d071a71d767807f4`
- `sqlite3.c` SHA-256:
  `8cd2347d18886fee4651153fabeeeffb7cec008700987dab1c00261febc4220f`
- `sqlite3.h` SHA-256:
  `e36eb6ed4de77e56a633866a8f84ed339785662d0cf79cf099f420b638bf28ec`

To rebuild both files (the result was byte-identical on macOS and Ubuntu
26.04):

```sh
git clone --depth 1 --branch v4.20.0 https://github.com/sqlcipher/sqlcipher.git
mkdir bld && cd bld
../sqlcipher/configure --with-tempstore=yes \
  CFLAGS="-DSQLITE_HAS_CODEC -DSQLITE_EXTRA_INIT=sqlcipher_extra_init -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown"
make sqlite3.c
```

The WAL-reset correction GoPMgr used to backport into SQLite 3.33.0
(upstream `fe57e14b49f9189b56da8233ab3415ce5ff6b1ff`) is part of SQLite
since 3.52.0, so `sqlite3.c` carries no local change.
`TestWALResetCheckpointPreservesCommittedData` still runs the fault-injection
scenario against it, with a control that removes SQLite's check.

## Local files and changes

- `flags.go` adds `-DSQLITE_EXTRA_INIT=sqlcipher_extra_init` and
  `-DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown`, which SQLCipher has
  required since 4.7, and links `bcrypt` on Windows for `gopmgr_rng.c`.
- `gopmgr_rng.c` provides `rng_get_bytes`, from which the LibTomCrypt
  provider seeds its Fortuna generator (SQLCipher 4.5 and later). It reads
  only the operating system's generator (`getentropy`, or `BCryptGenRandom`
  on Windows) and returns 0 on failure, so SQLCipher refuses to start.
  LibTomCrypt's own `rng_get_bytes.c` is not used because it falls back to
  timing the C clock.
- `sqlite3.go`: every failed COMMIT calls ROLLBACK without replacing the
  original COMMIT error. A failed `SQLiteTx.Rollback` preserves its own
  error. Failed cleanup with an active transaction marks the connection
  unusable; the additive `IsValid` method implements `driver.Validator`.
  Preparation, statement execution and cursor stepping refuse that
  connection before native execution. Manual transactions are not
  invalidated merely for being open. Driver-owned statements keep callback
  handles alive across `sqlite3_close_v2` until the last statement
  finalizes, allowing deferred native rollback and lock release. No native
  pointer is retained after destruction.

Wrapper provenance:

- Published module: `github.com/mutecomm/go-sqlcipher/v4@v4.4.2`, tag commit
  `25f68adb9fb3bc95229bee7e3498a6ca2f3e752d`, Go module checksum
  `h1:eM10bFtI4UvibIsKr10/QT7Yfz+NADfjZYh0GKrXUNc=`
- Original `sqlite3.go` SHA-256:
  `8e1c03179bf1259c8fbbb983e6932aa9e847692c509c3cfd4cc3fcb6f6bb4071`
- Wrapper tree SHA-256 (the 89 files still from v4.4.2, with `sqlite3.go`
  restored to the original):
  `86c8178235210c18cb451ae981443bae4a7cb611166229eac873d574ed707088`
- Vendored 93-file runtime-tree SHA-256:
  `72c4ea6774d9760e2dd65c704217967749294ae54660f4ede10d55829b2d1ba4`

`MAINTENANCE` and `README.md` are go-sqlcipher's own files, kept unchanged;
their version statements describe v4.4.2, not this copy.

Transaction cleanup applies to the driver's Tx methods, not manually issued
SQL. Statement lifecycle handling covers driver-owned statements. It does not
establish safe closure with an externally retained
`SQLiteConn.Backup`, BLOB handle, or native statement created outside this driver.
Such resources must finish before connection close. When a ROLLBACK error occurs
after SQLite has already rolled back, the healthy connection is not invalidated.
The separate account writer disposes connections after any raw rollback error;
its conservative cleanup and regressions are recorded in
[account transaction evidence](../../docs/testing/account-transaction-cleanup.md).

## Behaviour that changed with the engine

- A page that fails to decrypt (damage or tampering) now puts that
  connection into a permanent error state until it is closed; SQLCipher
  4.4.2 kept serving other pages. GoPMgr's repair therefore makes its copy
  on a fresh connection (`db.InformativeSelfHeal`).
- SQL features newer than SQLite 3.33.0 (`RETURNING`, `STRICT`,
  `DROP COLUMN`, built-in JSON functions) are now available in the engine.
  A file that uses them cannot be opened by older GoPMgr builds, so adopting
  one is a file-format decision, not a convenience.

## Tests that pin this copy

`TestVendoredSQLCipherProvenance` verifies the file count, the engine and
notice hashes, the runtime-tree hash and the wrapper-tree hash.
`TestEngineReportsTheVendoredVersions` checks the versions of the linked
engine. `TestFilesFromSQLCipher442OpenUnderTheCurrentEngine` opens fixture
files written by the previous engine.

## Maintenance

When refreshing the engine:

1. Take the newest stable SQLCipher tag (not a beta), verify the tag commit,
   and rebuild `sqlite3.c` and `sqlite3.h` as above on two hosts.
2. Read the SQLCipher release notes for every version skipped, for new
   required build flags, provider functions and behaviour changes.
3. Update the hashes here and in `internal/sqlitedriver/provenance_test.go`,
   the versions in `tools/sbom/components.json`, and
   `THIRD_PARTY_NOTICES.txt` if upstream's notice changed.
4. Run `go test -count=1 ./internal/sqlitedriver`,
   `scripts/validate-encrypted-db.sh`, `reuse lint`, `make verify` and the
   broader release gates on macOS and Linux, and build on Windows.
5. Check the repair tests' page offsets
   (`internal/desktop/repair_and_swap_test.go`) with a page sweep.

Rolling back to the unmodified v4.4.2 module restores exposure to the
WAL-reset and failed-COMMIT defects and to SQLite 3.33.0's known
vulnerabilities, and is not a safe operational recovery.

The original `LICENSE` is retained unchanged. Repository-level SPDX annotations
and the standard texts in `LICENSES/` cover the MIT, BSD-3-Clause, SQLite
blessing, and Unlicense portions of this mixed-source module. Every binary
packaging path installs `THIRD_PARTY_NOTICES.txt`, which combines the applicable
source-specific copyright notices and license texts so they remain with
distributed applications.
