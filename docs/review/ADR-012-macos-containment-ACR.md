<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 ACR: macOS containment implementation gap

**Status: proposed, NOT APPROVED.** No acceptance criterion changes here.
**Owner approval:** pending. No privilege, new production dependency, transport
change or resource-policy relaxation is authorized by this document.
**Decision packet reviewed:** 2026-11-01 against integration head `7cb56af`.

## Requirement and evidence

ADR-012 v0.2 binds DF-033 (DataFusion pool, RSS and scratch bounded separately)
and DF-064 / DF0-D2 (combined host-wide RSS/CPU/jobs with 2/4/8 workers,
pre-launch rejection and clean crash/over-quota recovery). ADR-010 permits
Unix process groups without a containment claim. The spike has reservation
admission and per-child final accounting, not the required actual bounds.

The [accepted architecture](../design/ADR-012-datafusion-analytics-replacement.md#df-10--memorycpuspill-and-cancellation)
requires OS process limits as well as query-pool limits. The
[DF-064 criterion](../testing/adr-012-datafusion-feasibility-spike.md)
requires host-wide peak RSS/CPU/jobs within configured limits or early typed
rejection. Neither document defines an instantaneous CPU-rate ceiling, a
measurement window, or final numeric budgets. The previous version of this ACR
described a strict instantaneous ceiling as if those details were settled.
They are not. This correction does not make sampled compliance sufficient or
change the requirement to enforce actual bounds. Clarification must preserve
that requirement; any proposed allowance for exceeding it needs explicit
approval as a contract change.

The [reproducible native evidence](../testing/adr-012-df0-macos-limits.md)
shows that the examined AS, CPU-time and process-group primitives do not
compose into the required guarantees. The SDK's footprint-limit call also
returned `KERN_NO_ACCESS` to the unprivileged probe, although its task-info
control succeeded. This is a concrete implementation gap, not proof that
macOS cannot satisfy the requirement through any architecture. A setting,
header declaration, per-child lifetime peak or sampling monitor is insufficient
proof. CPU rate, sampling intervals, budgets and RSS/footprint accounting
semantics also need explicit definition before production qualification.

A later [pre-exec follow-up](../testing/adr-012-df0-macos-limits.md#absolute-limits-across-exec-and-child-creation-follow-up)
found a narrower building block: non-root child-local `RLIMIT_NPROC=0` survived
exec and refused direct fork/spawn, while an authenticated Rust generation job
still completed. Practical absolute AS ceilings were refused because the
launcher image's virtual size was already about 500 GB. A larger AS control
survived exec and denied a mapping, establishing VM control rather than an RSS
cap. These findings reduce design uncertainty without qualifying aggregate
RSS/CPU, delegated-process restrictions or the full sandbox contract.

## Alternatives

| Option | Impact and limits | Approval and verification |
| --- | --- | --- |
| **1. Preserve accepted requirements; defer cutover (governing interim position)** | Preserves current production analytics behavior and keeps the replacement's accepted security/resource requirements unchanged. Native containment remains a blocker. Further isolated evidence can reduce other gaps, but does not resolve this decision or clear current DuckDB licensing/distribution restrictions | Already within isolated DF0 authority. Define the resource accounting and qualify an enforcement mechanism before claiming DF-033/064. No production shipping or release clearance follows |
| **2. Design and measure a native monitoring/termination policy for macOS** | Admission, bounded query/transport allocations and an external observer could reduce resource exhaustion and stop detected overuse without adding a privileged service or VM. Sampling can miss bursts; a starved observer may not run before host exhaustion. This is not a hard ceiling or a hostile-process containment boundary | Owner must explicitly authorize any relaxation of actual bounds. First approve a design/measurement scope, then review measured overshoot, detection/termination delay and failure behavior against proposed numeric budgets. Sampling alone cannot prove a worst-case overshoot bound. No DF-033/064 PASS or production adoption follows from permission to investigate |
| **3. Evaluate a stronger OS isolation or privileged-containment architecture** | May offer stronger controls, but no candidate is qualified here. A helper/VM or new entitlement introduces deployment, trust, maintenance, licensing, signing and IPC-compatibility costs. A VM transport cannot silently replace accepted native gRPC/UDS. Do not assume root or a footprint-limit call alone gives aggregate RSS enforcement | Separate design/authority review before adoption, supported API and entitlement proof, per-target threat model, explicit RSS/CPU semantics and aggregate/process-escape tests, packaging and rollback evidence. No new helper, VM, entitlement or transport is selected here |

Retaining DuckDB in option 1 is a temporary compatibility position, not release
licensing approval. None of these options authorizes a DuckDB upgrade.

## Decision needed before selecting an implementation

Choose the policy direction separately from implementation and production
approval. Option 1 remains in force until the owner records a different
decision. An instruction to expedite replacement is not approval of option 2
or 3. Permission to investigate either is not acceptance of its residual risks.

| Contract field | Decision and evidence required |
| --- | --- |
| Accounted scope | Define the analytics budget across host source/Arrow/gRPC buffers, workers and descendants, and whether unrelated UI/host allocations are included. Specify whether multiple app instances share a machine budget. A controller for one Go host is not machine-wide enforcement |
| Memory | Choose current RSS or physical footprint and how shared pages are charged; report both when needed for comparison. Define the aggregate budget and whether any exceedance is permitted. Simultaneous usage cannot be reconstructed by adding independent lifetime peaks |
| CPU | Define cumulative CPU time, utilization over a stated window, or both, including units and aggregation. Reserved `CPUSlots`, worker thread count and `RLIMIT_CPU` are different controls; none by itself establishes a shared utilization ceiling |
| Reaction and failure | For option 2, propose detection/termination objectives and permitted exceedance from measurements, then request owner approval. Refuse new jobs on missing/stale measurements; specify who detects observer failure and how already running work ends. Record starvation and kill failure, rather than promising a sampling interval proves a maximum |
| Process ownership | Define direct and delegated child creation, session escape, PID reuse, host death and restart. Qualify identity/cleanup without trusting worker-reported usage or assuming process-group membership is permanent |
| Supported delivery | Qualify macOS 13+ Apple Silicon from the installed/signed app, with Linux next. Development evidence on macOS 27.2 is narrower. Windows execution is deferred; changing its advertised support or acceptance gate is a separate decision |

Final budgets remain reserved under DF-OWN-05. Synthetic fixture limits are
not product defaults. These fields do not authorize protected spill, broader
file/network access, new privileges, a different transport or shipping a
partially qualified worker. DF-062 and the remaining security matrix stay
independent of the resource-policy decision.

## Supported building blocks and limits of the sources

The native probes above establish effects on this development machine. They
do not qualify a deployable macOS containment architecture. Official Apple
documentation supplies further constraints, checked on 2026-11-01:

- Virtualization's [memorySize](https://developer.apple.com/documentation/virtualization/vzvirtualmachineconfiguration/memorysize)
  describes guest physical memory backed by a reserved host virtual-memory
  region. Its [cpuCount](https://developer.apple.com/documentation/virtualization/vzvirtualmachineconfiguration/cpucount)
  configures guest CPUs. These properties do not document a combined host-side
  RSS or CPU-rate ceiling for GoPMgr and its workers. Treat such a ceiling as
  unproved, not implied by choosing a VM.
- A [VM configuration](https://developer.apple.com/documentation/virtualization/vzvirtualmachineconfiguration)
  requires the virtualization entitlement. Image/kernel updates, native IPC,
  offline packaging, host overhead and rollback would need a separate design.
- [XPC](https://developer.apple.com/documentation/xpc) provides helper lifecycle
  and privilege isolation. That documentation does not establish the resource
  enforcement needed here. A helper needs a supported control mechanism and
  its own authorization, failure and release evidence; replacing analytics
  gRPC with XPC is not proposed.

## Recommendation and adversarial critique

Resolve the policy direction now instead of repeating independent qualification
slices while this blocker remains. Keep option 1 as the interim position. For
an ordinary unprivileged desktop deployment, option 2 is the smallest candidate
design to measure, **if the owner is willing to consider its weaker exhaustion
protection**. This is a conditional engineering recommendation, not evidence
that the residual risk is acceptable or that its tests will pass. If never
exceeding the configured aggregate limits is required even under hostile
allocation or observer starvation, sampling cannot meet that requirement;
choose option 3's separately authorized feasibility work or keep cutover
blocked. No qualified stronger mechanism is currently identified.

Challenges: option 1 delays removal and retains legacy licensing exposure;
option 2 may exhaust memory before its observer acts and is not equivalent
protection. Mitigation: preserve blocked gates and require a separate decision
on measured limits and residual risk before adoption, with no automatic fallback.
Option 3 might fail on the advertised minimum OS, require unavailable
entitlements, or move rather than solve the resource boundary. The evidence
does not select an implementation.

**Independent adversarial critique: pending reviewer availability.** The
structured self-review above is a disclosed fallback, not independent approval.
No ACR approval is required to preserve the accepted resource contract. A decision
to relax it or adopt a new production architecture must be explicit.

## Required proof before qualification or cutover

- After a policy direction is approved, first specify a minimal mechanism,
  resource-accounting definitions and independent oracle. Stop if required APIs
  or entitlements are unavailable, measurement cannot distinguish admitted
  capacity from actual usage, or the mechanism cannot meet the chosen policy.
  Do not replace a failed enforcement test with a reservation-only assertion.
- Define current/simultaneous RSS versus physical footprint, CPU-rate versus
  lifetime CPU-time accounting, included host/source buffers and descendants,
  and owner-approved budgets. Never sum independent lifetime peaks as if they
  were simultaneous measurements.
- Run native 2/4/8-worker cohorts against real DataFusion, including reserved
  mappings, allocations outside its query pool, bursts, wrong claims, overlap,
  fork/session escape, cancellation, kill, host death, observer/control failure
  and recovery. Check externally observed actual limits or typed early refusal.
- Prove no plaintext spill and independent OS-level file/network boundaries,
  separately from quotas. Preserve DF-062's BLOCKED status until demonstrated.
- Qualify Linux next, keep Windows and signed/minimum-OS gaps visible, and
  finish numeric/format parity, fair comparative performance, packaging,
  dependency licensing and independent review before the production decision.
- No data/schema migration is proposed. The experiment rolls back by reverting
  its tests/docs; production remains unchanged. A later architecture must supply
  its own deployment, failure recovery and rollback plan.
