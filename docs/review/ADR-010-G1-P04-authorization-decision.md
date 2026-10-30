<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 / P04 — Maintainer authorization

**Date:** 2026-10-26.

**Given by:** the maintainer, in the session conversation, in response to:

- the [G1 implementation plan](../design/adr-010-g1-implementation-plan.md) §8, decisions 1 to 8;
- the [G1 dependency review](../interoperability/adr-010-g1-dependency-review.md).

**Effect:**

- **Authorized:** bounded G1 production engineering, as slices G1-1 to G1-9
  in their documented order. Also the OWN-11 dependency work, under the
  conditions below.
- **Not authorized:** MPXJ import, project database migrations, broader
  architectural changes, or claims of release readiness.

**Binding on:**

- [ADR-010](../design/ADR-010-plugin-architecture.md);
- the [G1 transport design](../design/adr-010-g1-transport-design.md);
- the G1 implementation plan;
- the dependency review;
- the [test strategy](../testing/adr-010-test-strategy.md) (TPT-002, TPT-007, TPT-009, TPT-018);
- OWN-10 and OWN-11.

## Decision text (verbatim)

> GoPMgr ADR-010 — Maintainer Decisions for G1/P04
>
> I approve proceeding with P04/G1 implementation and the OWN-11 dependency work, subject to the eight decisions below and the existing accepted ADR-010 quality, security, compatibility, and test requirements.
>
> This authorizes bounded G1 production engineering, not MPXJ import implementation, project database migrations, broader architectural changes, or release readiness claims.
>
> Decision 1 — P04 authorization: APPROVED
>
> Authorize the nine G1 implementation slices, G1-1 through G1-9, in the documented dependency order.
>
> Keep changes in small, independently reviewable pull requests. Each must pass its applicable tests and security review before dependent work is merged. Do not combine slices merely to accelerate delivery.
>
> Decision 2 — OWN-11 dependencies: CONDITIONALLY APPROVED
>
> Approve the documented gRPC, Protocol Buffers, and Windows named-pipe dependencies, subject to successful security, licensing, compatibility, and reproducible-build verification.
>
> Retain patched golang.org/x/net and golang.org/x/text versions. Review the complete dependency graph and record all additions in docs/DEPENDENCIES.md.
>
> Prefer JDK NIO for Linux/macOS and minimize native-library dependencies. Use the approved Java LTS runtime and pinned, verified build tooling.
>
> Decision 3 — Windows admission security: APPROVED
>
> Make both protections mandatory:
>
> * Limit named-pipe instances, enforce first-instance ownership, restrict remote access, and validate operating-system peer identity.
> * Implement mutual authentication. The Go host must prove its identity to the Java worker before the worker accepts operational commands, in addition to the worker proving itself to GoPMgr.
>
> Test against same-user malicious processes, pipe impersonation, unauthorized second instances, replay, and privileged/unprivileged process interactions.
>
> Do not accept a design where a successful worker-to-host authentication masks an unauthenticated host-to-worker channel.
>
> Decision 4 — Shutdown and performance bounds: PROVISIONALLY APPROVED
>
> Approve provisional bounds G = 1 second, K = 1 second, and M = 2 seconds for engineering and measurement.
>
> Finalize these only after representative hardware testing and review of p99 latency distributions. A process that cannot be reaped must remain an explicit unhealthy condition; do not misrepresent M as a guaranteed operating-system termination time.
>
> Approve the proposed 20% performance-comparison threshold provisionally. Existing measurements that exceed it must be investigated, not waived or concealed.
>
> Decision 5 — Windows 10 x64: RETAIN SUPPORT
>
> Retain Windows 10 x64 within the existing declared compatibility target pending a separately approved lifecycle or support-policy decision.
>
> Do not remove a previously supported operating system merely because appropriate testing infrastructure is unavailable.
>
> Unexecuted Windows 10 tests remain BLOCKED. Any future change to the support matrix requires an explicit decision.
>
> Decision 6 — Test hardware and signing resources: PENDING AVAILABILITY
>
> Use available hosted runners, appropriately licensed virtual machines, or self-hosted systems for Windows 10/11 x64 and macOS minimum-version verification.
>
> Provide a specific resource acquisition and validation plan for outstanding targets.
>
> Use properly secured signing services and authorized test credentials for Developer ID, notarization, and Authenticode checks.
>
> Do not place signing keys or secrets in repositories, prompts, logs, or fixtures.
>
> Do not report unavailable testing as PASS. Production compatibility claims remain blocked until representative testing succeeds.
>
> Decision 7 — TPT-002 clarification: APPROVED
>
> Replace the implementation-specific phrase “native Netty runtime” with “selected UDS runtime.”
>
> This is a clarification only. Preserve every original security, signing, installed-application, runtime, and cross-platform requirement.
>
> Do not reduce acceptance coverage because the preferred Java transport changes from native Netty epoll/kqueue to JDK NIO.
>
> Decision 8 — Replay protection testing: APPROVED
>
> Separate verification into two independently required tests:
>
> 1. Transport admission: The production single-instance named-pipe listener refuses unauthorized additional connections.
> 2. Protocol authentication: A controlled test listener permits multiple connections solely to exercise and verify rejection of replayed authentication credentials.
>
> Both tests are mandatory. Neither substitutes for the other.
>
> Preserve existing negative tests and document why the production listener rejects a connection before that request reaches the protocol layer.
>
> Mandatory architectural safeguards
>
> * Native gRPC over local IPC remains required.
> * Windows npipe-async remains the preferred transport; no unapproved TCP or stdio-data fallback.
> * The minimal, language-neutral internal plugin capability registry remains mandatory.
> * GoPMgr retains exclusive authority over encryption, authentication, scheduling, audit, and project persistence.
> * No test may be weakened, deleted, or reclassified to conceal a defect.
> * Any requirement conflict or proposed architectural downgrade requires my explicit approval through an Architecture Change Request.
>
> Execution instructions
>
> Proceed with G1-1 and continue through the authorized slices as their gates permit. Maintain a separate, explicit BLOCKED list for hardware, signing, and platform tests.
>
> Before claiming G1 completion, provide a consolidated evidence report identifying implemented contracts, dependency/security review, tests actually executed, platform coverage, measured performance, unresolved defects, release readiness, and remaining approval requirements.
>
> My approval authorizes implementation within the accepted architecture. It does not authorize reducing the architecture’s quality or misrepresenting verification results.

