<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010: Interchange workers and the data platform for large schedules

**Status:** Proposed (v2.1). Nothing here is implemented.
**Date:** 2026-10-24
**Deciders:** GoPMgr maintainer
**Verified against:** `main` at `28a758d` (2026-10-24)
**Sources:** the v2.0 plan is the primary source. Its critical review addendum
is applied here, and the original plan is kept for reference. All three are
stored unchanged in [`adr-010/`](adr-010/README.md).
**Companion documents:** [implementation plan](adr-010-implementation-plan.md)
and [test strategy](../testing/adr-010-test-strategy.md).

This ADR records the decisions. The implementation plan holds the phases,
contracts, and interface design. The test strategy holds the acceptance
matrix. The addendum asked for that split (its section 6), because a single
24,000-word ADR is hard to keep current.

## Context

Users want to:

1. Open Microsoft Project (`.mpp`), Primavera P6 (`.xer`, `.pmxml`), and
   ProjectLibre (`.pod`) schedules without the original application.
2. Keep what those files hold: summary tasks, calendars, resources,
   assignments, baselines, timephased data, and custom fields.
3. Create a project from an import, compare two projects, and later merge
   selected changes.
4. Edit schedules well beyond 10,000 tasks.
5. Later, get optional local AI help that never edits a project on its own.

GoPMgr's current code cannot do this. Each limit below was verified at
`28a758d`:

| Area | Today | Why it limits the goal |
| --- | --- | --- |
| MS Project import | `internal/export/mspdi_import.go` reads a subset of MSPDI XML into one CPM chart. It skips null rows (line 165) and summary rows (169), converts durations at 8 hours a day (`mspdiHoursPerDay`, line 89), and flattens assignments to resource names. Input is capped at 64 MiB (`app_charts.go:920`). | It is a chart importer, not a project importer. `.mpp`, `.mpx`, and `.pod` get an actionable error (`import_formats_test.go`). |
| Schedule storage | A schedule is one JSON document in `charts.data`. Every edit stores whole before and after copies in the audit chain (`internal/db/charts.go:114-130`). | A one-field edit rewrites the whole schedule. |
| WBS | WBS v2 caps a scope tree at 8 MiB, 10,000 nodes, 64 levels, and 50,000 associations (`internal/wbs/model.go:18-23`), and keeps 256 receipts (`internal/db/wbs_store.go:26`). | A large schedule cannot become a WBS. The two are different things and must stay separate. |
| Analytics | `ImportTabular` returns at most 10,000 rows (`internal/analytics/duckdb.go:187`, `LIMIT` at 245). | This caps a result, not storage. Removing it would send huge payloads to the web view. |
| Audit | Event hashes cover the previous hash and canonical JSON (`internal/db/audit.go:291-337`). | Changing old event bytes or the hash rule would break every existing proof. |
| SQLite | The bundled SQLCipher uses SQLite **3.33.0** (`third_party/go-sqlcipher/sqlite3.h:126`). FTS5 and JSON1 compile only with the `sqlite_fts5` and `sqlite_json` build tags, and release builds use neither (`Makefile:19`). | No `STRICT` tables, `RETURNING`, `DROP COLUMN`, JSON functions, or full-text search unless the build changes. |
| Project files | The schema grows through `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... ADD COLUMN`. `user_version` is copied during encryption (`internal/db/encryption.go:299`) but never set or checked. | **No format gate exists.** An older build opens a newer file without knowing it must not write. |
| Project discovery | `ListProjects` scans `<DataDir>/projects` (`enumerateProjects`, `app_projects.go:807`). `system.db` holds no project catalog. | A complete project folder appearing there *is* registration. |
| Interface | No list or timeline virtualization exists in `frontend/src`. All interface text goes through the ADR-008 catalog. | Large-schedule views are new work and must be translatable. |

Some useful pieces already exist and should be reused rather than rebuilt:

- **Before a rename:** `prepareForRename` (checkpoint the WAL, then switch to
  the DELETE journal; `encryption.go:331`) and `requireEncryptedIntegrity`
  (`encryption.go:312`).
- **File hygiene:** `removeSQLiteSidecars` (`encryption.go:356`) and
  `ensurePrivateSQLiteFiles` (`sqlite.go:960`).
