<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->
# Provider integration assurance, adversarial validation and acceptance

**Status:** framework approved in principle; this is a proposed test plan only and **all 48 PI tests are NOT RUN** until separately executed and evidenced. Zero test passes claimed. **Normative sources:** `ADR-TBD-provider-neutral-integration-framework.md` and the accepted ADR-010/ADR-011 contracts if/when approved.

## 1. Test governance

- Every acceptance claim requires an independent test/oracle, bounded result, dated platform/configuration, tested code SHA, fixture source/license, reproducible seed, toolchain and executable/manifest hash. Use raw evidence and counts; do not mask initial failures with test retries or mark unavailable tests PASS.
- Named environments: Linux x86-64, macOS arm64 and x86-64 where supported, Windows amd64/arm64 where advertised. Per-platform local IPC is a **real packaged** test, not a simulated Linux test. Java MPXJ Windows gate is distinct from Go Jira Windows gate.
- Real test Jira tenant and documented user roles are required; synthetic HTTP fixtures are necessary for adversarial faults but insufficient for public API fidelity, OAuth, quotas or permission guarantees.
- Verify old release/open/backup/restore and current beta regression suites; schema upgrades must be reversible or honestly documented as forward-only with backups. A feature flag off means zero background connector operation.
- Include `go test`, race/fuzz/property tests, frontend accessibility/keyboard tests, service lifecycle tests, SQLCipher fault-injection/hard-kill tests, packaged codesigning/notarization tests, and `make verify`/`make check-release` from repository's authoritative process.

## 2. IDs and independent proof

| Test ID | Failure / invariant | Independent proof and mandatory result |
|---|---|---|
| PI-001 | Connector absent/disabled | Offline core works; no network/auth prompts; previous projects, scheduling, exports unchanged |
| PI-002 | No Jira coupling | Static import and dependency checks forbid `jira` imports in core/host domain; unrelated fake provider passes contracts unchanged |
| PI-003 | Manifest substitution | Modified/symlink-swapped/unsigned worker refused before start; signed release hash matched in each package |
| PI-004 | IPC spoof/replay | Unauthorized/wrong-tenant/wrong-worker/old nonce clients rejected; restricted UDS/npipe permissions/cleanup proven |
| PI-005 | Platform fidelity | Real gRPC HTTP2 over native UDS/npipe with streams, deadlines/cancellation; no TCP/stdio fallback and no unproven Windows claim |
| PI-006 | Same-user risk honesty | Worker has no DB/key/token API; adversarial process has no privileged broker actions; OS confinement claims verified separately |
| PI-007 | Broker route restrictions | SSRF to localhost/private DNS/metadata, redirects, alternate domains, crafted headers, cross-cloudId and unknown verbs rejected |
| PI-008 | Secret boundary | Instrument worker IPC, host logs, backup, crash report, env and project: zero tokens/client secrets; vault encrypted and login-gated |
| PI-009 | OAuth security | Correct state/redirect binding, callback forgery/CSRF prevention, scopes and tenant selection; approved confidential client secret topology proven |
| PI-010 | Refresh failure | Concurrent refresh, token rotation, expired/revoked grant and lost response leave correct persisted state; reauth rather than credential guess |
| PI-011 | Account isolation | A user/account/project copy cannot import/activate another user's site, vault grant or snapshot |
| PI-012 | No direct worker mutation | Inject hostile `canonical update` messages; host rejects; project/audit hashes unchanged |
| PI-013 | Big-data streaming | 10k/50k source data with bounded per-page memory/IPC/storage; inspect no truncation claimed as complete |
| PI-014 | Duplicate/order/resume | Reordered pages, duplicate remote issues, changed page tokens, process kill; stable record IDs/count/digests after retry |
| PI-015 | Snapshot atomicity | Kill process at every durable transition, restart/reopen: either previous complete or next complete revision; never mixed |
| PI-016 | Audit identity | New events and old audit chain independently verify; no history rehash; exact numeric serialization and privacy-safe references |
| PI-017 | Remote eventual consistency | Inject delayed search visibility and changed issue during paging: show uncertainty, reconcile overlap, no invented instant snapshot |
| PI-018 | Deletion vs permission | 403, 404, JQL scope loss, archive/move, revoked grant, true delete tested separately; no blind local deletion |
| PI-019 | Rate-limit/reliability | 429 Retry-After, points quotas, DNS failure, TLS error, 5xx, timeout, offline power loss; last good snapshot persists |
| PI-020 | Field accuracy | Jira custom field schema drift, ADF rich text, parent hierarchy, issue key rename, user IDs, locale/timezone/decimal tests |
| PI-021 | Domain semantic safety | Story points not converted to hours, due date not hard constraint, `blocks` not CPM dependency unless explicit preview approval |
| PI-022 | Copy/backup/restore | Encrypted source evidence and mappings survive, vault tokens do not transfer, restored project cannot dispatch remote writes until rebound |
| PI-023 | Privacy/retention | Data minimization, consent withdrawal, provider retention/purge, encrypted cache, no immutable personal payload in audit |
| PI-024 | UI safety/accessibility | Nontechnical user can connect, inspect coverage, preview, stop, disconnect, purge; keyboard/screen-reader and offline modes pass |
| PI-025 | Outbox crash/replay | Kill before, during and after remote send, lose response: `OUTCOME_UNKNOWN`, no unsafe blind retry, readback adjudication |
| PI-026 | Real racing writer | Two distinct Jira users change same field around approval; automatic remote updates blocked if no provider CAS equivalent |
| PI-027 | Jira workflow | Status transitions use available transition APIs/permissions; no arbitrary status PUT; failed transition does not fake success |
| PI-028 | Security supply chain | Pinned SBOM/licenses, dependency audit, signing/notarization, vuln fix policy, no unsigned replacement masquerading as bundled |
| PI-029 | Field sync granularity | Remote/local/derived/manual_shared ownership enforced per field; three-way compare rejects out-of-date revisions |
| PI-030 | Second provider conformance | Entire fake-other-provider (followed by required real second provider before general maturity claim) operates without core model, supervisor, audit or DB API changes |

