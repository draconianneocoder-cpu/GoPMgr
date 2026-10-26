<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 worker connection design (transport-neutral)

**Status:** PROPOSED, design only. Date: 2026-10-25.\
**Authority:** the [maintainer's G0 Windows transport decision](../review/ADR-010-G0-windows-transport-decision.md)
authorizes transport-neutral G1 *design* work.\
**Not authorized by this document:**

- G1 production code (slice P04 needs its own authorization);
- production dependencies (OWN-11);
- finalizing the Windows transport.

**Evidence:** the [G0 Windows transport report](../testing/adr-010-g0-windows-transport-report.md),
the [transport matrix](../interoperability/transport-matrix.md) and the
[G0 evidence report](../testing/adr-010-g0-evidence.md).

## 1. Purpose

G1 builds the registry, supervisor, job lifecycle and generated contract
(plan §3). This design keeps all of that independent of the local IPC that
carries the gRPC connection. The Windows transport can then be finalized,
or changed through an accepted ACR, without touching the code above it.

It specifies one host interface, one worker interface, the lifecycle
around them, and the regression tests a transport must pass before G1 may
ship it. It does **not** pick the Windows transport.

## 2. Fixed constraints

| Source | Constraint |
|---|---|
| OWN-02, D-03 | Native gRPC over HTTP/2, with one generated service contract on every OS |
| OWN-03 | No stdio business protocol and no automatic TCP fallback |
| INT-02 | A one-shot stdin bootstrap carries the endpoint and the credential, never business data |
| D-05 | Private, unpredictable, operation-scoped endpoints, OS peer checks and a single-use credential |
| D-06 | The lifecycle states, bounded resources, and no orphaned process, goroutine or endpoint |
| Decision 2026-10-25 | Windows priority stays as approved:<br>- `npipe-async` is the preferred candidate, conditionally;<br>- `npipe-dual` is a tested alternative;<br>- AF_UNIX is a verified alternative |

## 3. Shape

```text
            Go host                                     Java worker
 ┌───────────────────────────────┐            ┌──────────────────────────────┐
 │ registry ─ supervisor ─ job   │            │ job handlers                 │
 │              │                │            │      │                       │
 │              ▼                │            │      ▼                       │
 │   ipc.Transport / Endpoint    │  local IPC │  WorkerTransport             │
 │   (one per platform, fixed)   │◀══════════▶│  (chosen by the descriptor)  │
 │              │                │  HTTP/2    │      │                       │
 │              ▼                │            │      ▼                       │
 │   grpc.Server (generated)     │            │  ManagedChannel + stubs      │
 └───────────────────────────────┘            └──────────────────────────────┘
          │ stdin: bootstrap (once, ≤ 4 KiB, expires)  ▲
          └────────────────────────────────────────────┘
```

The host owns the endpoint and is the gRPC server. The worker connects as
the client (plan §2.2). Nothing above the `ipc` package knows the
transport's name.

### 3.1 Host interface (illustrative, `internal/interop/ipc`)

```go
// Transport creates endpoints of one kind. A build registers exactly one
// per platform (§4).
type Transport interface {
	ID() string // "uds", "npipe-async", "npipe-dual", "af-unix"
	Listen(scope Scope) (Endpoint, error)
}

// Endpoint is one operation's private IPC endpoint.
type Endpoint interface {
	// Descriptor goes into the bootstrap: transport ID and address only.
	Descriptor() *contractpb.EndpointDescriptor
	// Listener admits only the expected peer, once (§3.3).
	Listener() net.Listener
	// Expect sets the launched child's PID. It is called once, right after
	// the process starts. Until then every connection is refused.
	Expect(pid int) error
	// Close removes the endpoint and any OS object it created. It is
	// idempotent and bounded.
	Close() error
}
```

### 3.2 Worker interface (illustrative)

```java
interface WorkerTransport {
  String id();
  /** Opens the channel within the deadline, or throws; never retries forever. */
  ManagedChannel open(EndpointDescriptor d, Deadline openBy) throws TransportException;
}
```

- **Implementations:** Netty UDS (epoll, kqueue or JDK NIO), the
  overlapped-pipe bridge and the two-connection pipe bridge.
- **Selection:** by the descriptor's transport ID. An unknown ID ends the
  worker with the bootstrap exit code (20).
- **Close:** `shutdownNow()`, then a bounded wait for termination, then the
  transport's own resources (§6).

### 3.3 Admission rules (every transport)

1. **Peer check at accept time.** It runs before any byte reaches HTTP/2.
   The OS-reported peer PID must equal the launched child's.
   - The PID cannot be reused while the check matters: on Unix the child
     stays unreaped; on Windows the host holds its process handle.
   - With `npipe-dual`, *each* half is checked as it is accepted. G0 found
     a pairing listener that checked only after pairing, which let a
     foreign half spoil the worker's pair.
2. **One-shot listener.** Once the worker's connection is admitted (the
   pair, for `npipe-dual`), the endpoint stops accepting and closes the OS
   listener.
   - This matters on Windows: go-winio v0.6.4 creates pipe instances
     without limit (`maxInstances` is `0xffffffff` in `pipe.go`).
   - Closing the listener does not end the admitted connection: grpc-go's
     `Serve` returns, and live connections continue until `Stop`.
