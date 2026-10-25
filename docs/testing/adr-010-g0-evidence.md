<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 P01/G0 — Go↔Java gRPC local-IPC evidence

**Status:** Evidence for owner review, 2026-10-25. P01/G0 is authorized by
the [maintainer acceptance](../review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md).
This is spike code in its own modules. Nothing reaches GoPMgr's `go.mod`,
build or release (OWN-11). **G0 is not complete:** see §8.\
**Canonical results:** [transport matrix](../interoperability/transport-matrix.md).
**Dependencies and licenses:** [G0 dependency report](../interoperability/adr-010-g0-dependency-report.md).
**Harness:** [`spikes/adr010-g0-transport`](../../spikes/adr010-g0-transport/README.md).

Results come from `7d67cae`. The next commit only removes an unused Java
dependency (`javax.annotation-api`; dependency report, L1). It does not
change transport code, and CI re-runs every platform on it.

## 1. Outcome

- **Linux and macOS:** real grpc-go ↔ grpc-java over a private Unix-domain
  socket works on Linux amd64, macOS arm64 and macOS Intel. It works with
  both Netty native transports (epoll, kqueue) and with the JDK's own Unix
  sockets (Netty NIO). 26 of 26 scenarios pass on each.
- **Windows named pipes:** they work on Windows Server 2025 amd64 when reads
  and writes never block each other on one handle. Two variants pass all 9
  scenarios:
  - two synchronous pipe connections, one per direction (`npipe-dual`);
  - one overlapped connection (`npipe-async`).

  The plain single synchronous handle (`npipe`) deadlocks.
- **Windows AF_UNIX:** it works on the same Windows Server 2025 runner with
  no bridge code: 17 of 17 scenarios pass.
- **Every PASS run held these conditions:**
  - no TCP socket;
  - credential delivered only through the one-shot stdin bootstrap;
  - credential absent from argv and the environment;
  - empty or minimal worker environment;
  - peer-process check enforced;
  - detector self-check passing.
- **Not shown yet:** packaged and signed apps, an offline `jlink` image,
  Linux and Windows arm64, Windows 10/11 desktop editions, and CPU cost.
  These are BLOCKED (§8).

## 2. What the spike does

| Piece | Summary |
|---|---|
| Contract | `proto/g0spike.proto`: one bidirectional `WorkerSession.Connect` stream. The worker sends hello, records, completion and pong; the host sends execute, source blocks, ping and finish. |
| Host | Go, grpc-go server. Creates a fresh private endpoint per job, launches the JVM with an empty environment (Windows: `SystemRoot`, `TEMP`, `TMP`), writes a length-prefixed protobuf bootstrap to stdin, then closes it. |
| Worker | Java 21, grpc-java client. Reads at most 4,096 bootstrap bytes, closes stdin, connects, and proves possession of the credential with HMAC-SHA256 over the operation and worker IDs. |
| Authentication | Constant-time, single-use proof check in a gRPC interceptor (`Unauthenticated` on failure). The listener admits one connection and checks the peer process ID: `SO_PEERCRED` (Linux), `LOCAL_PEERPID` (macOS), `GetNamedPipeClientProcessId` (pipes), `SIO_AF_UNIX_GETPEERPID` (Windows AF_UNIX). |
| Endpoint safety | UDS: private 0700 directory under `/tmp`, refuses an existing path, 100-byte path cap. Pipes: explicit user-only DACL `D:P(A;;GA;;;<user SID>)`; go-winio creates the first instance with `FILE_CREATE` (anti-squat) and rejects remote clients. |
| Windows pipe bridge | The JDK has no Netty channel for named pipes. gRPC talks to an in-JVM Netty `LocalChannel`, and two threads copy bytes between it and the pipe. No OS endpoint other than the pipe is opened. |

## 3. Versions

| Component | Version | Source |
|---|---|---|
| Go | 1.27.3 | `host/go.mod` |
| grpc-go | v1.84.0 | `host/go.mod` |
| protobuf-go | v1.36.12 | `host/go.mod` |
| golang.org/x/sys | v0.49.0 | `host/go.mod` |
| go-winio (Windows only) | v0.6.4 | `host/go.mod` |
| grpc-java | 1.85.1 | `worker/pom.xml` |
| Netty | 4.2.17.Final | `worker/pom.xml` |
| protobuf-java | 3.25.9 | `worker/pom.xml` (grpc-java 1.85.1 needs 3.25.x) |
| JDK on CI | Temurin 21.0.12+1 LTS | evidence `java_version` |
| JDK in container | OpenJDK 21 (Ubuntu package) | local run |
| Code generation | protoc 3.25.9 (libprotoc 25.9), protoc-gen-go v1.36.12, protoc-gen-go-grpc v1.6.2, protoc-gen-grpc-java 1.85.1 | `scripts/generate.sh` |

