<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G0 transport spike (P01)

**Non-production.** This folder is the isolated feasibility work the
maintainer authorized on 2026-10-24 (decision record §5.1). It is not a
GoPMgr component.

**What it does not touch:**

- GoPMgr's `go.mod`;
- `make`, `wails build` and the release packages;
- any project database.

It must never become a production worker; G1 needs its own accepted work
slice.

## The question it answers

ADR-010 OWN-02 requires native gRPC over HTTP/2, with typed Protocol
Buffers, between the Go host and a Java worker over private local IPC. The
spike asks whether that works on each OS:

- Unix-domain sockets on Linux and macOS;
- restricted named pipes on Windows, with AF_UNIX as the alternative.

It runs real processes on real operating systems, with no mocks. A
transport that does not work is reported FAIL or BLOCKED. The probe never
falls back to TCP or to a stdio data protocol (OWN-03).

## Layout

| Path | What it is |
| --- | --- |
| `proto/g0spike.proto` | The spike's contract. It is the minimum for the measurements, not the production IDL |
| `scripts/generate.sh` | Regenerates the committed Go and Java code with pinned tools |
| `host/` | The Go host: gRPC server and the `g0probe` evidence harness. A separate Go module |
| `worker/` | The Java worker (Maven). It reads a one-shot stdin bootstrap and connects as the gRPC client |
| `evidence/` | Recorded runs, one JSON object per scenario |

## Design under test

**Roles.** The host creates the endpoint and is the gRPC server. The worker
connects as the client and opens one bidirectional `Connect` stream (plan
§2.2).

**Endpoints:**

- **Unix sockets:** a fresh `0700` directory under `/tmp`, owner-checked,
  holding a `0600` socket. The path stays short, because macOS limits it to
  104 bytes.
- **Named pipes:** a random name, a DACL for the current user only, and
  first-instance creation, which refuses a squatted name.
- **Windows AF_UNIX:** a private directory under `%TEMP%`.

**Bootstrap (INT-02).**

- One uvarint-length-prefixed protobuf message is written on the child's
  stdin, and then stdin closes.
- It is versioned, limited to 4 KiB, and expires after 60 seconds.
- It carries the endpoint and a 32-byte single-use credential. The
  credential never appears in argv or the environment.

**Authentication.**

- The worker sends HMAC-SHA256(credential, label, operation ID, worker ID)
  as call metadata.
- The host checks it before any handler runs, and accepts it once.
- The listener also refuses any connection whose OS-reported peer process is
  not the launched child:
  - `SO_PEERCRED` on Linux;
  - `LOCAL_PEERPID` on macOS;
  - `GetNamedPipeClientProcessId` on Windows pipes;
  - `SIO_AF_UNIX_GETPEERPID` on Windows AF_UNIX, where available.

**Process controls.**

- **Fixed argument list.** No shell is involved.
- **Clean environment.** It is empty on Unix. On Windows it holds only
  `SystemRoot`, `TEMP` and `TMP`. `JAVA_TOOL_OPTIONS`, `_JAVA_OPTIONS` and
  `CLASSPATH` never pass.
- **Private working directory.**
- **JVM flags:** `-Xmx128m`, `-XX:+DisableAttachMechanism` and
  `-XX:+ExitOnOutOfMemoryError` (an out-of-memory worker exits at once
  with code 3 instead of limping on).
- **Process tree:**
  - **Unix:** the worker leads its own process group; the host ends the
    tree with `kill(-pgid, SIGKILL)`.
  - **Windows:** the worker starts suspended, joins a Job Object with
    kill-on-close, and only then resumes. A child it starts is in the job
    from its first instruction. If the job cannot be set up, the launch
    fails; the worker never runs outside the job.
- **Capped capture.** The host keeps at most 64 KiB of the child's stdout
  and of its stderr, and persists none of it.

**Java transports:**

| Name | How it works |
| --- | --- |
| `uds-epoll` | Linux Netty native epoll |
| `uds-kqueue` | macOS Netty native kqueue |
| `uds-nio` | Netty's `NioDomainSocketChannel`: JDK Unix-domain sockets (Java 16+), with no native library, on all three OSes |
| `npipe` | One synchronous Windows pipe handle (`RandomAccessFile`), bridged to gRPC through an in-JVM `LocalChannel` |
| `npipe-dual` | Two synchronous pipe connections, one per direction, so reads and writes never share a handle. The host pairs them in `host/pairing.go`: each half is peer-checked when it is accepted, and a half whose partner does not arrive within 3 s is dropped |
| `npipe-async` | One pipe connection opened through `AsynchronousFileChannel`, which the JDK opens for overlapped I/O, so a pending read does not block a write. The channel gets its own executor, closed with the channel (see below) |
| `npipe-async-default` | **Diagnostic control only, not a candidate.** The same, on the JDK's default completion-port thread. It reproduces the exit delay the fix removes |

