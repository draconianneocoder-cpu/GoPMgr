<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 v2.2 — Test strategy, quality gates and traceable acceptance matrix

**Status:** ACCEPTED test strategy (maintainer, 2026-10-24; [decision record](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)); **no tests executed or claimed PASS**. **Date:** 2026-10-24.\
**Normative references:** [ADR-010 v2.2](../design/ADR-010-plugin-architecture.md) and [implementation plan](../design/adr-010-implementation-plan.md).\
**Important:** The earlier PLG, SCALE, REVIEW and ADR10 test inventories remain in the archived v2.0 and agent v2.1 drafts. **Retain those IDs in the agent's implementation ledger; do not erase or rename them to hide work.** The tests here add/update explicit v2.2 criteria. Existing cases need a machine-readable crosswalk before implementation.

## 1. Evidence discipline

A test is `PASS` only after it was executed with matching program, OS, tooling, fixture, source SHA and required validation. `FAIL` means executed and did not meet assertion. `BLOCKED` means a prerequisite (Windows Java transport, licensed fixture, OS, owner's schema decision) is absent. `SKIPPED` requires explicit reason and does not pass a gate. `PROPOSED` and `NOT RUN` are the only valid results for this document's unexecuted examples. No synthetic fixture may substitute for all real producer-produced fixtures; no mocked IPC may establish production transport support.

For every test produce: ID; owning phase/PR; requirement ID; test name/command; expected negative cases; OS/arch and installed packaging; code commit; libraries/JDK versions; fixture source, permitted use and cryptographic digest; settings; hardware profile; result; measured metrics and raw evidence location; deviations; reviewer; timestamp. No private project data, filenames or credentials in public test logs.

## 2. Test levels and ownership

| Layer | Primary test mechanism | What it may prove | What it cannot prove alone |
|---|---|---|---|
| L0 Static | ADR/source comparison, generated contracts, license inventory | Conformance and completeness | Runtime feature support |
| L1 Unit | Go/Java, table-driven, property tests | Decisions/encoding/state transitions | Packaged OS IPC |
| L2 Integration | Actual Go host + Java worker + SQLCipher fixture | Host/workers and transaction behavior | Installed runtime and OS security |
| L3 Installed | Actual installer/package on macOS/Windows/Linux offline | Signing, native libs, startup, local IPC | All source-version fidelity |
| L4 Adversarial | Fuzz, fault injection, malicious files, endpoint races | Negative security and recovery paths | Performance under sustained load |
| L5 Performance | Controlled representative tiered workloads | Capacity and p95/p50 under measured hardware | General all-hardware guarantees |
| L6 UX | Automated frontend, keyboard, screen reader, translation/manual flows | Usability of tested interfaces | Untested languages/AT combinations |

## 3. Fixture program

Fixture manifest `testdata/interop/fixtures.json` (proposed) MUST define: fixture ID; hash; file size; source app and version; supported license/redistribution permission; privacy classification; generation steps; expected logical counts; summary vs leaf; calendars; constraints; assignments; baselines; timephased; expected precision and known unsupported fields; applicable test IDs. Never include proprietary third-party files without lawful redistribution rights. Keep real MPP from multiple supported Microsoft Project versions, MSPDI, XER, PMXML, ProjectLibre POD from an independently verified supported version, malformed/protected inputs and copy/re-import sequences. Generate synthetic T1/T2/T3 from version-pinned scripts and deterministic seeds, while retaining realistic independent fixtures. Distinguish deliberately unsupported/proprietary unknown fields from known-but-unreadable attributes.

**Workload tiers:** T0 100–500 tasks; T1 10,000; T2 50,000 (+ representative resources/relationships/fields); T3 100,000+ tasks and >=1 million timephased points. Approved minimum and recommended hardware targets must be recorded by S0. Benchmark cold and warm operation, file-system and project encryption status, cross-project concurrency, revision invalidation, process-tree memory and scratch. For p95 user interactions use at least a statistically useful repeated sample size (initial proposal >=30 independent interaction measurements over multiple sessions with documented distribution; S0 sets exact method), not 5 runs claiming precise p95.

## 4. New mandatory tests: transport and plugin system

| Test ID | Requirement | Gate | Test / expected evidence |
|---|---|---|---|
| TPT-001 | D-03 | G0 | Real grpc-go server + grpc-java client transfer health over UDS on each advertised Linux build/arch; no TCP listener |
| TPT-002 | D-03 | G0 | Same over UDS on supported macOS Intel/Apple Silicon; native Netty runtime loads in installed signed app |
| TPT-003 | D-03 | G0 | Real Java gRPC HTTP/2 client connects to Go named-pipe listener on Windows; bidirectional streamed messages and cancellation; **must not PASS based only on go-winio server** |
| TPT-004 | D-03 | G0 | Windows AF_UNIX alternative passes identical real Java client tests, or marked BLOCKED/FAIL with evidence |
| TPT-005 | D-03 | G0 | When native IPC not proven, no silent TCP or stdio fallback; UI truthful, OS worker marked blocked |
| TPT-022 | D-05 | G0 | INT-02 stdin bootstrap: one-shot, versioned and length-bounded; expires; pipe closed after one read; no project data on stdio; credential absent from argv, environment, logs and crash reports; hostile child, replay and endpoint squat rejected; run on Linux, macOS and Windows |
| TPT-006 | D-05 | G1 | Per-job endpoint is private and fresh; stale socket/pipe, symlink replacement, collision, squatting and cleanup cannot admit worker |
| TPT-007 | D-05 | G1 | Per-launch credential bootstrap not in argv/env/manifest/log; reused/stolen nonce and duplicate connection rejected |
| TPT-008 | D-05 | G1 | Linux/macOS parent directory permission, ownership and cleanup checks; no deleting another app's socket |
| TPT-009 | D-05 | G1 | Windows named-pipe SDDL, local-only and client ACL rules tested as actual second user/process |
| TPT-010 | D-03 | G1 | Golden generated protobuf compatibility N, N+1, N-1; reserved field numbers, unknown enum/presence handling |
| TPT-011 | D-03 | G1 | Prohibited untyped `Any`/opaque authoritative records rejected by schema lint; bytes allowed only for raw file chunks/labelled evidence |
| TPT-012 | D-03 | G1 | Bidi simultaneous source upload, progress, cancellation and flow control cannot deadlock; slow reader bounded |
| TPT-013 | D-06 | G1 | Multiple parallel workers obey process count/RSS/scratch budget; no orphan after cancellation or process crash |
| TPT-014 | D-06 | G1 | Handshake timeout, exit code, missing JVM, corrupted frames, missing completion and mismatched digest all fail cleanly |
| TPT-015 | D-02 | G1 | Two fake neutral worker IDs advertise different capabilities through registry; no MPXJ-specific supervisor code |
| TPT-016 | D-02 | G1 | Read vs write capability claim differs; unsupported native MPP export never advertised |
| TPT-017 | D-17 | G1 | Invalid signature/digest, symlinked/extra files, untrusted library injection and startup env injection refuse launch |
| TPT-018 | D-05 | G1 | Unauthorized process and endpoint impersonation attempts rejected before business payloads |
| TPT-019 | D-07 | G1 | Worker not labeled sandboxed; OS-enforced restrictions distinguished from absent/observed controls |
| TPT-020 | D-03 | G1 | No custom project-data stdio protocol or stealth TCP endpoint in launched worker; bootstrap one-shot only |
| TPT-021 | D-06 | G1 | Worker stdout/stderr is untrusted diagnostics: capped, sanitized, never mixed into the RPC stream and never persisted raw by default; opt-in debug logs are scoped, redacted, encrypted and time-limited (plan §3.4). A fixture whose parser prints task notes leaves no note text in any log |

## 5. Parsing, field fidelity, and provenance

| Test ID | Requirement | Gate | Test / expected evidence |
|---|---|---|---|
| FID-001 | D-09 | G2 | Inspect-only produces no project writes, audit records, staging candidate or implicit network activity |
| FID-002 | D-09 | G2 | Format/version claims grounded in actual fixture matrix; mismatched extension detected, not trusted |
| FID-003 | D-09 | G2 | Password-protected, truncated, malformed, nested and decompression-bomb inputs fail bounded without note/path/password leakage |
| FID-004 | D-09 | S4 | Source summary tasks, leaf tasks, blank/null rows, hierarchy, resource assignments and calendar rules counted separately and mapped honestly |
| FID-005 | D-09 | S4 | Strict essential exact/native/editable field policy rejects read-only opaque retention; reviewed mode highlights it explicitly |
| FID-006 | D-09 | S4 | Multidimensional ledger simultaneous `lossless/native/editable` and `preserved_opaque/read_only` combinations decode correctly |
| FID-007 | D-09 | S4 | Reader-known-unsupported versus unknowable proprietary fields distinguished, with no fabricated enumeration |
| FID-008 | D-08 | S4 | Source digest changes between versions but logical lineage remains stable only through strong evidence or user confirmation |
| FID-009 | D-08 | S4 | Copied project GUID/UIDs, UID reuse, project relocation and branch divergence never auto-merge |
| FID-010 | D-08 | S4 | Immutable mapping history: moving/re-parenting/retiring produces append-only evidence, no prior mapping overwritten |
| FID-011 | D-09 | S4 | Six-/eight-/twelve-hour and exception calendars preserve original working semantics, not fixed 8h conversion |
| FID-012 | D-09 | S4 | Time-only/date-only/instant and DST ambiguity preserved; no forced UTC midnight for date-only |
| FID-013 | D-09 | S4 | Exact currency (including JPY and 3-decimal source) cannot silently round; ADR-009 conversion explicit |
| FID-014 | D-09 | S4 | Baselines 0–10, sparse/dense timephased and typed custom fields read back or ledger reports precise limits |
| FID-015 | D-09 | S4 | External links, duplicate resource names and unique source IDs preserved, not fabricated/matched by name |
| FID-016 | D-09 | S4 | Source-computed dates/critical/float preserved separately from GoPMgr recalculated values |
| FID-017 | D-09 | S4 | Per-format/MPXJ-version machine-readable mapping matrix and readback matched to real fixtures |
| FID-018 | D-09 | S4 | Every source field group has explicit presence/transformation/representation/edit/export/validation status |
| FID-019 | D-17 | G4 | New upstream MPXJ dependency/reader version cannot be silently upgraded without fixture/license regression |
| FID-020 | D-09 | G4 | Native MPP writing rejected and MSPDI export only claims validated field parity, including timephased options |

## 6. Encrypted publication, session binding and recovery

| Test ID | Requirement | Gate | Test / expected evidence |
|---|---|---|---|
| PUB-001 | D-10 | G3 | `ListProjects` never sees staging; visible project is complete with encrypted DB, receipt and audit event |
| PUB-002 | D-10 | G3 | Retained source and field report remain decryptable within published project package after successful cleanup |
| PUB-003 | D-10 | G3 | Whole-folder backup/restore retains source, ledger, provenance and audit; no broken evidence reference |
| PUB-004 | D-10 | G3 | Fresh login and seal-only staging require fresh publication approval; expired epoch receipt cannot authorize publish |
| PUB-005 | D-10 | G3 | Account switch, sign-out, project switch, delete/recreate user, administrator escrow and key rotation cannot publish wrong account's project |
| PUB-006 | D-10 | G3 | Orphan reconciliation inspects only authorized staging namespace; legacy project folders with no import journal remain untouched |
| PUB-007 | D-10 | G3 | HMAC-authenticated journal (INT-01) and physical state after `PUBLISHING` crash reconcile once; destination never duplicated |
| PUB-008 | D-10 | G3 | No-replace race with an existing empty/nonempty folder never overwrites on each supported OS |
| PUB-009 | D-10 | G3 | Cross-volume filesystem or missing atomic primitive stops publication; no unsafe `os.Rename` fallback |
| PUB-010 | D-10 | G3 | `SQLITE_FULL`, `EIO`, WAL checkpoint failure, sidecar persistence and interrupted fsync prevent partial visible project |
| PUB-011 | D-10 | G3 | Corrupt manifest/retained evidence/audit/count mismatch blocks successful completion |
| PUB-012 | D-10 | G3 | Successful import does not require any post-publish SQL write; read-only open passes immediately |
| PUB-013 | D-10 | G3 | Crash before receipt returned still recovers the receipt without duplicate import |
| PUB-014 | D-10 | G3 | On cancel during staging candidate absent from project list; after publish boundary user sees correct noncancelable state |
| PUB-015 | D-10 | G3 | Quarantine is account-owned, encrypted where appropriate; no automatic deletion of retained user content |
| PUB-016 | D-10 | G3 | Wrong DEK, expired recovery code, changed account, malformed project and legacy `.pmforge` remain safe |
| PUB-017 | D-10 | G3 | Source retention opt-out does not claim raw source recoverable; report still accurately preserves defined receipt metadata |
| PUB-018 | D-10 | G3 | Existing CreateProject and RestoreProjectArchive folders are not considered interop orphans |
| PUB-019 | D-10 | G3 | Integrity, cipher where available, foreign key, audit hash, exact count and source checksums verified before seal |
| PUB-020 | D-10 | G3 | On Windows/macOS/Linux actual crash/restart boundary tests; power-loss limitations explicitly documented |
| PUB-021 | D-10 | G3 | INT-01 tamper: any changed field in the journal or seal manifest fails HMAC verification; no publication is authorized and nothing is destroyed; operation-owned staging is quarantined for authenticated review |
| PUB-022 | D-10 | G3 | INT-01 rollback: an older, validly authenticated journal restored over newer state is detected or handled safely through the receipt marker, physical state and seal digests; a valid MAC is never treated as freshness |
| PUB-023 | D-10 | G3 | INT-01 key lifecycle: after a password change (DEK re-wrap), recovery-code reset and escrow rotation, the owner's legitimate recovery still verifies; another user cannot verify or publish |
| PUB-024 | D-10 | G3 | INT-01 replay: re-presenting a journal, approval or seal for an operation already published never publishes twice |
| PUB-025 | D-10 | G3 | INT-01 crash states: an interrupted journal fsync or a missing seal manifest leaves the import unpublished and recoverable; a crash at each journal-state write classifies deterministically |
| PUB-026 | D-10 | G3 | INT-03: a sync-managed, network or otherwise unsupported location never causes deletion, relocation or blocked reads of existing projects; new publication warns or blocks only under the approved supported-location policy, with relocation and backup guidance; no claim of perfect sync-client detection |

### 6.1 Fault-injection boundary schedule

| Boundary | Failure injected | Expected post-restart response |
|---|---|---|
| B01 | Created staging, before durable intent | Authenticated unowned staging orphan isolated; no project shown |
| B02 | During record writes before commit | PREPARED candidate incomplete; no publication |
| B03 | During report/evidence write | Not sealed; no success, retain recoverable info |
| B04 | During audit receipt append | Transaction rolls back; not sealed |
| B05 | Integrity check, foreign key or cipher validation | FAIL; no published project |
| B06 | WAL checkpoint, sidecar cleanup | FAIL or recoverable staged state; no incomplete visible project |
| B07 | Database closes, before seal journal fsync | Reconstruct only with integrity verification; no false SEALED |
| B08 | SEALED, before fresh authorization | Recovery asks authenticated owner; does not silently publish |
| B09 | After old session expires | Old approval denied; fresh authorization needed |
| B10 | Destination name collision | Atomic no-replace refuses; no existing content altered |
| B11 | After publishing-intent durable, before rename | Candidate still staged; safe retry on fresh authorization |
| B12 | After rename, before journal update | Physical destination+hash resolves once; no duplicate |
| B13 | After rename, before directory durability | Document/test filesystem-specific guarantees; no invented certainty |
| B14 | Published, before readback | Recovery verifies; no second import |
| B15 | After readback, before staging cleanup | Published project valid; cleanup idempotent; evidence still present |
| B16 | Destination folder appears without journal | **Never quarantine an established preexisting project** |

For every B01–B16 inject: crash at process boundary, selected `EIO`, disk full where meaningful, and restart under original and different authenticated user. On Linux/macOS/Windows run OS-relevant subsets on actual packaged environment, not only fake filesystem. Report true power-loss testing separately; not all OSes provide identical fsync guarantees.

## 7. Storage, version gate, audit, analytics and scale

| Test ID | Requirement | Gate | Test / expected evidence |
|---|---|---|---|
| DB-001 | D-12 | S2 | Pre-gate binaries documented as unsafe for unknown future versions; only gate-enabled versions claim rejection |
| DB-002 | D-12 | S2 | Gate checks `user_version`/min writer across GUI, CLI, backup, restore, admin, repair, migration and all writers |
| DB-003 | D-12 | S3 | SQLCipher encrypted migrations pass integrity/FK/account recovery/admin escrow and old fixture roundtrip |
| DB-004 | D-12 | S3 | Normalized schedule is sole write authority; chart projection mutators reject writes to normalized schedules |
| DB-005 | D-12 | S3 | 50,000 editable tasks with full source outline, IDs, relationships and selected fields persist/read back, no whole chart JSON |
| DB-006 | D-12 | S3 | WBS v2 8MiB/10k/association limits remain; schedule import never silently becomes authoritative WBS scope |
| DB-007 | D-12 | S3 | Exact query plans and targeted index hit counts on representative realistic fixture under approved schema |
| DB-008 | D-14 | S5 | Revision-aware keyset pagination neither drops nor duplicates rows; stale cursor explicitly rejected/reanchored |
| DB-009 | D-14 | S5 | No long-held reader across Wails calls; WAL checkpoints proceed under slow UI navigation |
| DB-010 | D-14 | S5 | Viewport API returns only visible row/time data and clipped links; DOM/response sizes bounded |
| DB-011 | D-14 | S5 | Schedule cache invalidation from task, relation, calendar/resource changes agrees with full recompute |
| DB-012 | D-14 | S5 | One-field edit does not reserialize full chart; p95 edit and audit bytes measured |
| DB-013 | D-14 | S7A | 10,001 tabular rows: legacy preview honestly truncated; new ingest retains all, row 10,001 retrievable |
| DB-014 | D-14 | S7A | Schedule/resource/financial/risk/portfolio results reconcile with kernel + exact `internal/money` |
| DB-015 | D-14 | S7A | Cross-user portfolio queries cannot access other account data or stale approval contexts |
| DB-016 | D-13 | S7B | Legacy event bytes/hashes unmodified; mixed old and new typed chain validates |
| DB-017 | D-13 | S7B | Compact event exact decimal and large revision survives canonicalization/roundtrip |
| DB-018 | D-13 | S7B | Evidence object missing/tampered detected by digests; snapshots/replay reconstruct approved state |
| DB-019 | D-13 | S7B | Undo, receipts, immutable audit and evidence retention differ correctly; no accidental 256-limit reuse |
| DB-020 | D-11 | S6 | Shadow generation merge does not expose any partial schedule; stale revision/epoch rejects |
| DB-021 | D-12 | S0 | INT-03 evidence: on each OS's default data root and on representative redirected or sync-managed folders, record locking, same-volume no-replace rename, WAL and checkpoint behavior, case, path and ACL behavior, and durability; evidence only, no data moved |

## 8. UX, accessibility, comparison, AI and governance

| Test ID | Requirement | Gate | Test / expected evidence |
|---|---|---|---|
| UXQ-001 | D-15 | G3 | Choose→Inspect→Map→Review→Import flow accessible and truthful; cannot approve unseen material loss. G2 runs the Choose and Inspect stages |
| UXQ-002 | D-15 | G2 | Plugin Center reports tested read/write formats accurately and worker missing/incompatible states |
| UXQ-003 | D-15 | G3 | Job center distinguishes queued, parsing, staged, approved, publishing, recovered and failed |
| UXQ-004 | D-15 | G3 | 'Quick chart import' warns of skipped summary/assumption/limited project import; no deceptive parity |
| UXQ-005 | D-15 | G3 | Field inspector provides each independent fidelity dimension; privacy-safe report export |
| UXQ-006 | D-15 | G3 | Localization ADR-008 full sentence/plural; no error parsing by English string; IME and keyboard work |
| UXQ-007 | D-15 | S5 | 50k-row tree focus/selection restored across page/sort/filter/expand, native screen reader operation |
| UXQ-008 | D-15 | S5 | 50k Gantt viewport no all-row SVG/DOM; minimap and dependency jumps work without disorientation |
| UXQ-009 | D-15 | S6 | Compare includes identity-confidence, hidden selected counts and nonautomatic conflicts; no source-wins |
| UXQ-010 | D-15 | S6 | Selective merge summary matches actual committed changes; unapproved rows unchanged |
| UXQ-011 | D-15 | G2 | Every `interop.*` error code, job state and recovery state maps to a catalog key in `messages/en.ts` with matching placeholders and plural forms (plan Appendix B); new components start at zero in `hardcoded-text.test.ts` |
| AIQ-001 | D-16 | G5 | Revoked MCP grant rejects subsequent and in-flight tools after session/project/copy changes |
| AIQ-002 | D-16 | G6 | AI output is a proposal, never an autonomous direct schedule or money write |
| AIQ-003 | D-16 | G6 | Cloud providers optional, consented, off by default; local core works without them |
| AIQ-004 | D-16 | G7 | Third-party file/network access restricted by actual OS sandbox on every claimed platform |
| GOV-001 | D-19 | All | Source document immutable and decision IDs preserved; deviations require signed owner ACR |
| GOV-002 | D-19 | All | Test `BLOCKED/SKIPPED` cannot be reported as PASS; past SHA evidence not treated as current |
| GOV-003 | D-19 | All | No technical workaround changes normative design/test scope without explicit approval |
| GOV-004 | D-19 | All | PR shows assumptions, risks, failures, exact tests, rollback, architecture-diff and next priority |

## 9. Performance measurement and acceptance

**T2 target is not declared achieved** until realistic 50k-task encrypted project fixtures pass import→publish→reopen→paged UI→edit→save→audit→export/backups. T3 remains stress target. Track Linux, macOS and Windows individually; inspect idle/run JVM start cost separately from parser and IPC, and native+heap JVM memory separately from Go host. Track I/O amplification, WAL growth, retained evidence bytes, per-record index overhead, source/field mapping fidelity, peak disk use and time to recover interrupted imports.

Initial *proposed* UI budgets on owner-approved recommended hardware: warm 200-row paged query p95 <=250 ms, single-field foreground edit p95 <=500 ms, but do not treat these as approved targets until G0/S0 owner signs the hardware and methodology. UI scrolling metrics should use frame-time distribution, long-task frequency, focus loss and practical screen-reader behavior as well as p95. Measure cold/warm separately; retain raw latency distributions and confidence statements, not just one percentile. Test same file across parser versions to detect regressions.

## 10. Gate map and release authority

| Gate | Dependencies | Acceptance criterion |
|---|---|---|
| G0 | Read-only research/evidence | TPT-001–005, TPT-022 and licensing, Java native runtime, packaged feasibility, explicit Windows outcome |
| G1 | G0 acceptable candidate per shipping OS | TPT-006–021, fake workers, protocol compatibility, neutral registry, no DB mutation |
| G2 | G1 | FID-001–003, UXQ-001 (Choose and Inspect stages), UXQ-002, UXQ-011, malformed format corpus, accurate inspect counts and privacy |
| S0/S1 | Baselines + owner-approved ADR-011 | DB-021 (S0), schema DDL, format gate/old-version warning, migration plan, hardware profiles |
| S2 | S1 | DB-001/002; format gate shipped/tested before new format writes |
| S3 | S2 | DB-003–007, staging infrastructure, reproducible 50k typed records |
| S4 | S3+G2 | FID-004–018, source evidence, typed mapping, deterministic receipts |
| G3 | S4+G2 | PUB-001–026, all fault boundaries, UXQ-001/003–006, authenticated new-project import |
| S5 | S3 | DB-008–012, UXQ-007/008, package UI performance and accessibility |
| G4 | G3+S5 for full-scale claims | FID-019/020, tested formats/OS, export and package/sign/license tests |
| S6 | S3/S4/S5 | DB-020, UXQ-009/010, three-way conflict/rollback tests |
| S7A | S3 | DB-013–015, domain analytics and realistic data workloads |
| S7B | S3 and approved audit design | DB-016–019, mixed-chain and retention/replay evidence |
| G5/G6/G7 | Separate accepted ADRs | AIQ tests plus scope-specific security and distribution proofs |

**Important phase fix:** 50k typed storage tests belong to S3, but full 50k **MPXJ** import+readback is a G3 test after S4. End-to-end compare user acceptance belongs to S6, not prematurely to G4. The test corpus must include producer version families and each claimed OS; a test on one machine is not portable evidence for another.

## 11. Crosswalk to existing v2.1 acceptance IDs

Built in P00 as [`adr-010-case-crosswalk.csv`](adr-010-case-crosswalk.csv): all 110 earlier IDs, each with owner approval `pending`. The original instruction: during P00/P01 build `docs/testing/adr-010-case-crosswalk.csv` with columns: existing ID (`PLG-*`, `SCALE-*`, `REVIEW-*`, `ADR10-*`), existing description, v2.2 corresponding IDs, owner gate, retained/strengthened/changed, reason and owner approval. **Never delete a prior high-quality assertion because it is inconvenient.** Where v2.1 case wording was impossible (unknown fields a parser cannot enumerate; no listener observed ≠ network blocked), strengthen it to truthful precise semantics and link to the old ID. Additional test IDs in v2.2 are additive and do not imply historical tests have passed.

## 12. Test-report template

```yaml
id: TPT-003
status: BLOCKED  # proposed example, not an executed result
source_head: '<actual full commit SHA>'
branch: '<branch>'
platform: windows-amd64
jdk_vendor_version: '<actual pinned runtime>'
grpc_versions: '<actual go/java versions>'
fixture_sha256: '<when applicable>'
command: '<exact reproducible command>'
expected: 'Real grpc-java client streams over Go-hosted named pipe HTTP/2'
observed: '<record evidence; do not invent>'
resource_stats: {host_peak_rss_bytes: null, worker_peak_rss_bytes: null}
security_check: '<ACL, credentials, listener process evidence>'
blocker: 'Real transport prototype not yet demonstrated'
owner_approved_exception: false
notes: 'Do not select fallback transport without an ACR.'
```

**End of test strategy — all test entries are proposed.**