3. **Credential proof.** HMAC proof in call metadata, accepted once, checked
   before any handler runs (G0 design, unchanged).
4. **Refusal before `Expect`.** A connection that arrives before the child's
   PID is known is refused. The worker's open retry covers this. The retry
   is bounded (G0: 5 ms rising to 200 ms, 5 s in total).
5. **Pairing deadline (`npipe-dual` only).** A half whose partner does not
   arrive within the deadline is dropped (G0: 3 s). Refusals and timeouts
   are counted and reported.

## 4. Transport selection

- **One transport per platform per release,** fixed in the build and
  listed in the release notes. The supervisor never tries a second
  transport at run time.
- **Failure is typed.** A transport that fails to start is
  `PLUGIN_START_FAILED` (one bounded retry, plan B.9). A platform with no
  accepted transport is `TRANSPORT_UNAVAILABLE`. GoPMgr's other features
  are unaffected.
- **Why no fallback between native transports:** a fallback doubles the
  security surface on every machine and hides real failures. It would also
  make support reports depend on which path happened to run. It is not
  forbidden by OWN-03, so it is listed as open question Q1.

| Platform | Approved order (unchanged) | G1 status |
|---|---|---|
| Linux x64, arm64 | UDS | Choose Netty epoll or JDK NIO in G1 (Q3) |
| macOS arm64, x64 | UDS | Choose Netty kqueue or JDK NIO in G1 (Q3). NIO avoids extracting native libraries, which interacts with signing (dependency report §2.1) |
| Windows | 1. named pipe; 2. AF_UNIX | `npipe-async` preferred, conditionally. Not final until §8's tests pass on the platforms the release claims |

## 5. Lifecycle

The D-06 states, with the transport events that move between them:

| From → to | Event | Bound (proposed) |
|---|---|---|
| VERIFIED → STARTING | The supervisor creates the endpoint, then starts the process (Windows: suspended → Job Object → resume) | Fails closed if the tree cannot be owned |
| STARTING → CONNECTING | `Expect(pid)`, then the bootstrap is written and stdin closed | Bootstrap expires after 60 s (G0) |
| CONNECTING → AUTHENTICATED | Peer check, then credential proof | Handshake deadline, set from G0 cold-start p95 plus margin (G0 Windows p95 ≈ 1 s) |
| AUTHENTICATED → READY | `Connect` stream open; capabilities exchanged | — |
| RUNNING → DRAINING | Host cancel, user cancel, deadline or stale session | — |
| DRAINING → EXITED | Worker exits by itself | Grace period G (below) |
| DRAINING → EXITED (forced) | G expires: kill the whole tree, then reap | Kill-to-reap bound (G0: within 5 s) |
| any → EXITED | Process exits on its own (crash, OOM) | The host waits on the process handle or `wait`; no polling |

**Cancellation is deterministic:**

1. The host cancels the stream (HTTP/2 RST_STREAM).
2. The worker sees `CANCELLED` and exits with code 21.
3. The host waits at most G on the process itself, using a timer and the
   process wait, never a sleep loop.
4. When G expires, it ends the tree.
5. Then it reaps the process, closes the endpoint and removes scratch
   files.

There is one escalation step, with no retries and no open-ended waits. The
spike's 15 s grace is a test bound, not a proposal.

**Proposed G:** at least 4 × the worst p99 cancel-to-exit measured on the
release's hardware profiles, and no less than 1 s. The G0 Windows fix
brings cancel-to-exit to the same range as the other transports (see the
report). Hardware profiles are owner decisions (OWN-10), so G is Q5.

