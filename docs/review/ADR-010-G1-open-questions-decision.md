<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 — Maintainer direction on the open questions

**Date:** 2026-10-26.

**Given by:** the maintainer, in the session conversation, in response to:

- the [G0 Windows transport report](../testing/adr-010-g0-windows-transport-report.md)
  at `473708f`;
- §9 of the [G1 transport design](../design/adr-010-g1-transport-design.md).

**Effect:** design directions for G1, under the existing ADR-010
requirements and acceptance gates.

- It answers G1 design questions Q1 to Q5.
- It keeps `npipe-async` as the preferred Windows G1 candidate.
- It authorizes three things: recording these decisions, finishing G0
  evidence within the scope already authorized, and preparing the G1
  implementation plan and dependency review for separate approval.
- It does **not** authorize production G1 code, production dependencies
  or release integration.

**Binding on:**

- [ADR-010](../design/ADR-010-plugin-architecture.md) D-03, D-05 and D-06;
- OWN-02, OWN-03 and OWN-11;
- the [G0 Windows transport decision](ADR-010-G0-windows-transport-decision.md);
- the [G1 transport design](../design/adr-010-g1-transport-design.md).

## Decision text (verbatim)

> GoPMgr ADR-010 — Maintainer Direction on G1 Open Questions
>
> The G0 evidence and the proposed G1 transport-neutral design have been reviewed. I approve the following design directions, subject to existing ADR-010 requirements and acceptance gates.
>
> Q1 — Transport fallback: APPROVED
>
> Use exactly one explicitly selected, validated local-IPC transport per platform and release. Do not silently fall back to another native transport, loopback TCP, or a custom stdio business-data protocol. A transport change requires an evidence-backed decision.
>
> Q2 — Unix process containment: CONDITIONALLY APPROVED
>
> For G1, use process groups and deterministic termination for the trusted bundled worker. Document the residual setsid escape risk. This is not approval to claim complete process containment or sandboxing.
>
> Evaluate Linux cgroup v2 confinement and stronger OS controls during subsequent security work. Third-party workers must meet their separate sandbox and containment gates before release.
>
> Q3 — Unix gRPC implementation: APPROVED DIRECTION
>
> Prefer JDK NIO Unix Domain Sockets on Linux and macOS to reduce native-library packaging, signing, and maintenance complexity. Confirm equivalent correctness, security, and acceptable performance against Netty epoll/kqueue on target hardware before final production selection.
>
> Q4 — Windows and signing validation: APPROVED TEST POLICY; RESOURCES PENDING
>
> Windows 11 x64 desktop validation is required before broadly claiming Windows desktop support. Retain Windows 11 ARM64 verification where ARM64 support is advertised. Validate Windows 10 x64 if it remains in GoPMgr’s approved support matrix.
>
> Identify available CI, virtual-machine, or physical-device test environments and report any unavailable coverage as BLOCKED. Do not silently remove supported platforms.
>
> Use controlled test-signing infrastructure for development. Production signing and notarization require separately authorized credentials or signing services. Never request or place private signing keys in source code, fixtures, logs, or chat.
>
> Q5 — Worker shutdown grace: APPROVED DIRECTION
>
> Use a bounded cancellation grace period with a provisional minimum of one second. Determine the final value from representative p99 latency measurements on approved hardware, with a documented escalation policy and maximum shutdown bound.
>
> If the worker does not exit within the grace period, terminate and reap the owned process tree. Do not conceal slow shutdown through increased arbitrary delays.
>
> Windows transport decision
>
> Retain npipe-async as the preferred Windows G1 candidate.
>
> The latest G0 report indicates that the previous cancellation delay was diagnosed and corrected. Preserve the associated regression tests, including the additional bridge shutdown and empty-buffer defects discovered during testing.
>
> Keep npipe-dual and AF_UNIX as evaluated alternatives. Neither becomes an automatic production fallback.
>
> The production Windows transport remains conditional on completing all required integration, desktop platform, security, packaging, and signed-execution checks.
>
> Authorized next actions
>
> 1. Record these five decisions in the G1 design and decision register.
> 2. Complete remaining G0 feasibility and platform evidence within the previously authorized scope.
> 3. Prepare the G1 implementation plan and dependency review for separate approval.
> 4. Keep production G1 code, production dependency adoption, and release integration outside the existing authorization until explicitly approved.
> 5. Preserve all accepted ADR-010 architectural requirements and test expectations. Submit an Architecture Change Request rather than weakening them.
>
> Report the remaining BLOCKED items, resource requirements, and any decisions requiring my further approval.

## Summary

| ID | Disposition | What it binds |
|---|---|---|
| G1-Q1 | **APPROVED** | One explicitly selected, validated transport per platform and release. No silent fallback to another native transport, loopback TCP or a stdio business protocol. Changing transport needs an evidence-backed decision |
| G1-Q2 | **CONDITIONALLY APPROVED** | Process groups and deterministic termination for the trusted bundled worker in G1. The `setsid` escape is a documented residual. No claim of full containment or sandboxing. Linux cgroup v2 and stronger controls are evaluated in later security work. Third-party workers need their own sandbox and containment gates |
| G1-Q3 | **APPROVED DIRECTION** | Prefer JDK NIO Unix-domain sockets on Linux and macOS. Final selection needs equivalent correctness, security and acceptable performance against Netty epoll/kqueue on target hardware |
| G1-Q4 | **APPROVED TEST POLICY; RESOURCES PENDING** | Windows 11 x64 desktop validation before any broad Windows desktop claim. Windows 11 arm64 wherever arm64 is advertised. Windows 10 x64 if it stays in the support matrix. Unavailable coverage is BLOCKED, and no supported platform is silently removed. Controlled test-signing for development; production signing needs separately authorized credentials. No private keys anywhere in the repository, fixtures, logs or chat |
| G1-Q5 | **APPROVED DIRECTION** | A bounded grace period, provisional minimum 1 s. The final value comes from p99 on approved hardware, with an escalation policy and a maximum shutdown bound. Past the grace period, terminate and reap the owned tree. Never hide slow shutdown behind longer delays |
| Windows | **Retained** | `npipe-async` preferred. The regression tests for W2, B8 and B9 are kept. `npipe-dual` and AF_UNIX are evaluated alternatives, never automatic fallbacks. Production stays conditional on the integration, desktop, security, packaging and signed-execution checks |

## What this record does not do

- It does not approve G1 production code, production dependencies or
  release integration.
- It does not change OWN-02's order.
- It does not relax any test ID, acceptance criterion or invariant. A
  conflict goes to an Architecture Change Request.
