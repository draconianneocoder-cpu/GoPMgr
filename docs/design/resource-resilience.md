<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Resource resilience: required development contract

**Direction:** Owner requested this planning integration on 2026-11-14.
**Baseline:** `0952c8f0267e57b3351f8d4ba26744d1bb28494b` (PR #214).
**Status:** Required planning and acceptance obligations for affected development;
application-wide enforcement is not implemented or qualified. This document does
not authorize runtime code, numeric budgets, OS mechanisms, dependencies, schemas
or a release. Existing bounded implementation authorizations keep their conditions.

## Requirement and guarantee boundary

Memory pressure from an admitted workload must not terminate the desktop, corrupt
project data or lose acknowledged durable changes within its qualified hardware,
platform and workload profile. Eligible new heavy work waits for temporary capacity;
worker failure is contained, safely replayable work may retry within policy, and
the user can understand and control recovery. Saving, cancellation and recovery
must remain responsive while heavy work waits or fails.

Prevention, containment and recovery are distinct obligations. System-wide OS
termination, power loss and hardware failure cannot be promised away. Such failures
require recovery of acknowledged durable writes and supported draft checkpoints;
unpersisted UI edits must never be described as durable. If the stated profile or
checkpoint guarantee cannot be met, mark the affected capability BLOCKED, preserve
the prior durable state and obtain a decision. Do not waive a release gate or
reduce existing scale, format, accuracy or support commitments to pass it.

## Critique of the assessment and current evidence

| Assessment gap | Required correction |
| --- | --- |
| Existing worker prose could imply delivery | ADR-010 D-06 is accepted architecture; ADR-015 D07–D10 is proposed detailed design. `internal/interop/deps_pin.go` retains dependencies only. No production application-wide memory governor was found at this baseline. |
| A limit on iterations could appear sufficient | `internal/desktop/app_charts.go` calls `kernel.RunMonteCarlo` in the host. `internal/kernel/montecarlo.go` preallocates tasks × iterations samples and additional worker state. At 50,000 × 10,000 × 8 bytes the samples alone are 4,000,000,000 bytes; this calculation is not a benchmark or capacity claim. |
| Admission could wait for future workers | MR-1 protects existing in-process paths independently of G1. It is cooperative allocation control, not fatal-OOM containment. MR-2 adds isolation through the existing supervisor only. |
| Queue/retry could conceal loss or endless failure | Bound descriptors and attempts; distinguish temporary contention from permanently oversized work. Reconcile committed effects before replay; notification failure never triggers recomputation. |
| Qualification could slip to the end | MEM requirements and applicable tests are exit conditions for each affected implementation slice. MR-5 evidence accompanies delivery, rather than replacing earlier checks. |

Fixed schedule/row/record limits remain useful input defenses. They do not prove
aggregate memory safety. Go's [memory limit](https://go.dev/doc/gc-guide#Memory_limit)
is soft and does not cover all native allocations. `recover` is not containment
for fatal runtime OOM. A goroutine or worker thread shares its process's failure
boundary. A process group, signing or RSS monitoring alone is not a memory sandbox.

## Contract and ownership

The maintainer assigns a named implementer, domain owner and independent reviewer
before each authorized slice. Roles below identify accountable functions, not
people already assigned. Every applicable test starts **NOT RUN**; the
[assurance contract](../testing/resource-resilience-assurance.md) defines oracles.

| ID | Mandatory outcome | Accountable role | Earliest planning phase | Oracle |
| --- | --- | --- | --- | --- |
| MEM-01 | Define effective machine capacity and a versioned workload profile from installed RAM, platform/container/job limits, current pressure, OS headroom and application baseline. Failed discovery never means unlimited memory. Record multi-instance policy; multiple app instances cannot each assume the same free capacity. | Platform owner | MR-0 | MEM-T01 |
| MEM-02 | Assign logical subsystem budgets under one aggregate envelope: host/control, webview/rendering, SQLCipher/native/cache memory, ingestion, compute, export/signing, transport, descendants and staging. Preserve explicit interactive/save/recovery reserve. Count overlapping copies and peaks without claiming a per-module hard heap boundary. | Host owner | MR-0 | MEM-T02 |
| MEM-03 | Atomically reserve a checked resource estimate before source loading, decoding, task conversion, expansion, graph/sample creation or result transfer. Enforce batch/record/output bounds incrementally. Cover overflow, uncertainty and all concurrent consumers; no check-then-allocate race or unlimited fallback. | Host/domain owner | MR-1 | MEM-T03 |
| MEM-04 | Queue fair bounded descriptors, not complete datasets. Define count/byte limits, age/expiry, cancellation, visibility and admission backpressure. Waiting holds no project lock, transaction or partial resource reservation. Temporary capacity shortage waits; oversized/unqualified work becomes visibly blocked with approved options, never an endless queue or silent truncation. | Job owner | MR-1 | MEM-T04 |
| MEM-05 | Keep saves, cancellation, quit/recovery and essential UI outside the heavy-work queue. Re-evaluate external pressure and estimation error after admission with bounded hysteresis. Stop new heavy admissions and safely cancel/checkpoint affected jobs if reserve is threatened. If checkpoint capacity is unavailable, retain prior durable state, report pending edits honestly and block further unsafe work; no false save acknowledgement. | Desktop/persistence owner | MR-1 | MEM-T05 |
| MEM-06 | Run qualified heavy computation/render/parse work in owned worker processes using the existing ADR-010 registry, typed IPC and supervisor. Bound the whole process tree before expensive work. Keep keys, DB authority, validation, receipts and final PAdES authority in the host; bound remaining host operations. No second supervisor, privileged worker DB access or DataFusion revival. | Supervisor/domain owner | MR-2 | MEM-T06 |
| MEM-07 | Give cancellation an owner and bounded checkpoints. On exit, reap descendants and close streams/handles/staging before returning capacity. A cancel or kill request is not proof of reclamation; failed cleanup retains unhealthy accounting and prevents oversubscription. | Supervisor/host owner | MR-1, then MR-2 | MEM-T07 |
| MEM-08 | Retry only classified replay-safe work with bounded attempts/time/cost, fresh capacity and authorization after verified cleanup. Preserve immutable inputs, seed, algorithm/runtime identity and operation identity, with distinct attempts. No identical same-cap OOM loop; no automatic replay of non-idempotent or ambiguous effects. Reconcile receipts before deciding to retry; unknown death is not confirmed OOM. | Job/domain owner | MR-2 | MEM-T08 |
| MEM-09 | Distinguish volatile queue acceptance, durable acceptance, started, result available, durable commit/issue acknowledged and notification delivered. Define draft checkpoint coverage/recovery window and storage guarantees; accept durably only after the required write succeeds. Use existing approved persistence where adequate; new queue/journal/receipt/issue schemas require applicable format/audit/retention decisions. | Persistence owner | MR-0, then MR-3 | MEM-T09 |
| MEM-10 | Bind work to account incarnation/session, project copy, revision, source digest, parameters and approved destination. Revalidate when starting/retrying and at commit. Use consistent snapshots; changed revisions or expired grants require stale-result handling/new approval. Waiting never extends a transaction or grants another account access. | Security/domain owner | MR-1 | MEM-T10 |
| MEM-11 | Show queued reason, progress, resource block, confirmed/unknown termination, retry count, cancellation boundary, durable result and recovery options with accessible localized messages. Keep diagnostics bounded and private. Notification is best effort; its failure cannot undo a commit or cause duplicate work. Restart explanations require a successfully persisted record. | UX/job owner | MR-1 | MEM-T11 |
| MEM-12 | Independently qualify each claimed platform/profile/workload with positive, pressure, failure, recovery and prohibited-effect evidence. Preserve 50k editable/100k stress objectives and exact numerical/report semantics; no lower iterations, approximated percentiles, missing rows or reduced fidelity without a separate accepted contract. | Qualification/domain owner | MR-0; every delivery | MEM-T12 |

Subsystem limits inside one process are cooperative accounting, not separate
failure domains. One explicitly owned host admission service supplies the common
budget model; it is not a global service locator. The supervisor reuses that model
when available. Estimates need measured error margins and runtime backpressure;
they are not promises that physical RAM will remain available. Acquire required
resources coherently so jobs cannot deadlock by holding one scarce resource while
waiting for another. Do not shrink an active limit below usage and assume safety.

## Work sequence and delivery gates

| Phase | Required work and dependencies | Exit before affected work is called complete |
| --- | --- | --- |
| MR-0 Profile and contract | PS-00 / DEC-HARDWARE and RILD-0: inventory allocation boundaries and workload families; name owners, supported profiles, draft/durability semantics and independent oracles. Existing numeric hypotheses remain unapproved. | MEM-01/02/09/12 scoped; measured baseline and numeric decision approved before capacity acceptance. Characterize safely in bounded disposable environments, not system-wide exhaustion on the user's host. |
| MR-1 Protect existing host paths | Independently of G1/Wasm, add checked estimates, admission/accounting, bounded queues, cancellation and UI/persistence reserves to the authorized in-process slice. Monte Carlo is the first proposed pilot; include chart loading/JSON decode before its samples. | Applicable MEM-01–05/07/09–12 tests. MR-1 does not qualify heavy-work OOM isolation; preserve its explicit residual risk until MR-2 evidence. |
| MR-2 Isolate and supervise | After applicable approved G1 prerequisites, move the pilot's heavy computation through the existing supervisor; then adopt the same boundary for eligible ingestion, rendering and analysis. Fresh workers and safe retry retain current G1 conditions. | MEM-06–08/10–12 and host-reserve checks on real processes. G1 process lifecycle is not OS memory containment; demonstrate per-platform enforcement before the corresponding claim. No new dependency/transport/runtime selection by this table. |
| MR-3 Durable acceptance and recovery | Use approved save/checkpoint mechanisms now where adequate. New persistent queue/journal, command receipt and issued-byte storage require applicable DEC-FORMAT, DEC-AUDIT, ADR-011 and retention/security decisions alongside RILD-2b. | MEM-08–11 recovery/receipt oracles; no durable queued-job or every-keystroke guarantee before demonstrated storage behavior. Pure read-only admission need not wait for new schemas. |
| MR-4 Adopt per domain | Apply MR-0/1 and applicable MR-2/3 to PS-03 imports, ARC-05/08 large data/UI, PS-09 analytics and RILD-2a/4/5 reports from each affected slice. Bounded host-native reporting does not wait for Wasm/public SDK. | Each domain's allocation-to-output path, source integrity and exact numerical/format oracles pass. Unsupported heavy profiles remain BLOCKED; a small-path success cannot qualify a large path. |
| MR-5 Qualify alongside delivery | Q5/Q7/Q8 and applicable EX3/EX4/EX6: native installed evidence, pressure/failure/recovery and responsiveness per profile before each corresponding capability/release claim. This is repeated, not a final catch-all phase. | MEM-T01–12 applicability and retained independent evidence; no mandatory failure or NOT RUN may be labelled PASS. |

MR-0 definition precedes the affected implementation plan. MR-1 can proceed under
its own bounded authorization without waiting for G1; MR-2 depends on G1, not the
reverse. MR-3's new storage decisions gate stronger recovery claims, not every
read-only report. Existing save/correctness repairs and urgent security work stay
prioritized; this contract does not authorize incidental scheduling changes.

Numeric policies remain with DEC-HARDWARE/OWN-10. Linux
[cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) and
Windows [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
provide candidate mechanisms, with installation permissions and process-tree limits
to prove. macOS requires a demonstrated supported mechanism; polling, a pressure
notification, JVM heap cap or Go GC tuning does not prove a hard aggregate limit.
Missing enforcement blocks the corresponding isolation claim, not permission to
silently weaken the supported-platform contract.

## Mandatory slice handoff

Existing memory-intensive paths must enter the MR-0 inventory and receive the
same applicable evidence before a new capacity/resilience qualification claim.
Every new or materially expanded memory-intensive path must link this contract and
record: affected MEM IDs and phase; named owners/reviewer; input/output bounds;
profile and effective ceiling; estimated and observed peak plus estimation error;
queue/admission/cancellation behavior; cleanup and retry classification; source and
operation identity; durable acknowledgement/checkpoint contract; applicable
MEM-T results with exact code/build/data/environment; and prohibited effects.
N/A needs specific path/profile reasoning and independent reviewer agreement.

Missing prerequisites or failed mandatory oracles leave the affected slice BLOCKED
and prevent its completion/qualification claim. These are review obligations, not
an implemented CI enforcer: no automated runtime or manifest gate is claimed here.
Future agents must update [STATUS](../STATUS.md), the applicable plan/register and
evidence together. Original source archives and the companion DOCX retain their
historical baselines; this contract supplements current canonical plans.
