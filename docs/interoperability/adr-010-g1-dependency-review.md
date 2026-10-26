<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 dependency review (for approval)

**Status:** **CONDITIONALLY APPROVED** by the maintainer on 2026-10-26
([P04-D2](../review/ADR-010-G1-P04-authorization-decision.md)). The
conditions:

- security, licensing, compatibility and reproducible-build verification;
- the patched x/net and x/text versions kept;
- the complete dependency graph recorded in `docs/DEPENDENCIES.md`.

The verification and the `go.mod` change are slice G1-1.

**Basis:**

- the [G0 dependency, native-code and license inventory](adr-010-g0-dependency-report.md),
  which lists what the spike links and bundles and is not repeated here;
- the [G1 transport design](../design/adr-010-g1-transport-design.md);
- the maintainer's [G1 open questions decision](../review/ADR-010-G1-open-questions-decision.md),
  in particular Q3 (prefer JDK NIO on Unix).

**Not a legal opinion.** The licenses below are the ones each project
declares. Notice and redistribution duties for a bundled runtime remain
legal-review items L2, L5 and L6 of the G0 report.

## 1. Why new dependencies at all

OWN-02 and D-03 require native gRPC over local IPC, with one generated
contract on every OS. The Go standard library has no gRPC or protobuf
implementation, so GoPMgr needs grpc-go and protobuf-go. The worker needs
grpc-java. Writing either protocol stack in-house would be a far larger
and riskier dependency on our own code. OWN-03 rules out the alternatives
(TCP, a stdio protocol).

## 2. Go modules (host)

Checked 2026-10-26 with `go list -m -versions` and govulncheck v1.9.0
(database `vuln.go.dev`), built with Go 1.27.3, for `GOOS` linux, darwin
and windows.

| Module | Proposed | Latest stable | Role | Linked on | License | Notes |
|---|---|---|---|---|---|---|
| `google.golang.org/grpc` | v1.84.0 | v1.84.0 (2026-09-17) | gRPC server over the selected local transport | all | Apache-2.0 | Direct |
| `google.golang.org/protobuf` | v1.36.12 | v1.36.12 | Generated contract runtime | all | BSD-3-Clause | Direct |
| `github.com/Microsoft/go-winio` | v0.6.4 | v0.6.4 | Named-pipe listener with an explicit DACL, first-instance creation and remote-client rejection | Windows only | MIT | Direct. Needed only while a named pipe is the selected Windows transport |
| `golang.org/x/sys` | v0.49.0 (already in `go.mod`) | v0.49.0 | Job Objects, process creation, peer credentials | all | BSD-3-Clause | Existing direct dependency |
| `golang.org/x/net` | **≥ v0.60.0** (main module has v0.60.0) | v0.61.0 | HTTP/2 framing under grpc-go | all | BSD-3-Clause | Indirect. See §2.1 |
| `golang.org/x/text` | **≥ v0.41.0** (main module has v0.43.0) | v0.43.0 | Indirect via x/net | all | BSD-3-Clause | Indirect. See §2.1 |
| `google.golang.org/genproto/googleapis/rpc` | as grpc-go requires | — | `google.rpc.Status` | all | Apache-2.0 | Indirect |

**Alternatives considered for go-winio:**

- calling `CreateNamedPipe` and overlapped I/O through `x/sys/windows`
  directly. GoPMgr would then own subtle security-descriptor and
  overlapped-I/O code that go-winio already maintains, and that
  containerd and Docker use widely;
- not using a pipe at all, that is AF_UNIX. That changes OWN-02's order
  and needs an ACR.

### 2.1 Vulnerability finding in the spike (fixed)

govulncheck found that the spike module pinned `golang.org/x/net` v0.57.0
and `golang.org/x/text` v0.40.0 as indirect dependencies:

| Advisory | Summary | Reachable from the spike | Fixed in |
|---|---|---|---|
| GO-2026-6603 | HTTP/2 server memory exhaustion via trailer headers | Yes, via `grpc.Server.Serve` | x/net v0.60.0 |
| GO-2026-6611 | Excessive CPU from repeated initial-window changes | Yes | x/net v0.60.0 |
| GO-2026-6612 | Double flow-control refund on HTTP/2 server streams | Yes | x/net v0.60.0 |
| GO-2026-6617 | HTTP/2 server crash from an HPACK encoder race | Yes | x/net v0.60.0 |
| GO-2026-6610 | HTTP/2 transport accepts malformed framing headers | Package imported, not called | x/net v0.60.0 |
| GO-2026-6629 | Panic parsing crafted input in `x/text/secure/precis` | Module only | x/text v0.41.0 |

The spike now pins x/net v0.61.0 and x/text v0.43.0, and govulncheck
reports no vulnerabilities on linux, darwin or windows. GoPMgr's own
`go.mod` already has x/net v0.60.0 and x/text v0.43.0. **G1 must keep both
at or above the fixed versions.** The existing govulncheck CI job then
covers the new modules.

### 2.2 Code generation (build time only, not linked)

