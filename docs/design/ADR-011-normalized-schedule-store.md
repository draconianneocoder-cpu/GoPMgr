<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-011: Normalized schedule store, project format gate and encrypted migration

**Status:** Proposed. **Draft 1**, 2026-10-27.

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
| The bundled engine is SQLite 3.33.0 in SQLCipher 4.4.2. It has no `RETURNING`, `STRICT`, JSON1, FTS5 or `DROP COLUMN`. | S0 finding F5; vendored `third_party/go-sqlcipher` | The schema must use conventional SQLite only. |
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
- **Slice B status (2026-10-27):** the read paths in §5.3 now open read-only and never migrate ([evidence](../testing/read-only-project-access.md)). Writable opens are unchanged; refusing a stamped file before `Migrate()` on them is Slice E.
