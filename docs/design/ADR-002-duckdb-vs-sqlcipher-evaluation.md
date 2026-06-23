<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-002: Evaluating DuckDB (Go client) as a replacement for SQLite + SQLCipher

**Status:** **Accepted — Option B (complementary analytical engine).** SQLCipher remains the primary transactional store; DuckDB is added as an optional, in-memory analytical engine. Decided 2026-06-23 by James L. Burns. Design + plan: [duckdb-analytics-engine.md](duckdb-analytics-engine.md).
**Date:** 2026-06-23
**Deciders:** James L. Burns (project owner)
**Supersedes / relates to:** [ADR-001 — Per-user database encryption at rest](ADR-001-database-encryption-at-rest.md)

> This ADR evaluates migrating PMForge's persistence layer from
> SQLite (`mattn`-style driver) + SQLCipher (`mutecomm/go-sqlcipher/v4`)
> to DuckDB via the official Go client (`github.com/duckdb/duckdb-go/v2`).
> The evaluation is grounded in the DuckDB 1.5 documentation and the
> 2025-11-19 "Data-at-Rest Encryption in DuckDB" engineering blog, and
> in a footprint audit of PMForge's current DB layer.

## Context

PMForge is a **local-first, single-machine, single-user desktop app**
(Wails). Its persistence layer is the product's backbone *and* a core
security selling point:

- Each user's project lives in an individual **SQLCipher-encrypted**
  `.pmforge` file. A per-user 32-byte DEK is the raw SQLCipher key,
  wrapped by the password **and** by each Argon2id recovery code
  (ADR-001 key hierarchy). `system.db` stays plaintext.
- The DB layer is **22 tables**, **17 foreign-key references**, rowid
  `INTEGER PRIMARY KEY` autoincrement, ~11 `ON CONFLICT … DO UPDATE`
  upserts, `strftime('%Y-%m-%dT%H:%M:%fZ','now')` timestamps, and a
  `PRAGMA user_version`-driven `Migrate()`.
- Durability/recovery leans on SQLite primitives: `PRAGMA
  journal_mode=WAL`, `foreign_keys=ON`, and a **self-heal subsystem**
  built on `PRAGMA integrity_check` / `PRAGMA cipher_integrity_check`
  with atomic snapshot swap (`repair.go`), plus `.pmba` backups that
  preserve the encrypted bytes verbatim (`backup.go`).
- Release gates encode these assumptions: `scripts/validate-encrypted-db.sh`,
  `scripts/release-gate-scope-check.sh` (fails if README stops
  documenting SQLCipher or `go.mod` drops `go-sqlcipher/v4`), and
  `check-release.sh`.

So "swap the database" is not a driver change — it touches the schema,
the encryption model, the self-heal/backup design, the release gates,
and ADR-001.

## What DuckDB actually is (verified, 2026-06-23)

| Dimension | Finding | Source |
|---|---|---|
| Go client | `github.com/duckdb/duckdb-go/v2`, v1.5.3, `database/sql`, **CGO** (bundles the DuckDB C++ engine) | docs/clients/go |
| Engine type | **OLAP / columnar**, vectorized; optimized for analytical scans, not many small row writes | duckdb.org/why_duckdb |
| Concurrency | **One read-write process**, or many read-only processes. Multi-writer needs Quack (beta in 1.5.2, "mature ~v2.0, fall 2026") or DuckLake+PostgreSQL. Optimistic concurrency → same-row edits raise *transaction conflict* (retry) | docs/connect/concurrency |
| Encryption | **Yes, since v1.4 (Nov 2025).** AES-GCM-256 / CTR-256; encrypts main file, WAL, and temp files; KDF + memory-locked secure key cache; `ATTACH 'db' (ENCRYPTION_KEY '…')` | encryption blog; docs/sql/statements/attach |
| Transactions | A single transaction may write to **only one** attached database | docs/sql/statements/attach |
| SQLite interop | Can `ATTACH … (TYPE sqlite)` to read/write **plaintext** SQLite files (not SQLCipher-encrypted ones) | docs/core_extensions/sqlite |

### The encryption maturity picture (decisive for PMForge)

DuckDB's encryption is real and reasonably designed, but it is **new and
carries caveats that directly conflict with PMForge's security posture**:

1. **Not NIST-compliant yet.** DuckDB's own docs: *"DuckDB's encryption
   does not yet meet the official NIST requirements"* (tracking issue
   #20162, "Store and verify tag for canary encryption").
2. **A shipped RNG vulnerability.** After 1.4.0, security researchers
   found an RNG flaw in the MbedTLS path (advisory `GHSA-vmp8-hg63-v2hp`).
   DuckDB's response: **disable writing to encrypted databases in
   MbedTLS mode** from 1.4.1.