**Exit codes** (worker contract):

| Code | Meaning | Host code (plan B.9) |
|---|---|---|
| 0 | Done | — |
| 20 | Bootstrap refused (malformed, wrong version, unknown transport) | `PLUGIN_START_FAILED` |
| 21 | Cancelled | `USER_CANCELLED` or the cancelling cause |
| 22 | Request rejected | `INTERNAL` with an error ID, unless a typed status says more |
| 23 | Worker error | Typed gRPC status if one arrived; otherwise `PLUGIN_UNHEALTHY` |
| 24 | Bootstrap expired | `PLUGIN_START_FAILED` |
| 3 | JVM out of memory (`-XX:+ExitOnOutOfMemoryError`) | `RESOURCE_LIMIT` (memory) |
| other, or killed | Crash or forced exit | `PLUGIN_UNHEALTHY` |

## 6. Process tree and resources

**Windows:**

- The worker starts suspended, joins a Job Object with kill-on-close, and
  then resumes. If the assignment fails, the launch fails.
- G1 should evaluate assigning the job at creation through a
  process-thread attribute list. That would remove the suspend-and-resume
  step and the thread snapshot the spike uses to resume. Check what the Go
  version in use supports before relying on it.
- The job can also carry memory and active-process limits. These are
  candidates for D-06 resource policy, set from G2 measurement.

**Unix:**

- The worker leads its own process group, and the host ends the tree with
  `kill(-pgid, SIGKILL)`.
- **Residual risk:** a process that calls `setsid` leaves the group. The
  worker starts no children in normal operation, so a child at all means
  the worker is misbehaving. G1 options:
  - Linux: a cgroup v2 scope;
  - macOS: no equivalent;
  - both: report the residual.
- This is Q2.

**JVM flags:**

- `-XX:+DisableAttachMechanism`;
- `-XX:+ExitOnOutOfMemoryError`;
- an explicit `-Xmx` from the resource policy;
- no inherited `JAVA_TOOL_OPTIONS`, `_JAVA_OPTIONS` or `CLASSPATH`.

**Per-connection limits.** These are G0 candidates; G2 measurement sets the
final values:

| Limit | G0 value |
|---|---|
| Message size | 4 MiB, enforced: G0 `oversize` gets `RESOURCE_EXHAUSTED` |
| Bootstrap | 4 KiB, 60 s expiry |
| Captured stdout and stderr | 64 KiB each, not persisted |
| Connections | One admitted connection; for `npipe-dual`, one pair |
| Streams | A low concurrent-stream limit on the server, sized for one `Connect` stream plus control calls |

## 7. Requirements for Java pipe bridges

These rules come from the G0 bridge review. They apply to any transport
that bridges an OS handle into gRPC through an in-JVM channel.

| ID | Requirement | Why (G0) |
|---|---|---|
| BR-1 | Nothing blocks the Netty event loop: no blocking `put`, no `Future.get()`, no sleep | B1, B2, B5 |
| BR-2 | At most one read in flight; issue the next read only when gRPC has taken the last one | Bounded memory toward gRPC |
| BR-3 | Bounded write queue with high and low water marks that toggle `autoRead`. `LocalChannel` gives no write backpressure, so the bound must be explicit; HTTP/2 flow control then holds the sender | B1 |
| BR-4 | One gRPC connection per pipe; refuse a second at once | B4 |
| BR-5 | `close()` is idempotent and bounded. It releases every buffer, including one whose I/O completes after close. A completion that arrives after the event loop has stopped runs its cleanup inline | B3, B5 |
| BR-6 | **Each `AsynchronousFileChannel` gets its own executor, shut down with the channel.** The JDK's default completion-port thread waits in native code forever. HotSpot's `VM_Exit` then waits up to 30 × 10 ms for it, which is the ≈ 480 ms Windows exit delay | W2 root cause |
| BR-6a | Shut that executor down **only after every issued operation has delivered its completion.** Use a bounded wait woken by the last completion. The JDK hands aborted operations' failures to the executor from its completion-port thread, possibly after `close()` returns. A shut-down executor makes that thread die with `ShutdownChannelGroupException`, and the operation's handler never runs | B8 (found in G0 on Windows) |
| BR-6b | Any uncaught exception on any worker thread fails the test run, even if the session succeeded | B8 was visible only on stderr |
| BR-7 | No swallowed errors: every I/O failure closes the bridge and is counted | B6 |
| BR-7a | Never issue a zero-length pipe write; drop empty buffers. grpc-netty writes an empty buffer when a client transport starts. `LocalChannel` merges it into the next buffer only when both are queued. Read alone, it reached the overlapped channel, which completes a zero-length write with 0 bytes, and the bridge then closed the connection | B9 (found in G0 on Windows; intermittent, 1–3 of 10 second connections) |
| BR-8 | The bridge exposes counters (refused children, autoRead toggles, peak queued bytes, late completions, forced pool shutdowns, dropped empty buffers) and the exception behind any I/O failure, for the supervisor's diagnostics | Evidence for §8; B9 was found from the recorded exception |

