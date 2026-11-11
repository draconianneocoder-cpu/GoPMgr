<!--
SPDX-FileCopyrightText: 2026 GoPMgr evaluation prepared for maintainer review.
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# GoPMgr: Consolidated Architecture, Capability, and Outstanding-Issues Evaluation

**Evaluation date:** November 10, 2026 (America/Chicago)  
**Document revision:** 1.0, independent assessment; **not an accepted ADR or implementation authorization**  
**Repository:** [draconianneocoder-cpu/GoPMgr](https://github.com/draconianneocoder-cpu/GoPMgr)  
**Inspected `main` baseline:** [`1ca5ffaed63446b35b197a6083d03899c0143f55`](https://github.com/draconianneocoder-cpu/GoPMgr/commit/1ca5ffaed63446b35b197a6083d03899c0143f55)  
**Open-PR observation:** [#199, “Build the database engine to distrust file schemas”](https://github.com/draconianneocoder-cpu/GoPMgr/pull/199), **open, not merged** at inspection  
**Scope:** current product architecture and engineering evidence, verified changes, open problems, planned features, proposed roadmap additions, and decisions needed.  
**Evidence limitations:** read-only review of the connected GitHub repository, selected file contents and PR metadata, the November 4 critique, the November 5 architecture package, and the preceding ChatGPT evaluation. No source tree was built locally; no tests, benchmarks, native app workflows, security exploits or distribution checks were independently executed for this report. No source code or repository files were changed. Cognee recall returned no existing memories for this project.

> **Governing priority:** foundational architectural integrity → security and privacy → exact data correctness → independently measurable capability → maintainability and sustainability → engineering efficiency. Release timing is **not** a criterion for weakening or reordering quality requirements. Quality does not justify gratuitous abstraction or dependency growth.

## Contents

1. [Executive judgment](#1-executive-judgment)
2. [How the previous review must be corrected](#2-how-the-previous-review-must-be-corrected)
3. [Verified baseline and evidence confidence](#3-verified-baseline-and-evidence-confidence)
4. [Outstanding issue register: architectural foundations](#4-outstanding-issue-register-architectural-foundations)
5. [Outstanding issue register: security and recovery](#5-outstanding-issue-register-security-and-recovery)
6. [Outstanding issue register: verification and platform assurance](#6-outstanding-issue-register-verification-and-platform-assurance)
7. [Outstanding product features by roadmap workstream](#7-outstanding-product-features-by-roadmap-workstream)
8. [Recommended additions to the roadmap](#8-recommended-additions-to-the-roadmap)
9. [Documentation and governance contradictions](#9-documentation-and-governance-contradictions)
10. [Dependency-driven engineering sequence](#10-dependency-driven-engineering-sequence)
11. [Minimum acceptance and qualification matrix](#11-minimum-acceptance-and-qualification-matrix)
12. [Owner decisions required](#12-owner-decisions-required)
13. [Conclusion and source register](#13-conclusion-and-source-register)

## 1. Executive judgment

**GoPMgr is an established local-first application with functioning core capabilities and unusually extensive recorded verification activity. Its greatest unproven promise remains an integrated, correct, recoverable and responsive professional scheduling workspace at 50,000 editable activities.** That cannot be obtained merely by accelerating calculations or lifting constant limits. Normalized storage, authority-preserving transactions, compact audit, version-safe encrypted migration, bounded queries and native desktop virtualization must be designed and verified together, while implemented as separately approved slices.

**Current strengths, supported by repository source/evidence:** Go scheduling and CPM; resource leveling and Monte Carlo; EVM and exact-money analytics; SQLCipher encryption; WBS v2; budget/cost, Agile and Six Sigma functions; export and document tooling; Wails v3/Svelte desktop; Arrow-Go Parquet; secure-archive and recovery mechanisms; extensive automated tests. These are not equivalent to independent acceptance of every workflow on every platform. [S1][S2][S3][S4]

**Critical architectural debts:** (1) schedule data remains too concentrated in chart JSON; (2) historical whole-document audit writes scale with schedule size; (3) normalized storage and new-format/older-writer protections are specified but not activated; (4) the unified scheduling workspace and high-fidelity imports still need their authoritative data contracts; (5) genuine 50k activity end-to-end qualification is absent. [S5][S6][S7]

**Immediate security work:** PR #199 proposes compile-time SQLite untrusted-schema/defensive defaults but is not merged. Independent malicious-schema/backup conformance remains needed, and private signing material in archival bundles requires an explicit policy. The new SQLCipher core, CSV formula neutralization and certificate-path repair must **not** be counted as open implementation gaps. [S8][S9][S10][S11]

**Technology decision:** DuckDB was removed November 2; the DataFusion spike was **closed without adoption November 9** and preserved under `archive/adr012-datafusion-spike`. Arrow-Go v18.8.0 plus Go-native analytics is the deployed direction. DataFusion 56/Arrow 60, Iceberg-Rust and a Rust analytics worker should not appear in an active implementation queue without a *new*, explicit PS-09 workload and owner decision. [S1][S12]

**Product priority:** complete the authority model, professional scheduling data semantics, and testable import fidelity before attempting to deliver large numbers of disconnected features. The standalone read-only schedule-quality audit is a viable parallel feature because it does not change source records. [S6][S13]

### Status vocabulary

- **Implemented:** code is present on the inspected `main` branch, or a merged PR clearly establishes it. Does not itself claim native qualification.
- **Recorded evidence:** tests or measurements are described in source records; not rerun independently in this assessment.
- **Proposed / approved in principle:** design accepted as direction but its implementation not authorized or not present.
- **Authorized slice:** an explicitly approved bounded implementation stage; later stages require their own decisions.
- **Open / partial / blocked / unverified:** should retain its actual meaning, not be translated into “failure” or “pass” by a planning document.

## 2. How the previous review must be corrected

The prior evaluation identified the correct large-scale architectural concern but mixed older baselines, active design decisions, and recently completed fixes. Its improved successor must make the following corrections explicit.

| Prior review or November 5 package treatment | Corrected present assessment | Action |
|---|---|---|
| Wails declarations need to be generated as though no generation exists | **Wails 3.1.0 and generated bindings were merged November 9**; a typed frontend adapter exists. Contract drift, boundary semantics and complete native qualification remain valid targets. | Replace “implement binding generator” with **verify generation/completeness/security and exact-value transport**. [S1][S14] |
| SQLCipher fork still uses SQLite 3.33.0 | **SQLCipher 4.20.0 / SQLite 3.54.1** engine was merged in PR #196, while retaining locally maintained Go driver bindings. Windows qualification and ongoing C-code advisory surveillance remain open. | Update dependency inventory and ADR-011 feature assumptions; retain older-writer compatibility controls. [S9][S10] |
| DataFusion 56/Arrow 60 preparation is an immediate next priority | **DF0-G decided no-go November 9.** Reopening requires a new PS-09 justification, not an automatic library upgrade. | Remove from active critical path; retain archived evidence and a future evaluation trigger. [S12] |
| The archive certificate path is still unbounded/unvalidated | **PR #198 merged** a regular-file/extension/size check, controlled reading, and clearing of signing settings on restore. | Close the obsolete audit status; separately decide whether the private `.p12` should be inside the backup. [S11] |
| Every 50k benchmark is missing | Historical S0 measured 50k/100k CPM, edit/audit, and layout; RES-A-1 changed overallocation costs. **Full installed-app, current-tree T2/T3 results remain unqualified**. | Preserve benchmarks as diagnostics, not a feature acceptance pass. [S5][S15] |
| Dashboard restructuring R3/R4 is still deferred | The dedicated design/implementation record says R1–R4 were completed in August; an older backlog row is stale. | Reconcile active status, avoid reimplementing finished work. [S16] |
| Excelize pseudo-version must be maintained indefinitely | `go.mod` now pins **Excelize v2.11.1**. The older pseudo-version issue is resolved in the current dependency graph. | Remove from current outstanding issues. [S2] |
| PR #199 should be treated as completed hardening | It is **open** on a separate branch; the PR body reports tests but `main` remains unchanged until merge. | Require reviewed patch, negative controls, merged status and post-merge qualification. [S8] |
| “50 years” unambiguously means 18,250 working days | Code defines `MaxScheduleDays = 50 * 365` and uses the value as a **working-day offset**. That may be intentionally conservative, but it is not the same concept as 50 elapsed calendar years. | Formalize units and policy; separately test working calendars, elapsed-time claims and extreme values. [S17] |
| Existing documents can all serve as equally current status | ADR-010/011 and professional-scheduling documents include historical text from earlier source revisions; `docs/ARCHITECTURE.md` still says Wails v2. | Adopt a current-state index, while preserving historical decisions verbatim. [S1][S6][S18] |

**Critical methodological correction:** The severity labels in this report are *recommendations*. They are not official issue labels or blanket assertions of exploitable vulnerabilities. A finding can be a verified behavior, a recorded security observation, an untested threat, or a product requirement. Those are different evidence classes.

## 3. Verified baseline and evidence confidence

| Capability / element | Current observation | Confidence and remaining caveat |
|---|---|---|
| Go runtime | `go.mod`: Go **1.27.3** | Direct dependency-file observation. [S2] |
| Desktop | Wails **3.1.0**, Svelte 5 / TypeScript 6 / Vite 8, GTK 4 on Linux | Implemented; release/native qualification incomplete. [S1][S14] |
| SQLCipher | Driver bindings based on v4.4.2, vendored engine **SQLCipher 4.20.0 / SQLite 3.54.1** | Implemented, with preserved legacy compatibility tests recorded. Windows engine build/crypto RNG verification not established. [S2][S10] |
| Analytics | Go-native `PortfolioRollup` / `ImportTabular`; Arrow-Go **v18.8.0** | Implemented; 10,000-row bounded tabular result remains a separate cap. DataFusion rejected. [S1][S2][S12] |
| Scheduling | CPM, leveling, overallocation and Monte Carlo in Go | Implemented algorithms; incomplete calendar/constraint/effort/progress coverage and independent real-project verification. [S19] |
| Encryption / access / audit | SQLCipher project files, password/key recovery, administrator controls and current hash chain | Working foundation; future normalized audit transition and selected adversarial cases remain. [S6][S20] |
| Import/export | MSPDI quick chart import/export, Parquet/CSV/JSON data handling, PDF/A and other exports | Present for documented subsets; broad MPP/POD/XER field fidelity and worker integration not delivered. [S19][S7] |
| User interface | Dashboard, Gantt/CPM/network/PERT, cost and Agile/Six Sigma | Multiple screens implemented, not yet one revision-consistent connected professional scheduling workspace. [S19] |
| 50k/100k diagnostics | S0: CPM ~205 ms at 50k; ~577 ms at 100k. Whole-chart edit 2.5 s p50 / 3.8 s p95 and 35.5 MB audit per edit at 50k (historical). | Older synthetic Linux probe, not current native desktop qualification; old 61s Gantt layout is not a current result after RES-A-1. [S5][S15] |
| Build/security tooling | Make-centered ADR-013; P0–P5 substantially implemented; many source/race/static checks recorded | P6–P9 and final-artifact/native qualification remain. No tests independently executed here. [S1][S21] |
| Development and open work | At observation, one open PR (#199) and zero open standalone issues returned by GitHub issue search | Not proof that the backlog is empty; much of the work is tracked in Markdown and proposed designs. [S8][S1] |

**Separate three scale boundaries:** (a) **10,000-row** tabular import *result* limit; (b) **10,000-node** WBS v2 aggregate limit; and (c) **50,000 editable schedule activities** target with **100,000 stress**. No single constant change fixes all three. [S5][S7]

## 4. Outstanding issue register: architectural foundations

The following are unresolved *program* concerns, not necessarily discrete GitHub Issues. Suggested issue identifiers make traceability possible without claiming the repo already assigned them.

| ID / priority | Problem and present state | Corrective design | Required closure evidence / dependency |
|---|---|---|---|
| **ARC-01 / P0** | **Normalized schedule persistence is not active.** Legacy chart JSON and whole-chart writes remain the baseline. | Finalize ADR-011 physical schema for activities, hierarchy, links, calendars, constraints, assignments, baselines, notes and source identities; stable typed IDs and one write owner per domain. | Approved schema/DDL, measured query plans, migration design, foreign-key tests, compatibility review; before scale API. [S6][S7] |
| **ARC-02 / P0** | **Format/older-writer gate is incomplete.** Earlier binaries cannot be taught to refuse future files. | New distinct encrypted format/extension + isolated directory, non-mutating pre-write compatibility gate, explicit conversion and verified recovery copy; test historical binaries. | Negative tests for *every* writer route, read-only nonmutation, actual older-binary attempts, crash-safe fallback. Owner-approved ADR-011 slices. [S6] |
| **ARC-03 / P0** | **Whole-document audit growth and undecided compact bridge (DEC-04).** | Versioned compact typed domain events, exact numeric string representation, immutable historical bytes/hashes, receipts bound to sequence/hash; defined replay/checkpoints and evidence retention. | Existing chain verifies across mixed generations, tampering fails, replay matches state, per-edit bytes bounded by changed data rather than schedule size; independent DEC-04 decision. [S5][S6] |
| **ARC-04 / P0** | **No active normalized transactional command path.** | Typed revision-checked, idempotent commands; reserve write owner before revision read, atomic rows + revision + audit + receipt; indeterminate commit reconciliation. | Concurrent/stale-command, retry, crash, WAL, partial failure and exact readback tests; audit bridge and active format gate first. [S6] |
| **ARC-05 / P0** | **End-to-end 50k editable schedule still not proven.** | Keyset/revision-bound queries; bounded Wails transport; viewport Gantt and virtualized tree grid; no all-row DOM/SVG payload. | Native action-to-paint T2, T3 stress, sort/filter/pan/search/edit/undo/open/reopen with memory/latency distributions and correctness, on approved hardware. [S5][S15] |
| **ARC-06 / P0** | **Source-of-truth domain linking is incomplete.** Chart schedule, WBS scope, assignments, ledger, baselines and provenance remain separately implemented. | Stable cross-domain IDs and explicit relationship records; one authoritative record writer; clearly separated calculated forecasts from actual expenditures. | Cross-domain invariants, referential checks, revision-aware snapshots and no duplicate authority; never silently map chart outline to WBS nodes. [S6][S19] |
| **ARC-07 / P1** | **Imported project lineage and custom-field semantics not yet embodied in normalized data.** | Preserve artifact hashes, source IDs, field-level fidelity dimensions and typed/opaque extensions. | Re-import identity tests, UID reuse/renumbering, field roundtrip and explicit unsupported dispositions. [S7] |
| **ARC-08 / P1** | **Large result and analytics interface is still bounded by a 10k-row legacy response.** | Bounded typed, revision-safe paged or streamed result API, with exact numeric representation; Arrow-Go for suitable columnar data only when measured. | Cancellation, pagination, limit/exhaustion, large integer/decimal/null and no plaintext-spill tests; no automatic Rust engine. [S1][S12] |

**Design invariant:** SQLCipher in Go remains the authoritative persistent project database. Arrow, DataFusion, Iceberg, document formats and workers cannot become second writers or scheduling authorities. The Go kernel owns scheduling semantics; the cost ledger owns commitments/actuals. [S7]

## 5. Outstanding issue register: security and recovery

| ID / priority | Problem and evidence state | Recommended treatment | Exit criterion |
|---|---|---|---|
| **SEC-01 / P0** | **PR #199 remains open**: `SQLITE_TRUSTED_SCHEMA=0` / defensive defaults are proposed, not merged. | Review compatibility of flags for all connection paths, including restore, repair, plaintext conversion and legacy files; merge only with negative controls and independent review. | Post-merge baseline + hostile schema tests; no authorized app feature regression; Windows verification if Windows is claimed. [S8] |
| **SEC-02 / P0** | **Untrusted archive table definitions**: existing restore rejects views/triggers/virtual tables but the audit reports that crafted table constraints/foreign keys may remain trusted. Exploitability of each variant has not been independently established. | Version-specific allowlist of expected schema structure and critical invariants; read-only validation before writable connection or publication; fuzzed malformed files. Avoid relying on self-declared archive SHA-256 manifests as authenticity. | Adversarial fixtures are rejected without writes; legitimate historical backups accepted; exact schema drift and rejection reason tracked. [S9] |
| **SEC-03 / P1** | **Signing private key in `.pmba`**: PR #198 fixed path/size reading but notes `.p12` may include a private signing key and remains bundled. | Decide explicit opt-in encrypted key backup versus public-chain-only default; separate project recovery from signing-key portability. | Archive inspection tests, explicit user consent/retention contract, successful restore and re-provisioning behavior; cryptographic threat review. [S11] |
| **SEC-04 / P1** | **Low-severity residual account/security observations**: clipboard recovery codes, retained `.pre-encryption.bak`, sign-in timing/attempt policy, DEK material in DSN, other scan items. Recorded observations are not all verified exploitable vulnerabilities. | Triage individually against local attacker/offline threat model; prioritize plaintext secret retention and accidental disclosure, define controlled recovery. | For each finding: proof or refutation, owner disposition, deterministic regression test and documented residual risk. [S9][S20] |
| **SEC-05 / P1** | **Windows SQLCipher compatibility** remains unverified after engine upgrade; native RNG uses BCrypt. | Build and execute on Windows, test cryptographic RNG, encrypted read/write/restore/WAL/repair and all historical fixtures. | Native Windows signed/unsigned test evidence appropriate to claimed platform, not a cross-compilation-only claim. [S10] |
| **SEC-06 / P1** | **Vendored C-code advisory visibility** is separate from Go's `govulncheck`. | Pin upstream SQLite/SQLCipher provenance and monitor C advisories/CVEs; monitor ABI/build flags and local patches. | Dated dependency disposition with upstream diff, SBOM, regression results and documented residual findings. [S2][S9] |
| **SEC-07 / P1** | **Migration/backups lack power-loss and hostile-input qualification at the new-format level.** | Fault-injection and actual filesystem durability program for restore, no-replace publication, WAL/checkpoint, disk full and interruption. | Exact post-restart state, project/audit hashes, no partial published project, verified backup recovery on supported OS. [S6][S22] |
| **SEC-08 / P2** | **FIDO2 / FIPS policy and integrations** approved only as design direction. | Keep outside core storage migration until device-bound two-key recoverability and authorization are proven; no product-wide FIPS compliance claim inferred from device certificates. | Real hardware tests, escrow/incident recovery proof, signed installer and platform support qualification; separate authorization. [S23] |
| **SEC-09 / P2** | **Dev-only dependency alerts** (`minimatch`, `deepmerge`) remain with constrained fixes per current audit. | Monitor upstream remediation and effective exposure; do not run breaking `npm audit fix --force` merely to zero counts. | Reviewed advisories, updated when compatible patches exist, scanner results bound to source revision. [S9] |

**Do not reopen completed repairs:** PR #196 upgraded SQLCipher, bounded excessive schedule allocations, corrected formula injection and improved Monte Carlo; PR #198 hardened archive certificate handling. Retest these on changed source, but do not count them as unimplemented. [S10][S11]

## 6. Outstanding issue register: verification and platform assurance

| ID / priority | Gap | Required qualification |
|---|---|---|
| **VAL-01 / P0** | Wails v3/GTK 4 conversion is implemented but full native behavior is not qualified. | Real installed macOS 26 / Ubuntu 26.04 lifecycle, v2-data backup/restore, dirty-editor save/discard/cancel, crash recovery, file dialogs, IME/HiDPI/Wayland/accessibility; Windows only with actual Windows qualification. [S14] |
| **VAL-02 / P0** | Performance budgets/hardware remain proposals and representative real project fixtures are missing. | Owner-selected hardware, licensed fixtures, exact build/OS and raw traces; T0 500 / T1 10k / T2 50k / T3 100k; correct output before timing; distinguish benchmark from installed UI. [S15] |
| **VAL-03 / P1** | Build assurance ADR-013 P6–P9 is incomplete. | Final-artifact identity/provenance, scanner-failure behavior, native recovery qualification, policy adoption only after verified negative controls and owner sign-off. [S21] |
| **VAL-04 / P1** | Package trust is not fully operational. | Signed/notarized artifacts when applicable, actual certificate trust evidence for PAdES, Ed25519 update channel with protected signer and invalid/rollback manifest testing, real install/upgrade/uninstall proof. Keep offline operation. [S1][S24] |
| **VAL-05 / P1** | Independent scheduling and ProjectLibre comparison corpora not fully established. | Reference calculations and licensed representative source projects with checksums, exact differences, human review, versioned expected results; do not change oracles to match the implementation. [S19] |
| **VAL-06 / P1** | Frontend generated bindings are in place but output-contract and authorization drift need recurring verification. | Detect missing signatures, precision loss beyond 2^53, null/optional mismatch and exposed methods; independently test project/account authorization at host and store. [S1][S14] |
| **VAL-07 / P2** | Locale and accessibility coverage remains incomplete across workflows. | Keyboard/assistive-technology and translation tests on genuine desktop targets, including recovery and high-density schedule views. [S1][S24] |

**Evidence is not binary.** A passed model or smoke test is not proof of packaged-app correctness. A successful build is not proof of a supported operating system. `BLOCKED`, `NOT RUN`, `INCONCLUSIVE`, and `FAIL` need separate tracking.

## 7. Outstanding product features by roadmap workstream

The existing PS-series continues to be the correct product decomposition **if** its status table is refreshed to supersede the old DuckDB/DataFusion stages.

| Workstream | Outstanding feature or correctness requirement | Current state / governing dependency | Suggested criticality |
|---|---|---|---|
| **PS-00** | Accepted hardware profiles, representative fixtures, approved budgets and capability parity evidence | Baseline recorded; qualification inputs still incomplete. [S15] | P0 enabler |
| **PS-01** | Normalized connected schedule/resource/cost/provenance model | ADR-011 proposed, not activated. [S6] | P0 |
| **PS-02** | Read-only schedule-quality audit: disconnected logic, cycles, constraints, float, leads/lags, progress, overallocation | Detailed 18-rule spec exists; implementation not found on main. Independent slice needs approval. [S13] | P1, parallel |
| **PS-03** | Real MPXJ worker inspection, high-fidelity MPP/MSPDI/POD/Primavera parsing and new-project import, field-level fidelity receipts | MSPDI quick import/export narrow; production G1/G2/G3/G4 gates unfinished. [S7][S19] | P1 |
| **PS-04** | 50k safe editing, compact audit, viewport Gantt, indexed bounded queries | Architectural storage/audit blockers; overallocation rescan fixed, full UI not requalified. [S5] | P0 |
| **PS-05** | Synchronized Gantt/outline/dependency view, shared selection, saved filters/group/sort, revision-aware undo/redo | Separate chart editors exist; connected workspace proposal outstanding. [S19] | P1 |
| **PS-06** | Task Usage / Resource Usage timephased views and resource histograms | Kernel computes usage, UI and normalized assignment linkage incomplete. [S19] | P1 |
| **PS-07** | Effective-dated resource cost rates, overtime and cost-per-use; assignment cost reconciliation; controlled financial rebaselining | Cost ledger implemented, advanced rates/forecast semantics incomplete. [S19][S24] | P1 |
| **PS-08** | Missing constraint types; task/project/resource calendars; backward scheduling; manual splits; effort-driven/fixed-work policies; status-date semantics | Existing scheduler partial; require independent reference fixtures before adding policies. [S19] | P1 |
| **PS-09** | Indexed portfolio and resource/cost analysis, huge-dataset paging, comparison, selective user-approved merge and provenance-aware reports | Go-native analytics in production, DataFusion no-go; separate advanced features still open. [S1][S12] | P1/P2 |
| **PS-10** | Shared resource pools, optional controlled local collaboration and future extensions | Requires identity, locking, security and separate approved ADRs. [S1][S7] | P2 |
| **A-series** | Optional offline hardware security-key authentication; FIPS-mode governance | Approved in principle; no production authorization or hardware qualification. [S23] | P2 |
| **C-series** | Provider-neutral external Jira/work-management integration | Framework approved in principle; C-01 onward not authorized; keep cloud optional. [S25] | P2 |
| **ADR-008** | Complete localization and keyboard/screen-reader workflows | Phase 1 partly done; work-item editors localized Nov 10 and prior account localization. [S1] | P1/P2 |

**Professional capability gap of highest value after the foundation:** scheduling consistency, real-file import fidelity, integrated usage views and accurate project-controls reporting. A second analytical query engine or an optional cloud connector will not solve those gaps.

## 8. Recommended additions to the roadmap

These are **proposals only**, intended to supplement or sharpen existing PS/ADR tasks without duplicate frameworks.

| Proposed ID | Addition / rationale | Scope boundary and acceptance |
|---|---|---|
| **NEW-01** | **Adversarial project-file and archive conformance corpus.** The repository already has security findings for SQL schema and backup boundaries. | Shared, versioned fixtures for malformed SQLCipher, `.pmba`, MSPDI and later MPP; fuzz parsing and validate exact fail-closed result. No duplicate parser. |
| **NEW-02** | **Canonical operation/revision/provenance envelope.** Make scheduling, analysis, import and reporting compare results consistently. | Project/schedule generation, calculation version, status date, calendar version, source digest; separate source facts from computed values. Avoid creating a second transaction authority. |
| **NEW-03** | **Time and unit semantics ADR.** Current `MaxScheduleDays` language and import duration assumptions risk ambiguity. | Distinguish elapsed calendar time, working-day offsets, effort duration, lag and time-zone/DST interpretation; exact conversion fixtures and user-facing limits. |
| **NEW-04** | **Hostile and power-loss recovery laboratory.** Extend existing local probes. | Fault-injection, inode/rename/no-replace, transaction indeterminacy, read-only opening, disk-full and recovery across supported filesystems. No unverified guarantee of universal power-loss durability. |
| **NEW-05** | **Local-first, opt-in diagnostic bundle.** Supply evidence for real 50k issues without telemetry. | User-controlled export, scrub project data/DEKs/paths, include source hash, OS, WebView and timings. Never send automatically. |
| **NEW-06** | **Requirements/evidence register and supersession rules.** Prevent autonomous agents from following stale status claims. | Link requirement → accepted decision → implementation SHA → tests/fixtures → qualification → residual risks; preserve historical records rather than rewrite them. |
| **NEW-07** | **Secure backup material-classification policy.** Current `.pmba` may include `.p12` private key. | Decide default exclusion, explicit opt-in, encrypted retention, restore/re-enroll; separate signing certificates from project data. |
| **NEW-08** | **Data export fidelity and portability corpus.** The vision promises open portable exports. | Format/version fixture comparisons, round-trip limits, privacy controls, formula neutralization and export declarations; no false native MPP-writing claims. |
| **NEW-09** | **Supported SQLite feature policy for next-gen format.** Engine upgraded but older GoPMgr builds remain. | Evaluate modern constraints/indexes only under tested new-format isolation, minimum-writer guard and older-binary refusal. |
| **NEW-10** | **Architecture mutation/adversarial test drills.** Detect incorrectly weakened gates even if code compiles. | Fault-seeded schema, authorization, audit, crypto, worker permission and format gate tests tied to named requirements; avoid arbitrary test-count targets. |

**Not recommended as active foundational additions:** Iceberg-Rust, `datafusion-iceberg`, Apache Ballista, alternative transactional databases, untrusted third-party plugin marketplaces, mandatory cloud services, and a new performance-target deadline. They require separately demonstrated needs and cannot replace the SQLCipher project authority.

## 9. Documentation and governance contradictions

| DOC ID | Observed inconsistency | Repair, without deleting history |
|---|---|---|
| **DOC-01** | `docs/ARCHITECTURE.md` still describes **Wails v2**, root `App` layout and historical `wails build`; `go.mod` and `AGENTS.md` identify **Wails v3**, with `App` moved under `internal/desktop`. | Refresh current architecture overview and source paths. Keep dated migration evidence separately. [S2][S18] |
| **DOC-02** | `professional-scheduling-modernization-plan.md` diagram/PS-09 row still points DF0 → DataFusion cutover, but DF0-G is no-go and DuckDB removed. | Add a current-status amendment, replacing only the active roadmap, not historical approved decision text. [S12][S19] |
| **DOC-03** | October 26 capability matrix predates the SQLCipher upgrade, Wails migration, Go analytics and new security fixes. | Issue current capability matrix based on HEAD and exact test evidence, retain old matrix as versioned baseline. [S19] |
| **DOC-04** | `code-health-audit-2026-11-09.md` marks certificate path **Open** even though the same repository contains merged PR #198 and #199's body explicitly says the record needs correcting. | Fix **active disposition** to resolved by #198; keep original observation and explain the residual `.p12` policy decision. [S9][S11] |
| **DOC-05** | Some `beta-release-backlog.md` entries remain partially historical after implementation; dashboard R3/R4 row conflicts with its dedicated implementation record. | Separate `current outstanding` index from `historical execution` while keeping audit references. [S16][S24] |
| **DOC-06** | ADR-011 originally forbade certain SQLite 3.54 features because the engine was 3.33, and it contains earlier proposed options overridden by its own October 27 amendment. | Maintain a compact *controlling decisions* section: 128-bit IDs, isolated format/location, read-only no-migrate, layered older-writer protection. Update feature feasibility after formal compatibility testing. [S6][S10] |
| **DOC-07** | `docs/VISION.md` targets primarily 20–500 tasks, while accepted professional architecture pursues 50k editable tasks. These need not conflict, but the capacity promise is not explicitly tied to supported customer profiles. | Preserve primary-audience definition and state 50k as a **qualified advanced capacity objective**, not a claim of current functionality or mandatory enterprise PMO scope. [S26][S19] |
| **DOC-08** | Older ADR-010 status says only historical slices are authorized, but later October decisions authorize bounded G1 work; older doc also references obsolete analytics imports. | Maintain authorization/supersession index. Do not silently rewrite historical records or equate authorization with qualification. [S7][S20] |

**Recommended governance mechanism:** a small authoritative status file containing evidence date, commit, status, decision authority, release claims and exact link to record. Historical ADRs remain immutable; status overlays and new ADR amendments resolve changes. Avoid treating a document rewrite as proof of code modification.

## 10. Dependency-driven engineering sequence

A quality-first program uses **gates and prerequisites**, not calendar deadlines. Independent streams can proceed in parallel where they do not race on the same authority.

| Stage | Scope | Governing dependency | Exit gate |
|---|---|---|---|
| **A. Reconcile and secure current foundation** | Complete PR #199 review; fix security-document dispositions; decide private signing-key backup; qualify current SQLCipher engine and restore schema. | Current `main` and security decisions. | All active security findings classified and addressed/refuted; real fixtures and platform evidence for each claim. |
| **B. Finalize ADR-011 design** | Field/unit/identity semantics, normalized schema, old-writer isolation, migration publication model, audit bridge DEC-04 and recovery matrices. | Approved architecture and source evidence. | Explicit owner decisions and independent design critique; no production migration yet. |
| **C. Implement inactive storage and protected format** | Non-mutating gate, separate project format/location, inactive candidate building, pre-migration encrypted backups. | Stage B design. | Read-only and old binary tests; no mutation of legacy projects; fault-injection through inactive stages. |
| **D. Add compact audit and typed commands** | Single-writer/revision-checked/idempotent transactions; receipt and audit continuity; only then activate normalized generation under separately approved gates. | Stage C plus DEC-04. | Byte-for-byte old-chain fidelity; state replay, concurrent retries, power-loss and receipt reconciliation. |
| **E. Deliver truly connected large schedules** | Keyset and viewport queries; virtualized Gantt/outline; linked usage/baselines/cost projections. | Stable normalized query and command model. | 50k real-app editing and 100k stress, correct end-to-end results, approved native hardware budget. |
| **F. Expand professional semantics and import** | MPXJ worker/field ledger, custom calendars, constraint/task policies, full-fidelity new-project import, resource/financial controls. | Source/provenance and schedule authority; native worker security gates. | Golden source fixture/readback, exact semantic parity and no silent loss or unexpected writer access. |
| **G. Add selective advanced capabilities** | Schedule-quality audit may run in parallel from Stage A; later S6/S7 analytics, security keys, providers, local diagnostics. | Separate specific decisions and needed foundations. | Feature-specific correctness, security, privacy, accessibility, compatibility and acceptance evidence. |

**No stage may:** introduce dual mutable authorities; rewrite historical audit bytes; publish incomplete encrypted projects; allow Rust/Java workers project DEKs or writable databases; weaken tests for speed; drop an approved future deliverable merely because it is difficult; or claim unsupported OS/format capability.

## 11. Minimum acceptance and qualification matrix

These are **test categories**, not invented timing or budget thresholds. Numeric targets must be approved after representative hardware/fixtures are selected. All relevant negative controls must fail when the safety mechanism is deliberately disabled.

| Gate | Smallest meaningful verification | Evidence to retain |
|---|---|---|
| **GATE-01 Security / hostile input** | Current, older and malformed SQLCipher; hostile archive schema; oversized values; wrong key; unauthorized account/file paths. | Fixture hash, observed error, unchanged file/rows, scanner/test logs. |
| **GATE-02 Legacy and older-writer compatibility** | Legacy `.gopmgr` read-only probe and normal read; new-format attempt using historical binaries through GUI, headless, restore and explicit path. | File/header/journal hashes; no unauthorized write; explicit refusal/backup behavior. |
| **GATE-03 Migration durability** | Interrupted inactive migration, disk-full, write conflict, WAL/checkpoint failure, uncertain commit, restore, interrupted publish and repeat idempotent recovery. | Final authority generation, verified audit, ciphertext/no plaintext residual, recovery receipt. |
| **GATE-04 Audit integrity** | Mixed legacy + compact chain, tamper, reorder, omission, evidence deletion, replay/snapshot, money/integer beyond JS safe range. | Independent verifier result, exact original-event hashes, replay readback. |
| **GATE-05 Scheduling correctness** | FS/SS/FF/SF, cycles, positive/negative lag, eight constraint types, calendars, DST, actuals/remaining work, leveling, baseline/EVM. | Independent expected outputs with provenance; policy versions. |
| **GATE-06 Scale / UI** | T0 500, T1 10k, T2 50k native interactive and T3 100k bounded stress. Cover open/edit/paint/scroll/filter/pan/audit/backup/reopen, not just CPM. | Raw action-to-complete traces, p50/p95 and confidence bounds, peak RSS, payload size, edit/audit bytes, failures. |
| **GATE-07 Interoperability** | Versioned MSPDI/MPP/POD/XER fixtures where legally and technically supported, inspector-only mode, expected field-disposition ledger, incomplete/cancelled import. | Source digest, format/version, exact field fidelity, encrypted all-or-nothing publication, readback. |
| **GATE-08 Worker isolation** | Go-owned auth/grants, file stream, native gRPC UDS/Windows named pipes, cancellation/restarts, denied DEK/database/direct path access, OS containment claims. | Actual platform traces, negative permission tests, supervisor receipt and resource bounds. |
| **GATE-09 Desktop / compatibility** | Native installed Wails v3 macOS 26 and Ubuntu 26.04 on claimed profiles; Windows only if advertised; save/discard/cancel, recovery, keyboard/AT/IME/HiDPI/Wayland. | App/install SHA, OS/WebView, test script, UI evidence and failures. |
| **GATE-10 Dependencies / distribution** | Go and C advisories, codegen/bridge contract, locked toolchain, release SBOM, signing/notarization, invalid/rollback update manifest. | Provenance hashes, disclosure/waivers, independently checked final signed artifacts. |
| **GATE-11 Legal and privacy** | GPL/third-party notices, MPXJ LGPL analysis for actual packaging, opt-in telemetry/diagnostics and secret-free logs. | Artifact notice inventory, reviewer disposition, user consent and negative tests. |

The existing OWN-10 sampling plan is the starting point for **how** to measure, not evidence that its unapproved candidate budgets were met. [S15]

## 12. Owner decisions required

Do **not** infer authority from this assessment. In particular, the following need explicit acceptance or bounded implementation permission at their proper gates.

| Decision | Proposed disposition | Why it must be explicit |
|---|---|---|
| **DEC-AUDIT** | Approve a compact, versioned append-only audit bridge preserving all old hashes. | Enables normalized writes; otherwise ARC-04 remains blocked. |
| **DEC-FORMAT** | Ratify next-gen file format semantics, user-visible conversion, backup/rollback, old-writer proof and SQLite compatibility policy. | File incompatibility and data loss are high-stakes irreversible outcomes. |
| **DEC-UNITS** | Define whether “50 years” is elapsed duration or working-day offsets; units for lag/work/effort and timephased records. | Avoid silent calendar and import distortions. |
| **DEC-SECRET** | Choose whether `.pmba` may include private signing keys and under what consent/encryption rules. | Material confidentiality and restoration tradeoff. |
| **DEC-HARDWARE** | Select supported performance hardware profiles and representative real licensed project fixtures, then approve numeric budgets. | Prevent synthetic performance from being advertised as delivered capacity. |
| **DEC-INTEROP** | Authorize G1/G2/G3 bounded production worker slices and signed runtime packaging when prerequisites are proven. | Existing spikes and conditional dependency approval are not release qualification. |
| **DEC-WINDOWS** | State actual OS support intent while preserving earlier cross-platform promises unless explicitly amended; require real host tests. | Compiling Windows artifacts is not running Windows. |
| **DEC-ANALYTICS** | Keep Go-native analytics; open a new ACR only for an evidenced advanced workload that Go cannot satisfy reasonably. | DF0-G explicitly declined DataFusion. |
| **DEC-ROADMAP** | Accept a concise current-status register and corrected active roadmap while retaining historical records. | Prevent agents from implementing stale instructions or claiming completed work is missing. |

## 13. Conclusion and source register

### Consolidated engineering verdict

GoPMgr's next quality leap is **not** a different analytics engine, a broader feature catalog, or a faster release. It is a stable, formally governed project data model with one writer per record; a fully compatible and provable audit transition; secure and reversible-by-backup next-generation encrypted storage; and real, tested 50,000-activity interactive capability. Once those foundations exist, MPXJ import fidelity, advanced scheduling semantics and connected reporting become tractable without architectural compromises. Security hardening and an isolated read-only schedule-quality audit can progress concurrently. Keep all previous design decisions and the complete feature scope traceable, but correct stale current-state summaries.

### Sources consulted (links are directly reviewable)

| Ref | Repository source | Supports |
|---|---|---|
| **S1** | [Current roadmap](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ROADMAP.md) | Wails v3, DF0 no-go, feature priorities, P0/P1 follow-ups |
| **S2** | [go.mod](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/go.mod) and [dependencies](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/DEPENDENCIES.md) | Current build/runtime and native driver pins |
| **S3** | [Architecture overview](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ARCHITECTURE.md) | Current/historical architecture description |
| **S4** | [AGENTS.md](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/AGENTS.md) | Agent guardrails and Wails v3 source structure |
| **S5** | [ADR-010 S0 benchmark](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/adr-010-s0-baseline.md) | Historical 50k/100k measurements, limit distinctions |
| **S6** | [ADR-011](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-011-normalized-schedule-store.md) and [Oct 27 directive](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/review/architecture-directive-2026-10-27.md) | New format, domain authority, transactional audit, no older-writer mutation |
| **S7** | [ADR-010 v2.2](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-010-plugin-architecture.md) | Worker/native IPC, imports, identity and source fidelity, scale boundaries |
| **S8** | [Open PR #199](https://github.com/draconianneocoder-cpu/GoPMgr/pull/199) | Proposed SQLite untrusted-schema hardening, not yet merged |
| **S9** | [November code-health/security audit](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/engineering/code-health-audit-2026-11-09.md) | Recorded threats and corrections; its certificate status is stale |
| **S10** | [Merged PR #196](https://github.com/draconianneocoder-cpu/GoPMgr/pull/196) and [SQLCipher upgrade evidence](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/sqlcipher-4.20-upgrade.md) | New engine/limits/Monte Carlo, Windows qualification gap |
| **S11** | [Merged PR #198](https://github.com/draconianneocoder-cpu/GoPMgr/pull/198) | Certificate path/size fix; residual `.p12` policy question |
| **S12** | [DF0-G no-go](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/adr-012-df0-g-report.md) | DataFusion non-adoption and archived evidence |
| **S13** | [PS-02 schedule-quality spec](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/PS-02-schedule-quality-audit-spec.md) | Read-only 18-rule proposed audit |
| **S14** | [Wails v3 qualification](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/wails-v3-migration-evidence.md) | What passed and what installed/native work is open |
| **S15** | [OWN-10 qualification plan](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/testing/ps04-own10-qualification-plan.md) | Workload tiers, provisional budgets and sampling |
| **S16** | [Dashboard restructuring record](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/dashboard-ia-restructuring-proposal.md) | R1–R4 already implemented |
| **S17** | [Schedule limits code](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/internal/kernel/limits.go) | Actual `MaxScheduleDays` expression and units comment |
| **S18** | [App move PR #187](https://github.com/draconianneocoder-cpu/GoPMgr/pull/187) | App relocated to `internal/desktop` |
| **S19** | [PS capability matrix](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/review/PS-capability-matrix.md) and [professional modernization plan](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/professional-scheduling-modernization-plan.md) | October baseline of outstanding professional capabilities; requires current-state reconciliation |
| **S20** | [G1 authorization clarification](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/review/ADR-010-G1-authorization-clarification-2026-10-28.md) | Approved scope vs. unimplemented production worker |
| **S21** | [ADR-013 build assurance](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-013-engineering-build-assurance.md) | P0–P9 phases and gaps |
| **S22** | [Beta backlog](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/beta-release-backlog.md) | Recovery, package lifecycle and security risk items |
| **S23** | [Tiered security-key roadmap](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ROADMAP-TIERED-SECURITY-KEYS.md) | FIDO2/FIPS proposal and non-authorization |
| **S24** | [Next alpha plan](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/next-alpha-release-plan.md) | Install/upgrade/signing/UX evidence conditions, not deadlines |
| **S25** | [Provider-neutral integration proposal](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-TBD-provider-neutral-integration-framework.md) | External integrations approved only in principle |
| **S26** | [VISION.md](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/VISION.md) | Local-first priorities, primary 20–500-task audience, export portability |

**Source handling:** The October 25 S0 metrics and October 26 capability matrix are *dated evidence*, not claims about the inspected November 10 binary. The November 5 quality-first package was treated as a prior proposal and corrected for later accepted changes. The preceding ChatGPT review is a secondary interpretation, not primary repository authority. No external authority to merge, migrate, sign, publish or change owner policy is inferred from this report.
