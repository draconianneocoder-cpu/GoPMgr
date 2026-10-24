<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 v2.2 — Detailed engineering blueprint and phased implementation plan

**Status:** ACCEPTED target architecture (maintainer, 2026-10-24; [decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)). Only P00, P01/G0 and P02/S0 are authorized (§8.2); no production code or schema change is authorized. **Companion:** [binding ADR](ADR-010-plugin-architecture.md), [test strategy](../testing/adr-010-test-strategy.md), [agent prompt](../agent/ADR-010-architecture-preservation-agent-prompt.md).\
**Repository baseline:** `main@28a758d` and agent's `claude/adr-010-plugin-architecture@49ac2bf` as of 2026-10-24. Rebase, re-census, and resolve conflicts before coding.\
**Plan owners:** maintainer approves contracts; implementation agents propose narrowly scoped PRs with independent verification.

## 0. Engineering precedence and completion definitions

This plan operationalizes **QI-01…QI-15** and **D-01…D-19** from the companion ADR. All hard requirements are traceable in the test strategy. On conflict, do not choose a simpler solution silently: halt the affected work package, document the conflict and raise an Architecture Change Request (ACR). Follow repository `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/DEPENDENCIES.md`, `docs/VISION.md`, `docs/ROADMAP.md`, ADR-001, ADR-004/005, ADR-008/009, the unified WBS contract, and release gates. Existing source behavior is evidence, not permission to delete requirements.

**Explicit scope boundaries:** G0/G1/G2 establish *inspection* capability and IPC; G3 imports complete validated new projects; G4 packages/export-tests; S1–S7 make >10,000 editable entities and analytics real; G5–G7 are separately accepted extension capabilities. No phase is called finished merely because its document or fake-worker tests exist.

## 1. High-level architecture and boundaries

```text
+--------------------------------------------------------------------------+
| Wails v2 / Svelte 5: Import Assistant, Plugin Center, Job Center,        |
| Fidelity Inspector, virtualized Outline + Gantt, future AI workspace     |
+--------------------------------------+-----------------------------------+
                                       | typed, bounded Wails methods
+--------------------------------------v-----------------------------------+
| GoPMgr HOST (authoritative)                                               |
| Account/session/epoch + project copy/handle + permission checks           |
|  internal/interop/registry    internal/interop/supervisor                 |
|  internal/interop/ipc         internal/interop/job                        |
|  internal/interop/contract    internal/interop/importjob                  |
|  internal/interop/evidence   internal/interop/publish                    |
|  internal/interop/fidelity   internal/interop/mapping                    |
|  project services / SQLCipher / scheduler / exact money / audit           |
+--------------------+----------------------+------------------------------+
                     | gRPC/HTTP2            | authorized SQL transactions
                     | UDS / named pipe      v
+--------------------v---------+   +--------------------------------------+
| Bundled MPXJ Java Worker     |   | Encrypted staging project folder     |
| Reads selected source,       |   | DB + receipts + source evidence      |
| emits typed entities,        |   | (project-owned; no published partial) |
| no .gopmgr / key / host RPC   |   +------------------+-------------------+
+------------------------------+                      | verified atomic no-replace
                                                     v
                                        projects/<folder>/project.gopmgr
                                        (normal folder scan, no catalog)

Future bounded workers: Rust analytics / other adapters; AI/MCP is a
separate controlled gateway, NOT the underlying plugin transport.
```

### 1.1 Owner-by-domain table

| Domain | Authority | Worker-visible form | Additional rule |
|---|---|---|---|
| Accounts/DEK/session | Host `internal/users`, `session_state.go` | None | Epoch and incarnation checked again at publish |
| SQLCipher/schema/format version | Host `internal/db`, ADR-011 | Typed records only | No DB handle, password, raw SQL or project path |
| Source identity | Host `interop/mapping` | Source-native UIDs in emitted records | Never use task name/path as identity |
| Source evidence | Host encrypted project evidence store | Worker may read selected source stream | Source-retention choice explicit |
| Scheduling/EVM/money | Existing Go kernels | Source factual dates/numbers only | No re-scheduling during import |
| Scope/WBS | WBS v2, eventual ADR-011 | Source outline separate | Import must not bypass WBS limits |
| Audit | Host append-only chain | Sanitized structured diagnostics only | Existing hashes invariant |
| UI and localization | Wails/Svelte + ADR-008 catalog | No raw worker message to UI | No English string matching |
| AI | Future permissioned host services | Read-only MCP initially | Model output untrusted; user-controlled proposals |

### 1.2 Concrete proposed paths, subject to repo naming review

```text
internal/interop/
  contract/           # generated Go code and descriptor set (sources: proto/)
  registry/           # capability registry, manifests and compatibility
  supervisor/         # host process tree + start/stop resource policies
  ipc/                # transport-neutral RPC client/server adapter
    unix/             # UDS for linux/darwin
    windows/          # named pipes, pending proof
  job/                # lifecycle, cancellation, phase reporting
  importjob/          # parsing orchestration and bounded staging
  mapping/            # identities, conversions, record mappings
  fidelity/           # field-ledger and format evidence
  evidence/           # encrypted source/field report and retention
  publish/            # intent journal and crash-safe publication
workers/mpxj-bridge/  # Java project and verified native grpc transports
proto/gopmgr/interop/v1/ # authoritative .proto sources, no ad hoc copies
frontend/src/...       # follow existing Svelte naming and component conventions

# Future, only with separately approved ADRs:
internal/schedule/     # normalized schedule kernel/storage adapters
internal/ai/           # model providers / MCP authorization gateway
```

Do **not** create a monolithic `plugin.go`, global service locator, universal JSON serializer, or direct Wails bindings for every worker method. The host's existing project domain APIs remain the only access route.

## 2. G0: platform gRPC IPC feasibility — first mandatory gate

**Goal:** select an actual Go↔Java **gRPC/HTTP2 over local OS IPC** implementation on *every advertised platform* before finalizing the worker launch protocol or shipping it. Cross-language interoperability is a **technical test**, not a literature-based assertion. G0 is intentionally disposable: no application data or product migrations.

### 2.1 Candidate transports and test matrix

| Platform | Primary experiment | Secondary experiment | Cannot assume |
|---|---|---|---|
| Linux x86_64/arm64 where supported | Go `net.Listen("unix",...)` ↔ grpc-java Netty epoll domain socket | Java client or server orientation, same contract | Java grpc-java defaults support UDS without native setup |
| macOS arm64 and amd64 where supported | Go UDS ↔ grpc-java Netty kqueue domain socket | Alternative native Java transport packaging | Working on one architecture proves the other |
| Windows amd64/arm64 where supported | Go `go-winio` restricted named pipe ↔ **real Java grpc HTTP/2** named-pipe transport | Windows AF_UNIX in selected JDK + Java gRPC transport | Go `go-winio` or .NET docs prove Java named-pipe support |

The Windows named-pipe Java binding must use a maintained transport API where possible. If custom Netty Channel / JNA / JNI or a carefully isolated Windows transport shim is necessary, profile maintenance, ABI, code signing, security and testing cost. Reject speculative “it should work”. A Go↔Java gRPC handshake, unary RPC, server/client/bidirectional streaming, deadlines, cancellation and authentication must actually run on the configured Windows CI/test machine. Native AF_UNIX availability at OS/JDK level **does not establish grpc-java integration**. Document supported minimum Windows build and JDK.

**Verified leads for G0 (read from upstream source on 2026-10-24; hypotheses, not proofs):**