3. **Writing encrypted data now requires the `httpfs` (OpenSSL)
   extension.** From 1.4.2, DuckDB *"tries to auto-install and auto-load
   the httpfs extension whenever a write is attempted."* `httpfs` is the
   HTTP/S3 filesystem extension. For a **local-first, offline,
   no-telemetry** app, an engine that auto-downloads a network extension
   from the internet by default in order to write encrypted data is a
   serious threat-model and packaging problem (must pre-bundle the
   extension and disable autoinstall/autoload).
4. **Storage version pin.** Encryption implies `STORAGE_VERSION ≥
   v1.4.0`; older DuckDB builds cannot open the file.

By contrast, SQLCipher is a 15+ year, widely-deployed, offline,
self-contained, FIPS-capable engine with no extension/network
dependency for crypto. Trading it for a seven-month-old, non-NIST,
extension-dependent encryption path is a **maturity regression** for the
exact feature PMForge advertises.

**Two further points worth recording, because popular tutorials gloss
over them:**

- **Same threat model, not a stronger one.** DuckDB's encryption (like
  SQLCipher's) protects data *at rest* — against file theft, storage
  access, and VM compromise — but **not** against memory dumps or
  inspection of the running process. So switching engines yields **no
  security gain** on the axis PMForge cares about; it only changes
  *which* implementation (and maturity level) PMForge depends on.
- **Third-party "it just works" write-ups are out of date.** Tutorials
  published right after the 1.4.0 launch (e.g. byteiota, 2025-11-21)
  describe MbedTLS-by-default with "no external dependencies." That was
  superseded within weeks: the RNG CVE disabled MbedTLS *writes*, and
  1.4.2 routes encrypted writes through auto-installed `httpfs`/OpenSSL.
  Evaluate against DuckDB's own release notes, not launch-week tutorials.

## The case *for* DuckDB (steelman)

To be fair, there are genuine attractions, and defenders of a migration
would emphasize:

- **Superior analytics.** Window functions, rich aggregates, `PIVOT`,
  `QUALIFY`, `SUMMARIZE` — a much better engine for EVM rollups, Six
  Sigma statistics, and cross-project portfolio aggregation than SQLite.
- **Direct file ingestion.** Native CSV/Parquet/**Excel**/JSON readers
  could absorb the Sigma spreadsheet-import path (potentially retiring
  `read-excel-file`) and power "import any data" features.
- **PostgreSQL-flavored SQL** is more expressive and more portable than
  SQLite's dialect.
