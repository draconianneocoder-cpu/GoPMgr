<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-001: Per-user database encryption at rest

**Status:** Implemented (option A via **A3**, 2026-06-13)
**Date:** 2026-06-11 (proposed) / 2026-06-12 (accepted) /
2026-06-13 (implemented)
**Deciders:** James L. Burns (project owner)

> **Acceptance note (2026-06-12).** James accepted A3:
> `mutecomm/go-sqlcipher/v4 v4.4.2` as-is, with the 2020-frozen
> bundled engine documented as a known risk and `sqlcipher_export`
> as the escape hatch to a future replacement. Basis: the Phase 0
> spike passed every functional check on both development platforms
> (Appendix A) and no maintained alternative tracks current
> SQLCipher. The key hierarchy landed the same day; SQLCipher project
> database open/migration, secondary opener handling, and release gates
> landed on 2026-06-13 — see Appendix B.

## Context

PMForge is local-first: every user's project data lives in
`~/Documents/PMForge/<username>/projects/*.pmforge` (SQLite, WAL mode,
foreign keys) plus a shared `~/Documents/PMForge/system.db` holding
the account list. The V2 protections are per-user directory isolation
(POSIX `0700`) and Argon2id password hashes (PHC format) for login.
For raw-disk theft and admin-level host access, OS-level full-disk
encryption (FileVault / BitLocker / LUKS) remains recommended
whole-device defence in depth. This ADR records the native database
encryption design now implemented for per-user `.pmforge` project
databases.

Relevant existing machinery this design must not break:

- **Driver:** `mattn/go-sqlite3 v1.14.22` (CGO), opened as `"sqlite3"`
  with `PRAGMA journal_mode = WAL` and `PRAGMA foreign_keys = ON`
  (`internal/db/sqlite.go`).
- **Self-heal:** `InformativeSelfHeal` + `SwapInSnapshot` +
  `checkSnapshotIntegrity` (`internal/db/repair.go`) read and verify
  database files; any encryption must keep these working.
- **Backups:** `CreateArchivalBundle` zips the raw database file into
  a `.pmba` (`internal/db/backup.go`).
- **Existing crypto:** `internal/crypto/encrypt.go` already provides
  AES-256-GCM with Argon2id key derivation (OWASP interactive
  parameters: t=1, m=64MiB, p=4) for export bundles — proven KDF
  parameters and code patterns to reuse for key wrapping.
- **Account recovery:** eight one-time recovery codes per account,
  Argon2id-hashed in `system.db`. `ResetWithRecoveryCode` changes the
  password WITHOUT knowing the old one. This is the most important
  constraint in this document: any key hierarchy derived solely from
  the login password would make recovery-code resets orphan all of
  the user's data.

### Threat model

In scope: theft of the powered-off device without OS FDE enabled;
exfiltration of individual `.pmforge` files (cloud-sync folders,
backups, support bundles); reads by other OS accounts with elevation;
disposal/resale of drives.

Out of scope (unchanged by this design): malware running AS the
logged-in user while PMForge is unlocked; memory scraping of a
running process; a hostile OS administrator with a keylogger.

## Decision

Adopt **SQLCipher page-level encryption for the per-user `.pmforge`
project databases**, keyed by a per-user random Data Encryption Key
(DEK) that is wrapped by both the login password and each recovery
code. Keep `system.db` plaintext. Newly created project databases are
encrypted by default; existing plaintext project databases can be
migrated from Project Settings after recovery codes are reissued.

`system.db` stays plaintext deliberately: it contains only account
names, Argon2id password hashes, recovery-code hashes, and (new)
wrapped DEK blobs — all of which are designed to be safe at rest.
Encrypting it would create a bootstrapping problem (you need it to
log in before any user key exists).

## Options considered

### Option A: SQLCipher via a Go binding (chosen)

Replace the SQLite driver with a SQLCipher-enabled binding. Two
candidates to evaluate in the Phase 0 spike: `mutecomm/go-sqlcipher`
(bundles the SQLCipher amalgamation; registers the `"sqlite3"` driver
name, so it REPLACES `mattn/go-sqlite3` rather than sitting beside
it) or building `mattn/go-sqlite3` with the `libsqlite3` tag against
a system/vendored SQLCipher. Exact package, version, license text,
and maintenance status MUST be verified during the spike before any
dependency is added (repo rule: no guessed versions).

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — one driver swap, key plumbing, migration tool |
| Crash safety | High — encryption sits below the transaction layer; WAL and crash recovery semantics are SQLite's own |
| Packaging | Medium — CGO already required (mattn); adds the SQLCipher C amalgamation per platform |
| Performance | Expected single-digit % overhead; measure in spike |
| Team familiarity | Standard SQLite surface; PRAGMA key/rekey are the only new concepts |