## Summary

| ID | Disposition | What it binds |
|---|---|---|
| P04-D1 | **APPROVED** | Slices G1-1 to G1-9 in dependency order. Small PRs. Each passes its tests and security review before dependent work merges. Slices are not combined to go faster |
| P04-D2 | **CONDITIONALLY APPROVED** | grpc-go, protobuf-go and go-winio, subject to security, licensing, compatibility and reproducible-build verification. Patched x/net and x/text kept. The whole dependency graph reviewed and recorded in `docs/DEPENDENCIES.md`. JDK NIO preferred; few or no native libraries. The approved Java LTS runtime and pinned, verified tooling |
| P04-D3 | **APPROVED, mandatory** | Design §3.3 rules 6 (pipe instance limit, first instance, remote rejection, OS peer identity) and 7 (mutual authentication: the host proves itself before the worker accepts operational commands). Tests against same-user malicious processes, pipe impersonation, second instances, replay, and privileged and unprivileged interactions |
| P04-D4 | **PROVISIONALLY APPROVED** | G = 1 s, K = 1 s, M = 2 s, for engineering and measurement only. An unreaped process stays an explicit unhealthy state, and M is not an OS termination guarantee. The 20 % comparison threshold is provisional, and any measurement above it is investigated |
| P04-D5 | **RETAIN SUPPORT** | Windows 10 x64 stays in the declared target. Its unrun tests stay BLOCKED. The support matrix changes only by explicit decision |
| P04-D6 | **PENDING AVAILABILITY** | Hosted runners, licensed VMs or self-hosted systems for Windows 10 and 11 x64 and the macOS minimum. A resource acquisition and validation plan ([BLOCKED list](../testing/adr-010-g1-blocked.md)). Secured signing services and authorized test credentials only. No key or secret in repositories, prompts, logs or fixtures |
| P04-D7 | **APPROVED** | TPT-002: "native Netty runtime" becomes "selected UDS runtime". Clarification only; no coverage reduced |
| P04-D8 | **APPROVED** | Two required tests: the production single-instance listener refuses extra connections, and a controlled multi-connection test listener shows replayed credentials are rejected. Existing negative tests are kept |

## What this record does not do

- It does not authorize MPXJ import, project database migrations, broader
  architectural changes, or release-readiness claims.
- It does not change OWN-02's order, OWN-03, D-03 or the registry
  requirement.
- It does not relax any test ID, acceptance criterion or invariant. A
  conflict goes to an Architecture Change Request.
