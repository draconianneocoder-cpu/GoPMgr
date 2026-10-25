<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G0 dependency, native-code and license inventory

**Status:** Draft inventory for owner and legal review, 2026-10-25. It
records what the G0 spike actually links or bundles at `7d67cae`, and what
MPXJ 16.10.0 declares. **It is not a legal opinion.** Licenses below are the
ones each project declares in its own metadata. This report asserts no
LGPL, signing or notarization compliance (handoff rule). Adding any of
these to GoPMgr's `go.mod` or release still needs an authorized G1 slice
and a `docs/DEPENDENCIES.md` review (OWN-11).

## 1. Go host (spike module `spikes/adr010-g0-transport/host`)

These are the modules actually linked into the probe binary, from
`go list -deps` per OS. The module graph in `go.sum` is larger; most of
it is never compiled in.

| Module | Version | Linked on | Declared license (LICENSE file) |
|---|---|---|---|
| google.golang.org/grpc | v1.84.0 | all | Apache-2.0 |
| google.golang.org/genproto/googleapis/rpc | v0.0.0-20260706201446-f0a921348800 | all | Apache-2.0 |
| google.golang.org/protobuf | v1.36.12 | all | BSD-3-Clause |
| golang.org/x/net | v0.57.0 | all | BSD-3-Clause |
| golang.org/x/sys | v0.49.0 | all | BSD-3-Clause |
| golang.org/x/text | v0.40.0 | all | BSD-3-Clause |
| github.com/Microsoft/go-winio | v0.6.4 | Windows only | MIT |

Source: each module's `LICENSE` file in the Go module cache, with the
checksums pinned by `host/go.sum`.

## 2. Java worker (`spikes/adr010-g0-transport/worker`, shaded `g0-worker.jar`)

Every artifact bundled into the shaded jar. The license is declared in
the artifact's POM, or in its parent POM where noted.

| Artifact | Version | Declared license |
|---|---|---|
| io.grpc: grpc-api, grpc-context, grpc-core, grpc-netty, grpc-protobuf, grpc-protobuf-lite, grpc-stub, grpc-util | 1.85.1 | Apache-2.0 |
| io.netty: netty-buffer, -codec-base, -codec-compression, -codec-http, -codec-http2, -codec-socks, -common, -handler, -handler-proxy, -resolver, -transport, -transport-classes-epoll, -transport-classes-kqueue, -transport-native-epoll, -transport-native-kqueue, -transport-native-unix-common | 4.2.17.Final | Apache-2.0 (netty-parent) |
| com.google.protobuf:protobuf-java | 3.25.9 | BSD-3-Clause (protobuf-parent) |
| com.google.api.grpc:proto-google-common-protos | 2.64.1 | Apache-2.0 |
| com.google.guava:guava | 33.6.0-android | Apache-2.0 (guava-parent) |
| com.google.guava:failureaccess | 1.0.3 | Apache-2.0 (guava-parent) |
| com.google.guava:listenablefuture | 9999.0-empty-to-avoid-conflict-with-guava | Empty placeholder artifact; no license declared |
| com.google.code.gson:gson | 2.14.0 | Apache-2.0 |
| com.google.errorprone:error_prone_annotations | 2.50.0 | Apache-2.0 |
| com.google.j2objc:j2objc-annotations | 3.1 | Apache-2.0 |
| com.google.android:annotations | 4.1.1.4 | Apache-2.0 |
| com.google.code.findbugs:jsr305 | 3.0.2 | Apache-2.0 |
| io.perfmark:perfmark-api | 0.27.0 | Apache-2.0 |
| org.jspecify:jspecify | 1.0.0 | Apache-2.0 |
| org.jctools:jctools-core | 4.0.6 | Apache-2.0 |
| org.codehaus.mojo:animal-sniffer-annotations | 1.27 | MIT (animal-sniffer-parent) |