- **Encryption is comprehensive** where it exists (DB + WAL + temp
  files), and **performance overhead is small** with OpenSSL
  (near-parity in DuckDB's own TPC-H Power test).
- **One dependency** could cover storage *and* analytics *and*
  file import/export.
- **External-database connectors.** `postgres` / `mysql` / `sqlite` /
  `odbc` scanners `ATTACH` and query external databases — directly
  relevant to PMForge's planned external-DB-via-plugin requirement
  (owner direction, 2026-06-23). This is a genuine point in DuckDB's
  favor for the **Option B** analytics/connector role (not for replacing
  the encrypted transactional store).

These are real, but they are overwhelmingly **analytical/read-side**
benefits — DuckDB's home turf — not transactional-store benefits.

## Evaluation against PMForge's needs

| Concern | SQLite + SQLCipher (today) | DuckDB | Verdict |
|---|---|---|---|
| Workload fit | OLTP: many small CRUD writes — SQLite's sweet spot | OLAP columnar; small row updates/deletes are its weak spot | **SQLite** (DuckDB is the wrong tool for the transactional core) |
| Encryption maturity | Battle-tested, offline, self-contained | New (Nov 2025), non-NIST, needs OpenSSL/`httpfs`, had an RNG CVE | **SQLite/SQLCipher** |
| Offline / local-first | Fully offline | Auto-install/auto-load of `httpfs` to write encrypted data by default | **SQLite/SQLCipher** |
| Self-heal / integrity | `PRAGMA integrity_check` + `cipher_integrity_check` + atomic swap | **No `PRAGMA integrity_check` equivalent**; different corruption model — entire `repair.go` design would need rethinking | **SQLite** |
| Schema port | N/A | 22 tables: autoincrement→`CREATE SEQUENCE`, `user_version`→meta table, upsert/FK semantics to re-verify, `strftime` rewrite | Migration cost (high) |
| Backup model | `.pmba` copies encrypted bytes | Different on-disk format; rewrite backup/restore + repair swap | Migration cost |
| Binary size | SQLCipher is small | DuckDB bundles a large C++ engine (tens of MB) → bigger desktop download, heavier Wails release matrix | **SQLite** |
| Format stability | SQLite format is famously stable "forever" | Improving (explicit `STORAGE_VERSION`), but encryption pins ≥1.4.0; younger compatibility track record | **SQLite** |
| Release gates / ADRs | Encoded and green | Rewrite `validate-encrypted-db.sh`, `release-gate-scope-check.sh`, ADR-001 | Migration cost |
| Analytics / reporting | Adequate for current features | **Materially better** | **DuckDB** |
| File import (CSV/xlsx/Parquet) | App-layer parsers | **Native, excellent** | **DuckDB** |

## Decision / Recommendation

**Do not replace SQLite + SQLCipher as PMForge's primary transactional
store.** The migration is high-cost (schema + encryption + self-heal +
backup + gates + ADR rewrite), high-risk (seven-month-old, non-NIST,
extension-dependent encryption replacing a hardened one; loss of
`integrity_check`-based self-heal; binary bloat; format-stability
exposure), and the upside is almost entirely **analytical**, which does
not require replacing the transactional core.

This preserves the ADR-001 decision and PMForge's local-first,
mature-encryption posture.

### Recommended alternative — DuckDB as a complementary analytical engine (Option B)

If the motivation is better analytics/reporting and frictionless data
import (the likely goal), adopt DuckDB **alongside** SQLCipher, not
instead of it:

- Keep the encrypted SQLite file as the **system of record**.
- Spin up an **in-memory / ephemeral DuckDB** for heavy analytical
  queries (EVM, Six Sigma stats, portfolio rollups) and for **direct
  CSV/Parquet/Excel/JSON ingestion**. The app layer feeds DuckDB
  decrypted rows (or a temporary decrypted export) — never the encrypted
  file directly.
- No change to the at-rest security model; DuckDB touches only data the
  app already has in memory; no persistent DuckDB file, so no new
  encryption surface, no `httpfs` auto-install, no format-stability
  exposure.
- Gate it behind a build tag so the heavy CGO dependency is optional.

This captures DuckDB's real strengths with near-zero risk to the core.

### If a full migration is pursued anyway — phase it like ADR-001

Do **not** do a big-bang swap. Mirror ADR-001's spike-gated approach:

- **Phase 0 — Binding & viability spike (no `go.mod` change).** In a
  throwaway branch, prove on **all three** release platforms:
  (a) `duckdb-go/v2` builds under CGO and the Wails release matrix;
  (b) measured **binary-size** delta is acceptable for a desktop
  download; (c) **encrypted write works fully offline** with a
  *pre-bundled* `httpfs`/OpenSSL and `autoinstall/autoload` disabled
  (no network); (d) the DEK → `ENCRYPTION_KEY` mapping round-trips and
  recovery-code rewraps still preserve data; (e) a representative slice
  of the schema ports (sequences, upserts, FK enforcement, timestamps).
  **Go/no-go gate:** if (b), (c), or NIST/CVE posture is unacceptable,
  stop here and keep SQLCipher.
- **Phase 1 — Dialect & schema port** behind a `duckdb` build tag, with
  a parallel test suite; keep SQLCipher as default.
- **Phase 2 — Self-heal/backup redesign** for DuckDB's format (no
  `integrity_check`; define a new corruption-detection + snapshot-swap
  strategy).
- **Phase 3 — Migration tool** (`COPY FROM DATABASE` / per-table copy)
  to move existing users' encrypted SQLCipher data into encrypted
  DuckDB, with a `.bak` retention guarantee.
- **Phase 4 — Release gates + ADR-001 rewrite**, version bump, and a
  full `make check-release` round-trip on the encrypted DuckDB path.

## Risks & open questions

- **Encryption posture is the gating risk.** Until #20162 closes
  (NIST/canary tag) and the MbedTLS write path is restored — or `httpfs`
  is comfortably pre-bundled offline — DuckDB encryption is weaker, in
  maturity terms, than what PMForge ships today.
- **No `integrity_check` analogue** undermines the self-heal feature
  that is currently a differentiator.
- **Single-writer-DB-per-transaction** and optimistic-conflict retries
  are a poor match for transactional CRUD ergonomics (though the
  existing `App.mu` single-writer discipline mitigates concurrency).
- **Binary size / build complexity** for a downloadable desktop app.
- **Open question for the owner:** what is the actual motivation —
  analytics, SQL expressiveness, native file import, or a general
  modernization preference? If analytics/import, **Option B** delivers
  it without the risk.

## Sources

- [DuckDB Go Client](https://duckdb.org/docs/current/clients/go)
- [DuckDB Concurrency](https://duckdb.org/docs/current/connect/concurrency)
- [DuckDB ATTACH / Database Encryption](https://duckdb.org/docs/current/sql/statements/attach)
- [Data-at-Rest Encryption in DuckDB (2025-11-19)](https://duckdb.org/2025/11/19/encryption-in-duckdb)
- [Securing DuckDB](https://duckdb.org/docs/current/operations_manual/securing_duckdb/overview)
- PMForge: `internal/db/`, `internal/sqlitedriver/`, `internal/crypto/keywrap.go`, `docs/design/ADR-001-database-encryption-at-rest.md`