`PI-025`–`PI-027` and outbound parts of `PI-029` are **later-release** gates; not required to make a **read-only** release pass. `PI-009` is BLOCKING for real-distributed Jira authentication. `PI-005` must be passed on every **advertised** platform; missing support must be explicitly marked BLOCKED, never silently rerouted.

## 3. Detailed crash and transaction matrix

For both local initial sync and ongoing refresh, inject hard termination/power interruption at each relevant persisted transition:

| Cut point | Expected recovery |
|---|---|
| Prior to first remote request | No new snapshot; no leaked credential; pending attempt expires |
| During a paginated response | Received but unacknowledged data ignored; last complete revision remains active |
| After page staged, before checkpoint | Replay/dedup page; no active partial snapshot |
| After durable checkpoint, before next page | Continue using checked page digest/token or restart bounded window safely |
| After final page, before completeness verification | Staged incomplete; no activation |
| After verification, before revision commit | Old snapshot active; stage recoverable/quarantined |
| During atomic snapshot/audit commit | Either full new revision+audit or old revision, verified on reopen |
| After commit, before success UI | Reopen idempotency receipt; show complete once, no duplicate |
| After credential revocation, before next poll | New lease fails; cached view treatment follows consent/visibility policy |
| After outbound intent commit, before dispatch | Pending intent requires fresh authorization; no remote request |
| During outbound remote HTTP request | Outcome unknown, halt unsafe retry, inspect provider result |
| After Jira commit, before local confirmation | Re-read remote and verify semantic postcondition; may remain unresolved |
| After confirmation, before local receipt | Repeat safe verification; do not send write again |

Use real SQLCipher reopen, backup and independent audit verification; unit-mocking transactions cannot establish power-loss durability. Confirm journal/WAL behavior on minimum supported filesystem and platform; state limits of simulated sudden power cut versus real controlled power-loss testing. Do not claim absolute stability across arbitrary storage hardware failures.

## 3a. New high-severity contract, isolation, consistency and recovery cases (v2 review)

