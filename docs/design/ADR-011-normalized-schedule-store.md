<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-011: Normalized schedule store, project format gate and encrypted migration

> **Proposed companion designs, 2026-11-13:** [ADR-014 reporting integrity](ADR-014-reporting-integrity-and-lifecycle.md)
> and [ADR-015 secure extensions](ADR-015-secure-extension-platform.md). Follow the
> [dependency sequence](reporting-extension/implementation-plan.md). These proposals
> do not supersede accepted decisions, authorize migrations/commands or qualify
> a worker. Historical owner records and current STATUS retain their authority.

**Status:** Design complete for ratification. Draft 1 and Amendment 1 remain below as written. Amendment 2, 2026-11-18, is the text offered for ratification. The owner has not ratified it. This document authorizes no file format change and no production audit change.

**Authority:** the maintainer's 2026-10-27 decisions ([record](../review/PS-04-owner-decisions-2026-10-27.md), items 3 and 4) authorize drafting this ADR, starting from the proposed domain-ownership table. They authorize **architectural design and review only**. Nothing here activates a schema, migrates a project or changes a production database. Each implementation slice named in §9 needs its own approval.

**Repository state read for this draft:** `main` at `0db7b00`.

**Related:**
- ADR-010 v2.2 reserves this ADR as its S1 (D-12, D-13, OWN-08). ADR-010 is on [PR #140](https://github.com/draconianneocoder-cpu/GoPMgr/pull/140) (head `3d00f53`), not yet merged.
- The professional-scheduling plan and domain model are on [PR #152](https://github.com/draconianneocoder-cpu/GoPMgr/pull/152) (head `3ea7f51`).
- The PS-04 design package, received 2026-10-27 and not in this repository:
  - PS04-DATA-003, storage, compact audit and encrypted migration (SHA-256 `10d54e7d7f4257207fe20baa7090735708ce7fab2e8dd183c1da641027665a4e`);
  - PS04-EXEC-006, sequence and decision ledger (hash in [resource-overallocation-contract.md](../testing/resource-overallocation-contract.md)).
- [ADR-001](ADR-001-database-encryption-at-rest.md) (encryption) and [ADR-009](ADR-009-yen-reporting-currency.md) (money).

## 1. Problem

A professional schedule of 50,000 activities must be editable. Today it is not:

| Fact on `main` | Source | Consequence |
| --- | --- | --- |
| A schedule is one JSON document in `charts.data`. `SaveChart` rewrites the whole document and appends its full before and after JSON to the audit chain in the same transaction. | `internal/db/charts.go` | S0 measured 2.5 s p50 and 35.5 MB of audit growth for one field edit at 50,000 tasks (ADR-010 S0, findings F1 and F2; [PR #142](https://github.com/draconianneocoder-cpu/GoPMgr/pull/142), head `77e1733`). |
| Every open runs `Migrate()` before anything else, including the administrator's read-only copy, which `OpenEncryptedCopyReadOnly` opens through `InitEncryptedDB` first. | `internal/db/sqlite.go` (`initDBWithDSN`), `internal/db/encryption.go` | No build can refuse a newer file before it has already run schema statements on it. |
| No project carries a format stamp. `Migrate()` never sets `PRAGMA user_version`; only the old plaintext-to-encrypted converter copies it. | `internal/db/encryption.go` | A newer format cannot be recognized, and pre-gate binaries cannot be told to stop. |
| When this was accepted the bundled engine was SQLite 3.33.0 in SQLCipher 4.4.2, with no `RETURNING`, `STRICT`, JSON1, FTS5 or `DROP COLUMN`. The engine was upgraded to SQLite 3.54.1 on 2026-11-10; the consequence stands, because a file using newer features cannot be opened by older builds. | S0 finding F5; vendored `third_party/go-sqlcipher` | The schema must use conventional SQLite only. |
| The audit hash covers 13 fields of an event but not its `ID`, and canonicalizes JSON through Go's `any` (numbers become `float64`). | `internal/db/audit.go` (`eventHash`, `canonicalJSON`) | New events must bind by sequence and hash, and carry exact values as strings. |
| WBS v2 has its own dormant store: explicit initialization, copy-bound revisions, receipts and transactional audit. | `internal/db/wbs_store.go` | A proven pattern to reuse; a separate authority to leave alone. |

## 2. Decision summary

The core decisions are proposed for owner review. The open questions in §8 stay open.

1. **SQLCipher in the Go host stays the only authoritative project store.** The Go kernel stays the only schedule calculator.
2. **One write owner per authoritative record** (§3). WBS v2, financial controls and the audit chain keep their independent owners.
3. **One authority per schedule:** either the legacy chart or one active normalized generation, never both (§4).
4. **A non-mutating format gate runs before `Migrate()` on every open path**, with distinct version namespaces. It fails closed on a newer format (§5).
5. **Typed, revision-checked, idempotent commands** write only the affected rows, the revision, an audit event and a receipt, in one transaction (§6).
6. **Migration builds an inactive candidate inside the encrypted file and activates it in one transaction** (§7). Plaintext staging and dual writing are rejected.
7. **Old audit events stay byte-for-byte verifiable.** The compact audit bridge is **not** decided here: it depends on the separate audit-sequencing request (DEC-04), which is not approved (§6.3).

## 3. Domain ownership

The maintainer adopted this table, proposed in the PS-01 domain model (PR #152 at `3ea7f51`, §1), as this ADR's starting contract (decision item 3). Each row has one owner of authoritative writes. Final ownership is settled by this ADR's review.

| Domain | Owns | Authority today | Never owned elsewhere |
| --- | --- | --- | --- |
| Project | identity, currency, country, calendar policy, format version | `internal/db` project row | — |
| Schedule | generations, activities, outline, links, constraints, schedule calendars, progress, actuals | Chart documents (`internal/db/charts.go`), calculated by `internal/kernel` | Dates and float are computed only by the Go kernel |
| Scope (WBS) | WBS v2 nodes and work-package associations | `internal/wbs` (10,000-node limit) | The schedule outline is not the WBS |
| Resources | definitions, groups, resource calendars, capacity | `internal/db/resource_calendars.go`, plus resources inside charts | — |
| Assignments | task–resource units, work, timephased segments | Inside chart documents | Usage views derive from assignments, never the reverse |
| Cost control | budgets, commitments, actual expenditure, ledger, procurement, cost baselines | `internal/budget`, cost ledger | No forecast becomes an actual or a commitment without a financial transaction |
| Baselines | schedule, scope and financial snapshots | Schedule baselines per chart; cost baselines | — |
| Provenance | source artifacts, lineages, field observations, import receipts | Proposed (ADR-010 D-09) | Imported values and calculated values stay distinct |
| Audit | the hash-chained audit log | `internal/db/audit.go` | Workers never write it |
| Analytics | nonauthoritative computed views | DuckDB today; ADR-012 target | Never a system of record |

Rules that follow from the table:

- **A record has exactly one writing domain.** Another domain refers to it by a stable typed ID and never writes it.
- **The Schedule domain writes only through the command path in §6.** Chart projections, workers and analytics never write schedule records.
- **WBS v2 keeps its store, its 10,000-node limit and its write path.** Migrating 50,000 activities does not create WBS nodes. Any outline-to-WBS mapping is a separate decision (O-4).
- **Cost control stays the financial authority.** Schedule-derived costs are forecasts and never become actuals or commitments without a financial transaction.

## 4. Authority states

```text
LEGACY_CHART          the chart document is the schedule; today's path
  │ owner-authorized migration
  ▼
CANDIDATE_BUILDING    inactive normalized rows; legacy still authoritative
  ▼
CANDIDATE_VERIFIED    parity, identity, fidelity, foreign-key and cipher checks pass;
                      still not authoritative
  │ one activation transaction (§7)
  ▼
NORMALIZED_ACTIVE     typed commands only; the chart is a read-only projection
```

- **One editable authority per schedule, recorded in the database.** The legacy chart and the normalized generation are never both writable. Writers route by this recorded state, not by chart kind.
- **One active editable generation per schedule,** with one revision at the generation level. A multi-entity command advances it once. Per-entity version columns may exist for queries but do not replace the generation check.
- **After activation, `SaveChart` and `DeleteChart` refuse a normalized schedule.** Other chart kinds, documents, WBS v2 and cost records keep their paths, subject to the format gate.

## 5. Project format gate

### 5.1 Version namespaces

These stay separate and are never used as proxies for one another:

| Namespace | Where it lives | Purpose |
| --- | --- | --- |
| Cipher parameters | SQLCipher header and pragmas | Unchanged by this ADR |
| Format stamp | `PRAGMA user_version` (proposed, see O-13) | Minimum format a build must understand to write |
| Format detail | A `project_format` table (proposed) | Minimum reader and writer, migration state, journal pointer |
| Domain model version | Per normalized record family | Decoder dispatch |
| Audit payload version | Inside each new event's JSON | Event decoder dispatch |
| `.pmba` archive schema | Backup manifest (`internal/db/backup.go`) | Archive layout only: v1 and v2 today |

A file with `user_version` 0 and no `project_format` table is a **legacy project**, not a corrupt one. That is safe because no GoPMgr code has ever written a nonzero stamp: across the repository's history, the only code touching `user_version` is the plaintext converter, which copies it unchanged (`git log --all -S'user_version' -- '*.go'`).

| Stamp | Meaning | Written by |
| --- | --- | --- |
| 0 | Legacy: no normalized schedule data. Every existing file | Nothing; never written |
| 1 | Contains an active normalized generation, so only a gate-aware build may write it | Only the activation transaction (§7) |
| 2 and later | Later formats | The ADR revision that defines them |

`CreateProject` and `Migrate()` never raise the stamp. A new project stays at 0 until a normalized generation is activated in it.

### 5.2 Opening a project

Every open replaces "open, then `Migrate()`" with this sequence. Steps 2 to 4 are **non-mutating**:
- the main file's bytes, header and journal mode do not change;
- no schema statement runs;
- file permissions are not touched.

For a file already in WAL mode, SQLite may create its `-wal` and `-shm` sidecars; a file in DELETE mode (the plaintext converter leaves files that way) must not be switched to WAL. The probe therefore opens read-only, without the standard `_journal_mode=WAL` connection option and without `ensurePrivateSQLiteFiles`.

1. Resolve the file's identity and refuse unsafe links.
2. Open with the session's DEK on a read-only connection that does not set the journal mode.
3. Prove the key and the file: a key or cipher failure stops here.
4. Read the format stamp and the `project_format` table, if present. **No schema statement runs.**
5. Compare with what this build supports:
   - newer than supported: **refuse**, with a typed error that offers only read-only actions this build has proven safe;
   - supported: continue.
6. Only now open the writable session, and run `Migrate()` only if this build's migration policy allows it for that format.

The gate check is a host-owned capability that every writer receives. It is not a check scattered across call sites. P06 must prove two things:
- **Refusal:** with a deliberately newer stamp, each path in §5.3 refuses before any write.
- **Non-mutation:** after the probe, the main file is byte-identical and `PRAGMA journal_mode` is unchanged. This is checked on a DELETE-mode legacy fixture, a WAL-mode legacy fixture and a newer-stamp fixture.

**Legacy files keep today's behavior.** The gate changes behavior only for a file whose stamp is newer than the build supports. A stamp-0 file is opened as today: writable, with today's additive `Migrate()`. That includes the read paths, because `ProjectsOverview`, `RunPortfolioAnalytics` and `projectDeletionRequest` rely on `Migrate()` having added columns before they read. Making those reads non-mutating for legacy files is a later step, which first makes them tolerate missing additive columns.

### 5.3 Paths the gate must cover

Verified on `main` at `0db7b00`:

| Path | Today | Required |
| --- | --- | --- |
| Headless start, `main.go` (`InitDB`, `InitEncryptedDB`) | Opens and migrates | Gate first |
| `OpenProject`, `CreateProject`, `CreateProjectFromLaunchpad` | Open and migrate | Gate first |
| `restoreProjectArchiveFrom` (`InitEncryptedDB`, or `InitDB` for a plaintext archive) | Opens and migrates the restored file | Gate first; the restored file is checked before it is published |
| **Read paths:** `ProjectsOverview`, `RunPortfolioAnalytics`, `projectDeletionRequest` | **Open, and so migrate, every project they read**: viewing the project list can change files | Gate first: a newer file is refused or read without migrating. Legacy files keep today's open until these readers tolerate missing columns (§5.2) |
| Repair and swap recovery, `RepairAndSwap` in `app_documents.go` | Reopens and migrates | Gate first; repair never rewrites an unsupported format |
| Administrator copy, `app_admin_access.go` through `OpenEncryptedCopyReadOnly` | **Migrates the copy, then reopens it query-only** | Gate first; a read-only inspection never migrates |
| Plaintext-to-encrypted converter, `MigratePlaintextToEncrypted` (reached from `EncryptProjectAtRest`) | Converts legacy plaintext files | Unchanged in purpose; never used for schema migration |
| `.pmba` restore, `RestoreArchivalBundle` | Verifies the archive, publishes, then the caller opens | The archive check stays; the database inside is gated before it is opened for writing |
| WBS v2 `OpenWBSStore` and `Save` | Own guards | Keeps them; adds the format gate |
| Every other writer (documents, cost, baselines, scenarios, agile, sigma, stakeholders, settings, resource calendars) | Write through the open `*Database` | Inherit the gate from the session capability |
The implementation slice must regenerate this list from the call graph (`make code-map`) rather than trust it.

### 5.4 Older binaries

A gate cannot protect a file from a build that predates it. Every GoPMgr release up to the one that ships this gate ignores the stamp, and can open, migrate and write a newer file.

The rollout must therefore:
- name the unsafe versions;
- take and verify an encrypted backup before the first migration;
- warn before migrating.

Option O-13 below considers isolating migrated projects from pre-gate builds altogether. "Rollback" means restoring a verified pre-migration backup through a supported build, not downgrading a migrated file.

The stamp changes only in the activation transaction (§7), or in a separately released gate-only step proven to be monotonic. A crash never leaves new data under an older, permissive stamp.

## 6. Commands, transactions and audit

### 6.1 Command contract

A schedule edit is a typed command. It is not a JSON patch or a whole document:

```text
ApplyScheduleCommand(project capability, schedule_id, generation_id,
                     expected_revision, command_id, actor and session,
                     typed operation, typed arguments)
  -> new_revision, audit (sequence, hash), affected IDs, receipt
   | typed error (REVISION_CONFLICT, COMMAND_ID_REUSE, VALIDATION, FORMAT_REFUSED, ...)
```

### 6.2 Transaction rules

1. **Reserve the single SQLite writer before reading the revision or the audit tip.** Use the same-connection pattern `WBSStore.Save` uses: its first statement in the transaction is a write (`checkCopy(ctx, tx, true)`, `internal/db/wbs_store.go:254–257`), so no read snapshot precedes the lock. Never mix a manual `BEGIN IMMEDIATE` with an open `sql.Tx`.
2. **Reconcile idempotency before checking the revision.** Look up the receipt by `command_id`:
   - a matching fingerprint returns the original receipt;
   - a different fingerprint is `COMMAND_ID_REUSE`.
3. **Compare and set the generation revision** with an `UPDATE … WHERE revision = :expected` that must affect exactly one row. `RETURNING` is not available, so read back in the same transaction.
4. **Commit everything together or nothing:** the affected rows, the revision, one audit event and the receipt. If the audit append fails, the data change rolls back.
5. **A failed `Commit()` is indeterminate.** Discard the connection, reopen through the gate, and reconcile by receipt before answering, as `commitWBS` does. Never blindly retry.
6. **A receipt binds its audit event by `(project_id, sequence_number, event_hash)`,** not by event ID alone, because the hash does not cover the ID.
7. **Money is stored as checked integer minor units** (ADR-009). Exact numbers, revisions and IDs in audit JSON are strings.

### 6.3 Audit bridge: not decided here

New schedule events must append to the existing chain through `appendAuditEventTx`. Old events are never rewritten, resequenced or recanonicalized, and `VerifyAuditChain` must pass on mixed history.

The compact typed payload, its encoding, replay and evidence policy are the subject of the audit-sequencing change request (PS-04 DEC-04). That request is **not approved**. Until it is, P08 cannot enable a normalized write path, and this ADR does not choose a payload format.

## 7. Migration strategy

**Proposed:** strategy A, an inactive candidate built inside the existing encrypted database.

```text
LEGACY_READY
  │ owner authorization; gate passes; encrypted backup taken and verified by reopening it
  ▼
CANDIDATE_BUILDING ── interrupted ──► resume by journal, or quarantine operation-owned rows only
  ▼
CANDIDATE_VERIFIED  every source field classified (exact, converted-with-policy, preserved-opaque,
                    unsupported, unknown, rejected); IDs, references, foreign keys and cipher integrity checked
  │ one transaction: recheck the legacy revision and digest, the audit tip and the gate;
  │ set the authority pointer and the format stamp; append the migration event and receipt
  ▼
NORMALIZED_ACTIVE
```

- **The migration journal lives inside SQLCipher.** Each batch's checkpoint commits with the batch it describes.
- **The legacy source is frozen while the candidate builds.** If it changes, the build restarts with new consent; moving edits are never merged in.
- **An activation commit with an unknown outcome is reconciled by reading the stored state.** It is never retried blindly.

Alternatives:

| Strategy | Disposition |
| --- | --- |
| A. In-file inactive candidate, transactional activation | **Proposed**, subject to WAL-growth and disk-headroom measurement at T2 and T3 |
| B. One large in-file transaction after a backup | Only if measured safe on approved hardware |
| C. New encrypted sibling file, then atomic publication | Backup and recovery alternative only, after per-OS publication tests |
| D. Plaintext scratch conversion | **Rejected**: breaks confidentiality |
| E. Dual writing to chart and normalized store | **Rejected**: two mutable authorities |

`.pmba` v1 and v2 restores keep working. A future archive format is a separate version decision.

## 8. Open decisions

| # | Decision | Proposal in this draft | State |
| --- | --- | --- | --- |
| O-1 | Column types, units and precision for durations, work, lag and money | Money in integer minor units. Durations, lag and work as integers in a declared unit. The unit and the conversion to the kernel's working-day floats are explicit, with no silent eight-hour day | Open: needs a unit decision and kernel conversion tests |
| O-2 | Host ID scheme and source UID scope | The repository's `newID` pattern (`internal/db/ids.go`: a prefix plus hex from `io.ReadFull(rand.Reader, …)`), with a 16-byte body for normalized record families. Today's 4-byte body gives a 50,000-activity generation about a 25% chance of a collision. IDs are immutable and the primary key enforces uniqueness, with a new ID drawn on conflict. Source UIDs are scoped by source artifact and format | Proposed |
| O-3 | Revision granularity | One revision per generation, with optional per-entity versions for diagnostics | Proposed |
| O-4 | Mapping the schedule outline to the WBS | None by default. An explicit, separately approved mapping record | Open |
| O-5 | Raising the WBS 10,000-node limit | Not changed by this ADR | Out of scope |
| O-6 | Calendars, exceptions, time zones and DST | Project time zone identity, source calendars and exceptions preserved | Open |
| O-7 | Timephased representation | Sparse typed segments; never per-minute or per-day rows | Open: format to be chosen with PS-06 |
| O-8 | Number of baselines | All supported source baselines; no "baseline 0 only" | Open |
| O-9 | Custom fields | Definition plus value families, with source type kept beside modeled type | Open |
| O-10 | Legacy conversion and recovery | Strategy A (§7) | Proposed |
| O-11 | Typed audit events and the bridge | Depends on DEC-04 | Blocked |
| O-12 | Where rate tables live | Cost domain owns rates; schedule references them | Open (PS-07) |
| O-13 | How the format stamp is stored, and whether migrated projects are isolated from pre-gate builds | `user_version` as the stamp, readable before any schema statement, plus the `project_format` table. Isolation options: (a) none, rely on warnings and backups; (b) a new file extension that old builds do not list; (c) a separate project directory | Open: owner choice |

## 9. Delivery and tests

Each slice needs its own approval. The test IDs are PS04-DATA-003's (ST, AT, MT, PF).

| Slice | Scope | Must prove before merge |
| --- | --- | --- |
| P03 (this ADR) | Design review; exact DDL, indexes and `EXPLAIN QUERY PLAN` in a later draft | Review sign-off. No code |
| P06 format gate | §5 on every path in §5.3; no new format activated | MT-001 to MT-007: a newer stamp is refused before any write on every path; legacy files open unchanged; the administrator copy no longer migrates |
| P07 storage | Inactive normalized tables and migration mechanics; legacy remains authoritative | MT-008 to MT-020, crash matrix, encrypted backup and restore |
| ACR-AUDIT, then P08 | Audit bridge, then typed commands | AT-001 to AT-016, ST-001 to ST-014; no normalized write before the bridge |

## 10. Consequences

**Benefits:**
- A one-field edit writes one row, a revision, a receipt and an audit event, not 15 MB of JSON.
- Opening a file can no longer change it before the build knows it can.

**Costs:**
- The gate must cover every open path, and new paths must keep it.
- Migrated files are unsafe in pre-gate builds unless O-13 isolates them.
- Strategy A grows the WAL and the file during a migration; that must be measured before approval.

**Kept unchanged:**
- SQLCipher settings and the vendored driver. A driver or engine upgrade is a separate decision and never shares a migration with a schema change.
- WBS v2, cost control, the audit chain's history, `.pmba` v1 and v2, and the legacy chart path for unmigrated schedules.

## Amendment 1 (2026-10-27): maintainer architecture directive

The maintainer's [architecture directive](../review/architecture-directive-2026-10-27.md) settles parts of this draft. The text above is kept as drafted; where they differ, this amendment governs.

- **O-2, decided.** New authoritative records get cryptographically generated 128-bit IDs (UUIDv4 preferred, or an equivalent with verified properties), with database-enforced uniqueness and no fallback to short IDs. Existing IDs and their references are kept. This replaces the 16-byte `newID` proposal in §8.
- **O-13, decided: layered protection.**
  - A distinct next-generation format and extension.
  - An isolated storage location older builds do not traverse (illustratively `projects_v2/.../project.gopmgr2`).
  - The format and minimum-writer gate of §5.
  - Explicit migration only.
  - Verified encrypted recovery copies.
  - Positive evidence that older builds cannot modify migrated projects.

  New builds discover and manage both formats.
- **§5.2, changed.** Read-only operations stop migrating, legacy files included. The draft's "legacy files keep today's open" sentence no longer applies. The read paths in §5.3 move to a non-mutating inspection mode whose readers tolerate missing additive columns.
- **Implementation** of this ADR is authorized in small, gated slices: B (read paths), C (IDs), E (format gate) and F (migration, inactive until qualified). No migration is activated before its acceptance and recovery gates pass, and the ADR stays Proposed until reviewed.

## Amendment 2 (2026-11-18): design for ratification

Draft 1 and Amendment 1 stay above as written. Where either differs from this amendment, this amendment governs. Repository state read for this amendment: `main` at `5ff757dc`.

This amendment changes no executable behavior. It activates no schema, migrates no project, and appends no audit event.

### Authority this amendment uses

Two decisions in [owner decisions, 2026-11-14](../review/owner-decisions-2026-11-14.md) are the authority. Both were chosen by James L. Burns on 2026-11-14. This amendment quotes them and does not add a format decision or an audit decision.

DEC-FORMAT, chosen option "Finish design, then ratify":

> Completing ADR-011's open items (O-1, O-6 to O-10) into a full schema, units, migration and recovery specification for the owner's ratification. No file format change until ratified. The layered protection decided on 2026-10-27 (O-13) stands.

DEC-AUDIT, chosen option "Approve bridge design", recorded as ADR-011 DEC-04:

> A design slice for a versioned, append-only compact audit event that joins the existing hash chain, never rewrites, resequences or recanonicalizes old events, and keeps `VerifyAuditChain` passing on mixed history. Implementation needs a separate approval.

The following stay in force and are not reopened here.

- O-2 from Amendment 1. New authoritative ids are 128 random bits. The repository form is `newID` in `internal/db/ids.go`: a prefix, an underscore, and 32 lowercase hex characters from `io.ReadFull` on `crypto/rand.Reader`. The primary key rejects a duplicate. `ErrGeneratedIDCollision` writes nothing. Existing ids and their references stay. A source UID is a separate column, scoped by source artifact and source format. It is not the primary key.
- O-13 from Amendment 1, which DEC-FORMAT keeps. The illustrative path remains `projects_v2/.../project.gopmgr2`. Final names wait for the older-writer test.
- Amendment 1 on read-only opens. A read-only open does not migrate, including a legacy file.
- ADR-009. Every currency, yen included, is stored as hundredths of the major unit. This amendment uses that storage. It does not introduce an ISO minor-unit exponent.
- DEC-UNITS, 2026-11-11. The kernel limit is 18,250 working days (`MaxScheduleDays` in `internal/kernel/limits.go`). Documents call them working days.
- The Go kernel remains the only schedule calculator. SQLCipher in the Go host remains the only authoritative project store.
- Analytics reads through the Go-native engine. That engine is not a system of record. DuckDB stays retired. DataFusion stays unadopted.
- PDF/A metadata and the output intent are applied before PAdES. This amendment does not change that order.
- WBS v2 keeps its store and its 10,000-node limit. O-4 stays as proposed: this schema has no outline-to-WBS mapping. O-5 stays out of scope.

Amendment 1 authorized gated slices and left the ADR proposed until review. DEC-FORMAT is later. It withholds every file format change until this specification is ratified. This amendment is that specification. It starts no slice.

### Units

O-1 is specified here for ratification. The scale below is part of the text the owner ratifies. No column exists until that ratification, and none exists because of this amendment.

Duration and lag are signed integers. One unit is one millionth of a working day. The column suffix is `working_day_micros`. The kernel converts that integer to its working-day float by dividing by 1000000. Only the Go kernel performs that division. A source value that is not an integer number of these units is not stored in the integer column. The field disposition records `preserved-opaque` or `converted-with-policy`. The authoritative column is not rounded.

The stored magnitude is at most 18,250,000,000. That is the kernel limit expressed in the same unit. A lead is a negative lag. The same bound applies in both directions.

Work is a different quantity. `work_integer` is stored only with a `work_unit` string copied from the source. The one host code this design defines is `working_day_micros`, and only when the source value is already a count of millionths of a working day. Any other unit string is kept and is not converted. A missing unit means the integer is omitted and the original field is `preserved-opaque`. Work is not turned into duration. An eight-hour day is not assumed.

Assignment units use the same integer scale. One whole assignment unit is 1,000,000. The source column stays empty when the source omitted units. The kernel default of one unit is applied at calculation time. It is not written back into the source column.

Money columns are integers of hundredths of the major unit, per ADR-009. An absent amount is null. It is not zero. Audit JSON carries that integer as a decimal string.

Percent complete is the source decimal in a text column. The kernel parses it when it calculates. An unparseable value stays text, with disposition `preserved-opaque`.

Civil dates use `kernel.DateLayout`, which is `YYYY-MM-DD`. A date is stored with no clock.

### Schema

The families are the reference list in [ADR-010 implementation plan §5.2](adr-010-implementation-plan.md). The column lists below are the physical contract for ratification.

Identifiers, names, states, dates, digests, source text, and `percent_complete` are `TEXT`. Counts, flags, revisions, micros, hundredths, ordinals, and batch numbers are `INTEGER`. `active`, `milestone`, `is_working`, and `is_critical` are `INTEGER` values 0 or 1. An absent optional integer or an absent optional reference is null. A micros column, when present, is at most 18,250,000,000 in absolute value.

`WITHOUT ROWID` is unused. Each `generation_id` references `schedule_generations.id`. Each `activity_id` and each activity `parent_id` references `schedule_activities.id`. Each `calendar_id` references `schedule_calendars.id`. Each `resource_id` references `schedule_resources.id`. Each `baseline_id` references `schedule_baselines.id`. Each `series_id` references `timephased_series.id`. Each `definition_id` references `custom_field_definitions.id`. Those foreign keys are `ON DELETE RESTRICT`. A command deletes children before the parent, in the same transaction. Foreign key checks are deferred to commit of that transaction. The moment `PRAGMA foreign_keys` is set stays as it is today.

One project has at most one generation row with `active` equal to 1. The generation revision starts at 1. A command advances it by 1. That is the O-3 rule this design keeps.

`schedule_generations`: `id`, `project_id`, `authority_state`, `revision`, `active`, `calendar_id`, `time_zone`, `status_date`, `calculation_version`, `source_digest`, `frozen_legacy_revision`. The new file uses `CANDIDATE_BUILDING`, `CANDIDATE_VERIFIED`, and `NORMALIZED_ACTIVE`. `LEGACY_CHART` means there is no active new file. `time_zone` is the source zone name, or empty. This design does not fill a default zone. `status_date` is a civil date or empty. `calculation_version` is empty until the host stores kernel results. `source_digest` is the digest of the frozen legacy chart, or empty for a schedule created in the new file.

`schedule_activities`: `id`, `generation_id`, `parent_id`, `ordinal`, `source_uid`, `source_artifact_id`, `title`, `duration_working_day_micros`, `milestone`, `constraint_type`, `constraint_date`, `percent_complete`, `actual_start`, `actual_finish`, `budgeted_cost_hundredths`, `actual_cost_hundredths`, `preserved_source`. `parent_id` is empty at a root. `ordinal` orders siblings. `constraint_type` is a `kernel.ConstraintType` token, or empty for ASAP. Milestone is 0 or 1.

`schedule_activity_results` holds kernel output, one current row per activity. The activity row keeps source facts. A calculation writes this table and does not write the source columns. Columns are `activity_id`, `es_working_day_micros`, `ef_working_day_micros`, `ls_working_day_micros`, `lf_working_day_micros`, `float_working_day_micros`, `is_critical`, `start_date`, `finish_date`, `calculation_version`. The host stores a result integer only when the kernel value is an integer number of micros. Otherwise the previous result row stays, and the disposition records that the result was not stored.

`schedule_links`: `id`, `generation_id`, `predecessor_id`, `successor_id`, `link_type`, `lag_working_day_micros`, `external_predecessor`. `link_type` is `FS`, `SS`, `FF`, or `SF`. A dangling predecessor keeps the source token in `external_predecessor` and leaves `predecessor_id` empty. The row is kept. The kernel skips a dangling reference, as `CalculateCPM` does today.

`schedule_calendars`: `id`, `generation_id`, `name`, `time_zone`, `source_uid`, `source_artifact_id`, `preserved_source`.

`calendar_week_days`: primary key `calendar_id`, `weekday`. `weekday` uses Go `time.Weekday`, Sunday 0 through Saturday 6. `is_working` is 0 or 1. The table has no hour column.

`calendar_exceptions`: `id`, `calendar_id`, `start_date`, `end_date`, `is_working`, `preserved_source`. Both dates are inclusive civil dates. A one-day exception sets them equal. Working time inside a day stays in `preserved_source` and does not become an exception row. Stored civil dates are not shifted for daylight saving. When a calculation needs a zoned reading, the host reads the civil date in the recorded zone. That matches the current `kernel.DayOffset` rule for a date that carries a zone.

`schedule_resources`: `id`, `generation_id`, `source_uid`, `source_artifact_id`, `name`, `max_units_micros`. Name is not unique. Two resources may share a name.

`assignments`: `id`, `generation_id`, `activity_id`, `resource_id`, `units_micros`, `calendar_id`, `work_integer`, `work_unit`.

`schedule_baselines`: `id`, `generation_id`, `source_key`, `name`, unique on `generation_id` and `source_key`. Every source baseline key gets a row. The schema has no single-baseline shortcut.

`baseline_values`: primary key `baseline_id`, `activity_id`. Value columns use the same unit columns as activities, plus `preserved_source`.

`timephased_series`: `id`, `generation_id`, `owner_kind`, `owner_id`, `unit_code`, `scale_denominator`. The denominator is how many stored integers equal one named unit. For `working_day_micros` it is 1000000.

`timephased_segments`: primary key `series_id`, `start_integer`. Also `end_integer` and `quantity_integer`. The interval is half-open. `end_integer` is greater than `start_integer`. One source interval is one row. The schema has no per-minute row and no per-day row.

`custom_field_definitions`: `id`, `generation_id`, `field_key`, `source_type`, `modeled_type`, `version`. `source_type` is required. An empty `modeled_type` means the field is not modeled. `version` starts at 1.

`custom_field_values`: primary key `definition_id`, `owner_kind`, `owner_id`. `source_text` is kept when `modeled_text` is also set.

`schedule_command_receipts`: primary key `command_id`. Also `project_id`, `generation_id`, `fingerprint`, `new_revision`, `audit_sequence`, `audit_event_hash`. The receipt lookup in §6.2 runs before the revision check. The same fingerprint returns the stored receipt. A different fingerprint is `COMMAND_ID_REUSE`. The receipt binds `(project_id, sequence_number, event_hash)` through `audit_sequence` and `audit_event_hash`.

`schedule_migration_journal`: primary key `batch_no`. Also `state`, `legacy_digest`, `rows_committed`. The journal row commits in the same transaction as the rows it describes.

`schedule_field_dispositions`: primary key `generation_id`, `entity_kind`, `entity_id`, `field_name`. `disposition` is `exact`, `converted-with-policy`, `preserved-opaque`, `unsupported`, `unknown`, or `rejected`.

`project_format` has one row: `format_stamp`, `min_reader`, `min_writer`, `migration_state`, `journal_batch`.

Indexes follow the hot paths in ADR-010 §5.2:

- `schedule_generations (project_id, active, revision)`
- `schedule_activities (generation_id, parent_id, ordinal, id)` and `(generation_id, source_uid)`
- `schedule_links (generation_id, predecessor_id)` and `(generation_id, successor_id)`
- `calendar_exceptions (calendar_id, start_date)`
- `assignments (generation_id, resource_id)` and `(generation_id, activity_id)`
- `timephased_segments (series_id, start_integer, end_integer)`

The operation envelope from evaluation item NEW-02 is these columns on the generation, plus the receipt. One command transaction writes the rows, the revision, the audit event, and the receipt. There is no second transaction authority. Source columns and `schedule_activity_results` stay distinct.

Provenance tables beyond `source_digest` and `schedule_field_dispositions` are not specified. ADR-010 OWN-05 still reserves the choice between in-database evidence chunks and an encrypted sidecar. This amendment does not make that choice.

O-12 stays the proposal already in §8. The cost domain owns rates. A schedule row refers to a rate by id. This schema does not define the rate table, its effective dates, or a rate column. A schedule cost stored here is a forecast. It becomes an actual or a commitment only through a financial transaction in cost control.

### Migration and recovery

O-10 in §8 proposes strategy A, an inactive candidate inside the existing encrypted file. Amendment 1 then decided O-13, a distinct file and an isolated directory. DEC-FORMAT says that layered protection stands. The two texts conflict. This amendment follows the later decision.

The candidate is built only in the new SQLCipher file. The illustrative location is `projects_v2/.../project.gopmgr2`. The legacy `.gopmgr` file is not given normalized tables, and its `user_version` is not raised. Inside `internal/db`, the plaintext converter copies `user_version` and does not choose a new value (`internal/db/encryption.go`). Account `system.db` keeps a separate reader level in `internal/users/schema_gate.go`. That level is not this project format stamp.

Before the build starts, the host takes an encrypted backup of the legacy file and proves it by opening that backup. The legacy chart is frozen. The frozen digest is stored in the journal. If the legacy chart changes, the build stops. A replacement build needs new consent. Moving edits are not merged.

Each committed batch is `CANDIDATE_BUILDING`. After the parity, identity, reference, foreign-key, and cipher checks, the state is `CANDIDATE_VERIFIED`. The legacy file is still the authority.

Activation is one transaction in the new file. It rechecks the frozen digest, the audit tip, and the format gate. It sets `NORMALIZED_ACTIVE`, sets `user_version` to 1, writes `project_format`, and appends the migration event and the receipt. A stamp of 2 or higher is undefined. A build that reads a stamp above 1 refuses the file before any schema statement. `CreateProject` and `Migrate()` do not raise the stamp.

An activation whose commit outcome is unknown is answered by opening the new file and reading the stored state. It is not run again on a guess.

Rollback means a supported build reads the legacy file again. The new file is not rewritten into a legacy file. The legacy file was not converted in place.

| Recorded state | Legacy file | Next authorized action |
| --- | --- | --- |
| No new file | Authority | Do nothing |
| `CANDIDATE_BUILDING`, batch committed | Authority | Resume after that batch, or quarantine the new file only |
| `CANDIDATE_BUILDING`, batch not committed | Authority | The open batch is absent. Resume at the last committed batch |
| Frozen digest mismatch | Authority | Stop. Ask for new consent. Do not merge edits |
| `CANDIDATE_VERIFIED` | Authority | Wait for the activation transaction |
| Activation outcome unknown | Authority | Read the new file's pointer and stamp. Answer from that read |
| `NORMALIZED_ACTIVE` | Kept as the recovery copy | Typed commands against the new file |
| Activation rolled back | Authority | Leave the candidate inactive |

Quarantine applies to the operation-owned new file. It does not apply to an existing legacy project directory. That matches the legacy-folder rule in ADR-010 §6.3.

Older builds do not traverse the isolated directory and do not open the new extension. The required proof is a test that those builds cannot modify the migrated project. The illustrative names stay illustrative until that test passes. New builds discover both formats.

Strategy B in §7 remains available only if a later measurement shows an in-file transaction is safe, and only if the owner revises O-13. Strategy D and strategy E stay rejected, as §7 already says.

### SQLite features in the new file

Evaluation item NEW-09 is decided here only as a boundary. The vendored engine is SQLite 3.54.1. That upgrade does not add features to this design.

The new file uses ordinary tables, `INTEGER`, `TEXT`, `BLOB`, primary keys, foreign keys, indexes, check constraints, and transactions. This design does not use `STRICT`, `RETURNING`, JSON1, FTS5, or `DROP COLUMN`. Admitting any of those is a later format decision.

The legacy file continues to use the SQL current GoPMgr already writes.

### Compact audit bridge

DEC-AUDIT approves this design. It does not approve implementation.

New schedule events and the migration event append through the existing `appendAuditEventTx`. They are rows in `audit_events`. `eventHash` in `internal/db/audit.go` is unchanged. This design adds no column to that table and no field to that function. The compact body sits in `after_canonical_json`. When the command has a before image, that image sits in `before_canonical_json`. An empty before image stays the existing canonical null.

`canonicalJSON` decodes JSON numbers through Go `any`, so large numbers become `float64`. Every exact value in the payload is therefore a JSON string. The payload version is the string `1`.

```json
{
	"audit_payload_version": "1",
	"command_id": "",
	"generation_id": "",
	"expected_revision": "",
	"new_revision": "",
	"operation": "",
	"changes": [
		{
			"entity_type": "",
			"entity_id": "",
			"field": "",
			"before": "",
			"after": ""
		}
	]
}
```

`event_type` is `schedule.command`. `entity_type` is `schedule_generation`. `entity_id` is the generation id. Activation uses the same shape with operation `migrate.activate`. The event does not contain the chart document.

An old event stays the bytes that were written. Its sequence number stays. Its canonical JSON stays. A later compact event links by `previous_event_hash` to that event's `event_hash`. `VerifyAuditChain` recomputes every stored event with the existing hash function and checks sequence order. A mixed history passes when those stored bytes still match. A changed old row fails the same check.

Replay reads the stored change strings in sequence order, or reads the rows the command committed. Replay is not a second chain. Existing audit checkpoints stay as they are. This design does not define a new checkpoint. Audit replay is not undo.

The §6.1 command and the §6.2 transaction rules stay. The one audit event in those rules is this compact event. No normalized write is enabled by this amendment. Enabling it requires the separate implementation approval DEC-AUDIT names, after the format specification is ratified.

### What the named slices may rely on

Reliance means the contract below is the design input. It does not mean the slice may be coded.

| Slice | Contract it may rely on | Still required before code |
| --- | --- | --- |
| ARC-01 | The tables, units, and single active generation in this amendment. One write owner per domain, as §3. | Ratification of DEC-FORMAT, then a slice approval. |
| ARC-02 | O-13 layered protection, the gate in §5 as Amendment 1 changed it, stamp 0 on the legacy file, stamp 1 written only by activation in the new file. | The same ratification, plus older-writer evidence on the final names. |
| ARC-03 | The compact event, the unchanged `eventHash`, and mixed-chain verification. | A separate implementation approval. |
| ARC-04 | §6.1, §6.2, and the compact event as the audit write in that transaction. | A ratified ARC-02 and an approved ARC-03 implementation. |
| ARC-05 | The contracts of ARC-01 through ARC-04. | Those slices, plus DEC-HARDWARE. This amendment sets no numeric budget. |
| PS-01 | The domain table, the table families, and one active generation. | The ARC-01 gate. |
| PS-04 | The ARC-01 through ARC-05 contracts for storage, audit, and commands. | Those gates. Gantt rendering is outside this design. RES-A-1 is unchanged. |
| PS-05 | One active generation, one generation revision, and a chart that is a read-only projection after activation. Bounded reads keep the revision they were read at. | A later workspace slice. ADR-010 §7.1 method names stay illustrative. |
| PS-06 | Sparse `timephased_series` and `timephased_segments`. Usage is derived from assignments. The kernel calculates it. | A later view slice. |
| PS-07 | Cost control owns rates. Schedule money is hundredths of the major unit. A forecast becomes an actual only through a financial transaction. | A later decision that names the rate table. |

### Still blocked

- The owner has not ratified this amendment. Until that happens, no file format changes.
- DEC-AUDIT still requires a separate approval before any compact event is written by production code.
- DEC-HARDWARE still sets no numeric budget.
- O-4, O-5, and the O-12 rate table remain as this amendment states them.
- G2 and G3 remain unauthorized.
- Final directory and extension names remain illustrative until the older-writer test.