**Pros:** transparent to every existing query and to the repair/WAL
machinery (open-with-key, then everything behaves as SQLite); per-page
HMAC gives tamper evidence; `PRAGMA rekey` enables key rotation;
industry-standard, widely audited design.
**Cons:** driver replacement touches the build for all three OS
targets (the V2 deferral reason — now bounded because CGO is already
mandatory); binary size grows; `checkSnapshotIntegrity` and any tool
that opens a database must learn to supply the key.

### Option B: Whole-file AES-256-GCM envelope (decrypt on open, encrypt on close)

Reuse `crypto.EncryptBuffer` around the entire `.pmforge` file.

**Pros:** pure Go, no new dependency, smallest diff.
**Cons (disqualifying):** plaintext working copy must exist on disk
while the app runs (WAL writes continuously); a crash leaves the
plaintext behind — exactly the crash-recovery hazard the V2 deferral
recorded; re-encrypt-on-close loses data on power failure; breaks
self-heal and `.pmba` bundling of live files. Rejected.

### Option C: Application-level field encryption (encrypt `charts.data`, `documents.content`, …)

**Pros:** pure Go; no driver change; selective.
**Cons:** metadata stays plaintext (project names, task labels in
chart JSON would need per-field handling, stakeholder names, audit
log, settings); encryption logic smears across every db accessor;
no page integrity; key handling identical to Option A anyway but
with far more code. Rejected as primary; not pursued as an interim
either, because the interim already exists (Option D).

### Option D: Status quo — OS full-disk encryption, documented

**Pros:** zero work; FileVault/BitLocker/LUKS are excellent.
**Cons:** depends on the user enabling it; does nothing for per-file
exfiltration from an unlocked, running system or for files copied
into cloud-sync folders. Remains recommended after SQLCipher ships as
defence in depth for the whole device.

## Key hierarchy (the recovery-code constraint)

```
login password ──Argon2id──► KEK_pw ────┐
recovery code 1 ──Argon2id──► KEK_r1 ──┤  each wraps the same
        …                               ├─► DEK (32 random bytes)
recovery code 8 ──Argon2id──► KEK_r8 ──┘        │
                                                ▼
                              PRAGMA key = raw DEK (x'…' keyspec)
                              for every .pmforge of that user
```

- **DEK:** 32 cryptographically random bytes, generated at account
  creation (or at migration time for existing accounts). Supplied to
  SQLCipher as a raw-key keyspec so SQLCipher's internal KDF is
  bypassed (we already paid for Argon2id; no double-KDF).
- **Wrapping:** AES-256-GCM via the existing `crypto` package
  patterns. `system.db` user row gains `wrapped_dek_pw` and one
  wrapped blob per active recovery code.
- **Password change (knows old password):** unwrap DEK with old
  KEK_pw, re-wrap with new. No database re-encryption.
- **Recovery-code reset (does NOT know the password):** unwrap DEK
  with that code's KEK_ri, set the new password, re-wrap as
  `wrapped_dek_pw`, burn the code. Data survives. Without this,
  recovery would orphan every project — this is why DEK wrapping per
  recovery code is non-negotiable.
- **All codes spent + password forgotten:** data is unrecoverable by
  design. The recovery-code issuance UI must say so explicitly.

## Migration (plaintext V2 → encrypted V3)

Per database, mirroring the proven `SwapInSnapshot` atomic pattern:

1. Open plaintext db; run `PRAGMA integrity_check`.
2. `ATTACH` a new encrypted file with the DEK;
   `SELECT sqlcipher_export('encrypted')`; copy `PRAGMA user_version`.
3. Open the new file with the key; `PRAGMA integrity_check` again.
4. Atomic rename: original → `.pre-encryption.bak`, encrypted →
   live name. Keep the `.bak` until the user deletes it (Settings
   surface, like repair backups).
5. Failure at any step leaves the plaintext original untouched.

Downgrade story: none (matches the V1→V2 stance). The `.bak` is the
escape hatch during the opt-in phase.

## Consequences

- Easier: honest at-rest security claims; per-file exfiltration of
  encrypted project databases is no longer a project-data breach;
  `.pmba` bundles preserve the encrypted `project.pmforge` bytes.
- Harder: every code path that opens a database needs the key
  (repair, backup verification, headless CLI export needs credentials);
  release builds include the SQLCipher amalgamation on
  macOS/Windows/Linux; veraPDF/PAdES gates are unaffected and
  `check-release` now includes an encrypted round-trip gate.