Netty native libraries shipped in the worker jar (SHA-256 of the bytes in
the jar):

| Library | SHA-256 |
|---|---|
| `libnetty_transport_native_epoll_x86_64.so` | `2e37cf5f75a64759e9ae88afb6bcfff7c5fc14bab8ee51c6c026b9235c9331b5` |
| `libnetty_transport_native_epoll_aarch_64.so` | `f8aeaafaa8532b423a3db750f9e2c5eba4bb33f3aa119b8038532f555c587213` |
| `libnetty_transport_native_kqueue_x86_64.jnilib` | `bffc7d11af9299008e82795fe6c494bc3223bd23b6b1db3719f586a8909f703e` |
| `libnetty_transport_native_kqueue_aarch_64.jnilib` | `cb073e4d374a26fd8ce78dbb7ef7480799068725661e129fea0af4f69bb34f03` |

Netty has no Windows native transport. Windows uses only pure-Java code
(NIO for AF_UNIX, `RandomAccessFile` or `AsynchronousFileChannel` for pipes).

## 4. Results by test ID

Run at `7d67cae` on GitHub-hosted runners, and in the session container.

| Test ID | Requirement (short) | Result | Basis |
|---|---|---|---|
| TPT-001 | grpc-go ↔ grpc-java over UDS on Linux, no TCP | **PASS** on amd64; arm64 **BLOCKED** | 26/26 on CI and in the container |
| TPT-002 | Same on macOS Intel and Apple Silicon; native Netty in an installed signed app | **PASS** unsigned on both architectures; signed-app clause **BLOCKED** | 26/26 on each |
| TPT-003 | Real Java gRPC client to the Go named-pipe listener; bidirectional streams and cancellation | **PASS** for `npipe-dual` and `npipe-async` on Server 2025 amd64; **FAIL** for one synchronous handle | 9/9, 9/9, 0/1 + 8 blocked |
| TPT-004 | Windows AF_UNIX passes the same real-client tests | **PASS** on Server 2025 amd64 | 17/17 |
| TPT-005 | No silent TCP or stdio fallback | **PASS** for the spike | Each transport runs alone; failures are FAIL/BLOCKED; `tcp_sockets` is 0 in every PASS record |
| TPT-022 | One-shot, bounded, expiring stdin bootstrap; credential absent from argv/env; hostile child, replay and squat refused; all three OSes | **PASS** on Linux, macOS and Windows for bounds, expiry, argv/env absence, replay, impersonation and squat. Absence from crash reports is **not tested** | `bootstrap-*`, `replay`, `impersonate`, `squat`, `detector-self-check` |

Earlier evidence for G1 IDs: the spike also exercises TPT-006, TPT-007,
TPT-008, TPT-012, TPT-013, TPT-014, TPT-017, TPT-018, TPT-020 and TPT-021
(see the README scenario table). They all pass where run. They are **not**
G1 passes: G1 requires the production implementation.

## 5. Run-to-run variance

Two complete CI runs, `76facf1` and `7d67cae`. Between them the Java
pipe bridge was refactored to add `npipe-async`; the UDS and AF_UNIX code
did not change. Values are p50.

| Platform / transport | Cold start ms | Ping RTT µs | Duplex MiB/s | Cancel to exit ms |
|---|---|---|---|---|
| Linux / epoll | 467 → 467 | 411 → 412 | 102 → 100 | 31 → 33 |
| Linux / NIO | 471 → 483 | 444 → 459 | 101 → 99 | 31 → 31 |
| macOS arm64 / kqueue | 772 → 640 | 621 → 697 | 62 → 63 | 28 → 23 |
| macOS arm64 / NIO | 651 → 494 | 409 → 844 | 67 → 74 | 32 → 63 |
| macOS Intel / kqueue | 897 → 1,994 | 783 → 1,350 | 26 → 13 | 44 → 79 |
| macOS Intel / NIO | 890 → 1,982 | 993 → 2,422 | 26 → 14 | 42 → 74 |
| Windows / AF_UNIX | 766 → 830 | 691 → 711 | 70 → 63 | 33 → 38 |
| Windows / `npipe-dual` | 790 → 802 | 1,016 → 956 | 69 → 66 | 35 → 40 |

Linux and Windows hosted runners vary by about 15% or less between runs.
macOS arm64 varies more, and hosted macOS Intel runners varied by a factor of
two. Budgets need owner-approved hardware, not hosted runners (OWN-10).

### 5.1 What changed between the runs

`76facf1` reported the detector self-check as FAIL on macOS and Windows.
The fault was in the self-check's planted process, not in the detectors:

