<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G0 — Maintainer decision on the Windows transport

**Date:** 2026-10-25. **Given by:** the maintainer, in the session
conversation, in response to the
[G0 evidence report](../testing/adr-010-g0-evidence.md) at `c6f47bf`.\
**Effect:** Conditional. It names a preferred G1 candidate and authorizes
more G0 diagnostics and transport-neutral G1 design work. It is **not**
production acceptance of any Windows transport, and it does not change the
transport priority in OWN-02.\
**Binding on:** [ADR-010](../design/ADR-010-plugin-architecture.md) D-03
and OWN-02/OWN-03, the
[decision record of 2026-10-24](ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md),
and the [test strategy](../testing/adr-010-test-strategy.md).

## Decision text (verbatim)

> Maintainer Decision — ADR-010 G0 Windows Transport
>
> I accept the G0 evidence that native Go↔Java gRPC over Windows named pipes is technically feasible on the tested Windows Server 2025 amd64 environment.
>
> My architectural decision is:
>
> 1. Single overlapped Windows named pipe (npipe-async) is the preferred G1 implementation candidate. This is conditional approval, not production acceptance.
> 2. Diagnose the repeatable approximately 500 ms cancellation-to-exit delay. Identify the actual cause using instrumented measurements, not assumptions.
> 3. Correct the cause where possible without introducing fragile workarounds, unbounded waits, busy polling, or weakening deterministic cancellation.
> 4. Inspect and correct the Java PipeBridge implementation for event-loop blocking, outbound queue saturation, incomplete shutdown signaling, and pending asynchronous I/O cleanup.
> 5. Preserve npipe-dual as a tested alternative, but evaluate and address its connection-pairing timeout and denial-of-service risks before treating it as production-ready.
> 6. Retain Windows AF_UNIX as a verified alternative. Do not change the approved transport priority without my explicit acceptance of a documented architectural change.
> 7. Test cancellation, process-tree cleanup, peer identity, credential protection, stream backpressure, concurrent connections, resource exhaustion, and abnormal JVM termination.
> 8. Validate Windows 10/11, offline jlink packaging, and signed application execution before making general Windows compatibility or release claims.
> 9. Keep all worker business data on native gRPC. No custom protobuf-over-stdio protocol or automatic TCP fallback is authorized.
> 10. Do not weaken ADR-010, revise test expectations to conceal failures, or substitute a lower-quality architecture to accelerate implementation.
>
> Authorization: Continue G0 diagnostics and transport-neutral G1 design work within the existing approved scope. Do not finalize the production Windows transport until the unresolved issues have documented fixes and passing regression tests.
>
> Provide a comparative report containing root cause, code changes, test results, resource measurements, cancellation latency distributions, residual risks, and your final transport recommendation.
>
> Preserve all existing acceptance criteria and report unresolved conditions as BLOCKED rather than PASS.

## What this record does not do

- It does not approve G1 production code, production dependencies or a
  production Windows transport.
- It does not change OWN-02's order: named pipes stay primary and AF_UNIX
  stays the accepted alternative.
- It does not relax any test ID, acceptance criterion or invariant.
