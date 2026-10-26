<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 transport matrix

**Status:** G0 evidence, updated 2026-10-26. Spike code only: these results
are not G1 gate passes. Each row names the commit and evidence file it
comes from. An unrun target is **BLOCKED**, never PASS.\
**Details:** [G0 Windows transport report](../testing/adr-010-g0-windows-transport-report.md)
(latest) and [G0 evidence report](../testing/adr-010-g0-evidence.md) (`7d67cae`).
**Harness:** [`spikes/adr010-g0-transport`](../../spikes/adr010-g0-transport/README.md).

Every PASS row means a real grpc-go server and a real grpc-java client
exchanged a bidirectional HTTP/2 stream over private local IPC, under
these conditions:

- with no TCP socket;
- after a one-shot stdin bootstrap;
- with the credential absent from the worker's command line and environment;
- with the peer-process check enforced;
- on a run whose detector self-check proved the leak and TCP detectors
  can see what they look for.

## Results at `473708f` (latest)

Since `7d67cae`, the suite has gained abnormal-ending, resource,
concurrency, flood and repeated-connection scenarios, run on every
transport. The worker now runs on a jlink image. The
[Windows transport report](../testing/adr-010-g0-windows-transport-report.md)
covers the fixes behind these results.

| OS / arch | Version tested | Transport | Result | Evidence |
|---|---|---|---|---|
| Linux amd64 | Ubuntu 26.04.1, kernel 7.0 (CI) | UDS epoll, UDS NIO | **PASS** 43/43; offline (empty network namespace) 9/9 | `ci-473708f-linux-amd64*.jsonl` |
| Linux arm64 | Ubuntu 24.04.5 (CI) | UDS epoll, UDS NIO | **PASS** 43/43; offline 9/9 | `ci-473708f-linux-arm64*.jsonl` |
| macOS arm64 | 15.7.9 (CI) | UDS kqueue, UDS NIO | **PASS** 43/43 | `ci-473708f-darwin-arm64.jsonl` |
| macOS Intel | 15.7.9 (CI) | UDS kqueue, UDS NIO | **PASS** 43/43 | `ci-473708f-darwin-amd64.jsonl` |
| Windows amd64 | Server 2025, build 26100 (CI) | AF_UNIX (NIO), `npipe-dual`, `npipe-async` | **PASS**: every scenario on all three | `ci-473708f-windows-amd64.jsonl` |
| Windows arm64 | Windows 11, build 26200 (CI) | AF_UNIX (NIO), `npipe-dual`, `npipe-async` | **PASS**: every scenario on all three | `ci-473708f-windows-arm64.jsonl` |
| Windows, both | as above | `npipe` (one synchronous handle) | **FAIL**: deadlock; 16 dependents BLOCKED. Not a candidate | same |
| Windows 10 / 11 x64 desktop | — | — | **BLOCKED** (not run) | — |
| Packaged, signed app; offline jlink on Windows and macOS | — | — | **BLOCKED** (not run) | — |

Cancel to exit, p50 / p95 ms, 30 runs, `473708f`:

| Runner | AF_UNIX | `npipe-dual` | `npipe-async` | Old overlapped behavior (control) |
|---|---|---|---|---|
| Windows Server 2025 x64 | 47 / 63 | 33 / 56 | 37 / 58 | 501 / 524 |
| Windows 11 arm64 | 30 / 56 | 32 / 47 | 29 / 33 | 525 / 537 |
| Linux x64 (epoll / NIO) | 26 / 39 · 29 / 37 | — | — | — |

## Results at `7d67cae` (history)