- **Session checks:** session epochs in `session_state.go`.
- **Foreign keys:** enforced on every pooled connection through the DSN
  (`sqlite.go:64-75`).
- **Import receipts:** the MSPDI import options and receipt
  (`mspdi_import.go:38-53`).

## Decision

### D-1 Out-of-process workers

Format conversion runs in a child process that GoPMgr starts. Go `plugin`
shared objects and JNI are rejected. The first and only worker in scope is a
Java bridge around MPXJ.

### D-2 Bundled, trusted workers only

The worker ships inside the signed app. Before every launch, its files are
checked against a release lock file of digests.

The following each need a separate ADR (the "G7" phase): third-party plugins,
downloads, independent updates, and a plugin marketplace.

A separate process is not a sandbox. The bundled worker is trusted code and
must not be described as sandboxed.

### D-3 A narrow worker MVP

The first version does only these things:

- launch, handshake, health check, cancel, and stop;
- resource limits and metrics;
- manifest and digest checks;
- capability negotiation;
- bounded streaming.

There is no general plugin registry, user-interface insertion point, SDK, or
update channel until a second worker exists and needs one (addendum P0-08).

### D-4 Contract: versioned protobuf messages; transport decided in G0

The contract is versioned Protocol Buffers messages with typed `oneof`
envelopes. Opaque `bytes` or `Any` payloads may not carry authoritative data.

G0 compares two transports with the same messages and keeps one:

- **(a) Recommended hypothesis:** length-delimited frames over the child's
  inherited stdin and stdout pipes, with stderr kept as a bounded log channel.
- **(b)** gRPC through `hashicorp/go-plugin` (MPL-2.0).

Why (a) is preferred:

- Pipes belong only to the parent and child on every OS. There is no port,
  socket file, loopback token, or Windows pipe permission to get wrong; v2.0's
  risk R-08 disappears.
- It adds no gRPC, yamux, or hclog dependencies.
- With one job per process (D-5), nothing needs multiplexing.

Choose (b) only if G0 shows a real need that (a) cannot meet, such as
concurrent calls within one job, *and* its Java handshake passes on Windows,
macOS, and Linux.

### D-5 One job per worker process

Each inspection, import, or export starts a fresh JVM, and the process exits
when the job ends. Cancelling kills the process. Nothing carries over between
users or projects.

If the start-up time measured in G0 exceeds its budget, the fallback is a
session-scoped worker that restarts on sign-out and on every project switch.

### D-6 The host owns everything authoritative

GoPMgr's Go host owns:

- project identity, sessions, and epochs;
- encryption keys and SQLCipher handles;
- the audit chain, the scheduling kernel, WBS, money, and revisions.

The worker receives only the source the user picked. That is a byte stream
by default, or a read-only handle where G2 proves it safe (D-10). It returns
typed records. It never receives database handles, keys, paths outside its
scratch folder, or a way to call back into `App`.

Every commit re-checks the principal, session epoch, project copy, expected
revision, and approval digest.

### D-7 Three identities, never matched by name

These follow addendum P0-01:

- **Source artifact:** the exact bytes, their SHA-256, the reader and its
  version, and the import time.
- **Source lineage:** which real project a file represents. It is established
  from the native project GUID or UID, producer details, and the user's
  confirmation. It is never inferred from a name or path.
- **Entity key:** the source UID, scoped to a lineage and a record type. Task
  IDs and ordinal positions are stored separately.

Links between source entities and GoPMgr entities are immutable records. Each
has a validity interval, a confidence category, and any human override.

Two files that reuse the same UIDs (a copied project) are never merged
automatically.

### D-8 New-project import publishes a sealed database with one rename

This follows addendum P0-02 and option 1 of P0-05. Steps:

1. **Build** the project in `<DataDir>/staging/<operation-id>/`. That folder
   is outside `projects/` but on the same volume, so the final rename is
   atomic. The work uses many bounded transactions. After the user approves,
   it writes the import receipt and audit event into the staged project, so
   the project is complete before anyone can see it.
2. **Validate** it:
   - `integrity_check`, `cipher_integrity_check`, and `foreign_key_check`;
   - record counts against the receipt;
   - the audit chain.