- grpc-java's built-in `unix:` channel provider (`UdsNettyChannelProvider`) is available only when Netty epoll loads, so it covers Linux alone. macOS needs an explicitly configured `KQueueDomainSocketChannel`.
- Netty 4.1 and 4.2 ship `NioDomainSocketChannel`, which uses the JDK's own Unix-domain sockets (JEP 380, Java 16+) without JNI. `NettyChannelBuilder.channelType(channelType, transportSocketType)` accepts a non-IP address type. Together these are a candidate JNI-free transport on Linux, macOS and Windows 10/Server 2019 or later, and Go's `net` package already listens on `unix` sockets on Windows. G0 must prove it end to end (TPT-001, TPT-002, TPT-004) before anyone relies on it.
- Socket paths are limited to about 104 bytes on macOS and 108 on Linux, so endpoints live in a short private runtime directory, never under the data root.

**No automatic fallback:** If native IPC cannot be demonstrated for Windows, mark G0 Windows **BLOCKED**. Offer owner a separate ACR evaluating (a) funded named-pipe adapter, (b) tested Windows UDS variant, (c) explicit, authenticated localhost gRPC exception, or (d) deferring *Windows worker support* while keeping core GoPMgr Windows functionality. The agent cannot substitute stdio or TCP independently.

### 2.2 Orientation and bootstrap

- **Recommended orientation:** Go host creates restricted IPC listener; worker connects as gRPC client and opens the authenticated bidirectional `WorkerSession.Connect` stream (§3.2). Host is service authority and job controller. If a real Java transport requires reverse orientation, document the security and lifecycle implications; keep semantics unchanged.
- Bootstrap includes worker ID, protocol range, endpoint descriptor, operation ID and an unpredictable single-use credential. Transmit secret via a reviewed inherited one-time descriptor or equivalent protected OS mechanism; not command line, persistent env, manifest, logs, crash reports or Java system properties. INT-02 approves a parent-owned, private, one-shot **stdin pipe** as the G0 candidate for the credential and bounded endpoint metadata; never business data. It must be versioned and length-bounded, expire, resist replay, and close after one read; G0 proves it on each OS (TPT-022).
- Connection may serve only one authenticated launch; reject second clients, replay, downgrade, wrong user/copy/worker digest. Use OS credentials/ACLs and protocol-level proof. UDS socket owner/mode and Windows pipe ACLs are enforced, with symlink/squat/restart adversarial tests.
- Host can cancel active RPC, escalate to kill process tree, reap and clean IPC artifact, with bounded deadlines. A stale worker cannot publish anything because it has no authoritative write API.
- Demonstrate transport across packaged signed builds; the Java Netty native dependencies and `jlink` image must run offline.

### 2.3 Required G0 metrics and artifacts

Create `docs/testing/adr-010-g0-evidence.md`, `docs/interoperability/transport-matrix.md`, a pinned **experimental** Go/Java project and a source-linked dependency/SBOM/licensing report. Capture platform and arch, Java/JDK version, grpc-go/grpc-java/Netty versions, native library names/hashes, codegen version, latency (cold/warm), bandwidth, memory, CPU, startup/termination, cancellation p95, access-denied behavior and packaged signing. Run five coarse startup samples; for a statistical p95 of interactive requests, run substantially more, documenting sample count and variance. An unrun target is BLOCKED, not PASS.

## 3. G1: versioned RPC, capabilities, process supervision

### 3.1 Minimal general registry schema

Worker manifest (a **proposed schema**, not currently supported YAML):

```yaml
api_version: gopmgr.plugin/v1
id: org.gopmgr.mpxj
name_key: plugin.mpxj.title
vendor: GoPMgr / MPXJ upstream
version: 0.1.0
protocol:
  min_major: 1
  max_major: 1
  min_minor: 0
capabilities:
  - id: schedule.inspect
    formats: [mpp, mspdi, xer, pmxml, pod]
    direction: read
  - id: schedule.import
    formats: [mpp, mspdi, xer, pmxml, pod]
    direction: read
runtime:
  kind: subprocess
  executable_id: java-runtime
  platform_ipc: {linux: grpc+uds, darwin: grpc+uds, windows: grpc+npipe}
permissions:
  selected_source_only: true
  host_database: false
  host_write_api: false
  network: not_enforced_for_bundled_worker
```

Advertised formats **must be filtered by tested version family and direction**, not blanket declared for all releases. Manifest trusted digests/permissions are embedded or verified against the signed host release, not read from a mutable self-authorizing plugin directory. Reject unknown mandatory manifest schema or protocol major; negotiate optional capability/minor versions. Use language-neutral IDs and typed error codes.

### 3.2 Proposed protobuf RPC shape

**Illustrative IDL — must be made compilable, reviewed, and generated before G1 implementation.** Never ship placeholder `bytes arbitrary_project` or protobuf `Any` for authoritative records.

```proto
syntax = "proto3";
package gopmgr.interop.v1;
option go_package = "gopmgr/internal/interop/contract/interopv1";

service WorkerSession {
  rpc Connect(stream WorkerEvent) returns (stream HostCommand);
}

message WorkerEvent {
  string job_id = 1;
  uint64 sequence = 2;
  oneof event {
    WorkerHello hello = 10;
    HealthResponse health = 11;
    ProjectHeader project = 12;
    TaskRecord task = 13;
    RelationshipRecord relationship = 14;
    ResourceRecord resource = 15;
    AssignmentRecord assignment = 16;
    CalendarRecord calendar = 17;
    TimephasedRecord timephased = 18;
    CustomFieldRecord custom_field = 19;
    FidelityNote fidelity_note = 20;
    JobProgress progress = 21;
    JobCompleted completed = 22;
    JobFailed failed = 23;
  }
}
message HostCommand {
  string job_id = 1;
  uint64 sequence = 2;
  oneof command {
    HostHello hello = 10;
    SourceBlock source = 11;
    ExecuteCommand execute = 12;
    CancelCommand cancel = 13;
  }
}
message WorkerHello { string worker_id = 1; uint32 protocol_major = 2; uint32 protocol_minor = 3; bytes auth_proof = 4; }
message HostHello { bytes challenge = 1; string operation_id = 2; }
message HealthResponse { string state = 1; }
message ProjectHeader { string source_id = 1; string producer = 2; string format = 3; }
message TaskRecord { string uid = 1; string parent_uid = 2; string name = 3; }
message RelationshipRecord { string predecessor_uid = 1; string successor_uid = 2; }
message ResourceRecord { string uid = 1; string name = 2; }
message AssignmentRecord { string uid = 1; string task_uid = 2; string resource_uid = 3; }
message CalendarRecord { string uid = 1; }
message TimephasedRecord { string owner_uid = 1; string start = 2; string end = 3; string exact_value = 4; }
message CustomFieldRecord { string owner_uid = 1; string definition_uid = 2; string encoded_value = 3; }
message FidelityNote { string source_field = 1; string outcome = 2; string reason_code = 3; }
message JobProgress { string phase = 1; uint64 processed = 2; }
message JobCompleted { string logical_digest = 1; uint64 task_count = 2; }
message JobFailed { string code = 1; }
message SourceBlock { bytes chunk = 1; bool last = 2; }
message ExecuteCommand { string capability = 1; string source_artifact_id = 2; }
message CancelCommand { string reason = 1; }
```

The **final** IDL must include every canonical record group, source type/units, calendar and timezone semantics, nullable/absent/unreadable values, raw/typed custom fields, baselines, cost fields, task constraints, predecessor lag and resource assignments, per-field fidelity dimensions, source/worker digests, negotiated limits, idempotency/correlation IDs, schema version, timestamps, and typed errors. The illustrative minimal record definitions above are intentionally **not sufficient for production import**. G1 freezes the complete required schema through separate proto review and golden fixtures; every removed field number reserved, fields never renumbered. `proto3 optional`/`oneof` used where presence matters; absent, explicit null, unsupported and zero are not conflated.

### 3.3 Flow control and state safety

