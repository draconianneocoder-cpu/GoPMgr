<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 P02/S0 — Source, storage and performance baseline

**Status:** Evidence for owner review. P02/S0 is authorized by the
[maintainer acceptance](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md).
This report changes no production code, schema or behavior. It sets no
budgets: OWN-10 reserves hardware profiles and numeric budgets for the
owner after S0.\
**Date:** 2026-10-25. **Source commits measured:** `91cf867` (benchmark),
`3be026f` (file-system probe) and `9eff4a2` (SQL probe), on branch
`claude/adr010-s0-baseline` from `3d00f53`. The later commits only add or
relabel probes; none changes the benchmark or production code.\
**Harness:** [`spikes/adr010-s0-baseline`](../../spikes/adr010-s0-baseline/README.md).
**Raw evidence:** `spikes/adr010-s0-baseline/evidence/*.jsonl`.

## 1. Summary

| # | Finding | Evidence | Why it matters for ADR-010 |
|---|---|---|---|
| F1 | One single-field edit of a 50,000-task chart takes **2.5 s p50, 3.8 s p95** (4.8 s p50 at 100,000). Each save rewrites the whole chart document. | §5, T2 storage | OWN-10 requires "genuinely usable, editable 50,000-task schedules". The current chart-document model does not meet that. This supports the normalized schedule store (D-08, ADR-011). |
| F2 | Each chart save writes the **whole chart twice into the audit log** (before and after). One single-field edit adds **7.0 MB** of audit data at 10,000 tasks, **35.5 MB** at 50,000 and **71.3 MB** at 100,000. | `internal/db/charts.go:111-131`; §5 | The audit chain stays correct, but its size grows with the schedule size times the edit count. This is the S7B audit-split input. |
| F3 | Gantt layout is roughly quadratic: **2.1 s** at 10,000 tasks, **61 s** at 50,000, **288 s** at 100,000. | §5; CPU profile §6.1 | The cause is over-allocation detection, which rescans every task for each over-allocated resource-day. Large-schedule views need a bounded, incremental path. |
| F4 | A 100,000-task MSPDI file (73.4 MB) is **larger than the 64 MiB import cap**. | `app_charts.go:920`; T3 MSPDI | The current Quick Chart Import cannot accept a T3 file of this shape. The cap is a deliberate safety limit and is not changed here. |
| F5 | The database engine is SQLite **3.33.0** in SQLCipher **4.4.2** community (libtomcrypt). It has **no** JSON functions, FTS5, `RETURNING`, `STRICT` or `DROP COLUMN`. | §4.3 | Confirms the plan's §5.2 restriction with direct evidence. ADR-011 must design without these. |
| F6 | New project databases carry no schema version stamp (`user_version` is 0). Migrations run in code on every open. | §4.1 | Supports the plan's §5.3 format gate: there is no existing writer gate to build on. |
| F7 | CPM is fast at every size: 24 ms at 10,000 tasks, 205 ms at 50,000, 577 ms at 100,000. | §5 | Schedule calculation is not the bottleneck. Storage, audit and layout are. |

**Not yet done (S0 remains open):** representative real-world fixtures
(MPP, MSPDI, XER, POD) and measurements on owner-approved hardware. See §8.

## 2. Method

- **Generator.** `s0bench` builds a seeded synthetic schedule (PCG seed
  20261024). Each task gets up to three predecessors from the previous 60
  tasks with FS, SS or FF links and lags. Each task has one or two
  assignments over 300 resources. 2% are milestones.
- **Tiers.** T0 500, T1 10,000, T2 50,000 and T3 100,000 tasks, matching
  the ADR-010 tiers.
- **Samples.** 30 per measure at T0 and T1, and 10 at T2 and T3. MSPDI
  uses 3 samples. Gantt layout stops once its samples exceed 120 s in total;
  the record says how many samples it took.
- **Storage.** The benchmark creates a fresh encrypted project database through
  `db.InitEncryptedDB`, the same path the app uses. It saves the chart, then
  makes single-field edits through `SaveChart`, and reads the audit growth from
  `audit_events`. It verifies the chain with `VerifyAuditChain`. The
  database is deleted afterwards.