- Revisit: per-resource calendars ADR is independent; SQLCipher key
  rotation policy (PRAGMA rekey cadence) once shipped; whether
  `system.db` should eventually move to OS keychain integration.

## Action items (Phase 0 spike first; no dependency lands before it)

1. [~] Spike: build against candidate bindings. **linux/arm64 AND
   macOS arm64 done 2026-06-12 — see Appendix A.** Remaining:
   Windows (when a Windows build target exists; run
   `docs/design/spike-sqlcipher/` per its README).
2. [~] Spike: WAL + integrity + key semantics + migration against an
   encrypted db. **Done for the standalone driver (Appendix A);
   `InformativeSelfHeal`/`SwapInSnapshot` integration check moves to
   implementation step 5.**
3. [x] Implement key hierarchy in `internal/users` + `internal/crypto`
   (DEK generation, wrap/unwrap, recovery-code re-wrap path) with
   exhaustive tests, INCLUDING the reset-via-recovery-code data
   survival test.
4. [x] Migration tool + Settings opt-in toggle; `.bak` retention UI.
5. [x] Thread key into `db.InitDB` and every secondary opener
   (repair, backup, CLI headless paths).
6. [x] Extend `check-release` with an encrypted-db round-trip gate;
   REUSE entries for the new dependency.
7. [x] Docs: README TODO #8 closure, recovery-code warning copy,
   AGENT.md §8 status.

## Appendix A: Phase 0 spike results (linux/arm64, 2026-06-12)

Candidate: `github.com/mutecomm/go-sqlcipher/v4 v4.4.2` (the only
maintained-ish self-contained binding found; registers the
`"sqlite3"` driver name, so it REPLACES `mattn/go-sqlite3` in the
build — both cannot coexist in one binary). Spike sources:
`docs/design/spike-sqlcipher/` (run on macOS to extend this table).

**Functional results — all PASS** against PMForge's usage profile
(WAL + foreign keys + charts-like schema):

| Check | Result |
|---|---|
| Encrypted create via DSN `_pragma_key=x'<64 hex>'` (raw keyspec) | PASS |
| `PRAGMA journal_mode=WAL` + `foreign_keys=ON` on encrypted db | PASS (`wal`) |
| `PRAGMA integrity_check` / `cipher_integrity_check` | ok / 0 failures |
| Wrong key rejected; keyless open rejected; right key reads back | PASS |
| File header randomised (`IsEncrypted()` helper = true) | PASS |
| Plaintext → encrypted migration via `ATTACH` + `sqlcipher_export()` | PASS (row counts match; output encrypted) |
| Clean build, no system deps (no OpenSSL; libtomcrypt bundled) | PASS, 15 s clean build |

**Performance** (5000-row insert tx + LIKE scan, three runs each):

*linux/arm64 (sandbox):*

| Metric | mattn v1.14.22 (plaintext) | go-sqlcipher (encrypted) | Overhead |
|---|---|---|---|
| insert 5000 rows | ~6.0–6.1 ms | ~15.6–22.6 ms | ~2.6–3.7× |
| full scan | ~330–343 µs | ~380–410 µs | ~15–20% |
| spike binary size | 6.85 MB | 6.68 MB | comparable |

*macOS arm64 (James's Mac mini, 2026-06-12 — all functional checks
PASS ×3, identical to linux):*

| Metric | mattn v1.14.22 (plaintext) | go-sqlcipher (encrypted) | Overhead |
|---|---|---|---|
| clean build | — | 9.5 s wall | — |
| insert 5000 rows | 7.7–13.8 ms | 14.5–20.6 ms | ~1.5–1.9× |
| full scan | 439–792 µs | 349–673 µs | within noise |
| spike binary size | 6.84 MB | 6.70 MB | comparable |

Absolute costs are negligible for PMForge's single-user, KB-scale
documents; the relative write overhead is page encryption doing its
job, and on macOS encrypted reads were indistinguishable from
plaintext.

