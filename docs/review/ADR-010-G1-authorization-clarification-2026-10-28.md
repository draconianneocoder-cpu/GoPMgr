<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1: authorization clarification, 2026-10-28

**Decision:** APPROVED by the maintainer, James L. Burns, on 2026-10-28,
by selecting option 1: reaffirm and reconcile conditional G1 approval.
This authorizes bounded engineering within the accepted architecture.
It does not establish that a dependency, implementation, PR or release qualifies.

## Authority and precedence

- **2026-10-24:** the [accepted ADR-010 owner record](ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md)
  authorized P00, P01/G0 and P02/S0, reserving G1 permission.
- **2026-10-26:** the [P04 authorization at PR #143's recorded revision](https://github.com/draconianneocoder-cpu/GoPMgr/blob/a3a6a335dbb4a20941b692c11fac7e957a81c929/docs/review/ADR-010-G1-P04-authorization-decision.md)
  recorded conditional G1 approval, P04-D1 through P04-D8.
- **2026-10-27:** the [architecture directive](architecture-directive-2026-10-27.md)
  §2.5 still described OWN-11 as not approved for G1. That historical
  assertion conflicts with the P04 record; neither is silently rewritten.
- **2026-10-28:** after reviewing this conflict, the maintainer stated,
  “I also approve option 1.” This fresh reaffirmation resolves the
  permission question without inferring which earlier record should win.

For **G1 permission only**, this clarification supersedes the earlier
not-authorized statements in ADR-010, its owner register, implementation
plan and agent prompt, and the directive's §2.5 assertion. Other accepted
requirements and phase gates remain binding. Historical records retain
their original wording and dates.

## Conditions carried forward

All P04-D1 through P04-D8 and the mandatory safeguards remain binding.
Their [immutable decision text](https://github.com/draconianneocoder-cpu/GoPMgr/blob/a3a6a335dbb4a20941b692c11fac7e957a81c929/docs/review/ADR-010-G1-P04-authorization-decision.md),
[G1 plan](https://github.com/draconianneocoder-cpu/GoPMgr/blob/a3a6a335dbb4a20941b692c11fac7e957a81c929/docs/design/adr-010-g1-implementation-plan.md),
[transport design](https://github.com/draconianneocoder-cpu/GoPMgr/blob/a3a6a335dbb4a20941b692c11fac7e957a81c929/docs/design/adr-010-g1-transport-design.md)
and [dependency review](https://github.com/draconianneocoder-cpu/GoPMgr/blob/a3a6a335dbb4a20941b692c11fac7e957a81c929/docs/interoperability/adr-010-g1-dependency-review.md)
are source records, not evidence that PR #143 passes review.

| Decision | Continuing condition |
| --- | --- |
| P04-D1 | G1-1 through G1-9 proceed in documented dependency order, as small, independently reviewable PRs. Each passes applicable tests and security review before dependent work merges. Do not combine slices to accelerate delivery. |
| P04-D2 / OWN-11 | The documented grpc-go, protobuf-go and Windows go-winio dependencies are conditionally approved, subject to security, licensing, compatibility and reproducible-build verification. Retain patched `golang.org/x/net` and `golang.org/x/text`; review the complete dependency graph and record additions in `docs/DEPENDENCIES.md`. Prefer JDK NIO for Java UDS, minimize native libraries, and retain the approved Java LTS runtime and pinned, verified tooling. |
| P04-D3 | Enforce pipe instance limits, first-instance ownership, remote-client rejection and OS peer identity. Require mutual authentication before operational commands. Test same-user malicious processes, impersonation, unauthorized second instances, replay, and privileged/unprivileged interactions. Worker-to-host proof cannot mask an unauthenticated host-to-worker channel. |
| P04-D4 | G = 1 s, K = 1 s, M = 2 s and the 20% performance-comparison threshold remain provisional engineering bounds. Finalize only after representative hardware and p99 review; investigate exceedances. An unreaped process remains unhealthy; M is not a guaranteed OS termination time. These do not finalize OWN-10 or ADR-012 workload budgets. |
| P04-D5 | Retain Windows 10 x64 support pending a separately approved support-policy change. Missing tests remain BLOCKED; lack of hardware is not permission to drop a target. |
| P04-D6 | Keep an explicit resource acquisition and validation plan for Windows 10/11 x64 and minimum-version macOS. Hardware, signing and platform checks not executed remain BLOCKED, never PASS. Use authorized test systems and secured signing services; no keys or secrets in repositories, prompts, logs or fixtures. This record grants no additional hosted-CI or credential authority. |
| P04-D7 | TPT-002 may say “selected UDS runtime” instead of “native Netty runtime”; signing, installed-application, security, runtime and cross-platform coverage are unchanged. |
| P04-D8 | Separately prove production single-instance transport admission and protocol replay rejection using a controlled multi-connection test listener. Both are required. Preserve negative tests and explain rejection before protocol handling. |

Native gRPC over local IPC remains required: private UDS on Linux/macOS
and restricted named pipes as the Windows primary target, with
`npipe-async` preferred for the Java transport. No unapproved TCP or
stdio business-data fallback is authorized. Keep the minimal language-neutral
internal capability registry. Go retains exclusive authority over keys,
authentication, scheduling, exact money, audit and project persistence.
A worker process is not an OS sandbox. Do not weaken, delete or reclassify
tests to conceal a failure. Requirement conflicts or architectural
downgrades require an explicitly approved Architecture Change Request.

## Shared foundation and independent qualification

The directive's hybrid target uses one ADR-010 supervisor and capability
registry. The conditional permission covers this language-neutral G1
foundation; it does not authorize a separate Rust-only supervisor.

| Scope | Evidence still required |
| --- | --- |
| Shared G1 foundation | Ordered contract, transport, supervision, lifecycle and registry slices; real transport/authentication tests on supported targets; worker-disabled regressions. No shipped worker follows from fake-worker tests. |
| Java / MPXJ | Existing Go-to-Java, packaging, signing and platform requirements remain. G1 does not authorize MPXJ import or replace G2/G3 gates. |
| Rust analytics | Independently prove Go-to-Rust UDS and Windows named-pipe interoperability, authentication, host-mediated reads, confidentiality, resource limits and installed-platform behavior under ADR-012 DF0-C through DF0-G and its DF-001 through DF-065 specification. Java evidence does not qualify Rust, and Rust evidence does not qualify Java. OWN-11 does not approve unspecified Rust crate pins. |

## Merge, cutover and release boundaries

Authorization is not qualification or blanket merge permission. Rebase
and review each actual diff and dependency graph against current `main`;
retarget stacked children before deleting merged parent branches.
Preserve valuable evidence from competing branches rather than merging
incompatible behavior wholesale. Report PASS, FAIL, BLOCKED and NOT-RUN
separately, with exact revisions and commands.

This clarification does not authorize project migrations, format activation,
MPXJ import, broader architecture changes, a production Rust analytics
integration or release-readiness claims. The directive's independently
authorized work remains subject to its own gates.

DuckDB remains the implemented analytics backend under ADR-002 until the
separate Slice J cutover qualifies. Preserve shipped functionality and
require functional, security, resource, platform, licensing and rollback
evidence before removal. ADR-012 DF0 evidence alone is not production
authorization. This documentation change proves no runtime gate and
does not waive any outstanding approval or verification requirement.