| OS / arch | Version tested | Transport | Result | Test IDs | Evidence |
|---|---|---|---|---|---|
| Linux amd64 | Ubuntu 26.04.1, kernel 7.0 (CI) | UDS, Netty native epoll | **PASS** 17/17 | TPT-001, TPT-005, TPT-022 | `ci-7d67cae-linux-amd64.jsonl` |
| Linux amd64 | Ubuntu 26.04.1 (CI) | UDS, Netty NIO (JDK) | **PASS** 9/9 | TPT-001 | same |
| Linux amd64 | Ubuntu 24.04.5 container | Both of the above | **PASS** 26/26 | TPT-001 | `linux-amd64-container.jsonl` |
| Linux arm64 | — | — | **BLOCKED** (not run) | TPT-001 | — |
| macOS arm64 | macOS 15.7.9 (CI) | UDS, Netty native kqueue | **PASS** 17/17 | TPT-002, TPT-022 | `ci-7d67cae-darwin-arm64.jsonl` |
| macOS arm64 | macOS 15.7.9 (CI) | UDS, Netty NIO (JDK) | **PASS** 9/9 | TPT-002 | same |
| macOS Intel | macOS 15.7.9 (CI) | UDS, kqueue and NIO | **PASS** 26/26 | TPT-002 | `ci-7d67cae-darwin-amd64.jsonl` |
| Windows amd64 | Server 2025, build 26100 (CI) | AF_UNIX, Netty NIO (JDK) | **PASS** 17/17 | TPT-004, TPT-022 | `ci-7d67cae-windows-amd64.jsonl` |
| Windows amd64 | Server 2025 (CI) | Named pipe, one synchronous handle (`npipe`) | **FAIL**: deadlock; 8 dependent scenarios BLOCKED | TPT-003 | same |
| Windows amd64 | Server 2025 (CI) | Named pipe, two synchronous connections (`npipe-dual`) | **PASS** 9/9 | TPT-003 | same |
| Windows amd64 | Server 2025 (CI) | Named pipe, one overlapped connection (`npipe-async`) | **PASS** 9/9; cancel-to-exit ~500 ms, unexplained | TPT-003 | same |
| Windows 10 / 11 desktop | — | — | **BLOCKED** (not run) | TPT-003, TPT-004 | — |
| Windows arm64 | — | — | **BLOCKED** (not run) | TPT-003, TPT-004 | — |
| Any OS, packaged and signed app, `jlink` image offline | — | — | **BLOCKED** (not run) | TPT-002 (signed-app clause), plan §2.2 | — |

## Key numbers (p50 / p95)

CI runners at `7d67cae`. Cold start and cancel are 30 samples each; ping
round trips are 200 per transport.

| Platform / transport | Cold start ms | Ping RTT µs | Duplex MiB/s | Cancel to exit ms |
|---|---|---|---|---|
| Linux / epoll | 467 / 479 | 412 / 637 | 100 | 33 / 41 |
| Linux / NIO | 483 / 495 | 459 / 683 | 99 | 31 / 39 |
| macOS arm64 / kqueue | 640 / 950 | 697 / 1,333 | 63 | 23 / 112 |
| macOS arm64 / NIO | 494 / 635 | 844 / 1,903 | 74 | 63 / 105 |
| macOS Intel / kqueue | 1,994 / 2,733 | 1,350 / 3,023 | 13 | 79 / 269 |
| macOS Intel / NIO | 1,982 / 2,877 | 2,422 / 11,482 | 14 | 74 / 247 |
| Windows / AF_UNIX (NIO) | 830 / 846 | 711 / 1,219 | 63 | 38 / 56 |
| Windows / `npipe-dual` | 802 / 828 | 956 / 1,494 | 66 | 40 / 59 |
| Windows / `npipe-async` | 819 / 1,027 | 1,123 / 1,580 | 57 | **500 / 524** |

Hosted macOS Intel runners are slow and noisy: the same commit range ran
about twice as fast on the earlier run (see the evidence report, §5).
None of these runners is an owner-approved hardware profile.

## What this means for OWN-02

OWN-02 makes restricted named pipes the primary Windows transport and
accepts proven AF_UNIX. Both now have a working candidate on Windows Server
2025 amd64. The single synchronous pipe does not work. Choosing the Windows
transport for G1 is an owner decision; see the evidence report, §7.

**Update, 2026-10-26.** The maintainer chose `npipe-async` as the preferred
G1 candidate, conditionally
([decision](../review/ADR-010-G0-windows-transport-decision.md)). Its exit
delay is fixed. At `473708f` it passes every scenario on Server 2025 x64
and Windows 11 arm64. The priority is unchanged. The transport is not
final: Windows 10 and 11 x64 desktop, signed execution and offline packaged
runs remain BLOCKED.