| Tool | Version | Source and check |
|---|---|---|
| `protoc` | 25.9 (libprotoc), to match protobuf-java 3.25.9 | Maven Central `com.google.protobuf:protoc`, checksum verified |
| `protoc-gen-go` | v1.36.12 | `go install`, module checksum database |
| `protoc-gen-go-grpc` | v1.6.2 | `go install`, module checksum database |
| `protoc-gen-grpc-java` | 1.85.1 | Maven Central `io.grpc:protoc-gen-grpc-java`, checksum verified |

The generated code is committed, as in the spike, so normal builds need no
generator. A CI check regenerates and fails on any difference.

## 3. Java worker artifacts

Checked 2026-10-26 against Maven Central metadata and the OSV database
(`api.osv.dev`, Maven ecosystem): **no advisories** for any version
below.

| Artifact | Proposed | Latest on Maven Central | License | Notes |
|---|---|---|---|---|
| `io.grpc:grpc-netty`, `grpc-protobuf`, `grpc-stub` and their grpc-java dependencies | 1.85.1 | 1.85.1 | Apache-2.0 | Generated stubs use `@GrpcGenerated`; `javax.annotation-api` is not needed (G0 L1) |
| `io.netty:*` (transport, codec-http2, handler, buffer, common, resolver) | **latest 4.2.x patch when the slice starts** (4.2.19.Final today; the spike has 4.2.17.Final) | 4.2.19.Final | Apache-2.0 | **NIO only.** See §3.1 |
| `com.google.protobuf:protobuf-java` | 3.25.9 | 4.36.2 (4.37.0-RC1 is a release candidate) | BSD-3-Clause | Kept on 3.25 to match `protoc` 25.9 and grpc-java's tested line. Moving to 4.x is a separate decision with its own generator change |
| Transitive (guava, gson, perfmark, error_prone_annotations and others) | as resolved by grpc-java 1.85.1 | — | Apache-2.0 or MIT | Listed with checksums in the G0 report §2 |

### 3.1 Native libraries: none in the proposed worker

Following G1-Q3, the proposed worker uses Netty's NIO transport with JDK
Unix-domain sockets (`NioDomainSocketChannel`) on Linux and macOS. The
following artifacts are **dropped** from the production worker:

- `netty-transport-native-epoll`, with its `linux-x86_64` and
  `linux-aarch_64` classifiers;
- `netty-transport-native-kqueue`, with its `osx-x86_64` and
  `osx-aarch_64` classifiers;
- `netty-transport-classes-epoll` and `netty-transport-classes-kqueue`;
- `netty-transport-native-unix-common`.

That removes the four native libraries listed in the G0 report §2.1. It
also removes their run-time extraction to a temporary directory, and
their signing and notarization. On Windows the pipe bridge uses the
JDK's `AsynchronousFileChannel`, so no native library is needed there
either.

**The native transports stay in the G0 spike** as the §4.1 comparison
baseline. If the target-hardware comparison rejects NIO, adding them back
is a new dependency decision, not a fallback.

## 4. Runtime image (shipped with the worker)

| Item | Proposed | Notes |
|---|---|---|
| JDK for jlink | Eclipse Temurin 21 LTS, latest update when packaging starts (G0 used 21.0.12.1+1, checksum verified) | GPL-2.0 with Classpath Exception; `legal/` folder kept (G0 L6) |
| Modules | from `jdeps` on the final worker jar (G0: 9 modules, 65 MB on Linux x64) | Re-run `jdeps` for each worker release |
| JVM flags | as G1 design §6 | No attach, exit on OOM, explicit `-Xmx` |

## 5. Checks this slice must run before merge

1. `go mod tidy` with no unrelated changes. The `go.mod` diff is reviewed
   line by line.
2. govulncheck on the default build and the DuckDB build (existing CI
   job), plus `GOOS=windows` and `GOOS=darwin`.
3. OSV query for every bundled Maven artifact, recorded with the date.
4. License inventory regenerated, and notices added to the release
   bundle (G0 L2). SBOM generation is decided in the packaging slice
   (G0 L5).
5. `make license-check` and REUSE compliance for the generated code.
6. A `docs/DEPENDENCIES.md` entry for each new direct module, using
   the existing format: purpose, scope and build tags.

## 6. Proposed `docs/DEPENDENCIES.md` entries (not yet applied)

```markdown
- `google.golang.org/grpc`: gRPC server for the ADR-010 worker connection
  (`internal/interop/ipc`). Local IPC only: Unix-domain sockets on Linux
  and macOS, a named pipe on Windows (OWN-02). No TCP listener (OWN-03).
- `google.golang.org/protobuf`: runtime for the generated worker contract
  (`internal/interop/contract`). Generated code is committed and checked
  for drift in CI.
- `github.com/Microsoft/go-winio`: Windows named-pipe listener with an
  explicit user-only DACL (`internal/interop/ipc`, Windows build only).
```

## 7. Decision requested

Approve, change or reject each item:

1. The three new direct Go modules in §2, at the proposed versions or
   newer patch releases, with x/net ≥ v0.60.0 and x/text ≥ v0.41.0.
2. The Java worker set in §3, NIO only, with no native library.
3. Keeping protobuf-java on 3.25.x for G1.
4. The checks in §5 as merge conditions for the dependency slice.