- **Hardware.** A cloud container: 4 vCPU Intel Xeon @ 2.10 GHz, 16 GB RAM,
  Linux amd64, Go 1.27.3. **This is not an approved hardware profile.**
  Treat every number as a relative baseline, not a pass or fail.

### 2.1 Limits of this method

- The data is synthetic. ADR-010 D-18 says not to assert performance on
  synthetic-only sources. These numbers locate bottlenecks; they do not
  set budgets.
- The generator's critical path grows with size (336 working days at T0,
  54,794 at T3). Real 50,000-task programs run more work in parallel, so
  they have shorter horizons. That changes F3: fewer over-allocated
  resource-days, but each one still rescans all tasks.
- The benchmark measures Go calls, not the desktop UI. First paint,
  scroll and search in the WebView are not measured here.

## 3. Repository census

### 3.1 Writers to a project database

| Writer | Where | Notes |
|---|---|---|
| Desktop app | Wails bindings in the root package | The normal path. |
| Headless project mode | `main.go:400` (`headlessProjectMode`), `main.go:448` (`runHeadlessRepair`) | Opens an encrypted project under `<root>/<username>/projects` (`main.go:516`). |
| Backup restore | `internal/db/backup_restore.go` | Archive entries depend on `schema_version` (`internal/db/backup.go:30`). |

ADR-010 §5.3 requires a format gate checked "before every writer". All
three rows above are writers that the gate must cover.

### 3.2 Database settings

| Database | DSN settings | Where |
|---|---|---|
| Project (`.gopmgr`) | WAL, `synchronous=NORMAL`, foreign keys on, busy timeout 5 s | `internal/db/sqlite.go:75` |
| Deletion log | WAL, `synchronous=FULL` | `internal/deletionlog/log.go:84` |
| Catalog | WAL, `synchronous=NORMAL` | `internal/catalog/store.go:82` |
| Accounts (`system.db`) | WAL, SQLite default synchronous | `internal/users/store.go:141` |

### 3.3 Size limits in force today

| Limit | Value | Where |
|---|---|---|
| MSPDI import file | 64 MiB | `app_charts.go:920` |
| Analytics query result rows | 10,000 | `internal/analytics/duckdb.go:187` |
| WBS payload | 8 MiB | `internal/wbs/model.go:18` |
| WBS nodes | 10,000 | `internal/wbs/model.go:19` |
| WBS levels | 64 | `internal/wbs/model.go:20` |
| WBS text field | 64 KiB | `internal/wbs/model.go:21` |
| WBS identifier | 256 bytes | `internal/wbs/model.go:22` |
| WBS associations | 50,000 | `internal/wbs/model.go:23` |
| WBS receipts kept | 256 | `internal/db/wbs_store.go:26` |
| Cost attachment | 10 MiB | `internal/db/cost_control_attachments.go:21` |

The 10,000-row analytics cap limits one query's result. It is not a
database maximum (ADR-010 anti-degradation rule).

### 3.4 Import boundary

- The import dialog offers `*.xml;*.mpp;*.mpx;*.pod` (`app_charts.go:897`).
- The importer routes by file extension, not content. It refuses `.mpp`,
  `.mpx` and `.pod` with an explanation (`app_charts.go:940-952`) and
  sends everything else to the MSPDI parser.
- Recommendation for G2/G3 design: route by content signature, and do not
  offer formats in the file dialog that the build cannot read.

## 4. Storage and file-system baseline

### 4.1 Schema

- `internal/db/sqlite.go` holds 37 `CREATE TABLE` statements and 28
  `ALTER TABLE` statements. `internal/db/wbs_store.go` adds 4 more tables.
  There are 34 persistent table names, counting the WBS tables;
  `sigma_projects` is rebuilt through `sigma_projects_new`.
- Migrations are idempotent code run at open. `PRAGMA user_version` is
  never set; it is only copied when a plaintext database is converted
  (`internal/db/encryption.go:299-303`). A fresh project reads
  `user_version = 0` (§4.3).

