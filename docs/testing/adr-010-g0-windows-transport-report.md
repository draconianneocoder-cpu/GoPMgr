<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G0 — Windows transport comparative report

**Status:** G0 diagnostics for owner review, 2026-10-26. Written under the
[maintainer's G0 Windows transport decision](../review/ADR-010-G0-windows-transport-decision.md).
This is spike code, not G1 production code. **The production Windows
transport is not finalized.**\
**Evidence:** `spikes/adr010-g0-transport/evidence/`.

- Baseline: `31e7066`.
- Rounds since: `d4e392b`, `70a304c`, `da6d459`.
- Final run: `473708f`. All six CI runners plus a cloud container.

**Related:** [G1 transport design (proposed)](../design/adr-010-g1-transport-design.md),
[transport matrix](../interoperability/transport-matrix.md),
[G0 evidence report](adr-010-g0-evidence.md).

## 1. Outcome

1. **The ≈ 500 ms cancel-to-exit delay is found and fixed (§3).**
   - **Cause:** the JDK's default I/O completion-port threads wait in native
     code forever. At process exit, HotSpot's `VM_Exit` waits up to 30 ticks
     of 10 ms for threads in native code, and at Windows timer granularity
     that is ≈ 480 ms.
   - **Fix:** each overlapped pipe channel gets its own executor, shut down
     with the channel.
   - **Result** (cancel to exit p50, 30 runs; the old behavior runs as a
     control in the same job):

     | Runner | Fixed | Old behavior |
     |---|---|---|
     | Windows Server 2025 x64 | 37 ms | 501 ms |
     | Windows 11 arm64 | 29 ms | 525 ms |
2. **The pipe bridge is corrected (§4)** for event-loop blocking, queue
   saturation, shutdown signaling and pending-I/O cleanup (B1 to B7). The
   Windows runs then exposed two more defects in my own rewrite, both fixed
   and covered by regression tests:
   - **B8:** a close race that killed a JDK thread;
   - **B9:** an empty gRPC buffer that intermittently closed new
     connections.
3. **The `npipe-dual` pairing denial-of-service is fixed (§5).**
   - Each half's peer is checked when it is accepted.
   - A lone half is dropped after 3 s.
   - Connections are refused until the worker's PID is known.
4. **Final run, `473708f`:** every required test area passes on every
   Windows transport, on both Windows runners (§8). The only failures are
   the known single-synchronous-handle deadlock (`npipe`) and the tests
   BLOCKED behind it. `npipe` is not a candidate.
5. **Still BLOCKED (§11):** Windows 10 and 11 **x64** desktop, signed and
   installed application execution, and owner hardware profiles.
6. **Recommendation (§12):** keep `npipe-async` as the preferred G1
   candidate and the approved priority unchanged. It is not production-ready
   until the §11 BLOCKED items pass.

## 2. What ran

CI uses a pinned, checksum-verified Temurin 21.0.12.1+1. The worker runs on
a **jlink image** of nine modules (65 MB on Linux x64), not the full JDK.

| Runner | OS build | Arch | Transports | Final evidence |
|---|---|---|---|---|
| windows-2025 | Windows Server 2025, 10.0.26100 | x64 | AF_UNIX (NIO), `npipe`, `npipe-dual`, `npipe-async`; control `npipe-async-default` | `ci-473708f-windows-amd64.jsonl` |
| windows-11-arm | Windows 11, 10.0.26200 | arm64 | same | `ci-473708f-windows-arm64.jsonl` |
| ubuntu-26.04 | Ubuntu 26.04.1, kernel 7.0 | x64 | UDS epoll, UDS NIO | `ci-473708f-linux-amd64.jsonl`, `…-offline.jsonl` |
| ubuntu-24.04-arm | Ubuntu 24.04.5, kernel 6.17 | arm64 | same | `ci-473708f-linux-arm64.jsonl`, `…-offline.jsonl` |
| macos-15 | macOS 15.7.9 | arm64 | UDS kqueue, UDS NIO | `ci-473708f-darwin-arm64.jsonl` |
| macos-15-intel | macOS 15.7.9 | x64 | same | `ci-473708f-darwin-amd64.jsonl` |
| Cloud container | Ubuntu 24.04.5, kernel 6.18 | x64 | same as Linux | `container-473708f-linux-amd64*.jsonl` |

None of these is an owner-approved hardware profile (OWN-10). The
maintainer dropped macOS Intel from CI on 2026-10-26. Its results here are
its last.

## 3. Root cause of the ≈ 500 ms exit delay (finding W2)

### 3.1 Measurement

At `31e7066`, the worker recorded a monotonic mark at each shutdown step,
and the host timed its own side. Windows Server 2025 x64, `npipe-async`,
30 runs, p50 / p95 ms:

| Phase | Clock | Time |
|---|---|---|
| Host cancel → host sees the connection close | host | 18.85 / 38.76 |
| Stream error → `System.exit` called | worker | 13.75 / 22.94 |
| Stream error → last shutdown hook done | worker | 17.35 / 27.52 |
| Connection close → process exit | host | **473.88 / 499.68** |

The same phase took 26.41 ms on `npipe-dual` and 16.61 ms on AF_UNIX.

The worker's own shutdown finished in under 30 ms. Nearly all the delay
came after its last shutdown hook, inside JVM teardown.

### 3.2 Mechanism (JDK source)

**HotSpot `VM_Exit::wait_for_threads_in_native_to_block()`** (JDK 21u,
`src/hotspot/share/runtime/vmOperations.cpp`):

- At the final safepoint, it loops while any other Java thread is
  `_thread_in_native`.
- It allows user threads `max_wait_user_thread = 30` attempts ("at least
  300 milliseconds"), each `ml.wait(10)`.
- A 10 ms wait lasts about one timer tick, nominally 15.6 ms on Windows.
  31 waits come to ≈ 480 ms, which is consistent with the measured 474 ms
  p50.

**`WindowsAsynchronousFileChannelImpl.open`:**

- Without an executor, it uses `DefaultIocpHolder.defaultIocp`, a
  process-wide completion port. Its threads wait in
  `GetQueuedCompletionStatus` for the life of the process, and nothing
  closes the port.
- With an executor, it creates `new Iocp(null, pool)`. On close,
  `iocp.detachFromThreadPool()` wakes that port's thread, which leaves
  native code and ends.

### 3.3 Fix

- Open the pipe with
  `AsynchronousFileChannel.open(path, {READ, WRITE}, executor)`, using a
  one-thread daemon executor that the bridge owns.
- On close, shut that executor down only after every pending operation has
  completed (B8, §4).
- No timeout was shortened, no thread is interrupted, and no JVM flag or
  `Runtime.halt` is used. The JVM still exits through its normal path,
  shutdown hooks included.
- The old behavior stays in every run as `npipe-async-default`, a
  diagnostic control.

### 3.4 Verification (final run `473708f`, same job, 30 cancels each)

| Runner | Transport | Cancel → exit p50 / p95 / p99 ms | Connection close → exit p50 / p95 ms | Runs with a thread in a native wait at exit |
|---|---|---|---|---|
| Server 2025 x64 | `npipe-async` (fixed) | 36.69 / 58.21 / 63.9 | 17.9 / 29.49 | 0 / 30 |
| Server 2025 x64 | `npipe-async-default` (control) | 500.56 / 524.07 / 530.73 | 481 / 507.54 | **30 / 30**, all in `sun.nio.ch.Iocp.getQueuedCompletionStatus` |
| Windows 11 arm64 | `npipe-async` (fixed) | 28.68 / 33.04 / 36.4 | 16.6 / 18.68 | 0 / 30 |
| Windows 11 arm64 | `npipe-async-default` (control) | 524.88 / 537.33 / 537.69 | 511.01 / 523.16 | **30 / 30**, same frame |

The fix held in all four rounds on both runners. The fixed transport's
p50 ranged from 23 to 37 ms; the control's, from 482 to 533 ms. One
round's x64 control has no figure: a handshake failed (B9).

**How the "native wait at exit" list works.** Just before `System.exit`,
the worker lists threads whose top stack frame is a native method. It
leaves out the VM's own waits (`Object.wait0`, `Unsafe.park`,
`Thread.sleep`). The list is a sample, so a thread caught for an instant
inside a native call can appear:

- `Thread.start0`, in 3 of 30 runs per transport on macOS Intel at
  `473708f`;
- an `Unsafe` CAS in earlier rounds.

None of those runs exited slowly. The signal is a thread *blocked* in a
native wait, as in all 30 control runs.

## 4. Pipe bridge review and corrections

The bridge (`worker/.../PipeBridge.java`) connects a Windows pipe to
grpc-java through an in-JVM `LocalChannel`.

- **B1 to B7** are defects in the `31e7066` code, found by review.
- **B8 and B9** were in my own rewrite. The Windows runs found them.

| ID | Defect | Effect | Fix | Regression test |
|---|---|---|---|---|
| B1 | `channelRead` called `outbound.put()` on the Netty event loop (queue of 256) | A slow pipe stalled the event loop, and every gRPC frame on it | Event-loop-confined write queue. `autoRead` off above 1 MiB queued, on below 256 KiB; HTTP/2 flow control holds the sender | `bridge-selftest` backpressure (peak 1,114,112 B, the bound); `backpressure`; `cancel-backpressure` |
| B2 | The pipe opened inside `channelActive`, on the event loop, retrying with `Thread.sleep(20)` | The event loop blocked for up to 5 s | The pipe opens before gRPC connects, with bounded backoff (5 ms to 200 ms, 5 s in total) | Every handshake; `connect-flood`; `connect-repeat` |
| B3 | Shutdown sent a `STOP` sentinel with `offer()`, dropped when the queue was full | The writer could stay blocked after close | No sentinel. `close()` is explicit, idempotent and bounded (2 s): child, then server, pipe and event loop | `bridge-selftest` close-with-pending-io |
| B4 | Each gRPC child opened the pipe again and overwrote the shared pipe and queue | A second connection corrupted the first | The bridge claims one child and closes any second one | `bridge-selftest` second-child-refused |
| B5 | Async mode used `Future.get()` per operation on dedicated threads; pending I/O was never cleaned up | Threads blocked, and operations were left pending at close | `CompletionHandler` I/O. Buffers are released on completion or failure, even after close | `bridge-selftest` close-with-pending-io (0 unreleased) |
| B6 | I/O errors were caught and ignored | Failures went silent | Every failure closes the bridge, is counted, and its exception is recorded (`g0-note`) | Every scenario records them |
| B7 | The reader called `writeAndFlush` toward gRPC without waiting | Buffering toward gRPC had no bound | One read in flight; the next is issued when gRPC has taken the last one | `duplex`; `bridge-selftest` integrity (16 MiB each way) |
| B8 | **Mine (`7aab68c`):** `AsyncDuplex.close()` shut its executor down right after closing the channel. The JDK hands aborted operations' failures to that executor from its completion-port thread | `Exception in thread "Thread-0" java.nio.channels.ShutdownChannelGroupException` in every `npipe-async` run on both runners at `d4e392b`. The handler never ran, so a buffer still being written would not have been released | `close()` counts operations and waits, bounded (1 s) and woken by the last completion, until all have completed; only then does it shut the executor down (`70a304c`) | New check in every scenario: an uncaught worker exception FAILs the run. Zero since `70a304c` |
| B9 | **Mine (`7aab68c`):** the bridge passed an empty buffer to the overlapped write. The `31e7066` loop had written only while bytes remained | grpc-netty writes `Unpooled.EMPTY_BUFFER` when a client transport starts (`NettyClientHandler`'s `NOOP_MESSAGE`). `LocalChannel.readInbound` merges it into the next buffer only if both are queued when the bridge reads. Read alone, it reached `AsynchronousFileChannel.write`, which completes a zero-length write with 0 bytes. The bridge read that as "no progress" and closed the connection. Intermittent: the second connection in `replay` failed in 1–3 of 10 runs, and one control handshake failed | The bridge drops empty buffers (`bridge_empty_buffers`), and `AsyncDuplex` completes an empty write without I/O (`473708f`) | `bridge-selftest` empty-buffer: FAILs before the fix, PASSes after, on every OS. On Windows at `473708f`, the empty buffer arrived alone in replay on both runners, was dropped, and the connection succeeded: 10/10 |

**How B9 was found.**

1. `connect-repeat` (50 fresh connections) and the recorded I/O exception
   were added at `da6d459`.
2. Every failed run carried the same note: `bridge write failed:
   java.io.IOException: pipe write made no progress`.
3. The cause was confirmed in grpc-netty 1.85.1's and Netty 4.2.17's
   bytecode (`javap`) and in the JDK's `implWrite`.
4. `npipe-dual` never failed this way: a blocking zero-length write simply
   succeeds.

## 5. `npipe-dual` pairing: denial-of-service analysis and fix

`npipe-dual` needs two pipe connections from the worker, one per
direction, which the host pairs.

| ID | Risk at `31e7066` | Who can trigger it | Fix (`host/pairing.go`) |
|---|---|---|---|
| D1 | The peer check ran only after pairing. A foreign connection arriving between the worker's two halves was paired with one of them, and the pair was then refused | Any process of the same user that can open the pipe | Each half's OS peer PID is checked as it is accepted |
| D2 | No pairing deadline: a lone half held the pairing slot indefinitely | Same | A half without a partner after 3 s is dropped and counted (`pair_timeouts`) |
| D3 | Before the worker's PID was known, the expected PID was 0, and a check against 0 could match | Same, during startup | Every connection is refused until the PID is set, in both listeners |

A single accept goroutine with an explicit shutdown replaced the old
pairing loop.

**Unit tests** (`host/pairing_test.go`, 50 runs with `-race`):

- two halves pair;
- a foreign first half is refused;
- a foreign connection between the halves is refused;
- a stalled half times out, and the next pair succeeds;
- `Close` while waiting for a partner;
- refusal before the worker starts.

**On Windows at `473708f`:** a process other than the worker made 50
two-half dials while the worker started.

| Runner | Dials with both halves connected | Halves refused | Worker's session |
|---|---|---|---|
| Server 2025 x64 | 28 | 66 | Succeeded |
| Windows 11 arm64 | 14 | 41 | Succeeded |

The other dials found the pipe busy. Halves of dials whose second half
failed are also refused, which is why refusals exceed 2 × the connected
dials.

**Residual: availability.**

- A same-user process can keep the pipe busy, so the worker's open fails
  after its 5 s bound. That is reported as `PLUGIN_START_FAILED`.
- This applies to every transport: a same-user process can also kill the
  worker.
- Confidentiality and integrity hold, through the peer check and the
  single-use proof.

## 6. Other changes in this round

**Process trees:**

- **Windows:** the worker starts suspended, is assigned to a Job Object
  with kill-on-close, and is then resumed. The launch fails if the
  assignment fails.
- **Unix:** the worker leads its own process group, and the host kills
  the group.

**Abnormal endings:**

- The worker runs with `-XX:+ExitOnOutOfMemoryError`.
- Exit codes are distinct: 0, 20 to 24, 70; the JVM uses 3 for OOM.

**Scenarios added:**

- `cancel-backpressure`, `oversize`, `concurrent`, `connect-flood`,
  `connect-repeat`;
- `tree-kill`, `crash` and `oom`, on every transport;
- `replay`, repeated 10 times;
- `bridge-selftest`, with five tests.

**Checks:**

- **Uncaught exceptions:** any uncaught worker exception FAILs the run.
- **Impersonation:** `impersonate` counts refusals at both admission
  layers. It falsely FAILed at `d4e392b`, because the pairing layer refused
  the impersonator before the peer listener saw it.
- **Failure diagnostics:** the worker records the exception behind any
  bridge I/O failure. The host keeps grpc-go's own log and attaches the
  matching lines to a failed session.

**Packaging:**

- a pinned JDK and Maven, with the publishers' checksums;
- the worker runs on a jlink image;
- the Linux suite also runs inside an empty network namespace
  (`unshare -rn`; only `lo`, and it is down).

## 7. Results by round

| Round | Windows x64 | Windows arm64 | Linux / macOS | What it showed |
|---|---|---|---|---|
| `d4e392b` | 51 PASS, 2 FAIL, 12 BLOCKED | 50 / 3 / 12 | all PASS | W2 fixed. B8 on stderr in every `npipe-async` run. `impersonate` check defect. One `replay` miss on arm64 |
| `70a304c` | 57 / 2 / 15 | 58 / 1 / 15 | all PASS | B8 fixed; `impersonate` check fixed. One control handshake failed (B9, cause not yet known) |
| `da6d459` | 61 / 2 / 16 | 61 / 2 / 16 | all PASS | Diagnostics named B9: `replay` misses 3/10 (x64), 1/10 (arm64); `connect-repeat` 50/50 |
| **`473708f`** | **62 PASS, 1 FAIL, 16 BLOCKED** | **62 / 1 / 16** | **Linux x64 and arm64 43/43 + 9/9 offline; macOS arm64 and Intel 43/43; container 43/43 + 9/9** | B9 fixed. The only FAIL is `npipe` (known deadlock); the 16 BLOCKED are its dependents |

## 8. Required test areas (decision item 7), final run, both Windows runners

| Area | Scenarios | AF_UNIX | `npipe-dual` | `npipe-async` |
|---|---|---|---|---|
| Cancellation | `cancel` (30 runs), `cancel-backpressure` (10) | PASS (4/4) | PASS (4/4) | PASS (4/4) |
| Process-tree cleanup | `tree-kill`, `crash`, `kill` | PASS (6/6) | PASS (6/6) | PASS (6/6) |
| Peer identity | `impersonate`, `connect-flood` | PASS (4/4) | PASS (4/4) | PASS (4/4) |
| Credential protection | `bad-credential`, `replay` (10 runs), `handshake` (leak detector with self-check) | PASS (6/6) | PASS (6/6) | PASS (6/6) |
| Stream backpressure | `backpressure`, `cancel-backpressure`, `duplex` | PASS (6/6) | PASS (6/6) | PASS (6/6) |
| Concurrent connections | `concurrent` (4 sessions) | PASS (2/2) | PASS (2/2) | PASS (2/2) |
| Resource exhaustion | `oversize`, `oom`, `connect-flood` | PASS (6/6) | PASS (6/6) | PASS (6/6) |
| Abnormal JVM termination | `kill`, `crash`, `oom` | PASS (6/6) | PASS (6/6) | PASS (6/6) |
| Repeated connections | `connect-repeat` (50 launches) | PASS (2/2) | PASS (2/2) | PASS (2/2) |
| Bridge internals | `bridge-selftest`, 5 tests, every OS | — | PASS | PASS |
| Windows 10 / 11 x64 desktop | — | **BLOCKED** | **BLOCKED** | **BLOCKED** |
| Signed, installed application | — | **BLOCKED** | **BLOCKED** | **BLOCKED** |
| jlink image, offline | Linux only (`unshare -rn`) | jlink: PASS on all six runners. Offline: PASS on Linux x64 and arm64. **Windows and macOS offline: BLOCKED** | | |

## 9. Cancellation latency distributions (`473708f`, 30 runs each)

| Platform | Transport | Cancel → exit p50 / p95 / p99 / max ms | Cancel → connection close p50 / p95 | Connection close → exit p50 / p95 | Native wait at exit |
|---|---|---|---|---|---|
| windows/amd64 | `uds-nio` (AF_UNIX) | 47 / 62.92 / 63.38 / 63.38 | 15.59 / 42.38 | 29.4 / 43.92 | 0/30 |
| windows/amd64 | `npipe-dual` | 32.61 / 55.99 / 59.52 / 59.52 | 14.75 / 22.8 | 16.88 / 39.81 | 0/30 |
| windows/amd64 | `npipe-async` | 36.69 / 58.21 / 63.9 / 63.9 | 17.44 / 31.02 | 17.9 / 29.49 | 0/30 |
| windows/amd64 | `npipe-async-default` (control) | 500.56 / 524.07 / 530.73 / 530.73 | 16.81 / 23.88 | 481 / 507.54 | 30/30 |
| windows/arm64 | `uds-nio` (AF_UNIX) | 29.95 / 56.48 / 57.47 / 57.47 | 11.2 / 19.2 | 18.32 / 40.97 | 0/30 |
| windows/arm64 | `npipe-dual` | 31.55 / 47.21 / 56.23 / 56.23 | 13.92 / 17.28 | 17.34 / 33.29 | 0/30 |
| windows/arm64 | `npipe-async` | 28.68 / 33.04 / 36.4 / 36.4 | 12.5 / 16 | 16.6 / 18.68 | 0/30 |
| windows/arm64 | `npipe-async-default` (control) | 524.88 / 537.33 / 537.69 / 537.69 | 13.13 / 16.93 | 511.01 / 523.16 | 30/30 |
| linux/amd64 | `uds-epoll` | 25.94 / 39.45 / 41.88 / 41.88 | 11.05 / 24.36 | 14.31 / 25.05 | 0/30 |
| linux/amd64 | `uds-nio` | 29.12 / 37.4 / 47.55 / 47.55 | 9.62 / 21.97 | 13.82 / 25.65 | 0/30 |
| linux/arm64 | `uds-epoll` | 25.35 / 40.22 / 102.36 / 102.36 | 10.98 / 23.81 | 14.64 / 20.41 | 0/30 |
| linux/arm64 | `uds-nio` | 25.12 / 41.61 / 43.53 / 43.53 | 11.18 / 23.34 | 14.03 / 22.65 | 0/30 |
| darwin/arm64 | `uds-kqueue` | 14.12 / 66.13 / 69.14 / 69.14 | 6.27 / 14.05 | 5.41 / 58.18 | 0/30 |
| darwin/arm64 | `uds-nio` | 28.29 / 99.88 / 109.18 / 109.18 | 6.15 / 16.5 | 23.64 / 94.17 | 0/30 |
| darwin/amd64 | `uds-kqueue` | 69.12 / 202.4 / 204.17 / 204.17 | 38.24 / 168.88 | 28.78 / 70.86 | 3/30 (transient `Thread.start0`) |
| darwin/amd64 | `uds-nio` | 60.23 / 132.02 / 136.74 / 136.74 | 19.5 / 39.72 | 31.35 / 104.37 | 3/30 (transient `Thread.start0`) |

Cancel while the host had stopped reading for 1 s and the worker's sends
waited on flow control (10 runs, p50 / p95 / max ms):

| Runner | AF_UNIX | `npipe-dual` | `npipe-async` |
|---|---|---|---|
| Server 2025 x64 | 30.23 / 50.35 / 50.35 | 31.27 / 40.46 / 40.46 | 31.45 / 33.09 / 33.09 |
| Windows 11 arm64 | 25.98 / 45.77 / 45.77 | 28.85 / 30.6 / 30.6 | 25.71 / 39.21 / 39.21 |

With the fix, the three Windows transports sit in the same range as each
other, and within about 20 ms of Linux at p50.

## 10. Resource measurements (`473708f`)

| Platform | Transport | Cold start p50 / p95 ms (`connect-repeat`, n = 50) | Ping RTT p50 / p95 µs (n = 200) | Duplex MiB/s | Worker RSS MiB (duplex) | Heap used MiB | Worker CPU ms (duplex) | 4 concurrent: wall ms / max RSS MiB |
|---|---|---|---|---|---|---|---|---|
| windows/amd64 | `uds-nio` | 865 / 882 | 721 / 1,104 | 63.6 | 139 | 39 | 6,468 | 7,298 / 135 |
| windows/amd64 | `npipe-dual` | 917 / 1,120 | 1,017 / 1,386 | 58.8 | 140 | 42 | 7,125 | 11,646 / 163 |
| windows/amd64 | `npipe-async` | 871 / 1,015 | 1,110 / 1,663 | 59.8 | 159 | 41 | 7,062 | 7,491 / 142 |
| windows/arm64 | `uds-nio` | 784 / 820 | 688 / 1,787 | 90.9 | 141 | 39 | 4,531 | 5,842 / 144 |
| windows/arm64 | `npipe-dual` | 843 / 866 | 1,034 / 1,524 | 88.4 | 144 | 42 | 4,984 | 5,447 / 144 |
| windows/arm64 | `npipe-async` | 772 / 790 | 1,064 / 1,605 | 85.2 | 160 | 41 | 5,093 | 4,951 / 153 |
| linux/amd64 | `uds-epoll` | — | 362 / 618 | 124.7 | 137 | 40 | 3,196 | 2,434 / 129 |
| linux/amd64 | `uds-nio` | — | 400 / 696 | 118.2 | 133 | 39 | 3,307 | 2,485 / 132 |
| linux/arm64 | `uds-epoll` | — | 595 / 999 | 113.1 | 133 | 39 | 3,371 | 2,051 / 125 |
| linux/arm64 | `uds-nio` | — | 617 / 863 | 110.1 | 133 | 39 | 3,465 | 2,336 / 120 |
| darwin/arm64 | `uds-kqueue` | — | 316 / 436 | 119.0 | 121 | 35 | 2,276 | 2,366 / 116 |
| darwin/arm64 | `uds-nio` | — | 307 / 649 | 129.1 | 119 | 35 | 2,237 | 2,415 / 118 |
| darwin/amd64 | `uds-kqueue` | — | 1,054 / 5,457 | 22.5 | 142 | 39 | 8,499 | 5,583 / 122 |
| darwin/amd64 | `uds-nio` | — | 1,129 / 6,752 | 16.0 | 145 | 39 | 9,628 | 4,024 / 121 |

How these were measured:

- **Duplex:** 64 MiB host to worker while 100,000 records flow back.
- **Concurrent:** four sessions of 8 MiB plus 20,000 records each.
- **Worker memory:** RSS stays at 119–160 MiB with `-Xmx128m`, and the
  heap used stays near 40 MiB on every transport.
- **Bridge buffering:** while cancelling under load, the bridge held at
  most 54–107 KB on Windows, far under its 1 MiB high-water mark.

**Hosted-runner variance.** On Server 2025 x64, throughput fell by about
half from `da6d459` on, for AF_UNIX and the pipe alike:

| Transport | Duplex MiB/s at `d4e392b` / `70a304c` | From `da6d459` on |
|---|---|---|
| `npipe-async` | 115 / 119 | 34–60 |
| AF_UNIX | 107 / 114 | ≈ 64 |

Cold start rose with it. From `da6d459` on, that runner also lists a
`vEthernet (nat)` adapter it did not have before, so its VM configuration
changed. Windows 11 arm64 and Linux show no such drop. Because both
transports moved together, it does not separate them; it is recorded here,
not attributed. These are hosted runners, not owner profiles (R6).

## 11. Residual risks

| # | Risk | Severity | Status |
|---|---|---|---|
| R1 | Windows 10 and 11 **x64** desktop not tested. Windows 11 is covered on arm64 (CI) only; Windows 10 not at all | High for any Windows claim | **BLOCKED**: needs owner-provided machines or VMs (design Q4) |
| R2 | Signed, installed application not tested | High for release | **BLOCKED**: needs owner signing identities and the G1 packaging slice |
| R3 | Offline jlink runs only on Linux (network namespace). Windows and macOS offline not shown | Medium | **BLOCKED** for Windows and macOS; the packaging slice should run the installed app with networking disabled |
| R4 | Same-user processes can deny service by flooding or killing the worker (§5) | Low; inherent to same-user local IPC | Accepted residual; reported as `PLUGIN_START_FAILED` |
| R5 | Unix process-group escape via `setsid` | Low: the worker starts no children in normal use | Design Q2 |
| R6 | Hosted-runner timings vary (§10) and are not owner profiles | Medium for the grace-period budget G | Design Q5; re-measure on OWN-10 profiles |
| R7 | The at-exit native-thread list is a stack sample (§3.4) | Low | Decisions rest on latency and on blocked native waits |
| R8 | The bridge is spike code. G1 must re-implement it under BR-1 to BR-8, including BR-6a and BR-7a, and pass the design's §8 tests | — | G1 scope |
| R9 | Notices and OpenJDK license terms for a bundled jlink runtime | Release | Legal/package review (dependency report L6) |
| R10 | B8 and B9 were found only by real Windows runs. The bridge has more JDK and Netty edge cases than an in-memory test can reach | Medium | Kept the stderr exception check, the `g0-note` reasons, `connect-repeat` and the B9 self-test. G1 should run the same matrix on every change to the bridge |

## 12. Transport recommendation

**Keep the approved priority, and take `npipe-async` (one overlapped named
pipe) into G1 as the preferred candidate, still conditionally.**

| | `npipe-async` | `npipe-dual` | AF_UNIX |
|---|---|---|---|
| Final run, both Windows runners | Every required area PASS | Every required area PASS | Every required area PASS |
| Cancel → exit p50 (x64 / arm64) | 37 / 29 ms | 33 / 32 ms | 47 / 30 ms |
| OS objects per session | One pipe connection | Two connections plus host pairing logic | One socket file in a private directory |
| Access control | Explicit user-only DACL; first-instance creation; remote clients rejected | Same | Directory ACLs, no explicit DACL on the socket |
| Defects found and fixed in this round | W2, B8, B9 (all in the bridge) | D1–D3 (pairing) | None |
| Bridge needed in the JVM | Yes, overlapped | Yes, blocking (reader stuck in `ReadFile` until the far end closes) | No: standard Netty NIO |
| OWN-02 position | Primary family (named pipe) | Primary family (named pipe) | Approved alternative |

**Why `npipe-async`:**

- It keeps OWN-02's primary family.
- It needs one OS connection, with the explicit pipe DACL and anti-squat
  protections.
- It no longer has a latency or reliability disadvantage on the evidence.

**What it costs:** a custom bridge. Two of its defects (B8, B9) surfaced
only on real Windows. That bridge is the largest remaining engineering
risk (R10), and the G1 design's BR rules and regression tests exist to
contain it.

**`npipe-dual`** remains the tested alternative. Its pairing risks are
fixed and covered by unit tests and `connect-flood`.

**AF_UNIX** remains the verified alternative. It needs no bridge, which is
its main technical advantage. Making it primary would change OWN-02's
order and needs the maintainer's acceptance of a documented architecture
change. This report does not propose that.

**Not finalized.** Per the decision, the production Windows transport is
not final until:

- R1, R2 and R3 pass on the production implementation;
- the G1 implementation passes the design's §8 regression tests.

## 13. Reproduce

```sh
cd spikes/adr010-g0-transport
mvn -B -q -f worker/pom.xml -DskipTests package
cd host
go run . -java "$JAVA_HOME/bin/java" -runtime jdk -jar ../worker/target/g0-worker.jar \
  -head "$(git rev-parse HEAD)" -out /tmp/g0.jsonl
```

CI runs the same probe on five runners, on a jlink image, through
`.github/workflows/adr010-g0-spike.yml`. macOS Intel was dropped on
2026-10-26. The bridge self-test runs alone
with:

```sh
java -cp worker/target/g0-worker.jar org.gopmgr.g0spike.BridgeSelfTest
```