- gRPC HTTP/2 transports and native stream flow control are used; impose explicit application credit/accounting so a parser can never unboundedly queue records while the host is validating/writing.
- Per-message max (candidate <= 4 MiB), per-stream max, total source uncompressed bytes, nesting, tasks, series and scratch caps. Limits set by G0/G2 measurement, not guesses.
- Worker can emit progress while receiving source blocks; prove full-duplex no-deadlock behavior with blocked receivers, huge source, stderr bursts, cancellation and process crash. Never rely on synchronous send loops that cause both sides to stop reading.
- Logical record digest must have a **canonical, versioned serialization contract** independent of wire reorderings; sort/order conventions and duplicate detection specified for every record group. Hash input should include schema version and record-type boundary to avoid ambiguity. Host recomputes independently on staging readback.
- Map gRPC status to stable GoPMgr `interop.*` codes and ADR-008 translation keys; never render arbitrary Java exception text. Establish retry policy by idempotency and phase; never retry non-idempotent publication blindly.

### 3.4 Lifecycle, integrity, package constraints

Process tree and named IPC endpoint belong to `supervisor`. Worker identity and version locked to built release provenance; host checks exact worker assets before each launch. Determine link/symlink security and macOS code-signing semantics; do not assume digest-only checks protect a mutable signed bundle. Do not depend on shell quoting. Path resolution must remain within verified worker directory. Strip dangerous inherited environment variables (`JAVA_TOOL_OPTIONS`, `_JAVA_OPTIONS`, `CLASSPATH`, `LD_PRELOAD`, dynamic loader paths, proxy configuration if needed, etc.), preserving only necessary runtime values under review. Security cannot depend solely on clearing environment. Standard out/err are treated as **potentially sensitive unstructured diagnostics**, capped and not persisted raw by default. Debug logging opt-in is explicitly scoped, redacted and time-bound.

## 4. G2: MPXJ inspect-only bridge and fidelity instrument

1. Pin candidate MPXJ **16.10.0** and a reviewed upstream release; enumerate all transitive build/runtime dependencies, native libraries and licenses. Make a `jlink` runtime with a supported LTS JDK **after** testing class/path/native module requirements; do not equate MPXJ Java 8 compilation target with JDK packaging choice.
2. Implement format detectors and real readers using **inspected MPXJ API methods**; map parser exceptions to stable codes. Do not guess API signatures or “supported versions.”
3. Parse real-world and synthetic `.mpp`, MSPDI XML, P6 `.xer`/`.pmxml`, and ProjectLibre `.pod` fixtures. Each format/version requires fixture IDs and a verified reader capability; unsupported export directions remain hidden.
4. Do not use MPXJ JSON output as a proxy for full fidelity. MPXJ's JSON writer defaults to omitting timephased data. Bridge emits typed records directly; required source fields missing from MPXJ are `unreadable` or `unknown` with accurate evidence, not fabricated.
5. Use a host-read source stream by default; seek-required formats may spool to restricted worker scratch storage **subject to user policy, caps and cleanup**. Scratch plaintext threat disclosure required; delete is not secure SSD erasure. Do not require system-wide Java installed or network download.
6. Capture independent `inspect` counts, source metadata, format/version confidence, memory/disk estimate and warning categories; use worker-declared values as untrusted until validated.
7. Inspect-only must never create or modify a `.gopmgr` project; cache/report should be ephemeral or explicitly user-consented encrypted metadata.
8. Measure JVM heap (`-Xmx`), native memory (Netty, direct buffers), process RSS, OS process tree, temp disk, parse durations; verify cancellation and anti-zip/XML bomb defenses. If a format exceeds a cap, return `RESOURCE_LIMIT` with clear partial-inspection status; never claim a complete inspection.
9. MPXJ must not open a network connection as a workflow requirement. The bundled worker may have OS privileges unless actual sandboxing is deployed; this is **not equivalent** to OS-enforced no-network.

**G2 output:** `docs/interoperability/mpxj-field-mapping.md` (source accessor, typed wire, canonical destination, unit/precision/null rules, fixture, round-trip claim); `docs/interoperability/mpxj-format-support.md` (read/write per version); `docs/interoperability/mpxj-dependency-inventory.md`, signed packaging proof and G2 test evidence.

## 5. Data model contract before G3 (ADR-011; S0–S4)

### 5.1 Logical entity families

Define typed normalized records for projects/schedule generations, source artifacts and lineages, source field observations, source hierarchy (including summaries, null/source placeholders where meaningful), work packages and activities, calendars/exceptions, constraints, logic relationships, resource definitions and calendars, resource assignments and work rates, costs, progress, baselines and snapshots, sparse timephased intervals, custom field definitions/values, notes, external relations, format-specific opaque evidence, scheduling results and provenance/receipts. A normalized schedule outline is **not automatically** the user's governance WBS tree. Existing WBS v2 retains 10,000-node / 8 MiB restrictions until its own reviewed migration.

### 5.2 Reference physical data design (not migration-ready SQL)

| Entity/table family | Hot access paths and indexes | Identity/lifecycle |
|---|---|---|
| `schedule_generations` | `(project_id, active, revision)` | Exactly one active editable generation per schedule owner |
| `schedule_activities` | `(generation_id, parent_id, ordinal, id)`, `(generation_id, source_uid)` | Stable host IDs; source UID separately scoped |
| `schedule_links` | `(generation_id, predecessor_id)`, `(generation_id, successor_id)` | Preserve dangling external references explicitly |
| `schedule_calendars`, `calendar_exceptions` | `(generation_id, calendar_id, start)` | Intrinsic schedule work time, exceptions and timezone |
| `schedule_resources`, `assignments` | `(generation_id, resource_id)` and `(generation_id, activity_id)` | IDs not names; preserve resource naming collisions |
| `schedule_baselines`, `baseline_values` | `(generation_id, baseline_id, activity_id)` | All supported source baselines, no “baseline 0 only” shortcut |
| `timephased_series`, `timephased_segments` | `(series_id, start, end)` | Native intervals, units and precision; no eager minute/day explosion |
| `custom_field_definitions`, `custom_field_values` | `(generation_id, definition_id, owner_id)` | Source type vs modeled type, versioned metadata |
| `source_artifacts`, `source_lineages`, `source_entity_keys`, `entity_link_events` | Artifact digest; lineage/key typed uniqueness | Append-only observations, confirmed/rejected matches |
| `import_receipts`, `field_fidelity_events`, `evidence_chunks` | Operation ID / field index / content digest | Encrypted, project-owned, retention-aware |
| `schedule_domain_events` or audit bridge | `(project_id, logical_command_id)` | Existing audit hash chain remains authoritative |

The real ADR-011 must supply *exact* column types, check constraints, FK/cascade/deferred behavior, numerical unit schema, deterministic sort/index design, migration scripts, feature flags, query benchmarks and `EXPLAIN QUERY PLAN`. SQLite 3.33.0 constraints remain until the approved SQLCipher upgrade; do not use `STRICT`, modern `RETURNING`, JSON1/FTS5 or `DROP COLUMN` without verifying compile/runtime support. `PRAGMA foreign_keys` changes must occur outside transactions as SQLite requires; after migration, run foreign-key and cipher integrity checks. Choose `WITHOUT ROWID` only when a specific measured access pattern supports it.

### 5.3 Format gate and legacy project support

- Introduce an authoritative min-writer/project-format gate that is **checked before every writer** and for any restore/import/repair/admin path, with explicit version negotiation and read-only rejection.
- A gate release cannot cause older *pre-gate* binaries to refuse; document unsafe historical versions, backup and compatibility workflows. No promise of unconditional binary rollback after migrations.
- Existing `.gopmgr` files remain encrypted and readable through supported migration; decryption/plaintext working copy is not an ordinary migration strategy. Rehearse recovery codes and administrator escrow.
- Normalize and activate only under one source of truth: immutable source and a single current normalized generation; charts are generated projections and existing chart mutators refuse normalized authority.
- Keep small/legacy charts and legacy MSPDI Quick Chart Import functional and explicitly labeled incomplete, without bypassing new data rules.