3. **Seal** it with `prepareForRename`, then close and fsync.
4. **Publish** it by renaming the folder into `projects/`. The rename must
   never replace an existing folder, so it uses each OS's no-replace call
   rather than `os.Rename` (plan §6.4).

Because discovery is a folder scan, that rename both publishes and registers
the project.

An intent journal (`PREPARED → SEALED → PUBLISHED → DONE`) lets reconciliation
after the next sign-in finish or quarantine any interrupted import. The result
is always exactly one complete project, or a quarantined staging folder that
the user is told about. It is never a partial project in the list.

Importing *into* an existing project needs an immutable shadow-schedule
generation and an atomic pointer switch. That design is separate and deferred.
Until it exists, only new-project import is offered. A selection that cannot
be written in one bounded transaction is refused with a clear limit error.
Visible segmented commits are never called atomic.

### D-9 Fidelity is measured per field

This merges addendum P0-06 with the v2.0 vocabulary. There are three import
modes:

- **Inspect only:** nothing is written.
- **Strict:** the import fails before commit if any field the user marked
  essential cannot be kept exactly.
- **Reviewed best effort:** the user reviews every field that is not exact.

Each source field gets exactly one disposition:

| Disposition | Meaning |
| --- | --- |
| `exact` | Stored with the same meaning and precision |
| `converted_exact` | Converted without loss, for example units |
| `converted_approximate` | Converted with a stated loss |
| `derived` | Recomputed by GoPMgr, not taken from the file |
| `preserved_read_only` | Kept as evidence, not editable |
| `unsupported_by_reader` | MPXJ cannot read it |
| `not_modeled` | Read, but GoPMgr has nowhere to put it |
| `excluded_by_user` | Left out by the user's choice |
| `source_absent` | Not present in this file |
| `failed` | Could not be read or validated |

Coverage claims are made per format *and* version family, with fixtures as
evidence. The receipt extends the existing `MSPDIImportReceipt` vocabulary
(imported, excluded, and transformed fields) rather than starting a new one.

### D-10 Worker resources are bounded and measured

This follows addendum P0-03. Streaming limits how much crosses the pipe, not
how much memory the parser uses. Each of these is set separately:

- JVM `-Xmx`;
- an OS memory limit where available (cgroups on Linux, Job Objects on
  Windows);
- a CPU deadline;
- a cap on timephased expansion;
- a cap on scratch bytes.

Before parsing, the host checks free disk space against an estimate for the
format. Too large a file is refused with `RESOURCE_LIMIT`, and inspect-only
stays available. The real budgets come from G0 and G2 measurements for each
format.

### D-11 Normalized schedule storage needs its own ADR

Large editable schedules move to indexed, normalized tables. The design is
decided in a separate storage ADR (proposed ADR-011), not here (addendum
P0-07). That ADR must:

- **Gate the format before writing it.** Ship a project-format gate that
  checks a minimum-writer version **at least one release before** any build
  writes normalized schedules. Builds older than the gate cannot be stopped,
  so the release notes must say which versions may open new files.
- **Fit SQLite 3.33.0,** or decide to upgrade SQLCipher under the
  `docs/DEPENDENCIES.md` rules.
- **Check foreign keys after migrating.** Migrations turn foreign keys off
  (`sqlite.go:665-676`), so each one must run `PRAGMA foreign_key_check`
  afterwards.
- **Keep one authority per schedule.** Once a schedule is normalized, its old
  chart becomes a read-only projection, and every chart mutator is blocked for
  that schedule.
- **Ship the evidence:** migration DDL, fixture queries, query plans, and
  rehearsed backup, restore, and administrator-escrow paths.

### D-12 Current limits stay until a bounded replacement exists

These stay as they are:

- `maxImportRows` stays as a cap on one result.
- The WBS v2 caps stay. A schedule's outline is not its WBS.
- `charts.data` stays for small charts.
- The MSPDI chart importer stays, renamed in the interface to
  **Quick chart import (MS Project XML)**.

### D-13 The audit chain stays verifiable

`eventHash` and the canonical JSON rule do not change. Old events are never
rewritten.

New schedule commands add compact, typed change events inside the existing
envelope. Exact decimals and large integers are written as strings
(addendum P1-08). The storage ADR holds the details: checkpoints, evidence
retention, and replay.

