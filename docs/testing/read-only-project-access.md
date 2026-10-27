<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Read-only project access (Slice B)

**Authority:**
- the maintainer's [architecture directive](../review/architecture-directive-2026-10-27.md), item 9 and §4;
- the Slice B directive of 2026-10-27;
- [ADR-011](../design/ADR-011-normalized-schedule-store.md) Amendment 1.

**Base:** `main` at `ec7b73a`. No schema migration is activated, and the writable open path is unchanged.

## 1. The problem

`InitDB` and `InitEncryptedDB` open a project for writing. They create a missing file, switch it to WAL, run `Migrate()` and tighten its permissions. Several paths that only read a project used them, so reading a project could upgrade its schema, backfill money columns, rewrite timestamps and change its journal mode.

## 2. Every path that opens a project database

Inventory at `ec7b73a`, from `rg 'InitEncryptedDB\(|InitDB\(|OpenEncryptedCopyReadOnly\(|MigratePlaintextToEncrypted\('` over non-test Go files.

| Path | Intent | Before | After |
| --- | --- | --- | --- |
| `ListProjects` (`enumerateProjects`) | Inspect names | Filenames only; never opened a database | Unchanged |
| `ProjectsOverview` | Read | `InitEncryptedDB` on every project: migrated each one, and loaded every chart and document to count them | Read-only; counts by `COUNT(*)`; typed `unreadable_reason` |
| `projectDeletionRequest` | Read | `InitEncryptedDB`: migrated the project before recording its deletion | Read-only; works on older schemas |
| `RunPortfolioAnalytics` | Read | `InitEncryptedDB` on every project | Read-only. Older or future-format projects are named in one typed error |
| `OpenEncryptedCopyReadOnly` (administrator copy) | Read | Migrated the copy, checked its integrity, then reopened it query-only | Read-only from the start. Integrity is checked first; an older copy is refused |
| Headless `--check`, `--schema-dump`, `--stats`, `--export-audit`, `--export` | Read | `InitDB`/`InitEncryptedDB`: migrated | Read-only. `--stats`, `--export-audit` and `--export` refuse an older schema with `ErrProjectNeedsUpgrade` |
| Headless `--repair`, `--vacuum` | Repair, write | Writable | Unchanged |
| `OpenProject`, `CreateProject`, `CreateProjectFromLaunchpad` | Write, create | Writable, migrates | Unchanged |
| `restoreProjectArchiveFrom` | Restore (creates and renames a new project) | Writable, migrates | Unchanged |
| `RepairAndSwap` recovery, `internal/db/repair.go` | Repair | Writable | Unchanged |
| `EncryptProjectAtRest` → `MigratePlaintextToEncrypted` | Convert | Writable | Unchanged |

The writable paths still run `Migrate()`. Refusing a future format stamp before that migration is the format gate, Slice E.

## 3. The read-only contract

**`db.OpenEncryptedReadOnly(path, dek)`** (and `db.OpenReadOnly` for legacy plaintext files) opens through a SQLite URI with `mode=ro`, so SQLite refuses writes on every pooled connection. It:
- refuses writes even after `PRAGMA query_only` is turned back off; `_query_only=1` is a second layer only;
- never creates a missing file;
- never changes the journal mode or file permissions;
- never runs `Migrate()`.

A file already in WAL mode gets its `-wal` and `-shm` files, as any SQLite reader needs, with the main file's permissions. The main file is never written.

Before returning, it:
1. checks that the path is a regular file, not a symlink;
2. proves the key;
3. refuses a format stamp (`user_version` ≠ 0, ADR-011 §5.1);
4. classifies the schema: **Current** if every table and column the current `Migrate()` creates is present, otherwise **Older**. The reference schema is taken by running `Migrate()` on an empty in-memory database, so it cannot drift.

**Typed errors** (each wraps the SQLite error):

| Error | Cause |
| --- | --- |
| `ErrProjectNotFound` | No file at the path |
| `ErrProjectUnreadable` | Wrong key, or not a SQLite database. SQLCipher reports both as `SQLITE_NOTADB` |
| `ErrProjectDamaged` | `SQLITE_CORRUPT`, from the open or from a later read by `ReadProjectOverview` or `VerifyAuditChainIfPresent` |
| `ErrProjectBusy` | Locked past the busy timeout |
| `ErrProjectInaccessible` | The file exists, but SQLite cannot open it or a file it needs beside it: a mode-000 file (`SQLITE_CANTOPEN`), or a WAL-mode file with no `-shm` in a folder the reader cannot write (`SQLITE_READONLY`). Both were observed on macOS |
| `ErrUnsupportedProjectFormat` | A format stamp this build does not know |
| `ErrProjectNeedsUpgrade` | The operation needs the current schema and the file is Older |

