<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 implementation plan (for approval)

**Status:** PROPOSED, 2026-10-26. This plan asks the maintainer to
authorize two things:

- **slice P04**, the G1 worker contract, registry and supervisor
  ([implementation plan](adr-010-implementation-plan.md) §8.2);
- **the OWN-11 dependency slice**, per the
  [G1 dependency review](../interoperability/adr-010-g1-dependency-review.md).

**Until the maintainer approves it,** no G1 production code is written,
no dependency is added to `go.mod`, and nothing is integrated into a
release.

**Basis:**

- the [G1 transport design](adr-010-g1-transport-design.md) and the
  maintainer's [answers to Q1–Q5](../review/ADR-010-G1-open-questions-decision.md);
- the [G0 Windows transport report](../testing/adr-010-g0-windows-transport-report.md);
- the [test strategy](../testing/adr-010-test-strategy.md), test IDs
  TPT-006 to TPT-021.

## 1. What G1 delivers, and what it does not

From plan §8.2 (P04) and §8.3 (gate G1).

| G1 delivers | G1 does not include |
|---|---|
| The `.proto` contract, generated Go and Java code, and the compatibility tests | MPXJ or any file parsing (G2) |
| The internal registry: manifests, capabilities, compatibility | Project reads or writes, and any database change |
| The supervisor: launch, containment, admission, cancellation and grace, exit-code mapping, captured output | Any UI. The Plugin Center and job center come with G3 (P11) |
| One local-IPC transport per platform (§3) | A transport fallback (G1-Q1) |
| A **fake worker** in Java, built for tests only, that exercises every lifecycle path over the real transports | A shipped worker |
| The G1 test suite (§5) on every supported platform it can reach; the rest reported BLOCKED | A claim of Windows desktop support while Windows 11 x64 is BLOCKED (G1-Q4) |

**G1 gate** (plan §8.3): the fake worker and registry pass every lifecycle
and authentication test, and GoPMgr shows no regression with workers
disabled, on all supported platforms. Any platform G1 cannot test stays
BLOCKED, and G1 is not complete there.

## 2. Packages

These narrow plan §1.2 to what G1 needs. No `common` or `util` package
(AGENTS.md).

| Package | Owns | Depends on |
|---|---|---|
| `proto/gopmgr/interop/v1/` | The authoritative `.proto` sources | — |
| `internal/interop/contract` | Generated Go code and the descriptor set; schema lint | protobuf-go |
| `internal/interop/ipc` | `Transport` and `Endpoint` (design §3.1); admission (§3.3); one implementation per platform, chosen at build time | grpc-go, x/sys, go-winio (Windows) |
| `internal/interop/supervisor` | Process launch, Job Object or process group, clean environment, bootstrap, captured output, the §5.1 grace and escalation | ipc, x/sys |
| `internal/interop/job` | The D-06 lifecycle states, cancellation, exit-code and gRPC-status mapping to typed `interop.*` codes | supervisor, contract |
| `internal/interop/registry` | Manifests, capability filtering by version and direction, protocol compatibility | contract |
| `workers/fake-worker/` (Java, test only) | A grpc-java worker with scripted behaviors (crash, hang, slow exit, oversize, flood, bad proof) | grpc-java, Netty NIO |

**Rules carried from G0:**

- **One goroutine owns each job's state.** Code that runs in a gRPC
  handler reaches the worker only through a synchronized handle. G0's
  race detector found handler code reading the worker handle before the
  launching goroutine had published it.
- **The server waits for its handlers on Stop**
  (`grpc.WaitForHandlers(true)`), so a job's results are read only after
  its session has finished.
- **A check that cannot run is an error, never a pass.** This covers the
  peer check, the leak and socket checks in tests, and the tree count.

## 3. Transport per platform in G1

| Platform | G1 transport | Condition |
|---|---|---|
| Linux x64 | UDS, JDK NIO on the worker side | Final selection after design §4.1 on the approved hardware |
| macOS arm64 | UDS, JDK NIO | Same; plus the signed, notarized check (TPT-002) |
| Windows x64 | `npipe-async` (one overlapped pipe) | Conditional. Not final until design §8 passes, including Windows 11 x64 desktop and signed execution |

**New Windows requirement from the G0 code review (proposed as admission
rule 4).** G0 found two facts that together let a same-user process stand
between the worker and the host:

- go-winio creates pipes with unlimited instances;
- the spike's DACL grants the user generic access.