`javax.annotation:javax.annotation-api` 1.3.2 ("CDDL + GPLv2 with
classpath exception") was bundled up to `7d67cae` and is removed in the
commit that adds this report; see L1.

### 2.1 Native code in the worker jar

| File | Platform | SHA-256 |
|---|---|---|
| `META-INF/native/libnetty_transport_native_epoll_x86_64.so` | Linux amd64 | `2e37cf5f75a64759e9ae88afb6bcfff7c5fc14bab8ee51c6c026b9235c9331b5` |
| `META-INF/native/libnetty_transport_native_epoll_aarch_64.so` | Linux arm64 | `f8aeaafaa8532b423a3db750f9e2c5eba4bb33f3aa119b8038532f555c587213` |
| `META-INF/native/libnetty_transport_native_kqueue_x86_64.jnilib` | macOS Intel | `bffc7d11af9299008e82795fe6c494bc3223bd23b6b1db3719f586a8909f703e` |
| `META-INF/native/libnetty_transport_native_kqueue_aarch_64.jnilib` | macOS arm64 | `cb073e4d374a26fd8ce78dbb7ef7480799068725661e129fea0af4f69bb34f03` |

Netty extracts these to a temporary folder at run time. On macOS that
interacts with code signing, hardened runtime and notarization; the
packaging spike must test it (TPT-002, BLOCKED). A transport that needs no
native library (Netty NIO with JDK Unix sockets) passed on every OS
tested, so the native libraries are optional for G1.

## 3. MPXJ 16.10.0 (future worker, not bundled in G0)

Source: `https://repo1.maven.org/maven2/net/sf/mpxj/mpxj/16.10.0/mpxj-16.10.0.pom`,
SHA-1 `85c4134f6ac561940ed6119c734618b0e63d61dd` (matches Maven Central's
published `.sha1`), SHA-256
`9179e340f062462bd1024a7fae593e0d18e7683bcd20367cf93b96017bf9cabb`.

Declared license: **GNU Lesser General Public License v2.1 or later.**

Declared compile dependencies (their own licenses not yet resolved):

| Dependency | Version | Note |
|---|---|---|
| org.apache.poi:poi | 5.5.1 | Microsoft OLE2/MPP container reading |
| org.xerial:sqlite-jdbc | 3.51.2.0 | **Bundles native SQLite libraries for many platforms**; a second SQLite engine, separate from GoPMgr's SQLCipher |
| com.healthmarketscience.jackcess:jackcess | 4.0.10 | Access database files |
| jakarta.xml.bind:jakarta.xml.bind-api | 3.0.1 | XML binding API |
| org.glassfish.jaxb:jaxb-runtime | 3.0.2 | XML binding runtime |
| com.fasterxml.jackson.core:jackson-core, jackson-databind | 2.22.3 | JSON |
| org.jsoup:jsoup | 1.23.1 | HTML |
| com.github.joniles:rtfparserkit | 1.16.0 | RTF notes |
| com.jgoodies:jgoodies-binding | 2.13.0 | Declared at compile scope |

## 4. Items for review before G1 or G2

| # | Item | Why | Suggested action |
|---|---|---|---|
| L1 | `javax.annotation-api` 1.3.2 was declared for `@javax.annotation.Generated`, but grpc-java 1.85.1 stubs use `@io.grpc.stub.annotations.GrpcGenerated` instead, and no bundled class referenced the API. | It was the only CDDL/GPLv2-style declared license in the jar. | **Done:** removed from `worker/pom.xml`. The rebuilt jar has none of its classes and passes 26/26 on Linux; CI re-runs all four platforms. |
| L2 | protobuf-java (BSD-3-Clause), animal-sniffer (MIT) and the Go BSD/MIT modules need their notices shipped. | Attribution terms. | Add them to the release notices in the G1 packaging slice. |
| L3 | MPXJ is LGPL-2.1-or-later. | GoPMgr is GPL-3.0-or-later and the worker runs as a separate process. Obligations must be confirmed by legal review, not assumed here. | Legal/package review before G2 ships MPXJ. |
| L4 | MPXJ pulls `sqlite-jdbc`, which ships native libraries, and POI, Jackcess, JAXB and Jackson. | More native code to sign and notarize, and a much larger attack surface for parsing hostile files. | Resolve the full MPXJ dependency tree and its licenses in G2; decide whether the MPXJ worker may exclude formats that need `sqlite-jdbc`. |
| L5 | No machine-readable SBOM yet. | The plan asks for an SBOM. | Generate SPDX or CycloneDX in the G1 packaging slice. No SBOM tool is added now, to avoid a new dependency. |