**Schema-tolerant readers:**
- `ReadProjectOverview` uses only the project table's original columns, plus `COUNT(*)` of charts and documents.
- `VerifyAuditChainIfPresent` treats a file without `audit_events` as the empty, valid chain `Migrate()` would create.

So the project list and deletion records work on Older files without upgrading them.

`ProjectSummary` gains an additive `unreadable_reason` field: `not_found`, `unreadable`, `damaged`, `busy`, `inaccessible`, `unsupported_format`, `no_project` or `error`.

SQLite result codes are read through `sqlitedriver.Classify`, so `internal/db` does not import the driver.

## 4. Behavior changes, for the owner

1. **The administrator copy of an Older project is refused** ("its owner needs to open it once"). This follows the directive's §7 wording, but conflicts with B-20. ADR-004 access matters most when the owner is unavailable. The alternative is an explicit, recorded "prepare the private copy" step that upgrades only the administrator's disposable copy.
2. **Preparing the deletion of a project with a newer format stamp fails,** where the old code migrated the file first. Today, unreadable projects already block deletion in the same way.
3. **Portfolio analytics** names every Older or newer-format project in one error, rather than silently upgrading them. Unreadable projects (wrong key, damaged) are still skipped silently, as before. That is a known gap against the directive.
4. **Headless `--stats`, `--export-audit` and `--export`** refuse an Older project and say to open it in GoPMgr.

## 5. Results

Run locally on darwin/arm64 with Go 1.27.3 at the base above. Linux runs in CI on the PR.

| ID | Requirement | Evidence | Result |
| --- | --- | --- | --- |
| B-01 | Inspecting a current project changes nothing | `TestOpenReadOnlyCurrentProject` (DELETE-mode file: main file byte-identical, journal mode unchanged) | PASS |
| B-02 | Inspecting an older project does not upgrade it | `TestOpenReadOnlyLeavesOlderProjectUnchanged`, which also shows the writable path does change the same fixture | PASS |
| B-03 | Portfolio and project list migrate nothing | `TestProjectsOverviewNeverWritesProjects`, `TestRunPortfolioAnalyticsNeverWritesProjects` (`-tags duckdb`), `TestPortfolioProjectMetricsOnAReadOnlyHandle` | PASS |
| B-04 | Administrator copy is never migrated | `TestAdminProjectCopyNeverMigrates`, `TestOpenEncryptedCopyReadOnlyRefusesOlderSchema`; `TestAdminProjectCopyOfALiveWALProject`: the owner holds the file open with commits only in `-wal`, the view sees them, and the copy's main file stays as copied | PASS |
| B-05 | Deletion preparation changes nothing | `TestProjectDeletionRequestNeverWritesProjects` | PASS |
| B-06 | A missing file is not created | `TestOpenReadOnlyTypedFailures/missing_file` | PASS |
| B-07 | A wrong key gives a typed error and no change | `…/wrong_key`, and the other-key project in the overview test | PASS |
| B-08 | A newer format is refused safely | `…/future_format_stamp`, plus the overview, deletion and portfolio tests | PASS |
| B-09 | A damaged file triggers no repair or migration | `…/damaged_pages` (`ErrCopyDamaged`, file unchanged); `TestReadProjectOverviewReportsDamage` and `TestProjectsOverviewReportsADamagedProject`: a damaged `project` page opens, the read reports `ErrProjectDamaged`, the list shows `damaged` beside the healthy project, and the file is unchanged | PASS |
| B-10 | Audit chains stay verifiable | Deletion test: the current project's terminal hash and count match `VerifyAuditChain` | PASS |
| B-11 | Older schemas stay readable where required | Overview and deletion read the older fixture | PASS |
| B-12 | Metadata and counts match an independent count | Literal counts in the fixtures, including a stray chart of another project id that must not count | PASS |
| B-13 | WAL and SHM behavior | `TestOpenReadOnlyReadsCommittedWALAndKeepsSidecarsPrivate`: rows only in the WAL are read; the main file is unchanged; sidecars keep the main file's 0600 mode | PASS |
| B-14 | Concurrent readers and writers | `TestOpenReadOnlyAlongsideAWriter` (40 writes and 40 reads, monotonic counts, integrity ok) | PASS |
| B-15 | An interrupted inspection leaves nothing half-migrated | `TestOpenReadOnlyCancelledRead`: a query cancelled mid-iteration leaves the file unchanged, and it reopens cleanly. Killing the process was not tested | PASS |
| B-16 | Symlinks, unsafe paths and permissions | `…/symlink`; `TestReadOnlyDSN` (`?`, `#`, UNC); `TestOpenReadOnlyPercentInPath`; `TestOpenReadOnlyInaccessibleFiles` (mode-000 file; WAL file in a read-only folder; both `ErrProjectInaccessible`, neither changed) | PASS |
| B-17 | Resources close | `TestOpenReadOnlyClosesItsFiles`: 200 opens, including failed ones, with no descriptor growth | PASS |
| B-18 | Pooled connections cannot write | `TestOpenReadOnlyRefusesEveryWrite`: 16 pooled connections with `query_only` turned off, plus a `Migrate()` on an older file, are all refused | PASS |
| B-19 | Backup and restore unchanged | Restore and backup code untouched; the existing `internal/db` and root suites pass | PASS (existing tests) |
| B-20 | Existing workflows keep working | Full `go test . ./internal/...` and `-tags duckdb` pass; headless test | PASS, except the administrator-copy change in §4 |
| — | Windows paths (`file:C:/...`) | No Windows Go-test job in CI | BLOCKED |
| — | `-race` | Slice B tests under `-race` | PASS |
| — | The key stays out of error text and file names | `TestOpenReadOnlyKeepsTheKeyOutOfErrorsAndNames` (`PRAGMA database_list` and typed errors) | PASS |