### D-14 The interface follows existing rules

- **Translation:** all new text goes through the ADR-008 catalog. Typed error
  codes map to catalog keys, so no screen matches English backend text.
- **Accessibility:** keyboard use and screen readers are acceptance criteria,
  not polish.
- **IME input:** Enter handlers ignore `isComposing`.
- **Rendering:** no view renders every row; lists are paged and virtualized.

### D-15 AI and MCP get a separate ADR

Any AI or MCP access needs:

- a host-issued client grant scoped to one account, project, and copy;
- read-only access, an expiry, and revocation on sign-out or project switch;
- an MCP server embedded in the running, signed-in app, not a separate
  broker.

Nothing in this ADR enables AI access (addendum P0-04).

### D-16 Licensing is compatible; obligations still apply

GoPMgr is GPL-3.0-or-later. The relevant licenses all combine with it:

- MPXJ is LGPL-2.1.
- `go-plugin`, if chosen, is MPL-2.0.
- MPXJ's ten runtime dependencies are all permissively licensed: Apache
  POI, Jackson, jackcess, rtfparserkit, and sqlite-jdbc (Apache-2.0); jsoup
  (MIT); JAXB (EDL-1.0, which is BSD-3-Clause); and jgoodies-binding (BSD).
  Each is compatible with GPL-3.0.
- `sqlite-jdbc` carries native libraries. These must be signed for macOS
  notarization, or left out if P6 SQLite databases are not supported.
- A bundled OpenJDK runtime is a separate program under GPL-2.0 with the
  Classpath Exception.

Obligations that remain:

- ship the notices;
- offer the corresponding source for MPXJ and keep its JARs replaceable;
- produce an SBOM;
- add REUSE annotations for bundled binaries;
- get a legal review before the first release that includes the worker.

MPXJ is never a Go dependency.

### D-17 Claims are limited to what was tested

- Format support is advertised per verified reader or writer.
- Native `.mpp` writing is never offered, because MPXJ cannot do it.
- "Full fidelity" is claimed only per format and version, with evidence.
- 50,000 tasks remains a design target, not a promise, until measured.

## Options considered

### Extension mechanism

| Option | Assessment | Outcome |
| --- | --- | --- |
| Go `plugin` shared objects | No Windows support; tight toolchain coupling; a crash takes down the host | Rejected |
| JVM in-process through JNI | CGO risk; a JVM crash kills GoPMgr | Rejected |
| Reimplement MPXJ in Go | Years of format work | Rejected for now |
| MPXJ command line, run once per import | Simple, but opaque output and no progress or cancel | Spike reference only |
| **Child process with protobuf contract** | Typed, language-neutral, isolated failures | **Chosen** |
| MCP as the plugin transport | No transaction or fidelity semantics | Rejected; MCP is only an AI adapter |

### Transport for the contract (new in v2.1)

| Dimension | (a) protobuf frames over stdio pipes | (b) gRPC through `go-plugin` | (c) custom gRPC on loopback |
| --- | --- | --- | --- |
| Channel privacy | Inherited pipes, private to parent and child | Unix socket on macOS and Linux, loopback TCP on Windows; optional auto-mTLS | Needs a per-launch token on loopback |
| Dependencies | `protobuf` only | grpc, yamux, hclog, go-plugin | grpc |
| Cancel | Close stdin, then kill | RPC cancel, then kill | RPC cancel, then kill |
| Concurrency within a job | One stream at a time | Multiplexed | Multiplexed |
| Java side | Plain `java.io` with protobuf-java | Must implement go-plugin's handshake and health | gRPC server |
| Main risk | Hand-written framing needs fuzzing | Java handshake on Windows unproven | Endpoint authentication |
| **Use** | **Recommended for the MVP** | If G0 proves a need | Not recommended |

### Publishing an imported project

| Option | Crash result | Outcome |
| --- | --- | --- |
| Build in the final `projects/` folder (as `CreateProject` and `RestoreProjectArchive` do today) | A partial project can appear in the list | Rejected for imports |
| **Build, seal, and validate in `staging/`, then one rename** | Exactly one complete project, or a quarantined staging folder | **Chosen** |
| Segmented commits into the final project | Partial data becomes visible | Rejected (addendum P0-05) |