So, after the host creates the worker's pipe, another process of the same
user can add an instance of that name. The worker's open may then reach
that process. The JDK also opens pipes without `SECURITY_SQOS_PRESENT`
(`WindowsChannelFactory` and `io_util_md.c` in JDK 21u). So that server
could impersonate the worker at `SecurityImpersonation` level.

The host's peer check still refuses the intruder's own connection, so it
cannot reach the host. What it gains:

- a denial of service, as in residual R4;
- the worker's token;
- the ability to feed the worker input.

G1 must therefore:

1. **create the pipe with an instance limit** equal to what the transport
   needs: 1 for `npipe-async`, 2 for `npipe-dual`. Use first-instance
   creation and remote-client rejection. If go-winio's `ListenPipe` cannot
   set the limit, the listener creates the handle with
   `x/sys/windows.CreateNamedPipe` and wraps it with go-winio's
   `NewOpenFile`;
2. **have the host prove the credential to the worker** before the worker
   acts on any command. This is mutual authentication: today only the
   worker proves itself;
3. **pass the G0 `second-instance` scenario.** It is expected to FAIL with
   go-winio's listener. The G0 variant `npipe-async-single` prototypes the
   fix: one instance, held by the host, with foreign clients disconnected
   from it rather than closed. CI shows the two side by side from the
   round after `2813546`;
4. **test an elevated host** with a non-elevated same-user intruder on
   Windows 11 x64, to confirm what mandatory integrity control allows
   there.

Items 1 and 2 add to design §3.3 and change no accepted requirement. They
need the maintainer's approval as design changes (§8).

## 4. PR slices

Each slice is one small PR with its own tests. The order follows the
dependencies; nothing merges before its predecessor.

| # | Slice | Content | Exit tests |
|---|---|---|---|
| G1-1 | Dependencies (OWN-11) | `go.mod` additions per the dependency review; `docs/DEPENDENCIES.md` entries; govulncheck clean on the default and DuckDB builds plus `GOOS=windows,darwin` | `go mod verify`, govulncheck, `make license-check` |
| G1-2 | Contract | `proto/gopmgr/interop/v1`, pinned generators, committed generated code, drift check in CI, schema lint (no `Any`, no opaque authoritative bytes), golden compatibility fixtures | TPT-010, TPT-011 |
| G1-3 | ipc, Unix | UDS endpoint in a private `0700` directory; owner and mode checks; `SO_PEERCRED` / `LOCAL_PEERPID` peer check before HTTP/2; one-shot listener; cleanup | TPT-006, TPT-008, TPT-018, TPT-020 |
| G1-4 | ipc, Windows | `npipe-async` endpoint with user-only DACL, instance limit, first instance, remote rejection, `GetNamedPipeClientProcessId` peer check; one-shot | TPT-006, TPT-009, TPT-018, `second-instance` |
| G1-5 | Supervisor | Launch with fixed argv and clean environment; Job Object (Windows) or process group (Unix); bootstrap over stdin (INT-02); capped, unpersisted stdout and stderr; §5.1 grace, escalation and bound M; fail-closed containment | TPT-007, TPT-013, TPT-014, TPT-017, TPT-021, TPT-022 |
| G1-6 | Job lifecycle | D-06 states; cancellation; exit-code and status mapping to `interop.*` codes and ADR-008 keys; no raw worker text in user messages | TPT-012, TPT-014 |
| G1-7 | Registry | Manifests embedded in or verified against the signed release; capability filtering by version and direction; unknown mandatory schema or protocol major rejected | TPT-015, TPT-016, TPT-017 |
| G1-8 | Fake worker and the G1 suite | Java fake worker (test only), the bridge under BR-1 to BR-8 (BR-6a, BR-6b, BR-7a), and the full §5 suite in CI on every reachable platform | TPT-003, TPT-004, TPT-012, TPT-013, TPT-018, TPT-019 |
| G1-9 | Worker-disabled regression | GoPMgr with workers disabled: full existing suite, frontend smoke, release checks | Existing gates |

## 5. Test plan

**Pyramid.**

- **Unit:** table-driven tests per package. Admission decisions,
  descriptor parsing, exit-code mapping, grace-timer arithmetic and the
  lifecycle transitions.
- **Integration:** real grpc-go against the real Java fake worker, over
  each platform's transport. This is the G0 scenario set, rebuilt on the
  production code, not the spike.
- **System:** the packaged, signed app on owner-provided machines (TPT-002
  signed clause, Windows 11 x64). BLOCKED until resources exist (§7).

