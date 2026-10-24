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
- **JVM flags:** `-Xmx128m` and `-XX:+DisableAttachMechanism`.
- **Own process group on Unix,** so the whole tree can be killed.
- **Capped capture.** The host keeps at most 64 KiB of the child's stdout
  and of its stderr, and persists none of it.

**Java transports:**

| Name | How it works |
| --- | --- |
| `uds-epoll` | Linux Netty native epoll |
| `uds-kqueue` | macOS Netty native kqueue |
| `uds-nio` | Netty's `NioDomainSocketChannel`: JDK Unix-domain sockets (Java 16+), with no native library, on all three OSes |
| `npipe` | One Windows pipe handle, bridged to gRPC through an in-JVM `LocalChannel` |
| `npipe-dual` | Two pipe connections, one per direction, so reads and writes never share a synchronous handle |

## Scenarios

| Scenario | Shows | Test IDs |
| --- | --- | --- |
| `handshake` | Bootstrap, authenticated connect, 200 ping round-trips, clean exit; no credential leak, no TCP socket, no dangerous environment variable | TPT-001 to TPT-004 (by OS and transport), TPT-005, TPT-007, TPT-017, TPT-020, TPT-022 |
| `duplex` | 64 MiB host to worker while 100,000 records flow back at the same time; SHA-256 checked on both sides | TPT-012 |
| `backpressure` | The host stops reading for 3 s while the worker floods; the worker waits on flow control and its heap stays bounded | TPT-012 |
| `noise` | The worker prints 1 MiB to stdout and stderr; the stream is unaffected and the capture stays capped | TPT-021 |
| `bad-credential` | A wrong proof is refused before any handler runs | TPT-007, TPT-018 |
| `replay` | The worker presents the same proof again from its own process; it is refused | TPT-007 |
| `impersonate` | Another local process with the stolen credential connects first; the peer check refuses it | TPT-018 |
| `kill` | The host kills the worker mid-stream; the stream fails promptly and the process is reaped | TPT-013, TPT-014 |
| `cancel` | 30 runs of a host cancel mid-stream; the worker exits with its cancel code; latency distribution | TPT-012, TPT-013 |
| `bootstrap-*` | Oversized, garbage, empty and expired bootstraps are refused without connecting | TPT-022 |
| `handshake-timeout`, `missing-jvm` | Wrong endpoint or missing JVM fails cleanly and on time | TPT-014 |
| `squat` | A pre-placed socket path, file or pipe name is refused | TPT-006, TPT-008 or TPT-009 |
| `detector-self-check` | The leak and TCP detectors catch a planted secret and a real listener, so their "none found" results mean something | TPT-007, TPT-020 |

These results are **G0 evidence about the spike code**. They are not PASS
results for the G1 gates: G1 requires the production implementation.

## Run it

You need JDK 17 or later, Maven 3.9 and Go 1.27.3.

```sh
mvn -B -q -f worker/pom.xml -DskipTests package
cd host
go run . -java "$JAVA_HOME/bin/java" -jar ../worker/target/g0-worker.jar \
  -head "$(git rev-parse HEAD)" -out ../evidence/my-run.jsonl
```

`.github/workflows/adr010-g0-spike.yml` runs the same command on Linux,
macOS (Apple silicon and Intel) and Windows runners when this folder
changes, and uploads the evidence. That workflow is not a gate: it reports,
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

The OWN-11 limit still applies: these dependencies exist only in this
spike. Adding any of them to GoPMgr's `go.mod` needs G0 results, a
`docs/DEPENDENCIES.md` review and an authorized G1 slice.