- on macOS, `sh -c "sleep 5"` is bash, which replaces itself with `sleep`
  and drops the secret from the command line;
- on Windows, the secret became an extra argument to `ping`, which exited
  before the check ran.

`7d67cae` re-runs the probe itself with `-g0-sleeper <secret>`, and the
self-check passes on all four platforms. The "no credential leak" results
from `76facf1` on macOS and Windows should be disregarded; those from
`7d67cae` stand.

## 6. Findings

| ID | Finding | Impact | Evidence |
|---|---|---|---|
| W1 | One synchronous pipe handle deadlocks: the worker's blocked `ReadFile` stalls its first write, so the HTTP/2 preface never leaves. The host times out after 75 s. | A plain `RandomAccessFile` pipe cannot carry gRPC. | `windows-amd64/npipe/handshake` |
| W2 | `npipe-async` exits about 500 ms after a cancel (p50 500, p95 524, n=30), against about 40 ms for the other transports. All 30 runs exit with the cancel code. | Not a correctness failure, but unexplained. Hypothesis: closing an `AsynchronousFileChannel` with a read pending waits for the JDK to cancel that I/O. Needs a diagnostic run before this variant is chosen. | `windows-amd64/npipe-async/cancel` |
| W3 | `npipe-dual` pairs two pipe connections. The host checks the peer process on both halves and rejects a mismatch. | Another local process that connects between the halves causes a refused session (denial of service), not impersonation. | `endpoint_windows.go` `peerPID` |
| W4 | On Windows the leak detector reads only the worker's command line; another process's environment cannot be read. | The environment is controlled by construction (three names only), and the worker reports its own variable names. | `worker_env_names` |
| N1 | Netty logs "Unknown channel option 'SO_KEEPALIVE'" for domain sockets and `LocalChannel`. | Harmless. G1 should not set the option on these channels. | `stderr_head` |
| N2 | macOS adds `__CF_USER_TEXT_ENCODING` to every process environment. | Not a leak; set by the OS. | `worker_env_names` |

## 7. Owner decision: Windows transport for G1

OWN-02 makes restricted named pipes primary and accepts proven AF_UNIX.
All three options below meet TPT-003 or TPT-004 on Server 2025 amd64.

| Option | For | Against |
|---|---|---|
| **A. Named pipe, one overlapped connection** (`npipe-async`) | One OS connection per session; explicit user-only DACL; anti-squat; remote clients rejected; simplest peer check | Needs our copy bridge in the JVM; ~500 ms cancel-to-exit not yet explained (W2) |
| **B. Named pipe, two connections** (`npipe-dual`) | Proven fast cancel; same DACL protections | Two handles and pairing logic per session (W3); same copy bridge |
| **C. AF_UNIX** (`uds-nio`) | No bridge: a standard Netty NIO channel, the same code as Linux and macOS; fewest moving parts | OWN-02 lists it as the alternative, not the primary; relies on directory ACLs instead of an explicit pipe DACL; needs Windows 10 1803 or later |

**Recommendation:** keep named pipes primary as OWN-02 says, and take
**option A** into G1 *if* a diagnostic run explains and removes the W2
delay. Keep option B as the proven fallback, and keep C tested as the
approved alternative. The diagnostic run is G0 work and needs no new
authority.

Choosing C as primary would change OWN-02's priority and would need an
Architecture Change Request. Nothing in this evidence requires one: named
pipes work.

## 8. Not done, and why

| Item | State | Needed |
|---|---|---|
| Packaged, signed app; Netty natives load under macOS hardened runtime; Windows signing | **BLOCKED** | Owner signing identities and a packaging spike slice |
| `jlink` image runs offline | **BLOCKED (not run)** | A follow-up spike round; no new authority needed |
| Linux arm64, Windows arm64 | **BLOCKED (not run)** | Runners for those architectures |
| Windows 10 / 11 desktop editions | **BLOCKED (not run)** | Runners or the owner's machines |
| CPU cost per transport | **Not measured** | Add process CPU time to the evidence records |
| W2 diagnosis | **Open** | One instrumented Windows run |

## 9. Reproduce

```sh
cd spikes/adr010-g0-transport
mvn -B -q -f worker/pom.xml -DskipTests package
cd host
go run . -java "$JAVA_HOME/bin/java" -jar ../worker/target/g0-worker.jar \
  -head "$(git rev-parse --short HEAD)" -out ../evidence/my-run.jsonl
```

CI: `.github/workflows/adr010-g0-spike.yml` runs the same on
`ubuntu-26.04`, `macos-15`, `macos-15-intel` and `windows-2025`, and
uploads each evidence file. Recorded files are in `evidence/`, named
`ci-<commit>-<platform>.jsonl`.
