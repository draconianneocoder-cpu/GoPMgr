<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012 ACR: macOS containment implementation gap

**Status: proposed, NOT APPROVED.** No acceptance criterion changes in this PR.
**Owner approval:** pending. No privilege, new production dependency, transport
change or resource-policy relaxation is authorized by this document.

## Requirement and evidence

ADR-012 v0.2 binds DF-033 (DataFusion pool, RSS and scratch bounded separately)
and DF-064 / DF0-D2 (combined host-wide RSS/CPU/jobs with 2/4/8 workers,
pre-launch rejection and clean crash/over-quota recovery). ADR-010 permits
Unix process groups without a containment claim. The spike has reservation
admission and per-child final accounting, not the required actual bounds.

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
| **1. Preserve the strict contract; defer cutover while completing authorized DF0 work (recommended)** | Preserves current production analytics behavior and keeps the replacement's accepted security/resource requirements unchanged. Native containment remains a blocker. Independent E/E2 and F/F2 work can reduce the rest of the critical path; current DuckDB licensing/distribution restrictions remain in force | Already within the isolated DF0 authority. Research a supported containment implementation; do not claim it exists. No production shipping or release clearance follows |
| **2. Propose a measured monitoring/termination contract for macOS** | Admission plus independent current-resource sampling could detect and stop overuse, but permits bursts, observer delay and possible descendant escape. It cannot guarantee a strict instantaneous aggregate ceiling. Lower adoption cost does not remove confidentiality, availability or exhaustion risk | Separate owner-approved relaxation with explicit overshoot/latency limits, fail-closed observer failure, minimum-OS tests, hostile burst/fork/escape cases, crash and recovery. It does not satisfy the current DF-033/064 contract |
| **3. Evaluate a stronger OS isolation or privileged-containment architecture** | May offer stronger controls, but no candidate is qualified here. A helper/VM or new entitlement introduces deployment, trust, maintenance, licensing, signing and IPC-compatibility costs. A VM transport cannot silently replace accepted native gRPC/UDS. Do not assume root or a footprint-limit call alone gives aggregate RSS enforcement | Separate design/authority review before adoption, supported API and entitlement proof, per-target threat model, explicit RSS/CPU semantics and aggregate/process-escape tests, packaging and rollback evidence. No new helper, VM, entitlement or transport is selected here |

Retaining DuckDB in option 1 is a temporary compatibility position, not release
licensing approval. None of these options authorizes a DuckDB upgrade.

## Recommendation and adversarial critique

Keep option 1 as the governing position while the strict contract is unresolved;
expedite independent feasibility evidence rather than adopting a weaker cap by
implication. Pursue supported OS containment inside DF0 where existing authority
permits. Request a specific design decision before adding privileges or changing
an accepted guarantee. Do not install a privileged service merely to make the
spike pass.

Challenges to the recommendation: it delays removal and retains legacy native
licensing exposure; parallel work could be wasted if containment cannot be
qualified. Mitigation: keep experiments isolated, finish reusable numeric and
format oracles, and retain explicit stop/go criteria. Option 2 expedites
integration only by accepting a different contract. Option 3 might fail on the
advertised minimum OS, require unavailable entitlements, or move rather than
solve the resource boundary. The evidence does not select an implementation.

**Independent adversarial critique: pending reviewer availability.** The
structured self-review above is a disclosed fallback, not independent approval.
No ACR approval is required to preserve the existing strict contract. A decision
to relax it or adopt a new production architecture must be explicit.

## Required proof before qualification or cutover

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