### 5.4 Data and audit loss controls

Every source field has the multidimensional ledger described in D-09. For fields not natively modeled, preserve safely as opaque project-owned evidence *when feasible*, noting edit/export limitations. A reader's failure to enumerate unknown fields must be stated as unknown, not “preserved”. No silent conversion to an eight-hour day, no float-based money conversion, no loss of time zone semantics, and no ID reuse masquerading as entity continuity. Existing hash chain uses canonical JSON with generic-number caveats; preserve strings for exact numbers, and test mixed event chains. Receipts must distinguish accepted, discarded, partially inspected and blocked sources; no false `success` for a truncated dataset.

## 6. G3: durable new-project import and recovery protocol

### 6.1 Project-contained sealed artifacts

```text
<data-root>/
  <account-data>/projects/<published-folder>/
    project.gopmgr                     # SQLCipher encrypted authoritative DB
    evidence/                         # only encrypted source/field evidence
      ...                              # optional, project-owned, authenticated
  <account-data>/staging/<operation-id>/
    intent.json                        # minimal non-sensitive metadata, atomic+fsynced
    candidate-project/                # EXACT entire folder to publish
      project.gopmgr
      evidence/                       # moves WITH project, never discarded
  <account-data>/quarantine/<operation-id>/
    ...                               # only operation-owned failed artifacts
```

Use the repo's **actual account directory model**, not guessed folders. The sample layout must be reconciled with `users.DefaultRootDir`, session/account ownership and `enumerateProjects`. Verified at `28a758d`: the data root is `users.DefaultRootDir()` (`$XDG_DATA_HOME/GoPMgr` when set, otherwise `~/Library/Application Support/GoPMgr` on macOS and `~/Documents/GoPMgr` on Linux and Windows); each account's `DataDir` is `<root>/<username>` (`internal/users/store.go:694`); projects are `<DataDir>/projects/<folder>/` (`app_projects.go:94`). So `<account-data>` above is `<root>/<username>`, and staging and quarantine sit beside `projects/` on the same volume. On Linux and Windows the root is under `~/Documents`, which a sync client (for example OneDrive folder redirection) may manage. INT-03 sets the policy: live SQLCipher/WAL storage is not declared safe on unvalidated sync-managed or network filesystems; existing locations are compatibility inputs and are never relocated, deleted or blocked from reading by heuristic; only new publication may warn or block, under an approved supported-location policy with relocation and backup guidance; a default-root change needs its own migration decision. S0 measures the filesystem properties (DB-021) and G3 tests the policy (PUB-026). Project-owned evidence may instead live as encrypted `evidence_chunks` *inside* `.gopmgr`; adopt an encrypted sidecar only after comparative backup/restore performance and recovery tests. If source retention is explicitly declined, the receipt records `source_bytes_retained=false` and the mapping ledger still states what is and is not recoverable. No source plaintext copied into long-lived sidecars or logs.

### 6.2 Transaction and state machine

```
NEW → INSPECTING → REVIEW_REQUIRED → APPROVED → PREPARED → BUILDING
    → VALIDATING → SEALING → SEALED → RECHECKING → PUBLISHING
    → PUBLISHED → RECONCILING → COMPLETE
                          \ → FAILED / CANCELLED / QUARANTINED
```

The host controls transitions with an operation ID and strictly monotonic **durable intent journal**. *Publish* becomes the non-cancellable, brief commit boundary, not the entire import job. Explicit events/receipts allow the UI to recover after crash. A successful worker `Complete` is necessary but not sufficient for project success.

The durable journal stores the five states named in ADR D-10; the other job states live in memory only:

| Journal state | Written when the job reaches | Meaning after a crash |
|---|---|---|
| `PREPARED` | PREPARED (kept through BUILDING and VALIDATING) | Candidate may be incomplete; never publish |
| `SEALED` | SEALED | Candidate is complete; publishing needs fresh approval |
| `PUBLISHING` | PUBLISHING, before the rename | The rename may or may not have happened; check both locations |
| `PUBLISHED` | PUBLISHED, after the rename and directory sync | Read back and verify |
| `RECONCILED` | COMPLETE | Nothing left to do; staging may be removed |

INT-01: the journal and the seal manifest are **HMAC-authenticated** over a canonical, versioned encoding (operation ID, principal incarnation, intended project identity, candidate and seal digests, evidence manifest reference, state, final target). Recovery verifies them before it may authorize publication. A valid MAC proves integrity, not freshness: a rolled-back journal still verifies, so recovery also checks the authoritative receipt marker, the physical state and the seal digests. The key must survive restart and follow GoPMgr's DEK re-wrap, recovery-code, password-reset, escrow and deletion rules, and is never stored usable in the journal. The algorithm, key derivation and anti-rollback design are reserved for a security-reviewed G3 publication contract (tests PUB-021…PUB-025).

**Step-by-step commit design:**