**The pipe bridge** (`worker/.../PipeBridge.java`) connects a pipe to
gRPC through an in-JVM `LocalChannel`. Its rules:

- Nothing blocks the Netty event loop. The pipe opens before gRPC
  connects, with a bounded retry (5 ms rising to 200 ms, 5 s in total).
- At most one read is in flight. The next read starts only when gRPC has
  taken the last one.
- Writes queue on the event loop. Above 1 MiB queued, the bridge stops
  reading from gRPC; below 256 KiB it starts again. HTTP/2 flow control
  then holds the sender.
- It serves one gRPC connection; a second is closed at once.
- It drops empty buffers. grpc-netty writes one when a client transport
  starts. Read alone, it became a zero-length overlapped write, which the
  JDK completes with 0 bytes, and the bridge then closed the connection
  (G0 finding B9).
- `close()` is idempotent and bounded: it closes the connection, the pipe
  and the event loop, waits at most 2 s, and releases every buffer it
  held, including one whose I/O completes after close.

## Scenarios

| Scenario | Shows | Test IDs |
| --- | --- | --- |
| `handshake` | Bootstrap, authenticated connect, 200 ping round-trips, clean exit; no credential leak, no TCP socket, no dangerous environment variable | TPT-001 to TPT-004 (by OS and transport), TPT-005, TPT-007, TPT-017, TPT-020, TPT-022 |
| `duplex` | 64 MiB host to worker while 100,000 records flow back at the same time; SHA-256 checked on both sides | TPT-012 |
| `backpressure` | The host stops reading for 3 s while the worker floods; the worker waits on flow control and its heap stays bounded | TPT-012 |
| `noise` | The worker prints 1 MiB to stdout and stderr; the stream is unaffected and the capture stays capped | TPT-021 |
| `bad-credential` | A wrong proof is refused before any handler runs | TPT-007, TPT-018 |
| `replay` | The worker presents the same proof again from its own process, 10 times; every attempt must be refused as `UNAUTHENTICATED`. Any other outcome is recorded with its gRPC detail | TPT-007 |
| `impersonate` | Another local process with the stolen credential connects first; the peer check refuses it | TPT-018 |
| `kill` | The host kills the worker mid-stream; the stream fails promptly and the process is reaped | TPT-013, TPT-014 |
| `cancel` | 30 runs of a host cancel mid-stream; the worker exits with its cancel code; latency distribution | TPT-012, TPT-013 |
| `bootstrap-*` | Oversized, garbage, empty and expired bootstraps are refused without connecting | TPT-022 |
| `handshake-timeout`, `missing-jvm` | Wrong endpoint or missing JVM fails cleanly and on time | TPT-014 |
| `squat` | A pre-placed socket path, file or pipe name is refused | TPT-006, TPT-008 or TPT-009 |
| `detector-self-check` | The leak and TCP detectors catch a planted secret and a real listener, so their "none found" results mean something. The planted process is the probe itself, re-run with `-g0-sleeper <secret>`, so no shell can rewrite its command line | TPT-007, TPT-020 |
| `cancel-backpressure` | Cancel while the host has stopped reading for 1 s and the worker's sends wait on flow control | TPT-012, TPT-013 |
| `oversize` | The worker sends one 5 MiB message; the host's 4 MiB limit refuses it with `RESOURCE_EXHAUSTED` | TPT-013, TPT-014 |
| `concurrent` | Four sessions at once, each with its own endpoint and full-duplex traffic; no session sees another's data | TPT-012, TPT-013, TPT-018 |
| `connect-flood` | A process other than the worker (the probe itself) opens 50 connections while the worker starts; all are refused and the session still succeeds | TPT-006, TPT-018 |
| `connect-repeat` | 50 fresh launches that only complete the handshake; any failure is recorded with the worker's notes and the host's gRPC log lines, and FAILs the scenario | TPT-003, TPT-004 (by transport) |
| `bridge-selftest` | The pipe bridge's own tests, in memory, on every OS: 16 MiB each way, bounded buffering, close with I/O pending, second connection refused | TPT-012, TPT-013 |
| `tree-kill` | On every transport: the worker starts a grandchild; the host ends the tree; both are gone | TPT-013, TPT-014 |
| `crash` | On every transport: the worker starts a grandchild, then halts mid-stream; the stream fails and no orphan remains | TPT-013, TPT-014 |
| `oom` | On every transport: the worker exhausts its heap; it exits with code 3 and the stream fails | TPT-013, TPT-014 |