**Fault seeding.** Each seed was applied to a disposable worktree, run against the tests above, and restored. All 20 were caught:
- each call site switched back to the writable open: overview, deletion, portfolio, administrator copy and headless;
- `mode=ro` removed from the URI;
- the symlink check removed;
- `%` left unescaped;
- the format-stamp check removed;
- the missing-table check removed, and separately the missing-column check removed. Each is caught by its own subtest, after a first run showed the older fixture hid one check behind the other;
- the audit table required;
- counts not scoped to the project;
- the wrong key left untyped;
- the copy's integrity check removed;
- the copy accepting Older schemas;
- the administrator copy switched back to the writable open, against the live-WAL test;
- `SQLITE_CANTOPEN` reported as "not found";
- errors from the overview read left unclassified;
- the overview's read-error branch dropping the reason.

## 6. Performance

`BenchmarkProjectsOverview`:
- **Fixture:** 20 current-schema projects, each with 40 charts of about 100 KB and 10 documents of about 50 KB.
- **Machine:** Apple M4, macOS 27.0.1, Go 1.27.3.
- **Method:** six alternating runs of five iterations against a `main` worktree; benchstat, n = 6.

| | Before (`main`) | After |
| --- | --- | --- |
| Time | 2.34 s ± 2% | 46.9 ms ± 35% |
| Memory | 110 MiB | 2.2 MiB |
| Allocations | 122,230 | 82,690 |

Most of the gain comes from counting rows instead of loading every chart and document. A synthetic fixture on one machine is not a budget (OWN-10).

## 7. Limits and follow-ups

- **"Current" checks tables and columns only.** A `Migrate()` interrupted after adding columns but before its timestamp or money retrofits is not detected; such a file reads with its values as stored.
- **The project list finds damage only on pages it reads.** A damaged `charts` page did not show, because the count was answered without reading it. The list is not an integrity check.
- **Wrong key and a damaged header** both surface as `ErrProjectUnreadable`. An administrator copy torn at page 1 therefore reads as unreadable, not as "copied while changing".
- **The format stamp is not checked on writable opens.** Refusing a stamped file before `Migrate()` on those paths is the format gate, Slice E.
- **Unreadable projects** are still skipped silently by portfolio analytics (§4).
- **The frontend does not yet show** `unreadable_reason`.
- **A DELETE-mode file with a hot rollback journal** from a crashed writer cannot be recovered by a read-only connection. That case was not tested.
- **Windows** is BLOCKED until a Windows Go-test job exists.