## Trade-off analysis

- **Stdio instead of gRPC.** This trades multiplexing for a much smaller
  attack surface and fewer dependencies. One job per process removes the need
  to multiplex. If a later worker needs concurrent streams, the protobuf
  messages carry over to gRPC unchanged.
- **A fresh JVM per job.** This costs start-up time on every import, around a
  second and still to be measured. In return, each project is isolated,
  cancel is simple, and a crashed or leaking worker cannot affect the next
  job. Imports are rare and long, so the cost is small compared with parsing.
- **Staging plus rename.** This adds a journal and a reconciliation step. In
  exchange there is never a partial project in the list, and it reuses code
  the encryption migration already proves.
- **A separate storage ADR.** This delays large-schedule editing. It also
  stops a schema change from slipping in inside an importer commit, which
  `AGENTS.md` and v2.0 both forbid.

## Consequences

**Easier:**

- Native MPP, P6, and POD files can be read without their applications.
- Failures in the worker cannot corrupt projects or crash GoPMgr.
- Import results are honest field by field.

**Harder:**

- Releases grow by a Java runtime and the MPXJ JARs; G0 measures how much.
- Packaging and signing must cover a second executable on three OSes.
- The storage, audit, and interface work for large schedules is substantial
  and is gated separately.

**To revisit:**

- the transport, if a second worker needs concurrency;
- a session-scoped worker, if JVM start-up exceeds its budget;
- an SQLCipher upgrade, if the storage ADR needs SQLite features newer than
  3.33.0;
- reusing the staging-and-rename protocol for `CreateProject` and
  `RestoreProjectArchive`, which can leave a partial project after a crash
  today.

## Changes from v2.0

### Corrections found by verification

| v2.0 or addendum statement | Finding at `28a758d` or from upstream docs | Change |
| --- | --- | --- |
| ProjectLibre `.pod` support is unverified | MPXJ documents `ProjectLibreReader` for `.pod` files from ProjectLibre 1.5.5 on (mpxj.org, *How to read ProjectLibre files*) | Treat as a documented candidate; still gated on fixtures |
| MPXJ 16.10.0 (2026-10-05), LGPL | Confirmed: the changelog lists 16.10.0 on 2026-10-05, and the license is LGPL-2.1 | Pin 16.10.0 in G0 |
| Which formats MPXJ writes | MPX, MSPDI, Planner, XER, PMXML, and SDEF; MPP is read-only (from Project 98 on) | The export list in the plan uses this |
| MPXJ's JSON omits timephased data by default (addendum) | Confirmed since 16.0.0 (`setWriteTimephasedData`, off by default) | Matters only for spikes that use MPXJ's JSON; the bridge emits typed protobuf |
| Java version unknown | MPXJ's docs say Java 8 or later | Pin per release in G0 and bundle a jlink-trimmed runtime |
| Licensing may block | The host is GPL-3.0-or-later and all candidates are compatible (D-16) | Lower risk; legal review still required |
| Loopback gRPC with a bearer token | Stdio pipes avoid the problem (D-4) | New recommended option |
| Segmented import "with independent atomic checkpoints" | Contradicts all-or-nothing import (addendum P0-05) | Removed |
| Publication ends in a `REGISTERED` catalog step (addendum) | No project catalog exists; discovery scans `projects/` | The rename registers; a catalog would add the step back |
| "Atomic rename without replacing an existing destination" (v2.0 §7.4) and atomic publication (addendum P0-02) | Go's `os.Rename` always replaces a file. For a folder on Linux and macOS it refuses an existing target only through a separate `Lstat` check, and on Windows it passes `MOVEFILE_REPLACE_EXISTING` | Publication and export use the no-replace calls already available in `golang.org/x/sys` (plan §6.4) |
| Verify foreign keys per connection (addendum P0-07) | Already enforced in the DSN; migrations disable them temporarily | The storage ADR must run `foreign_key_check` after each migration |
| FTS5 is an option for search (addendum P1-01) | Needs the `sqlite_fts5` build tag, which release builds do not set | A build decision with binary-size and test impact |
| "Do not assume new SQLite features" (addendum P0-07) | The core is 3.33.0 | Listed in D-11 and the plan's storage constraints |
| Old builds should refuse newer files (v2.0 §15) | There is no format gate to refuse with | D-11: ship the gate a release earlier |
| Not mentioned in v2.0 | ADR-008 requires catalog text, full sentences, and plurals | D-14 and the plan's copy section |
| Not mentioned in v2.0 | `MSPDIImportReceipt` already records imported and excluded fields | D-9 extends it |
| v2.0 inspected `a38b065` | `main` is now `28a758d`. #136-#139 changed export code (EVM currency, MSPDI lag rounding) and the account screens. None conflicts. | Evidence baseline updated |