**Every scenario** also fails if the worker's stderr reports a thread
ended by an uncaught exception (`Exception in thread ...`), even when the
session itself succeeded.

**Diagnostics on failure.**

- **Worker:** the pipe bridge records the exception behind any I/O failure
  (`g0-note:` lines on stderr).
- **Host:** the probe keeps grpc-go's own log. A job whose session failed
  carries the matching lines in its evidence (`host_grpc_log`).
- **Content:** neither source records the credential or the proof.

**Worker exit codes:** 0 done, 20 bootstrap refused, 21 cancelled, 22
rejected, 23 error, 24 bootstrap expired, 70 crash (the `crash` scenario's
deliberate halt). The JVM's own out-of-memory exit is 3.

**Cancel timing.** The `cancel` evidence splits the delay into phases:
host cancel to connection end, connection end to process exit, and the
worker's own marks (stream error, exit call, shutdown hook). At exit the
worker also lists any thread still in native code. That list is how the
Windows exit delay was traced (see
`docs/testing/adr-010-g0-windows-transport-report.md`).

These results are **G0 evidence about the spike code**. They are not PASS
results for the G1 gates: G1 requires the production implementation.

## Run it

You need JDK 17 or later, Maven 3.9 and Go 1.27.3. CI uses a pinned
toolchain: `scripts/toolchain.sh <platform> <dir>` fetches Temurin
jdk-21.0.12.1+1 and Maven 3.9.11, checks the publishers' checksums and
prints the JDK home.

```sh
mvn -B -q -f worker/pom.xml -DskipTests package
cd host
go run . -java "$JAVA_HOME/bin/java" -jar ../worker/target/g0-worker.jar \
  -head "$(git rev-parse HEAD)" -out ../evidence/my-run.jsonl
```

Useful flags:

- `-scenarios cancel,tree-kill` runs only those (the handshake always runs);
- `-transports npipe-async` runs only that transport;
- `-cancel-runs 30` sets the cancel repetitions;
- `-runtime jlink` labels the record when `-java` is a jlink image.

To run on a minimal runtime, as CI does:

```sh
mods="$(jdeps --ignore-missing-deps --multi-release 21 --print-module-deps worker/target/g0-worker.jar)"
jlink --add-modules "$mods" --strip-debug --no-header-files --no-man-pages --output /tmp/g0-runtime
```

`.github/workflows/adr010-g0-spike.yml` runs the probe on a jlink runtime
on five runners when this folder changes: Linux x64 and Arm, macOS Apple
silicon, Windows Server 2025 x64 and Windows 11 Arm. The maintainer dropped
macOS Intel on 2026-10-26; its earlier results stay in `evidence/`. On Linux
it runs a second time inside an empty network namespace (`unshare -rn`)
to show the worker needs no network. It uploads the evidence. That workflow is not a gate: it reports,
and the evidence file records PASS, FAIL or BLOCKED for each scenario.

## Pinned versions

| Component | Version |
| --- | --- |
| grpc-go | v1.84.0 |
| protobuf-go | v1.36.12 |
| golang.org/x/sys | v0.49.0 |
| go-winio | v0.6.4 |
| grpc-java | 1.85.1 |
| Netty | 4.2.17.Final (grpc-netty's own version) |
| protobuf-java | 3.25.9 (grpc-protobuf's own version) |
| protoc | 3.25.9 (libprotoc 25.9), so Java gencode matches the runtime |
| protoc-gen-go | v1.36.12 |
| protoc-gen-go-grpc | v1.6.2 |
| protoc-gen-grpc-java | 1.85.1 |
| JDK (CI) | Eclipse Temurin 21.0.12.1+1 |
| Maven (CI) | 3.9.11 |

The OWN-11 limit still applies: these dependencies exist only in this
spike. Adding any of them to GoPMgr's `go.mod` needs G0 results, a
`docs/DEPENDENCIES.md` review and an authorized G1 slice.