**The principal finding against adopting v4.4.2 as-is — staleness.**
The binding's MAINTENANCE file pins mattn `v1.14.5`, SQLCipher
`4.4.2`, and libtomcrypt from 2020-08-29; the bundled engine reports
`sqlite_version() = 3.33.0` (2020) vs `3.45.1` in PMForge's current
driver. PMForge's SQL uses nothing newer than 3.33 (no STRICT,
RETURNING, or JSONB), so compatibility risk is low — but an
*encryption* feature built on a crypto stack frozen in 2020 misses
five-plus years of upstream SQLite/SQLCipher fixes. Before
implementation, evaluate in this order: (A1) a maintained fork of
go-sqlcipher tracking current SQLCipher; (A2) building
`mattn/go-sqlite3 -tags libsqlite3` against a vendored current
SQLCipher (keeps SQLite fresh, reintroduces per-OS packaging work);
(A3) accepting v4.4.2 with the staleness documented as a known risk.
The key hierarchy, migration plan, and all other sections of this ADR
are binding-independent and unaffected.

## Appendix B: implementation status

- **Step 3 (key hierarchy) — done 2026-06-12.**
  `internal/crypto/keywrap.go`: `GenerateDEK` / `WrapKey` /
  `UnwrapKey` (base64 blobs over the existing Argon2id + AES-256-GCM
  construction) and `KeyspecHex` for the future `PRAGMA key` raw
  keyspec. `internal/users/dek.go`: probe-guarded migrations add
  `users.wrapped_dek_pw` and `recovery_codes.wrapped_dek`;
  `UnlockDEK` unwraps at login and lazily creates DEKs for pre-ADR
  accounts. `IssueRecoveryCodes` wraps the session DEK into every
  code; `ResetWithRecoveryCode` re-wraps the SAME DEK under the new
  password (legacy un-wrapped codes generate a fresh DEK — safe only
  pre-encryption, and the encryption-enable flow must force a code
  re-issue). `App` holds the unlocked DEK in session memory (set at
  login/create, zeroed at logout); no frontend API changes were
  needed. Tests include the data-survival invariant: reset via
  recovery code yields the identical DEK under the new password.
- **Steps 4–7 — done 2026-06-13.**
  `internal/sqlitedriver` centralizes SQLCipher driver registration.
  `internal/db/encryption.go` adds encrypted open, file-header
  detection, SQLCipher integrity checks, and plaintext-to-encrypted
  migration through `sqlcipher_export`. Project create/open paths use
  `InitEncryptedDB` with the session DEK; Project Settings exposes
  the opt-in migration and forces recovery-code reissue before
  encrypting a legacy plaintext project. Repair, encrypted snapshot
  swap, backup archive regression coverage, and headless maintenance
  paths now open encrypted databases with the DEK or authenticated
  credentials. `scripts/validate-encrypted-db.sh`,
  `make check-encrypted-db`, and `scripts/check-release.sh` provide
  the encrypted database release gate. `.pmba` archives preserve the
  encrypted `project.pmforge` bytes.

## Appendix C: bbolt considered — not a value add

Question raised at A3 acceptance: would `go.etcd.io/bbolt` (pure-Go
B+tree key-value store) add value alongside or instead of SQLite?

- **As the main store:** no. PMForge's model is relational — 15+
  tables, foreign keys with CASCADE, secondary indexes, ad-hoc SQL
  (filters, ordering, aggregation), and an additive
  `CREATE TABLE IF NOT EXISTS` migration story. bbolt offers none of
  that (manual buckets, manual indexes, manual migrations) and —
  decisive for THIS ADR — has no encryption layer at all, so
  adopting it would mean building page/value encryption by hand,
  recreating exactly the problem SQLCipher already solves. A
  migration would be a rewrite of `internal/db` for strictly less
  capability.
- **As a side store for wrapped DEKs / settings / caches:** no. The
  wrapped-DEK blobs are a few hundred bytes in two columns of
  `system.db`, which must exist before login anyway (the
  bootstrapping store). A second storage engine for that adds a
  dependency, a second file format in backups/repair/REUSE, and a
  second at-rest surface to secure — for zero new capability.
- **Hypothetical future where bbolt is still not the answer:** if a
  pure-Go no-CGO build ever matters (wasm/mobile), the natural
  candidate is a CGO-free SQLite port (e.g. `modnc`-style drivers),
  which keeps the SQL surface — not a KV store.

**Verdict: bbolt is not a value add for PMForge; rejected.**

**A1 fork survey (2026-06-12):** a web survey found no fork that
demonstrably tracks CURRENT SQLCipher. The most notable
(`grassto/go-sqlcipher`) exists to resolve the `"sqlite3"`
driver-name conflict with mattn, not to refresh the bundled engine;
others are dormant copies. Unless a deeper review finds otherwise,
A1 is effectively unavailable. James selected **A3** on 2026-06-12:
adopt v4.4.2 as-is because it is the smallest proven path on both dev
platforms, with the 2020-frozen engine documented as a known risk and
`sqlcipher_export` retained as the future escape hatch to whatever
replaces it.