**Note on blocking pipes:** a synchronous Windows `ReadFile` is not
cancelled by closing the handle from another thread. A blocking bridge
(`npipe-dual`) therefore depends on the far end closing to free its
reader. The host's tree kill guarantees that ending.

## 8. Regression tests required before a Windows transport is finalized

The maintainer's decision requires documented fixes *and* passing
regression tests. A transport is finalized for a platform only when every
row passes on that platform, on the production implementation, not the
spike.

| Area | Test (G0 scenario or new) | Test IDs |
|---|---|---|
| Handshake, duplex, ping | `handshake`, `duplex`, `connect-repeat` (50 fresh connections, zero failures) | TPT-003, TPT-004, TPT-012 |
| Backpressure | `backpressure`, `cancel-backpressure`, `bridge-selftest` | TPT-012 |
| Cancellation latency | `cancel`, at least 30 runs, with p50/p95/p99 within the G budget. The at-exit thread list must name no thread blocked in a native wait. The list samples stack tops, so a thread caught for an instant in a native intrinsic (an `Unsafe` CAS) can appear; G0 saw that once in about 40 Linux runs, with no exit delay | TPT-012, TPT-013 |
| Process tree | `tree-kill`, `crash` (a grandchild must not survive) | TPT-013, TPT-014 |
| Abnormal JVM exit | `kill`, `crash`, `oom` | TPT-013, TPT-014 |
| Peer identity | `impersonate`, `connect-flood`, pairing unit tests | TPT-018 |
| Credential protection | `bad-credential`, `replay`, leak detector with self-check | TPT-007, TPT-020 |
| Endpoint squatting | `squat` | TPT-008, TPT-009 |
| Resource exhaustion | `oversize`, `oom`, `connect-flood` | TPT-013 |
| Concurrency | `concurrent` (four sessions) | TPT-012, TPT-018 |
| Bootstrap | `bootstrap-*`, `handshake-timeout`, `missing-jvm` | TPT-022, TPT-014 |
| Packaging | All of the above on the release's jlink image, offline | Plan §2.2 |
| Signed execution | All of the above from the signed, installed app | TPT-002 (signed-app clause) |
| Platform coverage | Windows 10 and 11 x64, and Windows 11 arm64 if claimed | TPT-003, TPT-004 |

Current status per platform is in the [G0 Windows transport report](../testing/adr-010-g0-windows-transport-report.md).
Every unrun row is BLOCKED.

## 9. Open questions for the maintainer

| # | Question | Recommendation |
|---|---|---|
| Q1 | May the supervisor fall back from one native transport to another at run time (for example, named pipe to AF_UNIX)? | No. One transport per platform per release (§4). |
| Q2 | Is the Unix `setsid` escape an accepted residual for G1, or must G1 add a Linux cgroup scope? | Accept and report for G1; revisit with the G2 sandboxing review. |
| Q3 | On Linux and macOS, Netty native (epoll, kqueue) or JDK NIO Unix sockets? | JDK NIO: no native libraries to extract, sign or notarize. Both passed G0, with similar numbers on CI. |
| Q4 | Which Windows 10 and 11 x64 machines, and which signing identities, serve as validation profiles? | Needed for the BLOCKED rows in §8; owner-provided. |
| Q5 | Grace period G before the tree kill? | 4 × the worst measured p99, at least 1 s, on the approved hardware profiles (§5). |

## 10. What stays out of G1

- No new transport beyond those evaluated in G0.
- No TCP, and no stdio business protocol.
- No change to the Windows priority.
- No production dependency without its OWN-11 slice and a
  `docs/DEPENDENCIES.md` record.