| ID | Mandatory independent test | Required outcome |
|---|---|---|
| PI-031 | Account linked workspace exists with **no** project and local encrypted state is reopened offline | No implicit project creation, no plaintext cache, explicit stale status |
| PI-032 | Crash after sealed project publication but **before** account workspace acknowledgment | Locate receipt by operation ID and reconcile once, no duplicate published project |
| PI-033 | Delete/revoke account workspace and restore independent project backup | Project keeps its own provenance/evidence and works offline; link requires rebind |
| PI-034 | Two app processes/profiles attempt same binding at once; kill lease owner and skew wall clock | File/OS lock plus durable generation fences stale writer; only one promoted revision |
| PI-035 | Swap/tamper signed provider operation policy, manifest, worker or endpoint template | Reject policy/digest mismatch; no arbitrary URL, JQL, header or network escalation |
| PI-036 | Compromised same-user bundled worker directly opens a socket or reads another local file | Report actual OS enforcement or record trusted-process residual access; never claim broker enforced egress if it did not |
| PI-037 | Register a second real work provider with different rate, auth, tenancy and field model | No core schema/scheduling API changes, compatibility tests pass; no Jira-only assumptions |
| PI-038 | Register fake analytical/report plugin family using shared registry/supervisor | No reuse of work-provider protobuf for unrelated business data, host authority retained |
| PI-039 | Mixed minor/major protobuf versions and unknown enum/field; retire policy digest | Backward-compatible negotiated capability or fail closed; no silent writes |
| PI-040 | Multiple Jira issues have identical updated timestamps, change membership mid-page, cursor expires | Re-scan/dedup; no false snapshot completeness or missed record silently reported |
| PI-041 | 50k sparse inventory with selective ADF/custom field hydration and rate limits | Bounded memory/disk, truthful field fidelity, cost/quota accounting, performant local paging |
| PI-042 | Two connectors consume shared API quota, CPU and disk under backpressure | Fair bounded work; no starvation or resource runaway; offline UI remains responsive |
| PI-043 | Receive 429 for global points, burst and per-issue writes; Retry-After and quota reset | Appropriate per-budget cooldown, typed reason codes, bounded retries |
| PI-044 | Remote write returned ambiguous result, identical field value posted by another actor | No false causal confirmation; `POSTCONDITION_OBSERVED` distinct from correlated commit |
| PI-045 | Project and account workspace migrated to incompatible next-version schemas | Correct independent writer fences; encrypted backup; safe read-only/upgrade path |
| PI-046 | Revocation/erasure during staged import and old-snapshot retention | Enforce policy while preserving user-owned schedule, protected audit continuity and proper redaction |
| PI-047 | Key recovery, password rotation, admin/escrow cases for account vault/workspace | No plaintext tokens, no orphaned decryption state; paths consistent with existing ADRs |
| PI-048 | Signed app upgrade invalidates provider policy and connector API changes | Feature disabled safely; previous offline snapshot still browsable and repairable |

## 3b. Test evidence contract

For each PI test record fixture ID and raw digest, test seed, source/baseline commit and schema version, OS/arch/installer, adapter and policy hashes, actual execution commands, observed assertions, p50/p95 sample size when applicable, outcome (`PASS/FAIL/BLOCKED/NOT RUN`) and reviewer. A mock OAuth/HTTP server proves shape, *not* compliance with official Atlassian production OAuth or real tenant quota. Failure injection needs post-crash independent verification from a clean process; rerunning to green is not sufficient. A partial integration roll-out cannot inherit proof from ADR-010 MPXJ tests without evidence of the distinct Go worker and cloud API paths.

## 4. Performance evidence format (predeclared, no invented numbers)

For each `T0/T1/T2/T3` case record: OS, CPU, RAM, drive, bandwidth/latency, Jira tenant entitlements, user scopes, field population, work hierarchy depth, relationships, ADF/custom field payload, real/synthetic ratio, records changed per run, number of API requests, 429 count, bytes exchanged, full process RSS peak, CPU and wall duration distribution, local SQL DB/WAL/temp size, authenticated worker startup/cancel latency, consistency-pass duration, row-paging first paint and scroll latency, accessibility regression status.

Before a release, approve explicit budgets for: memory per live connector, max staging growth, max retry/time, 10k/50k sync completion and interactive first paint. Publish p50/p95 only with adequate measured samples and methodology; distinguish cold/warm cache, provider rate-limit effects, and total wall time. Stress may exceed target within **safe bounded failure** limits; label it correctly.

## 5. Adversarial review checklist and forced negative tests

- Security reviewer attempts credential exfiltration, fake worker registration, user account switch race, provider-cloudId spoofing, SSRF, HTTP redirect and log injection.
- Data reviewer attempts implicit semantic conversions, duplicate cloud identities, regenerated Jira issue keys, overwritten user-edited fields, broken source lineage and audit rewrites.
- Reliability reviewer injects disconnect/retry, process kill, token expiry, malformed ADF/custom fields, 429 under partial resume, staging collision, disk full, crypto integrity failure and bad filesystem permission.
- UX/accessibility reviewer tests paused, offline, not authorized, partially scoped, conflict and quarantine states without relying on color-only status or inaccessible virtualized views.
- Maintenance reviewer tests a second fake provider and protobuf previous-major/minor compatibility, protocol downgrade rejection, reproducible signed packaging, no unreviewed license changes, and code/doc/test drift.

**Stop rule:** any security, identity, data-loss, token, cross-project, false completion, or audit-integrity failure blocks the affected release. A failed Windows transport test blocks that connector on that target rather than inviting an agent to silently replace gRPC with TCP or stdio. An incomplete dataset is not success. A single green rerun without investigated original failure does not clear a gate.
