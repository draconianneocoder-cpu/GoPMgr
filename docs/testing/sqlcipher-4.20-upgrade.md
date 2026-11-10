<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# SQLCipher 4.20.0 upgrade evidence (2026-11-10)

The owner asked for the database engine to be upgraded to the newest stable
version. GoPMgr moved from SQLCipher 4.4.2 on SQLite 3.33.0 (2020) to
SQLCipher 4.20.0 on SQLite 3.54.1 (released 2026-10-27). SQLCipher 5.0.0 was
still a beta and was not used. Source, hashes and rebuild steps are in
[the provenance record](../../third_party/go-sqlcipher/GOPMGR.md).

## Why

SQLite 3.33.0 has known memory-corruption defects reachable from a crafted
database file (for example CVE-2021-20227 and CVE-2025-6965), found by the
2026-11-10 security scan. `govulncheck` cannot see C code built through cgo,
so nothing had flagged it.

## What changed

| Area | Change |
|---|---|
| Engine | `sqlite3.c` and `sqlite3.h` are the unmodified amalgamation built from the SQLCipher `v4.20.0` tag. The build is reproducible: macOS and Ubuntu 26.04 produced byte-identical files. |
| Build flags | `SQLITE_EXTRA_INIT` and `SQLITE_EXTRA_SHUTDOWN`, required since SQLCipher 4.7. |
| Random source | New `gopmgr_rng.c`: the LibTomCrypt provider now seeds itself through `rng_get_bytes`. GoPMgr's version uses only the operating system's generator and fails closed. |
| WAL-reset backport | Removed: SQLite has contained the correction since 3.52.0. The fault-injection test still runs, with a control that removes SQLite's own check. |
| Repair | `db.InformativeSelfHeal` makes its rebuilt copy on a fresh connection (see below). |

Unchanged: the Go bindings and LibTomCrypt files from go-sqlcipher v4.4.2,
GoPMgr's transaction-cleanup changes, the SQLCipher 4 file format, key
handling, and the driver API.

## A behaviour change found by the tests

Seven repair tests failed on the new engine. Since SQLCipher 4.5, a page
that fails to decrypt puts its connection into a permanent error state, so
that tampered data is never loaded. GoPMgr's repair ran its integrity check
and then rebuilt a copy on the same connection, so the check itself blocked
the rebuild.

A one-byte corruption sweep over every page of the repair fixture showed the
effect and the fix:

| Engine | Pages healed | Repair fails | Open fails | Query fails |
|---|---|---|---|---|
| 4.4.2 | 62 | 33 | 27 | 3 |
| 4.20.0, repair unchanged | 2 | 93 | 28 | 3 |
| 4.20.0, copy made on a fresh connection | 62 | 33 | 28 | 3 |

The fixture is one page longer under SQLite 3.54.1, so the pinned
"cannot heal" offset moved from page 119 to page 120.

Outside repair, the consequence for users is that a project with a damaged
page reports errors on that connection until the project is reopened,
rather than continuing to serve the undamaged pages.

## File compatibility

Existing files need no migration.

- **Old files under the new engine (the upgrade path), tested on every
  run:** `TestFilesFromSQLCipher442OpenUnderTheCurrentEngine` opens two
  hash-pinned fixtures written by the old engine, one encrypted and one
  plaintext, both in WAL mode. Each must read back exactly, pass
  `integrity_check` (and `cipher_integrity_check` when encrypted), accept
  writes including a cascading delete, reject a wrong key, and stay
  encrypted.
- **Both directions, checked once on 2026-11-10** with the two engines side
  by side: files created by each engine were opened, verified and written
  by the other. A real encrypted GoPMgr project created by the old engine
  (40 stakeholders) opened under the new engine with `GetProject`, both
  integrity checks and a write, and the old engine then opened it again.

The fixtures were created by a throwaway test that ran against the old
engine: tables `parent` and `child` (with a foreign key and an index), rows
`old café` and `東京`, key `x'42…42'`.

A new GoPMgr build will not write anything an older build cannot read
unless a later change deliberately adopts newer SQL features.

## Verification

- `go test ./internal/sqlitedriver`: provenance, linked-engine versions
  (`3.54.1`, `4.20.0`, `libtomcrypt`), old-file fixtures, WAL-reset scenario
  with its control (plaintext and keyed), commit and rollback cleanup.
- The full Go suite, `make verify`, the encrypted-database gate and race
  runs on macOS; `make verify` on Ubuntu 26.04.

## Not verified

- **Windows:** `gopmgr_rng.c` uses `BCryptGenRandom` and links `bcrypt`.
  No Windows compiler was available, so the Windows build and its random
  source are unverified. This must be built and run on Windows before a
  Windows release.
- The security scan's 2026 CVE identifiers were not independently
  confirmed; the two CVEs named above are.