**Carried from G0** (design §8; each must pass on the production code):

- handshake, duplex, ping, `connect-repeat` (50 launches, zero failures);
- backpressure, `cancel-backpressure`, bridge self-tests including the B9
  empty-buffer test;
- cancel × 30: p50, p95 and p99 within G, with no escalation;
- the at-exit native-wait check (W2) and the uncaught-exception check
  (B8);
- `tree-kill`, `crash`, `kill`, `oom`;
- `impersonate`, `connect-flood`, `bad-credential`, `replay` × 10, and
  the leak detector with its self-check;
- `squat`, `second-instance`, `oversize`, `concurrent`;
- `bootstrap-*`, `handshake-timeout`, `missing-jvm`;
- offline runs (network namespace, `sandbox-exec`, firewall), each proved
  by NetCheck.

**New in G1:**

| Test | What it proves | ID |
|---|---|---|
| Race detector | The supervisor, ipc and job packages under `-race`, in unit and integration runs, on every platform the race detector supports | — (engineering rule) |
| Grace escalation | A fake worker that ignores cancel is killed at G, reaped within K, and reported `PLUGIN_UNHEALTHY` if not reaped by M; the escalation is counted | TPT-013 |
| Mutual authentication | A server that cannot prove the credential gets no worker action beyond hello | TPT-018 |
| Elevated host (Windows) | An elevated host with a non-elevated intruder, on Windows 11 x64 | TPT-009, TPT-018 |
| Containment wording | No UI or document string calls the worker sandboxed or contained (G1-Q2) | TPT-019 |
| Protocol compatibility | N, N−1 and N+1 golden fixtures; reserved field numbers | TPT-010 |

**Coverage target:** every exported function in `internal/interop/...` is
covered by a unit test. Every admission and lifecycle branch is covered by
a test that would fail if the branch were removed. The existing coverage
ratchet applies.

## 6. Gates per PR

- `go test ./internal/interop/...` and `go test -race` on it;
- `gofmt`, `go vet`, govulncheck;
- the Java fake worker's build and its unit tests;
- the G1 integration suite in CI on Linux x64, Linux arm64, macOS arm64,
  Windows x64 and Windows arm64 runners. These are evidence for the
  hosted images only. Owner-machine rows stay BLOCKED;
- `make license-check`, REUSE, `git diff --check`;
- for G1-9: `make check-release`, `make frontend-smoke` and the full
  existing suite with workers disabled.

## 7. Blocked items and the resources they need

| Item | Needed for | Resource | Status |
|---|---|---|---|
| Windows 11 x64 desktop | Any Windows desktop claim (G1-Q4) | A hosted larger runner with the Windows 11 desktop image, a self-hosted runner, an Azure VM, or an owner device | **BLOCKED** |
| Windows 10 x64 | Only if it stays in the support matrix (`docs/INSTALL.md` lists it today) | No hosted image exists; a VM or device | **BLOCKED**, and a maintainer decision |
| macOS 13 arm64 | GoPMgr's stated minimum | No hosted image; an owner device | **BLOCKED** |
| Developer-ID test signing | Team-ID library validation and notarization behavior | A test signing identity held in a signing service or CI secret store. **No private key in the repository, fixtures, logs or chat** | **BLOCKED** |
| Authenticode test signing | Signed execution on Windows; WDAC behavior | A test signing service and a WDAC test VM | **BLOCKED** |
| Approved hardware profiles (OWN-10) | Final G, K and M; design §4.1 | Owner decision after S0 | **BLOCKED** |
| Production signing and notarization | Release only | Separately authorized credentials | Out of G1 |

## 8. Decisions requested

1. **Authorize slice P04** as planned in §4, in that order.
2. **Approve the OWN-11 dependency slice** per the dependency review.
3. **Approve the two Windows admission additions** in §3: the instance
   limit, and mutual authentication.
4. **Approve the provisional bounds** in design §5.1 (K = 1 s, M = 2 s)
   and the §4.1 performance budgets.
5. **Decide whether Windows 10 x64 stays** in the support matrix.
6. **Choose the resources in §7.** Name which machines and signing
   services the project will provide, or confirm they stay BLOCKED.
7. **TPT-002's wording** names "native Netty runtime". With JDK NIO
   preferred (G1-Q3), propose reading it as "the selected UDS runtime".
   That is an ACR-level wording change, not a weakening: the
   signed-installed-app check still applies in full.