1. Authenticate user, capture principal incarnation/session epoch and explicit user-selected source. Hash stream *as it is read*; bind worker digest, options and target user. Choose provisional project ID and create exclusive staging operation and journal.
2. Inspect format/record counts and generate fidelity preview. Prompt user to select strict or reviewed mode, required fields, evidence retention and any source lineage mapping; produce a canonical approval digest of all material choices.
3. Open an **encrypted** staged candidate project under its authorized owner. Ingest typed records in bounded invisible transactions. Retain source and field evidence in candidate. Validate references after stream completion; do not invent missing activities or silently drop source records.
4. Check source and staged counts, logical digest, exact money/units, foreign keys, full/cipher integrity, baseline/timephased rows, readback/semantic mapping and audit chain. Write import receipt and audit event before sealing. Check every retained evidence hash against its referencing record. Fail closed or explain reviewed loss.
5. Checkpoint WAL, switch journal mode using existing verified helpers where appropriate; close DB, remove sidecars only after safe checkpoint, ensure private modes, sync DB, evidence, candidate folders and intent journal as supported. Seal manifest lists DB and evidence digests, schema version, operation ID and approval digest. Seal/state must be durable before rename.
6. Refresh authentication binding and **fresh user approval if recovering from another session**. Choose collision-resistant final folder name, verify same-volume constraints, use OS-specific **atomic no-replace** directory rename (Linux `unix.Renameat2` with `RENAME_NOREPLACE`; macOS `unix.RenamexNp` with `RENAME_EXCL`; Windows `windows.MoveFileEx` without `MOVEFILE_REPLACE_EXISTING`; all in `golang.org/x/sys` v0.49.0, already a direct dependency. Go's `os.Rename` is not acceptable: it always replaces a file, guards an existing folder on Unix only with a separate `Lstat`, and passes `MOVEFILE_REPLACE_EXISTING` on Windows). No check-then-rename. On unsupported filesystem, fail closed with actionable error.
7. Sync containing directory where supported. Record `PUBLISHED`, read back the same sealed artifact, verify count/receipt/metadata and presentation. Record `RECONCILED`, clean operation-owned staging and only then show durable success.
8. If crash occurs, reconcile on next authorized login from staged operation namespace only; classify journal/rename states deterministically, never scan old projects looking for nonexistent journals or delete user-owned folders without authenticated operation provenance.

**Atomicity caveat:** folder renames are atomic namespace operations on suitable local filesystems when supported; durability after sudden power loss is OS/filesystem-dependent. Do not claim universal zero-loss or recovery success until real target testing. Journal and publication failure handling must distinguish `source intact`, `staged sealed`, `published pending reconciliation`, `quarantined`, and `complete`.

### 6.3 Recovery decision table

| Evidence state | Durable files | Action on next authenticated session | User message |
|---|---|---|---|
| Partial staging, valid PREPARED journal | candidate may be incomplete | Quarantine only operation-owned stage; never publish | “Import did not complete; no new project was added” |
| SEALED; candidate present; no final published folder | seal/receipt valid | Reinspect integrity and ownership, require **new approval**, then publish | “Import prepared; review before adding” |
| SEALED; candidate moved; final exists | candidate absent, final digest matches, owner matches | Reconcile as already published without issuing a second project | “Project was added; final verification completed” |
| PUBLISHED; project exists | receipt and evidence verified | Complete idempotently; do not reimport | “Import completed; recovered after interruption” |
| Orphan **within operation-owned stage**, no valid journal | unknown contents | Quarantine; explain and never auto-delete valuable source | “Unfinished import files need review” |
| Existing legacy `projects/` folder without ADR-010 journal | normal existing project | **Never quarantine by this protocol** | No special warning |
| Inconsistent digest or foreign ownership | suspect evidence | Stop and quarantine only what provenance authorizes; security log with no content | “Import requires recovery review” |
| Journal missing, invalid or failing HMAC verification (INT-01) | operation-owned staging only | Quarantine for authenticated review; never authorize publication or destructive cleanup | “Unfinished import files need review” |

Quarantine deletion/retention is a separate consent/retention decision, not an arbitrary “30 days then delete” default for valuable source material. Do not retain plaintext or hidden sensitive logs for automatic support collection.

### 6.4 Existing-project merge, future S6

Do not write imported changes directly into an active schedule. Compare source revisions and the approved target generation, classify conflicts, create a shadow generation or rigorously transactional equivalent, validate changes (including graph scheduling and money), then switch generation pointer atomically under expected revision. Preserve original source/lineage and explicit decisions for every selected conflict. No automatic merge of copied projects on coincident UID; no partial commits to visible charts.

## 7. S5–S7: scale, UX, analytics and audit optimization

### 7.1 Query patterns

Host endpoints (illustrative, not current exported methods):

```
ListOutlineRows(project_handle, schedule_id, cursor, limit, sort, filter, expansion)
  -> bounded rows, revision, next_cursor, count{value,exactness}
GetTimelineViewport(project_handle, generation_id, row_window, time_window, scale)
  -> bounded bars, clipped links, density/holiday hints, revision
GetTaskDetails(project_handle, task_id, tabs)
  -> typed data for chosen tab; no whole-project materialization
ApplyTaskPatch(project_handle, expected_revision, command_id, typed_patch)
  -> new_revision, audit_id, affected_task_ids, validation messages
GetFidelityReportPage(import_id, filter, cursor) -> bounded entries
GetJobStatus(job_id) -> phase/counters/ETA confidence/cancel state
```

Use bounded read transactions, deterministic keyset ordering with stable tie-breakers, HMAC-bound opaque cursor including principal/copy/generation/revision/filter/sort/expiry, no readers held across UI calls. When revision changes, signal `revision_changed` and reanchor on the selected task, not silent duplicate/skip. Render only viewport rows/time intervals, preserve accessibility of virtualized treegrid and IME composition, test on real devices and assistive technology.

### 7.2 Storage and derived query performance

Indexes for `(generation_id, parent_id, ordinal, id)` and both relationship directions are baseline candidates. Benchmark materialized flat outline views only when recursion becomes costly; maintain invalidation by revision/generation. Use sparse typed timephased segments and query zoom-level aggregates, never explode every assignment into per-minute rows. Run `ANALYZE` after bulk import and capture plans; measure index build cost vs import speed. Optional FTS5 and SQLite JSON functions require **build/compatibility review** in current SQLCipher. Change-impact/incremental CPM is an optimization that may become authoritative only after differential tests prove exact equivalence to full recomputation on cycles, lags, calendars and constraints.

### 7.3 Analytics neutrality

Domain queries and exact-money aggregation go through Go host with enforceable account/project scope. Build paged durable dataset import/query API distinct from materialized 10,000-row `ImportTabular`. Existing `maxImportRows` stays safe until replacement; explicitly report `truncated` and `fully_ingested` separately. Evaluate DataFusion/Arrow/Iceberg as candidate analytical implementations with measured CPU, throughput, cold-start, memory, license, complexity and local/offline behavior; **neither this plugin ADR nor the agent is authorized to choose a replacement unilaterally**. Heavy risk/Monte Carlo/portfolio jobs are cancellable, ordered and audited where they change stored projections.

### 7.4 Compact audit and replay

Typed domain events should record changed fields and/or immutable evidence hashes, not entire 50,000-row chart objects. Preserve canonical hash-chain logic for old events and validate mixed-chain verifier. Follow checkpoints with a bounded replay plan, protected evidence and explicit retention/undo scope. Do not confuse audit with undo; recomputation is independent of source fidelity. Quantify per-edit CPU, bytes written, WAL growth, event size, forensic reconstruction and backup/restore performance.

### 7.5 Usability and localization

| Milestone | Required user-visible workflows | Accessibility / privacy |
|---|---|---|
| G2 | Choose source → Inspect capability and format counts → Estimate costs; Plugin Center shows the bundled worker's status and tested read formats (D-15) | Keyboard, screen reader, source inspection disclosure |
| G3 | Map essential fields → Preview loss and source identities → Review retention → Import → Receipt | No fake 100% progress, explain blocked options, recovery states |
| G4 | Import/export direction labels, Plugin Center export directions, run diagnostics, signed bundle status | No claim of sandboxing or MPP writing |
| S5 | Large schedule tree, Gantt viewport, search, task navigation, saved view, minimap | Virtual treegrid focus/ARIA/IME, keyboard alternate to canvas |
| S6 | Compare two/three-way revisions, select conflict changes, impact preview, confirm | Counts of selected hidden rows, revision conflicts visible |
| G5/G6 | AI permissions, provider opt-in, proposals, audit | No background network egress or auto-approval |

All text in ADR-008 catalog; typed error `code` → locale key; never select behavior by comparing English strings. Preserve dense professional desktop workflows; avoid intrusive dialogs for safe, reversible navigation. Interface components are specified in [Appendix A](#appendix-a-interface-components) and the catalog copy, including every error code, in [Appendix B](#appendix-b-interface-copy-and-catalog-keys).

## 8. Delivery sequencing and PR breakdown

### 8.1 Dependency graph (binding)

```text
D0 docs + review
  ├── G0 transport/Java/legal/platform feasibility (no product behavior)
  └── S0 repository/limits/hardware/performance census
          └── S1 ADR-011 (schema/format gate/authority/migration)
                  └── S2 gate release -> additive normalized schema
                          └── S3 host transactional query/command staging
                                  └── S4 identity/fidelity/mapping/readback
G0 -> G1 grpc worker contract+registry -> G2 MPXJ inspect-only
S2 + S3 + S4 + G2 -> G3 durable new-project publication
S3 -> S5 scalable UI; G3 + S5 -> G4 packaged import/export readiness
S3/S4/S5 -> S6 compare / merge
S3 -> S7A analytics; S3 + audit design -> S7B audit modernization
G4 + separate AI ADR -> G5 then G6
Separate third-party ADR + OS sandbox proof -> G7
```

### 8.2 Incremental PR slices (do not collapse into a mega-PR)

| PR | Deliverable | Cannot include |
|---|---|---|
| P00 | v2.2 ADR/plan/test strategy/agent prompt; owner decisions | Production code |
| P01 | G0 native Go↔Java gRPC IPC PoC + signed packaging notes | Host project reads/writes |
| P02 | S0 exact source census, representative fixtures/hardware budgets | Hidden schema change |
| P03 | S1 ADR-011, SQLCipher compatibility and normalized DDL review | Migration code |
| P04 | G1 `.proto`, generated clients, fake worker, registry, supervisor, ACL/auth tests | MPXJ parsing; project writes |
| P05 | G2 inspect-only MPXJ adapter, real fixture matrix, sanitized diagnostics | Publication, merge |
| P06 | S2 gate release with every writer guarded | Normalized schema activation |
| P07 | S2 additive encrypted storage + schema upgrade tests | Unreviewed import activation |
| P08 | S3 staging/transactional record and query APIs | Direct UI edits to incomplete schema |
| P09 | S4 identity/fidelity/mapping, evidence and read-back; protobuf completeness | Publication |
| P10 | G3 durable project-contained evidence, journal, no-replace rename, cross-OS recovery | Import into existing project |
| P11 | G3 guided import assistant, field inspector, job center | Large-project UI giant JSON |
| P12 | S5 virtualized outline/Gantt and scoped edits | WBS aggregate cap changes |
| P13 | G4 MSPDI then validated format exports and signed packages | Native MPP writing claims |
| P14 | S6 read-only project comparison | Merge publication |
| P15 | S6 selectable merge with shadow generation and audited writes | Auto merge or AI approval |
| P16 | S7A engine-neutral domain/portfolio analytics and paged datasets | Unapproved mandatory DuckDB/DataFusion decision |
| P17 | S7B compact audit, checkpoints, replay and export | Rewrite old audit records |
| P18+ | G5/G6/G7 under their own approved ADRs | Unscoped AI/third-party code |

**Authorization (2026-10-24):** P00, P01 and P02 are authorized as separate small PRs (decision record §5.1). P03 onward needs its own accepted work slice; P01 code must not become a production worker, and P02 must not migrate any project.

Each PR states exact HEAD, changed files, invariant IDs, new tests, raw pass/blocked results, source/fixture hashes, performance change, threat/law/license change, migration/rollback and scope exclusions. Stop if old/source behavior differs materially from baseline.

### 8.3 Go/no-go gates

- **G0**: viable Go↔Java gRPC local transport per target; if Windows impossible, no Windows worker claim. Security bootstrap and packet/stream test works. Native build signing planned/verified.
- **G1**: fake worker/registry and all lifecycle/auth tests; no regressions worker-disabled; all supported platforms.
- **G2**: per-format/version read-only fixtures, mapped fields, parser cap, no DB writes, verified packaging.
- **S1/S2**: accepted storage ADR, full writer format-gate, immutable rollback reference fixtures, supported-version matrix.
- **S3/S4**: full transactional schema, typed readback, identity/fidelity, source evidence and auditors, no limits bypass.
- **G3**: complete sealed project + evidence + crash fault matrix, no partial results, signed-in authorization, legacy unaffected.
- **S5/G4**: 50k real-user navigation/readback, viewport/render memory, validated export direction, full release checks.
- **S6**: conflict-deterministic compare/merge, explicit approved changes, shadow generation atomicity.
- **S7A/S7B**: semantic analytics parity, audit chain/replay proofs and measured performance.
- **G5/G6/G7**: separate accepted ADRs and release tests; do not consider implemented from placeholder future methods.

## 9. Release packaging and rollback

- Worker belongs in platform app package, pinned by embedded digest inventory and code-signing/notarization. Installers remain valid and use existing release pipelines. Any optional runtime/JNI/native helpers require architecture-specific signatures and SBOM.
- Keep worker off by default until G1/G2 pass; enable inspect-only then import-publication separately, with feature flags that do not weaken DB version checks.
- Reject unknown/newer project format **before write** across CLI, Wails, repair, admin, archive and migration; pre-gate application risk documented.
- If new schema is present, rolling back the binary may be unsafe; recovery uses approved backups or a compatible previous binary only. No fictitious reversible migration claim.
- On critical incident (key leakage, cross-user leak, unsigned worker execution, partial publication, lineage corruption, bad audit hash, unbounded parsing, severe accessibility regression), stop rollout; disable worker and preserve evidence, not user project content in general logs.
- Installer support includes macOS codesigning/notarization, Windows signing/ACL, Linux packaging and offline runtime launch. Run actual package-install smoke tests, not only build unit tests.

## 10. Owner decision register (must remain visible)

| ID | Item | Proposed default | State |
|---|---|---|---|
| OWN-01 | Accept ADR-010 v2.2 D-01…D-19 | Yes, subject to owner review | APPROVED — target design |
| OWN-02 | Native gRPC IPC policy: UDS Unix / named pipes Windows | Bind as design, require G0 proof | APPROVED — transport policy; G0 proof still required |
| OWN-03 | Explicit TCP fallback policy | No fallback without ACR and approval | APPROVED — no implicit fallback |
| OWN-04 | Internal registry from first implementation | Yes, bundled-only initially | APPROVED |
| OWN-05 | Project-contained encrypted evidence | Yes; measure DB vs encrypted sidecars | APPROVED — keeping the original file is the recommended default, with opt-out |
| OWN-06 | Fresh approval for SEALED after new sign-in | Yes | APPROVED |
| OWN-07 | New project only first release; S6 tracked separately | Yes, program scope preserved | APPROVED — phased delivery |
| OWN-08 | Existing file version gate and prior-version disclosure | Yes, ADR-011 required | APPROVED |
| OWN-09 | MPXJ LTS/JDK/Notarization/license/replacement policy | G0 investigation + legal review | APPROVED FOR G0 INVESTIGATION; final choices reserved |
| OWN-10 | Minimum/recommended hardware and latency/throughput budgets | Decide after S0 measurements | APPROVED — goal and method; budgets reserved for after S0 |
| OWN-11 | New Go dependencies: `google.golang.org/grpc`, `google.golang.org/protobuf` and, for Windows named pipes, `github.com/Microsoft/go-winio` (none is in `go.mod` today) | Approve at G1 with a `docs/DEPENDENCIES.md` record, after G0 picks the transport | APPROVED FOR P01 EXPERIMENTS; G1 production dependencies not yet authorized |

Document owner decisions in an accepted ADR and test strategy. An agent must not mark `OPEN` as `APPROVED` through interpretation, code edits or “the test passes.” The maintainer approved all eleven on 2026-10-24; the State column copies the dispositions in the [decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md) §2, which governs their conditions.

## 11. Primary references and exact-file evidence

Existing files to check on HEAD: `go.mod`, `main.go`, `app_projects.go`, `app_charts.go`, `internal/export/mspdi_import.go`, `internal/analytics/duckdb.go`, `internal/db/charts.go`, `internal/db/audit.go`, `internal/db/wbs_store.go`, `internal/db/sqlite.go`, `internal/db/encryption.go`, `internal/wbs/model.go`, `session_state.go`, `docs/design/wbs-unified-model-contract.md`, `docs/DEPENDENCIES.md`, `docs/ARCHITECTURE.md`, `docs/TESTING.md`, `.github/workflows/{ci,release}.yml`, and `AGENTS.md`. Every relevant claim is retested on the PR HEAD.

External primary sources and caveats:

- https://github.com/joniles/mpxj ; https://github.com/joniles/mpxj/releases
- https://github.com/grpc/grpc-java ; Windows named-pipe feature request: https://github.com/grpc/grpc-java/issues/12027 (closed without published production Java pipe adapter)
- https://github.com/microsoft/go-winio (Go Windows pipes, **not** Java proof)
- https://openjdk.org/jeps/380 (Java AF_UNIX API, **not** grpc-java support proof)
- https://www.sqlite.org/wal.html ; https://www.sqlite.org/foreignkeys.html ; https://www.sqlite.org/pragma.html

## Appendix A. Interface components

These components extend GoPMgr's existing Svelte primitives (`Button` variants, `Card`, `Tabs`, `Input`, `Select`, `ConfirmDialog`, `ToastContainer`, `Spinner`) in the slate and cyan palette with IBM Plex. None adds a UI dependency. Every component has loading, empty, error, disabled and read-only states, and all of its text comes from the catalog (Appendix B).

| Component | Built from | Used by | Accessibility |
|---|---|---|---|
| `Stepper` | `Tabs` styling, `Button` | Import assistant: Choose file, Inspect, Map and check, Review, Import | `nav` with `aria-label`; `aria-current="step"`; a blocked stage says why |
| `DataGrid` | New; native table semantics | Fidelity inspector, job history, Plugin Center formats | `role="grid"`, roving tabindex, sortable headers with `aria-sort`, `aria-rowcount`; shows loaded and total rows |
| `TreeGrid` | `DataGrid` plus an outline model | S5 large-schedule outline | `role="treegrid"`, `aria-level`, `aria-expanded`, `aria-rowindex` and `aria-rowcount` while virtualized; focus anchored across paging (UXQ-007) |
| `FidelityTags` | `StatusBadge` | One inspector cell per field: the D-09 dimensions | Each dimension is a text tag; a screen reader hears, for example, "In the file: present. Kept: exactly. Stored: as a GoPMgr field. Editable. Exportable." |
| `StatusBadge` | Existing badge classes | Worker, job, recovery and disposition states | Text label always; colour only adds meaning |
| `PhaseProgress` | `Spinner` and a list | Job center | `aria-live="polite"` for phase changes only; shows counters, never a percentage without a measured total |
| `ResourceEstimate` | `Card` | Inspect stage | Values labelled as estimates; numbers and units through `formatNumber` |
| `RetentionChoice` | Radio group with `Input` styles | Map and check stage (OWN-05) | `fieldset` and `legend`; each option's consequence linked with `aria-describedby` |

**Status colours**, mapped to classes the frontend already uses:

| Meaning | Colour | Example classes |
|---|---|---|
| Kept exactly, Ready, Done | emerald | `text-emerald-300`, `bg-emerald-950/40` |
| Converted, Running | cyan | `text-cyan-300` |
| Changed in conversion, Needs attention, Finishing | amber | `text-amber-300`, `border-amber-700/50` |
| Not stored, Off, Cancelled | slate | `text-slate-400` |
| Failed, Can't start, Files don't match | red | `text-red-300`, `bg-red-950/40` |

## Appendix B. Interface copy and catalog keys

Keys follow `frontend/src/lib/i18n/messages/en.ts`: flat dotted keys, `{placeholders}` and `{one, other}` plural forms. Errors say what happened, why, and what to do. Product names (Microsoft Project, Primavera P6, ProjectLibre) and file extensions stay untranslated. "MPXJ", "gRPC" and "worker" appear only in diagnostics.

### B.1 Terms

| Term | Use for | Never |
|---|---|---|
| Import a project | The full MPXJ import into a new project | "Commit", "ingest" |
| Quick chart import | The legacy MS Project XML chart importer (D-12) | Implying it imports a whole project |
| Plugin | A bundled worker, as listed on the Plugins page | "Worker", "bridge", "sandboxed" |
| Original file | The source file the user chose | "Source artifact" |
| Import report | The receipt and field report together | "Receipt", "ledger" |
| Strict, Reviewed, Inspect only | The three fidelity modes (D-09) | Other names for the same modes |

### B.2 Import assistant

| Key | English |
|---|---|
| `interop.import.title` | Import a project |
| `interop.step.choose` | Choose file |
| `interop.step.inspect` | Inspect |
| `interop.step.map` | Map and check |
| `interop.step.review` | Review |
| `interop.step.import` | Import |
| `interop.step.blocked` | Finish {step} first. |
| `interop.choose.formats` | Supported: Microsoft Project (.mpp, .xml) and Primavera P6 (.xer, .xml). The Plugins page lists every tested format. |
| `interop.inspect.nothingSaved` | Inspecting reads the file only. Nothing is saved. |
| `interop.inspect.estimate` | Estimated need: {memory} of memory, {disk} of disk space, about {time}. |
| `interop.mode.strict` | Strict: stop if any essential field can't be kept exactly. |
| `interop.mode.reviewed` | Reviewed: import what can be kept, and review every field that changes or can't be kept. |
| `interop.mode.inspectOnly` | Inspect only: show what the file holds. Nothing is saved. |
| `interop.lineage.new` | Treat this as a new project |
| `interop.lineage.match` | This is a newer copy of {project} |
| `interop.lineage.copyWarning` | This file uses the same task IDs as {project}, but its history is different, so it may be a copy. GoPMgr won't combine them automatically. |
| `interop.review.attention` | `{one: "{count} field needs your attention before you import.", other: "{count} fields need your attention before you import."}` |
| `interop.import.asNew` | Import as new project |
| `interop.import.cancel` | Cancel import |
| `interop.import.finishing` | Finishing. This can't be cancelled now. The project appears when it's ready. |
| `interop.import.done` | {project} was imported. {kept} of {total} fields were kept exactly. |

### B.3 Keeping the original file

| Key | English |
|---|---|
| `interop.retention.legend` | Keep the original file? |
| `interop.retention.keep` | Keep a copy inside the project |
| `interop.retention.keepHelp` | The copy is encrypted with the project and included in its backups. It uses about {size} of disk space. |
| `interop.retention.discard` | Don't keep a copy |
| `interop.retention.discardHelp` | GoPMgr keeps the import report but can't re-read the original file later. Keep your own copy if you might need it. |

### B.4 Fidelity inspector

| Key | English |
|---|---|
| `interop.fidelity.column.field` | Field |
| `interop.fidelity.column.records` | Records |
| `interop.fidelity.column.inFile` | In the file |
| `interop.fidelity.column.kept` | Kept |
| `interop.fidelity.column.stored` | Stored as |
| `interop.fidelity.column.edit` | Editable |
| `interop.fidelity.column.export` | Exportable |
| `interop.fidelity.presence.present` | Present |
| `interop.fidelity.presence.absent` | Not in this file |
| `interop.fidelity.presence.unreadable` | Can't be read |
| `interop.fidelity.presence.unknown` | Unknown |
| `interop.fidelity.transform.exact` | Exactly |
| `interop.fidelity.transform.lossless` | Converted without loss |
| `interop.fidelity.transform.approximate` | Changed in conversion |
| `interop.fidelity.transform.derived` | Worked out from other fields |
| `interop.fidelity.store.native` | A GoPMgr field |
| `interop.fidelity.store.typedExtension` | An extra field |
| `interop.fidelity.store.opaque` | Original data, read-only |
| `interop.fidelity.store.notStored` | Not stored |
| `interop.filter.attention` | Needs attention |
| `interop.filter.exact` | Kept exactly |
| `interop.filter.converted` | Converted |
| `interop.filter.readOnly` | Kept read-only |
| `interop.filter.notStored` | Not stored |
| `interop.filter.unknown` | Unknown |
| `interop.fidelity.empty` | No fields match this filter. Clear the filter to see every field. |
| `interop.report.save` | Save import report |
| `interop.report.privacy` | The report leaves out notes, file locations and field values unless you include them. |

### B.5 Job center

| Key | English |
|---|---|
| `interop.job.title` | Imports and exports |
| `interop.job.empty` | No imports yet. Imports you start appear here, with their reports. |
| `interop.job.queued` | Waiting to start |
| `interop.job.reading` | Reading the file |
| `interop.job.staging` | Preparing the project |
| `interop.job.review` | Waiting for your review |
| `interop.job.finishing` | Finishing |
| `interop.job.done` | Done |
| `interop.job.recovered` | Finished after an interruption |
| `interop.job.failed` | Didn't finish |
| `interop.job.cancelled` | Cancelled |
| `interop.job.counter` | `{one: "{count} task checked", other: "{count} tasks checked"}` |
| `interop.job.details` | View details |
| `interop.job.retry` | Try again |

### B.6 Plugins page

| Key | English |
|---|---|
| `interop.plugins.title` | Plugins |
| `interop.plugins.intro` | Plugins add file formats to GoPMgr. They come with GoPMgr and run on this computer. |
| `interop.plugins.reads` | Reads: {formats} |
| `interop.plugins.writes` | Writes: {formats} |
| `interop.plugins.writesNone` | Writes: none yet |
| `interop.plugins.status.ready` | Ready |
| `interop.plugins.status.off` | Off |
| `interop.plugins.status.unavailable` | Not available on this computer |
| `interop.plugins.status.mismatch` | Files don't match this release |
| `interop.plugins.notSandboxed` | Runs as a separate program that comes with GoPMgr. It isn't sandboxed, so it has the same access to your files as GoPMgr. |
| `interop.plugins.diagnostics` | Run diagnostics |

### B.7 Recovery after an interruption

One message for each row of §6.3. The last row of §6.3, an existing project with no import journal, shows nothing.

| Key | English | Button (key) |
|---|---|---|
| `interop.recovery.unfinished` | An import didn't finish. Nothing was added to your projects. | View details (`interop.job.details`) |
| `interop.recovery.ready` | An import of {project} is ready, but it wasn't added. Review it before adding it. | Review import (`interop.recovery.reviewImport`) |
| `interop.recovery.addedChecked` | {project} was added. GoPMgr finished checking it. | Open project (`interop.recovery.openProject`) |
| `interop.recovery.completed` | {project} was imported. GoPMgr finished the import after an interruption. | Open project (`interop.recovery.openProject`) |
| `interop.recovery.files` | Files from an unfinished import need your review. GoPMgr hasn't deleted them. | Review files (`interop.recovery.reviewFiles`) |
| `interop.recovery.suspect` | An import needs a recovery check before GoPMgr can use it. Nothing in your projects was changed. | View details (`interop.job.details`) |

### B.8 Quick chart import and partial datasets

| Key | English |
|---|---|
| `interop.legacy.quickChart` | Quick chart import (MS Project XML) |
| `interop.legacy.quickChartHelp` | Reads tasks and links from an MS Project XML file into one chart. It skips summary tasks and assumes an 8-hour day. To keep everything in the file, use Import a project. |
| `analytics.dataset.truncated` | Showing the first {shown} of {total} rows. The rest of the file wasn't imported. |
| `analytics.dataset.complete` | All {total} rows were imported. |

### B.9 Error codes

Every typed error code (§3.3) maps to one key. The host never shows worker or Java exception text.

| Code | Meaning | Host action | Key | English |
|---|---|---|---|---|
| `PLUGIN_NOT_INSTALLED` | Worker files are absent | Offer Quick chart import | `interop.error.notInstalled` | The plugin for Microsoft Project and Primavera files isn't installed. You can still use Quick chart import for MS Project XML files. |
| `PLUGIN_SIGNATURE_INVALID` | Digest or signature mismatch | Refuse to launch; log a security event | `interop.error.signature` | GoPMgr didn't start the plugin because its files don't match this release. Reinstall GoPMgr to restore them. |
| `PLUGIN_INCOMPATIBLE` | Protocol major or manifest mismatch | Disable the worker | `interop.error.incompatible` | This plugin doesn't work with this version of GoPMgr. Reinstall GoPMgr to get a matching version. |
| `PLUGIN_START_FAILED` | JVM, handshake or authentication failure | One bounded retry | `interop.error.startFailed` | The plugin couldn't start. Try again. If it keeps failing, run diagnostics on the Plugins page. |
| `PLUGIN_UNHEALTHY` | Crash, hang or protocol violation | Kill the process tree; commit nothing | `interop.error.unhealthy` | The plugin stopped responding, so the import stopped. Nothing was saved. Try again. |
| `TRANSPORT_UNAVAILABLE` | No proven local IPC on this OS (D-03) | Mark the worker blocked; never fall back | `interop.error.transport` | This plugin isn't available on this computer yet. GoPMgr's other features work as usual. |
| `INPUT_UNSUPPORTED` | Format or version not tested | List tested formats | `interop.error.unsupported` | GoPMgr can't import this kind of file yet. It can import: {formats}. |
| `INPUT_INVALID` | Corrupt or malformed file | Keep the original untouched | `interop.error.invalid` | This file couldn't be read. It may be damaged or not a {format} file. Open it in the program that made it, save a new copy, and try again. |
| `INPUT_PROTECTED` | Password-protected file | Never log the secret | `interop.error.protected` | This file is password-protected. Remove the password in the program that made it, then import that copy. |
| `RESOURCE_LIMIT` | A memory, disk, time or expansion cap was reached | Offer inspect-only | `interop.error.resourceLimit` | This file needs more {resource} than GoPMgr can use for an import on this computer ({needed} needed, {available} available). You can still inspect it. |
| `DATA_UNMAPPED` | An essential field can't meet Strict mode | Block publication | `interop.error.unmapped` | `{one: "{count} essential field can't be kept exactly, so Strict mode stopped the import. Switch to Reviewed to import the rest.", other: "{count} essential fields can't be kept exactly, so Strict mode stopped the import. Switch to Reviewed to import the rest."}` |
| `STALE_SESSION` | Epoch, account or project changed | Discard the staged import | `interop.error.staleSession` | You signed out or switched accounts, so the import stopped. Nothing was saved. Sign in and start again. |
| `APPROVAL_REQUIRED` | A sealed import found after a new sign-in (D-10) | Ask for fresh approval | `interop.error.approvalRequired` | You signed in again, so review this import before GoPMgr adds it. |
| `PUBLISH_UNSUPPORTED` | No no-replace rename, or a different volume (PUB-009) | Fail closed; keep staging | `interop.error.publishUnsupported` | GoPMgr can't add the project safely in the folder that holds your GoPMgr data, so nothing was added. See Help for supported locations. |
| `REVISION_CONFLICT` | Destination changed after review | Review again | `interop.error.revisionConflict` | The project changed after you reviewed these changes. Review them again before applying them. |
| `USER_CANCELLED` | The user cancelled | Clean up | `interop.status.cancelled` | Import cancelled. Nothing was saved. |
| `EXPORT_UNSUPPORTED` | `.mpp` or an untested export | Suggest MS Project XML | `interop.error.exportUnsupported` | GoPMgr can't save .mpp files. Microsoft Project opens MS Project XML (.xml) files, so export that instead. |
| `INTERNAL` | Anything else | Show a sanitized error ID | `interop.error.internal` | Something went wrong, so nothing was saved. Error ID: {id}. |

### B.10 Alternatives, rationale and localization

**Main button on the Review step:**

| Option | Copy | Tone | Best for |
|---|---|---|---|
| A (chosen) | Import as new project | Plain, outcome first | v1, and later beside "Import into {project}" for S6 |
| B | Create project | Plain | Hides that the data comes from the file |
| C | Commit | Technical | Reads as a developer term; D-15's stage name is shown to users as "Import" |

**Rationale.** Labels start with a verb and name the result. Errors reassure first ("Nothing was saved") and then give one action. Recovery copy never says a project is lost, because §6.3 never deletes user content. The Plugins page states the trust model in plain words (QI-13).

**Localization notes for translators:**

- `{resource}` is a catalog value ("memory", "disk space" or "time"), never free text.
- `{needed}`, `{available}`, `{memory}`, `{disk}` and `{size}` are numbers with units, formatted by `formatNumber`.
- `{formats}` and `{format}` are product names and extensions; don't translate them.
- `interop.review.attention`, `interop.error.unmapped` and `interop.job.counter` take plural forms.
- Avoid idioms. German and French run 30 to 40 percent longer, so buttons wrap instead of truncating.