### 4.2 File-system probe (`s0fs`)

Linux container, ext4, at the temp folder, the home folder and the nearest
existing parent of the default data root (`~/Documents/GoPMgr` did not
exist yet):

| Check | Result |
|---|---|
| No-replace rename of a file onto an existing file (`renameat2 RENAME_NOREPLACE`) | Refused, as required |
| No-replace rename of a folder onto an existing empty folder | Refused, as required |
| Plain `rename(2)` of a folder onto an existing empty folder | **Replaced it.** Go's `os.Rename` must not be used for sealed publication. |
| Case-insensitive names | No |
| Second exclusive `flock` | Refused |
| Folder `fsync` | Works |
| Path of 286–288 characters | Works |
| Sync-folder hint | None recognized (a naming hint only; INT-03 forbids acting on it) |

macOS (`RenamexNp RENAME_EXCL`) and Windows (`MoveFileEx` without
`MOVEFILE_REPLACE_EXISTING`) probes are written and cross-compile, but
**have not run**. They are BLOCKED until a runner executes them.

### 4.3 SQL engine features (`s0sql`)

Effective settings on a fresh encrypted project: WAL, `synchronous=1`
(NORMAL), foreign keys on, page size 4096, cipher page size 4096, KDF
PBKDF2-HMAC-SHA512 with 256,000 iterations, HMAC-SHA512, provider
libtomcrypt 1.18.2-develop.

Compile options: `ENABLE_FTS3`, `ENABLE_FTS3_PARENTHESIS`, `ENABLE_RTREE`,
`ENABLE_UPDATE_DELETE_LIMIT`, `HAS_CODEC`, `OMIT_DEPRECATED`,
`TEMP_STORE=2`, `DEFAULT_WAL_SYNCHRONOUS=1`, `THREADSAFE=1`,
`DISABLE_INTRINSIC`, compiler gcc 13.3.0.

| Feature | Works | Feature | Works |
|---|---|---|---|
| UPSERT | Yes | JSON functions (`json_extract`) | **No** |
| `RENAME COLUMN` | Yes | JSON `->>` operator | **No** |
| Window functions | Yes | FTS4 | Yes |
| Generated columns | Yes | FTS5 | **No** |
| `RETURNING` | **No** | R-tree | Yes |
| `DROP COLUMN` | **No** | `WITHOUT ROWID` | Yes |
| Math functions (`sqrt`) | **No** | Partial indexes | Yes |
| `STRICT` tables | **No** | Deferred foreign keys | Yes |
| `PRAGMA cipher_integrity_check` | Yes | | |

## 5. Measurements

p50 / p95 in milliseconds unless stated. "n" is the sample count. All 24 records passed; the full run took 11 min 16 s.

| Measure | T0 (500) | T1 (10,000) | T2 (50,000) | T3 (100,000) |
|---|---|---|---|---|
| Dependency links | 718 | 14,887 | 74,996 | 150,089 |
| CPM (n=30/30/10/10) | 0.84 | 24.2 | 205 | 577 |
| Gantt layout | 6.25 (n=30) | 2,096 (n=30) | 61,240 (n=2) | 287,943 (n=1) |
| Over-allocated rows / horizon (days) | 111 / 336 | 2,323 / 5,308 | 11,328 / 27,604 | 22,588 / 54,794 |
| Chart JSON size | 143 KB | 2.95 MB | 15.0 MB | 30.1 MB |
| Chart JSON marshal | 1.66 | 21.8 | 130 | 227 |
| MSPDI size | 381 KB | 7.3 MB | 36.7 MB | **73.4 MB (over cap)** |
| MSPDI export / import (n=3) | 6.7 / 21.2 | 115 / 318 | 608 / 1,526 | 1,332 / 3,118 |
| Create encrypted chart | 10 | 236 | 1,449 | 2,334 |
| One-field edit p50 / p95 | 16.9 / 39.2 | 489 / 580 | **2,519 / 3,764** | **4,791 / 6,205** |
| Audit bytes per one-field edit | 341 KB | 7.0 MB | 35.5 MB | 71.3 MB |
| Open chart and parse p50 | 2.7 | 75.7 | 407 | 802 |
| Verify audit chain | 169 (32 events) | 4,101 (32 events) | 7,655 (12 events) | 13,098 (12 events) |
| Database / WAL size after the edits | 10.9 / 4.3 MB | 222 / 7.9 MB | 397 / 51.9 MB | 795 / 73.2 MB |
| Process peak RSS | 29 MB | 218 MB | 710 MB | 1,266 MB |

