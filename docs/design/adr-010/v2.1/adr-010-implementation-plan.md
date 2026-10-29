<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 implementation plan (v2.1)

**Status:** Proposed. This plan describes design, not implemented behavior.
**Decision record:** [ADR-010](ADR-010-plugin-architecture.md)
**Test strategy:** [`docs/testing/adr-010-test-strategy.md`](../testing/adr-010-test-strategy.md)
**Sources:** v2.0 (primary) and its critical review addendum (applied),
stored unchanged in [`adr-010/`](adr-010/README.md). Section numbers in
brackets, such as [v2 §9.2] or [P0-02], point to those originals.

**Rule for every phase:** a phase that passes gives permission to *propose*
the next one, nothing more. No phase may weaken an existing invariant.

If the plan conflicts with an accepted ADR or with code, stop. Write down the
conflict, then use this order to decide:

1. maintainer decisions;
2. accepted ADRs and the code at `HEAD`;
3. upstream documentation;
4. this plan;
5. its illustrations.

## Contents

1. [Architecture and ownership](#1-architecture-and-ownership)
2. [Worker MVP](#2-worker-mvp)
3. [Security](#3-security)
4. [Interchange model](#4-interchange-model)
5. [MPXJ worker](#5-mpxj-worker)
6. [Publication and recovery](#6-publication-and-recovery)
7. [Storage and scale](#7-storage-and-scale-inputs-to-adr-011)
8. [AI and MCP boundary](#8-ai-and-mcp-boundary)
9. [Interface design](#9-interface-design)
10. [Delivery plan](#10-delivery-plan)
11. [Release, migration, and rollback](#11-release-migration-and-rollback)
12. [Risks](#12-risks)
13. [Brief for coding agents](#13-brief-for-coding-agents)
14. [Documentation plan](#14-documentation-plan)

## 1. Architecture and ownership

```text
+---------------------------------------------------------------+
| Svelte 5 interface (Wails v2)                                 |
|   Import assistant, fidelity inspector, job center,           |
|   import tools (settings), paged outline and timeline         |
+-------------------------------+-------------------------------+
                                | Wails bound methods only
+-------------------------------v-------------------------------+
| Go host: App + internal/interop/...                           |
|   worker supervisor   job runner   capability check           |
|   resource budgets    receipts     intent journal             |
|   staging builder     validators   publish + reconcile        |
+---------------+----------------------------------+------------+
                | stdin/stdout frames (D-4 a)      | SQL through
                | one job per process (D-5)        | internal/sqlitedriver
+---------------v---------------+      +-----------v-------------+
| MPXJ worker (Java)            |      | staging/<op>/project    |
|   reads the selected source   |      | (encrypted, not listed) |
|   emits typed records         |      +-----------+-------------+
|   no DB, no keys, no network  |                  | one rename after seal
+-------------------------------+      +-----------v-------------+
                                       | projects/<folder>/      |
                                       | (discovered by scan)    |
                                       +-------------------------+
```

| Information | Authority | Never |
| --- | --- | --- |
| Account, session, epoch, project copy | Host session services | Claimed by a worker |
| Schedule activities and outline | Normalized tables after ADR-011; `charts.data` until then | Two writable copies |
| WBS scope | WBS v2 contract (`docs/design/wbs-unified-model-contract.md`) | Built automatically from a source outline |
| Calendars, resources, assignments | Host normalized records | Resource names used as keys |
| Money and currency | `internal/db` and `internal/money` (ADR-009) | Float values from the source written as money |
| Source facts and unsupported fields | Immutable source snapshot and receipt | Silently discarded |
| Imported schedule results | Kept as source facts | Recomputed during import |
| Audit and receipts | Host audit chain | Written by a worker |

**Packages (proposed, inspect namespaces first):**

| Package | Role |
| --- | --- |
| `internal/interop/worker` | Supervisor, launch, lifecycle |
| `internal/interop/protocol` | `.proto` files, generated code, framing |
| `internal/interop/importer` | Staging, validation, receipts |
| `internal/interop/publish` | Intent journal, seal, rename, reconcile |
| `workers/mpxj-bridge/` | Java bridge (Gradle or Maven, pinned) |

The original `internal/plugin/*` names [v2 §4.1] are replaced, because there
is no general plugin registry in the MVP (D-3).

## 2. Worker MVP

### 2.1 Trust chain, on one page

1. The release pipeline builds the bridge JAR and a jlink-trimmed Java
   runtime for each OS. It pins MPXJ 16.10.0 and its dependency tree by
   checksum, and writes `workers/lock.json`:
   - worker ID and version;
   - protocol major and minor version;
   - the SHA-256 of every file;
   - the MPXJ version.
2. The lock file is embedded in the GoPMgr binary at build time, so a
   replaced lock file on disk is never trusted. The worker files sit inside
   the signed, notarized app bundle.
3. Before each launch, the host walks the worker folder. It rejects symlinks
   and unexpected or missing files, and checks every digest against the
   embedded lock. Any mismatch returns `PLUGIN_SIGNATURE_INVALID` and nothing
   runs.
4. The host starts the bundled `java` executable with a fixed argument list:
   no shell, no `sh -c`, and no values taken from a manifest. It uses a clean
   environment, a scratch folder as the working directory, and stdin, stdout,
   and stderr as its only inherited handles. The one exception is the
   read-only source handle of strategy (b) in §5.6, where G2 approves it.
5. Handshake: the host writes `Hello{protocol, nonce, expected_worker_digest}`.
   The worker must answer `Hello{protocol, nonce, worker_id, version,
   capabilities}` within a deadline. A version mismatch returns
   `PLUGIN_INCOMPATIBLE`.
6. The job runs. When it ends, or on cancel, the host closes stdin, waits a
   short grace period, then kills and reaps the whole process tree.

Turning the worker off must leave the core app fully usable, including Quick
chart import [REVIEW-017].

### 2.2 Framing for transport (a)

- **Frames.** Each frame is a uvarint length followed by one protobuf `Frame`
  message. A frame is at most 4 MiB, and at most 4 frames are in flight; G0
  confirms both by measurement.
- **Ordering.** `Frame` carries a monotonic `sequence`, a `job_id`, and one
  `oneof` body. Anything out of order, duplicated, or longer than the limit
  ends the job with `INPUT_INVALID` or `PLUGIN_UNHEALTHY`.
- **Stdout is frames only.** The bridge must capture the real stdout
  descriptor for frames and point `System.out` at stderr *before* MPXJ or any
  dependency loads, so a stray library print cannot corrupt the stream.
- **Stderr.** It is a log channel, capped in size and scrubbed by the host. It
  never contains project content [PLG-020].
- **Fuzzing.** The framing code is fuzz-tested on both sides (a G1 exit
  criterion).

### 2.3 Messages (typed, no opaque payloads)

```proto
syntax = "proto3";
package gopmgr.interop.v1;

message Frame {
  uint64 sequence = 1;
  string job_id = 2;              // host-issued, opaque
  oneof body {
    Hello hello = 10;  Cancel cancel = 11;  Health health = 12;
    SourceBegin source_begin = 20;  SourceChunk source_chunk = 21;
    SourceEnd source_end = 22;
    ProjectHeader project = 30;  Calendar calendar = 31;
    Task task = 32;  Relation relation = 33;  Resource resource = 34;
    Assignment assignment = 35;  BaselineValue baseline = 36;
    TimephasedSegment timephased = 37;  CustomFieldDefinition custom_def = 38;
    CustomFieldValue custom_value = 39;  FieldNote field_note = 40;
    Diagnostic diagnostic = 50;  Progress progress = 51;
    Completion completion = 52;  Failure failure = 53;
  }
}
```

The other message bodies are written in G1, with golden encoding fixtures.
Their rules:

- **Values.** Durations and work use exact integers with an explicit unit.
  Money uses a decimal string, an ISO code, and the source's scale. Dates use
  the raw source text, its zone or offset, and an interpreted instant where
  one is known.
- **Field state.** `FieldNote` reports one field's disposition (ADR-010 D-9)
  and the reason for it.
- **Completion.** `Completion` reports counts for each record type, plus a
  digest of the canonical logical stream. The host recounts and checks
  references itself before trusting it.
- **Evolution.** Protobuf field numbers are never reused; removed ones are
  reserved. A change in meaning gets a new message or a new major version.

### 2.4 Lifecycle

Each worker process runs exactly one job, so worker states and job states can
be tracked separately:

```text
Worker:  VERIFYING -> STARTING -> HANDSHAKE -> RUNNING -> EXITED
                 \-> REFUSED (digest)   \-> FAILED (timeout, crash, protocol)
```

On sign-out, an account switch, or a project switch, the host cancels the
job, kills the worker, and refuses to commit. No application-wide mutex is
held while a worker runs or while a user is reviewing.

### 2.5 Error codes and interface keys

Codes are stable and typed. The interface maps each one to a catalog key, so
no screen matches English text from the backend (ADR-008). The copy is in
§9.4.

| Code | Meaning | Host action | Catalog key |
| --- | --- | --- | --- |
| `PLUGIN_NOT_INSTALLED` | Worker files absent | Offer Quick chart import | `interop.error.notInstalled` |
| `PLUGIN_SIGNATURE_INVALID` | Digest mismatch | Refuse to run; log a security event | `interop.error.signature` |
| `PLUGIN_INCOMPATIBLE` | Protocol mismatch | Disable the worker | `interop.error.incompatible` |
| `PLUGIN_START_FAILED` | JVM or launch failure | One bounded retry | `interop.error.startFailed` |
| `PLUGIN_UNHEALTHY` | Hang, crash, or bad frames | Kill; commit nothing | `interop.error.unhealthy` |
| `INPUT_UNSUPPORTED` | Format not supported | List supported formats | `interop.error.unsupported` |
| `INPUT_INVALID` | Corrupt or malformed file | Keep the original untouched | `interop.error.invalid` |
| `INPUT_PROTECTED` | Password-protected file | Never log the secret | `interop.error.protected` |
| `RESOURCE_LIMIT` | Memory, disk, time, or expansion cap reached | Offer inspect-only | `interop.error.resourceLimit` |
| `DATA_UNMAPPED` | Essential field cannot be kept (strict mode) | Block commit | `interop.error.unmapped` |
| `STALE_SESSION` | Epoch, account, or project changed | Discard the staged import | `interop.error.staleSession` |
| `REVISION_CONFLICT` | Destination changed after review | Review again (future merge) | `interop.error.revisionConflict` |
| `USER_CANCELLED` | User cancelled | Clean up | `interop.status.cancelled` |
| `EXPORT_UNSUPPORTED` | Asked for `.mpp` output | Suggest MS Project XML | `interop.error.exportUnsupported` |
| `INTERNAL` | Anything else | Show a sanitized error ID | `interop.error.internal` |

### 2.6 Streaming rules

These follow [v2 §6.5]:

- **Sequence.** Sequence numbers are monotonic. Duplicates, gaps, a digest or
  count mismatch, an early end of file, or completion before every record
  arrives each fail the job.
- **Totals.** Total limits apply to source bytes, decompressed size, records
  of each type, nesting depth, string length, calendar exceptions, and
  timephased segments. A limit on each message alone is not enough.
- **Backpressure.** It runs from the staging writer back to the parser, so a
  slow host never makes the worker buffer the whole project.
- **References.** They are resolved only after all records arrive. A dangling
  reference stays an explicit unresolved external reference; GoPMgr never
  invents a task for it.

## 3. Security

This section condenses [v2 §7] and adds the per-job process.

**Threats.** A malicious or corrupt `.mpp`, `.xer`, or XML file. A modified
worker bundle. A buggy or compromised worker. A session change partway
through an import. Plaintext leaking through temporary files or logs. A worker
running as the same OS user can read that user's files; SQLCipher does not
protect plaintext inside a signed-in session.

| Capability | Default | Enforced by |
| --- | --- | --- |
| Launch a worker | Bundled worker whose digests match the lock | Host check before every launch |
| Read the source | Only the file the user picked, for this job | The host streams it, or passes a proven read-only handle |
| Read or write a project | Never | No protocol message carries either |
| Keys, credentials, DB handles | Never | No wire field |
| Write output files | Host only | The host's export path rules |
| Network | Not needed | **Not enforced** for the bundled worker; the documentation says so |
| Third-party workers | Off | A separate ADR (G7) and an OS-confinement gate on each OS |

**Authorization for each operation.** Each job binds:

- the account and its incarnation, and the session epoch;
- the project copy, the target (a new project), and the source digest;
- the worker digest and the options;
- the expected revision, the deadline, and the resource budget.

Immediately before publication, the host re-reads the session, re-checks
every binding, and verifies the staged checksum against the approval digest
[v2 §7.3].

**Temporary files.**

- Staging databases are SQLCipher-encrypted with the user's key. An
  unencrypted staging database is forbidden.
- A worker scratch copy holds *only* the source file. It is private (0600 in a
  0700 folder) and is deleted on every exit path.
- The documentation says that deletion on an SSD is not secure erasure.
- Users can refuse disk spooling for sensitive files, which makes some
  formats inspect-only [v2 §7.4].

## 4. Interchange model

### 4.1 Identity (P0-01)

| Table (logical) | Key | Holds |
| --- | --- | --- |
| `source_artifacts` | `artifact_id` | SHA-256, size, format, reader and version, import time, retention policy |
| `source_lineages` | `lineage_id` | Native project GUID or UID if present, producer and application version, who confirmed it and when, confidence |
| `source_entity_keys` | `(lineage_id, record_type, source_uid)` | Task ID and ordinal kept *separately*; first and last artifact seen |
| `entity_links` | `link_id` | Source key → GoPMgr entity ID; `valid_from` and `valid_to` artifacts; mapping version; confidence (`native_uid`, `user_confirmed`, `unresolved`); override note |

**Matching rules:**

- **New lineage.** A new file is a new lineage unless the user confirms
  otherwise. GoPMgr may *suggest* an existing lineage when a native GUID
  matches.
- **No name matching.** Names, paths, and WBS display codes never decide a
  match.
- **Reused identifiers.** A copied project reuses UIDs, so when the GUID
  matches but the content history diverges, the files are flagged for the
  user and not merged.
- **Moves and deletions.** Re-parenting, deletion, and UID reuse inside a
  lineage are detected and shown. Links are retired, never rewritten.

### 4.2 Fidelity (P0-06 with the v2.0 vocabulary)

The modes and dispositions are in ADR-010 D-9. The receipt extends today's
`MSPDIImportReceipt`, whose `imported_fields`, `excluded_fields`, and
`transformations` keep their meaning:

```json
{
  "receipt_schema": 2,
  "operation_id": "host-issued",
  "mode": "reviewed_best_effort",
  "worker": {"id": "org.gopmgr.mpxj", "version": "1.0.0", "digest": "sha256:…", "mpxj": "16.10.0"},
  "source": {"artifact_id": "…", "format": "mpp", "sha256": "…", "bytes": "18874368"},
  "lineage": {"lineage_id": "…", "basis": "native_guid", "confirmed": true},
  "counts": {"tasks": {"source": "52310", "stored": "52310"}, "relations": {"source": "…", "stored": "…"}},
  "dispositions": {"exact": "…", "converted_exact": "…", "converted_approximate": "…", "derived": "…",
                   "preserved_read_only": "…", "unsupported_by_reader": "…", "not_modeled": "…",
                   "excluded_by_user": "…", "source_absent": "…", "failed": "0"},
  "imported_fields": [], "excluded_fields": [], "transformations": [],
  "field_report": "evidence object id (encrypted, paged)",
  "project_id": "host-issued, known before staging",
  "audit_reference": "…"
}
```

Counts are decimal strings, so large values survive JSON exactly. A field
report lists one row for each field path and record group, with its
disposition, reason, severity, and export handling. It is stored as encrypted
evidence and read in pages.

**Evidence matrix.** Every format and version family gets its own matrix,
covering: metadata, outline, tasks, relations, calendars, resources,
assignments, costs, constraints, baselines 0-10, timephased data, custom
fields, and notes. Each entry records:

- the source counts, and counts as MPXJ reads them;
- counts on the wire, in staging, and on read-back;
- counts on export, where export exists.

Nothing is marked "complete" without this matrix [P0-06 acceptance].

### 4.3 Value rules

These follow [v2 §8.3]:

- **Dates.** Keep the raw source text, its zone or offset, and the
  interpreted instant. Date-only values stay date-only; they are never
  converted to midnight UTC.
- **Durations, work, and lags.** Use exact units together with the source's
  calendar settings. The 8-hour assumption applies only to the legacy chart
  importer.
- **Money.** Keep the source's decimal precision and ISO code as typed
  source values. Convert only at the `internal/money` boundary, with checked
  rounding (ADR-009).
- **Missing values.** `unset`, `null`, zero, and `unsupported` are four
  different states.

### 4.4 Scheduling interpretation

The source's computed dates, float, and critical flags are stored as source
facts. GoPMgr's kernel recalculates only when the user asks, through
**Recalculate with GoPMgr**, which shows a before-and-after preview [UX-07].
A compare-only view explains any differences: calendar exceptions, lags,
scheduling modes, and time zones.

### 4.5 Field mapping deliverable

Before any production import, `docs/interoperability/mpxj-field-mapping.md`
must exist [v2 §8.6]. It has one row per mapping, with these columns:

- source format;
- MPXJ accessor;
- source type and units;
- wire message;
- GoPMgr destination;
- conversion rule and null rule;
- precision and round-trip claim;
- fixture and limitations.

It is generated from the pinned MPXJ API and from fixtures. Field names are
never guessed.

### 4.6 Create, compare, and apply

These follow [v2 §8.7]:

| Operation | Phase | Writes |
| --- | --- | --- |
| Create from import | G3 | A new project only (§6) |
| Compare two projects or snapshots | S6, read-only | None |
| Apply selected differences | S6, writes; separate approval | Needs a shadow generation (ADR-010 D-8). No automatic "source wins". |

Conflicts use the v2.0 taxonomy, such as `IDENTITY_AMBIGUOUS` and
`DESTINATION_EDITED`. Any selection must show how many rows are off screen
[REVIEW-013].

## 5. MPXJ worker

### 5.1 Formats (verified against mpxj.org on 2026-10-24)

| Format | MPXJ reads | MPXJ writes | GoPMgr v1 intent | Evidence gate |
| --- | --- | --- | --- | --- |
| Microsoft Project `.mpp` (Project 98 on) | Yes | **No** | Inspect, then import | Real MPP fixtures across versions |
| Microsoft Project `.mpt` templates | Yes | No | Candidate | Fixture |
| MSPDI `.xml` | Yes | Yes | Import; export later | Compare with Quick chart import |
| MPX | Yes | Yes | Candidate | Fixture |
| Primavera P6 `.xer` and PMXML | Yes | Yes | Import; export later | P6 calendar, resource, and baseline fixtures |
| P6 SQLite databases | Yes | No | Out of v1 (avoids sqlite-jdbc native libraries) | Separate decision |
| ProjectLibre `.pod` (ProjectLibre 1.5.5 on) | Yes (`ProjectLibreReader`) | No | Candidate | Fixture before advertising |
| Planner, SDEF, and others | Varies | Varies | Not listed until tested | Fixture |

Formats are detected by signature and a parse check, never by extension
alone. XML parsing disables external entities.

### 5.2 Import workflow

This revises [v2 §9.2]:

1. **Choose the file.** The user picks a file in the import assistant. The
   host requires a signed-in session and checks the file:
   - it must be a regular file under the size and quota limits;
   - its content signature is sniffed;
   - its SHA-256 is computed from the handle that was opened, never by
     opening the path again.
2. **Bind the job.** The host binds it to the account, epoch, source digest,
   worker digest, options, and budget (§3).
3. **Launch and inspect.** The host verifies and launches the worker (§2.1)
   and runs an inspection: format, producer and version, counts,
   relationships, calendars, baselines, timephased data, and custom fields.
   It also estimates the memory, disk, and time needed (§5.6). Nothing is
   written.
4. **Choose options.** The user picks a mode and a lineage (§4.1). An
   essential-field profile is optional. A saved mapping profile is offered
   only for the same producer and version family.
5. **Import into staging.** A second worker job runs the import. Records go
   into the encrypted staging database in `staging/<op>/`, in bounded
   transactions with backpressure.
6. **Validate staging.** The host checks:
   - every reference;
   - the counts against `Completion` and its own recount;
   - units, dates, and money;
   - the WBS boundary;
   - strict-mode essential fields.

   It builds the receipt and the field report.
7. **Review.** The user sees the impact summary and the fidelity inspector,
   then confirms **Import as new project**. That confirmation binds the
   staged checksum.
8. **Seal, recheck, and publish.** The host follows §6. It writes the
   receipt and the import audit event into the staged project, seals it,
   rechecks the session bindings, and renames it into `projects/`. The
   project is complete the moment it becomes visible.
9. **Read back.** The host reads the published project back and checks its
   counts. Only then does it report success. A completed worker job alone
   never counts as success [v2 §9.2 step 13].

### 5.3 Import job states

```text
CREATED -> SOURCE_CHECKED -> INSPECTING -> INSPECTED -> OPTIONS_SET
   -> STAGING -> VALIDATING -> READY_FOR_REVIEW -> APPROVED
   -> SEALING -> RECHECK -> PUBLISHING -> PUBLISHED -> VERIFIED

Before RECHECK:     cancel, timeout, crash, stale session, or invalid
                    -> CLEANUP -> CANCELLED or FAILED
                       (no project appears; staging is removed, or left
                       for reconciliation after a crash, §6.3)
From RECHECK on:    cancel is refused. The interface says the import is
                    finishing (§6.4). A stale session found at RECHECK
                    still stops the import before the rename.
```

Only the host moves a job to `APPROVED`, and only after a real user action. A
job publishes at most once; a duplicate request returns the existing receipt.

### 5.4 Export (G4, after import is proven)

This follows [v2 §9.4]. Only formats MPXJ actually writes are offered (§5.1),
and `.mpp` never is. The host:

1. snapshots the authorized data;
2. reports the risk of loss *before* writing;
3. lets the worker stream the output;
4. checks the output, optionally by re-reading it with MPXJ;
5. publishes the file atomically with a no-replace rename (§6.4), never
   overwriting unless the user asked;
6. records an export receipt.

### 5.5 Versions, licences, and dependencies

- **Pin everything.** Pin MPXJ 16.10.0, its dependency tree with checksums,
  the protobuf-java version, and the JDK used for jlink. Upgrades are
  deliberate and run the full fixture regression [R-17].
- **Runtime dependencies (verified from the pom).** POI, sqlite-jdbc,
  jgoodies-binding, rtfparserkit, the JAXB API and runtime, jackcess, jsoup,
  and Jackson. G0 checks each against the readers v1 needs:
  - sqlite-jdbc (native code; P6 SQLite only) is excluded if those databases
    stay out of scope;
  - jgoodies-binding (Swing binding, used by MPXJ's explorer) is excluded if
    no reader needs it.
- **Java runtime.** MPXJ targets Java 8 (`<source>1.8</source>`). The bundled
  runtime is a jlink image of a supported LTS JDK, and G0 tests it with the
  pinned MPXJ.
- **Licences (ADR-010 D-16).** Ship the notices, SBOM, and REUSE
  annotations, and offer the corresponding source for MPXJ. Get a legal review
  before the first release that includes the worker.

### 5.6 Resource model (P0-03)

| Budget | Initial setting | Set from |
| --- | --- | --- |
| JVM heap (`-Xmx`) | One per format family, measured | G0 and G2 heap profiles on T1 and T2 fixtures |
| OS memory limit | cgroups (Linux) and Job Objects (Windows) where available; macOS relies on `-Xmx` plus a watchdog on resident memory | G0 |
| CPU and wall-clock deadline | Measured, with headroom | G2 |
| Scratch bytes | Source size plus margin | §5.7 |
| Timephased segments | Capped; above the cap, inspect-only | G2 and S4 |
| Host plus worker resident memory | Reported together in every benchmark | P1-09 |

**Source strategies.** Each reader gets one of two, with its threat model
written down:

- **(a) Streamed bytes.** The worker writes the streamed bytes to a private
  scratch copy that it can seek in. This is the default.
- **(b) Read-only handle.** The host passes an already-open, read-only handle
  instead. This is allowed only where G2 proves that handle inheritance works
  on all three OSes and that the reader accepts a stream or channel.

Workers are never given a user path they could resolve themselves.

**Refusal.** Before parsing, the host compares its estimate with free memory,
free disk, and the budgets. If the import cannot fit, it refuses with
`RESOURCE_LIMIT` and keeps inspect-only available.

### 5.7 Cost of copies

A large import can touch the bytes several times:

- the source is read for its hash;
- the worker writes a scratch copy;
- the staging database is written;
- the evidence copy is written, if the source is kept;
- the published project is written.

G2 measures each step. The plan reuses the stream that computes the hash to
fill the scratch copy, so the source is read only once.

## 6. Publication and recovery

This applies P0-02 to GoPMgr's real layout.

### 6.1 Layout

```text
<DataDir>/
  projects/<folder>/project.gopmgr    discovered by enumerateProjects
  staging/<operation-id>/             never scanned; same volume as projects/
    journal.json                      phase record (no project content)
    project/project.gopmgr            staged encrypted database
    evidence/                         encrypted source copy and field report, if kept
  quarantine/<operation-id>/          interrupted or failed imports kept for the user
```

The staged folder is created exclusively, as `reserveProjectFolder` does
for new projects today. The project ID is issued before staging starts. The
final folder name in `projects/` is chosen at publication and written to the
journal *before* the rename. If the name is taken, the rename fails (§6.4) and
another name is chosen.

### 6.2 Phases and the helpers they reuse

| Phase | Work | Existing code to reuse |
| --- | --- | --- |
| PREPARED | Create `staging/<op>` and write `journal.json` (operation ID, project ID, account, source and worker digests). Create the encrypted database. Import in bounded transactions. After approval, write the receipt and the import audit event into the staged database | `db.InitEncryptedDB`, the session DEK, the existing audit append |
| SEALED | Validate integrity, foreign keys, counts, and the audit chain; checkpoint the WAL and switch to the DELETE journal; close; remove sidecars; set private modes; fsync the file and folder; record the seal digest in the journal | `requireEncryptedIntegrity`, `prepareForRename`, `removeSQLiteSidecars`, `ensurePrivateSQLiteFiles` |
| PUBLISHED | Re-check the session bindings; write the chosen folder name to the journal; rename `staging/<op>/project` to `projects/<folder>` without replacing anything (§6.4); fsync `projects/`; mark the journal PUBLISHED | (new) |
| DONE | Read back and verify the counts; remove `staging/<op>` | (new) |

No write reaches the project after the rename, so the published project is
always the sealed one.

### 6.3 Reconciliation

Reconciliation runs at sign-in, for that user's `staging/` folder only:

| Journal phase found | State on disk | Action | What the user sees |
| --- | --- | --- | --- |
| PREPARED | Staging database, maybe partial | Move to `quarantine/`; delete after 30 days (the source file is untouched) | "An import didn't finish. Nothing was added to your projects." |
| SEALED | Staged project still present | Verify the seal digest. If it matches, ask whether to publish; otherwise quarantine | "An import was ready but not added. Add it now?" |
| SEALED | Staged project gone; the journal's folder is in `projects/` | The rename finished before the journal update. If the seal digest matches, mark PUBLISHED and continue; otherwise stop and report | "An import finished while GoPMgr was closing." |
| PUBLISHED | Project in `projects/`, staging left over | Read back, verify, remove staging | "An import finished while GoPMgr was closing." |
| No journal | Unknown folder | Quarantine; never delete | Shown in Settings → Import tools |

### 6.4 Platform notes and the cancel boundary

**Platforms:**

- **Never replace.** Go's `os.Rename` is not enough here:
  - On Linux and macOS it refuses an existing target folder, but only
    through an `Lstat` check made before `rename(2)`. An empty folder
    created between the two steps is still replaced.
  - On Windows it passes `MOVEFILE_REPLACE_EXISTING`.

  Publication instead uses the no-replace forms from `golang.org/x/sys`
  (already a direct dependency, v0.49.0), which check and rename in one step:
  - `unix.Renameat2` with `RENAME_NOREPLACE` on Linux;
  - `unix.RenamexNp` with `RENAME_EXCL` on macOS;
  - `windows.MoveFileEx` without the replace flag on Windows.

  If a file system refuses the no-replace call, publication fails rather
  than falling back.
- A rename within one volume is atomic on all three OSes.
- Windows cannot fsync a directory, so recovery relies on the journal there.
  On macOS, closing the file is followed by an `F_FULLFSYNC` check where G3
  shows it is needed.
- Disk-full and power-loss faults are injected at every boundary, as listed in
  the test strategy.

**Cancel boundary:** cancelling is allowed through `SEALING`. From `RECHECK`
on, the interface shows "Finishing — can't cancel now" [UX-06].

**Follow-up outside this ADR:** `CreateProject` and `RestoreProjectArchive`
should adopt the same protocol. Today they write inside `projects/`.

## 7. Storage and scale (inputs to ADR-011)

This section collects the requirements that the storage ADR must settle. It is
*not* a migration [P0-07].

### 7.1 Constraints

| Constraint | Source | Consequence |
| --- | --- | --- |
| SQLite 3.33.0 | `sqlite3.h:126` | No `STRICT`, `RETURNING`, `DROP COLUMN`, or `unixepoch()`. Generated columns, UPSERT, window functions, and `WITHOUT ROWID` are available. |
| No JSON1 or FTS5 in releases | Build tags (`Makefile:19`) | No SQL JSON functions. Search uses indexed `LIKE` or prefix keys unless a tag change is approved. |
| Foreign keys on per connection | DSN (`sqlite.go:64-75`) | Keep it. Run `foreign_key_check` after migrations that disable foreign keys. |
| WAL, with `synchronous=NORMAL` | DSN | Single writer; checkpoint policy in §7.6 |
| No format gate | `user_version` never set | The gate release ships first (ADR-010 D-11) |
| Schema grows by `IF NOT EXISTS` and `ADD COLUMN` | `internal/db/sqlite.go` | New tables are additive; rollback cannot rely on `DROP COLUMN` |

### 7.2 Data layers

These follow [v2 §11.2]:

1. the source bytes (evidence);
2. the interchange snapshot (what was read);
3. normalized records (the authority);
4. WBS scope (its own contract);
5. derived projections, which can be rebuilt.

Charts become views, not storage, for large schedules.

### 7.3 Physical schema plan (P1-01)

- **Keys.** Use integer surrogate keys for heavy tables, with scoped unique
  constraints on business IDs. Avoid long composite text keys on hot paths.
- **Hierarchy.** Store `parent_task_id` and an ordinal for the outline. Add a
  separately measured flattened `visible_outline_index` for each schedule
  generation only if it beats recursive queries (§7.8).
- **Relations.** Index relations both ways, by predecessor and by successor.
- **Timephased data.** Index `(series_id, start, end)`; see §7.7.
- **Custom fields.** Store sparse typed custom values, keyed by definition.
- **Statistics.** Run `ANALYZE` after a bulk load. Keep the
  `EXPLAIN QUERY PLAN` output for every hot query.
- **`WITHOUT ROWID`.** Benchmark it only for small composite-key tables; it
  is not assumed faster in general.

### 7.4 Paging and cursors (P1-02)

- **Short reads.** Each page is a short read transaction. A long-lived reader
  across Wails calls would stop WAL checkpoints [REVIEW-009].
- **Keyset paging.** Pages use a keyset on `(sort key, stable surrogate ID)`.
- **Cursors.** Cursors are opaque and host-authenticated with an HMAC. Each is
  bound to the account, project copy, schedule generation, filter, and sort,
  and has an expiry.
- **Revision changes.** If the revision changes, the next page reports
  `revision_changed`. The interface then re-anchors on the selected task ID
  and does not silently skip or repeat rows [REVIEW-008].
- **Counts.** Each count is marked as `exact`, `estimated`, or `unknown`.

### 7.5 Derived computations (P1-03)

- **Cache keys.** Each cache is keyed by `(project copy, schedule generation,
  algorithm version, calendar revision, options digest)`.
- **Change impact.** A change-impact graph covers task edits, relations,
  assignments, calendars, WBS links, EVM, and risk.
- **Incremental CPM.** It becomes the authority only after it is proven equal
  to a full recompute on fixtures.
- **Expensive work.** Monte Carlo, levelling, and portfolio rollups run as
  cancellable jobs in the job center. A single save never waits for them.

### 7.6 Write volume and WAL (P1-04)

- **Bulk import.** Use prepared statements, bounded batches, and indexes
  built after the load where measured faster.
- **Checkpoints.** Use an explicit checkpoint policy after an import, and
  track the size of the WAL file.
- **Disk-full faults.** `SQLITE_FULL` is injected during staging, sealing, and
  normal edits.
- **Staging in memory.** Compared against encrypted staging by measured size.
  Plaintext must never spill to disk.

### 7.7 Timephased data (P1-05)

- Keep irregular segments and their original units. Never expand them into
  minute or day rows at import.
- Aggregate lazily to the zoom level the timeline asks for, and cache short
  ranges.
- Benchmark one million points with both sparse and dense contours.

### 7.8 Outline and timeline APIs (P1-06)

```text
ListOutlineRows(handle, schedule, cursor, limit, filter, sort, expanded)
  -> rows[], next_cursor, revision, count{value, kind}
GetTimelineViewport(handle, schedule, row_range, time_range, px_per_day)
  -> bars[], links[] (clipped), calendar_hints[], density_bins[]
GetActivityDetails(handle, schedule, task_id) -> typed entity, bounded tabs
ApplyActivityChange(expected_revision, operation_id, typed_patch)
  -> new_revision, audit_id, affected_ids, warnings
```

- Focus and selection survive sort, filter, folding, zoom, and refresh.
- Expanding everything at 100,000 rows is a background job, not a mass mount.
- Canvas is used for dense bars only if benchmarks show the gain. Visible rows
  keep DOM and ARIA semantics.

### 7.9 Analytics (S7A, P1-07)

- **Domain queries first.** Define them with exact-money aggregation over
  indexed SQLCipher records.
- **Larger imports.** `ImportTabularToDataset` and `QueryDatasetPage` handle
  large files. The legacy 10,000-row result cap stays, and reports
  `result_truncated` separately from `dataset_fully_imported`.
- **Engines.** No engine (DataFusion, Arrow, Iceberg) is chosen here.
  Results must match the GoPMgr kernel and `internal/money`.
- **Portfolio rollups.** They use only the snapshots the user approved. A
  user's account isolation is never bypassed [REVIEW-019].

### 7.10 Audit (S7B, P1-08)

- **Old events stay valid.** Old events keep their bytes; `eventHash` and
  `canonicalJSON` do not change.
- **New events.** New schedule commands write
  `{"schema":"gopmgr.audit.delta.v1", …}` inside the existing before and after
  fields. Numbers that need exactness (money, large integers, revisions) are
  strings, because the canonicalizer decodes generic JSON [P1-08].
- **Checkpoints.** They are triggered by either the edit count or the
  evidence bytes. Replay is checked against recorded per-record hashes.
- **Separation.** Receipts (idempotency), undo history, and audit evidence
  have separate retention rules. The 256-receipt WBS limit applies to WBS
  only.

### 7.11 Performance objectives (P1-09)

- **Hardware.** Publish two profiles: minimum supported and recommended. The
  owner decides them at S0.
- **Measure each stage separately:**
  - cold open, conversion, staging, seal and publish;
  - first viewport, scroll p95, indexed search, and a single edit;
  - dependency recalculation and a risk run;
  - export, snapshot, and restore;
  - worst-case host plus JVM memory, and scratch use.
- **Workload tiers [v2 §11.11]:**
  - T0: 100-500 tasks;
  - T1: 10,000;
  - T2: 50,000, the design target;
  - T3: 100,000 tasks and one million timephased points, a stress target.
- **Budgets are proposals until measured.** Proposed T2 budgets on the
  recommended profile: a page of up to 200 rows within 250 ms p95 when warm,
  and a foreground command within 500 ms p95.

## 8. AI and MCP boundary

The future AI ADR must decide the following, and nothing ships before it
does [P0-04]:

- **Where the server runs.** MCP is embedded in the running, signed-in app.
  A separate broker is out.
- **Client grants.** A grant names an account, project, and copy. It is
  read-only, short-lived, and revocable. Its token never appears on command
  lines, in logs, or in permanent configuration.
- **Revocation.** Grants are revoked on sign-out, project switch, account
  switch, GUI exit, administrator switch, and worker restart, and when a tool
  call outlives its grant [REVIEW-007].
- **Remote use.** Remote clients need a separate decision. Any HTTP transport
  validates `Origin`.
- **Tools and model output.** Tools read through the same paged query
  services as the interface. Model output is untrusted data. Writes come only
  as proposals that a user approves [v2 §10.3].

## 9. Interface design

### 9.1 Principles

- **Fit the existing app.** Dense, operational desktop screens in the
  existing slate and cyan palette, set in IBM Plex. Reuse `Button` variants,
  `Card`, `Tabs`, `Input`, `Select`, `ConfirmDialog`, `ToastContainer`, and
  `Spinner`.
- **Translatable.** All text goes in `messages/en.ts` as full sentences with
  `{placeholders}` and plural forms (ADR-008). The hardcoded-text ratchet
  covers every new component.
- **Keyboard first.** Every flow works without a mouse. Enter handlers ignore
  `isComposing`. Focus is visible and stays put across paging.
- **Honest status.** No "100%" during long validation, no "sandboxed" badge,
  and no success message before read-back verification.
- **Accessible and colour-independent.** Status always has a text label as
  well as a colour.
- **Consistent with the plan.** The work follows
  `docs/design/accessibility-remaining-work-plan.md` where it overlaps.

### 9.2 Screens

**Import assistant** (UX-01, G2 for inspection, G3 for import). It is a
five-step stepper:

1. **Choose file.** One file picker; supported formats are listed with
   read-only formats marked as such.
2. **Inspect.** Format, producer and version (with how sure GoPMgr is), size,
   and counts of summary and leaf tasks, relationships, calendars, resources,
   assignments, baselines, timephased data, and custom fields. It also shows
   the estimated memory, disk, and time, and any limits.
3. **Map and check.** Choose a mode (Strict, Reviewed, or Inspect only),
   whether this is a new lineage or matches an existing one, and an optional
   list of essential fields.
4. **Review.** An impact summary and the fidelity inspector.
5. **Import.** The **Import as new project** button. Merging is not offered
   in v1.

The stepper stays inspectable even when scale limits block the import.

**Fidelity inspector** (UX-02). A searchable grid with these columns:

- field and group;
- records affected;
- source form and GoPMgr form;
- what happens (the disposition);
- severity;
- whether it is editable or exportable.

Filters: Needs attention, Not supported, Kept read-only, Converted, Exact.
**Save import report** writes a report that leaves out notes, local paths,
and field values unless the user ticks them in. A PDF report uses the ADR-006
PDF/A-2u pipeline.

**Job center** (UX-06). One panel that lists running and recent jobs. Each
shows its phase, meaningful counters (bytes read, tasks checked, records
staged), and states that tell queued and running apart. Cancel is offered
only before the boundary in §6.4. A failed job keeps its receipt and offers
**View details**, and **Try again** where the operation can safely be
repeated.

**Import tools** (UX-05, Settings). Shows the bundled worker only:

- status, version, and supported formats, with read and write listed
  separately;
- whether it is on or off, its disk use, and its last self-test;
- release provenance, and a **Run diagnostics** button.

It states plainly that the worker runs as a separate program and is not
sandboxed. There is no install button.

**Imported versus recalculated** (UX-07). A toggle between source values and
GoPMgr's values. Differences are highlighted, each with a reason.
**Recalculate with GoPMgr** shows a preview first.

**Large schedules** (UX-04, S5):

- **Finding tasks:** search by ID, UID, WBS path, or text; jump to a task; a
  breadcrumb trail.
- **Navigating:** **Expand this branch** instead of expand-all, a sticky name
  column, keyboard moves to the parent, predecessor, and successor.
- **Overview:** a minimap and zoom to selection.
- **Counts:** loaded, total, and estimated rows.
- **Screen readers:** the row position is announced.

**History** (UX-08). Four distinct actions: Undo an unsaved edit, Revert a
saved change (which is a new audited change), Restore a backup, and Re-import
the original.

**Compare** (UX-03, S6) and **AI** (UX-09, the future ADR) are specified
there, not here.

### 9.3 Design system additions

Each addition extends an existing primitive. None brings a new UI dependency.

| Component | Built from | Purpose | Accessibility |
| --- | --- | --- | --- |
| `Stepper` | `Tabs` styling, `Button` | The five assistant steps; step state is complete, current, upcoming, or blocked | `nav` with `aria-label`; `aria-current="step"`; blocked steps explain why |
| `DataGrid` | New; table semantics | Fidelity inspector, job history, field reports | `role="grid"`, roving tabindex, sortable headers with `aria-sort`, paged with a loaded/total count |
| `TreeGrid` | `DataGrid` and an outline model | Large-schedule outline (S5), virtualized | `role="treegrid"`, `aria-level`, `aria-expanded`, `aria-rowindex` and `aria-rowcount` while virtualized |
| `StatusBadge` | Existing badge classes | One status vocabulary for workers, jobs, and dispositions | Text label always; colour only adds meaning |
| `PhaseProgress` | `Spinner` and a list | Job phases with counters; no fake percentages | `aria-live="polite"` for phase changes only |
| `ResourceEstimate` | `Card` | Memory, disk, and time needed versus available | Values as text, with units formatted through `formatNumber` |

**Status colour tokens**, mapped to classes already in use:

| Status | Colour | Example classes |
| --- | --- | --- |
| Exact, Ready, Done | emerald | `text-emerald-300`, `bg-emerald-950/40` |
| Converted, Running | cyan | `text-cyan-300` |
| Converted with loss, Needs attention, Finishing | amber | `text-amber-300`, `border-amber-700/50` |
| Not supported, Off, Cancelled | slate | `text-slate-400` |
| Failed, Can't start, Signature mismatch | red | `text-red-300`, `bg-red-950/40` |

**Every component has these states:** loading, empty, error, disabled, and
read-only. The empty and error states use the copy in §9.4.

### 9.4 Copy (catalog keys and English text)

Errors follow this pattern: what happened, why, and what to do. Product names
(Microsoft Project, Primavera P6, ProjectLibre) and file extensions are not
translated. "MPXJ" appears only in diagnostics.

| Key | English |
| --- | --- |
| `interop.import.title` | Import a project |
| `interop.step.choose` | Choose file |
| `interop.step.inspect` | Inspect |
| `interop.step.map` | Map and check |
| `interop.step.review` | Review |
| `interop.step.import` | Import |
| `interop.choose.formats` | Supported: Microsoft Project (.mpp, .xml) and Primavera P6 (.xer, .xml). Settings → Import tools lists every format. |
| `interop.inspect.nothingSaved` | Inspecting reads the file only. Nothing is saved. |
| `interop.mode.strict` | Strict: stop if any essential field can't be kept exactly. |
| `interop.mode.reviewed` | Reviewed: import what can be kept, and review every field that changes or can't be kept. |
| `interop.mode.inspectOnly` | Inspect only: show what the file holds. Nothing is saved. |
| `interop.lineage.new` | Treat this as a new project |
| `interop.lineage.match` | This is a newer copy of {project} |
| `interop.lineage.copyWarning` | This file uses the same task IDs as {project} but its history is different, so it may be a copy. GoPMgr won't merge them automatically. |
| `interop.review.attention` | `{one: "{count} field needs your attention.", other: "{count} fields need your attention."}` |
| `interop.import.asNew` | Import as new project |
| `interop.import.cancel` | Cancel import |
| `interop.import.finishing` | Finishing — can't cancel now. The project will appear when it's ready. |
| `interop.import.done` | {project} was imported. {kept} of {total} fields were kept exactly. |
| `interop.filter.attention` | Needs attention |
| `interop.filter.unsupported` | Not supported |
| `interop.filter.readOnly` | Kept, read-only |
| `interop.filter.converted` | Converted |
| `interop.filter.exact` | Exact |
| `interop.report.save` | Save import report |
| `interop.report.privacy` | The report leaves out notes, file locations, and field values unless you include them. |
| `interop.tools.notSandboxed` | Runs as a separate program bundled with GoPMgr. It isn't sandboxed. |
| `interop.tools.diagnostics` | Run diagnostics |
| `interop.recovery.unfinished` | An import didn't finish last time. Nothing was added to your projects. |
| `interop.recovery.ready` | An import of {project} was ready but not added. Add it now? |
| `interop.recovery.finished` | An import of {project} finished while GoPMgr was closing. It's in your projects. |
| `interop.legacy.quickChart` | Quick chart import (MS Project XML) |
| `interop.error.notInstalled` | The import tool for Microsoft Project and Primavera files isn't installed. You can still use Quick chart import for MS Project XML files. |
| `interop.error.signature` | GoPMgr didn't start the import tool because its files don't match this release. Reinstall GoPMgr to restore them. |
| `interop.error.incompatible` | This import tool doesn't work with this version of GoPMgr. Reinstall GoPMgr to get a matching version. |
| `interop.error.startFailed` | The import tool couldn't start. Try again. If it keeps failing, run diagnostics in Settings → Import tools. |
| `interop.error.unhealthy` | The import tool stopped responding, so the import was cancelled. Nothing was saved. Try again. |
| `interop.error.unsupported` | GoPMgr can't import {format} files. It can import: {formats}. |
| `interop.error.invalid` | This file couldn't be read. It may be damaged or not a {format} file. Open it in the program that made it, save a new copy, and try again. |
| `interop.error.protected` | This file is password-protected. Remove the password in the program that made it, then import that copy. |
| `interop.error.resourceLimit` | This file needs more {resource} than GoPMgr can use for an import on this computer ({needed} needed, {available} available). You can still inspect it. |
| `interop.error.unmapped` | `{one: "{count} essential field can't be kept exactly, so Strict mode stopped the import. Switch to Reviewed to import the rest.", other: "{count} essential fields can't be kept exactly, so Strict mode stopped the import. Switch to Reviewed to import the rest."}` |
| `interop.error.staleSession` | You signed out or switched accounts, so the import stopped. Nothing was saved. Sign in and start again. |
| `interop.error.revisionConflict` | The project changed after you reviewed these changes. Review them again before applying. |
| `interop.status.cancelled` | Import cancelled. Nothing was saved. |
| `interop.error.exportUnsupported` | GoPMgr can't save .mpp files. Microsoft Project opens MS Project XML (.xml), so export that instead. |
| `interop.error.internal` | Something went wrong, so nothing was saved. Error ID: {id}. |

**Translator notes:**

- `{resource}` is a catalog value ("memory", "disk space", or "time"), not
  free text.
- `{needed}` and `{available}` are formatted numbers with units.
- `interop.error.unmapped` takes a plural form.
- Avoid idioms; German and French text runs 30-40% longer, so buttons wrap
  rather than truncate.

**Alternatives considered for the main button:**

| Option | Copy | Notes |
| --- | --- | --- |
| A (chosen) | Import as new project | States the outcome; leaves room for "Import into…" later |
| B | Create project | Hides that the data comes from the file |
| C | Import | Ambiguous once merge exists |

## 10. Delivery plan

### 10.1 Phases (G)

| Phase | Goal | Exit gate (summary) |
| --- | --- | --- |
| **G0** Feasibility | Transport A against B with the same `Hello`; JVM start-up, heap, and memory; MPXJ 16.10.0 dependency, licence, and SBOM draft; jlink size per OS; signing and notarization of the runtime; fixture corpus plan; baselines for current imports | One transport chosen with evidence; launch, reap, and cancel work offline on Windows, macOS, and Linux; no blocker on licence or packaging |
| **G1** Worker MVP | `.proto` plus code generation; framing; supervisor; digest lock; fake worker; error codes; fuzzing; settings page that is read-only | No regressions with the worker off; no leaked processes or descriptors; no listener; golden protocol tests |
| **G2** Inspect-only MPXJ | Java bridge; streamed source and scratch; format detection; inspect counts; resource budgets; malformed, oversized, and protected fixtures | No database writes; counts match MPXJ directly; limits enforced; clean logs |
| **G3** New-project import | Typed records, field ledger, identity (§4.1), encrypted staging, publication (§6), assistant and inspector | Requires S0-S4 and the format gate; every failure injected leaves no partial project; read-back matches; honest labels |
| **G4** Export, scale, release | MSPDI export first; S5 interface; benchmarks; installers; notices | `make check-release` plus worker gates on supported targets |
| **G5** Read-only MCP | Only after the AI ADR | Grants revoked correctly; no writes; no data leaves the computer without consent |
| **G6** AI proposals | Separate approval | No self-approval; stale proposals do nothing |
| **G7** Third-party workers | Separate ADR | OS confinement proven on each target |

### 10.2 Scale workstream (S)

| Gate | Deliverable | Depends on |
| --- | --- | --- |
| **S0** | Census and baselines with file and line references (§7.1); hardware profiles | G0 |
| **S1** | ADR-011 storage design: DDL, format gate, migration and rollback | S0 |
| **S2** | Format gate release, then the additive schema behind a flag | S1 |
| **S3** | Typed commands and queries; encrypted staging; receipts; idempotency | S2, G1 |
| **S4** | Field completeness, ledger, and read-back | S3, G2 |
| **S5** | Virtualized outline and timeline | S3 |
| **S6** | Compare (read-only), then apply with separate approval | S3, S4 (S5 for the interface) |
| **S7A** | Analytics: dataset ingest, paging, deterministic aggregates | S3 |
| **S7B** | Compact audit events, mixed-chain verifier, replay | S3, and the audit design accepted |

### 10.3 Addendum amendment order

| Patch | Content | Status in v2.1 |
| --- | --- | --- |
| A0 | Numbering and decision inventory | Done (ADR-010) |
| A1 | Lineage and identity | Designed (§4.1); built in G3 |
| A2 | Publication journal | Designed (§6); built in G3 |
| A3 | Typed protocol and resource probe | Outlined (§2.3, §5.6); G1 and G2 |
| A4 | Strict modes and field matrix | Designed (§4.2); G3 |
| A5 | MCP grants | Requirements (§8); AI ADR |
| A6 | Physical schema and paging | Requirements (§7.3-7.4); ADR-011 |
| A7 | Incremental computation | Requirements (§7.5); ADR-011 and S7A |
| A8 | Audit semantics | Requirements (§7.10); S7B |
| A9 | Import, job, navigation, and compare UX | Designed (§9); G2-G4 and S5-S6 |

### 10.4 Pull-request slices

Each slice is its own pull request. Never combine G1-G3, or a schema change
with an importer.

| PR | Scope | Must not include |
| --- | --- | --- |
| P0 | This documentation (ADR, plan, test strategy, sources) | Code |
| P1 | G0 evidence and S0 census (documentation and throwaway spikes outside the product) | Production code paths |
| P2 | `.proto`, framing, fake worker, supervisor (G1) | MPXJ, database writes |
| P3 | Bridge inspect-only and packaging (G2) | Staging, commit |
| P4 | ADR-011 (S1) | Schema code |
| P5 | Format-gate release (S2 part 1) | New tables |
| P6 | Additive schema behind a flag (S2 part 2) | Import activation |
| P7 | Commands, queries, and staging (S3) | Interface |
| P8 | Field ledger and identity (S4, G3 part 1) | Publication |
| P9 | Publication and reconciliation (G3 part 2) | Merge |
| P10 | Assistant, inspector, and job center (G3 part 3) | Large views |
| P11 | Virtualized outline and timeline (S5) | All-row rendering |
| P12 | MSPDI export (G4) | `.mpp` writing |

Each pull request records:

- the HEAD it was built on;
- the contracts and invariants it touches;
- its fixtures, and the tests run, blocked, or skipped;
- the performance change;
- its effect on licences and the SBOM;
- how to roll it back;
- what users will see change.

## 11. Release, migration, and rollback

This condenses [v2 §15].

- **Flags.** One flag enables the worker supervisor. A separate flag enables
  import publication. Inspection can be on by itself.
- **Order:**
  1. documentation;
  2. G1 with the worker off;
  3. inspection for testers;
  4. the format-gate release;
  5. new-project import for testers;
  6. export formats, one at a time;
  7. AI, only after its ADR.
- **Stop at once if any of these happens:**
  - a digest bypass;
  - a leak across users or projects;
  - a key exposure;
  - a partial project published;
  - a commit from a stale session;
  - a leaked JVM;
  - an unbounded parse;
  - loss of a core flow.
- **Response:** turn the flag off, cancel jobs, keep the evidence without any
  project content, and ship the previous signed build.
- **Schema caveat.** Rolling back the binary does not roll back a schema.
  ADR-011 defines backups, read-only rules for older builds, and forward
  repair.

## 12. Risks

These are the v2.0 risks R-01 to R-20 and R-S01 to R-S12, with additions:

| ID | Risk | Severity | Mitigation |
| --- | --- | --- | --- |
| R-N01 | An old build writes the legacy chart of a normalized schedule | Critical | Format gate ships first (D-11); old charts become read-only projections |
| R-N02 | A stray library print corrupts the stdout frame stream | High | Capture stdout before loading; fuzzed framing; strict decoding |
| R-N03 | Staging and `projects/` are on different volumes, so the rename is not atomic | High | Both under `DataDir`; check that the device IDs are equal; refuse otherwise |
| R-N04 | Notarization rejects native code in sqlite-jdbc | Medium | Exclude it from v1, or sign the extracted libraries |
| R-N05 | JVM start-up per job is slow on low-end machines | Medium | Measured in G0; session-scoped fallback (D-5) |
| R-N06 | Reconciliation deletes user data | High | Never delete a folder without a journal; keep interrupted imports in quarantine for 30 days and tell the user |
| R-N09 | Publication replaces an existing folder | High | No-replace rename on every OS (§6.4); no fallback to `os.Rename` |
| R-N07 | New screens are not translatable | Medium | ADR-008 ratchet in place before G2 interface work |
| R-N08 | SQLite 3.33.0 blocks a needed feature | Medium | Design within it, or upgrade SQLCipher under the dependency rules |

Retained and still critical:

- R-01: treating a separate process as a sandbox;
- R-02: a stale session committing;
- R-03: claiming a full import based only on charts;
- R-04: exceeding the WBS caps;
- R-05: currency and date corruption;
- R-S02: compacting the audit chain in a way that changes its hashes.

## 13. Brief for coding agents

This condenses [v2 §18], with its numbering fixed.

1. **Start-up.** Run `pwd`, `git status --short`, `git log -1`, and read
   `AGENTS.md`. Protect unrelated work; never reset, stash blindly, or stage
   broadly.
2. **Compare against this plan.** Check HEAD against §7.1 and the ADR's
   evidence table, and report any difference before writing anything.
3. **Do the smallest next slice** in §10.4 that the owner has approved. Write
   its mini-plan: files, tests, and rollback.
4. **Verify external facts** (versions, APIs, licences) from primary sources.
   Never guess package names, flags, or field names.
5. **Respect boundaries.** For anything touching schema, encryption,
   accounts, WBS, audit, or signing, cite the accepted ADR and stop if a
   decision is missing.
6. **Follow the work cycle:**
   1. inspect;
   2. state the invariants and negative cases;
   3. write tests that fail;
   4. make the smallest change;
   5. format, then run targeted, race, and fuzz tests;
   6. run the broader gates;
   7. review the diff adversarially;
   8. stage by path;
   9. record the evidence.
7. **Forbidden:**
   - database or key access from a worker;
   - building shell commands from manifest values;
   - claiming a permission is enforced when the OS does not enforce it;
   - unbounded processes or listeners;
   - promising `.mpp` export;
   - guessing field mappings;
   - silently converting precision;
   - recalculating during import;
   - success messages before read-back;
   - bypassing epoch, revision, or audit checks;
   - automatic AI approval;
   - a marketplace;
   - migrations hidden inside importers;
   - claiming PASS for skipped fixtures.

## 14. Documentation plan

| Document | When | Notes |
| --- | --- | --- |
| ADR-010 (decisions) | Now | Accepted only by the owner |
| This plan | Now; updated at each gate | Status of each phase recorded here |
| Test strategy | Now; evidence added per gate | `docs/testing/adr-010-test-strategy.md` |
| `docs/interoperability/mpxj-field-mapping.md` | Before G3 | Generated from the API and fixtures |
| ADR-011 storage | S1 | Includes the format gate |
| AI and MCP ADR | Before G5 | Grants and transport |
| `docs/user-guide.md` and in-app Help | Each user-visible phase | Supported formats and limits, translated through ADR-008 |
| `docs/DEPENDENCIES.md` | G0 and G2 | Java runtime, MPXJ, protobuf, licences |
| `docs/TEST_COVERAGE_LEDGER.md` | Each code PR | New test files and headings |