### How the addendum is applied

| Item | Where |
| --- | --- |
| P0-01 lineage and identity | D-7; plan §4.1 |
| P0-02 crash-consistent publication | D-8, adapted to folder discovery; plan §6 |
| P0-03 JVM memory and file access | D-10; plan §5.6 |
| P0-04 MCP client grants | D-15; plan §8 |
| P0-05 atomicity contradiction | D-8 (option 1 chosen, existing-project import deferred) |
| P0-06 fidelity modes | D-9; plan §4.2 |
| P0-07 schema authority and versions | D-11; plan §7.1 |
| P0-08 narrow worker MVP | D-3; plan §10 (G1 becomes the worker MVP) |
| P1-01 to P1-09 | Plan §7 (storage, paging, caches, WAL, timephased, outline, analytics, audit, performance) |
| UX-01 to UX-09 | Plan §9 (screens, design system, copy) |
| A0 to A9 amendment order | Plan §10.3 |
| S7 split | S7A (analytics) and S7B (compact audit) in plan §10.2 |
| REVIEW-001 to REVIEW-020 | Test strategy §4, together with PLG and SCALE |
| Governance (§6) | This split: ADR, plan, test strategy, and future storage and AI ADRs |

### Defects in the source documents

These are kept as written in `adr-010/`, and fixed in the new documents:

- **Numbering:** v2.0 §0 numbers its constraints 1-12 and then 8 again, and
  §18.1 repeats steps 5 and 6. The addendum's patch A0 also notes this.
- **Placeholder protocol:** v2.0's protocol sketch carries authoritative data
  in placeholder `bytes` fields, which v2.0 itself forbids. The plan replaces
  them with typed messages.
- **Sample DDL:** v2.0's illustrative DDL stores a per-row `revision TEXT` and
  ISO date strings. These are fine as illustrations, but the storage ADR must
  define revision ownership and typed instants.
- **Missing reference:** v2.0 never mentions ADR-008. Every planned screen
  needs catalog keys.

## Owner decisions

These are needed before code. Each lists the recommended answer.

- [ ] Accept D-1 to D-17 as the direction (this document stays Proposed until
      then).
- [ ] Approve G0: worker feasibility (transport A versus B), the MPXJ 16.10.0
      licence and SBOM draft, JVM start-up and memory measurements, and the
      fixture corpus plan.
- [ ] Approve S0: a census of current limits and baselines. Evidence only, no
      behavior change.
- [ ] Confirm the first user workflow: **inspect, then import as a new
      project**. No merge.
- [ ] Commission the storage ADR (ADR-011) before any schema change,
      including the format gate that ships one release earlier.
- [ ] Decide the hardware profiles (minimum and recommended) for the
      performance budgets.
- [ ] Confirm that AI and MCP work waits for its own ADR.

## Action items

1. [x] Consolidate v2.0, the addendum, and the original into this ADR, the
   implementation plan, and the test strategy, with the sources kept
   unchanged.
2. [ ] G0 and S0 evidence pull request: no product behavior change (plan §10).
3. [ ] Storage ADR-011 draft, including the format gate.
4. [ ] `docs/interoperability/mpxj-field-mapping.md`, generated from the
   pinned MPXJ API and fixtures, before any production import.

## Status history

| Date | Status | Note |
| --- | --- | --- |
| 2026-10-23 | Draft v1 and v2.0 | Delivered as an external planning bundle |
| 2026-10-23 | Addendum | Critical review of v2.0 (8 P0 findings) |
| 2026-10-24 | Proposed v2.1 | Consolidated in-repo; verified against `28a758d`; addendum applied |