T0 and T1 storage made 30 edits. T2 and T3 made 10, so their database sizes
are not directly comparable with T1.

## 6. Analysis

### 6.1 Gantt layout (F3)

A CPU profile of the T0–T1 run puts 68% of all CPU time in
`kernel.DetectOverallocationsWithPlan`. Almost all of it is in
`tasksOnDay` (`internal/kernel/resources.go:364`), which loops over every
task for each over-allocated resource-day and calls `occupiesDay`. The
cost is about (over-allocated resource-days) × (tasks). Both grow with
size here, which gives the near-quadratic curve. `LayoutGantt` always runs this
step (`internal/charts/dag/gantt.go:58`).

This report proposes no fix; S0 is evidence only. A future slice could
index tasks by day and resource once per layout, which makes the cost about
linear in assignments and days. Any such change needs its own tests and
an authorized slice.

### 6.2 Edits and audit (F1, F2)

`SaveChart` writes the full chart row, then appends an audit event whose
before and after values are the whole chart serialized as JSON
(`chartAuditJSON`, `internal/db/charts.go:232`). The chart's data field
is itself JSON, so it is escaped again, which explains the factor of about
2.4 between chart size and audit bytes per edit. The cost of a small edit
therefore grows with the whole schedule, not with the change.

ADR-010 keeps the existing hash chain authoritative (§5.2, audit bridge).
The S7B design needs a field-level or delta payload that the chain can
still verify. This report does not change audit behavior.

### 6.3 Memory

Peak RSS reached 710 MB at T2 and 1.27 GB at T3, mostly the in-memory chart, the MSPDI round
trip and SQLite page cache in one process. The ADR-010 worker design moves
MPXJ parsing into a separate, bounded JVM. Host-side memory for a 50,000-task
edit session still needs its own budget.

## 7. Hardware profiles for owner decision (OWN-10)

OWN-10 reserves these choices for the owner. These are **candidates to
measure on**, not budgets:

| Profile | Candidate | Reason |
|---|---|---|
| Minimum | 4-core x86-64 (2019 or later) or Apple M1, 8 GB RAM, SSD; Windows 10 22H2+, macOS 14+, a current LTS Linux | Close to this container's CPU class; 8 GB keeps T2 within reach. |
| Recommended | 8-core x86-64 or Apple M2 or later, 16 GB RAM, NVMe SSD | T3 stress testing and parallel workers. |

Once the owner picks profiles, the same harness runs unchanged on them.
Budgets can then be set from measured numbers instead of this container.

## 8. Open S0 work and blockers

| Item | State | Needed |
|---|---|---|
| Representative fixtures (MPP, MSPDI, XER, POD with calendars, baselines, timephased data) | **BLOCKED** | A fixture set the owner may license into the repo, or approval to source public samples with recorded licenses |
| Measurements on owner-approved hardware | **BLOCKED** | OWN-10 profile choice |
| macOS and Windows `s0fs` runs | **BLOCKED (not run)** | A CI runner or the owner's machines |
| UI timings (first paint, scroll, search) at T1–T3 | Not started | A frontend harness; out of this Go-only baseline |

## 9. Reproduce

```sh
cd spikes/adr010-s0-baseline
head=$(git rev-parse --short HEAD)
go run ./cmd/s0fs   -head "$head" -out evidence/linux-amd64-container-fs.jsonl
go run ./cmd/s0sql  -head "$head" -out evidence/linux-amd64-container-sql.jsonl
go run ./cmd/s0bench -head "$head" -out evidence/linux-amd64-container-bench.jsonl
```
